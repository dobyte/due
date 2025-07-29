package value

import (
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xconv"
)

type Value interface {
	Int() int
	Int8() int8
	Int16() int16
	Int32() int32
	Int64() int64
	Uint() uint
	Uint8() uint8
	Uint16() uint16
	Uint32() uint32
	Uint64() uint64
	Float32() float32
	Float64() float64
	Bool() bool
	String() string
	Duration() time.Duration
	B() float64
	Ints() []int
	Int8s() []int8
	Int16s() []int16
	Int32s() []int32
	Int64s() []int64
	Uints() []uint
	Uint8s() []uint8
	Uint16s() []uint16
	Uint32s() []uint32
	Uint64s() []uint64
	Float32s() []float32
	Float64s() []float64
	Bools() []bool
	Strings() []string
	Bytes() []byte
	Bs() []float64
	Durations() []time.Duration
	Slice() []interface{}
	Map() map[string]interface{}
	Scan(pointer interface{}) error
	Value() interface{}
}

type value struct {
	v interface{}
}

func NewValue(v ...interface{}) Value {
	if len(v) == 0 {
		return &value{v: nil}
	}
	return &value{v: v[0]}
}

func (v *value) Int() int {
	return xconv.Int(v.Value())
}

func (v *value) Int8() int8 {
	return xconv.Int8(v.Value())
}

func (v *value) Int16() int16 {
	return xconv.Int16(v.Value())
}

func (v *value) Int32() int32 {
	return xconv.Int32(v.Value())
}

func (v *value) Int64() int64 {
	return xconv.Int64(v.Value())
}

func (v *value) Uint() uint {
	return xconv.Uint(v.Value())
}

func (v *value) Uint8() uint8 {
	return xconv.Uint8(v.Value())
}

func (v *value) Uint16() uint16 {
	return xconv.Uint16(v.Value())
}

func (v *value) Uint32() uint32 {
	return xconv.Uint32(v.Value())
}

func (v *value) Uint64() uint64 {
	return xconv.Uint64(v.Value())
}

func (v *value) Float32() float32 {
	return xconv.Float32(v.Value())
}

func (v *value) Float64() float64 {
	return xconv.Float64(v.Value())
}

func (v *value) Bool() bool {
	return xconv.Bool(v.Value())
}

func (v *value) String() string {
	return xconv.String(v.Value())
}

func (v *value) B() float64 {
	return xconv.B(v.Value())
}

func (v *value) Duration() time.Duration {
	return xconv.Duration(v.Value())
}

func (v *value) Ints() []int {
	return xconv.Ints(v.Value())
}

func (v *value) Int8s() []int8 {
	return xconv.Int8s(v.Value())
}

func (v *value) Int16s() []int16 {
	return xconv.Int16s(v.Value())
}

func (v *value) Int32s() []int32 {
	return xconv.Int32s(v.Value())
}

func (v *value) Int64s() []int64 {
	return xconv.Int64s(v.Value())
}

func (v *value) Uints() []uint {
	return xconv.Uints(v.Value())
}

func (v *value) Uint8s() []uint8 {
	return xconv.Uint8s(v.Value())
}

func (v *value) Uint16s() []uint16 {
	return xconv.Uint16s(v.Value())
}

func (v *value) Uint32s() []uint32 {
	return xconv.Uint32s(v.Value())
}

func (v *value) Uint64s() []uint64 {
	return xconv.Uint64s(v.Value())
}

func (v *value) Float32s() []float32 {
	return xconv.Float32s(v.Value())
}

func (v *value) Float64s() []float64 {
	return xconv.Float64s(v.Value())
}

func (v *value) Bools() []bool {
	return xconv.Bools(v.Value())
}

func (v *value) Strings() []string {
	return xconv.Strings(v.Value())
}

func (v *value) Bytes() []byte {
	return xconv.Bytes(v.Value())
}

func (v *value) Bs() []float64 {
	return xconv.Bs(v.Value())
}

func (v *value) Durations() []time.Duration {
	return xconv.Durations(v.Value())
}

func (v *value) Slice() []interface{} {
	return xconv.Interfaces(v.Value())
}

func (v *value) Map() map[string]interface{} {
	m := make(map[string]interface{})
	if err := v.Scan(&m); err != nil {
		return nil
	}

	return m
}

func (v *value) Scan(pointer interface{}) error {
	switch p := pointer.(type) {
	case *int:
		*p = v.Int()
	case *int8:
		*p = v.Int8()
	case *int16:
		*p = v.Int16()
	case *int32:
		*p = v.Int32()
	case *int64:
		*p = v.Int64()
	case *uint:
		*p = v.Uint()
	case *uint8:
		*p = v.Uint8()
	case *uint16:
		*p = v.Uint16()
	case *uint32:
		*p = v.Uint32()
	case *uint64:
		*p = v.Uint64()
	case *float32:
		*p = v.Float32()
	case *float64:
		*p = v.Float64()
	case *bool:
		*p = v.Bool()
	case *string:
		*p = v.String()
	case *time.Duration:
		*p = v.Duration()
	default:
		switch val := v.Value().(type) {
		case string, *string:
			return json.Unmarshal(v.Bytes(), pointer)
		case []byte, *[]byte:
			return json.Unmarshal(v.Bytes(), pointer)
		default:
			b, err := json.Marshal(val)
			if err != nil {
				return err
			}

			return json.Unmarshal(b, pointer)
		}
	}

	return nil
}

func (v *value) Value() interface{} {
	return v.v
}
