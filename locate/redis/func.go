package redis

import (
	"slices"
	"strings"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/locate"
)

// marshal serializes a locate event.
//
// It encodes a locate event as a JSON string for broadcasting through redis publish/subscribe. It
// returns the serialized JSON string and an error when serialization fails.
func marshal(event *locate.Event) (string, error) {
	buf, err := json.Marshal(event)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// unmarshal deserializes a locate event.
//
// It decodes a JSON string into a locate event. It returns the decoded locate event and an error
// when deserialization fails.
func unmarshal(data []byte) (*locate.Event, error) {
	evt := &locate.Event{}

	if err := json.Unmarshal(data, evt); err != nil {
		return nil, err
	}

	return evt, nil
}

// toUniqueKey generates a unique key.
//
// It sorts several instance kinds and joins them into a unique key, so that listeners with the same
// combination of instance kinds can share a single watch manager. It returns the sorted, joined
// unique key.
func toUniqueKey(kinds ...string) string {
	keys := make([]string, len(kinds))
	copy(keys, kinds)
	slices.Sort(keys)

	return strings.Join(keys, "&")
}
