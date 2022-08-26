/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/14 10:47 上午
 * @Desc: TODO
 */

package proto

import (
	"errors"

	"google.golang.org/protobuf/proto"

	"github.com/dobyte/due/encoding"
)

const Name = "proto"

var _ encoding.Codec = &codec{}

func init() {
	encoding.Register(&codec{})
}

type codec struct{}

// Name 编解码器名称
func (codec) Name() string {
	return Name
}

// Marshal 编码
func (codec) Marshal(v interface{}) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, errors.New("can't marshal a value that not implements proto.Message interface")
	}

	return proto.Marshal(msg)
}

// Unmarshal 解码
func (codec) Unmarshal(data []byte, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return errors.New("can't unmarshal to a value that not implements proto.Message")
	}

	return proto.Unmarshal(data, msg)
}
