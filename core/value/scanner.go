package value

import (
	"bytes"
	"encoding"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xreflect"
)

type jsonUnmarshaler interface {
	UnmarshalJSON([]byte) error
}

type unmarshalerKind uint8

const (
	unmarshalerNone unmarshalerKind = iota
	unmarshalerJSON
	unmarshalerText
)

var (
	jsonUnmarshalerType = reflect.TypeOf((*jsonUnmarshaler)(nil)).Elem()
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	unmarshalerKinds    sync.Map // map[reflect.Type]unmarshalerKind
	anyMapType          = reflect.TypeOf(map[string]any(nil))
	durationType        = reflect.TypeOf(time.Duration(0))
)

// unmarshalerKindOf reports which custom decoder the addressable destination type implements.
// Results are cached by type so that ordinary fields avoid interface boxing.
func unmarshalerKindOf(t reflect.Type) unmarshalerKind {
	if cached, ok := unmarshalerKinds.Load(t); ok {
		return cached.(unmarshalerKind)
	}

	kind := unmarshalerNone
	pointer := reflect.PointerTo(t)

	switch {
	case pointer.Implements(jsonUnmarshalerType):
		kind = unmarshalerJSON
	case pointer.Implements(textUnmarshalerType):
		kind = unmarshalerText
	}

	actual, _ := unmarshalerKinds.LoadOrStore(t, kind)

	return actual.(unmarshalerKind)
}

// scanValue scans src into dst, converting values between compatible types. The default tag of a
// struct field is applied when the field is absent or empty.
func scanValue(src any, dst reflect.Value) error {
	if !dst.CanSet() {
		return nil
	}

	if dst.Kind() == reflect.Interface {
		return scanInterface(src, dst)
	}

	if !xreflect.IsNil(src) {
		rv := reflect.ValueOf(src)
		if rv.Type().AssignableTo(dst.Type()) && !isContainerKind(dst.Kind()) {
			dst.Set(rv)
			return nil
		}
	}

	if kind := unmarshalerKindOf(dst.Type()); kind != unmarshalerNone {
		if ok, err := scanWithUnmarshaler(src, dst, kind); ok {
			return err
		}
	}

	switch dst.Kind() {
	case reflect.Pointer:
		if xreflect.IsNil(src) {
			return nil
		}
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return scanValue(src, dst.Elem())
	case reflect.Struct:
		return scanStruct(src, dst)
	case reflect.Map:
		return scanMap(src, dst)
	case reflect.Slice, reflect.Array:
		return scanSlice(src, dst)
	default:
		return scanScalar(src, dst)
	}
}

// scanInterface assigns src to an interface destination, copying JSON node containers so that the
// caller cannot mutate the config snapshot through the scanned value.
func scanInterface(src any, dst reflect.Value) error {
	if xreflect.IsNil(src) {
		return nil
	}

	rv := reflect.ValueOf(src)
	if dst.Type().NumMethod() == 0 {
		dst.Set(reflect.ValueOf(cloneNode(src)))
		return nil
	}
	if !rv.Type().Implements(dst.Type()) {
		msg := "scan: " + rv.Type().String() + " does not implement " + dst.Type().String()
		return errors.NewError(msg)
	}

	dst.Set(rv)

	return nil
}

// scanWithUnmarshaler gives custom decoders precedence over the built-in conversion rules.
func scanWithUnmarshaler(src any, dst reflect.Value, kind unmarshalerKind) (bool, error) {
	if kind == unmarshalerNone || !dst.CanAddr() {
		return false, nil
	}

	addr := dst.Addr().Interface()
	if kind == unmarshalerJSON {
		u := addr.(jsonUnmarshaler)
		if xreflect.IsNil(src) {
			return true, nil
		}
		data, err := jsonSource(src)
		if err != nil {
			return true, err
		}
		return true, u.UnmarshalJSON(data)
	}

	u := addr.(encoding.TextUnmarshaler)
	if xreflect.IsNil(src) {
		return true, nil
	}
	text, err := textSource(src)
	if err != nil {
		return true, err
	}
	return true, u.UnmarshalText([]byte(text))
}

