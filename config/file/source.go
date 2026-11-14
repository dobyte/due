package file

import (
	"github.com/dobyte/due/v2/core/config"
	corefile "github.com/dobyte/due/v2/core/config/file"
	"github.com/dobyte/due/v2/log"
)

// Name is the name of the file config source.
const Name = corefile.Name

// NewSource returns a new file config source configured with opts.
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.path == "" {
		log.Warnf("no config file path specified, loading the config will fail")
	}

	for _, file := range corefile.UnmatchableFiles(o.files) {
		log.Warnf("config file entry %q is absolute or contains a directory traversal, it will never be matched", file)
	}

	return corefile.NewSource(o.path, o.mode, o.files...)
}
