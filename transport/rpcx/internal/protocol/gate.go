package protocol

import (
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
)

type BindRequest struct {
	CID int64
	UID int64
}

type BindReply struct {
	Code int
}

type UnbindRequest struct {
	UID int64
}

type UnbindReply struct {
	Code int
}

type GetIPRequest struct {
	Kind   session.Kind
	Target int64
}

type GetIPReply struct {
	Code int
	IP   string
}

type PushRequest struct {
	Kind    session.Kind
	Target  int64
	Message *packet.Message
}

type PushReply struct {
	Code int
}

type MulticastRequest struct {
	Kind    session.Kind
	Targets []int64
	Message *packet.Message
}

type MulticastReply struct {
	Code  int
	Total int64
}

type BroadcastRequest struct {
	Kind    session.Kind
	Message *packet.Message
}

type BroadcastReply struct {
	Code  int
	Total int64
}

type StatRequest struct {
	Kind session.Kind
}

type StatReply struct {
	Code  int
	Total int64
}

type DisconnectRequest struct {
	Kind    session.Kind
	Target  int64
	IsForce bool
}

type DisconnectReply struct {
	Code int
}
