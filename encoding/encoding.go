/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/14 10:47 上午
 * @Desc: TODO
 */

package encoding

import (
	"log"
)

type Codec interface {
	// Name 编解码器类型
	Name() string
	// Marshal 编码
	Marshal(v interface{}) ([]byte, error)
	// Unmarshal 解码
	Unmarshal(data []byte, v interface{}) error
}

var codecs = make(map[string]Codec)

// Register 注册编解码器
func Register(codec Codec) {
	if codec == nil {
		log.Fatal("can't register a invalid codec")
	}

	if codec.Name() == "" {
		log.Fatal("can't register a codec without a name")
	}

	codecs[codec.Name()] = codec
}

// Invoke 调用编解码器
func Invoke(name string) Codec {
	codec, ok := codecs[name]
	if !ok {
		log.Fatalf("%s codec is not registered", name)
	}

	return codec
}
