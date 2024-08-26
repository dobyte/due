package actor

type Processor interface {
	// Kind 类型
	Kind() string
	// Init 初始化回调
	Init()
	// Start 启动回调
	Start()
	// Destroy 销毁回调
	Destroy()
}
