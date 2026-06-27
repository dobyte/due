package def

// 无状态路由消息分发策略
type Dispatch string

const (
	Random             Dispatch = "random" // 随机
	RoundRobin         Dispatch = "rr"     // 轮询
	WeightedRoundRobin Dispatch = "wrr"    // 加权轮询
	ConsistentHash     Dispatch = "ch"     // 一致性哈希分发
)
