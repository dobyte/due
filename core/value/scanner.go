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

const maxMaskedFields = 64

var (
	unmarshalerKinds    sync.Map // map[reflect.Type]unmarshalerKind
	jsonUnmarshalerType = reflect.TypeOf((*jsonUnmarshaler)(nil)).Elem()
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	anyMapType          = reflect.TypeOf(map[string]any(nil))
	anySliceType        = reflect.TypeOf([]any(nil))
	stringStringMapType = reflect.TypeOf(map[string]string(nil))
	stringIntMapType    = reflect.TypeOf(map[string]int(nil))
	stringInt64MapType  = reflect.TypeOf(map[string]int64(nil))
	stringBoolMapType   = reflect.TypeOf(map[string]bool(nil))
	stringSliceType     = reflect.TypeOf([]string(nil))
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
	return scanValueKind(src, dst, unmarshalerKindOf(dst.Type()), false)
}

// scanValueKind scans src into dst using a precomputed unmarshaler kind. fresh reports whether src
// was decoded by this scan, in which case container nodes can be adopted without cloning.
func scanValueKind(src any, dst reflect.Value, kind unmarshalerKind, fresh bool) error {
	if !dst.CanSet() {
		return nil
	}

	if dst.Kind() == reflect.Interface {
		return scanInterface(src, dst, fresh)
	}

	if src != nil {
		rv := reflect.ValueOf(src)
		if !isNilValue(rv) {
			rt := rv.Type()
			if !isContainerKind(dst.Kind()) && (rt == dst.Type() || rt.AssignableTo(dst.Type())) {
				dst.Set(rv)
				return nil
			}
		}
	}

	if kind != unmarshalerNone {
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
		return scanValueKind(src, dst.Elem(), unmarshalerKindOf(dst.Elem().Type()), fresh)
	case reflect.Struct:
		if dst.CanAddr() {
			if decoded, err := scanJSONStruct(src, dst); decoded {
				return err
			}
		}
		return scanStruct(src, dst, fresh)
	case reflect.Map:
		return scanMap(src, dst, fresh)
	case reflect.Slice, reflect.Array:
		return scanSlice(src, dst, fresh)
	default:
		return scanScalar(src, dst)
	}
}

type jsonInput struct {
	data   []byte
	text   string
	isText bool
}

func (in jsonInput) unmarshal(v any) error {
	if in.isText {
		return json.UnmarshalString(in.text, v)
	}
	return json.Unmarshal(in.data, v)
}

func (in jsonInput) isObject() bool {
	if in.isText {
		return isJSONObjectString(in.text)
	}
	return isJSONObjectBytes(in.data)
}

func (in jsonInput) isArray() bool {
	if in.isText {
		return isJSONArrayString(in.text)
	}
	return isJSONArrayBytes(in.data)
}

func jsonInputOf(src any) (jsonInput, bool) {
	if s, ok := stringValue(src); ok {
		return jsonInput{text: s, isText: true}, true
	}
	if data, ok := bytesValue(src); ok {
		return jsonInput{data: data}, true
	}

	return jsonInput{}, false
}

// scanJSONStruct decodes a JSON object directly into a struct. Structs without defaults use one
// direct decode; defaulted structs are scanned field by field from raw JSON, which keeps nested
// defaults and cross-type conversions working without an intermediate map[string]any.
func scanJSONStruct(src any, dst reflect.Value) (bool, error) {
	in, ok := jsonInputOf(src)
	if !ok || !in.isObject() {
		return false, nil
	}

	info := structInfoOf(dst.Type())

	if !info.hasDefaults {
		if in.unmarshal(dst.Addr().Interface()) == nil {
			return true, nil
		}
		return true, scanJSONStructRaw(in, dst, info)
	}

	if !info.hasNestedDefault && len(info.fields) <= maxMaskedFields {
		if scanJSONStructDefaults(in, dst, info) {
			return true, nil
		}
	}

	return true, scanJSONStructRaw(in, dst, info)
}

func scanJSONStructDefaults(in jsonInput, dst reflect.Value, info *structInfo) bool {
	if in.unmarshal(dst.Addr().Interface()) != nil {
		return false
	}

	if in.isText {
		return scanStructPresence(in.text, dst, info)
	}

	return scanStructPresence(in.data, dst, info)
}

// scanStructPresence records the fields present in a raw JSON object and applies the defaults of
// the remaining fields. A later duplicate key overrides an earlier one.
func scanStructPresence[T jsonText](data T, dst reflect.Value, info *structInfo) bool {
	var matched uint64

	err := walkJSONObject(data, func(key string, value T) error {
		index, ok := info.fieldIndex(key)
		if !ok {
			return nil
		}

		field := &info.fields[index]
		bit := uint64(1) << index
		if field.def != "" && isMissingJSONValue(value, field.typ) {
			matched &^= bit
		} else {
			matched |= bit
		}

		return nil
	})
	if err != nil {
		return false
	}

	for i := range info.fields {
		field := &info.fields[i]
		if field.def == "" || matched&(uint64(1)<<i) != 0 {
			continue
		}
		if err := scanField(nil, dst, field, false); err != nil {
			return false
		}
	}

	return true
}

// scanJSONStructRaw scans a JSON object into a struct without building an intermediate map,
// keeping nested defaults and cross-type conversions working.
func scanJSONStructRaw(in jsonInput, dst reflect.Value, info *structInfo) error {
	if in.isText {
		return scanRawObject(in.text, dst, info)
	}

	return scanRawObject(in.data, dst, info)
}

