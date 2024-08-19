package node

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

type BaseProcessor struct{}

// Kind 消息处理器类型
func (b *BaseProcessor) Kind() string { return "actor" }

// Init 初始化回调
func (b *BaseProcessor) Init() {}

// Start 启动回调
func (b *BaseProcessor) Start() {}

// Destroy 销毁回调
func (b *BaseProcessor) Destroy() {}
