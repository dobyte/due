package tcp

import (
	"encoding/binary"
	"errors"
	"github.com/panjf2000/gnet/v2"
)

var ErrIncompletePacket = errors.New("incomplete packet")

const (
	bodySize = 4
)

func init() {
}

// SimpleCodec Protocol format:
//
// * 0                                  4
// * +-----------+-----------------------+
// * |         body len                  |
// * |      methodName |
// * |         message id                |
// * +-----------+-----------+-----------+
// * |                                   |
// * +                                   +
// * |           body bytes              |
// * +                                   +
// * |            ... ...                |
// * +-----------------------------------+
type SimpleCodec struct{}

type CommonResponse struct {
	Route     uint16
	MessageId uint32
	Data      []byte
}

func (codec SimpleCodec) Encode(route uint16, messageId uint32, buf []byte) ([]byte, error) {
	bodyOffset := bodySize
	dataLen := len(buf) + 6
	msgLen := bodyOffset + dataLen

	data := make([]byte, msgLen)

	binary.LittleEndian.PutUint32(data[:bodyOffset], uint32(dataLen))
	binary.LittleEndian.PutUint16(data[bodyOffset:bodyOffset+2], route)
	binary.LittleEndian.PutUint32(data[bodyOffset+2:bodyOffset+6], messageId)
	copy(data[bodyOffset+6:msgLen], buf)
	return data, nil
}

func (codec *SimpleCodec) Decode(c gnet.Conn) (*CommonResponse, error) {
	bodyOffset := bodySize
	buf, _ := c.Peek(bodyOffset)
	if len(buf) < bodyOffset {
		return nil, ErrIncompletePacket
	}

	bodyLen := binary.LittleEndian.Uint32(buf[:bodyOffset])
	msgLen := bodyOffset + int(bodyLen)
	if c.InboundBuffered() < msgLen {
		return nil, ErrIncompletePacket
	}
	if bodyLen <= 0 {
		// 收到 ping 包
		_, _ = c.Discard(bodyOffset)
		return nil, ErrIncompletePacket
	}
	buf, _ = c.Peek(msgLen)
	_, _ = c.Discard(msgLen)

	route := binary.LittleEndian.Uint16(buf[bodyOffset : bodyOffset+2])
	messageId := binary.LittleEndian.Uint32(buf[bodyOffset+2 : bodyOffset+6])

	return &CommonResponse{
		Route:     route,
		MessageId: messageId,
		Data:      buf[bodyOffset+6 : msgLen],
	}, nil
}

func (codec SimpleCodec) Unpack(buf []byte) ([]byte, error) {
	bodyOffset := bodySize
	if len(buf) < bodyOffset {
		return nil, ErrIncompletePacket
	}

	bodyLen := binary.LittleEndian.Uint32(buf[:bodyOffset])
	msgLen := bodyOffset + int(bodyLen)
	if len(buf) < msgLen {
		return nil, ErrIncompletePacket
	}

	return buf[bodyOffset:msgLen], nil
}
