package toml

import (
	"bytes"
	"github.com/BurntSushi/toml"
	"github.com/dobyte/due/encoding"
)

const Name = "toml"

var codec encoding.Codec = &defaultCodec{}

func init() {
	encoding.Register(codec)
}

type defaultCodec struct{}

func NewCodec() *defaultCodec {
	return &defaultCodec{}
}

// Name 编解码器名称
func (defaultCodec) Name() string {
	return Name
}

// Marshal 编码
func (defaultCodec) Marshal(v interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	err := toml.NewEncoder(buffer).Encode(v)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Unmarshal 解码
func (defaultCodec) Unmarshal(data []byte, v interface{}) error {
	return toml.Unmarshal(data, v)
}

// Marshal 编码
func Marshal(v interface{}) ([]byte, error) {
	return codec.Marshal(v)
}

// Unmarshal 解码
func Unmarshal(data []byte, v interface{}) error {
	return codec.Unmarshal(data, v)
}
