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

type plainScanConfig struct {
	Name string   `json:"name"`
	Port int      `json:"port"`
	Tags []string `json:"tags"`
}

func BenchmarkValueScanJSONStructPlain(b *testing.B) {
	v := NewValue(`{"name":"due","port":8080,"tags":["a","b","c"]}`)

	var cfg plainScanConfig

	b.ReportAllocs()

	for b.Loop() {
		cfg = plainScanConfig{}
		if err := v.Scan(&cfg); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = cfg
}

type defaultScanConfig struct {
	Host string   `json:"host" default:"127.0.0.1"`
	Port int      `json:"port" default:"9000"`
	Name string   `json:"name"`
	Tags []string `json:"tags" default:"a,b"`
}

func BenchmarkValueScanJSONStructDefaults(b *testing.B) {
	v := NewValue(`{"name":"due","port":8080,"tags":["x","y"]}`)

	var cfg defaultScanConfig

	b.ReportAllocs()

	for b.Loop() {
		cfg = defaultScanConfig{}
		if err := v.Scan(&cfg); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = cfg
}

type nestedDefaultScanConfig struct {
	Host  string              `json:"host" default:"127.0.0.1"`
	Inner defaultScanConfig   `json:"inner"`
	Items []defaultScanConfig `json:"items"`
}

func BenchmarkValueScanJSONStructNestedDefaults(b *testing.B) {
	v := NewValue(`{"inner":{"name":"due","port":8080,"tags":["x","y"]},` +
		`"items":[{"name":"a"},{"port":"9090"}]}`)

	var cfg nestedDefaultScanConfig

	b.ReportAllocs()

	for b.Loop() {
		cfg = nestedDefaultScanConfig{}
		if err := v.Scan(&cfg); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = cfg
}

func BenchmarkValueScanJSONStructNestedDefaultsViaMap(b *testing.B) {
	v := NewValue(`{"inner":{"name":"due","port":8080,"tags":["x","y"]},` +
		`"items":[{"name":"a"},{"port":"9090"}]}`)

	var cfg nestedDefaultScanConfig

	b.ReportAllocs()

	for b.Loop() {
		cfg = nestedDefaultScanConfig{}
		if err := NewValue(v.Map()).Scan(&cfg); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = cfg
}

func BenchmarkValueScanJSONMap(b *testing.B) {
	v := NewValue(`{"host":"0.0.0.0","port":9000,"tags":["a","b"],"nested":{"count":1}}`)

	var dst map[string]any

	b.ReportAllocs()

	for b.Loop() {
		dst = nil
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanStringSlice(b *testing.B) {
	v := NewValue("alpha,beta,gamma,delta")

	var dst []string

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanStringIntSlice(b *testing.B) {
	v := NewValue("1,2,3,4")

	var dst []int

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanMapString(b *testing.B) {
	v := NewValue(map[string]any{"host": "0.0.0.0", "realm": "due", "mode": "release"})

	var dst map[string]string

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanJSONMapString(b *testing.B) {
	v := NewValue(`{"host":"0.0.0.0","realm":"due","mode":"release"}`)

	var dst map[string]string

	b.ReportAllocs()

	for b.Loop() {
		dst = nil
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanJSONMapInt(b *testing.B) {
	v := NewValue(`{"read":1,"write":2,"exec":3}`)

	var dst map[string]int

	b.ReportAllocs()

	for b.Loop() {
		dst = nil
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanJSONStringSlice(b *testing.B) {
	v := NewValue(`["alpha","beta","gamma","delta"]`)

	var dst []string

	b.ReportAllocs()

	for b.Loop() {
		dst = nil
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanJSONIntSlice(b *testing.B) {
	v := NewValue(`[1,2,3,4]`)

	var dst []int

	b.ReportAllocs()

	for b.Loop() {
		dst = nil
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanJSONStructSlice(b *testing.B) {
	v := NewValue(`[{"name":"a","port":8080},{"name":"b","port":9090}]`)

	var dst []plainScanConfig

	b.ReportAllocs()

	for b.Loop() {
		dst = nil
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
}

func BenchmarkValueScanMapInt(b *testing.B) {
	v := NewValue(map[string]any{"read": 1, "write": 2, "exec": "3"})

	var dst map[string]int

	b.ReportAllocs()

	for b.Loop() {
		if err := v.Scan(&dst); err != nil {
			b.Fatal(err)
		}
	}

	benchmarkSink = dst
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
