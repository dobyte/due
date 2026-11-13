package consul

import (
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

func TestMarshalMetaRoutes(t *testing.T) {
	tests := []struct {
		name string
		give []registry.Route
		want map[string]string
	}{
		{
			name: "empty",
			give: nil,
			want: map[string]string{},
		},
		{
			name: "single route without flags",
			give: []registry.Route{{ID: 1}},
			want: map[string]string{metaFieldRoutes + "-0": "-:1"},
		},
		{
			name: "single route with all flags",
			give: []registry.Route{{ID: 1, Internal: true, Stateful: true, Authorized: true}},
			want: map[string]string{metaFieldRoutes + "-0": "isa:1"},
		},
		{
			name: "unsorted contiguous routes",
			give: []registry.Route{
				{ID: 3, Stateful: true},
				{ID: 1, Stateful: true},
				{ID: 2, Stateful: true},
			},
			want: map[string]string{metaFieldRoutes + "-0": "s:1-3"},
		},
		{
			name: "routes grouped by flags and deduplicated",
			give: []registry.Route{
				{ID: 2, Stateful: true},
				{ID: 1, Internal: true},
				{ID: 3, Authorized: true},
				{ID: 2, Stateful: true},
			},
			want: map[string]string{metaFieldRoutes + "-0": "i:1;s:2;a:3"},
		},
		{
			name: "multiple ranges in one group",
			give: []registry.Route{
				{ID: 1, Stateful: true},
				{ID: 2, Stateful: true},
				{ID: 5, Stateful: true},
				{ID: 7, Stateful: true},
				{ID: 8, Stateful: true},
			},
			want: map[string]string{metaFieldRoutes + "-0": "s:1-2,5,7-8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := marshalMetaRoutes(tt.give)
			if err != nil {
				t.Fatalf("marshalMetaRoutes() error = %v", err)
			}

			if !maps.Equal(got, tt.want) {
				t.Fatalf("marshalMetaRoutes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarshalMetaRoutesNegativeID(t *testing.T) {
	if _, err := marshalMetaRoutes([]registry.Route{{ID: -1}}); err == nil {
		t.Fatal("marshalMetaRoutes() error = nil, want error")
	}
}

func TestMarshalMetaRoutesTooMany(t *testing.T) {
	routes := make([]registry.Route, maxRouteCount+1)
	for i := range routes {
		routes[i].ID = int32(i)
	}

	if _, err := marshalMetaRoutes(routes); !errors.Is(err, errors.ErrTooManyRoutes) {
		t.Fatalf("marshalMetaRoutes() error = %v, want %v", err, errors.ErrTooManyRoutes)
	}
}

func TestMarshalMetaRoutesChunked(t *testing.T) {
	routes := make([]registry.Route, 1000)
	for i := range routes {
		routes[i].ID = int32(i * 2)
		routes[i].Stateful = true
	}

	metas, err := marshalMetaRoutes(routes)
	if err != nil {
		t.Fatalf("marshalMetaRoutes() error = %v", err)
	}

	if len(metas) < 2 {
		t.Fatalf("marshalMetaRoutes() returned %d chunks, want at least 2", len(metas))
	}

	for field, value := range metas {
		if len(value) > metaValueSize {
			t.Fatalf("marshalMetaRoutes() chunk %q length = %d, want at most %d", field, len(value), metaValueSize)
		}
	}

	got, err := unmarshalMetaRoutes(metas)
	if err != nil {
		t.Fatalf("unmarshalMetaRoutes() error = %v", err)
	}

	if !slices.Equal(got, routes) {
		t.Fatalf("unmarshalMetaRoutes(marshalMetaRoutes()) = %+v, want %+v", got, routes)
	}
}

func TestMetaRoutesRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		give []registry.Route
		want []registry.Route
	}{
		{
			name: "empty",
			give: nil,
			want: []registry.Route{},
		},
		{
			name: "single route",
			give: []registry.Route{{ID: 1}},
			want: []registry.Route{{ID: 1}},
		},
		{
			name: "deduplicated and grouped",
			give: []registry.Route{
				{ID: 2, Stateful: true},
				{ID: 1, Internal: true},
				{ID: 2, Stateful: true},
			},
			want: []registry.Route{
				{ID: 1, Internal: true},
				{ID: 2, Stateful: true},
			},
		},
		{
			name: "all flag combinations",
			give: []registry.Route{
				{ID: 1, Internal: true, Stateful: true},
				{ID: 2, Internal: true, Authorized: true},
				{ID: 3, Stateful: true, Authorized: true},
				{ID: 4, Internal: true, Stateful: true, Authorized: true},
				{ID: 5},
			},
			want: []registry.Route{
				{ID: 5},
				{ID: 1, Internal: true, Stateful: true},
				{ID: 2, Internal: true, Authorized: true},
				{ID: 3, Stateful: true, Authorized: true},
				{ID: 4, Internal: true, Stateful: true, Authorized: true},
			},
		},
		{
			name: "max int32 route id",
			give: []registry.Route{
				{ID: math.MaxInt32 - 1, Stateful: true},
				{ID: math.MaxInt32, Stateful: true},
			},
			want: []registry.Route{
				{ID: math.MaxInt32 - 1, Stateful: true},
				{ID: math.MaxInt32, Stateful: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metas, err := marshalMetaRoutes(tt.give)
			if err != nil {
				t.Fatalf("marshalMetaRoutes() error = %v", err)
			}

			got, err := unmarshalMetaRoutes(metas)
			if err != nil {
				t.Fatalf("unmarshalMetaRoutes() error = %v", err)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("unmarshalMetaRoutes(marshalMetaRoutes()) = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalMetaRoutesLegacy(t *testing.T) {
	metas := map[string]string{
		metaFieldRoutes + "-0": "1-1,2-3",
		metaFieldRoutes + "-1": "5-0,-1-2",
	}

	got, err := unmarshalMetaRoutes(metas)
	if err != nil {
		t.Fatalf("unmarshalMetaRoutes() error = %v", err)
	}

	want := []registry.Route{
		{ID: 1, Internal: true},
		{ID: 2, Internal: true, Stateful: true},
		{ID: 5},
		{ID: -1, Stateful: true},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("unmarshalMetaRoutes() = %+v, want %+v", got, want)
	}
}

func TestUnmarshalMetaRoutesInvalid(t *testing.T) {
	tests := []string{
		"x:1",
		"s:",
		"s:1-",
		"s:3-1",
		"s:-1",
		"s:1,,2",
		"s:1-x",
		"isa-:1",
		"s:0-65536",
	}

	for _, give := range tests {
		t.Run(give, func(t *testing.T) {
			metas := map[string]string{metaFieldRoutes + "-0": give}

			if _, err := unmarshalMetaRoutes(metas); err == nil {
				t.Fatalf("unmarshalMetaRoutes(%q) error = nil, want error", give)
			}
		})
	}
}
