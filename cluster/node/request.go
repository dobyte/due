/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/19 12:20 下午
 * @Desc: TODO
 */

package node

import (
	"github.com/jinzhu/copier"
	"github.com/symsimmy/due/log"
	"strings"
)

// Request 请求数据
type Request struct {
	node    *Node
	GID     string   // 来源网关ID
	NID     string   // 来源节点ID
	CID     int64    // 连接ID
	UID     int64    // 用户ID
	Message *Message // 请求消息
}

type validator interface {
	Validate() error
}

// Parse 解析消息
func (r *Request) Parse(v interface{}) error {
	msg, ok := r.Message.Data.([]byte)
	if !ok {
		return copier.CopyWithOption(v, r.Message.Data, copier.Option{
			DeepCopy: true,
		})
	}

	if r.GID != "" && r.node.opts.decryptor != nil {
		data, err := r.node.opts.decryptor.Decrypt(msg)
		if err != nil {
			return err
		}

		return r.node.opts.codec.Unmarshal(data, v)
	}

	err := r.node.opts.codec.Unmarshal(msg, v)
	if strings.EqualFold("center", r.node.instance.Alias) {
		log.Infof("[Request] CID:%+v,UID:%+v,GID:%+v,NID:%+v,message:%+v", r.CID, r.UID, r.GID, r.NID, v)
	}
	return err
}

// Validate 解析消息
func (r *Request) Validate(v interface{}) error {
	if v, ok := v.(validator); ok {
		return v.Validate()
	}
	return nil
}
