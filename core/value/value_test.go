package value

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
)

type scanStatus int

type scanUint uint

type scanBool bool

type scanFlag bool

type scanDuration time.Duration

type scanUpperText string

type scanText string

type scanBytes []byte

func (s *scanUpperText) UnmarshalText(text []byte) error {
	*s = scanUpperText(strings.ToUpper(string(text)))

	return nil
}

type scanNestedConfig struct {
	Host string `json:"host" default:"127.0.0.1"`
	Port int    `json:"port" default:"9000"`
}

type scanConfig struct {
	Name    string            `json:"name" default:"due"`
	Enable  bool              `json:"enable" default:"true"`
	Timeout time.Duration     `json:"timeout" default:"5s"`
	Retries uint8             `json:"retries" default:"3"`
	Ratio   float64           `json:"ratio" default:"0.5"`
	Tags    []string          `json:"tags" default:"a,b"`
	Nested  scanNestedConfig  `json:"nested"`
	Pointer *scanNestedConfig `json:"pointer" default:"{\"host\":\"ptr\"}"`
	Count   int               `json:"count" default:"10"`
	Skip    string            `json:"-"`
	plain   string
}

func TestScanScalarConversion(t *testing.T) {
	tests := []struct {
		name string
		src  any
		dest func() any
		want any
	}{
		{name: "string to int", src: "8080", dest: func() any { return new(int) }, want: 8080},
		{name: "number to string", src: 8080, dest: func() any { return new(string) }, want: "8080"},
		{name: "string to bool", src: "true", dest: func() any { return new(bool) }, want: true},
		{name: "number to bool", src: 1, dest: func() any { return new(bool) }, want: true},
		{name: "string to uint8", src: "255", dest: func() any { return new(uint8) }, want: uint8(255)},
		{name: "float to int", src: float64(3), dest: func() any { return new(scanStatus) }, want: scanStatus(3)},
		{name: "int to float32", src: 3, dest: func() any { return new(float32) }, want: float32(3)},
		{name: "string to duration", src: "1h30m", dest: func() any { return new(time.Duration) }, want: 90 * time.Minute},
		{name: "day to duration", src: "1.5d", dest: func() any { return new(time.Duration) }, want: 36 * time.Hour},
		{name: "string to int slice", src: "1,2,3", dest: func() any { return new([]int) }, want: []int{1, 2, 3}},
		{name: "json to string slice", src: `["a","b"]`, dest: func() any { return new([]string) }, want: []string{"a", "b"}},
		{name: "string to bytes", src: "due", dest: func() any { return new([]byte) }, want: []byte("due")},
		{name: "json to map", src: `{"a":1}`, dest: func() any { return new(map[string]int) }, want: map[string]int{"a": 1}},
		{name: "json to int keyed map", src: map[string]any{"1": "a"}, dest: func() any { return new(map[int]string) }, want: map[int]string{1: "a"}},
		{name: "text unmarshaler", src: "abc", dest: func() any { return new(scanUpperText) }, want: scanUpperText("ABC")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := tt.dest()
			if err := NewValue(tt.src).Scan(dest); err != nil {
				t.Fatalf("scan failed, err: %v", err)
			}

			if got := reflect.ValueOf(dest).Elem().Interface(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("invalid scanned value, expect: %v, actual: %v", tt.want, got)
			}
		})
	}
}

func TestScanNamedSourceTypes(t *testing.T) {
	var cfg struct {
		Count   int           `json:"count"`
		Enabled bool          `json:"enabled"`
		Flag    bool          `json:"flag"`
		Timeout time.Duration `json:"timeout"`
		Tags    []int         `json:"tags"`
		Raw     []byte        `json:"raw"`
	}

	src := map[string]any{
		"count":   scanText("10"),
		"enabled": scanText("true"),
		"flag":    scanFlag(true),
		"timeout": scanText("1H"),
		"tags":    scanText("1,2,3"),
		"raw":     scanBytes("due"),
	}

	if err := NewValue(src).Scan(&cfg); err != nil {
		t.Fatalf("scan named sources failed, err: %v", err)
	}
	if cfg.Count != 10 || !cfg.Enabled || !cfg.Flag || cfg.Timeout != time.Hour {
		t.Fatalf("invalid scalar values, actual: %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.Tags, []int{1, 2, 3}) {
		t.Fatalf("invalid tags, actual: %v", cfg.Tags)
	}
	if string(cfg.Raw) != "due" {
		t.Fatalf("invalid raw bytes, actual: %s", cfg.Raw)
	}

	var nested scanConfig
	if err := NewValue(scanText(`{"name":"named","retries":"4"}`)).Scan(&nested); err != nil {
		t.Fatalf("scan named object failed, err: %v", err)
	}
	if nested.Name != "named" || nested.Retries != 4 {
		t.Fatalf("invalid named config, actual: %+v", nested)
	}
}

func TestScanScalarConversionError(t *testing.T) {
	tests := []struct {
		name string
		src  any
		dest any
	}{
		{name: "invalid integer", src: "abc", dest: new(scanStatus)},
		{name: "fractional integer", src: 3.5, dest: new(scanStatus)},
		{name: "negative unsigned", src: -1, dest: new(scanUint)},
		{name: "invalid bool", src: "not-bool", dest: new(scanBool)},
		{name: "invalid duration", src: "soon", dest: new(scanDuration)},
		{name: "invalid slice", src: "1,x", dest: new([]int)},
		{name: "invalid map source", src: 1, dest: new(map[string]int)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NewValue(tt.src).Scan(tt.dest); err == nil {
				t.Fatal("expect an error, actual: nil")
			}
		})
	}
}

