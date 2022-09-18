package cluster

const (
	Gate Kind = iota + 1 // 网关服
	Node                 // 节点服
)

const (
	Reconnect  Event = iota + 1 // 断线重连
	Disconnect                  // 断开连接
)

type Kind int

func (k Kind) String() string {
	switch k {
	case Gate:
		return "gate"
	default:
		return "node"
	}
}

type Event int
