package protocol

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/errors"
	"io"
	"sync"
)

var sizePool = sync.Pool{New: func() any {
	return make([]byte, 4)
}}

// ReadMessage 读取消息
func ReadMessage(reader io.Reader) (isHeartbeat bool, route uint8, seq uint64, data []byte, err error) {
	buf := sizePool.Get().([]byte)

	if _, err = io.ReadFull(reader, buf); err != nil {
		sizePool.Put(buf)
		return
	}

	size := binary.BigEndian.Uint32(buf)

	if size == 0 {
		sizePool.Put(buf)
		err = errors.ErrInvalidMessage
		return
	}

	data = make([]byte, defaultSizeBytes+size)
	copy(data[:defaultSizeBytes], buf)

	sizePool.Put(buf)

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
