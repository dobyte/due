//go:build !stdjson
// +build !stdjson

package json

import (
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/encoder"
)

// marshal encodes v with sonic, which is the default backend.
func marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// unmarshal decodes data into v with sonic, which is the default backend.
func unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}

// appendEncode appends the JSON encoding of v to dst without allocating when dst has capacity.
func appendEncode(dst []byte, v any) ([]byte, error) {
	if err := encoder.EncodeInto(&dst, v, 0); err != nil {
		return nil, err
	}

	return dst, nil
}
