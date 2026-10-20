// Package profile defines the shared protocol of the encrypted WS benchmark.
package profile

import (
	"fmt"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/aead/chacha20poly1305"
	"github.com/dobyte/due/v2/packet"
)

const (
	Greet        = 1
	PayloadLimit = 64 * 1024
	// JSON adds 14 bytes to the ASCII message; the security envelope adds 25.
	MaxMessageSize = PayloadLimit - 14 - 25
)

type Greeting struct {
	Message string `json:"message"`
}

// ConfigurePacket keeps all three processes on the same framing settings.
func ConfigurePacket() {
	packet.SetPacker(packet.NewPacker(
		packet.WithRouteBytes(4),
		packet.WithSeqBytes(4),
		packet.WithBufferBytes(PayloadLimit),
	))
}

// Suite selects exactly one suite so the reported algorithm is unambiguous.
func Suite(name string) (aead.Suite, error) {
	switch name {
	case "aesgcm":
		return aesgcm.Suite, nil
	case "chacha20poly1305":
		return chacha20poly1305.Suite, nil
	default:
		return nil, fmt.Errorf("unknown suite %q (use aesgcm or chacha20poly1305)", name)
	}
}
