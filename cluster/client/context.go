package client

import (
	"context"

	"github.com/dobyte/due/v2/core/buffer"
)

// Context is the route context.
//
// It carries the connection and message information used by route handlers.
type Context struct {
	ctx   context.Context // context
	conn  *Conn           // connection
	route int32           // message route
	seq   int32           // message sequence number
	buf   buffer.Buffer   // message data
}

// Context returns the context.
func (c *Context) Context() context.Context {
	return c.ctx
}

// CID returns the connection ID.
func (c *Context) CID() int64 {
	return c.conn.ID()
}

// UID returns the user ID.
func (c *Context) UID() int64 {
	return c.conn.UID()
}

// Conn returns the connection.
func (c *Context) Conn() *Conn {
	return c.conn
}

// Seq returns the message sequence number.
func (c *Context) Seq() int32 {
	return c.seq
}

// Route returns the message route.
func (c *Context) Route() int32 {
	return c.route
}

// Data returns the message data.
//
// It returns the raw, undecrypted message content; use [Context.Parse] when the plaintext is
// needed.
func (c *Context) Data() any {
	return c.buf
}

// Parse decrypts the message data and unmarshals it into v. It returns an error when decryption
// or unmarshalling fails.
func (c *Context) Parse(v any) (err error) {
	if c.conn.client.opts.encryptor != nil {
		data, err := c.conn.client.opts.encryptor.Decrypt(c.buf.Bytes())
		if err != nil {
			return err
		}

		return c.conn.client.opts.codec.Unmarshal(data, v)
	} else {
		return c.conn.client.opts.codec.Unmarshal(c.buf.Bytes(), v)
	}
}
