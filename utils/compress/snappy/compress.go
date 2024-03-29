package snappy

import "github.com/golang/snappy"

func Encode(in []byte) ([]byte, error) {
	result := snappy.Encode(nil, in)
	return result, nil
}

func Decode(in []byte) ([]byte, error) {
	return snappy.Decode(nil, in)
}
