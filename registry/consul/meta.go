package consul

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
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

// maxRouteCount is the maximum number of routes expanded from the route metadata.
//
// It guards against corrupt values such as "s:0-2147483647", which would otherwise exhaust memory
// while decoding.
const maxRouteCount = 1 << 16

// marshalMetaRoutes encodes the routes into compact Consul Meta values.
//
// Consul limits a single Meta value to [metaValueSize] bytes, so the plain "id-opts" form of a
// non-trivial route list reaches the limit quickly. Routes are grouped by their exact flags and
// consecutive IDs are merged into ranges:
//
//	flags = "-" | 1*3("i" | "s" | "a")
//	value = flags ":" ranges (";" flags ":" ranges)*
//	ranges = range ("," range)*
//	range = id ["-" id]
//
// "i", "s" and "a" stand for internal, stateful and authorized respectively, while "-" means no
// flags. For example, "s:1-1000;a:2000,2001" describes 1000 stateful routes and 2 authorized
// routes. The encoded text is split into chunks stored as <routes>-<index> when it exceeds
// [metaValueSize].
func marshalMetaRoutes(routes []registry.Route) (map[string]string, error) {
	var (
		metas  = make(map[string]string, len(routes)/32+1)
		groups = make(map[int][]int32, 8)
		order  = make([]int, 0, 8)
		seen   = make(map[int32]struct{}, len(routes))
		buf    = make([]byte, 0, metaValueSize)
		inSet  bool // Whether buf already holds the prefix of the current flags group
	)

	for _, route := range routes {
		if route.ID < 0 {
			return nil, errors.New("negative route id")
		}

		// Keep the first occurrence of a duplicated ID, which matches how the dispatcher builds its
		// routing table.
		if _, ok := seen[route.ID]; ok {
			continue
		}

		seen[route.ID] = struct{}{}

		var flags int

		if route.Internal {
			flags |= metaRouteInternal
		}

		if route.Stateful {
			flags |= metaRouteStateful
		}

		if route.Authorized {
			flags |= metaRouteAuthorized
		}

		if _, ok := groups[flags]; !ok {
			order = append(order, flags)
		}

		groups[flags] = append(groups[flags], route.ID)
	}

	if len(seen) > maxRouteCount {
		return nil, errors.ErrTooManyRoutes
	}

	flush := func() {
		if len(buf) == 0 {
			return
		}

		metas[fmt.Sprintf("%s-%d", metaFieldRoutes, len(metas))] = string(buf)
		buf = buf[:0]
		inSet = false
	}

	// Sort groups by flags so that the encoded value is stable across registrations.
	slices.Sort(order)

	for _, flags := range order {
		prefix := appendRouteFlags(make([]byte, 0, 3), flags)
		prefix = append(prefix, ':')

		ids := groups[flags]
		slices.Sort(ids)

		for j := 0; j < len(ids); {
			k := j

			// Compare as int64 to avoid overflowing when the previous ID is math.MaxInt32.
			for k+1 < len(ids) && int64(ids[k+1]) == int64(ids[k])+1 {
				k++
			}

			var (
				token = strconv.AppendInt(make([]byte, 0, 16), int64(ids[j]), 10)
				size  int
			)

			if k > j {
				token = append(token, '-')
				token = strconv.AppendInt(token, int64(ids[k]), 10)
			}

			// Reserve room for the separator and, when needed, the flags group prefix.
			size = len(token)
			if inSet {
				size++
			} else {
				size += len(prefix)
			}

			// Flush the current chunk and repeat the flags prefix so that every chunk stays
			// decodable on its own.
			if len(buf) > 0 && len(buf)+size > metaValueSize {
				flush()
			}

			if inSet {
				buf = append(buf, ',')
			} else {
				if len(buf) > 0 {
					buf = append(buf, ';')
				}

				buf = append(buf, prefix...)
				inSet = true
			}

			buf = append(buf, token...)

			j = k + 1
		}

		inSet = false
	}

	flush()

	return metas, nil
}

// appendRouteFlags appends the flag letters of flags to buf.
func appendRouteFlags(buf []byte, flags int) []byte {
	if flags == 0 {
		return append(buf, '-')
	}

	if flags&metaRouteInternal != 0 {
		buf = append(buf, 'i')
	}

	if flags&metaRouteStateful != 0 {
		buf = append(buf, 's')
	}

	if flags&metaRouteAuthorized != 0 {
		buf = append(buf, 'a')
	}

	return buf
}

// unmarshalMetaRoutes decodes the route metadata.
//
// It stays compatible with the legacy chunked "id-opts" format written by earlier versions.
func unmarshalMetaRoutes(metas map[string]string) ([]registry.Route, error) {
	var (
		routes  = make([]registry.Route, 0)
		indexes = make([]int, 0, len(metas))
	)

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
		value := metas[fmt.Sprintf("%s-%d", metaFieldRoutes, index)]

		// A legacy chunk consists of "id-opts" items only, while the compressed format always
		// contains the "flags:" prefix.
		if !strings.Contains(value, ":") {
			routes = append(routes, unmarshalLegacyMetaRoutes(value)...)
			continue
		}

		chunk, err := unmarshalCompressedMetaRoutes(value, maxRouteCount-len(routes))
		if err != nil {
			return nil, err
		}

		routes = append(routes, chunk...)
	}

	return routes, nil
}

// unmarshalCompressedMetaRoutes decodes the routes of a single chunk encoded by
// [marshalMetaRoutes], limiting the total number of decoded routes to remain.
func unmarshalCompressedMetaRoutes(value string, remain int) ([]registry.Route, error) {
	routes := make([]registry.Route, 0)

	for _, group := range strings.Split(value, ";") {
		flags, ranges, ok := strings.Cut(group, ":")
		if !ok {
			return nil, errors.ErrInvalidRoutes
		}

		var route registry.Route

		if flags != "-" {
			for _, flag := range flags {
				switch flag {
				case 'i':
					route.Internal = true
				case 's':
					route.Stateful = true
				case 'a':
					route.Authorized = true
				default:
					return nil, errors.ErrInvalidRoutes
				}
			}
		}

		for _, item := range strings.Split(ranges, ",") {
			start, end, hasEnd := strings.Cut(item, "-")

			startID, err := strconv.ParseInt(start, 10, 32)
			if err != nil || startID < 0 {
				return nil, errors.ErrInvalidRoutes
			}

			endID := startID

			if hasEnd {
				if endID, err = strconv.ParseInt(end, 10, 32); err != nil || endID < startID {
					return nil, errors.ErrInvalidRoutes
				}
			}

			if endID-startID+1 > int64(remain) {
				return nil, errors.ErrTooManyRoutes
			}

			remain -= int(endID - startID + 1)

			for id := startID; id <= endID; id++ {
				item := route
				item.ID = int32(id)
				routes = append(routes, item)
			}
		}
	}

	return routes, nil
}

// unmarshalLegacyMetaRoutes decodes a single chunk of the legacy, comma-separated "id-opts"
// format.
func unmarshalLegacyMetaRoutes(value string) []registry.Route {
	routes := make([]registry.Route, 0)

	for _, item := range strings.Split(value, ",") {
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
