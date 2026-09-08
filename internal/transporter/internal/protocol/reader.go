package protocol

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
)

// validateSize 校验帧长度合法性
// size 表示 size 字段之后的字节数；心跳帧为 1（仅header），数据帧至少为 header+route+seq
func validateSize(size uint32) bool {
	if size == def.HeaderBytes {
		return true
	}

	if size < def.MinFrameSize-def.SizeBytes {
		return false
	}

	return uint64(size)+def.SizeBytes <= def.MaxFrameSize
}

// ReadMessage 读取消息
func ReadMessage(reader *bufio.Reader, header *[4]byte) (bool, uint8, uint64, []byte, error) {
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return false, 0, 0, nil, err
	}

	size := binary.BigEndian.Uint32(header[:])
	if !validateSize(size) {
		return false, 0, 0, nil, errors.ErrInvalidMessage
	}

	data := make([]byte, def.SizeBytes+size)
	copy(data[:def.SizeBytes], header[:])

	if _, err := io.ReadFull(reader, data[def.SizeBytes:]); err != nil {
		return false, 0, 0, nil, err
	}

	isHeartbeat, route, seq := ParseBuffer(data)

	return isHeartbeat, route, seq, data, nil
}

// ReadBuffer 以buffer的形式读取消息
func ReaderBuffer(reader *bufio.Reader, header *[4]byte) (buffer.Buffer, error) {
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header[:])
	if !validateSize(size) {
		return nil, errors.ErrInvalidMessage
	}

	buf := buffer.MallocBytes(int(def.SizeBytes + size))
	data := buf.Bytes()

	copy(data[:def.SizeBytes], header[:])

	if _, err := io.ReadFull(reader, data[def.SizeBytes:]); err != nil {
		buf.Release()
		return nil, err
	}

	return buf, nil
}

// ParseBuffer 解析buffer
func ParseBuffer(data []byte) (bool, uint8, uint64) {
	if header := data[def.SizeBytes : def.SizeBytes+def.HeaderBytes][0]; header&def.HeartbeatBit == def.HeartbeatBit {
		return true, 0, 0
	} else {
		var (
			route = data[def.SizeBytes+def.HeaderBytes : def.SizeBytes+def.HeaderBytes+def.RouteBytes][0]
			seq   = binary.BigEndian.Uint64(data[def.SizeBytes+def.HeaderBytes+def.RouteBytes : def.SizeBytes+def.HeaderBytes+def.RouteBytes+8])
		)

		return false, route, seq
	}
}
