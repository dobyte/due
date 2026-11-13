package consul

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
	"github.com/hashicorp/consul/api"
)

const (
	defaultAddr = "127.0.0.1:8500"
	defaultPath = "config"
	defaultMode = config.ReadOnly
)

const (
	defaultAddrKey  = "etc.config.consul.addr"
	defaultPathKey  = "etc.config.consul.path"
	defaultModeKey  = "etc.config.consul.mode"
	defaultFilesKey = "etc.config.consul.files"
)

// Option is a function that configures the [options].
type Option func(o *options)

// options holds the configuration options of a [Source].
type options struct {
	// addr is the client connection address.
	// It configures the built-in client and defaults to 127.0.0.1:8500.
	addr string

	// client is the external client.
	// When it is provided, it takes precedence over the built-in client. It defaults to nil.
	client *api.Client

	// path is the path and defaults to /config.
	path string

	// mode is the read-write mode.
	// It supports the read-only, write-only and read-write modes and defaults to read-only.
	mode config.Mode

	// files is the list of files to load, store and watch, relative to path. An entry
	// without "*" is an exact path; an entry containing "*" is a pattern where "*"
	// matches any sequence of characters. When it is empty, every file under path is
	// available.
	files []string
}

// defaultOptions creates the default options, reading each parameter from the
// configuration environment and filling in its default value.
func defaultOptions() *options {
	return &options{
		addr:  etc.Get(defaultAddrKey, defaultAddr).String(),
		path:  etc.Get(defaultPathKey, defaultPath).String(),
		mode:  config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
		files: etc.Get(defaultFilesKey, []string{}).Strings(),
	}
}

// WithAddr sets the client connection address.
func WithAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithClient sets the external client.
func WithClient(client *api.Client) Option {
	return func(o *options) { o.client = client }
}

// WithPath sets the base path.
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode sets the read-write mode.
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}

// WithFiles sets the list of files to load, store and watch, relative to the source
// path. When it is empty, every file under the source path is available.
//
// An entry is an exact path unless it contains "*", in which case it is a pattern
// where "*" matches any sequence of characters and every other character is
// literal; only "*" is supported as a wildcard. When the list is not empty, an
// explicit load or store of a file outside the list is rejected.
func WithFiles(files ...string) Option {
	return func(o *options) { o.files = files }
}
