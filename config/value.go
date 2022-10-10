package config

import (
	"sync/atomic"
	"time"
)

type Value struct {
	val atomic.Value
}

func (v *Value) Value() interface{} {
	return v.val.Load()
}

func (v *Value) Int() int {
	return xconv.Int(v.Value())
}

func (v *Value) Int8() int8 {
	return xconv.Int8(v.Value())
}

func (v *Value) Int16() int16 {
	return xconv.Int16(v.Value())
}

func (v *Value) Int32() int32 {
	return xconv.Int32(v.Value())
}

func (v *Value) Int64() int64 {
	return xconv.Int64(v.Value())
}

func (v *Value) Uint() uint {
	return xconv.Uint(v.Value())
}

func (v *Value) Uint8() uint8 {
	return xconv.Uint8(v.Value())
}

func (v *Value) Uint16() uint16 {
	return xconv.Uint16(v.Value())
}

func (v *Value) Uint32() uint32 {
	return xconv.Uint32(v.Value())
}

func (v *Value) Uint64() uint64 {
	return xconv.Uint64(v.Value())
}

func (v *Value) Float32() float32 {
	return xconv.Float32(v.Value())
}

func (v *Value) Float64() float64 {
	return xconv.Float64(v.Value())
}

func (v *Value) Bool() bool {
	return xconv.Bool(v.Value())
}

func (v *Value) String() string {
	return xconv.String(v.Value())
}

func (v *Value) Time() time.Time {
	return time.Time{}
}

func (v *Value) Duration() time.Duration {
	return xconv.Duration(v.Value())
}

func (v *Value) Map() map[string]interface{} {
	return nil
}

func (v *Value) Slice() []interface{} {
	return nil
}

func (v *Value) Scan(pointer interface{}, mapping ...map[string]string) error {
	return nil
}
