package toml

import (
	"github.com/BurntSushi/toml"
)

const Name = "toml"

var DefaultCodec = &codec{}

type codec struct{}

// Name 编解码器名称
func (codec) Name() string {
	return Name
}

// Marshal 编码
func (codec) Marshal(v interface{}) ([]byte, error) {
	return toml.Marshal(v)
}

// Unmarshal 解码
func (codec) Unmarshal(data []byte, v interface{}) error {
	return toml.Unmarshal(data, v)
}

// Marshal 编码
func Marshal(v interface{}) ([]byte, error) {
	return DefaultCodec.Marshal(v)
}

// Unmarshal 解码
func Unmarshal(data []byte, v interface{}) error {
	return DefaultCodec.Unmarshal(data, v)
}
