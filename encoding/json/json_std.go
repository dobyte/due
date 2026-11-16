//go:build stdjson
// +build stdjson

package json

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// options align the encoding/json/v2 backend with the sonic backend, which is the default one:
// field names match case-insensitively, duplicate object members keep the last value, invalid
// UTF-8 is tolerated instead of failing the operation, and omitempty follows the legacy definition
// so that false and zero numbers are omitted like sonic instead of being kept by v2.
var options = []json.Options{
	jsonv1.OmitEmptyWithLegacySemantics(true),
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

// unmarshalString decodes a JSON string into v with encoding/json/v2, selected by the stdjson
// build tag.
func unmarshalString(s string, v any) error {
	return unmarshal([]byte(s), v)
}

// appendEncode appends the JSON encoding of v to dst with encoding/json/v2.
func appendEncode(dst []byte, v any) ([]byte, error) {
	data, err := json.Marshal(v, options...)
	if err != nil {
		return nil, err
	}

	return append(dst, data...), nil
}
