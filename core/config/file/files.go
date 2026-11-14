package file

import (
	coreconfig "github.com/dobyte/due/v2/core/config"
)

// UnmatchableFiles returns the configured entries that can never match a path relative to the
// source path, because they are absolute or contain a directory traversal.
func UnmatchableFiles(files []string) []string {
	return coreconfig.UnmatchableFiles(files)
}
