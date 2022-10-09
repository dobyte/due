package config

import (
	"github.com/dobyte/due/utils/conv"
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
	return conv.Int(v.Value())
}

func (v *Value) Int8() int8 {
	return conv.Int8(v.Value())
}

func (v *Value) Int16() int16 {
	return conv.Int16(v.Value())
}

func (v *Value) Int32() int32 {
	return conv.Int32(v.Value())
}

func (v *Value) Int64() int64 {
	return conv.Int64(v.Value())
}

func (v *Value) Uint() uint {
	return conv.Uint(v.Value())
}

func (v *Value) Uint8() uint8 {
	return conv.Uint8(v.Value())
}

func (v *Value) Uint16() uint16 {
	return conv.Uint16(v.Value())
}

func (v *Value) Uint32() uint32 {
	return conv.Uint32(v.Value())
}

func (v *Value) Uint64() uint64 {
	return conv.Uint64(v.Value())
}

func (v *Value) Float32() float32 {
	return conv.Float32(v.Value())
}

func (v *Value) Float64() float64 {
	return conv.Float64(v.Value())
}

func (v *Value) Bool() bool {
	return conv.Bool(v.Value())
}

func (v *Value) String() string {
	return conv.String(v.Value())
}

func (v *Value) Time() time.Time {
	return time.Time{}
}

func (v *Value) Duration() time.Duration {
	return conv.Duration(v.Value())
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
