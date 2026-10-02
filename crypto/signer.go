package crypto

import (
	"github.com/dobyte/due/v2/log"
)

type Signer interface {
	// Name returns the name of the signer.
	Name() string
	// Sign signs data.
	Sign(data []byte) ([]byte, error)
	// Verify verifies the signature of data.
	Verify(data []byte, signature []byte) (bool, error)
}

var signers = make(map[string]Signer)

// RegisterSigner registers a signer. It panics when the signer is nil or has an empty name, and
// overwrites the signer registered under the same name.
func RegisterSigner(signer Signer) {
	if signer == nil {
		log.Fatal("can't register a invalid signer")
	}

	name := signer.Name()

	if name == "" {
		log.Fatal("can't register a signer without name")
	}

	if _, ok := signers[name]; ok {
		log.Warnf("the old %s signer will be overwritten", name)
	}

	signers[name] = signer
}

// InvokeSigner returns the signer registered under name. It panics when no signer is registered
// under that name.
func InvokeSigner(name string) Signer {
	signer, ok := signers[name]
	if !ok {
		log.Fatalf("%s signer is not registered", name)
	}

	return signer
}