// scanRawObject scans a raw JSON object into a struct and applies defaults to absent fields.
func scanRawObject[T jsonText](data T, dst reflect.Value, info *structInfo) error {
	if len(info.fields) > maxMaskedFields {
		return scanRawObjectWide(data, dst, info)
	}

	var matched uint64

	if err := walkJSONObject(data, func(key string, value T) error {
		index, ok := info.fieldIndex(key)
		if !ok {
			return nil
		}

		matched |= uint64(1) << index

		return scanRawField(value, dst, &info.fields[index])
	}); err != nil {
		return err
	}

	for i := range info.fields {
		if matched&(uint64(1)<<i) != 0 {
			continue
		}
		if err := scanField(nil, dst, &info.fields[i], false); err != nil {
			return err
		}
	}

	return nil
}

// scanRawObjectWide is the raw object scanner for structs with more than 64 fields.
func scanRawObjectWide[T jsonText](data T, dst reflect.Value, info *structInfo) error {
	matched := make([]bool, len(info.fields))

	if err := walkJSONObject(data, func(key string, value T) error {
		index, ok := info.fieldIndex(key)
		if !ok {
			return nil
		}

		matched[index] = true

		return scanRawField(value, dst, &info.fields[index])
	}); err != nil {
		return err
	}

	for i := range info.fields {
		if matched[i] {
			continue
		}
		if err := scanField(nil, dst, &info.fields[i], false); err != nil {
			return err
		}
	}

	return nil
}

// scanRawField scans one raw JSON value into a struct field, falling back to the field default
// when the value counts as absent.
func scanRawField[T jsonText](raw T, dst reflect.Value, field *fieldInfo) error {
	if isMissingJSONValue(raw, field.typ) {
		return scanField(nil, dst, field, false)
	}

	fv, err := fieldByIndex(dst, field.index, true)
	if err != nil {
		return wrapFieldError(field.name, err)
	}

	if err := scanRawValue(raw, fv, field.unmarshaler); err != nil {
		return wrapFieldError(field.name, err)
	}

	return nil
}

// scanRawValue decodes one raw JSON value into dst. Types that carry default tags are scanned
// recursively; other values use a direct decode with a type-conversion fallback.
func scanRawValue[T jsonText](raw T, dst reflect.Value, kind unmarshalerKind) error {
	data := trimJSONSpace(raw)
	if len(data) == 0 || isJSONNull(data) {
		return nil
	}

	if dst.Kind() == reflect.Interface {
		var val any
		if err := json.Unmarshal([]byte(data), &val); err != nil {
			return err
		}
		return scanInterface(val, dst, true)
	}

	switch kind {
	case unmarshalerJSON:
		return dst.Addr().Interface().(jsonUnmarshaler).UnmarshalJSON([]byte(data))
	case unmarshalerText:
		var val any
		if err := json.Unmarshal([]byte(data), &val); err != nil {
			return err
		}
		return scanValueKind(val, dst, kind, true)
	}

	switch dst.Kind() {
	case reflect.Pointer:
		if elem := dst.Type().Elem(); typeNeedsDefaults(elem) {
			if dst.IsNil() {
				dst.Set(reflect.New(elem))
			}
			return scanRawValue(data, dst.Elem(), unmarshalerKindOf(elem))
		}
	case reflect.Struct:
		if data[0] == '{' {
			info := structInfoOf(dst.Type())
			if info.hasDefaults {
				return scanRawObject(data, dst, info)
			}
			if json.Unmarshal([]byte(data), dst.Addr().Interface()) == nil {
				return nil
			}
			return scanRawObject(data, dst, info)
		}
	case reflect.Map:
		if data[0] == '{' && typeNeedsDefaults(dst.Type().Elem()) {
			return scanRawMap(data, dst)
		}
	case reflect.Slice, reflect.Array:
		if data[0] == '[' && typeNeedsDefaults(dst.Type().Elem()) {
			return scanRawSlice(data, dst)
		}
	}

	if handled, err := scanRawPrimitive(data, dst); handled {
		return err
	}

	if val, ok := rawScalarValue(data); ok {
		return scanValueKind(val, dst, kind, true)
	}

	if err := json.Unmarshal([]byte(data), dst.Addr().Interface()); err == nil {
		return nil
	}

	switch dst.Kind() {
	case reflect.Pointer:
		elem := dst.Type().Elem()
		if dst.IsNil() {
			dst.Set(reflect.New(elem))
		}
		return scanRawValue(data, dst.Elem(), unmarshalerKindOf(elem))
	case reflect.Map:
		if data[0] == '{' {
			return scanRawMap(data, dst)
		}
	case reflect.Slice, reflect.Array:
		if data[0] == '[' {
			return scanRawSlice(data, dst)
		}
	}

	var val any
	if err := json.Unmarshal([]byte(data), &val); err != nil {
		return err
	}

	return scanValueKind(val, dst, kind, true)
}

