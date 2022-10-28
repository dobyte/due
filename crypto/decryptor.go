package crypto

import (
	"github.com/dobyte/due/log"
	"strings"
)

type Decryptor interface {
	// Name 名称
	Name() string
	// Decrypt 解密
	Decrypt(data []byte) ([]byte, error)
}

var decryptors = make(map[string]Decryptor)

// RegistryDecryptor 注册解密器
func RegistryDecryptor(decryptor Decryptor) {
	if decryptor == nil {
		log.Fatal("can't register a invalid decryptor")
	}

	name := strings.ToLower(decryptor.Name())

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
