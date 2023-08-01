package ws

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xtime"
)

const (
	closeSig        int = iota // 关闭信号
	dataPacket                 // 数据包
	heartbeatPacket            // 心跳包
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
	var opcode uint8
	opcode |= bit0

	if isWithTime {
		buf = make([]byte, 9)
		buf[0] = opcode
		binary.BigEndian.PutUint64(buf[1:], uint64(xtime.Now().UnixMilli()))
	} else {
		buf = make([]byte, 1)
		buf[0] = opcode
	}

	return
}

// 打包消息
func packMessage(msg []byte) (buf []byte) {
	var opcode uint8

	buf = make([]byte, len(msg)+1)
	buf[0] = opcode
	copy(buf[1:], msg)

	return
}

// 解析数据包
func parsePacket(packet []byte) (isHeartbeat bool, msg []byte, err error) {
	if len(packet) == 0 {
		err = errors.New("invalid data packet")
		return
	}

	isHeartbeat = packet[0]&bit0 == bit0

	if !isHeartbeat {
		msg = packet[1:]
	}

	return
}
