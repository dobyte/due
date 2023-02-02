/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/19 12:20 下午
 * @Desc: TODO
 */

package node

import (
	"github.com/jinzhu/copier"
)

// Request 请求数据
type Request struct {
	node    *Node
	gid     string   // 来源网关ID
	nid     string   // 来源节点ID
	cid     int64    // 连接ID
	uid     int64    // 用户ID
	message *Message // 请求消息
}

// GID 获取来源网关ID
func (r *Request) GID() string {
	return r.gid
}

// NID 获取来源节点ID
func (r *Request) NID() string {
	return r.nid
}

// CID 获取来源连接ID
func (r *Request) CID() int64 {
	return r.cid
}

// UID 获取来源用户ID
func (r *Request) UID() int64 {
	return r.uid
}

// Seq 获取消息序列号
func (r *Request) Seq() int32 {
	return r.message.Seq
}

// Route 获取消息路由
func (r *Request) Route() int32 {
	return r.message.Route
}

// Data 获取消息数据
func (r *Request) Data() interface{} {
	return r.message.Data
}

// Parse 解析消息
func (r *Request) Parse(v interface{}) error {
	msg, ok := r.message.Data.([]byte)
	if !ok {
		return copier.CopyWithOption(v, r.message.Data, copier.Option{
			DeepCopy: true,
		})
	}

	if r.gid != "" && r.node.opts.decryptor != nil {
		data, err := r.node.opts.decryptor.Decrypt(msg)
		if err != nil {
			return err
		}

		return r.node.opts.codec.Unmarshal(data, v)
	}

	return r.node.opts.codec.Unmarshal(msg, v)
}
