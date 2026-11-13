package core

import (
	"path"
	"path/filepath"
	"strings"
)

// normalizeFiles trims the configured file list and drops empty, "." and duplicate entries. Entries
// are converted to slash-separated, cleaned paths so that they match the relative path of a walked
// file on every platform.
func normalizeFiles(files []string) []string {
	if len(files) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(files))
	normalized := make([]string, 0, len(files))

	for _, file := range files {
		file = path.Clean(filepath.ToSlash(strings.TrimSpace(file)))
		if file == "." {
			continue
		}

		if _, ok := seen[file]; ok {
			continue
		}

		seen[file] = struct{}{}
		normalized = append(normalized, file)
	}

	return normalized
}
