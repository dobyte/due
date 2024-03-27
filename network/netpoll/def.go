package netpoll

import (
	"encoding/binary"
	"github.com/cloudwego/netpoll"
)

const sizeBytes = 4
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
	if _, err := writer.WriteBinary(msg); err != nil {
		return err
	}

	return writer.Flush()
}

// 执行读取操作
func read(reader netpoll.Reader) ([]byte, error) {
	buf, err := reader.Peek(sizeBytes)
	if err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(buf)
	//_ = reader.Release()

	if size == 0 {
		return nil, nil
	}

	return reader.Next(int(size) + sizeBytes)
}
