package file

import (
	"slices"
	"testing"
)

func TestUnmatchableFiles(t *testing.T) {
	tests := []struct {
		name string
		give []string
		want []string
	}{
		{name: "empty list", give: nil, want: nil},
		{name: "relative", give: []string{"config.json", "./gate.yaml", "gateway/*.yaml", ""}, want: nil},
		{name: "absolute", give: []string{"/etc/gate.yaml"}, want: []string{"/etc/gate.yaml"}},
		{name: "traversal", give: []string{"../gate.yaml", "..", "a/../../b"}, want: []string{"../gate.yaml", "..", "../b"}},
		{name: "pattern traversal", give: []string{"../*.yaml"}, want: []string{"../*.yaml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unmatchableFiles(tt.give); !slices.Equal(got, tt.want) {
				t.Fatalf("expect %v, got %v", tt.want, got)
			}
		})
	}
}
