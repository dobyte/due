package crypto

import (
	"github.com/dobyte/due/v2/log"
)

type Encryptor interface {
	// Name returns the name of the encryptor.
	Name() string
	// Encrypt encrypts data.
	Encrypt(data []byte) ([]byte, error)
	// Decrypt decrypts data.
	Decrypt(data []byte) ([]byte, error)
}

var encryptors = make(map[string]Encryptor)

// RegisterEncryptor registers an encryptor. It panics when the encryptor is nil or has an empty
// name, and overwrites the encryptor registered under the same name.
func RegisterEncryptor(encryptor Encryptor) {
	if encryptor == nil {
		log.Fatal("can't register a invalid encryptor")
	}

	name := encryptor.Name()

	if name == "" {
		log.Fatal("can't register a encryptor without name")
	}

	if _, ok := encryptors[name]; ok {
		log.Warnf("the old %s encryptor will be overwritten", name)
	}

	encryptors[name] = encryptor
}

// InvokeEncryptor returns the encryptor registered under name. It panics when no encryptor is
// registered under that name.
func InvokeEncryptor(name string) Encryptor {
	encryptor, ok := encryptors[name]
	if !ok {
		log.Fatalf("%s encryptor is not registered", name)
	}

	return encryptor
}
