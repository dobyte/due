package types

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/session"
)

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

type DeliverArgs struct {
	NID     string      // 接收节点。存在接收节点时，消息会直接投递给接收节点；不存在接收节点时，系统定位用户所在节点，然后投递。
	CID     int64       // 连接ID
	UID     int64       // 用户ID
	Route   int32       // 路由
	Message interface{} // 消息
	Async   bool        // 是否异步；异步投递不会同步等待投递结果，性能更好
}

type TriggerArgs struct {
	Event cluster.Event // 事件
	CID   int64         // 连接ID
	UID   int64         // 用户ID
	Async bool          // 是否异步
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
