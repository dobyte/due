package cache

import (
	"time"

	"github.com/dobyte/due/v2/core/value"
)

// Result 缓存读取结果接口，提供多种类型转换方法
type Result interface {
	// Err 返回错误信息
	// @return @1 error 错误信息
	Err() error
	// Result 返回原始值与错误
	// @return @1 value.Value 原始值
	// @return @2 error 错误信息
	Result() (value.Value, error)
	// Int 以 int 类型返回缓存值
	// @return @1 int 转换后的值
	// @return @2 error 错误信息
	Int() (int, error)
	// Int8 以 int8 类型返回缓存值
	// @return @1 int8 转换后的值
	// @return @2 error 错误信息
	Int8() (int8, error)
	// Int16 以 int16 类型返回缓存值
	// @return @1 int16 转换后的值
	// @return @2 error 错误信息
	Int16() (int16, error)
	// Int32 以 int32 类型返回缓存值
	// @return @1 int32 转换后的值
	// @return @2 error 错误信息
	Int32() (int32, error)
	// Int64 以 int64 类型返回缓存值
	// @return @1 int64 转换后的值
	// @return @2 error 错误信息
	Int64() (int64, error)
	// Uint 以 uint 类型返回缓存值
	// @return @1 uint 转换后的值
	// @return @2 error 错误信息
	Uint() (uint, error)
	// Uint8 以 uint8 类型返回缓存值
	// @return @1 uint8 转换后的值
	// @return @2 error 错误信息
	Uint8() (uint8, error)
	// Uint16 以 uint16 类型返回缓存值
	// @return @1 uint16 转换后的值
	// @return @2 error 错误信息
	Uint16() (uint16, error)
	// Uint32 以 uint32 类型返回缓存值
	// @return @1 uint32 转换后的值
	// @return @2 error 错误信息
	Uint32() (uint32, error)
	// Uint64 以 uint64 类型返回缓存值
	// @return @1 uint64 转换后的值
	// @return @2 error 错误信息
	Uint64() (uint64, error)
	// Float32 以 float32 类型返回缓存值
	// @return @1 float32 转换后的值
	// @return @2 error 错误信息
	Float32() (float32, error)
	// Float64 以 float64 类型返回缓存值
	// @return @1 float64 转换后的值
	// @return @2 error 错误信息
	Float64() (float64, error)
	// Bool 以 bool 类型返回缓存值
	// @return @1 bool 转换后的值
	// @return @2 error 错误信息
	Bool() (bool, error)
	// String 以 string 类型返回缓存值
	// @return @1 string 转换后的值
	// @return @2 error 错误信息
	String() (string, error)
	// Duration 以 time.Duration 类型返回缓存值
	// @return @1 time.Duration 转换后的值
	// @return @2 error 错误信息
	Duration() (time.Duration, error)
	// Ints 以 []int 切片类型返回缓存值
	// @return @1 []int 转换后的值
	// @return @2 error 错误信息
	Ints() ([]int, error)
	// Int8s 以 []int8 切片类型返回缓存值
	// @return @1 []int8 转换后的值
	// @return @2 error 错误信息
	Int8s() ([]int8, error)
	// Int16s 以 []int16 切片类型返回缓存值
	// @return @1 []int16 转换后的值
	// @return @2 error 错误信息
	Int16s() ([]int16, error)
	// Int32s 以 []int32 切片类型返回缓存值
	// @return @1 []int32 转换后的值
	// @return @2 error 错误信息
	Int32s() ([]int32, error)
	// Int64s 以 []int64 切片类型返回缓存值
	// @return @1 []int64 转换后的值
	// @return @2 error 错误信息
	Int64s() ([]int64, error)
	// Uints 以 []uint 切片类型返回缓存值
	// @return @1 []uint 转换后的值
	// @return @2 error 错误信息
	Uints() ([]uint, error)
	// Uint8s 以 []uint8 切片类型返回缓存值
	// @return @1 []uint8 转换后的值
	// @return @2 error 错误信息
	Uint8s() ([]uint8, error)
	// Uint16s 以 []uint16 切片类型返回缓存值
	// @return @1 []uint16 转换后的值
	// @return @2 error 错误信息
	Uint16s() ([]uint16, error)
	// Uint32s 以 []uint32 切片类型返回缓存值
	// @return @1 []uint32 转换后的值
	// @return @2 error 错误信息
	Uint32s() ([]uint32, error)
	// Uint64s 以 []uint64 切片类型返回缓存值
	// @return @1 []uint64 转换后的值
	// @return @2 error 错误信息
	Uint64s() ([]uint64, error)
	// Float32s 以 []float32 切片类型返回缓存值
	// @return @1 []float32 转换后的值
	// @return @2 error 错误信息
	Float32s() ([]float32, error)
	// Float64s 以 []float64 切片类型返回缓存值
	// @return @1 []float64 转换后的值
	// @return @2 error 错误信息
	Float64s() ([]float64, error)
	// Bools 以 []bool 切片类型返回缓存值
	// @return @1 []bool 转换后的值
	// @return @2 error 错误信息
	Bools() ([]bool, error)
	// Strings 以 []string 切片类型返回缓存值
	// @return @1 []string 转换后的值
	// @return @2 error 错误信息
	Strings() ([]string, error)
	// Bytes 以 []byte 切片类型返回缓存值
	// @return @1 []byte 转换后的值
	// @return @2 error 错误信息
	Bytes() ([]byte, error)
	// Durations 以 []time.Duration 切片类型返回缓存值
	// @return @1 []time.Duration 转换后的值
	// @return @2 error 错误信息
	Durations() ([]time.Duration, error)
	// Slice 以 []any 切片类型返回缓存值
	// @return @1 []any 转换后的值
	// @return @2 error 错误信息
	Slice() ([]any, error)
	// Map 以 map[string]any 类型返回缓存值
	// @return @1 map[string]any 转换后的值
	// @return @2 error 错误信息
	Map() (map[string]any, error)
	// Scan 将缓存值扫描到目标对象
	// @param pointer any 目标对象指针
	// @return @1 error 错误信息
	Scan(pointer any) error
}

