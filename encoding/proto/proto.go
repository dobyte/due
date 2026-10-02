package proto

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

const Name = "proto"

var DefaultCodec = &codec{}

type codec struct{}

// Name returns the codec name.
func (codec) Name() string {
	return Name
}

// Marshal encodes v.
func (codec) Marshal(v any) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, errors.New("can't marshal a value that not implements proto.Buffer interface")
	}

	return proto.Marshal(msg)
}

// Unmarshal decodes data into v.
func (codec) Unmarshal(data []byte, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return errors.New("can't unmarshal to a value that not implements proto.Buffer")
	}

	return proto.Unmarshal(data, msg)
}

// Marshal encodes v using the default codec.
func Marshal(v any) ([]byte, error) {
	return DefaultCodec.Marshal(v)
}

// Unmarshal decodes data into v using the default codec.
func Unmarshal(data []byte, v any) error {
	return DefaultCodec.Unmarshal(data, v)
}
