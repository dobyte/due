package stream

const (
	connClosed int32 = iota // 连接打开
	connOpened              // 连接关闭
)

type RouteHandler func(conn *ServerConn, data []byte) error