// result 缓存读取结果默认实现
type result struct {
	err   error
	value value.Value
}

// NewResult 创建一个缓存读取结果
// @param val any 缓存值
// @param err ...error 可选错误信息
// @return @1 Result 缓存读取结果
func NewResult(val any, err ...error) Result {
	if len(err) > 0 {
		return &result{err: err[0], value: value.NewValue(val)}
	} else {
		return &result{value: value.NewValue(val)}
	}
}

// Err 返回错误信息
// @return @1 error 错误信息
func (r *result) Err() error {
	return r.err
}

// Result 返回原始值与错误
// @return @1 value.Value 原始值
// @return @2 error 错误信息
func (r *result) Result() (value.Value, error) {
	return r.value, r.err
}

// Int 以 int 类型返回缓存值
// @return @1 int 转换后的值
// @return @2 error 错误信息
func (r *result) Int() (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int(), nil
}

// Int8 以 int8 类型返回缓存值
// @return @1 int8 转换后的值
// @return @2 error 错误信息
func (r *result) Int8() (int8, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int8(), nil
}

// Int16 以 int16 类型返回缓存值
// @return @1 int16 转换后的值
// @return @2 error 错误信息
func (r *result) Int16() (int16, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int16(), nil
}

// Int32 以 int32 类型返回缓存值
// @return @1 int32 转换后的值
// @return @2 error 错误信息
func (r *result) Int32() (int32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int32(), nil
}

// Int64 以 int64 类型返回缓存值
// @return @1 int64 转换后的值
// @return @2 error 错误信息
func (r *result) Int64() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int64(), nil
}

// Uint 以 uint 类型返回缓存值
// @return @1 uint 转换后的值
// @return @2 error 错误信息
func (r *result) Uint() (uint, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint(), nil
}

// Uint8 以 uint8 类型返回缓存值
// @return @1 uint8 转换后的值
// @return @2 error 错误信息
func (r *result) Uint8() (uint8, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint8(), nil
}

// Uint16 以 uint16 类型返回缓存值
// @return @1 uint16 转换后的值
// @return @2 error 错误信息
func (r *result) Uint16() (uint16, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint16(), nil
}

// Uint32 以 uint32 类型返回缓存值
// @return @1 uint32 转换后的值
// @return @2 error 错误信息
func (r *result) Uint32() (uint32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint32(), nil
}

// Uint64 以 uint64 类型返回缓存值
// @return @1 uint64 转换后的值
// @return @2 error 错误信息
func (r *result) Uint64() (uint64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint64(), nil
}

// Float32 以 float32 类型返回缓存值
// @return @1 float32 转换后的值
// @return @2 error 错误信息
func (r *result) Float32() (float32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Float32(), nil
}

// Float64 以 float64 类型返回缓存值
// @return @1 float64 转换后的值
// @return @2 error 错误信息
func (r *result) Float64() (float64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Float64(), nil
}

// Rune 以 rune 类型返回缓存值
// @return @1 rune 转换后的值
// @return @2 error 错误信息
func (r *result) Rune() (rune, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Rune(), nil
}

// Bool 以 bool 类型返回缓存值
// @return @1 bool 转换后的值
// @return @2 error 错误信息
func (r *result) Bool() (bool, error) {
	if r.err != nil {
		return false, r.err
	}

	return r.value.Bool(), nil
}

