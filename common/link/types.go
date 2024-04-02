package link

import (
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/session"
)

type GetIPArgs struct {
	GID    string       // 网关ID，会话类型为用户时可忽略此参数
	Kind   session.Kind // 会话类型，session.Conn 或 session.User
	Target int64        // 会话目标，CID 或 UID
}

type IsOnlineArgs struct {
	GID    string       // 网关ID，会话类型为用户时可忽略此参数
	Kind   session.Kind // 会话类型，session.Conn 或 session.User
	Target int64        // 会话目标，CID 或 UID
}

type GetIdArgs struct {
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
}

type MulticastArgs struct {
	GID     string       // 网关ID，会话类型为用户时可忽略此参数
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Targets []int64      // 会话目标，CID 或 UID
	Message *Message     // 消息
}

type BroadcastArgs struct {
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Message *Message     // 消息
}

type DeliverArgs struct {
	NID     string      // 接收节点。存在接收节点时，消息会直接投递给接收节点；不存在接收节点时，系统定位用户所在节点，然后投递。
	CID     int64       // 连接ID
	UID     int64       // 用户ID
	Message interface{} // 消息
}

type TriggerArgs struct {
	Event cluster.Event // 事件
	CID   int64         // 连接ID
	UID   int64         // 用户ID
}

type DisconnectArgs struct {
	GID     string       // 网关ID，会话类型为用户时可忽略此参数
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Target  int64        // 会话目标，CID 或 UID
	IsForce bool         // 是否强制断开
}

type MulticastDeliverArgs struct {
	Kind    DeliverKind
	Targets []string
	Message interface{} // 消息
}

type BroadcastDeliverArgs struct {
	Kind    DeliverKind
	Message interface{} // 消息
}

const (
	Gate DeliverKind = iota + 1
	Center
	Game
)

type DeliverKind int
