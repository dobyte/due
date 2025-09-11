package internal

import (
	"fmt"
	"strconv"
	"time"
	"unsafe"
)

func String(val any) string {
	switch v := val.(type) {
	case int:
		return strconv.Itoa(v)
	case *int:
		return fmtPointer(unsafe.Pointer(v))
	case int8:
		return strconv.Itoa(int(v))
	case *int8:
		return fmtPointer(unsafe.Pointer(v))
	case int16:
		return strconv.Itoa(int(v))
	case *int16:
		return fmtPointer(unsafe.Pointer(v))
	case int32:
		return strconv.Itoa(int(v))
	case *int32:
		return fmtPointer(unsafe.Pointer(v))
	case int64:
		return strconv.FormatInt(v, 10)
	case *int64:
		return fmtPointer(unsafe.Pointer(v))
	case uint:
		return strconv.Itoa(int(v))
	case *uint:
		return fmtPointer(unsafe.Pointer(v))
	case uint8:
		return strconv.Itoa(int(v))
	case *uint8:
		return fmtPointer(unsafe.Pointer(v))
	case uint16:
		return strconv.Itoa(int(v))
	case *uint16:
		return fmtPointer(unsafe.Pointer(v))
	case uint32:
		return strconv.Itoa(int(v))
	case *uint32:
		return fmtPointer(unsafe.Pointer(v))
	case uint64:
		return strconv.FormatInt(int64(v), 10)
	case *uint64:
		return fmtPointer(unsafe.Pointer(v))
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case *float32:
		return fmtPointer(unsafe.Pointer(v))
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case *float64:
		return fmtPointer(unsafe.Pointer(v))
	case complex64:
		return strconv.FormatComplex(complex128(v), 'e', -1, 64)
	case *complex64:
		return fmtPointer(unsafe.Pointer(v))
	case complex128:
		return strconv.FormatComplex(v, 'e', -1, 128)
	case *complex128:
		return fmtPointer(unsafe.Pointer(v))
	case bool:
		return strconv.FormatBool(v)
	case *bool:
		return fmtPointer(unsafe.Pointer(v))
	case string:
		return v
	case *string:
		return fmtPointer(unsafe.Pointer(v))
	case time.Time:
		return v.String()
	case *time.Time:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func fmtPointer(p unsafe.Pointer) string {
	return "0x" + strconv.FormatInt(int64(uintptr(p)), 16)
}
