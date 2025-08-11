package link

import (
	"github.com/dobyte/due/v2/cluster"
)

type (
	Message         = cluster.Message
	GetIPArgs       = cluster.GetIPArgs
	IsOnlineArgs    = cluster.IsOnlineArgs
	DisconnectArgs  = cluster.DisconnectArgs
	PushArgs        = cluster.PushArgs
	MulticastArgs   = cluster.MulticastArgs
	BroadcastArgs   = cluster.BroadcastArgs
	PublishArgs     = cluster.PublishArgs
	SubscribeArgs   = cluster.SubscribeArgs
	UnsubscribeArgs = cluster.UnsubscribeArgs
)

type DeliverArgs struct {
	NID     string // 接收节点。存在接收节点时，消息会直接投递给接收节点；不存在接收节点时，系统定位用户所在节点，然后投递。
	CID     int64  // 连接ID
	UID     int64  // 用户ID
	Route   int32  // 路由
	Message any    // 消息
}

type TriggerArgs struct {
	Event cluster.Event // 事件
	CID   int64         // 连接ID
	UID   int64         // 用户ID
}
