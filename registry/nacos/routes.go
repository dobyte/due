package nacos

import (
	"slices"
	"strconv"
	"strings"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

// Route flag bits.
const (
	routeFlagInternal = 1 << iota
	routeFlagStateful
	routeFlagAuthorized
)

// maxRouteCount is the maximum number of routes expanded from a single metadata value.
//
// It guards against corrupt values such as "s:0-2147483647", which would otherwise exhaust memory
// while decoding.
const maxRouteCount = 1 << 16

var (
	errInvalidRoutes = errors.New("invalid routes metadata")
	errTooManyRoutes = errors.New("routes metadata exceeds the route count limit")
)

// marshalRoutes encodes routes into a compact, human-readable metadata value.
//
// Nacos limits the total length of all instance metadata, so the JSON form of a non-trivial route
// list is too large to register. Routes are grouped by their exact flags and consecutive IDs are
// merged into ranges:
//
//	flags = "-" | 1*3("i" | "s" | "a")
//	value = flags ":" ranges (";" flags ":" ranges)*
//	ranges = range ("," range)*
//	range = id ["-" id]
//
// "i", "s" and "a" stand for internal, stateful and authorized respectively, while "-" means no
// flags. For example, "s:1-1000;a:2000,2001" describes 1000 stateful routes and 2 authorized
// routes.
func marshalRoutes(routes []registry.Route) (string, error) {
	groups := make(map[int][]int32, 8)
	order := make([]int, 0, 8)
	seen := make(map[int32]struct{}, len(routes))

	for _, route := range routes {
		if route.ID < 0 {
			return "", errors.New("negative route id")
		}

		// Keep the first occurrence of a duplicated ID, which matches how the dispatcher builds
		// its routing table.
		if _, ok := seen[route.ID]; ok {
			continue
		}

		seen[route.ID] = struct{}{}

		var flags int

		if route.Internal {
			flags |= routeFlagInternal
		}

		if route.Stateful {
			flags |= routeFlagStateful
		}

		if route.Authorized {
			flags |= routeFlagAuthorized
		}

		if _, ok := groups[flags]; !ok {
			order = append(order, flags)
		}

		groups[flags] = append(groups[flags], route.ID)
	}

	if len(seen) > maxRouteCount {
		return "", errTooManyRoutes
	}

	// Sort groups by flags so that the encoded value is stable across registrations.
	slices.Sort(order)

	buf := make([]byte, 0, len(routes)*4)

	for i, flags := range order {
		if i > 0 {
			buf = append(buf, ';')
		}

		buf = appendRouteFlags(buf, flags)
		buf = append(buf, ':')

		ids := groups[flags]
		slices.Sort(ids)

		for j := 0; j < len(ids); {
			k := j

			// Compare as int64 to avoid overflowing when the previous ID is math.MaxInt32.
			for k+1 < len(ids) && int64(ids[k+1]) == int64(ids[k])+1 {
				k++
			}

			if j > 0 {
				buf = append(buf, ',')
			}

			buf = strconv.AppendInt(buf, int64(ids[j]), 10)

			if k > j {
				buf = append(buf, '-')
				buf = strconv.AppendInt(buf, int64(ids[k]), 10)
			}

			j = k + 1
		}
	}

	return string(buf), nil
}

// appendRouteFlags appends the flag letters of flags to buf.
func appendRouteFlags(buf []byte, flags int) []byte {
	if flags == 0 {
		return append(buf, '-')
	}

	if flags&routeFlagInternal != 0 {
		buf = append(buf, 'i')
	}

	if flags&routeFlagStateful != 0 {
		buf = append(buf, 's')
	}

	if flags&routeFlagAuthorized != 0 {
		buf = append(buf, 'a')
	}

	return buf
}

// unmarshalRoutes decodes the route metadata value encoded by marshalRoutes.
//
// A value starting with "[" is decoded as the legacy JSON array, so instances registered by an
// earlier version remain readable during a rolling upgrade.
func unmarshalRoutes(value string) ([]registry.Route, error) {
	if value == "" {
		return nil, nil
	}

	if value[0] == '[' {
		routes := make([]registry.Route, 0)
		if err := json.Unmarshal([]byte(value), &routes); err != nil {
			return nil, err
		}

		if len(routes) > maxRouteCount {
			return nil, errTooManyRoutes
		}

		return routes, nil
	}

	routes := make([]registry.Route, 0)

	for _, group := range strings.Split(value, ";") {
		flags, ranges, ok := strings.Cut(group, ":")
		if !ok {
			return nil, errInvalidRoutes
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
					return nil, errInvalidRoutes
				}
			}
		}

		for _, item := range strings.Split(ranges, ",") {
			start, end, hasEnd := strings.Cut(item, "-")

			startID, err := strconv.ParseInt(start, 10, 32)
			if err != nil || startID < 0 {
				return nil, errInvalidRoutes
			}

			endID := startID

			if hasEnd {
				if endID, err = strconv.ParseInt(end, 10, 32); err != nil || endID < startID {
					return nil, errInvalidRoutes
				}
			}

			if endID-startID+1 > maxRouteCount-int64(len(routes)) {
				return nil, errTooManyRoutes
			}

			for id := startID; id <= endID; id++ {
				item := route
				item.ID = int32(id)
				routes = append(routes, item)
			}
		}
	}

	return routes, nil
}
