/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/14 10:42 上午
 * @Desc: TODO
 */

package json

import (
	"github.com/dobyte/due/encoding"
	jsoniter "github.com/json-iterator/go"
)

const Name = "json"

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
	return jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(v)
}

// Unmarshal 解码
func (defaultCodec) Unmarshal(data []byte, v interface{}) error {
	return jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(data, v)
}

// Marshal 编码
func Marshal(v interface{}) ([]byte, error) {
	return codec.Marshal(v)
}

// Unmarshal 解码
func Unmarshal(data []byte, v interface{}) error {
	return codec.Unmarshal(data, v)
}
