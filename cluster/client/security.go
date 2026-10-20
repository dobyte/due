package client

import (
	"time"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/crypto"
)

// defaultHandshakeTimeout is the time a client waits for the ServerHello.
const defaultHandshakeTimeout = 5 * time.Second

// securityOptions configures the session security between the client and the gate.
type securityOptions struct {
	signer crypto.Signer // verifier of the gate signature
	suites []aead.Suite  // negotiable AEAD suites, empty means the handshake defaults
}