// String 以 string 类型返回缓存值
// @return @1 string 转换后的值
// @return @2 error 错误信息
func (r *result) String() (string, error) {
	if r.err != nil {
		return "", r.err
	}

	return r.value.String(), nil
}

// B 以 float64 类型返回缓存值
// @return @1 float64 转换后的值
// @return @2 error 错误信息
func (r *result) B() (float64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.B(), nil
}

// Duration 以 time.Duration 类型返回缓存值
// @return @1 time.Duration 转换后的值
// @return @2 error 错误信息
func (r *result) Duration() (time.Duration, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Duration(), nil
}

// Ints 以 []int 切片类型返回缓存值
// @return @1 []int 转换后的值
// @return @2 error 错误信息
func (r *result) Ints() ([]int, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Ints(), nil
}

// Int8s 以 []int8 切片类型返回缓存值
// @return @1 []int8 转换后的值
// @return @2 error 错误信息
func (r *result) Int8s() ([]int8, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int8s(), nil
}

// Int16s 以 []int16 切片类型返回缓存值
// @return @1 []int16 转换后的值
// @return @2 error 错误信息
func (r *result) Int16s() ([]int16, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int16s(), nil
}

// Int32s 以 []int32 切片类型返回缓存值
// @return @1 []int32 转换后的值
// @return @2 error 错误信息
func (r *result) Int32s() ([]int32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int32s(), nil
}

// Int64s 以 []int64 切片类型返回缓存值
// @return @1 []int64 转换后的值
// @return @2 error 错误信息
func (r *result) Int64s() ([]int64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int64s(), nil
}

// Uints 以 []uint 切片类型返回缓存值
// @return @1 []uint 转换后的值
// @return @2 error 错误信息
func (r *result) Uints() ([]uint, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uints(), nil
}

// Uint8s 以 []uint8 切片类型返回缓存值
// @return @1 []uint8 转换后的值
// @return @2 error 错误信息
func (r *result) Uint8s() ([]uint8, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint8s(), nil
}

// Uint16s 以 []uint16 切片类型返回缓存值
// @return @1 []uint16 转换后的值
// @return @2 error 错误信息
func (r *result) Uint16s() ([]uint16, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint16s(), nil
}

// Uint32s 以 []uint32 切片类型返回缓存值
// @return @1 []uint32 转换后的值
// @return @2 error 错误信息
func (r *result) Uint32s() ([]uint32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint32s(), nil
}

// Uint64s 以 []uint64 切片类型返回缓存值
// @return @1 []uint64 转换后的值
// @return @2 error 错误信息
func (r *result) Uint64s() ([]uint64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint64s(), nil
}

// Float32s 以 []float32 切片类型返回缓存值
// @return @1 []float32 转换后的值
// @return @2 error 错误信息
func (r *result) Float32s() ([]float32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Float32s(), nil
}

// Float64s 以 []float64 切片类型返回缓存值
// @return @1 []float64 转换后的值
// @return @2 error 错误信息
func (r *result) Float64s() ([]float64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Float64s(), nil
}

// Runes 以 []rune 切片类型返回缓存值
// @return @1 []rune 转换后的值
// @return @2 error 错误信息
func (r *result) Runes() ([]rune, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Runes(), nil
}

// Bools 以 []bool 切片类型返回缓存值
// @return @1 []bool 转换后的值
// @return @2 error 错误信息
func (r *result) Bools() ([]bool, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bools(), nil
}

// Strings 以 []string 切片类型返回缓存值
// @return @1 []string 转换后的值
// @return @2 error 错误信息
func (r *result) Strings() ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Strings(), nil
}

// Bs 以 []float64 切片类型返回缓存值
// @return @1 []float64 转换后的值
// @return @2 error 错误信息
func (r *result) Bs() ([]float64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bs(), nil
}

// Bytes 以 []byte 切片类型返回缓存值
// @return @1 []byte 转换后的值
// @return @2 error 错误信息
func (r *result) Bytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bytes(), nil
}

// Durations 以 []time.Duration 切片类型返回缓存值
// @return @1 []time.Duration 转换后的值
// @return @2 error 错误信息
func (r *result) Durations() ([]time.Duration, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Durations(), nil
}

// Slice 以 []any 切片类型返回缓存值
// @return @1 []any 转换后的值
// @return @2 error 错误信息
func (r *result) Slice() ([]any, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Slice(), nil
}

// Map 以 map[string]any 类型返回缓存值
// @return @1 map[string]any 转换后的值
// @return @2 error 错误信息
func (r *result) Map() (map[string]any, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Map(), nil
}

// Scan 将缓存值扫描到目标对象
// @param pointer any 目标对象指针
// @return @1 error 错误信息
func (r *result) Scan(pointer any) error {
	if r.err != nil {
		return r.err
	}

	return r.value.Scan(pointer)
}