// scanStruct decodes an object into a struct, matching fields by json tag or field name.
func scanStruct(src any, dst reflect.Value) error {
	obj, err := scanObject(src)
	if err != nil {
		return err
	}

	info := structInfoOf(dst.Type())

	var folded map[string]any

	for _, field := range info.fields {
		fv, err := fieldByIndex(dst, field.index, true)
		if err != nil {
			return wrapFieldError(field.name, err)
		}

		var (
			val any
			ok  bool
		)
		if obj != nil {
			val, ok = obj[field.name]
			if !ok {
				if folded == nil {
					folded = foldObject(obj, info.lowers)
				}
				val = folded[field.lower]
			}
		}
		if isMissingValue(val, fv) {
			if field.def == "" {
				val = nil
			} else {
				val = field.defValue
			}
		}

		if err = scanValue(val, fv); err != nil {
			return wrapFieldError(field.name, err)
		}
	}

	return nil
}

// scanMap decodes an object into a map, converting keys and values to the declared map types.
func scanMap(src any, dst reflect.Value) error {
	obj, err := scanObject(src)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil
	}

	if dst.Type() == anyMapType {
		out := make(map[string]any, len(obj))
		for key, val := range obj {
			out[key] = cloneNode(val)
		}
		dst.Set(reflect.ValueOf(out))
		return nil
	}

	out := reflect.MakeMapWithSize(dst.Type(), len(obj))
	for key, val := range obj {
		kv := reflect.New(dst.Type().Key()).Elem()
		if err = scanValue(key, kv); err != nil {
			return wrapFieldError(key, err)
		}

		vv := reflect.New(dst.Type().Elem()).Elem()
		if err = scanValue(val, vv); err != nil {
			return wrapFieldError(key, err)
		}

		out.SetMapIndex(kv, vv)
	}
	dst.Set(out)

	return nil
}

// scanSlice decodes a slice or array from another slice, an array or a delimited string.
func scanSlice(src any, dst reflect.Value) error {
	if xreflect.IsNil(src) {
		return nil
	}

	if dst.Type().Elem().Kind() == reflect.Uint8 {
		if s, ok := stringValue(src); ok {
			return setBytesValue(dst, []byte(s))
		}
		if data, ok := bytesValue(src); ok {
			return setBytesValue(dst, bytes.Clone(data))
		}
	}

	if srcValue, ok := sliceSource(src); ok {
		return scanSliceValue(srcValue, dst)
	}

	items, err := scanItems(src)
	if err != nil {
		return err
	}

	if dst.Kind() == reflect.Array {
		count := min(len(items), dst.Len())
		for i := range count {
			if err = scanValue(items[i], dst.Index(i)); err != nil {
				return wrapIndexError(i, err)
			}
		}
		return nil
	}

	out := reflect.MakeSlice(dst.Type(), len(items), len(items))
	for i := range items {
		if err = scanValue(items[i], out.Index(i)); err != nil {
			return wrapIndexError(i, err)
		}
	}
	dst.Set(out)

	return nil
}

// scanScalar converts a scalar source into a scalar destination.
func scanScalar(src any, dst reflect.Value) error {
	if xreflect.IsNil(src) {
		return nil
	}

	if dst.Type() == durationType {
		d, err := parseDuration(src)
		if err != nil {
			return err
		}
		dst.SetInt(int64(d))
		return nil
	}

	switch dst.Kind() {
	case reflect.String:
		dst.SetString(xconv.String(src))
	case reflect.Bool:
		b, err := parseBool(src)
		if err != nil {
			return err
		}
		dst.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := parseInt(src, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := parseUint(src, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := parseFloat(src, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetFloat(n)
	default:
		return errors.NewError("scan: unsupported destination type " + dst.Type().String())
	}

	return nil
}

// scanObject normalizes src into a string-keyed object.
func scanObject(src any) (map[string]any, error) {
	if xreflect.IsNil(src) {
		return nil, nil
	}

	switch v := src.(type) {
	case map[string]any:
		return v, nil
	case string:
		return decodeObject([]byte(v))
	case []byte:
		return decodeObject(v)
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return decodeObject([]byte(rv.String()))
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return decodeObject(rv.Bytes())
		}
		return nil, unsupportedSource(rv, "object")
	case reflect.Map:
		obj := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			obj[xconv.String(iter.Key().Interface())] = iter.Value().Interface()
		}
		return obj, nil
	case reflect.Struct:
		return structObject(rv)
	default:
		return nil, unsupportedSource(rv, "object")
	}
}

// sliceSource returns the source slice or array value without building an intermediate []any.
func sliceSource(src any) (reflect.Value, bool) {
	if xreflect.IsNil(src) {
		return reflect.Value{}, false
	}

	switch src.(type) {
	case string, []byte:
		return reflect.Value{}, false
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return reflect.Value{}, false
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return reflect.Value{}, false
		}
		return rv, true
	default:
		return reflect.Value{}, false
	}
}

