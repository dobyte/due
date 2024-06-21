package server

const (
	connClosed int32 = iota // 连接打开
	connOpened              // 连接关闭
)

type RouteHandler func(conn *Conn, data []byte) error

type chData struct {
	isHeartbeat bool   // 是否心跳
	route       uint8  // 路由
	data        []byte // 数据
}
