package value

import (
	"bytes"
	"encoding"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xreflect"
)

type jsonUnmarshaler interface {
	UnmarshalJSON([]byte) error
}

var _durationType = reflect.TypeOf(time.Duration(0))

// scanValue scans src into dst, converting values between compatible types. The default tag of a
// struct field is applied when the field is absent or empty.
func scanValue(src any, dst reflect.Value, path string) error {
	if !dst.CanSet() {
		return nil
	}

	if dst.Kind() == reflect.Interface {
		return scanInterface(src, dst, path)
	}

	if !xreflect.IsNil(src) {
		rv := reflect.ValueOf(src)
		if rv.Type().AssignableTo(dst.Type()) && !isContainerKind(dst.Kind()) {
			dst.Set(rv)
			return nil
		}
	}

	if ok, err := scanWithUnmarshaler(src, dst); ok {
		return wrapScanError(path, err)
	}

	switch dst.Kind() {
	case reflect.Pointer:
		if xreflect.IsNil(src) {
			return nil
		}
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return scanValue(src, dst.Elem(), path)
	case reflect.Struct:
		return scanStruct(src, dst, path)
	case reflect.Map:
		return scanMap(src, dst, path)
	case reflect.Slice, reflect.Array:
		return scanSlice(src, dst, path)
	default:
		return scanScalar(src, dst, path)
	}
}

// scanInterface assigns src to an interface destination, copying JSON node containers so that the
// caller cannot mutate the config snapshot through the scanned value.
func scanInterface(src any, dst reflect.Value, path string) error {
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
		return wrapScanError(path, errors.NewError(msg))
	}

	dst.Set(rv)

	return nil
}

// scanWithUnmarshaler gives custom decoders precedence over the built-in conversion rules.
func scanWithUnmarshaler(src any, dst reflect.Value) (bool, error) {
	if !dst.CanAddr() {
		return false, nil
	}

	addr := dst.Addr().Interface()
	if u, ok := addr.(jsonUnmarshaler); ok {
		if xreflect.IsNil(src) {
			return true, nil
		}
		data, err := jsonSource(src)
		if err != nil {
			return true, err
		}
		return true, u.UnmarshalJSON(data)
	}

	if u, ok := addr.(encoding.TextUnmarshaler); ok {
		if xreflect.IsNil(src) {
			return true, nil
		}
		text, err := textSource(src)
		if err != nil {
			return true, err
		}
		return true, u.UnmarshalText([]byte(text))
	}

	return false, nil
}

// scanStruct decodes an object into a struct, matching fields by json tag or field name.
func scanStruct(src any, dst reflect.Value, path string) error {
	obj, err := scanObject(src)
	if err != nil {
		return wrapScanError(path, err)
	}

	for _, field := range structInfoOf(dst.Type()).fields {
		fv, err := fieldByIndex(dst, field.index, true)
		if err != nil {
			return wrapScanError(joinPath(path, field.name), err)
		}

		var val any
		if obj != nil {
			val, _ = lookupObject(obj, field.name)
		}
		if isMissingValue(val, fv) {
			if field.def == "" {
				val = nil
			} else {
				val = field.def
			}
		}

		if err = scanValue(val, fv, joinPath(path, field.name)); err != nil {
			return err
		}
	}

	return nil
}

// scanMap decodes an object into a map, converting keys and values to the declared map types.
func scanMap(src any, dst reflect.Value, path string) error {
	obj, err := scanObject(src)
	if err != nil {
		return wrapScanError(path, err)
	}
	if obj == nil {
		return nil
	}

	out := reflect.MakeMapWithSize(dst.Type(), len(obj))
	for key, val := range obj {
		kv := reflect.New(dst.Type().Key()).Elem()
		if err = scanValue(key, kv, joinPath(path, key)); err != nil {
			return err
		}

		vv := reflect.New(dst.Type().Elem()).Elem()
		if err = scanValue(val, vv, joinPath(path, xconv.String(key))); err != nil {
			return err
		}

		out.SetMapIndex(kv, vv)
	}
	dst.Set(out)

	return nil
}

