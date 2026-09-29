package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/quic-go/quic-go"
)

type client struct {
	opts              *clientOptions
	cid               atomic.Int64
	connectHandler    network.ConnectHandler
	disconnectHandler network.DisconnectHandler
	receiveHandler    network.ReceiveHandler
	heartbeatHandler  network.HeartbeatHandler
}

var _ network.Client = (*client)(nil)

// NewClient 创建一个QUIC客户端
// 须在拨号前注册各类hook函数
// @param opts ...ClientOption 客户端配置项
// @return @1 network.Client 客户端实例
func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}
	if o.tlsConfig == nil {
		o.tlsConfig = &tls.Config{}
	} else {
		o.tlsConfig = o.tlsConfig.Clone()
	}
	o.tlsConfig.NextProtos = []string{alpn}
	return &client{opts: o}
}

// Dial 拨号连接
// 建立一条双向流；拨号超时时间为0时不设置整体截止时间，QUIC传输层的握手超时仍然生效；
// OnConnect 先于 OnReceive 触发，与 Dial 是否已返回无关
// @param addr ...string 拨号地址
// @return @1 network.Conn 连接对象
// @return @2 error 错误信息
func (c *client) Dial(addr ...string) (network.Conn, error) {
	if c.opts.tlsErr != nil {
		return nil, c.opts.tlsErr
	}
	address := c.opts.addr
	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	}
	ctx := context.Background()
	if c.opts.dialTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.dialTimeout)
		defer cancel()
	}
	config := transportConfig(c.opts.heartbeatInterval)
	config.MaxIncomingStreams = -1
	config.HandshakeIdleTimeout = c.opts.dialTimeout
	// 保留主机名，以便 quic-go 推导 TLS ServerName
	qc, err := quic.DialAddr(ctx, address, c.opts.tlsConfig, config)
	if err != nil {
		return nil, err
	}
	stream, err := qc.OpenStreamSync(ctx)
	if err != nil {
		_ = qc.CloseWithError(0, "open stream failed")
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = stream.SetWriteDeadline(deadline)
	}
	// 仅打开流并不会向对端宣告QUIC流的存在；发送一次普通心跳后，
	// 即使未开启周期心跳，服务器也能主动向客户端下发消息
	hb := packet.PackHeartbeat()
	err = writeBuffer(stream, hb)
	hb.Release()
	if err != nil {
		_ = qc.CloseWithError(0, "initialize stream failed")
		return nil, err
	}
	_ = stream.SetWriteDeadline(time.Time{})
	return newClientConn(c, qc, stream), nil
}

// Protocol 获取协议名称
// @return @1 string 协议名称
func (c *client) Protocol() string { return protocol }

// OnConnect 监听连接打开
// 须在 Dial 之前注册，Dial 之后注册存在数据竞争
// @param h network.ConnectHandler 连接打开处理函数
func (c *client) OnConnect(h network.ConnectHandler) { c.connectHandler = h }

// OnDisconnect 监听连接关闭
// 须在 Dial 之前注册，Dial 之后注册存在数据竞争
// @param h network.DisconnectHandler 连接关闭处理函数
func (c *client) OnDisconnect(h network.DisconnectHandler) { c.disconnectHandler = h }

// OnReceive 监听接收到消息
// 须在 Dial 之前注册，Dial 之后注册存在数据竞争；处理函数拥有每个接收缓冲的所有权
// @param h network.ReceiveHandler 消息接收处理函数
func (c *client) OnReceive(h network.ReceiveHandler) { c.receiveHandler = h }

// OnHeartbeat 监听心跳
// 须在 Dial 之前注册，Dial 之后注册存在数据竞争
// @param h network.HeartbeatHandler 心跳处理函数
func (c *client) OnHeartbeat(h network.HeartbeatHandler) { c.heartbeatHandler = h }

func makeClientTLSConfig(caFile, serverName string) (*tls.Config, error) {
	config := &tls.Config{ServerName: serverName}
	if caFile == "" {
		return config, nil
	}
	certs, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	config.RootCAs = x509.NewCertPool()
	if !config.RootCAs.AppendCertsFromPEM(certs) {
		return nil, errors.ErrInvalidCertFile
	}
	return config, nil
}

func transportConfig(heartbeat time.Duration) *quic.Config {
	config := &quic.Config{MaxIncomingStreams: 1, MaxIncomingUniStreams: -1}
	if heartbeat > 0 {
		config.MaxIdleTimeout = 3 * heartbeat
		config.KeepAlivePeriod = heartbeat / 2
	}
	// 应用层心跳关闭时，保留 quic-go 默认空闲超时并发送传输层保活包，
	// 使健康但空闲的会话得以存活
	if heartbeat == 0 {
		config.KeepAlivePeriod = 10 * time.Second
	}
	return config
}
