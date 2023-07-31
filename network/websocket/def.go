package websocket

import (
	"github.com/gobwas/ws"
)

const (
	closeSig        int = iota // 关闭信号
	dataPacket                 // 数据包
	heartbeatPacket            // 心跳包
)

const (
	TextMessage   = ws.OpText
	BinaryMessage = ws.OpBinary
)

const (
	textMessage   = "text"
	binaryMessage = "binary"
)

type chWrite struct {
	typ int
	msg []byte
}
