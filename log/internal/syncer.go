package internal

// Syncer writes log entities to a destination.
type Syncer interface {
	// Name returns the syncer name.
	Name() string
	// Write writes the log entity.
	Write(entity *Entity) error
	// Close closes the syncer.
	Close() error
}
