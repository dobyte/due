package drpcs

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
	return &reader{reader: bufio.NewReaderSize(conn, 1<<16)}
}

// readBuffer 以buffer的形式读取消息
func (r *reader) readBuffer() (bool, uint8, uint64, *buffer.Bytes, error) {
	if _, err := io.ReadFull(r.reader, r.header[:]); err != nil {
		return false, 0, 0, nil, err
	}

	size := binary.BigEndian.Uint32(r.header[:def.SizeBytes])

	if !r.validateSize(size) {
		return false, 0, 0, nil, errors.ErrInvalidMessage
	}

	if header := r.header[def.SizeBytes:][0]; header&def.HeartbeatBit == def.HeartbeatBit {
		return true, 0, 0, nil, nil
	}

	buf := buffer.MallocBytes(int(size) - def.HeaderBytes)

	if buf == nil {
		return false, 0, 0, nil, errors.ErrMessageTooLarge
	}

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

// validateSize 校验帧长度合法性
// size 表示 size 字段之后的字节数；心跳帧为 1（仅header），数据帧至少为 header+route+seq
func (r *reader) validateSize(size uint32) bool {
	if size == def.HeaderBytes {
		return true
	}

	if size < def.MinFrameSize-def.SizeBytes {
		return false
	}

	return uint64(size)+def.SizeBytes <= def.MaxFrameSize
}