func TestScanRejectsInvalidPointer(t *testing.T) {
	if err := NewValue("1").Scan(1); !errors.Is(err, errors.ErrInvalidScanner) {
		t.Fatalf("expect ErrInvalidScanner, actual: %v", err)
	}

	var pointer *int
	if err := NewValue("1").Scan(pointer); !errors.Is(err, errors.ErrInvalidScanner) {
		t.Fatalf("expect ErrInvalidScanner, actual: %v", err)
	}
}

func TestScanStructDefaultAndConversion(t *testing.T) {
	src := map[string]any{
		"NAME":    "svc",
		"retries": "5",
		"timeout": "2s",
		"tags":    "x, y",
		"nested":  map[string]any{"port": "8080"},
		"count":   0,
		"Skip":    "ignored",
	}

	var cfg scanConfig
	if err := NewValue(src).Scan(&cfg); err != nil {
		t.Fatalf("scan struct failed, err: %v", err)
	}

	if cfg.Name != "svc" {
		t.Fatalf("invalid name, expect: svc, actual: %s", cfg.Name)
	}
	if !cfg.Enable {
		t.Fatal("expect the default of enable to be true")
	}
	if cfg.Timeout != 2*time.Second {
		t.Fatalf("invalid timeout, expect: 2s, actual: %s", cfg.Timeout)
	}
	if cfg.Retries != 5 {
		t.Fatalf("invalid retries, expect: 5, actual: %d", cfg.Retries)
	}
	if cfg.Ratio != 0.5 {
		t.Fatalf("invalid ratio, expect: 0.5, actual: %f", cfg.Ratio)
	}
	if !reflect.DeepEqual(cfg.Tags, []string{"x", "y"}) {
		t.Fatalf("invalid tags, expect: [x y], actual: %v", cfg.Tags)
	}
	if cfg.Nested.Host != "127.0.0.1" || cfg.Nested.Port != 8080 {
		t.Fatalf("invalid nested config, actual: %+v", cfg.Nested)
	}
	if cfg.Pointer == nil || cfg.Pointer.Host != "ptr" || cfg.Pointer.Port != 9000 {
		t.Fatalf("invalid pointer config, actual: %+v", cfg.Pointer)
	}
	if cfg.Count != 0 {
		t.Fatalf("explicit zero should override the default, actual: %d", cfg.Count)
	}
	if cfg.Skip != "" {
		t.Fatalf("json ignored field should not be scanned, actual: %s", cfg.Skip)
	}
}

