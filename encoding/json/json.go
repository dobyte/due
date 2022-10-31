/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/14 10:42 上午
 * @Desc: TODO
 */

package json

import (
	"encoding/json"

	"github.com/dobyte/due/encoding"
)

const Name = "json"

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
	return json.Marshal(v)
}

// Unmarshal 解码
func (codec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
