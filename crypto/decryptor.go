package crypto

import (
	"github.com/dobyte/due/crypto/ecc"
	"github.com/dobyte/due/crypto/rsa"
	"github.com/dobyte/due/log"
)

type Decryptor interface {
	// Name 名称
	Name() string
	// Decrypt 解密
	Decrypt(data []byte) ([]byte, error)
}

var decryptors = make(map[string]Decryptor)

func init() {
	RegisterDecryptor(ecc.DefaultDecryptor)
	RegisterDecryptor(rsa.DefaultDecryptor)
}

// RegisterDecryptor 注册解密器
func RegisterDecryptor(decryptor Decryptor) {
	if decryptor == nil {
		log.Fatal("can't register a invalid decryptor")
	}

	name := decryptor.Name()

	if name == "" {
		log.Fatal("can't register a decryptor without name")
	}

	if _, ok := decryptors[name]; ok {
		log.Warnf("the old %s decryptor will be overwritten", name)
	}

	decryptors[name] = decryptor
}

// InvokeDecryptor 调用解密器
func InvokeDecryptor(name string) Decryptor {
	decryptor, ok := decryptors[name]
	if !ok {
		log.Fatalf("%s decryptor is not registered", name)
	}

	return decryptor
}
