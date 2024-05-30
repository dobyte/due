package client

import "github.com/dobyte/due/v2/cluster"

type Options struct {
	ID   string       // 实例ID
	Kind cluster.Kind // 实例类型
	Addr string       // 连接地址
}
