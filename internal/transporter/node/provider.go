package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
)

type Provider interface {
	// Trigger 触发事件
	Trigger(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error
	// Deliver 投递消息
	Deliver(ctx context.Context, gid, nid string, cid, uid int64, message []byte) error
	// GetState 获取状态
	GetState() (cluster.State, error)
	// SetState 设置状态
	SetState(state cluster.State) error
}
