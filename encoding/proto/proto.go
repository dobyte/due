/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/14 10:47 上午
 * @Desc: TODO
 */

package proto

import (
	"errors"

	"github.com/gogo/protobuf/proto"

	"github.com/dobyte/due/encoding"
)

const Name = "proto"

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
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, errors.New("can't marshal a value that not implements proto.Message interface")
	}

	return proto.Marshal(msg)
}

// Unmarshal 解码
func (defaultCodec) Unmarshal(data []byte, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return errors.New("can't unmarshal to a value that not implements proto.Message")
	}

	return proto.Unmarshal(data, msg)
}

// Marshal 编码
func Marshal(v interface{}) ([]byte, error) {
	return codec.Marshal(v)
}

// Unmarshal 解码
func Unmarshal(data []byte, v interface{}) error {
	return codec.Unmarshal(data, v)
}
