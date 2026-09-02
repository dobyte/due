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

const metaValueSize = 512

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

const (
	metaRouteInternal = 1 << iota
	metaRouteStateful
	metaRouteAuthorized
)

// 编码元数据路由
func marshalMetaRoutes(routes []registry.Route) map[string]string {
	var (
		key   string
		size  int
		metas = make(map[string]string)
		items string
	)

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

		if s := len(items); s == 0 {
			size = len(val)
		} else {
			size = s + 1 + len(val)
		}

		if size <= metaValueSize {
			if len(items) == 0 {
				items = val
			} else {
				items += "," + val
			}
		}

		if size >= metaValueSize {
			key = fmt.Sprintf("%s-%d", metaFieldRoutes, len(metas))
			metas[key] = items
		}

		switch {
		case size < metaValueSize:
			// ignore
		case size > metaValueSize:
			items = val
		default:
			items = ""
		}
	}

	if len(items) > 0 {
		key = fmt.Sprintf("%s-%d", metaFieldRoutes, len(metas))
		metas[key] = items
	}

	return metas
}

// 解码元数据路由
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

	// 按分块序号解码，保证多块时路由顺序稳定
	sort.Ints(indexes)

	for _, index := range indexes {
		for _, item := range strings.Split(metas[fmt.Sprintf("%s-%d", metaFieldRoutes, index)], ",") {
			val := strings.Split(item, "-")

			if len(val) != 2 {
				continue
			}

			opts := xconv.Int(val[1])

			routes = append(routes, registry.Route{
				ID:         xconv.Int32(val[0]),
				Internal:   opts&metaRouteInternal != 0,
				Stateful:   opts&metaRouteStateful != 0,
				Authorized: opts&metaRouteAuthorized != 0,
			})
		}
	}

	return routes
}

// 编码元数据列表
// 由于 Consul Meta 单条 value 存在长度限制（metaValueSize），较大的列表需要分块存储。
// 每块均为一个合法的 JSON 数组，meta key 形如 <field>-<index>
// @param field string 字段名
// @param list []T 待编码的列表
// @return map[string]string 编码后的 meta 键值对
// @return error 编码失败时返回错误
func marshalMetaList[T any](field string, list []T) (map[string]string, error) {
	metas := make(map[string]string)

	if len(list) == 0 {
		return metas, nil
	}

	var (
		chunk []T
		size  int // 当前块 JSON 数组的精确字节长度
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

		// 单个元素即使独占一块也无法容纳时直接返回错误
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

// 解码元数据列表
// 兼容旧版本未分块、直接以字段名作为 key 的存储格式
// @param field string 字段名
// @param metas map[string]string Consul Meta
// @return []T 解码后的列表
// @return error 解码失败时返回错误
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

	// 兼容旧版本未分块存储格式
	if v, ok := metas[field]; ok {
		chunk := make([]T, 0)

		if err := json.Unmarshal(xconv.StringToBytes(v), &chunk); err != nil {
			return nil, err
		}

		list = append(list, chunk...)
	}

	return list, nil
}
