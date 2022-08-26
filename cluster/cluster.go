package cluster

type Name string

type Event int

const (
	Gate Name = "gate" // 网关服
	Node Name = "node" // 节点服
)

const (
	Reconnect  Event = iota + 1 // 断线重连
	Disconnect                  // 断开连接
)
