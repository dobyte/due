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
	// Name returns the codec type.
	Name() string
	// Marshal encodes v.
	Marshal(v any) ([]byte, error)
	// Unmarshal decodes data into v.
	Unmarshal(data []byte, v any) error
}

// Appender is an optional interface implemented by codecs that can append an encoded value to dst.
// It lets callers encode directly into a pooled buffer. Implementations may reallocate dst when its
// capacity is insufficient; callers detect that by comparing the returned length with the original
// capacity.
type Appender interface {
	// Append appends the encoding of v to dst.
	Append(dst []byte, v any) ([]byte, error)
}

// Register registers a codec.
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

// Invoke returns the codec registered under name, panicking when it is not registered.
func Invoke(name string) Codec {
	codec, ok := codecs[name]
	if !ok {
		log.Fatalf("%s codec is not registered", name)
	}

	return codec
}
