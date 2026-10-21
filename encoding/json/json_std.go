//go:build stdjson
// +build stdjson

package json

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// options align the encoding/json/v2 backend with the sonic backend, which is the default one:
// field names match case-insensitively, duplicate object members keep the last value, and invalid
// UTF-8 is tolerated instead of failing the operation.
var options = []json.Options{
	json.MatchCaseInsensitiveNames(true),
	jsontext.AllowDuplicateNames(true),
	jsontext.AllowInvalidUTF8(true),
}

// marshal encodes v with encoding/json/v2, selected by the stdjson build tag.
func marshal(v any) ([]byte, error) {
	return json.Marshal(v, options...)
}

// unmarshal decodes data into v with encoding/json/v2, selected by the stdjson build tag.
func unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v, options...)
}
