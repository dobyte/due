package drpc

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
)

type reader struct {
	reader *bufio.Reader
	header [def.SizeBytes + def.HeaderBytes]byte
}

func newReader(conn *net.TCPConn) *reader {
	return &reader{reader: bufio.NewReaderSize(conn, 1<<13)}
}

// read 以buffer的形式读取消息
func (r *reader) read() (bool, uint8, uint64, *buffer.Bytes, error) {
	if _, err := io.ReadFull(r.reader, r.header[:]); err != nil {
		return false, 0, 0, nil, err
	}

	size := binary.BigEndian.Uint32(r.header[:def.SizeBytes])
	header := r.header[def.SizeBytes:][0]

	if header&def.HeartbeatBit == def.HeartbeatBit {
		if size != def.HeaderBytes {
			return false, 0, 0, nil, errors.ErrInvalidMessage
		}
		return true, 0, 0, nil, nil
	}

	if size < def.MinFrameSize-def.SizeBytes {
		return false, 0, 0, nil, errors.ErrInvalidMessage
	}

	if uint64(size)+def.SizeBytes > def.MaxFrameSize {
		return false, 0, 0, nil, errors.ErrMessageTooLarge
	}

	buf := buffer.MallocBytes(int(size) - def.HeaderBytes)

	if _, err := io.ReadFull(r.reader, buf.Bytes()); err != nil {
		buf.Release()
		return false, 0, 0, nil, err
	}

	var (
		data  = buf.Bytes()
		route = data[:def.RouteBytes][0]
		seq   = binary.BigEndian.Uint64(data[def.RouteBytes : def.RouteBytes+def.SeqBytes])
	)

	buf.MoveTo(def.RouteBytes + def.SeqBytes)

	return false, route, seq, buf, nil
}
