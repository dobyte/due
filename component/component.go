package component

type Component interface {
	// Name 组件名称
	Name() string
	// Init 初始化组件
	Init()
	// Start 启动组件
	Start()
	// Restart 重启组件
	Restart()
	// Destroy 销毁组件
	Destroy()
}

type Base struct {
}

// Name 组件名称
func (b *Base) Name() string { return "base" }

// Init 初始化组件
func (b *Base) Init() {}

// Start 启动组件
func (b *Base) Start() {}

// Restart 重启组件
func (b *Base) Restart() {}

// Destroy 销毁组件
func (b *Base) Destroy() {}
