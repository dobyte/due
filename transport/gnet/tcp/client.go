package tcp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/transport"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const network = "tcp"
const maxMsgSize = 65536
const connectTimeout = 3

type WriteChPacket struct {
	messageId  uint32
	methodName uint16
	req        proto.Message
	ch         chan []byte
	needReply  bool
}

type ReplyPacket struct {
	MessageId  uint32
	MethodName uint16
	ReplyCh    chan []byte
	Req        proto.Message
}

type Client struct {
	client            net.Conn
	reader            *bufio.Reader
	messageId         uint32
	messageMap        sync.Map
	Connected         bool
	Target            string
	DisconnectHandler transport.DisconnectHandler
	writeCh           chan *WriteChPacket
}

func NewClient(target string) (*Client, error) {
	c := &Client{}
	c.Target = target
	tcpClient, err := net.DialTimeout(network, target, connectTimeout*time.Second)
	if err != nil {
		return nil, err
	}
	c.client = tcpClient
	c.reader = bufio.NewReaderSize(tcpClient, maxMsgSize)
	c.Connected = true
	c.writeCh = make(chan *WriteChPacket, 4096)

	go c.sendTask()
	go c.pingTask()
	go c.receiveTask()

	return c, nil
}

func (c *Client) pingTask() {
	for {
		pingBytes := make([]byte, 10)
		messageId := c.Next()
		binary.LittleEndian.PutUint32(pingBytes[:4], uint32(6))
		binary.LittleEndian.PutUint16(pingBytes[4:6], 0)
		binary.LittleEndian.PutUint32(pingBytes[6:10], messageId)
		_, err := c.client.Write(pingBytes)
		time.Sleep(30 * time.Second)
		if err != nil {
			c.ForceClose(err)
			break
		}
	}
	log.Warnf("client[%+v -> %+v] send ping task failed.stop sending ping packet", c.client.LocalAddr(), c.client.RemoteAddr())
}

func (c *Client) sendTask() {
	for {
		select {
		case packet, ok := <-c.writeCh:
			if !ok {
				return
			}

			if packet.needReply {
				c.messageMap.Store(packet.messageId, packet.ch)
			}

			err := c.send(packet.messageId, packet.methodName, packet.req)
			if err != nil {
				log.Warnf("messageId:%+v, methodName:%+v,send message failed,err:%+v", packet.messageId, packet.methodName, err)
				continue
			}
		}
	}
	log.Warnf("client[%+v -> %+v] send task failed.stop sending packet", c.client.LocalAddr(), c.client.RemoteAddr())
}

func (c *Client) ForceClose(err error) {
	c.Connected = false
	c.DisconnectHandler(c.Target)
	log.Warnf("client[%+v -> %+v] close. err:%+v", c.client.LocalAddr(), c.client.RemoteAddr(), err)
}

func (c *Client) Send(methodName uint16, req proto.Message) (err error) {
	messageId := c.Next()
	c.writeCh <- &WriteChPacket{
		messageId:  messageId,
		methodName: methodName,
		req:        req,
		needReply:  false,
	}

	return nil
}

func (c *Client) SendWithReply(methodName uint16, req proto.Message) (replyPacket *ReplyPacket, err error) {
	defer func() {
		if err := recover(); err != nil {
			return
		}
	}()

	id := c.Next()
	replyCh := make(chan []byte, 1)

	replyPacket = &ReplyPacket{
		MessageId:  id,
		MethodName: methodName,
		ReplyCh:    replyCh,
		Req:        req,
	}

	c.writeCh <- &WriteChPacket{
		messageId:  id,
		methodName: methodName,
		req:        req,
		ch:         replyCh,
		needReply:  true,
	}

	return replyPacket, err
}

func (c *Client) send(messageId uint32, methodName uint16, req proto.Message) (err error) {
	msg, err := proto.Marshal(req)
	if err != nil {
		return
	}

	msgLen := len(msg)
	dataLen := msgLen + 10
	data := make([]byte, dataLen)

	binary.LittleEndian.PutUint32(data[:4], uint32(msgLen+6))
	binary.LittleEndian.PutUint16(data[4:6], methodName)
	binary.LittleEndian.PutUint32(data[6:10], messageId)
	copy(data[10:], msg)
	_, err = c.client.Write(data)

	if err != nil {
		c.ForceClose(err)
		log.Warnf("client[%+v -> %+v] send method[%+v] request:%+v failed", c.client.LocalAddr(), c.client.RemoteAddr(), methodName, req)
	}
	return
}

func (c *Client) receiveTask() {
	for {
		select {
		default:
			methodName, messageId, data, err := c.Receive()
			if err != nil {
				log.Warnf("read message failed: %v", err)
				c.ForceClose(err)
				return
			}

			log.Debugf("receive message.methodName:%+v,messageId:%+v,data:%+v", methodName, messageId, data)
			if v, ok := c.messageMap.Load(messageId); ok {
				ch := v.(chan []byte)
				ch <- data
			} else {
				log.Warnf("messageId[%+v] receive channel not exists.", messageId)
			}

			c.messageMap.Delete(messageId)
		}
	}
}

// Receive 读取连接数据
func (c *Client) Receive() (methodName uint16, messageId uint32, data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	packet := make([]byte, 4)
	if _, err = io.ReadFull(c.reader, packet); err != nil {
		return
	}

	var (
		buf    = bytes.NewBuffer(packet)
		msgLen uint16
	)

	if err = binary.Read(buf, binary.LittleEndian, &msgLen); err != nil {
		return
	}

	if msgLen > 0 {
		msg := make([]byte, msgLen)
		if _, err = io.ReadFull(c.reader, msg); err != nil {
			return
		}

		methodName = binary.LittleEndian.Uint16(msg[:2])
		messageId = binary.LittleEndian.Uint32(msg[2:6])
		data = msg[6:]
		if err != nil {
			return 0, 0, nil, err
		}
	}

	return
}

func (c *Client) Next() uint32 {
	return atomic.AddUint32(&c.messageId, 1)
}
