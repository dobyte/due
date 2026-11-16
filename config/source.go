package config

import "github.com/dobyte/due/v2/core/config"

// The read-write modes of a [Source].
const (
	ReadOnly  = config.ReadOnly  // ReadOnly: the config source can only be read
	WriteOnly = config.WriteOnly // WriteOnly: the config source can only be written
	ReadWrite = config.ReadWrite // ReadWrite: the config source can be read and written
)

// Mode is the read-write mode of a [Source].
type Mode = config.Mode

// Source loads, stores and watches configurations.
type Source = config.Source

// Watcher watches the config changes of a [Source].
type Watcher = config.Watcher
