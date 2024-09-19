package server

type RouteHandler func(conn *Conn, data []byte) error

type chData struct {
	isHeartbeat bool   // 是否心跳
	route       uint8  // 路由
	data        []byte // 数据
}
