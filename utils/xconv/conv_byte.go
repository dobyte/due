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
		return []byte(v)
	case *string:
		return []byte(*v)
	case []byte:
		return v
	case *[]byte:
		return *v
	default:
		return nil
	}
}
