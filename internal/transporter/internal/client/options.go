package client

import "github.com/dobyte/due/v2/cluster"

type Options struct {
	Addr         string       // 连接地址
	InsID        string       // 实例ID
	InsKind      cluster.Kind // 实例类型
	CloseHandler func()       // 关闭处理器
}
