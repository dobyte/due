package netpoll

import (
	"encoding/binary"
	"github.com/cloudwego/netpoll"
)

const sizeBytes = 2
const sizeMax = int32(^(uint32(0)) >> 1)

const (
	closeSig   int = iota // 关闭信号
	dataPacket            // 数据包
)

type chWrite struct {
	typ int
	msg []byte
}

// 执行写入操作
func write(writer netpoll.Writer, msg []byte) error {
	alloc, err := writer.Malloc(sizeBytes + len(msg))
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint16(alloc, uint16(len(msg)))
	copy(alloc[sizeBytes:], msg)

	return writer.Flush()
}

// 执行读取操作
func read(reader netpoll.Reader) ([]byte, error) {
	buf, err := reader.Next(sizeBytes)
	if err != nil {
		return nil, err
	}

	size := binary.LittleEndian.Uint16(buf)
	//_ = reader.Release()

	if size == 0 {
		return nil, nil
	}

	return reader.Next(int(size))
}
