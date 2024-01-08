package xconv

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/encoding/json"
	"reflect"
)

func Byte(any interface{}) byte {
	return Uint8(any)
}

func Bytes(any interface{}) []byte {
	if any == nil {
		return nil
	}

	var (
		err error
		buf = bytes.NewBuffer(nil)
	)

	switch v := any.(type) {
	case bool, *bool, int, *int, int8, *int8, int16, *int16, int32, *int32, int64, *int64, uint, *uint, uint8, *uint8, uint16, *uint16, uint32, *uint32, uint64, *uint64, float32, *float32, float64, *float64:
		err = binary.Write(buf, binary.BigEndian, v)
	case complex64, *complex64, complex128, *complex128:
		return nil
	case string:
		return StringToBytes(v)
	case *string:
		return StringToBytes(*v)
	case []byte:
		return v
	case *[]byte:
		return *v
	default:
		var (
			rv   = reflect.ValueOf(any)
			kind = rv.Kind()
		)

		for kind == reflect.Ptr {
			rv = rv.Elem()
			kind = rv.Kind()
		}

		switch kind {
		case reflect.Invalid:
			return nil
		case reflect.Bool:
			err = binary.Write(buf, binary.BigEndian, rv.Bool())
		case reflect.String:
			return StringToBytes(rv.String())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			err = binary.Write(buf, binary.BigEndian, rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			err = binary.Write(buf, binary.BigEndian, rv.Uint())
		case reflect.Float32, reflect.Float64:
			err = binary.Write(buf, binary.BigEndian, rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return nil
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil
			}
			return b
		}
	}
	if err != nil {
		return nil
	}

	return buf.Bytes()
}

func BytePointer(any interface{}) *byte {
	v := Byte(any)
	return &v
}

func BytesPointer(any interface{}) *[]byte {
	v := Bytes(any)
	return &v
}
