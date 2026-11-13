// Package pattern implements the wildcard matching shared by the config sources.
package pattern

import "strings"

// IsWildcard reports whether a configured file entry contains the "*" wildcard.
func IsWildcard(entry string) bool {
	return strings.Contains(entry, "*")
}

// MatchFile reports whether name is selected by files. An entry without "*" must equal name;
// otherwise it is a pattern where "*" matches any sequence of characters and every other character
// is literal; only "*" is supported as a wildcard.
func MatchFile(files []string, name string) bool {
	for _, file := range files {
		if file == name {
			return true
		}

		if IsWildcard(file) && MatchWildcard(file, name) {
			return true
		}
	}

	return false
}

// MatchWildcard matches name against pattern, where "*" matches any sequence of characters and
// every other character is literal.
func MatchWildcard(pattern, name string) bool {
	segments := strings.Split(pattern, "*")
	if len(segments) == 1 {
		return pattern == name
	}

	if !strings.HasPrefix(name, segments[0]) {
		return false
	}
	name = name[len(segments[0]):]

	for _, segment := range segments[1 : len(segments)-1] {
		index := strings.Index(name, segment)
		if index < 0 {
			return false
		}

		name = name[index+len(segment):]
	}

	return strings.HasSuffix(name, segments[len(segments)-1])
}
