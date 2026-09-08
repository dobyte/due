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

type Reader struct {
	reader *bufio.Reader
	header [def.SizeBytes]byte
}

func NewReader(conn *net.TCPConn) *Reader {
	return &Reader{reader: bufio.NewReaderSize(conn, 1<<16)}
}

// ReadBuffer 以buffer的形式读取消息
func (r *Reader) ReadBuffer() (buffer.Buffer, error) {
	if _, err := io.ReadFull(r.reader, r.header[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(r.header[:])

	if !r.validateSize(size) {
		return nil, errors.ErrInvalidMessage
	}

	buf := buffer.MallocBytes(def.SizeBytes + int(size))
	data := buf.Bytes()

	copy(data[:def.SizeBytes], r.header[:])

	if _, err := io.ReadFull(r.reader, data[def.SizeBytes:]); err != nil {
		buf.Release()
		return nil, err
	}

	return buf, nil
}

// validateSize 校验帧长度合法性
// size 表示 size 字段之后的字节数；心跳帧为 1（仅header），数据帧至少为 header+route+seq
func (r *Reader) validateSize(size uint32) bool {
	if size == def.HeaderBytes {
		return true
	}

	if size < def.MinFrameSize-def.SizeBytes {
		return false
	}

	return uint64(size)+def.SizeBytes <= def.MaxFrameSize
}
