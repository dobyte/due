package file

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultPath = "./config"
	defaultMode = config.ReadOnly
)

const (
	defaultPathKey  = "etc.config.file.path"
	defaultModeKey  = "etc.config.file.mode"
	defaultFilesKey = "etc.config.file.files"
)

// Option configures a [Source].
type Option func(o *options)

type options struct {
	// Path of the config file or config directory.
	path string

	// Read-write mode.
	// It supports read-only, write-only and read-write; the default is read-only.
	mode config.Mode

	// List of files to load, store and watch, relative to path. An entry without "*" is an
	// exact path; an entry containing "*" is a pattern where "*" matches any sequence of
	// characters. When empty, every file under path is available.
	files []string
}

func defaultOptions() *options {
	return &options{
		path:  etc.Get(defaultPathKey, defaultPath).String(),
		mode:  config.Mode(etc.Get(defaultModeKey, defaultMode).String()),
		files: etc.Get(defaultFilesKey, []string{}).Strings(),
	}
}

// WithPath sets the path of the config file or config directory.
func WithPath(path string) Option {
	return func(o *options) { o.path = path }
}

// WithMode sets the read-write mode.
func WithMode(mode config.Mode) Option {
	return func(o *options) { o.mode = mode }
}

// WithFiles sets the list of files to load, store and watch, relative to the source path. When it
// is empty, every file under the source path is available.
//
// An entry is an exact path unless it contains "*", in which case it is a pattern where "*"
// matches any sequence of characters and every other character is literal; only "*" is supported
// as a wildcard. When the list is not empty, an explicit load or store of a file outside the list
// is rejected. Empty and duplicate entries are ignored.
func WithFiles(files ...string) Option {
	return func(o *options) { o.files = files }
}

// unmatchableFiles returns the configured entries that can never match a path relative to the
// source path, because they are absolute or contain a directory traversal.
func unmatchableFiles(files []string) []string {
	var invalid []string

	for _, file := range files {
		file = path.Clean(filepath.ToSlash(strings.TrimSpace(file)))
		if file == "." {
			continue
		}

		if strings.HasPrefix(file, "/") || filepath.IsAbs(filepath.FromSlash(file)) ||
			file == ".." || strings.HasPrefix(file, "../") {
			invalid = append(invalid, file)
		}
	}

	return invalid
}
