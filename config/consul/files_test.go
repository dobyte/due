package consul

import (
	"context"
	"testing"

	"github.com/dobyte/due/v2/config"
)

func TestMatchFile(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		value string
		want  bool
	}{
		{name: "no files", files: nil, value: "config.json", want: true},
		{name: "exact", files: []string{"config.json"}, value: "config.json", want: true},
		{name: "exact mismatch", files: []string{"config.json"}, value: "gate.json", want: false},
		{name: "pattern", files: []string{"gateway/*.yaml"}, value: "gateway/gate.yaml", want: true},
		{name: "pattern mismatch", files: []string{"gateway/*.yaml"}, value: "config.json", want: false},
		{name: "wildcard crosses separator", files: []string{"*.yaml"}, value: "gateway/gate.yaml", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{opts: &options{files: tt.files}}

			if got := s.matchFile(tt.value); got != tt.want {
				t.Fatalf("matchFile(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestStoreRejectsEmptyFile(t *testing.T) {
	s := &Source{opts: &options{mode: config.ReadWrite}}

	if err := s.Store(context.Background(), "", []byte(`{}`)); err == nil {
		t.Fatal("expect an error for the empty file name")
	}
}
