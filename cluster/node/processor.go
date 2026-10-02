package node

// Processor is the processor interface of an actor.
//
// It defines the init, start and destroy callbacks of the actor lifecycle.
type Processor interface {
	// Init is the init callback.
	Init()
	// Start is the start callback.
	Start()
	// Destroy is the destroy callback.
	Destroy()
}

// BaseProcessor is a base processor.
//
// It provides empty implementations of every callback so that a custom Processor can embed it to
// avoid implementing the empty methods.
type BaseProcessor struct{}

// Init is the init callback.
func (b *BaseProcessor) Init() {}

// Start is the start callback.
func (b *BaseProcessor) Start() {}

// Destroy is the destroy callback.
func (b *BaseProcessor) Destroy() {}
