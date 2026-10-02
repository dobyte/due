package xconv

import (
	"bytes"
	"encoding/binary"
	"reflect"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

// Byte converts val to a byte (equivalent to [Uint8]).
func Byte(val any) byte {
	return Uint8(val)
}

// Bytes converts val to a byte slice.
//
// Basic numeric types are encoded in big-endian order (int as 8 bytes, uint as 8 bytes and the
// others at their own width); a string is converted without copying; a []byte is returned as is;
// other types are preferentially JSON-marshalled. It returns nil when the conversion fails.
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

// BytePointer converts val to a pointer to byte.
func BytePointer(any any) *byte {
	v := Byte(any)
	return &v
}

// BytesPointer converts val to a pointer to a byte slice.
func BytesPointer(any any) *[]byte {
	v := Bytes(any)
	return &v
}
