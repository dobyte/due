package redis

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/dobyte/due/v2/locate"
)

func marshal(event *locate.Event) (string, error) {
	buf, err := json.Marshal(event)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

func unmarshal(data []byte) (*locate.Event, error) {
	evt := &locate.Event{}

	if err := json.Unmarshal(data, evt); err != nil {
		return nil, err
	}

	return evt, nil
}

func toUniqueKey(kinds ...string) string {
	keys := make([]string, len(kinds))
	copy(keys, kinds)
	slices.Sort(keys)

	return strings.Join(keys, "&")
}
