package protocol

import (
	"encoding/binary"
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
)

// ReadMessage 读取消息
func ReadMessage(reader io.Reader) (isHeartbeat bool, route uint8, seq uint64, data []byte, err error) {
	buf := buffer.MallocBytes(defaultSizeBytes)
	defer buf.Release()

	if _, err = io.ReadFull(reader, buf.Bytes()); err != nil {
		return
	}

	size := binary.BigEndian.Uint32(buf.Bytes())

	if size == 0 {
		err = errors.ErrInvalidMessage
		return
	}

	data = make([]byte, defaultSizeBytes+size)
	copy(data[:defaultSizeBytes], buf.Bytes())

	if _, err = io.ReadFull(reader, data[defaultSizeBytes:]); err != nil {
		return
	}

	header := data[defaultSizeBytes : defaultSizeBytes+defaultHeaderBytes][0]

	isHeartbeat = header&heartbeatBit == heartbeatBit

	if isHeartbeat {
		return
	}

	route = data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes][0]

	seq = binary.BigEndian.Uint64(data[defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes : defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+8])

	return
}

// ReadBuffer 以buffer的形式读取消息
func ReaderBuffer(reader io.Reader) (buffer.Buffer, error) {
	buf1 := buffer.MallocBytes(defaultSizeBytes)
	defer buf1.Release()

	if _, err := io.ReadFull(reader, buf1.Bytes()); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(buf1.Bytes())

	if size == 0 {
		return nil, errors.ErrInvalidMessage
	}

	buf2 := buffer.MallocBytes(int(defaultSizeBytes + size))
	data := buf2.Bytes()

	copy(data[:defaultSizeBytes], buf1.Bytes())

	if _, err := io.ReadFull(reader, data[defaultSizeBytes:]); err != nil {
		buf2.Release()
		return nil, err
	}

	return buf2, nil
}

// ParseBuffer 解析buffer
func ParseBuffer(buf buffer.Buffer) (isHeartbeat bool, route uint8, seq uint64) {
	data := buf.Bytes()
	header := data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes][0]

	isHeartbeat = header&heartbeatBit == heartbeatBit

	if isHeartbeat {
		return
	}

	route = data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes][0]
	seq = binary.BigEndian.Uint64(data[defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes : defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+8])

	return
}
