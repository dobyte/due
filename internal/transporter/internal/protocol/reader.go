package protocol

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
)

// ReadMessage 读取消息
func ReadMessage(reader *bufio.Reader, header *[4]byte) (bool, uint8, uint64, []byte, error) {
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return false, 0, 0, nil, err
	}

	size := binary.BigEndian.Uint32(header[:])
	if size == 0 {
		return false, 0, 0, nil, errors.ErrInvalidMessage
	}

	data := make([]byte, defaultSizeBytes+size)
	copy(data[:defaultSizeBytes], header[:])

	if _, err := io.ReadFull(reader, data[defaultSizeBytes:]); err != nil {
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
	if size == 0 {
		return nil, errors.ErrInvalidMessage
	}

	buf := buffer.MallocBytes(int(defaultSizeBytes + size))
	data := buf.Bytes()

	copy(data[:defaultSizeBytes], header[:])

	if _, err := io.ReadFull(reader, data[defaultSizeBytes:]); err != nil {
		buf.Release()
		return nil, err
	}

	return buf, nil
}

// ParseBuffer 解析buffer
func ParseBuffer(data []byte) (bool, uint8, uint64) {
	if header := data[defaultSizeBytes : defaultSizeBytes+defaultHeaderBytes][0]; header&heartbeatBit == heartbeatBit {
		return true, 0, 0
	} else {
		var (
			route = data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes][0]
			seq   = binary.BigEndian.Uint64(data[defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes : defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+8])
		)

		return false, route, seq
	}
}
