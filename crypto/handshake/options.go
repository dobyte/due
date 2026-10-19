package handshake

import (
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/crypto/aesgcm"
	"github.com/dobyte/due/v2/crypto/chacha20poly1305"
)

// Option is a handshake option.
type Option func(o *options)

type options struct {
	// Negotiable AEAD suites, ordered by local preference.
	suites []*crypto.AEADSuite
}

// defaultOptions returns the default handshake options.
func defaultOptions() *options {
	return &options{
		suites: []*crypto.AEADSuite{aesgcm.Suite, chacha20poly1305.Suite},
	}
}

// WithSuites sets the AEAD suites used by the handshake. The order is the local preference, and
// the server selects the first of its suites that the client also offers.
func WithSuites(suites ...*crypto.AEADSuite) Option {
	return func(o *options) {
		if len(suites) > 0 {
			o.suites = suites
		}
	}
}
