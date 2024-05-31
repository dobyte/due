package gate

import "github.com/dobyte/due/v2/cluster"

type Options struct {
	InsID   string       // 实例ID
	InsKind cluster.Kind // 实例类型
}
