package component

// Component is the interface that every component of the framework must implement.
type Component interface {
	// Name returns the component name.
	Name() string
	// Init initializes the component.
	Init()
	// Start starts the component.
	Start()
	// Close closes the component.
	Close()
	// Destroy destroys the component.
	Destroy()
}

// Base is an embeddable no-op implementation of [Component].
type Base struct {
}

// Name returns the component name.
func (b *Base) Name() string { return "base" }

// Init initializes the component.
func (b *Base) Init() {}

// Start starts the component.
func (b *Base) Start() {}

// Close closes the component.
func (b *Base) Close() {}

// Destroy destroys the component.
func (b *Base) Destroy() {}
