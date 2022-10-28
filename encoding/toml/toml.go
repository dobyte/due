package toml

import (
	"bytes"
	"github.com/BurntSushi/toml"
	"github.com/dobyte/due/encoding"
)

const Name = "toml"

var _ encoding.Codec = &codec{}

func init() {
	encoding.Register(&codec{})
}

type codec struct{}

func NewCodec() *codec {
	return &codec{}
}

// Name 编解码器名称
func (codec) Name() string {
	return Name
}

// Marshal 编码
func (codec) Marshal(v interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	err := toml.NewEncoder(buffer).Encode(v)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Unmarshal 解码
func (codec) Unmarshal(data []byte, v interface{}) error {
	return toml.Unmarshal(data, v)
}
