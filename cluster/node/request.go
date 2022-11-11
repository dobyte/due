/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/19 12:20 下午
 * @Desc: TODO
 */

package node

import (
	"bytes"
	"context"
	"encoding/gob"
	"github.com/dobyte/due/session"
)

type Request interface {
	// GID 获取来源网关ID
	GID() string
	// NID 获取来源节点ID
	NID() string
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
	// GetIP 获取IP地址
	GetIP() (string, error)
	// Response 响应请求
	Response(message interface{}) error
	// BindGate 绑定网关
	BindGate(uid int64) error
	// UnbindGate 解绑网关
	UnbindGate() error
	// BindNode 绑定节点
	BindNode() error
	// UnbindNode 解绑节点
	UnbindNode() error
}

// 请求数据
type request struct {
	gid   string      // 来源网关ID
	nid   string      // 来源节点ID
	cid   int64       // 连接ID
	uid   int64       // 用户ID
	seq   int32       // 消息序列号
	route int32       // 消息路由
	data  interface{} // 消息内容
	node  *Node       // 节点服务器
}

// GID 获取来源网关ID
func (r *request) GID() string {
	return r.gid
}

// NID 获取来源节点ID
func (r *request) NID() string {
	return r.nid
}

// CID 获取来源连接ID
func (r *request) CID() int64 {
	return r.cid
}

// UID 获取来源用户ID
func (r *request) UID() int64 {
	return r.uid
}

// Seq 获取消息序列号
func (r *request) Seq() int32 {
	return r.seq
}

// Route 获取消息路由
func (r *request) Route() int32 {
	return r.route
}

// Data 获取消息数据
func (r *request) Data() interface{} {
	return r.data
}

// Parse 解析消息
func (r *request) Parse(v interface{}) (err error) {
	msg, ok := r.data.([]byte)
	if !ok {
		var buf bytes.Buffer
		if err = gob.NewEncoder(&buf).Encode(r.data); err != nil {
			return
		}
		return gob.NewDecoder(&buf).Decode(v)
	}

	if r.gid != "" && r.node.opts.decryptor != nil {
		msg, err = r.node.opts.decryptor.Decrypt(msg)
		if err != nil {
			return
		}
	}

	return r.node.opts.codec.Unmarshal(msg, v)
}

// Context 获取上线文
func (r *request) Context() context.Context {
	return context.Background()
}

// GetIP 获取IP地址
func (r *request) GetIP() (string, error) {
	return r.node.proxy.GetIP(r.Context(), &GetIPArgs{
		GID:    r.gid,
		Kind:   session.Conn,
		Target: r.cid,
	})
}

// Response 响应请求
func (r *request) Response(message interface{}) error {
	return r.node.proxy.Response(r.Context(), r, message)
}

// BindGate 绑定网关
func (r *request) BindGate(uid int64) error {
	return r.node.proxy.BindGate(r.Context(), r.gid, r.cid, uid)
}

// UnbindGate 解绑网关
func (r *request) UnbindGate() error {
	return r.node.proxy.UnbindGate(r.Context(), r.uid)
}

// BindNode 绑定节点
func (r *request) BindNode() error {
	return r.node.proxy.BindNode(r.Context(), r.uid)
}

// UnbindNode 解绑节点
func (r *request) UnbindNode() error {
	return r.node.proxy.UnbindNode(r.Context(), r.uid)
}
