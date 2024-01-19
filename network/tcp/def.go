package tcp

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/utils/xtime"
	"io"
	"net"
)

const (
	sizeBytes = 4
	sizeMax   = int32(^(uint32(0)) >> 1)
)

const (
	closeSig   int = iota // 关闭信号
	dataPacket            // 数据包
)

const (
	bit0 = 1 << (7 - iota)
	bit1
	bit2
	bit3
	bit4
	bit5
	bit6
	bit7
)

type chWrite struct {
	typ int
	msg []byte
}

// 打包心跳
func packHeartbeat(isWithTime bool) (buf []byte) {
	if isWithTime {
		buf = make([]byte, sizeBytes+1+8)
	} else {
		buf = make([]byte, sizeBytes+1)
	}

	s := len(buf) - sizeBytes
	buf[0] = byte(s >> 24)
	buf[1] = byte(s >> 16)
	buf[2] = byte(s >> 8)
	buf[3] = byte(s)
	buf[4] = byte(bit0)

	if isWithTime {
		t := xtime.Now().UnixNano()
		buf[5] = byte(t >> 56)
		buf[6] = byte(t >> 48)
		buf[7] = byte(t >> 40)
		buf[8] = byte(t >> 32)
		buf[9] = byte(t >> 24)
		buf[10] = byte(t >> 16)
		buf[11] = byte(t >> 8)
		buf[12] = byte(t)
	}

	return
}

// 打包消息
func packMessage(msg []byte) []byte {
	s := len(msg) + 1
	buf := make([]byte, sizeBytes+s)
	buf[0] = byte(s >> 24)
	buf[1] = byte(s >> 16)
	buf[2] = byte(s >> 8)
	buf[3] = byte(s)
	copy(buf[5:], msg)

	return buf
}

// 执行读取操作
func read(conn net.Conn) (isHeartbeat bool, msg []byte, err error) {
	buf := make([]byte, sizeBytes)
	if _, err = io.ReadFull(conn, buf); err != nil {
		return
	}

	size := binary.BigEndian.Uint32(buf)
	if size == 0 {
		return
	}

	buf = make([]byte, size)
	if _, err = io.ReadFull(conn, buf); err != nil {
		return
	}

	isHeartbeat = buf[0]&bit0 == bit0

	if !isHeartbeat {
		msg = buf[1:]
	}

	return
}
