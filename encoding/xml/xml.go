package xml

import (
	"encoding/xml"
	"github.com/dobyte/due/encoding"
)

const Name = "xml"

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
	return xml.Marshal(v)
}

// Unmarshal 解码
func (defaultCodec) Unmarshal(data []byte, v interface{}) error {
	return xml.Unmarshal(data, v)
}

// Marshal 编码
func Marshal(v interface{}) ([]byte, error) {
	return codec.Marshal(v)
}

// Unmarshal 解码
func Unmarshal(data []byte, v interface{}) error {
	return codec.Unmarshal(data, v)
}
