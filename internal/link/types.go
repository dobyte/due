package link

import (
	"github.com/dobyte/due/v2/cluster"
)

type (
	Message        = cluster.Message
	GetIPArgs      = cluster.GetIPArgs
	IsOnlineArgs   = cluster.IsOnlineArgs
	DisconnectArgs = cluster.DisconnectArgs
	PushArgs       = cluster.PushArgs
	MulticastArgs  = cluster.MulticastArgs
	BroadcastArgs  = cluster.BroadcastArgs
)

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
}