// scanRawPrimitive converts a JSON scalar into a scalar destination of the matching kind.
func scanRawPrimitive[T jsonText](data T, dst reflect.Value) (bool, error) {
	switch dst.Kind() {
	case reflect.String:
		if data[0] != '"' {
			return false, nil
		}
		value, err := unquoteJSONString(data)
		if err != nil {
			return true, err
		}
		dst.SetString(value)
		return true, nil
	case reflect.Bool:
		switch {
		case len(data) == 4 && data[0] == 't':
			dst.SetBool(true)
			return true, nil
		case len(data) == 5 && data[0] == 'f':
			dst.SetBool(false)
			return true, nil
		default:
			return false, nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := parseJSONInt(data)
		if !ok {
			return false, nil
		}
		if dst.OverflowInt(n) {
			return true, errors.NewError("scan: value overflows " + dst.Type().String())
		}
		dst.SetInt(n)
		return true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, ok := parseJSONUint(data)
		if !ok {
			return false, nil
		}
		if dst.OverflowUint(n) {
			return true, errors.NewError("scan: value overflows " + dst.Type().String())
		}
		dst.SetUint(n)
		return true, nil
	case reflect.Float32, reflect.Float64:
		f, ok := parseJSONFloat(data)
		if !ok {
			return false, nil
		}
		if dst.OverflowFloat(f) {
			return true, errors.NewError("scan: value overflows " + dst.Type().String())
		}
		dst.SetFloat(f)
		return true, nil
	default:
		return false, nil
	}
}

// rawScalarValue decodes a JSON scalar into a Go primitive for cross-type conversion.
func rawScalarValue[T jsonText](data T) (any, bool) {
	switch data[0] {
	case '"':
		value, err := unquoteJSONString(data)
		if err != nil {
			return nil, false
		}
		return value, true
	case 't':
		return true, len(data) == 4
	case 'f':
		return false, len(data) == 5
	case 'n':
		return nil, isJSONNull(data)
	default:
		if n, ok := parseJSONInt(data); ok {
			return n, true
		}
		if f, err := strconv.ParseFloat(string(data), 64); err == nil {
			return f, true
		}
		return nil, false
	}
}

// scanRawMap scans a raw JSON object into a map without an intermediate object.
func scanRawMap[T jsonText](data T, dst reflect.Value) error {
	switch dst.Type() {
	case stringStringMapType:
		return scanRawStringMap(data, dst)
	case stringIntMapType:
		return scanRawIntMap(data, dst)
	case stringInt64MapType:
		return scanRawInt64Map(data, dst)
	case stringBoolMapType:
		return scanRawBoolMap(data, dst)
	}

	var (
		keyType  = dst.Type().Key()
		keyKind  = unmarshalerKindOf(keyType)
		elemKind = unmarshalerKindOf(dst.Type().Elem())
		out      = reflect.MakeMap(dst.Type())
	)

	if err := walkJSONObject(data, func(key string, value T) error {
		var kv reflect.Value
		if keyType.Kind() == reflect.String && keyKind == unmarshalerNone {
			kv = reflect.ValueOf(key).Convert(keyType)
		} else {
			kv = reflect.New(keyType).Elem()
			if err := scanValueKind(key, kv, keyKind, false); err != nil {
				return wrapFieldError(key, err)
			}
		}

		vv := reflect.New(dst.Type().Elem()).Elem()
		if err := scanRawValue(value, vv, elemKind); err != nil {
			return wrapFieldError(key, err)
		}

		out.SetMapIndex(kv, vv)

		return nil
	}); err != nil {
		return err
	}
	dst.Set(out)

	return nil
}

// scanRawStringMap scans a raw JSON object into a map[string]string.
func scanRawStringMap[T jsonText](data T, dst reflect.Value) error {
	out := make(map[string]string)

	if err := walkJSONObject(data, func(key string, value T) error {
		val, err := rawValueOf(value)
		if err != nil {
			return wrapFieldError(key, err)
		}
		out[key] = xconv.String(val)

		return nil
	}); err != nil {
		return err
	}
	dst.Set(reflect.ValueOf(out))

	return nil
}

// scanRawIntMap scans a raw JSON object into a map[string]int.
func scanRawIntMap[T jsonText](data T, dst reflect.Value) error {
	out := make(map[string]int)

	if err := walkJSONObject(data, func(key string, value T) error {
		val, err := rawValueOf(value)
		if err != nil {
			return wrapFieldError(key, err)
		}
		if val == nil {
			out[key] = 0
			return nil
		}
		n, err := parseInt(val, strconv.IntSize)
		if err != nil {
			return wrapFieldError(key, err)
		}
		out[key] = int(n)

		return nil
	}); err != nil {
		return err
	}
	dst.Set(reflect.ValueOf(out))

	return nil
}

// scanRawInt64Map scans a raw JSON object into a map[string]int64.
func scanRawInt64Map[T jsonText](data T, dst reflect.Value) error {
	out := make(map[string]int64)

	if err := walkJSONObject(data, func(key string, value T) error {
		val, err := rawValueOf(value)
		if err != nil {
			return wrapFieldError(key, err)
		}
		if val == nil {
			out[key] = 0
			return nil
		}
		n, err := parseInt(val, 64)
		if err != nil {
			return wrapFieldError(key, err)
		}
		out[key] = n

		return nil
	}); err != nil {
		return err
	}
	dst.Set(reflect.ValueOf(out))

	return nil
}

// scanRawBoolMap scans a raw JSON object into a map[string]bool.
func scanRawBoolMap[T jsonText](data T, dst reflect.Value) error {
	out := make(map[string]bool)

	if err := walkJSONObject(data, func(key string, value T) error {
		val, err := rawValueOf(value)
		if err != nil {
			return wrapFieldError(key, err)
		}
		if val == nil {
			out[key] = false
			return nil
		}
		b, err := parseBool(val)
		if err != nil {
			return wrapFieldError(key, err)
		}
		out[key] = b

		return nil
	}); err != nil {
		return err
	}
	dst.Set(reflect.ValueOf(out))

	return nil
}

// rawValueOf decodes a raw JSON value into its scalar form, or a generic node for composites.
func rawValueOf[T jsonText](raw T) (any, error) {
	value := trimJSONSpace(raw)
	if len(value) == 0 || isJSONNull(value) {
		return nil, nil
	}
	if val, ok := rawScalarValue(value); ok {
		return val, nil
	}

	var val any
	if err := json.Unmarshal([]byte(value), &val); err != nil {
		return nil, err
	}

	return val, nil
}

// scanRawSlice scans a raw JSON array into a slice or array without an intermediate slice.
func scanRawSlice[T jsonText](data T, dst reflect.Value) error {
	count, err := countJSONArray(data)
	if err != nil {
		return err
	}

	kind := unmarshalerKindOf(dst.Type().Elem())

	if dst.Kind() == reflect.Array {
		index := 0
		limit := min(count, dst.Len())
		return walkJSONArray(data, func(_ int, value T) error {
			if index >= limit {
				return nil
			}
			if err := scanRawValue(value, dst.Index(index), kind); err != nil {
				return wrapIndexError(index, err)
			}
			index++
			return nil
		})
	}

	out := reflect.MakeSlice(dst.Type(), count, count)
	index := 0
	if err = walkJSONArray(data, func(_ int, value T) error {
		if err := scanRawValue(value, out.Index(index), kind); err != nil {
			return wrapIndexError(index, err)
		}
		index++
		return nil
	}); err != nil {
		return err
	}
	dst.Set(out)

	return nil
}

// typeNeedsDefaults reports whether t or any nested type declares a default tag, in which case a
// plain JSON decode would silently drop those defaults.
func typeNeedsDefaults(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		return structInfoOf(t).hasDefaults
	case reflect.Slice, reflect.Array, reflect.Map:
		return typeNeedsDefaults(t.Elem())
	default:
		return false
	}
}

