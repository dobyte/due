package xml

import (
	"encoding/xml"
	"github.com/dobyte/due/encoding"
)

const Name = "xml"

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
	return xml.Marshal(v)
}

// Unmarshal 解码
func (codec) Unmarshal(data []byte, v interface{}) error {
	return xml.Unmarshal(data, v)
}
