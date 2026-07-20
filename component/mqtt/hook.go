package mqtt

import (
	"bytes"

	"github.com/dobyte/due/v2/utils/xcall"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

var _ mqtt.Hook = (*eventHook)(nil)

type eventHook struct {
	mqtt.HookBase
	server *Server
}

// ID 组件ID
func (h *eventHook) ID() string {
	return "due-hook"
}

func (h *eventHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
	}, []byte{b})
}

// OnConnect 连接成功
func (h *eventHook) OnConnect(cli *mqtt.Client, pk packets.Packet) error {
	if cli.Net.Inline {
		return nil
	}

	if handler, ok := h.server.events[Connect]; ok {
		xcall.Call(func() { handler(&Client{cli}) })
	}

	return nil
}

// OnDisconnect 断开连接
func (h *eventHook) OnDisconnect(cli *mqtt.Client, err error, expire bool) {
	if cli.Net.Inline {
		return
	}

	if handler, ok := h.server.events[Disconnect]; ok {
		xcall.Call(func() { handler(&Client{cli}) })
	}
}

// OnSubscribed 订阅成功
func (h *eventHook) OnSubscribed(cli *mqtt.Client, _ packets.Packet, _ []byte) {
	if cli.Net.Inline {
		return
	}

	if handler, ok := h.server.events[Subscribed]; ok {
		xcall.Call(func() { handler(&Client{cli}) })
	}
}

// OnUnsubscribed 取消订阅
func (h *eventHook) OnUnsubscribed(cli *mqtt.Client, _ packets.Packet) {
	if cli.Net.Inline {
		return
	}

	if handler, ok := h.server.events[Unsubscribed]; ok {
		xcall.Call(func() { handler(&Client{cli}) })
	}
}
