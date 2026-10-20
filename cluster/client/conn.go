package client

import (
	"net"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// Conn wraps a network connection.
//
// It exposes message pushing, attribute management and basic information querying on top of the
// underlying network connection.
type Conn struct {
	conn        network.Conn                      // underlying network connection
	client      *Client                           // owning client
	security    atomic.Pointer[handshake.Session] // session security state
	handshakeCh chan []byte                       // received handshake frames
}

// ID returns the connection ID.
func (c *Conn) ID() int64 {
	return c.conn.ID()
}

// UID returns the user ID.
func (c *Conn) UID() int64 {
	return c.conn.UID()
}

// Bind binds the given user ID to the connection.
func (c *Conn) Bind(uid int64) {
	c.conn.Bind(uid)
}

// Unbind unbinds the user ID from the connection.
func (c *Conn) Unbind() {
	c.conn.Unbind()
}

// SetAttr sets the attribute value of the given key.
func (c *Conn) SetAttr(key, value any) {
	c.conn.Attr().Set(key, value)
}

// GetAttr returns the attribute value of the given key.
func (c *Conn) GetAttr(key any) value.Value {
	if val, ok := c.conn.Attr().Get(key); ok {
		return value.NewValue(val)
	} else {
		return value.NewValue()
	}
}

// DelAttr deletes the attribute value of the given key.
func (c *Conn) DelAttr(key any) {
	c.conn.Attr().Del(key)
}

// LocalIP returns the local IP address, or an error when it cannot be determined.
func (c *Conn) LocalIP() (string, error) {
	return c.conn.LocalIP()
}

// LocalAddr returns the local address, or an error when it cannot be determined.
func (c *Conn) LocalAddr() (net.Addr, error) {
	return c.conn.LocalAddr()
}

// RemoteIP returns the remote IP address, or an error when it cannot be determined.
func (c *Conn) RemoteIP() (string, error) {
	return c.conn.RemoteIP()
}

// RemoteAddr returns the remote address, or an error when it cannot be determined.
func (c *Conn) RemoteAddr() (net.Addr, error) {
	return c.conn.RemoteAddr()
}

// Push pushes message to the server after encoding and protecting its data.
//
// When session security is enabled, the payload is sealed with the negotiated session key and the
// route and the sequence number are bound by the additional authenticated data.
func (c *Conn) Push(message *cluster.Message) error {
	var (
		err    error
		data   []byte
		output *buffer.Bytes
	)

	if message.Data != nil {
		if v, ok := message.Data.([]byte); ok {
			data = v
		} else {
			if data, err = c.client.opts.codec.Marshal(message.Data); err != nil {
				return err
			}
		}
	}

	if c.client.opts.security != nil {
		session := c.security.Load()
		if session == nil {
			return errors.New("session security is not established")
		}

		output = buffer.MallocBytesWithHeadroom(
			packet.MessageBufferHeadroom(),
			len(data)+session.Overhead(),
		)
		if output == nil {
			output = buffer.MallocBytes(len(data) + session.Overhead())
			if output == nil {
				return errors.ErrMessageTooLarge
			}
		}
		if _, err = session.SealMessage(output.Bytes()[:0], data, handshake.ClientToServer, message.Route, message.Seq); err != nil {
			output.Release()
			return err
		}
	} else {
		if len(data) != 0 && c.client.opts.encryptor != nil {
			if data, err = c.client.opts.encryptor.Encrypt(data); err != nil {
				return err
			}
		}
	}

	var buf buffer.Buffer
	if output != nil {
		buf, err = packet.PackMessageBuffer(message.Route, message.Seq, output)
	} else {
		buf, err = packet.PackMessage(&packet.Message{Seq: message.Seq, Route: message.Route, Buffer: data})
	}
	if err != nil {
		if output != nil {
			output.Release()
		}
		return err
	}

	if err = c.conn.Push(buf); err != nil {
		buf.Release()
		return err
	}

	return nil
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// handshake performs the client side handshake on the connection.
func (c *Conn) handshake() error {
	opts := c.client.opts.security

	client := handshake.NewClient(opts.signer, opts.suites...)

	hello, err := client.Hello()
	if err != nil {
		return err
	}

	if err = c.pushHandshake(hello); err != nil {
		return err
	}

	timer := time.NewTimer(defaultHandshakeTimeout)
	defer timer.Stop()

	select {
	case frame := <-c.handshakeCh:
		session, reply, err := client.Handle(frame)
		if err != nil {
			return err
		}

		// Publish the session before the Finished frame is sent: the gate may push a downlink
		// immediately after confirming, and handleReceive would drop it while the session is
		// still invisible.
		c.security.Store(session)

		if err = c.pushHandshake(reply); err != nil {
			return err
		}

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
