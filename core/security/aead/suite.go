package aead

import "crypto/cipher"

// Suite is a negotiable AEAD algorithm.
type Suite interface {
	// ID returns the ID of the AEAD suite.
	ID() uint8
	// Name returns the name of the AEAD suite.
	Name() string
	// New returns a new AEAD cipher.
	New(key []byte) (cipher.AEAD, error)
}
