package aesgcm

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/errors"
)

const (
	// ID is the wire identifier of the suite.
	ID uint8 = 1
	// Name is the suite name.
	Name = "aes-gcm"
	// KeySize is the required key size in bytes.
	KeySize = 32
)

// Suite is the AES-256-GCM suite.
var Suite aead.Suite = suite{}

// suite implements aead.Suite.
type suite struct{}

// ID returns the wire identifier of the suite.
func (suite) ID() uint8 { return ID }

// Name returns the name of the suite.
func (suite) Name() string { return Name }

// New builds an AES-256-GCM cipher from key.
func (suite) New(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, errors.New("aes-gcm: invalid key size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}