// scanSliceValue scans a slice or array source directly, avoiding an intermediate []any.
func scanSliceValue(src, dst reflect.Value) error {
	if dst.Kind() == reflect.Array {
		count := min(src.Len(), dst.Len())
		for i := range count {
			if err := scanSliceElement(src.Index(i), dst.Index(i), i); err != nil {
				return err
			}
		}
		return nil
	}

	if src.Type().Elem() == dst.Type().Elem() && dst.Type().Elem().Kind() != reflect.Interface {
		out := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
		reflect.Copy(out, src)
		dst.Set(out)
		return nil
	}

	out := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
	for i := range src.Len() {
		if err := scanSliceElement(src.Index(i), out.Index(i), i); err != nil {
			return err
		}
	}
	dst.Set(out)

	return nil
}

// scanSliceElement converts one element, using a direct primitive path when possible.
func scanSliceElement(src, dst reflect.Value, index int) error {
	if ok, err := scanPrimitive(src, dst); ok {
		if err != nil {
			return wrapIndexError(index, err)
		}
		return nil
	}

	if err := scanValue(src.Interface(), dst); err != nil {
		return wrapIndexError(index, err)
	}

	return nil
}

// scanPrimitive converts primitive element values directly, avoiding the interface boxing that
// scanValue would require. It reports whether the source kind was handled.
func scanPrimitive(src, dst reflect.Value) (bool, error) {
	switch src.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := src.Int()
		switch dst.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if dst.OverflowInt(n) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if n < 0 || dst.OverflowUint(uint64(n)) {
				return true, errors.NewError("scan: value cannot be converted to " + dst.Type().String())
			}
			dst.SetUint(uint64(n))
		case reflect.Float32, reflect.Float64:
			f := float64(n)
			if dst.OverflowFloat(f) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetFloat(f)
		default:
			return false, nil
		}
		return true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n := src.Uint()
		switch dst.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if dst.OverflowUint(n) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetUint(n)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if n > math.MaxInt64 || dst.OverflowInt(int64(n)) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetInt(int64(n))
		case reflect.Float32, reflect.Float64:
			f := float64(n)
			if dst.OverflowFloat(f) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetFloat(f)
		default:
			return false, nil
		}
		return true, nil
	case reflect.Float32, reflect.Float64:
		f := src.Float()
		switch dst.Kind() {
		case reflect.Float32, reflect.Float64:
			if dst.OverflowFloat(f) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetFloat(f)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if math.Trunc(f) != f || f < math.MinInt64 || f >= math.MaxInt64 {
				return true, errors.NewError("scan: value cannot be converted to " + dst.Type().String())
			}
			n := int64(f)
			if dst.OverflowInt(n) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if math.Trunc(f) != f || f < 0 || f >= math.MaxUint64 {
				return true, errors.NewError("scan: value cannot be converted to " + dst.Type().String())
			}
			n := uint64(f)
			if dst.OverflowUint(n) {
				return true, errors.NewError("scan: value overflows " + dst.Type().String())
			}
			dst.SetUint(n)
		default:
			return false, nil
		}
		return true, nil
	case reflect.Bool:
		if dst.Kind() != reflect.Bool {
			return false, nil
		}
		dst.SetBool(src.Bool())
		return true, nil
	case reflect.String:
		if dst.Kind() != reflect.String {
			return false, nil
		}
		dst.SetString(src.String())
		return true, nil
	}

	return false, nil
}

