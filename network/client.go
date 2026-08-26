/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/11 10:01 上午
 * @Desc: TODO
 */

package network

// Client 客户端接口
type Client interface {
	// Dial 拨号连接
	// @param addr ...string 拨号地址
	// @return @1 Conn 连接对象
	// @return @2 error 错误信息
	Dial(addr ...string) (Conn, error)
	// Protocol 获取协议名称
	// @return @1 string 协议名称
	Protocol() string
	// OnConnect 监听连接打开
	// @param handler ConnectHandler 连接打开处理函数
	OnConnect(handler ConnectHandler)
	// OnReceive 监听接收消息
	// @param handler ReceiveHandler 消息接收处理函数
	OnReceive(handler ReceiveHandler)
	// OnDisconnect 监听连接断开
	// @param handler DisconnectHandler 连接关闭处理函数
	OnDisconnect(handler DisconnectHandler)
}
