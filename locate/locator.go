/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/18 11:40 上午
 * @Desc: 定位用户所在网关和节点
 */

package locate

import (
	"context"
)

type Locator interface {
	// Name 获取定位器组件名
	Name() string
	// Watch 监听用户定位变化
	Watch(ctx context.Context, kinds ...string) (Watcher, error)
	// BindGate 绑定网关
	BindGate(ctx context.Context, uid int64, gid string) error
	// BindNode 绑定节点
	BindNode(ctx context.Context, uid int64, name, nid string) error
	// UnbindGate 解绑网关
	UnbindGate(ctx context.Context, uid int64, gid string) error
	// UnbindNode 解绑节点
	UnbindNode(ctx context.Context, uid int64, name string, nid string) error
	// LocateGate 定位用户所在网关
	LocateGate(ctx context.Context, uid int64) (string, error)
	// LocateNode 定位用户所在节点
	LocateNode(ctx context.Context, uid int64, name string) (string, error)
}

type Watcher interface {
	// Next 返回用户位置列表
	Next() ([]*Event, error)
	// Stop 停止监听
	Stop() error
}

type Event struct {
	// 用户ID
	UID int64 `json:"uid"`
	// 事件类型
	Type EventType `json:"type"`
	// 实例ID
	InsID string `json:"insID"`
	// 实例类型
	InsKind string `json:"insKind"`
	// 实例名称
	InsName string `json:"insName"`
}

type EventType int

const (
	BindGate   EventType = iota + 1 // 绑定网关
	BindNode                        // 绑定节点
	UnbindGate                      // 解绑网关
	UnbindNode                      // 解绑节点
)
