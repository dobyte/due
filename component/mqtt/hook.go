package mqtt

import (
	"bytes"

	"github.com/dobyte/due/v2/utils/xcall"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

var _ mqtt.Hook = (*hook)(nil)

type hook struct {
	mqtt.HookBase
	server *Server
}

// ID 组件ID
func (h *hook) ID() string {
	return "due-hook"
}

func (h *hook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
		mqtt.OnPublished,
		mqtt.OnPublish,
	}, []byte{b})
}

// OnConnect 连接成功
func (h *hook) OnConnect(cli *mqtt.Client, pk packets.Packet) error {
	if cli.Net.Inline {
		return nil
	}

	if handler, ok := h.server.events[Connect]; ok {
		xcall.Call(func() {
			handler(&Client{cli})
		})
	}

	return nil
}

// OnDisconnect 断开连接
func (h *hook) OnDisconnect(cli *mqtt.Client, err error, expire bool) {
	if cli.Net.Inline {
		return
	}

	if handler, ok := h.server.events[Disconnect]; ok {
		xcall.Call(func() {
			handler(&Client{cli})
		})
	}
}