// scanSlice decodes a slice or array from another slice, an array or a delimited string.
func scanSlice(src any, dst reflect.Value, path string) error {
	if xreflect.IsNil(src) {
		return nil
	}

	if dst.Type().Elem().Kind() == reflect.Uint8 {
		if data, ok := bytesSource(src); ok {
			return setBytesValue(dst, data)
		}
	}

	items, err := scanItems(src)
	if err != nil {
		return wrapScanError(path, err)
	}

	if dst.Kind() == reflect.Array {
		count := min(len(items), dst.Len())
		for i := range count {
			if err = scanValue(items[i], dst.Index(i), joinIndexPath(path, i)); err != nil {
				return err
			}
		}
		return nil
	}

	out := reflect.MakeSlice(dst.Type(), len(items), len(items))
	for i := range items {
		if err = scanValue(items[i], out.Index(i), joinIndexPath(path, i)); err != nil {
			return err
		}
	}
	dst.Set(out)

	return nil
}

// scanScalar converts a scalar source into a scalar destination.
func scanScalar(src any, dst reflect.Value, path string) error {
	if xreflect.IsNil(src) {
		return nil
	}

	if dst.Type() == _durationType {
		d, err := parseDuration(src)
		if err != nil {
			return wrapScanError(path, err)
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
			return wrapScanError(path, err)
		}
		dst.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := parseInt(src, dst.Type().Bits())
		if err != nil {
			return wrapScanError(path, err)
		}
		dst.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := parseUint(src, dst.Type().Bits())
		if err != nil {
			return wrapScanError(path, err)
		}
		dst.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := parseFloat(src, dst.Type().Bits())
		if err != nil {
			return wrapScanError(path, err)
		}
		dst.SetFloat(n)
	default:
		err := errors.NewError("scan: unsupported destination type " + dst.Type().String())
		return wrapScanError(path, err)
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

func lookupObject(obj map[string]any, name string) (any, bool) {
	if val, ok := obj[name]; ok {
		return val, true
	}

	for key, val := range obj {
		if strings.EqualFold(key, name) {
			return val, true
		}
	}

	return nil, false
}

// isMissingValue reports whether a struct field should fall back to its default. An empty string
// or byte slice counts as missing for every destination except string and byte slices, where the
// empty value itself is meaningful.
func isMissingValue(val any, dst reflect.Value) bool {
	if xreflect.IsNil(val) {
		return true
	}
	if !isBlankValue(val) {
		return false
	}

	switch dst.Kind() {
	case reflect.String:
		return false
	case reflect.Slice, reflect.Array:
		return dst.Type().Elem().Kind() != reflect.Uint8
	default:
		return true
	}
}

// isBlankValue reports whether val is an empty string or an empty byte slice, including named
// types.
func isBlankValue(val any) bool {
	switch v := val.(type) {
	case string:
		return v == ""
	case []byte:
		return len(v) == 0
	}

	rv := reflect.ValueOf(val)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return rv.String() == ""
	case reflect.Slice:
		return rv.Type().Elem().Kind() == reflect.Uint8 && rv.Len() == 0
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

// bytesSource returns the bytes of a string or byte-slice source, including named types.
func bytesSource(src any) ([]byte, bool) {
	switch v := src.(type) {
	case string:
		return []byte(v), true
	case []byte:
		return v, true
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return []byte(rv.String()), true
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return rv.Bytes(), true
		}
	}

	return nil, false
}

func setBytesValue(dst reflect.Value, data []byte) error {
	if dst.Kind() == reflect.Slice {
		dst.SetBytes(bytes.Clone(data))
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
	if data, ok := bytesSource(src); ok {
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

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func joinIndexPath(path string, index int) string {
	return path + "[" + strconv.Itoa(index) + "]"
}

func wrapScanError(path string, err error) error {
	if err == nil || path == "" {
		return err
	}

	return errors.NewError("scan value failed at "+path, err)
}
