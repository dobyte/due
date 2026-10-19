package crypto

import (
	"crypto/cipher"
	"sync"

	"github.com/dobyte/due/v2/log"
)

// AEADFactory builds an AEAD cipher from the given key.
type AEADFactory func(key []byte) (cipher.AEAD, error)

// AEADSuite describes a negotiable AEAD algorithm.
//
// ID is the wire identifier carried by the handshake and must stay stable once released. The
// nonce size of the cipher built by New must be 12 bytes.
type AEADSuite struct {
	ID      uint8
	Name    string
	KeySize int
	New     AEADFactory
}

var (
	aeadSuitesMu sync.RWMutex
	aeadSuites   = make(map[uint8]*AEADSuite)
)

// RegisterAEADSuite registers an AEAD suite. It panics when the suite is invalid, and overwrites
// the suite registered under the same ID.
func RegisterAEADSuite(suite *AEADSuite) {
	if suite == nil || suite.ID == 0 || suite.Name == "" || suite.KeySize <= 0 || suite.New == nil {
		log.Fatal("can't register a invalid aead suite")
	}

	aeadSuitesMu.Lock()

	if _, ok := aeadSuites[suite.ID]; ok {
		log.Warnf("the old %s aead suite will be overwritten", suite.Name)
	}

	aeadSuites[suite.ID] = suite

	aeadSuitesMu.Unlock()
}

// InvokeAEADSuite returns the AEAD suite registered under id and reports whether it exists.
func InvokeAEADSuite(id uint8) (*AEADSuite, bool) {
	aeadSuitesMu.RLock()
	suite, ok := aeadSuites[id]
	aeadSuitesMu.RUnlock()

	return suite, ok
}
