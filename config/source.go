package config

import "context"

// The read-write modes of a [Source].
const (
	ReadOnly  Mode = "read-only"  // ReadOnly: the config source can only be read
	WriteOnly Mode = "write-only" // WriteOnly: the config source can only be written
	ReadWrite Mode = "read-write" // ReadWrite: the config source can be read and written
)

// Mode is the read-write mode of a [Source].
type Mode string

// Source loads, stores and watches configurations.
type Source interface {
	// Name returns the name of the source.
	Name() string
	// Load loads the configurations of the given files, or every file of the source when file is
	// empty.
	Load(ctx context.Context, file ...string) ([]*Configuration, error)
	// Store saves content as file.
	Store(ctx context.Context, file string, content []byte) error
	// Watch starts watching the source and returns a [Watcher].
	Watch(ctx context.Context) (Watcher, error)
	// Close closes the source.
	Close() error
}

// Watcher watches the config changes of a [Source].
type Watcher interface {
	// Next blocks until a config changes and returns the changed configurations.
	Next() ([]*Configuration, error)
	// Stop stops watching.
	Stop() error
}
