package core

import (
	"slices"
	"testing"
)

func TestNormalizeFiles(t *testing.T) {
	tests := []struct {
		name string
		give []string
		want []string
	}{
		{name: "empty list", give: nil, want: nil},
		{name: "trim and dedupe", give: []string{" a.json ", "", "b.yaml", "a.json", "  "}, want: []string{"a.json", "b.yaml"}},
		{name: "clean", give: []string{"./a.json", "gateway/", "x//y.json", ".//z.json"}, want: []string{"a.json", "gateway", "x/y.json", "z.json"}},
		{name: "all empty", give: []string{"", ".", " ./ "}, want: []string{}},
		{name: "wildcard", give: []string{"./*.yaml", "gateway/*.yaml"}, want: []string{"*.yaml", "gateway/*.yaml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeFiles(tt.give); !slices.Equal(got, tt.want) {
				t.Fatalf("expect %v, got %v", tt.want, got)
			}
		})
	}
}
