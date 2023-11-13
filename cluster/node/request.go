/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/6/19 12:20 下午
 * @Desc: TODO
 */

package node

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/jinzhu/copier"
)

// Request 请求数据
type Request struct {
	node    *Node
	GID     string           // 来源网关ID
	NID     string           // 来源节点ID
	CID     int64            // 连接ID
	UID     int64            // 用户ID
	Message *cluster.Message // 请求消息
}

// Parse 解析消息
func (r *Request) Parse(v interface{}) error {
	msg, ok := r.Message.Data.([]byte)
	if !ok {
		return copier.CopyWithOption(v, r.Message.Data, copier.Option{
			DeepCopy: true,
		})
	}

	if r.GID != "" && r.node.opts.encryptor != nil {
		data, err := r.node.opts.encryptor.Decrypt(msg)
		if err != nil {
			return err
		}

		return r.node.opts.codec.Unmarshal(data, v)
	}

	return r.node.opts.codec.Unmarshal(msg, v)
}
