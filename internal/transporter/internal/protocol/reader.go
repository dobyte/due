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