func isMissingJSONValue[T jsonText](raw T, t reflect.Type) bool {
	data := trimJSONSpace(raw)
	if isJSONNull(data) {
		return true
	}

	return len(data) == 2 && data[0] == '"' && data[1] == '"' && !typeKeepsEmpty(t)
}

// jsonText is a JSON document or a slice of one, so string sources need no byte copy.
type jsonText interface {
	~string | ~[]byte
}

const maxJSONDepth = 10000

// rawScan walks a JSON document without building intermediate values.
type rawScan[T jsonText] struct {
	data  T
	pos   int
	depth int
}

// walkJSONObject visits every member of a JSON object in document order.
func walkJSONObject[T jsonText](data T, fn func(key string, value T) error) error {
	s := rawScan[T]{data: data}
	s.skipSpace()
	if s.pos >= len(data) || data[s.pos] != '{' {
		return errors.NewError("scan: source is not a JSON object")
	}
	s.pos++
	s.skipSpace()
	if s.pos < len(data) && data[s.pos] == '}' {
		s.pos++
		return s.end()
	}

	for {
		s.skipSpace()
		if s.pos >= len(data) || data[s.pos] != '"' {
			return errors.NewError("scan: invalid JSON object")
		}
		keyStart := s.pos
		if err := s.skipString(); err != nil {
			return err
		}
		key, err := unquoteJSONString(data[keyStart:s.pos])
		if err != nil {
			return err
		}

		s.skipSpace()
		if s.pos >= len(data) || data[s.pos] != ':' {
			return errors.NewError("scan: invalid JSON object")
		}
		s.pos++
		s.skipSpace()
		valueStart := s.pos
		if err := s.skipValue(); err != nil {
			return err
		}
		if err := fn(key, data[valueStart:s.pos]); err != nil {
			return err
		}

		s.skipSpace()
		if s.pos >= len(data) {
			return errors.NewError("scan: invalid JSON object")
		}
		switch data[s.pos] {
		case ',':
			s.pos++
		case '}':
			s.pos++
			return s.end()
		default:
			return errors.NewError("scan: invalid JSON object")
		}
	}
}

// walkJSONArray visits every element of a JSON array in document order.
func walkJSONArray[T jsonText](data T, fn func(index int, value T) error) error {
	s := rawScan[T]{data: data}
	s.skipSpace()
	if s.pos >= len(data) || data[s.pos] != '[' {
		return errors.NewError("scan: source is not a JSON array")
	}
	s.pos++
	s.skipSpace()
	if s.pos < len(data) && data[s.pos] == ']' {
		s.pos++
		return s.end()
	}

	for index := 0; ; index++ {
		s.skipSpace()
		valueStart := s.pos
		if err := s.skipValue(); err != nil {
			return err
		}
		if err := fn(index, data[valueStart:s.pos]); err != nil {
			return err
		}

		s.skipSpace()
		if s.pos >= len(data) {
			return errors.NewError("scan: invalid JSON array")
		}
		switch data[s.pos] {
		case ',':
			s.pos++
		case ']':
			s.pos++
			return s.end()
		default:
			return errors.NewError("scan: invalid JSON array")
		}
	}
}

// countJSONArray returns the number of elements of a JSON array.
func countJSONArray[T jsonText](data T) (int, error) {
	count := 0
	err := walkJSONArray(data, func(int, T) error {
		count++
		return nil
	})

	return count, err
}

func (s *rawScan[T]) skipSpace() {
	for s.pos < len(s.data) && isJSONSpace(s.data[s.pos]) {
		s.pos++
	}
}

// end reports an error when any non-space content follows the scanned document.
func (s *rawScan[T]) end() error {
	s.skipSpace()
	if s.pos != len(s.data) {
		return errors.NewError("scan: invalid JSON document")
	}

	return nil
}

func (s *rawScan[T]) skipValue() error {
	s.skipSpace()
	if s.pos >= len(s.data) {
		return errors.NewError("scan: unexpected end of JSON document")
	}

	switch s.data[s.pos] {
	case '"':
		return s.skipString()
	case '{':
		return s.skipObject()
	case '[':
		return s.skipArray()
	case 't':
		return s.skipLiteral("true")
	case 'f':
		return s.skipLiteral("false")
	case 'n':
		return s.skipLiteral("null")
	default:
		return s.skipNumber()
	}
}

func (s *rawScan[T]) skipLiteral(literal string) error {
	if s.pos+len(literal) > len(s.data) {
		return errors.NewError("scan: invalid JSON literal")
	}
	for i := range len(literal) {
		if s.data[s.pos+i] != literal[i] {
			return errors.NewError("scan: invalid JSON literal")
		}
	}
	s.pos += len(literal)

	return nil
}