// scanItems normalizes src into a slice of elements.
func scanItems(src any) ([]any, error) {
	if xreflect.IsNil(src) {
		return nil, nil
	}

	switch v := src.(type) {
	case string:
		return splitItems(v)
	case []byte:
		return splitItems(string(v))
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return splitItems(rv.String())
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			return splitItems(string(rv.Bytes()))
		}
		items := make([]any, rv.Len())
		for i := range items {
			items[i] = rv.Index(i).Interface()
		}
		return items, nil
	default:
		return nil, unsupportedSource(rv, "slice")
	}
}

func decodeObject(data []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] != '{' {
		return nil, errors.NewError("scan: source is not a JSON object")
	}

	var obj map[string]any
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func structObject(rv reflect.Value) (map[string]any, error) {
	info := structInfoOf(rv.Type())
	obj := make(map[string]any, len(info.fields))
	for _, field := range info.fields {
		fv, err := fieldByIndex(rv, field.index, false)
		if err != nil {
			continue
		}
		if !fv.IsValid() || !fv.CanInterface() {
			continue
		}
		obj[field.name] = fv.Interface()
	}

	return obj, nil
}

// splitItems parses a JSON array or a comma-separated list.
func splitItems(s string) ([]any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	if s[0] == '[' {
		var items []any
		if err := json.Unmarshal([]byte(s), &items); err != nil {
			return nil, err
		}
		return items, nil
	}

	parts := strings.Split(s, ",")
	items := make([]any, len(parts))
	for i := range parts {
		items[i] = strings.TrimSpace(parts[i])
	}

	return items, nil
}

// fieldByIndex resolves a cached field index path. When alloc is true, nil embedded pointers on
// the way are allocated; otherwise the field is reported as unavailable.
func fieldByIndex(v reflect.Value, index []int, alloc bool) (reflect.Value, error) {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !alloc || !v.CanSet() {
					return reflect.Value{}, errors.NewError("scan: nil embedded pointer")
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}

		if v.Kind() != reflect.Struct {
			return reflect.Value{}, errors.NewError("scan: invalid field index")
		}
		v = v.Field(x)
	}

	return v, nil
}

// foldObject builds a lower-case key index restricted to the keys the struct can consume.
func foldObject(obj map[string]any, lowers map[string]struct{}) map[string]any {
	folded := make(map[string]any, min(len(obj), len(lowers)))
	for key, val := range obj {
		lower := strings.ToLower(key)
		if _, ok := lowers[lower]; ok {
			folded[lower] = val
		}
	}

	return folded
}

// isMissingValue reports whether a struct field should fall back to its default. An empty string
// or byte slice counts as missing for every destination except string and byte slices, where the
// empty value itself is meaningful.
func isMissingValue(val any, dst reflect.Value) bool {
	if val == nil {
		return true
	}

	switch v := val.(type) {
	case string:
		return v == "" && !keepsEmptyValue(dst)
	case []byte:
		if v == nil {
			return true
		}
		return len(v) == 0 && !keepsEmptyValue(dst)
	case bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return false
	}

	rv := reflect.ValueOf(val)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return true
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.UnsafePointer:
		return rv.IsNil()
	case reflect.Slice:
		if rv.IsNil() {
			return true
		}
		return rv.Type().Elem().Kind() == reflect.Uint8 && rv.Len() == 0 && !keepsEmptyValue(dst)
	case reflect.String:
		return rv.String() == "" && !keepsEmptyValue(dst)
	default:
		return false
	}
}

// keepsEmptyValue reports whether an empty string or byte slice is a meaningful value for dst.
func keepsEmptyValue(dst reflect.Value) bool {
	switch dst.Kind() {
	case reflect.String:
		return true
	case reflect.Slice, reflect.Array:
		return dst.Type().Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}

func isContainerKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Map, reflect.Slice, reflect.Array:
		return true
	default:
		return false
	}
}

