package client

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

type Conn struct {
	conn   network.Conn
	client *Client
}

// Push 推送消息
func (c *Conn) Push(message *cluster.Message) error {
	var (
		err    error
		buffer []byte
	)

	if v, ok := message.Data.([]byte); ok {
		buffer = v
	} else {
		buffer, err = c.client.opts.codec.Marshal(message.Data)
		if err != nil {
			return err
		}
	}

	if c.client.opts.encryptor != nil {
		buffer, err = c.client.opts.encryptor.Encrypt(buffer)
		if err != nil {
			return err
		}
	}

	msg, err := packet.Pack(&packet.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: buffer,
	})
	if err != nil {
		return err
	}

	return c.conn.Push(msg)
}

// Close 关闭连接
func (c *Conn) Close() error {
	return c.conn.Close()
}
