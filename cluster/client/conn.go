package client

import (
	"net"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// Conn wraps a network connection.
//
// It exposes message pushing, attribute management and basic information querying on top of the
// underlying network connection.
type Conn struct {
	conn   network.Conn // underlying network connection
	client *Client      // owning client
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

// Push pushes message to the server after encoding and encrypting its data.
func (c *Conn) Push(message *cluster.Message) error {
	var (
		err    error
		buffer []byte
	)

	if message.Data != nil {
		if v, ok := message.Data.([]byte); ok {
			buffer = v
		} else {
			if buffer, err = c.client.opts.codec.Marshal(message.Data); err != nil {
				return err
			}
		}

		if c.client.opts.encryptor != nil {
			if buffer, err = c.client.opts.encryptor.Encrypt(buffer); err != nil {
				return err
			}
		}
	}

	buf, err := packet.PackMessage(&packet.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: buffer,
	})
	if err != nil {
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
