/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/14 10:47 上午
 * @Desc: TODO
 */

package encoding

import (
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/encoding/msgpack"
	"github.com/dobyte/due/v2/encoding/proto"
	"github.com/dobyte/due/v2/encoding/toml"
	"github.com/dobyte/due/v2/encoding/xml"
	"github.com/dobyte/due/v2/encoding/yaml"
	"github.com/dobyte/due/v2/log"
)

var codecs = make(map[string]Codec)

func init() {
	Register(json.DefaultCodec)
	Register(proto.DefaultCodec)
	Register(toml.DefaultCodec)
	Register(xml.DefaultCodec)
	Register(yaml.DefaultCodec)
	Register(msgpack.DefaultCodec)
}

type Codec interface {
	// Name 编解码器类型
	Name() string
	// Marshal 编码
	Marshal(v any) ([]byte, error)
	// Unmarshal 解码
	Unmarshal(data []byte, v any) error
}

// Register 注册编解码器
func Register(codec Codec) {
	if codec == nil {
		log.Fatal("can't register a invalid codec")
	}

	name := codec.Name()

	if name == "" {
		log.Fatal("can't register a codec without name")
	}

	if _, ok := codecs[name]; ok {
		log.Warnf("the old %s codec will be overwritten", name)
	}

	codecs[name] = codec
}

// Invoke 调用编解码器
func Invoke(name string) Codec {
	codec, ok := codecs[name]
	if !ok {
		log.Fatalf("%s codec is not registered", name)
	}

	return codec
}