func TestScanStructFromEmptyValue(t *testing.T) {
	var cfg scanConfig
	if err := NewValue().Scan(&cfg); err != nil {
		t.Fatalf("scan defaults failed, err: %v", err)
	}

	if cfg.Name != "due" || !cfg.Enable || cfg.Timeout != 5*time.Second || cfg.Retries != 3 || cfg.Ratio != 0.5 {
		t.Fatalf("invalid default config, actual: %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.Tags, []string{"a", "b"}) {
		t.Fatalf("invalid default tags, actual: %v", cfg.Tags)
	}
	if cfg.Nested.Host != "127.0.0.1" || cfg.Nested.Port != 9000 {
		t.Fatalf("invalid default nested config, actual: %+v", cfg.Nested)
	}
	if cfg.Pointer == nil || cfg.Pointer.Host != "ptr" || cfg.Pointer.Port != 9000 {
		t.Fatalf("invalid default pointer config, actual: %+v", cfg.Pointer)
	}
	if cfg.Count != 10 {
		t.Fatalf("invalid default count, expect: 10, actual: %d", cfg.Count)
	}
}

func TestScanStructFromJSONString(t *testing.T) {
	var cfg scanConfig
	if err := NewValue(`{"name":"json","nested":{"host":"10.0.0.1"}}`).Scan(&cfg); err != nil {
		t.Fatalf("scan json failed, err: %v", err)
	}

	if cfg.Name != "json" {
		t.Fatalf("invalid name, expect: json, actual: %s", cfg.Name)
	}
	if cfg.Nested.Host != "10.0.0.1" || cfg.Nested.Port != 9000 {
		t.Fatalf("invalid nested config, actual: %+v", cfg.Nested)
	}
	if !cfg.Enable {
		t.Fatal("expect the default of enable to be true")
	}
}

func TestScanStructEmptyStringPolicy(t *testing.T) {
	var cfg scanConfig
	if err := NewValue(map[string]any{"name": "", "retries": scanText("")}).Scan(&cfg); err != nil {
		t.Fatalf("scan failed, err: %v", err)
	}

	if cfg.Name != "" {
		t.Fatalf("empty string should be kept for string fields, actual: %s", cfg.Name)
	}
	if cfg.Retries != 3 {
		t.Fatalf("empty string should fall back to the default, actual: %d", cfg.Retries)
	}
}

func TestScanStructEmptyCollectionPolicy(t *testing.T) {
	type collectionConfig struct {
		Tags []string       `json:"tags" default:"a,b"`
		Meta map[string]int `json:"meta" default:"{\"count\":1}"`
		Raw  []byte         `json:"raw" default:"due"`
	}

	var cfg collectionConfig
	if err := NewValue(map[string]any{"tags": "", "meta": "", "raw": ""}).Scan(&cfg); err != nil {
		t.Fatalf("scan failed, err: %v", err)
	}
	if !reflect.DeepEqual(cfg.Tags, []string{"a", "b"}) {
		t.Fatalf("empty string should fall back to the slice default, actual: %v", cfg.Tags)
	}
	if !reflect.DeepEqual(cfg.Meta, map[string]int{"count": 1}) {
		t.Fatalf("empty string should fall back to the map default, actual: %v", cfg.Meta)
	}
	if len(cfg.Raw) != 0 {
		t.Fatalf("empty string should be kept for byte slices, actual: %v", cfg.Raw)
	}

	var empty struct {
		Tags []int `json:"tags"`
	}
	if err := NewValue(map[string]any{"tags": ""}).Scan(&empty); err != nil {
		t.Fatalf("scan failed, err: %v", err)
	}
	if empty.Tags != nil {
		t.Fatalf("empty string should keep the zero value without a default, actual: %v", empty.Tags)
	}
}

func TestScanStructWithEmbeddedField(t *testing.T) {
	type embedded struct {
		Name string `json:"name" default:"embedded"`
	}
	type outer struct {
		embedded
		Port int `json:"port" default:"80"`
	}

	var cfg outer
	if err := NewValue(map[string]any{"port": "8080"}).Scan(&cfg); err != nil {
		t.Fatalf("scan failed, err: %v", err)
	}
	if cfg.Name != "embedded" || cfg.Port != 8080 {
		t.Fatalf("invalid embedded config, actual: %+v", cfg)
	}
}

func TestScanStructToMap(t *testing.T) {
	src := struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}{Name: "svc", Port: 8080}

	var dst map[string]any
	if err := NewValue(src).Scan(&dst); err != nil {
		t.Fatalf("scan failed, err: %v", err)
	}

	if dst["name"] != "svc" || dst["port"] != 8080 {
		t.Fatalf("invalid scanned map, actual: %v", dst)
	}
}

func TestScanMapDoesNotAliasSource(t *testing.T) {
	src := map[string]any{"nested": map[string]any{"count": 1}}

	var dst map[string]any
	if err := NewValue(src).Scan(&dst); err != nil {
		t.Fatalf("scan failed, err: %v", err)
	}

	dst["nested"].(map[string]any)["count"] = 2
	if got := src["nested"].(map[string]any)["count"]; got != 1 {
		t.Fatalf("scanned map should not alias the source, actual: %v", got)
	}
}

func TestScanDefaultError(t *testing.T) {
	type badConfig struct {
		Port int `json:"port" default:"abc"`
	}

	var cfg badConfig
	err := NewValue().Scan(&cfg)
	if err == nil {
		t.Fatal("expect an error for an invalid default value, actual: nil")
	}
	if !strings.Contains(err.Error(), "port") {
		t.Fatalf("error should contain the field path, actual: %v", err)
	}
}

func TestScanTime(t *testing.T) {
	var tm time.Time
	if err := NewValue("2026-11-13T10:00:00Z").Scan(&tm); err != nil {
		t.Fatalf("scan time failed, err: %v", err)
	}
	if got := tm.Format(time.RFC3339); got != "2026-11-13T10:00:00Z" {
		t.Fatalf("invalid time, actual: %s", got)
	}
}

func TestScanIntoAny(t *testing.T) {
	var dst any
	if err := NewValue(map[string]any{"count": 1}).Scan(&dst); err != nil {
		t.Fatalf("scan any failed, err: %v", err)
	}

	obj, ok := dst.(map[string]any)
	if !ok || obj["count"] != 1 {
		t.Fatalf("invalid scanned any, actual: %v", dst)
	}
}

func TestMapStillWorks(t *testing.T) {
	m := NewValue(map[string]any{"count": 1}).Map()
	if m["count"] != 1 {
		t.Fatalf("invalid map result, actual: %v", m)
	}
}
