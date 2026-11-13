package value

import (
	"strconv"
	"testing"
)

var benchmarkSink any

func BenchmarkValueScanPrimitive(b *testing.B) {
	v := NewValue("8080")

	var n int

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&n); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = n
}

func BenchmarkValueScanStruct(b *testing.B) {
	v := NewValue(map[string]any{
		"NAME":    "svc",
		"retries": "5",
		"timeout": "2s",
		"tags":    "x, y",
		"nested":  map[string]any{"port": "8080"},
	})

	var cfg scanConfig

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&cfg); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = cfg
}

func BenchmarkValueScanStructLargeMap(b *testing.B) {
	src := map[string]any{
		"NAME":    "svc",
		"ENABLE":  "true",
		"TIMEOUT": "2s",
		"RETRIES": "5",
		"RATIO":   "0.5",
		"TAGS":    "x, y",
		"NESTED":  map[string]any{"port": "8080"},
		"COUNT":   1,
	}
	for i := range 120 {
		src["EXTRA_"+strconv.Itoa(i)] = i
	}

	v := NewValue(src)

	var cfg scanConfig

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&cfg); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = cfg
}

func BenchmarkValueScanMapAny(b *testing.B) {
	v := NewValue(map[string]any{
		"host": "0.0.0.0",
		"port": 9000,
		"tags": []any{"a", "b"},
	})

	var m map[string]any

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&m); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = m
}

func BenchmarkValueScanIntSlice(b *testing.B) {
	src := make([]int, 1024)
	for i := range src {
		src[i] = i
	}

	v := NewValue(src)

	var dst []int64

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}
