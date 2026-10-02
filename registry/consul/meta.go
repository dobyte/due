package consul

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
)

// metaValueSize is the maximum length limit of a single Consul Meta value.
const metaValueSize = 512

// Service instance metadata field names.
const (
	metaFieldID           = "id"
	metaFieldKind         = "kind"
	metaFieldAlias        = "alias"
	metaFieldState        = "state"
	metaFieldRoutes       = "routes"
	metaFieldEvents       = "events"
	metaFieldWeight       = "weight"
	metaFieldServices     = "services"
	metaFieldEndpoint     = "endpoint"
	defaultMetadataPrefix = "_"
)

// Route metadata flag bits.
const (
	metaRouteInternal = 1 << iota
	metaRouteStateful
	metaRouteAuthorized
)

// marshalMetaRoutes encodes the route metadata.
//
// The route list is stored in chunks as comma-separated strings so that a single Consul Meta value
// does not exceed its length limit.
func marshalMetaRoutes(routes []registry.Route) map[string]string {
	metas := make(map[string]string)

	var (
		items []string
		size  int
	)

	flush := func() {
		if len(items) == 0 {
			return
		}

		metas[fmt.Sprintf("%s-%d", metaFieldRoutes, len(metas))] = strings.Join(items, ",")

		items = nil
		size = 0
	}

	for _, route := range routes {
		var opts int

		if route.Internal {
			opts |= metaRouteInternal
		}

		if route.Stateful {
			opts |= metaRouteStateful
		}

		if route.Authorized {
			opts |= metaRouteAuthorized
		}

		val := fmt.Sprintf("%d-%d", route.ID, opts)

		if size > 0 && size+1+len(val) > metaValueSize {
			flush()
		}

		items = append(items, val)

		if size == 0 {
			size = len(val)
		} else {
			size += 1 + len(val)
		}
	}

	flush()

	return metas
}

// unmarshalMetaRoutes decodes the route metadata.
func unmarshalMetaRoutes(metas map[string]string) []registry.Route {
	routes := make([]registry.Route, 0)

	indexes := make([]int, 0, len(metas))

	for field := range metas {
		parts := strings.Split(field, "-")

		if len(parts) != 2 || parts[0] != metaFieldRoutes {
			continue
		}

		indexes = append(indexes, xconv.Int(parts[1]))
	}

	// Decode by chunk index to keep the route order stable across multiple chunks.
	sort.Ints(indexes)

	for _, index := range indexes {
		for _, item := range strings.Split(metas[fmt.Sprintf("%s-%d", metaFieldRoutes, index)], ",") {
			// Split at the last "-" to support negative route IDs; opts is always non-negative, so
			// there is no ambiguity.
			idx := strings.LastIndex(item, "-")
			if idx <= 0 {
				continue
			}

			opts := xconv.Int(item[idx+1:])

			routes = append(routes, registry.Route{
				ID:         xconv.Int32(item[:idx]),
				Internal:   opts&metaRouteInternal != 0,
				Stateful:   opts&metaRouteStateful != 0,
				Authorized: opts&metaRouteAuthorized != 0,
			})
		}
	}

	return routes
}

// marshalMetaList encodes a metadata list.
//
// Because a single Consul Meta value has a length limit ([metaValueSize]), large lists are stored
// in chunks. Each chunk is a valid JSON array and its meta key has the form <field>-<index>.
func marshalMetaList[T any](field string, list []T) (map[string]string, error) {
	metas := make(map[string]string)

	if len(list) == 0 {
		return metas, nil
	}

	var (
		chunk []T
		size  int // Exact byte length of the current chunk's JSON array
	)

	flush := func() {
		if len(chunk) == 0 {
			return
		}

		if data, err := json.Marshal(chunk); err == nil {
			metas[fmt.Sprintf("%s-%d", field, len(metas))] = xconv.BytesToString(data)
		}

		chunk = nil
		size = 0
	}

	for _, item := range list {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}

		// Return an error when a single element cannot fit even in a standalone chunk.
		if len(data) > metaValueSize-2 {
			return nil, errors.New("consul meta value size exceeded")
		}

		if size > 0 && size+1+len(data) > metaValueSize {
			flush()
		}

		chunk = append(chunk, item)

		if size == 0 {
			size = len(data) + 2
		} else {
			size += len(data) + 1
		}
	}

	flush()

	return metas, nil
}

// unmarshalMetaList decodes a metadata list and stays compatible with the legacy storage format,
// which was not chunked and used the field name directly as the key.
func unmarshalMetaList[T any](field string, metas map[string]string) ([]T, error) {
	list := make([]T, 0)

	indexes := make([]int, 0, len(metas))

	for metaField := range metas {
		parts := strings.Split(metaField, "-")

		if len(parts) != 2 || parts[0] != field {
			continue
		}

		indexes = append(indexes, xconv.Int(parts[1]))
	}

	sort.Ints(indexes)

	for _, index := range indexes {
		chunk := make([]T, 0)

		if err := json.Unmarshal(xconv.StringToBytes(metas[fmt.Sprintf("%s-%d", field, index)]), &chunk); err != nil {
			return nil, err
		}

		list = append(list, chunk...)
	}

	// Stay compatible with the legacy, non-chunked storage format.
	if v, ok := metas[field]; ok {
		chunk := make([]T, 0)

		if err := json.Unmarshal(xconv.StringToBytes(v), &chunk); err != nil {
			return nil, err
		}

		list = append(list, chunk...)
	}

	return list, nil
}
