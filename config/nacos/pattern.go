package nacos

import (
	"strings"
	"unicode/utf8"
)

// minTermLength is the minimum length, in runes, of a literal segment used as a server-side blur
// keyword. A shorter segment narrows the candidate set too little to be worth a request.
const minTermLength = 2

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
