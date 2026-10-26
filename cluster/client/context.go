package client

import (
	"context"

	"github.com/dobyte/due/v2/core/buffer"
)

// Context is the route context.
//
// It carries the connection and message information used by route handlers. A Context is valid
// only during the route handler call; it is recycled after the handler returns and must not be
// retained.
type Context struct {
	ctx   context.Context // context
	conn  *Conn           // connection
	route int32           // message route
	seq   int32           // message sequence number
	buf   buffer.Buffer   // message data
}

// reset clears the context for reuse.
func (c *Context) reset() {
	c.ctx = nil
	c.conn = nil
	c.route = 0
	c.seq = 0
	c.buf = nil
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
// When session security is enabled the payload has already been decrypted by the receive path.
func (c *Context) Data() any {
	return c.buf
}

// Parse unmarshals the message data into v. It returns an error when unmarshalling fails.
//
// Session security decryption happens once in the receive path, so Parse is idempotent.
func (c *Context) Parse(v any) (err error) {
	if c.conn.client.opts.security != nil {
		return c.conn.client.opts.codec.Unmarshal(c.buf.Bytes(), v)
	} else if c.conn.client.opts.encryptor != nil {
		data, err := c.conn.client.opts.encryptor.Decrypt(c.buf.Bytes())
		if err != nil {
			return err
		}

		return c.conn.client.opts.codec.Unmarshal(data, v)
	} else {
		return c.conn.client.opts.codec.Unmarshal(c.buf.Bytes(), v)
	}
}
