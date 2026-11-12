package nacos

import (
	"math"
	"slices"
	"testing"

	"github.com/dobyte/due/v2/registry"
)

func TestMarshalRoutes(t *testing.T) {
	tests := []struct {
		name string
		give []registry.Route
		want string
	}{
		{
			name: "empty",
			give: nil,
			want: "",
		},
		{
			name: "single route without flags",
			give: []registry.Route{{ID: 1}},
			want: "-:1",
		},
		{
			name: "single route with all flags",
			give: []registry.Route{{ID: 1, Internal: true, Stateful: true, Authorized: true}},
			want: "isa:1",
		},
		{
			name: "unsorted contiguous routes",
			give: []registry.Route{
				{ID: 3, Stateful: true},
				{ID: 1, Stateful: true},
				{ID: 2, Stateful: true},
			},
			want: "s:1-3",
		},
		{
			name: "routes grouped by flags",
			give: []registry.Route{
				{ID: 2, Stateful: true},
				{ID: 1, Internal: true},
				{ID: 3, Authorized: true},
				{ID: 2, Stateful: true},
			},
			want: "i:1;s:2;a:3",
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
			want: "s:1-2,5,7-8",
		},
		{
			name: "max int32 route id",
			give: []registry.Route{
				{ID: math.MaxInt32 - 1, Stateful: true},
				{ID: math.MaxInt32, Stateful: true},
			},
			want: "s:2147483646-2147483647",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := marshalRoutes(tt.give)
			if err != nil {
				t.Fatalf("marshalRoutes() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("marshalRoutes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarshalRoutesNegativeID(t *testing.T) {
	if _, err := marshalRoutes([]registry.Route{{ID: -1}}); err == nil {
		t.Fatal("marshalRoutes() error = nil, want error")
	}
}

func TestRoutesRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		give []registry.Route
		want []registry.Route
	}{
		{
			name: "empty",
			give: nil,
			want: nil,
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
			// Groups are emitted in ascending flag order, so the internal route comes first.
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
			// Route 5 has no flags, so its group is emitted before the flag groups.
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
			value, err := marshalRoutes(tt.give)
			if err != nil {
				t.Fatalf("marshalRoutes() error = %v", err)
			}

			got, err := unmarshalRoutes(value)
			if err != nil {
				t.Fatalf("unmarshalRoutes(%q) error = %v", value, err)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("unmarshalRoutes(marshalRoutes()) = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalRoutesLegacyJSON(t *testing.T) {
	got, err := unmarshalRoutes(`[{"i":1,"n":true,"s":true},{"i":2,"a":true}]`)
	if err != nil {
		t.Fatalf("unmarshalRoutes() error = %v", err)
	}

	want := []registry.Route{
		{ID: 1, Internal: true, Stateful: true},
		{ID: 2, Authorized: true},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("unmarshalRoutes() = %+v, want %+v", got, want)
	}
}

func TestUnmarshalRoutesInvalid(t *testing.T) {
	tests := []string{
		"s",
		"x:1",
		"s:",
		"s:1-",
		"s:3-1",
		"s:-1",
		"s:1,,2",
		"s:1-x",
		"isa-:1",
		"s:0-1048576",
	}

	for _, give := range tests {
		t.Run(give, func(t *testing.T) {
			if _, err := unmarshalRoutes(give); err == nil {
				t.Fatalf("unmarshalRoutes(%q) error = nil, want error", give)
			}
		})
	}
}
