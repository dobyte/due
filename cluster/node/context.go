package node

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/transport"
)

type Context interface {
	// GID 获取网关ID
	GID() string
	// NID 获取节点ID
	NID() string
	// CID 获取连接ID
	CID() int64
	// UID 获取用户ID
	UID() int64
	// Seq 获取消息序列号
	Seq() int32
	// Route 获取消息路由号
	Route() int32
	// Event 获取事件类型
	Event() cluster.Event
	// Kind 上下文消息类型
	Kind() Kind
	// Parse 解析消息
	Parse(v any) error
	// Defer 添加defer延迟调用栈
	// 此方法功能与go defer一致，作用域也仅限于当前handler处理函数内，推荐使用Defer方法替代go defer使用
	// 区别在于使用Defer方法可以对调用栈进行取消操作
	// 同时，在调用Task和Next方法是会自动取消调用栈
	// 也可通过Cancel方法进行手动取消调用栈
	// bottom用于标识是否挂载到栈底部
	Defer(fn func(), bottom ...bool)
	// Cancel 取消defer调用栈
	Cancel()
	// Clone 克隆Context
	Clone() Context
	// Task 投递任务
	// 调用此方法会自动取消Defer调用栈的所有执行函数
	Task(fn func(ctx Context))
	// Proxy 获取代理API
	Proxy() *Proxy
	// Context 获取上下文
	Context() context.Context
	// SetValue 为上下文设置值
	SetValue(key, val any)
	// GetValue 获取上下文中的值
	GetValue(key any) any
	// GetIP 获取客户端IP
	GetIP() (string, error)
	// Deliver 投递消息给节点处理
	Deliver(args *cluster.DeliverArgs) error
	// Reply 回复消息
	Reply(message *cluster.Message) error
	// Response 响应消息
	Response(message any) error
	// Disconnect 关闭来自网关的连接
	Disconnect(force ...bool) error
	// BindGate 绑定网关
	BindGate(uid ...int64) error
	// UnbindGate 解绑网关
	UnbindGate(uid ...int64) error
	// BindNode 绑定节点
	BindNode(uid ...int64) error
	// UnbindNode 解绑节点
	UnbindNode(uid ...int64) error
	// Subscribe 订阅频道
	Subscribe(channel string, uids ...int64) error
	// Unsubscribe 取消订阅
	Unsubscribe(channel string, uids ...int64) error
	// BindActor 绑定Actor
	BindActor(kind, id string) error
	// UnbindActor 解绑Actor
	UnbindActor(kind string)
	// Next 消息下放
	// 调用此方法会自动取消Defer调用栈的所有执行函数
	Next() error
	// Spawn 衍生出一个新的Actor
	Spawn(creator Creator, opts ...ActorOption) (*Actor, error)
	// Kill 杀死存在的一个Actor
	Kill(kind, id string) bool
	// Actor 获取Actor
	Actor(kind, id string) (*Actor, bool)
	// Invoke 调用函数（线程安全）
	// ctx在全局的处理器中，调用的就是proxy.Invoke
	// ctx在Actor的处理器中，调用的就是actor.Invoke
	Invoke(fn func())
	// AfterFunc 延迟调用，与官方的time.AfterFunc用法一致
	// ctx在全局的处理器中，调用的就是proxy.AfterFunc
	// ctx在Actor的处理器中，调用的就是actor.AfterFunc
	AfterFunc(d time.Duration, f func()) *Timer
	// AfterInvoke 延迟调用（线程安全）
	// ctx在全局的处理器中，调用的就是proxy.AfterInvoke
	// ctx在Actor的处理器中，调用的就是actor.AfterInvoke
	AfterInvoke(d time.Duration, f func()) *Timer
	// NewMeshClient 新建微服务客户端
	// target参数可分为三种种模式:
	// 服务直连模式: 	direct://127.0.0.1:8011
	// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
	// 服务发现模式: 	discovery://service_name
	NewMeshClient(target string) (transport.Client, error)
	// 保存当前Actor
	storeActor(actor *Actor)
	// 增长版本号
	incrVersion() int32
	// 获取版本号
	loadVersion() int32
	// 比对版本号后进行回收对象
	compareVersionRecycle(version int32)
	// 执行defer调用栈
	compareVersionExecDefer(version int32)
}

type Kind int

const (
	Event   Kind = iota // 事件
	Request             // 请求
)
