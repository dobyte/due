package node

type Processor interface {
	// Init 初始化回调
	Init()
	// Start 启动回调
	Start()
	// Destroy 销毁回调
	Destroy()
}

type BaseProcessor struct{}

// Init 初始化回调
func (b *BaseProcessor) Init() {}

// Start 启动回调
func (b *BaseProcessor) Start() {}

// Destroy 销毁回调
func (b *BaseProcessor) Destroy() {}
