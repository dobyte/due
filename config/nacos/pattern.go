package nacos

import (
	"strings"
	"unicode/utf8"
)

// minTermLength is the minimum length, in runes, of a literal segment used as a server-side blur
// keyword. A shorter segment narrows the candidate set too little to be worth a request.
const minTermLength = 2

// isPattern reports whether a configured file entry contains the "*" wildcard.
func isPattern(file string) bool {
	return strings.Contains(file, "*")
}

// matchDataId reports whether dataId is selected by the configured files. An entry without "*" must
// equal dataId; otherwise it is a pattern where "*" matches any sequence of characters and every
// other character is literal.
func matchDataId(files []string, dataId string) bool {
	for _, file := range files {
		if file == dataId {
			return true
		}

		if isPattern(file) && matchWildcard(file, dataId) {
			return true
		}
	}

	return false
}

// matchWildcard matches dataId against pattern, where "*" matches any sequence of characters and
// every other character is literal.
func matchWildcard(pattern, dataId string) bool {
	segments := strings.Split(pattern, "*")
	if len(segments) == 1 {
		return pattern == dataId
	}

	if !strings.HasPrefix(dataId, segments[0]) {
		return false
	}
	dataId = dataId[len(segments[0]):]

	for _, segment := range segments[1 : len(segments)-1] {
		index := strings.Index(dataId, segment)
		if index < 0 {
			return false
		}

		dataId = dataId[index+len(segment):]
	}

	return strings.HasSuffix(dataId, segments[len(segments)-1])
}

// patternTerm returns the first literal segment of a pattern long enough to be used as the
// server-side blur keyword. The leading segment is preferred because it is usually the most
// selective part of a config name; a pattern with no usable segment, such as "*", returns an empty
// string and makes the caller scan the whole group.
func patternTerm(pattern string) string {
	for _, segment := range strings.Split(pattern, "*") {
		if utf8.RuneCountInString(segment) >= minTermLength {
			return segment
		}
	}

	return ""
}
