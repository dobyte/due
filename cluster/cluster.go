package cluster

import (
	"github.com/dobyte/due/v2/session"
)

const (
	Gate   Kind = iota + 1 // 网关服
	Node                   // 节点服
	Mesh                   // 微服务
	Master                 // 管理服
)

// Kind 集群实例类型
type Kind int

func (k Kind) String() string {
	switch k {
	case Gate:
		return "gate"
	case Node:
		return "node"
	case Mesh:
		return "mesh"
	default:
		return "master"
	}
}

const (
	Shut State = iota // 关闭（节点已经关闭，无法正常访问该节点）
	Work              // 工作（节点正常工作，可以分配更多玩家到该节点）
	Busy              // 繁忙（节点资源紧张，不建议分配更多玩家到该节点上）
	Hang              // 挂起（节点即将关闭，正处于资源回收中）
)

// State 集群实例状态
type State int

func (s State) String() string {
	switch s {
	case Work:
		return "work"
	case Busy:
		return "busy"
	case Hang:
		return "hang"
	default:
		return "shut"
	}
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

const (
	Init    Hook = iota // 初始化组件
	Start               // 启动组件
	Restart             // 重启组件
	Destroy             // 销毁组件
)

// Hook 生命周期钩子
type Hook int

func (h Hook) String() string {
	switch h {
	case Start:
		return "start"
	case Restart:
		return "restart"
	case Destroy:
		return "destroy"
	default:
		return "init"
	}
}

type GetIPArgs struct {
	GID    string       // 网关ID，会话类型为用户时可忽略此参数
	Kind   session.Kind // 会话类型，session.Conn 或 session.User
	Target int64        // 会话目标，CID 或 UID
}

type Message struct {
	Seq   int32       // 序列号
	Route int32       // 路由ID
	Data  interface{} // 消息数据，接收json、proto、[]byte
}

type PushArgs struct {
	GID     string       // 网关ID，会话类型为用户时可忽略此参数
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Target  int64        // 会话目标，CID 或 UID
	Message *Message     // 消息
	Async   bool         // 是否异步；异步推送不会同步等待推送结果，性能更好
}

type MulticastArgs struct {
	GID     string       // 网关ID，会话类型为用户时可忽略此参数
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Targets []int64      // 会话目标，CID 或 UID
	Message *Message     // 消息
	Async   bool         // 是否异步；异步推送不会同步等待推送结果，性能更好
}

type BroadcastArgs struct {
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Message *Message     // 消息
	Async   bool         // 是否异步；异步推送不会同步等待推送结果，性能更好
}

type TriggerArgs struct {
	Event int   // 事件
	CID   int64 // 连接ID
	UID   int64 // 用户ID
	Async bool  // 是否异步
}

type IsOnlineArgs struct {
	GID    string       // 网关ID，会话类型为用户时可忽略此参数
	Kind   session.Kind // 会话类型，session.Conn 或 session.User
	Target int64        // 会话目标，CID 或 UID
}

type DisconnectArgs struct {
	GID    string       // 网关ID，会话类型为用户时可忽略此参数
	Kind   session.Kind // 会话类型，session.Conn 或 session.User
	Target int64        // 会话目标，CID 或 UID
	Force  bool         // 是否强制断开
	Async  bool         // 是否异步；异步断开连接不会同步等断连送结果，性能更好
}

type DeliverArgs struct {
	NID     string   // 接收节点。存在接收节点时，消息会直接投递给接收节点；不存在接收节点时，系统定位用户所在节点，然后投递。
	UID     int64    // 用户ID
	Async   bool     // 是否异步
	Message *Message // 消息
}

//type (
//	GetIPArgs      = link.GetIPArgs
//	PushArgs       = link.PushArgs
//	MulticastArgs  = link.MulticastArgs
//	BroadcastArgs  = link.BroadcastArgs
//	IsOnlineArgs   = link.IsOnlineArgs
//	DisconnectArgs = link.DisconnectArgs
//	Message        = link.Message
//
//)
