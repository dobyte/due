//go:build !stdjson
// +build !stdjson

package json

import (
	"github.com/bytedance/sonic"
)

// marshal encodes v with sonic, which is the default backend.
func marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// unmarshal decodes data into v with sonic, which is the default backend.
func unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}
