package node

// Processor Actor处理器接口
// 定义Actor生命周期中的初始化、启动与销毁回调
type Processor interface {
	// Init 初始化回调
	Init()
	// Start 启动回调
	Start()
	// Destroy 销毁回调
	Destroy()
}

// BaseProcessor 基础处理器
// 提供所有回调的空实现，可嵌入自定义Processor中避免实现空方法
type BaseProcessor struct{}

// Init 初始化回调
func (b *BaseProcessor) Init() {}

// Start 启动回调
func (b *BaseProcessor) Start() {}

// Destroy 销毁回调
func (b *BaseProcessor) Destroy() {}
