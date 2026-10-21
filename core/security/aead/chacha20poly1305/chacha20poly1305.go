package chacha20poly1305

import (
	"crypto/cipher"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/errors"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// ID is the wire identifier of the suite.
	ID uint8 = 2
	// Name is the suite name.
	Name = "chacha20poly1305"
	// KeySize is the required key size in bytes.
	KeySize = chacha20poly1305.KeySize
)

// Suite is the ChaCha20-Poly1305 suite.
var Suite aead.Suite = suite{}

// suite implements aead.Suite.
type suite struct{}

// ID returns the wire identifier of the suite.
func (suite) ID() uint8 { return ID }

// Name returns the name of the suite.
func (suite) Name() string { return Name }

// New builds a ChaCha20-Poly1305 cipher from key.
func (suite) New(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, errors.New("chacha20poly1305: invalid key size")
	}

	return chacha20poly1305.New(key)
}
