package cluster

import "github.com/dobyte/due/v2/internal/link"

const (
	Master Kind = "master" // 管理服
	Gate   Kind = "gate"   // 网关服
	Node   Kind = "node"   // 节点服
	Mesh   Kind = "mesh"   // 微服务
)

// Kind 集群实例类型
type Kind string

func (k Kind) String() string {
	return string(k)
}

const (
	Work State = "work" // 工作（节点正常工作，可以分配更多玩家到该节点）
	Busy State = "busy" // 繁忙（节点资源紧张，不建议分配更多玩家到该节点上）
	Hang State = "hang" // 挂起（节点即将关闭，正处于资源回收中）
	Shut State = "shut" // 关闭（节点已经关闭，无法正常访问该节点）
)

// State 集群实例状态
type State string

func (s State) String() string {
	return string(s)
}

const (
	Connect    Event = iota + 1 // 打开连接
	Reconnect                   // 断线重连
	Disconnect                  // 断开连接
)

// Event 事件
type Event int

func (e Event) String() string {
	switch e {
	case Connect:
		return "connect"
	case Reconnect:
		return "reconnect"
	case Disconnect:
		return "disconnect"
	}

	return ""
}

type (
	GetIPArgs      = link.GetIPArgs
	PushArgs       = link.PushArgs
	MulticastArgs  = link.MulticastArgs
	BroadcastArgs  = link.BroadcastArgs
	DisconnectArgs = link.DisconnectArgs
	Message        = link.Message
)

type DeliverArgs struct {
	NID     string   // 接收节点。存在接收节点时，消息会直接投递给接收节点；不存在接收节点时，系统定位用户所在节点，然后投递。
	UID     int64    // 用户ID
	Message *Message // 消息
}
