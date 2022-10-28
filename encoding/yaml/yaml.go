package yaml

import (
	"github.com/dobyte/due/encoding"
	"gopkg.in/yaml.v3"
)

const Name = "yaml"

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
	return yaml.Marshal(v)
}

// Unmarshal 解码
func (codec) Unmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
