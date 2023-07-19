package protocol

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/packet"
)

type TriggerRequest struct {
	Event cluster.Event
	GID   string
	CID   int64
	UID   int64
}

type TriggerReply struct {
	Code int
}

type DeliverRequest struct {
	GID     string
	NID     string
	CID     int64
	UID     int64
	Message *packet.Message
}

type DeliverReply struct {
	Code int
}
