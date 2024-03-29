package protocol

import "github.com/symsimmy/due/session"

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
	Message *Message
}

type PushReply struct {
	Code int
}

type MulticastRequest struct {
	Kind    session.Kind
	Targets []int64
	Message *Message
}

type MulticastReply struct {
	Code  int
	Total int64
}

type BroadcastRequest struct {
	Kind    session.Kind
	Message *Message
}

type BroadcastReply struct {
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

type StatRequest struct {
	Kind session.Kind // 推送类型 1：CID 2：UID
}

type StatReply struct {
	Total int64 // 会话数量
	Code  int
}

type IsOnlineRequest struct {
	Kind   session.Kind // 推送类型 1：CID 2：UID
	Target int64        // 推送目标
}

type IsOnlineReply struct {
	IsOnline bool // 是否在线
	Code     int
}

type GetIdRequest struct {
	Kind   session.Kind // 推送类型 1：CID 2：UID
	Target int64        // 推送目标
}

type GetIdReply struct {
	Id   int64 // 是否在线
	Code int
}
