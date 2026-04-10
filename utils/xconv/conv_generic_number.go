package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

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
