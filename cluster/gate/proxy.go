package gate

import (
	"context"
	"fmt"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/errcode"
	"github.com/symsimmy/due/internal/link"
	"github.com/symsimmy/due/internal/pb"
	"github.com/symsimmy/due/internal/prom"
	"github.com/symsimmy/due/internal/route"
	"github.com/symsimmy/due/internal/util"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/packet"
	"github.com/symsimmy/due/session"
	"strings"
)

var (
	ErrInvalidGID         = link.ErrInvalidGID
	ErrInvalidNID         = link.ErrInvalidNID
	ErrInvalidMessage     = link.ErrInvalidMessage
	ErrInvalidArgument    = link.ErrInvalidArgument
	ErrInvalidSessionKind = link.ErrInvalidSessionKind
	ErrNotFoundUserSource = link.ErrNotFoundUserSource
	ErrReceiveTargetEmpty = link.ErrReceiveTargetEmpty
)

type Proxy struct {
	gate *Gate      // 网关服
	link *link.Link // 连接

}

func newProxy(gate *Gate) *Proxy {
	return &Proxy{gate: gate, link: link.NewLink(&link.Options{
		GID:         gate.opts.id,
		Locator:     gate.opts.locator,
		Registry:    gate.opts.registry,
		Transporter: gate.opts.transporter,
		Codec:       gate.opts.codec,
		Encryptor:   gate.opts.encryptor,
	})}
}

// 绑定用户与网关间的关系
func (p *Proxy) bindGate(ctx context.Context, cid, uid int64) error {
	err := p.gate.opts.locator.Set(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		return err
	}

	p.trigger(ctx, cluster.Reconnect, cid, uid)

	return nil
}

// 解绑用户与网关间的关系
func (p *Proxy) unbindGate(ctx context.Context, cid, uid int64) error {
	err := p.gate.opts.locator.Rem(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		log.Errorf("user unbind failed, gid: %d, cid: %d, uid: %d, err: %v", p.gate.opts.id, cid, uid, err)
	}

	return err
}

// 触发事件
func (p *Proxy) trigger(ctx context.Context, event cluster.Event, cid, uid int64) {
	if err := p.link.Trigger(ctx, &link.TriggerArgs{
		Event: event,
		CID:   cid,
		UID:   uid,
	}); err != nil {
		log.Warnf("trigger event failed, gid: %s, cid: %d, uid: %d, event: %v, err: %v", p.gate.opts.id, cid, uid, event, err)
	}
}

// 投递消息
func (p *Proxy) deliver(ctx context.Context, cid, uid int64, data []byte) {
	message, err := packet.Unpack(data)
	if err != nil {
		log.Errorf("unpack data to struct failed: %v", err)
		return
	}

	// track 收到client消息数量
	prom.GateReceiveClientMsgCountCounter.WithLabelValues(p.GetServerIP(), util.ToString(message.Route)).Inc()

	// track 收到client消息大小
	prom.GateReceiveClientMsgBytesGauge.WithLabelValues(p.GetServerIP(), util.ToString(message.Route)).Set(float64(len(data)))

	if len(p.gate.opts.receiveHook) > 0 {
		for _, f := range p.gate.opts.receiveHook {
			f(ctx, cid, uid, message)
		}
	}

	err = p.link.Deliver(ctx, &link.DeliverArgs{
		CID:     cid,
		UID:     uid,
		Message: message,
	})
	if err != nil {
		log.Warnf("cid:[%+v], uid:[%+v] deliver message[route:%+v] failed: %v,send kickoff user message back to client", cid, uid, message.Route, err)
		// 发送消息失败，往客户端推送一条kickoff的消息
		kickoffNotify := &pb.S2CKickOffPlayerNotify{
			ErrorCode: errcode.Game_server_down_kickoff,
			Uid:       uint64(uid),
			Reason:    fmt.Sprintf("gate deliver message to game server failed.route:%+v,err:%+v", message.Route, err),
		}
		buffer, _ := p.link.ToBuffer(kickoffNotify, true)
		message := &packet.Message{
			Seq:    0,
			Route:  route.S2c_kick_off_player_notify,
			Buffer: buffer,
		}
		data, _ := packet.Pack(message)

		p.gate.session.Push(session.User, uid, data)
	} else {
		// track gate to server
		prom.GateSendGameServerMsgCountCounter.WithLabelValues(util.ToString(message.Route)).Inc()
	}
}

// DeliverN 通过nodeId投递消息给节点处理
func (p *Proxy) deliverN(ctx context.Context, nid string, message *link.Message) error {
	return p.link.Deliver(ctx, &link.DeliverArgs{
		NID:     nid,
		Message: message,
	})
}

// 启动监听
func (p *Proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Node)

	p.link.WatchServiceInstance(ctx, cluster.Node)
}

// GetServerIP 获取GRPC SERVER IP
func (p *Proxy) GetServerIP() string {
	return strings.Split(p.gate.rpc.Endpoint().Address(), ":")[0]
}

// GetServerPort 获取GRPC SERVER Port
func (p *Proxy) GetServerPort() string {
	addr := strings.Split(p.gate.rpc.Endpoint().Address(), ":")
	if len(addr) == 2 {
		return addr[1]
	} else {
		return ""
	}
}

// GetSession 获取Gate持有Session
func (p *Proxy) GetSession() *session.Session {
	return p.gate.session
}

// GetId 获取Gate Id
func (p *Proxy) GetId() string {
	return p.gate.opts.id
}
