package node

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/transport"
)

// Context 消息上下文
// 封装网关、节点、连接、用户等基础信息，并提供路由处理、消息投递与Actor操作等系列方法
type Context interface {
	// GID 获取来源网关ID
	// @return @1 string 来源网关ID
	GID() string
	// NID 获取来源节点ID
	// @return @1 string 来源节点ID
	NID() string
	// CID 获取连接ID
	// @return @1 int64 连接ID
	CID() int64
	// UID 获取用户ID
	// @return @1 int64 用户ID
	UID() int64
	// Seq 获取消息序列号
	// @return @1 int32 消息序列号
	Seq() int32
	// Route 获取消息路由号
	// @return @1 int32 消息路由号
	Route() int32
	// Event 获取事件类型
	// @return @1 cluster.Event 事件类型
	Event() cluster.Event
	// Kind 上下文消息类型
	// @return @1 Kind 消息类型（事件或请求）
	Kind() Kind
	// Parse 解析消息
	// 默认反序列化目标对象；存在加密器与来源网关时先解密再反序列化
	// @param v any 解析结果目标对象
	// @return @1 error 解密或反序列化失败时返回的错误
	Parse(v any) error
	// Defer 添加defer延迟调用栈
	// 此方法功能与go defer一致，作用域也仅限于当前handler处理函数内，推荐使用Defer方法替代go defer使用
	// 区别在于使用Defer方法可以对调用栈进行取消操作
	// 同时，在调用Task和Next方法是会自动取消调用栈
	// 也可通过Cancel方法进行手动取消调用栈
	// @param fn func() 待加入调用栈的函数
	// @param bottom ...bool 是否挂载到栈底部，默认挂载到栈顶
	Defer(fn func(), bottom ...bool)
	// Cancel 取消defer调用栈
	Cancel()
	// Clone 克隆Context
	// 克隆出一个新的独立上下文，用于将消息投递给其他Actor
	// @return @1 Context 克隆后的上下文
	Clone() Context
	// Task 投递任务
	// 推荐使用此方法替代task.Add和go func，调用此方法会自动取消Defer调用栈的所有执行函数
	// @param fn func(ctx Context) 待执行的任务函数
	Task(fn func(ctx Context))
	// Proxy 获取代理API
	// @return @1 *Proxy 节点代理
	Proxy() *Proxy
	// Context 获取上下文
	// @return @1 context.Context 底层标准上下文
	Context() context.Context
	// SetValue 为上下文设置值
	// @param key any 键
	// @param val any 值
	SetValue(key, val any)
	// GetValue 获取上下文中的值
	// @param key any 键
	// @return @1 any 键对应的值，不存在时返回nil
	GetValue(key any) any
	// GetIP 获取客户端IP
	// @return @1 string 客户端IP
	// @return @2 error 来源非法或获取失败时返回的错误
	GetIP() (string, error)
	// Deliver 投递消息给节点处理
	// @param args *cluster.DeliverArgs 投递参数
	// @return @1 error 投递失败时返回的错误
	Deliver(args *cluster.DeliverArgs) error
	// Reply 回复消息
	// 根据消息来源（网关、Actor或节点）将消息回传至对应目标
	// @param message *cluster.Message 待回复的消息
	// @return @1 error 回复失败时返回的错误
	Reply(message *cluster.Message) error
	// Response 响应消息
	// 以响应方式回复当前路由消息，路由号与序列号沿用原请求
	// @param message any 响应内容
	// @return @1 error 回复失败时返回的错误
	Response(message any) error
	// Disconnect 关闭来自网关的连接
	// @param force ...bool 是否强制断开，默认非强制
	// @return @1 error 来源非法或断开失败时返回的错误
	Disconnect(force ...bool) error
	// BindGate 绑定网关
	// 未指定uid时使用当前上下文用户的uid
	// @param uid ...int64 待绑定的用户ID
	// @return @1 error 用户ID非法或绑定失败时返回的错误
	BindGate(uid ...int64) error
	// UnbindGate 解绑网关
	// 未指定uid时使用当前上下文用户的uid
	// @param uid ...int64 待解绑的用户ID
	// @return @1 error 用户ID非法或解绑失败时返回的错误
	UnbindGate(uid ...int64) error
	// BindNode 绑定节点
	// 未指定uid时使用当前上下文用户的uid
	// @param uid ...int64 待绑定的用户ID
	// @return @1 error 用户ID非法或绑定失败时返回的错误
	BindNode(uid ...int64) error
	// UnbindNode 解绑节点
	// 未指定uid时使用当前上下文用户的uid
	// @param uid ...int64 待解绑的用户ID
	// @return @1 error 用户ID非法或解绑失败时返回的错误
	UnbindNode(uid ...int64) error
	// Subscribe 订阅频道
	// 指定uids时订阅用户频道，否则订阅当前连接的频道
	// @param channel string 频道名
	// @param uids ...int64 待订阅的用户ID
	// @return @1 error 订阅失败时返回的错误
	Subscribe(channel string, uids ...int64) error
	// Unsubscribe 取消订阅
	// 指定uids时退订用户频道，否则退订当前连接的频道
	// @param channel string 频道名
	// @param uids ...int64 待退订的用户ID
	// @return @1 error 退订失败时返回的错误
	Unsubscribe(channel string, uids ...int64) error
	// BindActor 绑定Actor
	// 将当前用户与指定Actor建立绑定关系
	// @param kind string Actor类型
	// @param id string Actor编号
	// @return @1 error 绑定失败时返回的错误
	BindActor(kind, id string) error
	// UnbindActor 解绑Actor
	// @param kind string Actor类型
	UnbindActor(kind string)
	// Next 消息下放
	// 将当前消息下放到节点的调度器进行二次分发，调用此方法会自动取消Defer调用栈的所有执行函数
	// @return @1 error 分发失败时返回的错误
	Next() error
	// Spawn 衍生出一个新的Actor
	// @param creator Creator Actor处理器创建函数
	// @param opts ...ActorOption Actor配置项
	// @return @1 *Actor 衍生出的Actor实例
	// @return @2 error Actor已存在或创建失败时返回的错误
	Spawn(creator Creator, opts ...ActorOption) (*Actor, error)
	// Kill 杀死存在的一个Actor
	// @param kind string Actor类型
	// @param id string Actor编号
	// @return @1 bool 是否成功杀死
	Kill(kind, id string) bool
	// Actor 获取Actor
	// @param kind string Actor类型
	// @param id string Actor编号
	// @return @1 *Actor Actor实例
	// @return @2 bool Actor是否存在
	Actor(kind, id string) (*Actor, bool)
	// Invoke 调用函数（线程安全）
	// ctx在全局的处理器中，调用的就是proxy.Invoke
	// ctx在Actor的处理器中，调用的就是actor.Invoke
	// @param fn func() 待调用的函数
	// @param isBlock ...bool 是否阻塞调用，默认非阻塞
	// @return @1 error 任务入队失败时返回的错误
	Invoke(fn func(), isBlock ...bool) error
	// AfterFunc 延迟调用，与官方的time.AfterFunc用法一致
	// ctx在全局的处理器中，调用的就是proxy.AfterFunc
	// ctx在Actor的处理器中，调用的就是actor.AfterFunc
	// @param d time.Duration 延迟时长
	// @param f func() 待调用的函数
	// @return @1 *Timer 定时器，可通过Stop取消
	// @return @2 error 创建失败时返回的错误
	AfterFunc(d time.Duration, f func()) (*Timer, error)
	// AfterInvoke 延迟调用（线程安全）
	// 延迟后通过任务队列串行执行函数，保证线程安全
	// ctx在全局的处理器中，调用的就是proxy.AfterInvoke
	// ctx在Actor的处理器中，调用的就是actor.AfterInvoke
	// @param d time.Duration 延迟时长
	// @param f func() 待调用的函数
	// @return @1 *Timer 定时器，可通过Stop取消
	// @return @2 error 创建失败时返回的错误
	AfterInvoke(d time.Duration, f func()) (*Timer, error)
	// NewMeshClient 新建微服务客户端
	// target参数可分为三种种模式:
	// 服务直连模式: 	direct://127.0.0.1:8011
	// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
	// 服务发现模式: 	discovery://service_name
	// @param target string 微服务目标地址
	// @return @1 transport.Client 微服务客户端
	// @return @2 error 创建失败时返回的错误
	NewMeshClient(target string) (transport.Client, error)
	// 保存当前Actor
	// @param actor *Actor 当前Actor
	storeActor(actor *Actor)
	// 删除当前Actor
	deleteActor()
	// 增长版本号
	// @return @1 int32 自增后的版本号
	incrVersion() int32
	// 减少版本号
	// @return @1 int32 自减后的版本号
	decrVersion() int32
	// 获取版本号
	// @return @1 int32 当前版本号
	loadVersion() int32
	// 比对版本号后进行回收对象
	// 版本号一致时执行后置路由处理器并释放上下文对象
	// @param version int32 待比对的版本号
	compareVersionRecycle(version int32)
	// 执行defer调用栈
	// 版本号一致时触发栈顶延迟函数
	// @param version int32 待比对的版本号
	compareVersionExecDefer(version int32)
	// 取消Defer调用栈
	cancelDefer()
	// 恢复Defer调用栈
	recoverDefer()
	// 释放上下文
	// 清空字段并回收上下文对象到对象池
	release()
}

// Kind 消息类型
type Kind int

const (
	Event   Kind = iota // 事件
	Request             // 请求
)
