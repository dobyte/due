package node

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// 服务提供者
type provider struct {
	node *Node
}

// Trigger 触发事件
// @param ctx context.Context 上下文
// @param gid string 网关ID
// @param cid int64 连接ID
// @param uid int64 用户ID
// @param event cluster.Event 事件类型
// @return @1 error 节点已关闭时返回ErrNodeShutdown，否则为触发错误
func (p *provider) Trigger(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.node.trigger.trigger(event, gid, cid, uid)
	}
}

// Deliver 投递消息
// @param ctx context.Context 上下文
// @param gid string 网关ID
// @param nid string 节点ID
// @param cid int64 连接ID
// @param uid int64 用户ID
// @param message []byte 消息字节
// @return @1 error 投递失败时返回的错误
func (p *provider) Deliver(ctx context.Context, gid, nid string, cid, uid int64, message []byte) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	}

	msg, err := packet.UnpackMessage(message)
	if err != nil {
		return err
	}

	stateful, ok := p.node.router.CheckRouteStateful(msg.Route)
	if !ok && !p.node.router.HasDefaultRouteHandler() {
		return nil
	}

	if stateful {
		if uid == 0 {
			return errors.ErrInvalidArgument
		}

		if _, ok, err = p.node.proxy.AskNode(ctx, uid, p.node.opts.name, p.node.opts.id); err != nil {
			return err
		}

		if !ok {
			return errors.ErrNotFoundSession
		}
	}

	return p.node.router.deliver(gid, nid, "", cid, uid, msg.Seq, msg.Route, msg.Buffer)
}

// GetState 获取状态
// @return @1 cluster.State 当前节点状态
// @return @2 error 通常返回nil
func (p *provider) GetState() (cluster.State, error) {
	return p.node.getState(), nil
}

// SetState 设置状态
// @param state cluster.State 目标状态
// @return @1 error 状态设置失败时返回的错误
func (p *provider) SetState(state cluster.State) error {
	return p.node.setState(state)
}
