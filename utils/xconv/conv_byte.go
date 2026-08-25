package xconv

import (
	"bytes"
	"encoding/binary"
	"reflect"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

// Byte 将任意值转换为字节（等价于 Uint8）
// @param val any 待转换的值
// @return @1 byte 转换后的字节
func Byte(val any) byte {
	return Uint8(val)
}

// Bytes 将任意值转换为字节切片
// 基础数值类型按大端序编码（int 编码为8字节、uint 编码为8字节、其他按自身位宽编码），
// 字符串无拷贝转换，[]byte 直接返回，其他类型优先 JSON 序列化；无法转换时返回 nil
// @param val any 待转换的值
// @return @1 []byte 转换后的字节切片
func Bytes(val any) []byte {
	if val == nil {
		return nil
	}

	var (
		err error
		buf = bytes.NewBuffer(nil)
	)

	switch v := val.(type) {
	case int:
		err = binary.Write(buf, binary.BigEndian, int64(v))
	case *int:
		if v == nil {
			return nil
		}

		err = binary.Write(buf, binary.BigEndian, int64(*v))
	case uint:
		err = binary.Write(buf, binary.BigEndian, uint64(v))
	case *uint:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, uint64(*v))
	case bool, int8, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64:
		err = binary.Write(buf, binary.BigEndian, v)
	case *bool:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *int8:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *int16:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *int32:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *int64:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *uint8:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *uint16:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *uint32:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *uint64:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *float32:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case *float64:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, *v)
	case uintptr:
		err = binary.Write(buf, binary.BigEndian, uint64(v))
	case *uintptr:
		if v == nil {
			return nil
		}
		err = binary.Write(buf, binary.BigEndian, uint64(*v))
	case complex64, *complex64, complex128, *complex128:
		return nil
	case string:
		return StringToBytes(v)
	case *string:
		if v == nil {
			return nil
		}
		return StringToBytes(*v)
	case []byte:
		return v
	case *[]byte:
		if v == nil {
			return nil
		}
		return *v
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Invalid:
			return nil
		case reflect.Bool:
			err = binary.Write(buf, binary.BigEndian, rv.Bool())
		case reflect.String:
			return StringToBytes(rv.String())
		case reflect.Int, reflect.Int64:
			err = binary.Write(buf, binary.BigEndian, rv.Int())
		case reflect.Int8:
			err = binary.Write(buf, binary.BigEndian, int8(rv.Int()))
		case reflect.Int16:
			err = binary.Write(buf, binary.BigEndian, int16(rv.Int()))
		case reflect.Int32:
			err = binary.Write(buf, binary.BigEndian, int32(rv.Int()))
		case reflect.Uint, reflect.Uint64, reflect.Uintptr:
			err = binary.Write(buf, binary.BigEndian, rv.Uint())
		case reflect.Uint8:
			err = binary.Write(buf, binary.BigEndian, uint8(rv.Uint()))
		case reflect.Uint16:
			err = binary.Write(buf, binary.BigEndian, uint16(rv.Uint()))
		case reflect.Uint32:
			err = binary.Write(buf, binary.BigEndian, uint32(rv.Uint()))
		case reflect.Float32, reflect.Float64:
			err = binary.Write(buf, binary.BigEndian, rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return nil
		default:
			if b, e := json.Marshal(v); e != nil {
				return nil
			} else {
				return b
			}
		}
	}
	if err != nil {
		return nil
	}

	return buf.Bytes()
}

// BytePointer 将任意值转换为字节指针
// @param val any 待转换的值
// @return @1 *byte 转换后的字节指针
func BytePointer(any any) *byte {
	v := Byte(any)
	return &v
}

// BytesPointer 将任意值转换为字节切片指针
// @param val any 待转换的值
// @return @1 *[]byte 转换后的字节切片指针
func BytesPointer(any any) *[]byte {
	v := Bytes(any)
	return &v
}