// cloneNode copies JSON nodes so that scanned values do not alias config snapshots.
func cloneNode(src any) any {
	switch v := src.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = cloneNode(val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = cloneNode(v[i])
		}
		return out
	default:
		return v
	}
}

func jsonSource(src any) ([]byte, error) {
	switch v := src.(type) {
	case []byte:
		return v, nil
	case string:
		return json.Marshal(v)
	default:
		return json.Marshal(src)
	}
}

func textSource(src any) (string, error) {
	switch v := src.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	}

	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Array, reflect.Struct:
		return "", unsupportedSource(rv, "text unmarshaling")
	default:
		return xconv.String(src), nil
	}
}

// stringValue returns the value of a string source, including named string types.
func stringValue(src any) (string, bool) {
	if v, ok := src.(string); ok {
		return v, true
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", false
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.String {
		return rv.String(), true
	}

	return "", false
}

// bytesValue returns the value of a byte-slice source, including named byte-slice types. The
// returned slice may alias the source and must be copied when ownership is transferred.
func bytesValue(src any) ([]byte, bool) {
	if v, ok := src.([]byte); ok {
		return v, true
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		return rv.Bytes(), true
	}

	return nil, false
}

// setBytesValue assigns data to a byte slice or array. The caller must ensure that data does not
// alias data that must remain owned by the source.
func setBytesValue(dst reflect.Value, data []byte) error {
	if dst.Kind() == reflect.Slice {
		dst.SetBytes(data)
		return nil
	}

	count := min(len(data), dst.Len())
	for i := range count {
		dst.Index(i).SetUint(uint64(data[i]))
	}

	return nil
}

func parseInt(src any, bits int) (int64, error) {
	if s, ok := src.(string); ok {
		return strconv.ParseInt(strings.TrimSpace(s), 10, bits)
	}
	if b, ok := src.([]byte); ok {
		return strconv.ParseInt(strings.TrimSpace(string(b)), 10, bits)
	}

	var (
		n  int64
		rv = reflect.ValueOf(src)
	)

	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return strconv.ParseInt(strings.TrimSpace(rv.String()), 10, bits)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseInt(strings.TrimSpace(string(rv.Bytes())), 10, bits)
		}
		return 0, unsupportedSource(rv, "integer")
	case reflect.Bool:
		if rv.Bool() {
			n = 1
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := rv.Uint()
		if u > math.MaxInt64 {
			return 0, errors.NewError("scan: value overflows integer")
		}
		n = int64(u)
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if math.Trunc(f) != f {
			return 0, errors.NewError("scan: fractional value cannot be converted to integer")
		}
		if f < math.MinInt64 || f >= math.MaxInt64 {
			return 0, errors.NewError("scan: value overflows integer")
		}
		n = int64(f)
	default:
		return 0, unsupportedSource(rv, "integer")
	}

	if bits < 64 {
		minValue := -(int64(1) << (bits - 1))
		maxValue := int64(1)<<(bits-1) - 1
		if n < minValue || n > maxValue {
			return 0, errors.NewError("scan: value overflows int" + strconv.Itoa(bits))
		}
	}

	return n, nil
}

func parseUint(src any, bits int) (uint64, error) {
	if s, ok := src.(string); ok {
		return strconv.ParseUint(strings.TrimSpace(s), 10, bits)
	}
	if b, ok := src.([]byte); ok {
		return strconv.ParseUint(strings.TrimSpace(string(b)), 10, bits)
	}

	var (
		n  uint64
		rv = reflect.ValueOf(src)
	)

	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return strconv.ParseUint(strings.TrimSpace(rv.String()), 10, bits)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseUint(strings.TrimSpace(string(rv.Bytes())), 10, bits)
		}
		return 0, unsupportedSource(rv, "unsigned integer")
	case reflect.Bool:
		if rv.Bool() {
			n = 1
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := rv.Int()
		if i < 0 {
			return 0, errors.NewError("scan: negative value cannot be converted to unsigned integer")
		}
		n = uint64(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n = rv.Uint()
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if math.Trunc(f) != f || f < 0 {
			return 0, errors.NewError("scan: value cannot be converted to unsigned integer")
		}
		if f >= math.MaxUint64 {
			return 0, errors.NewError("scan: value overflows unsigned integer")
		}
		n = uint64(f)
	default:
		return 0, unsupportedSource(rv, "unsigned integer")
	}

	if bits < 64 {
		maxValue := uint64(1)<<bits - 1
		if n > maxValue {
			return 0, errors.NewError("scan: value overflows uint" + strconv.Itoa(bits))
		}
	}

	return n, nil
}

