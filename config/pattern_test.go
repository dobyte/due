package config_test

import (
	"testing"

	"github.com/dobyte/due/v2/config"
)

func TestIsWildcard(t *testing.T) {
	tests := []struct {
		entry string
		want  bool
	}{
		{entry: "config.json", want: false},
		{entry: "*.json", want: true},
		{entry: "gate-*.json", want: true},
		{entry: "*", want: true},
	}

	for _, tt := range tests {
		if got := config.IsWildcard(tt.entry); got != tt.want {
			t.Fatalf("IsWildcard(%q) = %v, want %v", tt.entry, got, tt.want)
		}
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "suffix", pattern: "*.yaml", value: "gate.yaml", want: true},
		{name: "prefix", pattern: "gate*", value: "gate.yaml", want: true},
		{name: "middle segment", pattern: "gate-*-prod.json", value: "gate-a-prod.json", want: true},
		{name: "middle segment across separator", pattern: "gate-*-prod.json", value: "gate-a/b-prod.json", want: true},
		{name: "multiple segments", pattern: "a*b*c", value: "a-1-b-2-c", want: true},
		{name: "multiple segments mismatch", pattern: "a*b*c", value: "a-1-b-2-d", want: false},
		{name: "middle segment missing", pattern: "gate-*-prod.json", value: "gate-a-dev.json", want: false},
		{name: "middle segment order", pattern: "gate-*-prod-*.json", value: "gate-prod-1-a.json", want: false},
		{name: "only wildcard", pattern: "*", value: "any/path.json", want: true},
		{name: "empty segments", pattern: "a**b", value: "ab", want: true},
		{name: "mismatch", pattern: "gate-*.json", value: "gate-a.yaml", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.MatchWildcard(tt.pattern, tt.value); got != tt.want {
				t.Fatalf("MatchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestMatchFile(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		value string
		want  bool
	}{
		{name: "empty list", files: nil, value: "config.json", want: false},
		{name: "exact", files: []string{"config.json"}, value: "config.json", want: true},
		{name: "exact mismatch", files: []string{"config.json"}, value: "gate.json", want: false},
		{name: "pattern", files: []string{"config.json", "gate-*.json"}, value: "gate-a.json", want: true},
		{name: "star matches separator", files: []string{"rooms/*.yaml"}, value: "rooms/a/b.yaml", want: true},
		{name: "literal meta in exact entry", files: []string{"weird[1]?.json"}, value: "weird[1]?.json", want: true},
		{name: "literal meta is not a wildcard", files: []string{"weird[1]?.json"}, value: "weird1.json", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.MatchFile(tt.files, tt.value); got != tt.want {
				t.Fatalf("MatchFile(%v, %q) = %v, want %v", tt.files, tt.value, got, tt.want)
			}
		})
	}
}
