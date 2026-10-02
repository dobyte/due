package file

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/config/file/core"
	"github.com/dobyte/due/v2/log"
)

// Name is the name of the file config source.
const Name = core.Name

// Source is a file-based config source.
type Source struct {
	opts *options
}

// NewSource returns a new file config source configured with opts.
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.path == "" {
		log.Fatal("no config file path specified")
	}

	return core.NewSource(o.path, o.mode)
}
