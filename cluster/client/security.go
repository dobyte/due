package client

import (
	"time"

	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/crypto/handshake"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// defaultHandshakeTimeout is the time a client waits for the ServerHello.
const defaultHandshakeTimeout = 5 * time.Second

// securityOptions configures the session security between the client and the gate.
type securityOptions struct {
	verifier crypto.Verifier    // verifier of the gate signature
	opts     []handshake.Option // handshake options
	timeout  time.Duration      // handshake timeout
}

// securityState is the security state of a single connection.
type securityState struct {
	session *handshake.Session
}

// handshake performs the client side handshake on the connection.
func (c *Conn) handshake() error {
	opts := c.client.opts.security

	client := handshake.NewClient(opts.verifier, opts.opts...)

	hello, err := client.Hello()
	if err != nil {
		return err
	}

	if err = c.pushHandshake(hello); err != nil {
		return err
	}

	timeout := opts.timeout
	if timeout <= 0 {
		timeout = defaultHandshakeTimeout
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case frame := <-c.handshakeCh:
		session, reply, err := client.Handle(frame)
		if err != nil {
			return err
		}

		if err = c.pushHandshake(reply); err != nil {
			return err
		}

		c.security.Store(&securityState{session: session})

		return nil
	case <-timer.C:
		return errors.ErrDeadlineExceeded
	case <-c.client.ctx.Done():
		return c.client.ctx.Err()
	}
}

// pushHandshake pushes a plaintext handshake frame to the server.
func (c *Conn) pushHandshake(frame []byte) error {
	buf, err := packet.PackHandshake(frame)
	if err != nil {
		return err
	}

	if err = c.conn.Push(buf); err != nil {
		buf.Release()
		return err
	}

	return nil
}

// deliverHandshake hands a received handshake frame to the pending handshake.
func (c *Conn) deliverHandshake(frame []byte) {
	if c.handshakeCh == nil {
		return
	}

	select {
	case c.handshakeCh <- append([]byte(nil), frame...):
	default:
	}
}
