package aesgcm

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/errors"
)

const (
	// Name is the suite name.
	Name = "aes-gcm"
	// ID is the wire identifier of the suite.
	ID uint8 = 1
	// KeySize is the required key size in bytes.
	KeySize = 32
)

// Suite is the AES-256-GCM suite.
var Suite = &crypto.AEADSuite{
	ID:      ID,
	Name:    Name,
	KeySize: KeySize,
	New:     New,
}

// New builds an AES-256-GCM cipher from key.
func New(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, errors.New("aes-gcm: invalid key size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func init() {
	crypto.RegisterAEADSuite(Suite)
}
