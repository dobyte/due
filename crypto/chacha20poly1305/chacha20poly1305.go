package chacha20poly1305

import (
	"crypto/cipher"

	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/errors"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// Name is the suite name.
	Name = "chacha20poly1305"
	// ID is the wire identifier of the suite.
	ID uint8 = 2
	// KeySize is the required key size in bytes.
	KeySize = chacha20poly1305.KeySize
)

// Suite is the ChaCha20-Poly1305 suite.
var Suite = &crypto.AEADSuite{
	ID:      ID,
	Name:    Name,
	KeySize: KeySize,
	New:     New,
}

// New builds a ChaCha20-Poly1305 cipher from key.
func New(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, errors.New("chacha20poly1305: invalid key size")
	}

	return chacha20poly1305.New(key)
}

func init() {
	crypto.RegisterAEADSuite(Suite)
}
