package value

import (
	"reflect"
	"strings"
	"sync"
)

const (
	_jsonTagKey    = "json"
	_defaultTagKey = "default"
)

type fieldInfo struct {
	name     string
	lower    string
	index    []int
	def      string
	defValue any
}

type structInfo struct {
	fields []fieldInfo
	lowers map[string]struct{}
}

var _structInfos sync.Map // map[reflect.Type]*structInfo

// structInfoOf returns the cached scan metadata of t, building it on first use.
func structInfoOf(t reflect.Type) *structInfo {
	if cached, ok := _structInfos.Load(t); ok {
		return cached.(*structInfo)
	}

	info := &structInfo{}
	buildStructInfo(t, nil, info, make(map[reflect.Type]bool))
	info.lowers = make(map[string]struct{}, len(info.fields))
	for _, field := range info.fields {
		info.lowers[field.lower] = struct{}{}
	}
	actual, _ := _structInfos.LoadOrStore(t, info)

	return actual.(*structInfo)
}

// buildStructInfo collects exported fields in declaration order, flattening anonymous structs to
// follow the field-promotion rules of encoding/json.
func buildStructInfo(
	t reflect.Type,
	prefix []int,
	info *structInfo,
	visited map[reflect.Type]bool,
) {
	if visited[t] {
		return
	}
	visited[t] = true
	defer delete(visited, t)

	for i := range t.NumField() {
		field := t.Field(i)
		name := jsonFieldName(field)
		if name == "-" {
			continue
		}

		index := make([]int, len(prefix)+1)
		copy(index, prefix)
		index[len(prefix)] = i

		if field.Anonymous && name == "" {
			embedded := field.Type
			if embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				buildStructInfo(embedded, index, info, visited)
				continue
			}
		}

		if field.PkgPath != "" {
			continue
		}
		if name == "" {
			name = field.Name
		}

		def := field.Tag.Get(_defaultTagKey)
		info.fields = append(info.fields, fieldInfo{
			name:     name,
			lower:    strings.ToLower(name),
			index:    index,
			def:      def,
			defValue: parseDefaultValue(def, field.Type),
		})
	}
}

// jsonFieldName returns the field name declared by the json tag.
func jsonFieldName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get(_jsonTagKey), ",")

	return name
}

// parseDefaultValue decodes a default tag once so repeated scans do not re-parse it. Scalar and
// custom-encoded defaults keep their text form.
func parseDefaultValue(def string, t reflect.Type) any {
	if def == "" {
		return nil
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if unmarshalerKindOf(t) != _unmarshalerNone {
		return def
	}

	switch t.Kind() {
	case reflect.Struct, reflect.Map:
		if obj, err := decodeObject([]byte(def)); err == nil && obj != nil {
			return obj
		}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return def
		}
		if items, err := splitItems(def); err == nil && items != nil {
			return items
		}
	}

	return def
}
