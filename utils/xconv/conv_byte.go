package xconv

func Byte(any interface{}) byte {
	return Uint8(any)
}

func Bytes(any interface{}) []byte {
	if any == nil {
		return nil
	}

	switch v := any.(type) {
	case string:
		return StringToBytes(v)
	case *string:
		return StringToBytes(*v)
	case []byte:
		return v
	case *[]byte:
		return *v
	default:
		return nil
	}
}

func BytePointer(any interface{}) *byte {
	v := Byte(any)
	return &v
}

func BytesPointer(any interface{}) *[]byte {
	v := Bytes(any)
	return &v
}
