package client

import (
	"context"
	"github.com/dobyte/due/packet"
)

type Request interface {
	// CID 获取来源连接ID
	CID() int64
	// UID 获取来源用户ID
	UID() int64
	// Seq 获取消息序列号
	Seq() int32
	// Route 获取路由
	Route() int32
	// Data 获取数据
	Data() interface{}
	// Parse 解析请求
	Parse(v interface{}) error
	// Context 获取上线文
	Context() context.Context
	// Proxy 获取代理
	Proxy() Proxy
}

// 请求数据
type request struct {
	client  *Client         // 客户端
	message *packet.Message // 消息
}

// CID 获取来源连接ID
func (r *request) CID() int64 {
	return r.client.conn.ID()
}

// UID 获取来源用户ID
func (r *request) UID() int64 {
	return r.client.conn.UID()
}

// Seq 获取消息序列号
func (r *request) Seq() int32 {
	return r.message.Seq
}

// Route 获取消息路由
func (r *request) Route() int32 {
	return r.message.Route
}

// Data 获取消息数据
func (r *request) Data() interface{} {
	return r.message.Buffer
}

// Parse 解析消息
func (r *request) Parse(v interface{}) (err error) {
	buffer := r.message.Buffer

	if r.client.opts.decryptor != nil {
		buffer, err = r.client.opts.decryptor.Decrypt(buffer)
		if err != nil {
			return
		}
	}

	return r.client.opts.codec.Unmarshal(buffer, v)
}

// Context 获取上线文
func (r *request) Context() context.Context {
	return context.Background()
}

// Proxy 响应请求
func (r *request) Proxy() Proxy {
	return r.client.proxy
}