func (s *rawScan[T]) skipNumber() error {
	start := s.pos

	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '+', '.', 'e', 'E':
			s.pos++
		default:
			if s.pos == start {
				return errors.NewError("scan: invalid JSON value")
			}
			return nil
		}
	}

	if s.pos == start {
		return errors.NewError("scan: invalid JSON value")
	}

	return nil
}

func (s *rawScan[T]) skipString() error {
	if s.pos >= len(s.data) || s.data[s.pos] != '"' {
		return errors.NewError("scan: invalid JSON string")
	}

	s.pos++
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case '\\':
			s.pos += 2
		case '"':
			s.pos++
			return nil
		default:
			s.pos++
		}
	}

	return errors.NewError("scan: unterminated JSON string")
}

func (s *rawScan[T]) skipObject() error {
	if s.depth >= maxJSONDepth {
		return errors.NewError("scan: JSON nesting too deep")
	}
	s.depth++
	err := s.skipObjectBody()
	s.depth--

	return err
}

func (s *rawScan[T]) skipObjectBody() error {
	s.pos++
	s.skipSpace()
	if s.pos < len(s.data) && s.data[s.pos] == '}' {
		s.pos++
		return nil
	}

	for {
		s.skipSpace()
		if err := s.skipString(); err != nil {
			return err
		}
		s.skipSpace()
		if s.pos >= len(s.data) || s.data[s.pos] != ':' {
			return errors.NewError("scan: invalid JSON object")
		}
		s.pos++
		if err := s.skipValue(); err != nil {
			return err
		}
		s.skipSpace()
		if s.pos >= len(s.data) {
			return errors.NewError("scan: invalid JSON object")
		}
		switch s.data[s.pos] {
		case ',':
			s.pos++
		case '}':
			s.pos++
			return nil
		default:
			return errors.NewError("scan: invalid JSON object")
		}
	}
}

func (s *rawScan[T]) skipArray() error {
	if s.depth >= maxJSONDepth {
		return errors.NewError("scan: JSON nesting too deep")
	}
	s.depth++
	err := s.skipArrayBody()
	s.depth--

	return err
}

func (s *rawScan[T]) skipArrayBody() error {
	s.pos++
	s.skipSpace()
	if s.pos < len(s.data) && s.data[s.pos] == ']' {
		s.pos++
		return nil
	}

	for {
		if err := s.skipValue(); err != nil {
			return err
		}
		s.skipSpace()
		if s.pos >= len(s.data) {
			return errors.NewError("scan: invalid JSON array")
		}
		switch s.data[s.pos] {
		case ',':
			s.pos++
		case ']':
			s.pos++
			return nil
		default:
			return errors.NewError("scan: invalid JSON array")
		}
	}
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isJSONNull[T jsonText](data T) bool {
	return len(data) == 4 && data[0] == 'n' && data[1] == 'u' && data[2] == 'l' && data[3] == 'l'
}

func trimJSONSpace[T jsonText](data T) T {
	start, end := 0, len(data)
	for start < end && isJSONSpace(data[start]) {
		start++
	}
	for end > start && isJSONSpace(data[end-1]) {
		end--
	}

	return data[start:end]
}

// unquoteJSONString decodes a quoted JSON string, avoiding an extra copy when no escapes exist.
func unquoteJSONString[T jsonText](data T) (string, error) {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return "", errors.NewError("scan: invalid JSON string")
	}

	body := data[1 : len(data)-1]
	for i := range len(body) {
		if body[i] != '\\' {
			continue
		}

		var value string
		if err := json.Unmarshal([]byte(data), &value); err != nil {
			return "", err
		}
		return value, nil
	}

	return string(body), nil
}

