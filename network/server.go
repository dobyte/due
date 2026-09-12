/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/11 10:01 上午
 * @Desc: TODO
 */

package network

import "github.com/dobyte/due/v2/core/buffer"

type (
	// StartHandler 服务器启动处理函数
	StartHandler func()
	// CloseHandler 服务器关闭处理函数
	CloseHandler func()
	// ConnectHandler 连接打开处理函数
	// @param conn Conn 连接
	ConnectHandler func(conn Conn)
	// DisconnectHandler 连接关闭处理函数
	// @param conn Conn 连接
	DisconnectHandler func(conn Conn)
	// HeartbeatHandler 心跳处理函数
	// @param conn Conn 连接
	// @param heartbeatTime int64 客户端收到服务器发送的心跳时间（纳秒）
	HeartbeatHandler func(conn Conn, heartbeatTime int64)
	// ReceiveHandler 消息接收处理函数
	// @param conn Conn 连接
	// @param buf buffer.Buffer 消息缓冲区，用户层需自行控制释放点，避免内存泄漏
	ReceiveHandler func(conn Conn, buf buffer.Buffer)
)

// Server 服务器接口
type Server interface {
	// Addr 获取监听地址
	// @return @1 string 监听地址
	Addr() string
	// Start 启动服务器
	// @return @1 error 错误信息
	Start() error
	// Stop 关闭服务器
	// @return @1 error 错误信息
	Stop() error
	// Protocol 获取协议名称
	// @return @1 string 协议名称
	Protocol() string
	// OnStart 监听服务器启动
	// @param handler StartHandler 服务器启动处理函数
	OnStart(handler StartHandler)
	// OnStop 监听服务器关闭
	// @param handler CloseHandler 服务器关闭处理函数
	OnStop(handler CloseHandler)
	// OnConnect 监听连接打开
	// @param handler ConnectHandler 连接打开处理函数
	OnConnect(handler ConnectHandler)
	// OnHeartbeat 监听心跳
	// @param handler HeartbeatHandler 心跳处理函数
	OnHeartbeat(handler HeartbeatHandler)
	// OnReceive 监听接收消息
	// @param handler ReceiveHandler 消息接收处理函数
	OnReceive(handler ReceiveHandler)
	// OnDisconnect 监听连接断开
	// @param handler DisconnectHandler 连接关闭处理函数
	OnDisconnect(handler DisconnectHandler)
}
