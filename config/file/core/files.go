package core

import (
	"path"
	"path/filepath"
	"strings"
)

// UnmatchableFiles returns the configured entries that can never match a path relative to the
// source path, because they are absolute or contain a directory traversal.
func UnmatchableFiles(files []string) []string {
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