func parseJSONInt[T jsonText](data T) (int64, bool) {
	if len(data) == 0 {
		return 0, false
	}

	var (
		i   = 0
		neg = data[0] == '-'
		n   uint64
	)
	if neg {
		i = 1
	}
	if i == len(data) {
		return 0, false
	}

	for ; i < len(data); i++ {
		c := data[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint64(c - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}

	if neg {
		if n > uint64(math.MaxInt64)+1 {
			return 0, false
		}
		if n == uint64(math.MaxInt64)+1 {
			return math.MinInt64, true
		}
		return -int64(n), true
	}
	if n > math.MaxInt64 {
		return 0, false
	}

	return int64(n), true
}

func parseJSONUint[T jsonText](data T) (uint64, bool) {
	if len(data) == 0 || data[0] == '-' {
		return 0, false
	}

	var n uint64
	for i := range len(data) {
		c := data[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint64(c - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}

	return n, true
}

func parseJSONFloat[T jsonText](data T) (float64, bool) {
	f, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return 0, false
	}

	return f, true
}

func typeKeepsEmpty(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String:
		return true
	case reflect.Slice, reflect.Array:
		return t.Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}

func isJSONObjectString(s string) bool {
	s = trimSpaceFast(s)

	return s != "" && s[0] == '{'
}

func isJSONObjectBytes(data []byte) bool {
	data = bytes.TrimSpace(data)

	return len(data) != 0 && data[0] == '{'
}

func isJSONArrayString(s string) bool {
	s = trimSpaceFast(s)

	return s != "" && s[0] == '['
}

func isJSONArrayBytes(data []byte) bool {
	data = bytes.TrimSpace(data)

	return len(data) != 0 && data[0] == '['
}

// isNilValue reports whether rv holds a nil chan, func, map, pointer, slice or unsafe pointer.
func isNilValue(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}

// scanInterface assigns src to an interface destination, copying JSON node containers so that the
// caller cannot mutate the config snapshot through the scanned value.
func scanInterface(src any, dst reflect.Value, fresh bool) error {
	if xreflect.IsNil(src) {
		return nil
	}

	rv := reflect.ValueOf(src)
	if dst.Type().NumMethod() == 0 {
		if fresh {
			dst.Set(rv)
		} else {
			dst.Set(reflect.ValueOf(cloneNode(src)))
		}
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
func scanStruct(src any, dst reflect.Value, fresh bool) error {
	obj, decoded, err := scanObjectFresh(src)
	if err != nil {
		return err
	}

	fresh = fresh || decoded
	info := structInfoOf(dst.Type())

	if obj == nil {
		return scanStructDefaults(dst, info, fresh)
	}
	if len(info.fields) > maxMaskedFields {
		return scanStructWide(obj, dst, info, fresh)
	}

	return scanStructByKey(obj, dst, info, fresh)
}

// scanStructDefaults applies the defaults of an absent object.
func scanStructDefaults(dst reflect.Value, info *structInfo, fresh bool) error {
	for i := range info.fields {
		if err := scanField(nil, dst, &info.fields[i], fresh); err != nil {
			return err
		}
	}

	return nil
}

// scanStructByKey matches each source key once and records the consumed field in a bitmask, so a
// small object is traversed only once instead of once per field.
func scanStructByKey(obj map[string]any, dst reflect.Value, info *structInfo, fresh bool) error {
	var matched uint64

	for key, val := range obj {
		index, ok := info.fieldIndex(key)
		if !ok {
			continue
		}

		bit := uint64(1) << index
		if matched&bit != 0 {
			continue
		}
		matched |= bit

		if err := scanField(val, dst, &info.fields[index], fresh); err != nil {
			return err
		}
	}

	for i := range info.fields {
		if matched&(uint64(1)<<i) != 0 {
			continue
		}
		if err := scanField(nil, dst, &info.fields[i], fresh); err != nil {
			return err
		}
	}

	return nil
}

// scanStructWide is the fallback for structs with more than 64 fields.
func scanStructWide(obj map[string]any, dst reflect.Value, info *structInfo, fresh bool) error {
	matched := make([]bool, len(info.fields))

	for key, val := range obj {
		index, ok := info.fieldIndex(key)
		if !ok || matched[index] {
			continue
		}
		matched[index] = true

		if err := scanField(val, dst, &info.fields[index], fresh); err != nil {
			return err
		}
	}

	for i := range info.fields {
		if matched[i] {
			continue
		}
		if err := scanField(nil, dst, &info.fields[i], fresh); err != nil {
			return err
		}
	}

	return nil
}

// scanField scans a single matched or missing field.
func scanField(val any, dst reflect.Value, field *fieldInfo, fresh bool) error {
	fv, err := fieldByIndex(dst, field.index, true)
	if err != nil {
		return wrapFieldError(field.name, err)
	}

	if isMissingValue(val, fv) {
		if field.def == "" {
			val = nil
		} else {
			val = field.defValue
			fresh = false // cached defaults are shared between scans
		}
	}

	if err = scanValueKind(val, fv, field.unmarshaler, fresh); err != nil {
		return wrapFieldError(field.name, err)
	}

	return nil
}

// scanMap decodes an object into a map, converting keys and values to the declared map types.
func scanMap(src any, dst reflect.Value, fresh bool) error {
	if in, ok := jsonInputOf(src); ok && in.isObject() && dst.Type() != anyMapType {
		if in.isText {
			return scanRawMap(in.text, dst)
		}
		return scanRawMap(in.data, dst)
	}

	obj, decoded, err := scanObjectFresh(src)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil
	}

	fresh = fresh || decoded

	if dst.Type() == anyMapType {
		if fresh {
			dst.Set(reflect.ValueOf(obj))
			return nil
		}
		out := make(map[string]any, len(obj))
		for key, val := range obj {
			out[key] = cloneNode(val)
		}
		dst.Set(reflect.ValueOf(out))
		return nil
	}

	if dst.Type() == stringStringMapType {
		out := make(map[string]string, len(obj))
		for key, val := range obj {
			out[key] = xconv.String(val)
		}
		dst.Set(reflect.ValueOf(out))
		return nil
	}

	if dst.Type() == stringIntMapType {
		out := make(map[string]int, len(obj))
		for key, val := range obj {
			if val == nil {
				out[key] = 0
				continue
			}
			n, err := parseInt(val, strconv.IntSize)
			if err != nil {
				return wrapFieldError(key, err)
			}
			out[key] = int(n)
		}
		dst.Set(reflect.ValueOf(out))
		return nil
	}

	if dst.Type() == stringInt64MapType {
		out := make(map[string]int64, len(obj))
		for key, val := range obj {
			if val == nil {
				out[key] = 0
				continue
			}
			n, err := parseInt(val, 64)
			if err != nil {
				return wrapFieldError(key, err)
			}
			out[key] = n
		}
		dst.Set(reflect.ValueOf(out))
		return nil
	}

	if dst.Type() == stringBoolMapType {
		out := make(map[string]bool, len(obj))
		for key, val := range obj {
			if val == nil {
				out[key] = false
				continue
			}
			b, err := parseBool(val)
			if err != nil {
				return wrapFieldError(key, err)
			}
			out[key] = b
		}
		dst.Set(reflect.ValueOf(out))
		return nil
	}

	if dst.Type().Key().Kind() == reflect.String &&
		unmarshalerKindOf(dst.Type().Key()) == unmarshalerNone {
		return scanStringKeyedMap(obj, dst, fresh)
	}

	var (
		keyKind  = unmarshalerKindOf(dst.Type().Key())
		elemKind = unmarshalerKindOf(dst.Type().Elem())
		out      = reflect.MakeMapWithSize(dst.Type(), len(obj))
	)

	for key, val := range obj {
		kv := reflect.New(dst.Type().Key()).Elem()
		if err = scanValueKind(key, kv, keyKind, false); err != nil {
			return wrapFieldError(key, err)
		}

		vv := reflect.New(dst.Type().Elem()).Elem()
		if err = scanValueKind(val, vv, elemKind, fresh); err != nil {
			return wrapFieldError(key, err)
		}

		out.SetMapIndex(kv, vv)
	}
	dst.Set(out)

	return nil
}

// scanStringKeyedMap scans a map whose key type is a string kind, avoiding per-key conversion.
func scanStringKeyedMap(obj map[string]any, dst reflect.Value, fresh bool) error {
	var (
		keyType  = dst.Type().Key()
		elemKind = unmarshalerKindOf(dst.Type().Elem())
		out      = reflect.MakeMapWithSize(dst.Type(), len(obj))
	)

	for key, val := range obj {
		kv := reflect.ValueOf(key).Convert(keyType)
		vv := reflect.New(dst.Type().Elem()).Elem()
		if err := scanValueKind(val, vv, elemKind, fresh); err != nil {
			return wrapFieldError(key, err)
		}
		out.SetMapIndex(kv, vv)
	}
	dst.Set(out)

	return nil
}

// scanSlice decodes a slice or array from another slice, an array or a delimited string.
func scanSlice(src any, dst reflect.Value, fresh bool) error {
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

	if in, ok := jsonInputOf(src); ok && in.isArray() && dst.Type() != anySliceType {
		if in.isText {
			return scanRawSlice(in.text, dst)
		}
		return scanRawSlice(in.data, dst)
	}

	if s, ok := stringValue(src); ok {
		if handled, err := scanStringSlice(s, dst); handled {
			return err
		}
	}

	if srcValue, ok := sliceSource(src); ok {
		return scanSliceValue(srcValue, dst, fresh)
	}

	items, err := scanItems(src)
	if err != nil {
		return err
	}

	return scanSliceItems(items, dst, true)
}

func scanSliceItems(items []any, dst reflect.Value, fresh bool) error {
	if dst.Type() == stringSliceType {
		if out, ok := nativeStringSlice(items); ok {
			dst.Set(reflect.ValueOf(out))
			return nil
		}
	}

	kind := unmarshalerKindOf(dst.Type().Elem())

	if dst.Kind() == reflect.Array {
		count := min(len(items), dst.Len())
		for i := range count {
			if err := scanValueKind(items[i], dst.Index(i), kind, fresh); err != nil {
				return wrapIndexError(i, err)
			}
		}
		return nil
	}

	out := reflect.MakeSlice(dst.Type(), len(items), len(items))
	for i := range items {
		if err := scanValueKind(items[i], out.Index(i), kind, fresh); err != nil {
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

// scanObjectFresh normalizes src into a string-keyed object. The fresh result reports whether the
// object was decoded from a string or byte source by this call and can therefore be adopted.
func scanObjectFresh(src any) (map[string]any, bool, error) {
	if xreflect.IsNil(src) {
		return nil, false, nil
	}

	switch v := src.(type) {
	case map[string]any:
		return v, false, nil
	case string:
		obj, err := decodeObjectString(v)
		return obj, err == nil, err
	case []byte:
		obj, err := decodeObject(v)
		return obj, err == nil, err
	}

	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		obj, err := decodeObjectString(rv.String())
		return obj, err == nil, err
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			obj, err := decodeObject(rv.Bytes())
			return obj, err == nil, err
		}
		return nil, false, unsupportedSource(rv, "object")
	case reflect.Map:
		obj := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			obj[xconv.String(iter.Key().Interface())] = iter.Value().Interface()
		}
		return obj, false, nil
	case reflect.Struct:
		obj, err := structObject(rv)
		return obj, false, err
	default:
		return nil, false, unsupportedSource(rv, "object")
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
func scanSliceValue(src, dst reflect.Value, fresh bool) error {
	if dst.Kind() == reflect.Array {
		kind := unmarshalerKindOf(dst.Type().Elem())
		count := min(src.Len(), dst.Len())
		for i := range count {
			if err := scanSliceElement(src.Index(i), dst.Index(i), i, kind, fresh); err != nil {
				return err
			}
		}
		return nil
	}

	if src.Type().Elem().Kind() == reflect.Interface && dst.Type() == stringSliceType {
		out := make([]string, src.Len())
		ok := true
		for i := range out {
			value, isString := stringValue(src.Index(i).Interface())
			if !isString {
				ok = false
				break
			}
			out[i] = value
		}
		if ok {
			dst.Set(reflect.ValueOf(out))
			return nil
		}
	}

	kind := unmarshalerKindOf(dst.Type().Elem())

	if src.Type().Elem() == dst.Type().Elem() && dst.Type().Elem().Kind() != reflect.Interface {
		out := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
		reflect.Copy(out, src)
		dst.Set(out)
		return nil
	}

	out := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
	for i := range src.Len() {
		if err := scanSliceElement(src.Index(i), out.Index(i), i, kind, fresh); err != nil {
			return err
		}
	}
	dst.Set(out)

	return nil
}

// scanSliceElement converts one element, using a direct primitive path when possible.
func scanSliceElement(src, dst reflect.Value, index int, kind unmarshalerKind, fresh bool) error {
	if ok, err := scanPrimitive(src, dst); ok {
		if err != nil {
			return wrapIndexError(index, err)
		}
		return nil
	}

	if err := scanValueKind(src.Interface(), dst, kind, fresh); err != nil {
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

// trimSpaceFast trims ASCII whitespace only when the string actually starts or ends with it.
func trimSpaceFast(s string) string {
	if s == "" {
		return s
	}
	if s[0] > ' ' && s[len(s)-1] > ' ' && s[0] < 0x80 && s[len(s)-1] < 0x80 {
		return s
	}
	return strings.TrimSpace(s)
}

// nativeStringSlice converts JSON array items into a native string slice.
func nativeStringSlice(items []any) ([]string, bool) {
	out := make([]string, len(items))
	for i, item := range items {
		value, ok := stringValue(item)
		if !ok {
			return nil, false
		}
		out[i] = value
	}

	return out, true
}

// scanStringSlice parses a delimited string directly into a primitive slice or array, avoiding an
// intermediate []any and []string.
func scanStringSlice(s string, dst reflect.Value) (bool, error) {
	s = trimSpaceFast(s)
	if s == "" {
		if dst.Kind() == reflect.Slice {
			dst.Set(reflect.MakeSlice(dst.Type(), 0, 0))
		}
		return true, nil
	}
	if s[0] == '[' {
		return false, nil
	}

	elem := dst.Type().Elem()
	if elem != durationType {
		switch elem.Kind() {
		case reflect.String, reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
		default:
			return false, nil
		}
	}

	count := strings.Count(s, ",") + 1
	if dst.Kind() == reflect.Array {
		count = min(count, dst.Len())
	}

	if dst.Type() == stringSliceType {
		out := make([]string, count)
		start := 0
		index := 0
		for i := range len(s) + 1 {
			if i < len(s) && s[i] != ',' {
				continue
			}
			if index >= count {
				break
			}
			out[index] = trimSpaceFast(s[start:i])
			index++
			start = i + 1
		}
		dst.Set(reflect.ValueOf(out))
		return true, nil
	}

	var out reflect.Value
	if dst.Kind() == reflect.Array {
		out = dst
	} else {
		out = reflect.MakeSlice(dst.Type(), count, count)
	}

	start := 0
	index := 0
	for i := range len(s) + 1 {
		if i < len(s) && s[i] != ',' {
			continue
		}
		if index >= count {
			break
		}
		if err := setStringSliceItem(out.Index(index), elem, trimSpaceFast(s[start:i])); err != nil {
			return true, wrapIndexError(index, err)
		}
		index++
		start = i + 1
	}

	if dst.Kind() == reflect.Slice {
		dst.Set(out)
	}

	return true, nil
}

// setStringSliceItem assigns one parsed string segment to a primitive slice element.
func setStringSliceItem(item reflect.Value, elem reflect.Type, value string) error {
	if elem == durationType {
		d, err := parseDurationText(value)
		if err != nil {
			return err
		}
		item.SetInt(int64(d))
		return nil
	}

	switch elem.Kind() {
	case reflect.String:
		item.SetString(value)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		item.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, elem.Bits())
		if err != nil {
			return err
		}
		item.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, elem.Bits())
		if err != nil {
			return err
		}
		item.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(value, elem.Bits())
		if err != nil {
			return err
		}
		item.SetFloat(n)
	}

	return nil
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

func decodeObjectString(s string) (map[string]any, error) {
	trimmed := trimSpaceFast(s)
	if trimmed == "" {
		return nil, nil
	}
	if trimmed[0] != '{' {
		return nil, errors.NewError("scan: source is not a JSON object")
	}

	var obj map[string]any
	if err := json.UnmarshalString(trimmed, &obj); err != nil {
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
	s = trimSpaceFast(s)
	if s == "" {
		return nil, nil
	}

	if s[0] == '[' {
		var items []any
		if err := json.UnmarshalString(s, &items); err != nil {
			return nil, err
		}
		return items, nil
	}

	parts := strings.Split(s, ",")
	items := make([]any, len(parts))
	for i := range parts {
		items[i] = trimSpaceFast(parts[i])
	}

	return items, nil
}

// fieldByIndex resolves a cached field index path. When alloc is true, nil embedded pointers on
// the way are allocated; otherwise the field is reported as unavailable.
func fieldByIndex(v reflect.Value, index []int, alloc bool) (reflect.Value, error) {
	if len(index) == 1 {
		if v.Kind() != reflect.Struct {
			return reflect.Value{}, errors.NewError("scan: invalid field index")
		}
		return v.Field(index[0]), nil
	}

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
		return strconv.ParseInt(trimSpaceFast(s), 10, bits)
	}
	if b, ok := src.([]byte); ok {
		return strconv.ParseInt(trimSpaceFast(string(b)), 10, bits)
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
		return strconv.ParseInt(trimSpaceFast(rv.String()), 10, bits)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseInt(trimSpaceFast(string(rv.Bytes())), 10, bits)
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
		return strconv.ParseUint(trimSpaceFast(s), 10, bits)
	}
	if b, ok := src.([]byte); ok {
		return strconv.ParseUint(trimSpaceFast(string(b)), 10, bits)
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
		return strconv.ParseUint(trimSpaceFast(rv.String()), 10, bits)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseUint(trimSpaceFast(string(rv.Bytes())), 10, bits)
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
		return strconv.ParseFloat(trimSpaceFast(s), bits)
	}
	if b, ok := src.([]byte); ok {
		return strconv.ParseFloat(trimSpaceFast(string(b)), bits)
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
		return strconv.ParseFloat(trimSpaceFast(rv.String()), bits)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseFloat(trimSpaceFast(string(rv.Bytes())), bits)
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
		return strconv.ParseBool(trimSpaceFast(v))
	case []byte:
		return strconv.ParseBool(trimSpaceFast(string(v)))
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
		return strconv.ParseBool(trimSpaceFast(rv.String()))
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return strconv.ParseBool(trimSpaceFast(string(rv.Bytes())))
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
	s = strings.ToLower(trimSpaceFast(s))
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
