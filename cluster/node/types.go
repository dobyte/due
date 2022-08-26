package node

import (
	"github.com/dobyte/due/session"
)

type GetIPArgs struct {
	GID    string       // 网关ID，会话类型为用户时可忽略此参数
	Kind   session.Kind // 会话类型，session.Conn 或 session.User
	Target int64        // 会话目标，CID 或 UID
}

type PushArgs struct {
	GID     string       // 网关ID，会话类型为用户时可忽略此参数
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Target  int64        // 会话目标，CID 或 UID
	Route   int32        // 路由ID
	Message interface{}  // 消息内容，接收json、proto、[]byte
}

type MulticastArgs struct {
	GID     string       // 网关ID，会话类型为用户时可忽略此参数
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Targets []int64      // 会话目标，CID 或 UID
	Route   int32        // 路由ID
	Message interface{}  // 消息内容，接收json、proto、[]byte
}

type BroadcastArgs struct {
	Kind    session.Kind // 会话类型，session.Conn 或 session.User
	Route   int32        // 路由ID
	Message interface{}  // 消息内容，接收json、proto、[]byte
}

type DeliverArgs struct {
	GID     string      // 来源网关
	NID     string      // 来源节点
	CID     int64       // 连接ID
	UID     int64       // 用户ID
	Route   int32       // 路由ID
	Message interface{} // 消息内容，接收json、proto、[]byte
}
