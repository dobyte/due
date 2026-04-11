package link

import (
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
)

type Options struct {
	ID                string            // 实例ID
	Kind              cluster.Kind      // 实例类型
	Codec             encoding.Codec    // 编解码器
	Locator           locate.Locator    // 定位器
	Registry          registry.Registry // 注册器
	Encryptor         crypto.Encryptor  // 加密器
	Dispatch          cluster.Dispatch  // 无状态路由消息分发策略
	ConnNum           int               // 连接数
	CallTimeout       time.Duration     // 调用超时时间
	DialTimeout       time.Duration     // 拨号超时时间
	DialRetryTimes    int               // 拨号重试次数
	WriteTimeout      time.Duration     // 写入超时时间
	WriteBufferSize   int               // 写入缓冲区大小
	FaultRecoveryTime time.Duration     // 故障恢复时间
	WaitHandler       func()            // 等待处理
	DoneHandler       func()            // 完成处理
}