func parseFloat(src any, bits int) (float64, error) {
	if s, ok := src.(string); ok {
		return strconv.ParseFloat(strings.TrimSpace(s), bits)
	}
	if b, ok := src.([]byte); ok {
		return strconv.ParseFloat(strings.TrimSpace(string(b)), bits)
	}

	var (
		n  float64
		rv = reflect.ValueOf(src)
	)

	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return strconv.ParseFloat(strings.TrimSpace(rv.String()), bits)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseFloat(strings.TrimSpace(string(rv.Bytes())), bits)
		}
		return 0, unsupportedSource(rv, "float")
	case reflect.Bool:
		if rv.Bool() {
			n = 1
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n = float64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		n = rv.Float()
	default:
		return 0, unsupportedSource(rv, "float")
	}

	if bits == 32 && math.Abs(n) > math.MaxFloat32 {
		return 0, errors.NewError("scan: value overflows float32")
	}

	return n, nil
}

func parseBool(src any) (bool, error) {
	switch v := src.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(v))
	case []byte:
		return strconv.ParseBool(strings.TrimSpace(string(v)))
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return strconv.ParseBool(strings.TrimSpace(rv.String()))
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseBool(strings.TrimSpace(string(rv.Bytes())))
		}
		return false, unsupportedSource(rv, "bool")
	case reflect.Bool:
		return rv.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() != 0, nil
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0, nil
	default:
		return false, unsupportedSource(rv, "bool")
	}
}

func parseDuration(src any) (time.Duration, error) {
	if d, ok := src.(time.Duration); ok {
		return d, nil
	}
	if s, ok := stringValue(src); ok {
		return parseDurationText(s)
	}
	if data, ok := bytesValue(src); ok {
		return parseDurationText(string(data))
	}

	n, err := parseInt(src, 64)
	if err != nil {
		return 0, err
	}

	return time.Duration(n), nil
}

func parseDurationText(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, nil
	}

	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	if len(s) > 1 && s[len(s)-1] == 'd' {
		if f, err := strconv.ParseFloat(s[:len(s)-1], 64); err == nil {
			return time.Duration(f * float64(24*time.Hour)), nil
		}
	}

	return 0, errors.NewError("scan: invalid duration " + s)
}

func unsupportedSource(rv reflect.Value, target string) error {
	return errors.NewError("scan: unsupported source type " + rv.Type().String() + " for " + target)
}

type scanPathError struct {
	path string
	err  error
}

func (e *scanPathError) Error() string {
	return "scan value failed at " + e.path + ": " + e.err.Error()
}

func (e *scanPathError) Unwrap() error {
	return e.err
}

// wrapFieldError prepends a struct field or map key to a scan error. The path is only materialized
// when an error occurs.
func wrapFieldError(name string, err error) error {
	if err == nil || name == "" {
		return err
	}

	pathErr, ok := err.(*scanPathError)
	if !ok {
		return &scanPathError{path: name, err: err}
	}
	if pathErr.path != "" && pathErr.path[0] == '[' {
		return &scanPathError{path: name + pathErr.path, err: pathErr.err}
	}

	return &scanPathError{path: name + "." + pathErr.path, err: pathErr.err}
}

// wrapIndexError prepends a slice or array index to a scan error.
func wrapIndexError(index int, err error) error {
	if err == nil {
		return nil
	}

	step := "[" + strconv.Itoa(index) + "]"
	pathErr, ok := err.(*scanPathError)
	if !ok {
		return &scanPathError{path: step, err: err}
	}
	if pathErr.path != "" && pathErr.path[0] == '[' {
		return &scanPathError{path: step + pathErr.path, err: pathErr.err}
	}

	return &scanPathError{path: step + "." + pathErr.path, err: pathErr.err}
}
