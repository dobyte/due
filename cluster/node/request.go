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
	// Route 获取路由
	Route() int32
	// Parse 解析请求
	Parse(v interface{}) error
	// Context 获取上线文
	Context() context.Context
	// Response 响应请求
	Response(message interface{}) error
}

// 请求数据
type request struct {
	gid    string      // 来源网关ID
	nid    string      // 来源节点ID
	cid    int64       // 连接ID
	uid    int64       // 用户ID
	route  int32       // 消息路由
	buffer interface{} // 消息内容
	node   *Node       // 节点服务器
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

// Route 获取路由
func (r *request) Route() int32 {
	return r.route
}

// Parse 解析消息
func (r *request) Parse(v interface{}) error {
	if msg, ok := r.buffer.([]byte); ok {
		return r.node.opts.codec.Unmarshal(msg, v)
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(r.buffer); err != nil {
		return err
	}

	return gob.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(v)
}

// Context 获取上线文
func (r *request) Context() context.Context {
	return context.Background()
}

// Response 响应请求
func (r *request) Response(message interface{}) error {
	return r.node.proxy.Response(r.Context(), r, message)
}
