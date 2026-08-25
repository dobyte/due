package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

// GenericNumbers 将任意数值类型的切片/数组转换为指定泛型类型的数值切片
// 内部通过 JSON 序列化与反序列化实现类型转换，转换失败时返回空切片
// @param val any 待转换的切片或数组
// @return @1 []T 转换后的数值切片
func GenericNumbers[T any](val any) (slice []T) {
	if val == nil {
		return
	}

	var (
		b   []byte
		err error
	)

	switch v := val.(type) {
	case []int:
		b, err = json.Marshal(v)
	case *[]int:
		b, err = json.Marshal(*v)
	case []int8:
		b, err = json.Marshal(v)
	case *[]int8:
		b, err = json.Marshal(*v)
	case []int16:
		b, err = json.Marshal(v)
	case *[]int16:
		b, err = json.Marshal(*v)
	case []int32:
		b, err = json.Marshal(v)
	case *[]int32:
		b, err = json.Marshal(*v)
	case []int64:
		b, err = json.Marshal(v)
	case *[]int64:
		b, err = json.Marshal(*v)
	case []uint:
		b, err = json.Marshal(v)
	case *[]uint:
		b, err = json.Marshal(*v)
	case []uint8:
		b, err = json.Marshal(v)
	case *[]uint8:
		b, err = json.Marshal(*v)
	case []uint16:
		b, err = json.Marshal(v)
	case *[]uint16:
		b, err = json.Marshal(*v)
	case []uint32:
		b, err = json.Marshal(v)
	case *[]uint32:
		b, err = json.Marshal(*v)
	case []uint64:
		b, err = json.Marshal(v)
	case *[]uint64:
		b, err = json.Marshal(*v)
	case []float32:
		b, err = json.Marshal(v)
	case *[]float32:
		b, err = json.Marshal(*v)
	case []float64:
		b, err = json.Marshal(v)
	case *[]float64:
		b, err = json.Marshal(*v)
	case []complex64:
		b, err = json.Marshal(v)
	case *[]complex64:
		b, err = json.Marshal(*v)
	case []complex128:
		b, err = json.Marshal(v)
	case *[]complex128:
		b, err = json.Marshal(*v)
	case []string:
		temp := make([]int64, len(v))
		for i := range v {
			temp[i] = Int64(v[i])
		}

		b, err = json.Marshal(temp)
	case *[]string:
		temp := make([]int64, len(*v))
		for i := range *v {
			temp[i] = Int64((*v)[i])
		}

		b, err = json.Marshal(temp)
	case []bool:
		temp := make([]int8, len(v))
		for i := range v {
			temp[i] = Int8(v[i])
		}

		b, err = json.Marshal(temp)
	case *[]bool:
		temp := make([]int8, len(*v))
		for i := range *v {
			temp[i] = Int8((*v)[i])
		}

		b, err = json.Marshal(temp)
	case []any:
		temp := make([]int64, len(v))
		for i := range v {
			temp[i] = Int64(v[i])
		}

		b, err = json.Marshal(temp)
	case *[]any:
		temp := make([]int64, len(*v))
		for i := range *v {
			temp[i] = Int64((*v)[i])
		}

		b, err = json.Marshal(temp)
	case [][]byte:
		temp := make([]int64, len(v))
		for i := range v {
			temp[i] = Int64(v[i])
		}

		b, err = json.Marshal(temp)
	case *[][]byte:
		temp := make([]int64, len(*v))
		for i := range *v {
			temp[i] = Int64((*v)[i])
		}

		b, err = json.Marshal(temp)
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			temp := make([]int64, count)
			for i := range count {
				temp[i] = Int64(rv.Index(i).Interface())
			}

			b, err = json.Marshal(temp)
		default:
			return
		}
	}

	if err == nil {
		_ = json.Unmarshal(b, &slice)
	}

	return
}
