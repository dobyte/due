package cluster

const (
	Master Kind = iota // 管理服
	Gate               // 网关服
	Node               // 节点服
)

// Kind 集群实例类型
type Kind int

const (
	Work State = iota + 1 // 工作（节点正常工作，可以分配更多玩家到该节点）
	Busy                  // 繁忙（节点资源紧张，不建议分配更多玩家到该节点上）
	Hang                  // 挂起（节点即将关闭，正处于资源回收中）
	Done                  // 关闭（节点已经关闭，无法正常访问该节点）
)

// State 集群实例状态
type State int

func (k Kind) String() string {
	switch k {
	case Gate:
		return "gate"
	default:
		return "node"
	}
}

const (
	Reconnect  Event = iota + 1 // 断线重连
	Disconnect                  // 断开连接
)

// Event 事件
type Event int
