package ws

import "github.com/gorilla/websocket"

const (
	closeSig        int = iota // 关闭信号
	dataPacket                 // 数据包
	heartbeatPacket            // 心跳包
)

const (
	TextMessage   = websocket.TextMessage
	BinaryMessage = websocket.BinaryMessage
)

type chWrite struct {
	typ     int
	msg     []byte
	msgType int
}
