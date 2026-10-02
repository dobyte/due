package msgpack

import (
	"github.com/shamaton/msgpack/v2"
)

const Name = "msgpack"

var DefaultCodec = &codec{}

type codec struct{}

// Name returns the codec name.
func (codec) Name() string {
	return Name
}

// Marshal encodes v.
func (codec) Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Unmarshal decodes data into v.
func (codec) Unmarshal(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

// Marshal encodes v using the default codec.
func Marshal(v any) ([]byte, error) {
	return DefaultCodec.Marshal(v)
}

// Unmarshal decodes data into v using the default codec.
func Unmarshal(data []byte, v any) error {
	return DefaultCodec.Unmarshal(data, v)
}
