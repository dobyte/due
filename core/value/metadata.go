package value

import (
	"reflect"
	"strings"
	"sync"
)

const (
	jsonTagKey    = "json"
	defaultTagKey = "default"
)

type fieldInfo struct {
	name        string
	lower       string
	index       []int
	typ         reflect.Type
	def         string
	defValue    any
	unmarshaler unmarshalerKind
}

type structInfo struct {
	fields           []fieldInfo
	byName           map[string]int
	byLower          map[string]int
	hasDefaults      bool
	hasNestedDefault bool
}

type structInfoCache struct {
	mu sync.RWMutex
	m  map[reflect.Type]*structInfo
}

var structInfos = structInfoCache{m: make(map[reflect.Type]*structInfo)}

// structInfoOf returns the cached scan metadata of t, building it on first use.
func structInfoOf(t reflect.Type) *structInfo {
	structInfos.mu.RLock()
	cached := structInfos.m[t]
	structInfos.mu.RUnlock()
	if cached != nil {
		return cached
	}

	info := &structInfo{}
	buildStructInfo(t, nil, info, make(map[reflect.Type]bool))
	info.byName = make(map[string]int, len(info.fields))
	info.byLower = make(map[string]int, len(info.fields))
	for i, field := range info.fields {
		info.byName[field.name] = i
		info.byLower[field.lower] = i
	}
	info.hasDefaults = typeHasDefaults(t, make(map[reflect.Type]bool))
	info.hasNestedDefault = typeHasNestedDefaults(t, make(map[reflect.Type]bool))

	structInfos.mu.Lock()
	if cached = structInfos.m[t]; cached == nil {
		structInfos.m[t] = info
		cached = info
	}
	structInfos.mu.Unlock()

	return cached
}

// fieldIndex resolves a source key to a field index, preferring an exact name match.
func (info *structInfo) fieldIndex(key string) (int, bool) {
	if index, ok := info.byName[key]; ok {
		return index, true
	}

	index, ok := info.byLower[strings.ToLower(key)]

	return index, ok
}

// typeHasDefaults reports whether t or any nested type declares a default tag.
func typeHasDefaults(t reflect.Type, seen map[reflect.Type]bool) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	defer delete(seen, t)

	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			field := t.Field(i)
			if field.PkgPath == "" && field.Tag.Get(defaultTagKey) != "" {
				return true
			}
			if typeHasDefaults(field.Type, seen) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		return typeHasDefaults(t.Elem(), seen)
	case reflect.Map:
		return typeHasDefaults(t.Elem(), seen)
	}

	return false
}

// typeHasNestedDefaults reports whether any field type of t contains default tags, excluding the
// tags declared by t itself.
func typeHasNestedDefaults(t reflect.Type, seen map[reflect.Type]bool) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	defer delete(seen, t)

	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			if typeHasDefaults(t.Field(i).Type, seen) {
				return true
			}
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		return typeHasDefaults(t.Elem(), seen)
	}

	return false
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

		def := field.Tag.Get(defaultTagKey)
		info.fields = append(info.fields, fieldInfo{
			name:        name,
			lower:       strings.ToLower(name),
			index:       index,
			typ:         field.Type,
			def:         def,
			defValue:    parseDefaultValue(def, field.Type),
			unmarshaler: unmarshalerKindOf(field.Type),
		})
	}
}

// jsonFieldName returns the field name declared by the json tag.
func jsonFieldName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get(jsonTagKey), ",")

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
	if unmarshalerKindOf(t) != unmarshalerNone {
		return def
	}

	switch t.Kind() {
	case reflect.Struct, reflect.Map:
		if obj, err := decodeObjectString(def); err == nil && obj != nil {
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
