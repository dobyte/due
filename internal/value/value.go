package value

import (
	"github.com/dobyte/due/utils/xconv"
	"time"
)

type Value interface {
	Value() interface{}
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
	Time() time.Time
	Duration() time.Duration
	Strings() []string
	Map() map[string]interface{}
	Slice() []interface{}
	Scan(pointer interface{}) error
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

func (v *value) Value() interface{} {
	return v.v
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

func (v *value) Time() time.Time {
	return time.Time{}
}

func (v *value) Duration() time.Duration {
	return xconv.Duration(v.Value())
}

func (v *value) Strings() []string {
	return xconv.Strings(v.Value())
}

func (v *value) Map() map[string]interface{} {
	return nil
}

func (v *value) Slice() []interface{} {
	return nil
}

func (v *value) Scan(pointer interface{}) error {
	return nil
}
