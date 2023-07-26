package tcp

import (
	"encoding/binary"
	"io"
	"net"
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
func write(conn net.Conn, msg []byte) error {
	buf := make([]byte, sizeBytes+len(msg))

	binary.BigEndian.PutUint32(buf, uint32(len(msg)))
	copy(buf[sizeBytes:], msg)

	_, err := conn.Write(buf)
	return err
}

// 执行读取操作
func read(conn net.Conn) ([]byte, error) {
	buf := make([]byte, sizeBytes)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(buf)
	if size == 0 {
		return nil, nil
	}

	buf = make([]byte, size)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}

	return buf, nil
}
