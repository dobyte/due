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
	NID    string // Receiving node. When set, the message is delivered directly to it; otherwise the system locates the user's node and delivers the message there.
	CID    int64  // Connection ID
	UID    int64  // User ID
	Route  int32  // Message route
	Buffer any    // Message to deliver
}

type TriggerArgs struct {
	Event cluster.Event // Event
	CID   int64         // Connection ID
	UID   int64         // User ID
}
