/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/14 10:42 上午
 * @Desc: TODO
 */

package json

import (
	"github.com/bytedance/sonic"
)

const Name = "json"

var DefaultCodec = &codec{}

type codec struct{}

// Name 编解码器名称
func (codec) Name() string {
	return Name
}

// Marshal 编码
func (codec) Marshal(v interface{}) ([]byte, error) {
	return sonic.Marshal(v)
}

// Unmarshal 解码
func (codec) Unmarshal(data []byte, v interface{}) error {
	return sonic.Unmarshal(data, v)
}

// Marshal 编码
func Marshal(v interface{}) ([]byte, error) {
	return DefaultCodec.Marshal(v)
}

// Unmarshal 解码
func Unmarshal(data []byte, v interface{}) error {
	return DefaultCodec.Unmarshal(data, v)
}
