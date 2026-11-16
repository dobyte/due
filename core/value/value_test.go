package value

import (
	"fmt"
	"reflect"
	"strconv"
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

type scanKey string

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
		{name: "delimited to string slice", src: "a, b", dest: func() any { return new([]string) }, want: []string{"a", "b"}},
		{name: "delimited to duration slice", src: "1s,2s", dest: func() any { return new([]time.Duration) }, want: []time.Duration{time.Second, 2 * time.Second}},
		{name: "json to string slice", src: `["a","b"]`, dest: func() any { return new([]string) }, want: []string{"a", "b"}},
		{name: "string to bytes", src: "due", dest: func() any { return new([]byte) }, want: []byte("due")},
		{name: "json to map", src: `{"a":1}`, dest: func() any { return new(map[string]int) }, want: map[string]int{"a": 1}},
		{name: "string map", src: map[string]any{"a": "x", "b": 2}, dest: func() any { return new(map[string]string) }, want: map[string]string{"a": "x", "b": "2"}},
		{name: "int map", src: map[string]any{"a": 1, "b": "2"}, dest: func() any { return new(map[string]int) }, want: map[string]int{"a": 1, "b": 2}},
		{name: "int64 map", src: map[string]any{"a": 1, "b": "2"}, dest: func() any { return new(map[string]int64) }, want: map[string]int64{"a": 1, "b": 2}},
		{name: "bool map", src: map[string]any{"a": true, "b": "false"}, dest: func() any { return new(map[string]bool) }, want: map[string]bool{"a": true, "b": false}},
		{name: "any slice to string slice", src: []any{"a", "b"}, dest: func() any { return new([]string) }, want: []string{"a", "b"}},
		{name: "named key map", src: map[string]any{"a": 1}, dest: func() any { return new(map[scanKey]int) }, want: map[scanKey]int{"a": 1}},
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
		RawText []byte        `json:"rawText"`
	}

	src := map[string]any{
		"count":   scanText("10"),
		"enabled": scanText("true"),
		"flag":    scanFlag(true),
		"timeout": scanText("1H"),
		"tags":    scanText("1,2,3"),
		"raw":     scanBytes("due"),
		"rawText": scanText("due"),
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
	if string(cfg.RawText) != "due" {
		t.Fatalf("invalid raw text bytes, actual: %s", cfg.RawText)
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
		"RETRIES": "5",
		"TIMEOUT": "2s",
		"TAGS":    "x, y",
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

func TestScanDefaultIsolation(t *testing.T) {
	type defaultConfig struct {
		Tags []string       `json:"tags" default:"a,b"`
		Meta map[string]int `json:"meta" default:"{\"count\":1}"`
	}

	var (
		first  defaultConfig
		second defaultConfig
	)

	if err := NewValue().Scan(&first); err != nil {
		t.Fatalf("scan first defaults failed, err: %v", err)
	}
	if err := NewValue().Scan(&second); err != nil {
		t.Fatalf("scan second defaults failed, err: %v", err)
	}

	first.Tags[0] = "changed"
	first.Meta["count"] = 2

	if second.Tags[0] != "a" {
		t.Fatalf("cached default slices should not be shared, actual: %v", second.Tags)
	}
	if second.Meta["count"] != 1 {
		t.Fatalf("cached default maps should not be shared, actual: %v", second.Meta)
	}
}

func TestScanJSONStructDirect(t *testing.T) {
	type plainConfig struct {
		Name string   `json:"name"`
		Port int      `json:"port"`
		Tags []string `json:"tags"`
	}

	var cfg plainConfig
	if err := NewValue(`{"name":"due","port":8080,"tags":["a","b"]}`).Scan(&cfg); err != nil {
		t.Fatalf("scan json struct failed, err: %v", err)
	}
	if cfg.Name != "due" || cfg.Port != 8080 || len(cfg.Tags) != 2 {
		t.Fatalf("invalid direct config, actual: %+v", cfg)
	}

	cfg = plainConfig{}
	if err := NewValue(`{"name":"due","port":"8080"}`).Scan(&cfg); err != nil {
		t.Fatalf("scan fallback struct failed, err: %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("cross-type fallback failed, actual port: %d", cfg.Port)
	}

	type withDefault struct {
		Inner scanNestedConfig `json:"inner"`
	}

	var nested withDefault
	if err := NewValue(`{"inner":{"port":8080}}`).Scan(&nested); err != nil {
		t.Fatalf("scan default struct failed, err: %v", err)
	}
	if nested.Inner.Host != "127.0.0.1" || nested.Inner.Port != 8080 {
		t.Fatalf("nested defaults missing, actual: %+v", nested.Inner)
	}
}

func TestScanJSONStructDefaults(t *testing.T) {
	type defaultConfig struct {
		Host string   `json:"host" default:"127.0.0.1"`
		Port int      `json:"port" default:"9000"`
		Name string   `json:"name"`
		Tags []string `json:"tags" default:"a,b"`
	}

	var cfg defaultConfig
	if err := NewValue(`{"port":8080}`).Scan(&cfg); err != nil {
		t.Fatalf("scan defaults failed, err: %v", err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 8080 || len(cfg.Tags) != 2 {
		t.Fatalf("invalid default config, actual: %+v", cfg)
	}

	cfg = defaultConfig{}
	if err := NewValue(`{"host":null,"port":8080}`).Scan(&cfg); err != nil {
		t.Fatalf("scan null default failed, err: %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("null should fall back to default, actual: %s", cfg.Host)
	}

	cfg = defaultConfig{}
	if err := NewValue(`{"host":"","port":8080}`).Scan(&cfg); err != nil {
		t.Fatalf("scan empty default failed, err: %v", err)
	}
	if cfg.Host != "" {
		t.Fatalf("empty string should be kept for string fields, actual: %s", cfg.Host)
	}

	cfg = defaultConfig{}
	if err := NewValue(`{"port":"8080"}`).Scan(&cfg); err != nil {
		t.Fatalf("scan cross-type default failed, err: %v", err)
	}
	if cfg.Port != 8080 || cfg.Host != "127.0.0.1" {
		t.Fatalf("cross-type fallback failed, actual: %+v", cfg)
	}
}

func TestScanJSONStructNestedDefaults(t *testing.T) {
	type nestedConfig struct {
		Host string `json:"host" default:"127.0.0.1"`
		Port int    `json:"port" default:"9000"`
	}
	type config struct {
		Name  string                  `json:"name" default:"due"`
		Inner nestedConfig            `json:"inner"`
		List  []nestedConfig          `json:"list"`
		Map   map[string]nestedConfig `json:"map"`
		Ptr   *nestedConfig           `json:"ptr"`
	}

	src := `{"inner":{"port":"8080"},` +
		`"list":[{"host":"a"},{"port":1},{"host":""}],` +
		`"map":{"first":{"port":"2"},"second":{"host":null}},` +
		`"ptr":{}}`

	var cfg config
	if err := NewValue(src).Scan(&cfg); err != nil {
		t.Fatalf("scan nested defaults failed, err: %v", err)
	}

	if cfg.Name != "due" || cfg.Inner.Host != "127.0.0.1" || cfg.Inner.Port != 8080 {
		t.Fatalf("invalid nested config, actual: %+v", cfg)
	}
	if len(cfg.List) != 3 {
		t.Fatalf("invalid nested list, actual: %+v", cfg.List)
	}
	if cfg.List[0].Host != "a" || cfg.List[0].Port != 9000 {
		t.Fatalf("invalid first list item, actual: %+v", cfg.List[0])
	}
	if cfg.List[1].Host != "127.0.0.1" || cfg.List[1].Port != 1 {
		t.Fatalf("invalid second list item, actual: %+v", cfg.List[1])
	}
	if cfg.List[2].Host != "" || cfg.List[2].Port != 9000 {
		t.Fatalf("empty string should be kept, actual: %+v", cfg.List[2])
	}
	if cfg.Map["first"].Host != "127.0.0.1" || cfg.Map["first"].Port != 2 {
		t.Fatalf("invalid nested map item, actual: %+v", cfg.Map)
	}
	if cfg.Map["second"].Host != "127.0.0.1" || cfg.Map["second"].Port != 9000 {
		t.Fatalf("null should fall back to the nested default, actual: %+v", cfg.Map)
	}
	if cfg.Ptr == nil || cfg.Ptr.Host != "127.0.0.1" || cfg.Ptr.Port != 9000 {
		t.Fatalf("invalid nested pointer, actual: %+v", cfg.Ptr)
	}
}

func TestScanJSONStructNestedDefaultError(t *testing.T) {
	type nestedConfig struct {
		Port int `json:"port" default:"9000"`
	}
	type config struct {
		Inner nestedConfig `json:"inner"`
	}

	var cfg config
	err := NewValue(`{"inner":{"port":"bad"}}`).Scan(&cfg)
	if err == nil || !strings.Contains(err.Error(), "inner.port") {
		t.Fatalf("expect a nested field path, actual: %v", err)
	}
}

func TestScanJSONStructRawEscapes(t *testing.T) {
	type nestedConfig struct {
		Host string `json:"host" default:"127.0.0.1"`
		Path string `json:"path" default:"/"`
	}
	type config struct {
		Name  string       `json:"name" default:"due"`
		Inner nestedConfig `json:"inner"`
	}

	src := "{\n\t\"n\\u0061me\": \"d\\u0075e\"," +
		"\n\t\"inner\": {\"host\": \"a\\\"b\", \"path\": \"\\\\tmp\"}\n}"

	var cfg config
	if err := NewValue(src).Scan(&cfg); err != nil {
		t.Fatalf("scan escaped json failed, err: %v", err)
	}
	if cfg.Name != "due" {
		t.Fatalf("invalid escaped key or value, actual name: %s", cfg.Name)
	}
	if cfg.Inner.Host != `a"b` || cfg.Inner.Path != `\tmp` {
		t.Fatalf("invalid escaped nested values, actual: %+v", cfg.Inner)
	}
}

func TestScanJSONStructRawDuplicateKeys(t *testing.T) {
	type config struct {
		Host string `json:"host" default:"127.0.0.1"`
		Port int    `json:"port" default:"9000"`
	}

	var cfg config
	if err := NewValue(`{"host":"first","host":"second","port":null}`).Scan(&cfg); err != nil {
		t.Fatalf("scan duplicate keys failed, err: %v", err)
	}
	if cfg.Host != "second" || cfg.Port != 9000 {
		t.Fatalf("the last duplicate key should win, actual: %+v", cfg)
	}
}

func TestScanJSONStructRawInvalidJSON(t *testing.T) {
	type nestedConfig struct {
		Port int `json:"port" default:"9000"`
	}
	type config struct {
		Inner nestedConfig `json:"inner"`
	}

	for _, src := range []string{`{"inner":}`, `{"inner":{"port":1}`, `{"inner":{"port":1}}x`} {
		var cfg config
		if err := NewValue(src).Scan(&cfg); err == nil {
			t.Fatalf("expect an error for %s, actual: nil", src)
		}
	}
}

func TestScanJSONStructRawNumbers(t *testing.T) {
	type nestedConfig struct {
		Int   int64   `json:"int" default:"0"`
		Uint  uint8   `json:"uint" default:"0"`
		Float float64 `json:"float" default:"0"`
	}
	type config struct {
		Inner nestedConfig `json:"inner"`
	}

	var cfg config
	src := `{"inner":{"int":9223372036854775807,"uint":255,"float":1e3}}`
	if err := NewValue(src).Scan(&cfg); err != nil {
		t.Fatalf("scan raw numbers failed, err: %v", err)
	}
	if cfg.Inner.Int != 9223372036854775807 || cfg.Inner.Uint != 255 || cfg.Inner.Float != 1000 {
		t.Fatalf("invalid raw numbers, actual: %+v", cfg.Inner)
	}

	var overflow config
	err := NewValue(`{"inner":{"uint":256}}`).Scan(&overflow)
	if err == nil || !strings.Contains(err.Error(), "inner.uint") {
		t.Fatalf("expect an overflow field path, actual: %v", err)
	}
}

func TestScanJSONTypedContainers(t *testing.T) {
	var texts map[string]string
	if err := NewValue(`{"host":"0.0.0.0","port":9000,"empty":null}`).Scan(&texts); err != nil {
		t.Fatalf("scan string map failed, err: %v", err)
	}
	if texts["host"] != "0.0.0.0" || texts["port"] != "9000" || texts["empty"] != "" {
		t.Fatalf("invalid string map, actual: %v", texts)
	}

	var ints map[string]int
	if err := NewValue(`{"read":1,"write":"2","missing":null}`).Scan(&ints); err != nil {
		t.Fatalf("scan int map failed, err: %v", err)
	}
	if ints["read"] != 1 || ints["write"] != 2 || ints["missing"] != 0 {
		t.Fatalf("invalid int map, actual: %v", ints)
	}

	var int64s map[string]int64
	if err := NewValue(`{"read":1,"write":2}`).Scan(&int64s); err != nil {
		t.Fatalf("scan int64 map failed, err: %v", err)
	}
	if int64s["read"] != 1 || int64s["write"] != 2 {
		t.Fatalf("invalid int64 map, actual: %v", int64s)
	}

	var bools map[string]bool
	if err := NewValue(`{"a":true,"b":"false"}`).Scan(&bools); err != nil {
		t.Fatalf("scan bool map failed, err: %v", err)
	}
	if !bools["a"] || bools["b"] {
		t.Fatalf("invalid bool map, actual: %v", bools)
	}

	var keyed map[int]string
	if err := NewValue(`{"1":"a","2":"b"}`).Scan(&keyed); err != nil {
		t.Fatalf("scan int keyed map failed, err: %v", err)
	}
	if keyed[1] != "a" || keyed[2] != "b" {
		t.Fatalf("invalid int keyed map, actual: %v", keyed)
	}

	var names []string
	if err := NewValue(`["a","b",3]`).Scan(&names); err != nil {
		t.Fatalf("scan string slice failed, err: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"a", "b", "3"}) {
		t.Fatalf("invalid string slice, actual: %v", names)
	}

	var ports []int
	if err := NewValue(`[1,"2"]`).Scan(&ports); err != nil {
		t.Fatalf("scan int slice failed, err: %v", err)
	}
	if !reflect.DeepEqual(ports, []int{1, 2}) {
		t.Fatalf("invalid int slice, actual: %v", ports)
	}

	var flags []bool
	if err := NewValue(`[true,"false"]`).Scan(&flags); err != nil {
		t.Fatalf("scan bool slice failed, err: %v", err)
	}
	if !reflect.DeepEqual(flags, []bool{true, false}) {
		t.Fatalf("invalid bool slice, actual: %v", flags)
	}

	var durations []time.Duration
	if err := NewValue(`["1s","2s"]`).Scan(&durations); err != nil {
		t.Fatalf("scan duration slice failed, err: %v", err)
	}
	if !reflect.DeepEqual(durations, []time.Duration{time.Second, 2 * time.Second}) {
		t.Fatalf("invalid duration slice, actual: %v", durations)
	}

	var bad []int
	err := NewValue(`[1,"x"]`).Scan(&bad)
	if err == nil || !strings.Contains(err.Error(), "[1]") {
		t.Fatalf("expect an element path, actual: %v", err)
	}

	var raw []byte
	if err = NewValue(`[1,2]`).Scan(&raw); err != nil {
		t.Fatalf("scan bytes failed, err: %v", err)
	}
	if string(raw) != "[1,2]" {
		t.Fatalf("json text should be kept for byte slices, actual: %s", raw)
	}
}

func TestScanJSONNestedStructConversion(t *testing.T) {
	type innerConfig struct {
		Port int  `json:"port"`
		Open bool `json:"open"`
	}
	type outerConfig struct {
		Inner innerConfig   `json:"inner"`
		List  []innerConfig `json:"list"`
	}

	var cfg outerConfig
	src := `{"inner":{"port":"8080","open":"true"},"list":[{"port":"9090","open":"false"}]}`
	if err := NewValue(src).Scan(&cfg); err != nil {
		t.Fatalf("scan nested conversion failed, err: %v", err)
	}
	if cfg.Inner.Port != 8080 || !cfg.Inner.Open {
		t.Fatalf("invalid nested conversion, actual: %+v", cfg.Inner)
	}
	if len(cfg.List) != 1 || cfg.List[0].Port != 9090 || cfg.List[0].Open {
		t.Fatalf("invalid nested list conversion, actual: %+v", cfg.List)
	}
}

func TestScanJSONStructWideDefaults(t *testing.T) {
	type nestedConfig struct {
		Host string `json:"host" default:"127.0.0.1"`
		Port int    `json:"port" default:"9000"`
	}

	fields := make([]reflect.StructField, 70)
	for i := range fields {
		fields[i] = reflect.StructField{
			Name: "F" + strconv.Itoa(i),
			Type: reflect.TypeOf(int(0)),
			Tag:  reflect.StructTag(`json:"f` + strconv.Itoa(i) + `" default:"1"`),
		}
	}
	fields[69] = reflect.StructField{
		Name: "Nested",
		Type: reflect.TypeOf(nestedConfig{}),
		Tag:  `json:"nested"`,
	}

	ptr := reflect.New(reflect.StructOf(fields))
	if err := NewValue(`{"f0":"2","nested":{"port":3}}`).Scan(ptr.Interface()); err != nil {
		t.Fatalf("scan wide defaults failed, err: %v", err)
	}

	got := ptr.Elem()
	if got.Field(0).Int() != 2 || got.Field(1).Int() != 1 {
		t.Fatalf("invalid wide fields, actual: %v, %v", got.Field(0), got.Field(1))
	}
	inner := got.Field(69).Interface().(nestedConfig)
	if inner.Host != "127.0.0.1" || inner.Port != 3 {
		t.Fatalf("invalid wide nested defaults, actual: %+v", inner)
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

	var nilBytes struct {
		Raw []byte `json:"raw" default:"due"`
	}
	if err := NewValue(map[string]any{"raw": []byte(nil)}).Scan(&nilBytes); err != nil {
		t.Fatalf("scan nil bytes failed, err: %v", err)
	}
	if string(nilBytes.Raw) != "due" {
		t.Fatalf("nil byte slice should fall back to the default, actual: %v", nilBytes.Raw)
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

func TestScanSliceFromSlice(t *testing.T) {
	var strs []string
	if err := NewValue([]string{"a", "b"}).Scan(&strs); err != nil {
		t.Fatalf("scan string slice failed, err: %v", err)
	}
	if !reflect.DeepEqual(strs, []string{"a", "b"}) {
		t.Fatalf("invalid string slice, actual: %v", strs)
	}

	var ints []int64
	if err := NewValue([]int{1, 2}).Scan(&ints); err != nil {
		t.Fatalf("scan int slice failed, err: %v", err)
	}
	if !reflect.DeepEqual(ints, []int64{1, 2}) {
		t.Fatalf("invalid int slice, actual: %v", ints)
	}

	var array [3]int
	if err := NewValue([]int{1, 2}).Scan(&array); err != nil {
		t.Fatalf("scan array failed, err: %v", err)
	}
	if array != [3]int{1, 2, 0} {
		t.Fatalf("invalid array, actual: %v", array)
	}

	src := []any{map[string]any{"count": 1}}
	var anys []any
	if err := NewValue(src).Scan(&anys); err != nil {
		t.Fatalf("scan any slice failed, err: %v", err)
	}
	anys[0].(map[string]any)["count"] = 2
	if src[0].(map[string]any)["count"] != 1 {
		t.Fatalf("scanned slice should not alias the source, actual: %v", src)
	}
}

func TestScanJSONMapIsIndependent(t *testing.T) {
	var (
		first  map[string]any
		second map[string]any
	)

	if err := NewValue(`{"nested":{"count":1}}`).Scan(&first); err != nil {
		t.Fatalf("scan first json map failed, err: %v", err)
	}
	if err := NewValue(`{"nested":{"count":1}}`).Scan(&second); err != nil {
		t.Fatalf("scan second json map failed, err: %v", err)
	}

	first["nested"].(map[string]any)["count"] = 2

	if second["nested"].(map[string]any)["count"] != float64(1) {
		t.Fatalf("json maps should not share decoded nodes, actual: %v", second)
	}
}

func TestScanStructWide(t *testing.T) {
	const fieldCount = 65

	fields := make([]reflect.StructField, fieldCount)
	for i := range fields {
		tag := fmt.Sprintf(`json:"f%02d"`, i)
		if i == 2 {
			tag += ` default:"7"`
		}
		fields[i] = reflect.StructField{
			Name: fmt.Sprintf("F%02d", i),
			Type: reflect.TypeOf(0),
			Tag:  reflect.StructTag(tag),
		}
	}

	typ := reflect.StructOf(fields)
	value := reflect.New(typ)

	if err := NewValue(map[string]any{"f00": 1, "F01": 2}).Scan(value.Interface()); err != nil {
		t.Fatalf("scan wide struct failed, err: %v", err)
	}

	if got := value.Elem().Field(0).Int(); got != 1 {
		t.Fatalf("invalid f00, expect: 1, actual: %d", got)
	}
	if got := value.Elem().Field(1).Int(); got != 2 {
		t.Fatalf("invalid f01, expect: 2, actual: %d", got)
	}
	if got := value.Elem().Field(2).Int(); got != 7 {
		t.Fatalf("invalid f02 default, expect: 7, actual: %d", got)
	}
	if got := value.Elem().Field(64).Int(); got != 0 {
		t.Fatalf("invalid f64, expect: 0, actual: %d", got)
	}

	value = reflect.New(typ)
	if err := NewValue().Scan(value.Interface()); err != nil {
		t.Fatalf("scan wide defaults failed, err: %v", err)
	}
	if got := value.Elem().Field(2).Int(); got != 7 {
		t.Fatalf("invalid wide default, expect: 7, actual: %d", got)
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

func TestScanErrorPaths(t *testing.T) {
	var cfg struct {
		Nested scanNestedConfig `json:"nested"`
		Tags   []int            `json:"tags"`
	}

	err := NewValue(map[string]any{
		"nested": map[string]any{"port": "bad"},
	}).Scan(&cfg)
	if err == nil || !strings.Contains(err.Error(), "nested.port") {
		t.Fatalf("expect a nested field path, actual: %v", err)
	}

	err = NewValue(map[string]any{"tags": "1,x"}).Scan(&cfg)
	if err == nil || !strings.Contains(err.Error(), "tags[1]") {
		t.Fatalf("expect a slice index path, actual: %v", err)
	}

	var list struct {
		Items []scanNestedConfig `json:"items"`
	}
	err = NewValue(map[string]any{
		"items": []any{map[string]any{"port": "bad"}},
	}).Scan(&list)
	if err == nil || !strings.Contains(err.Error(), "items[0].port") {
		t.Fatalf("expect an indexed field path, actual: %v", err)
	}

	var bytes []uint8
	err = NewValue([]int{300}).Scan(&bytes)
	if err == nil || !strings.Contains(err.Error(), "[0]") {
		t.Fatalf("expect an element overflow error, actual: %v", err)
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

	m = NewValue(`{"count":2}`).Map()
	if m["count"] != float64(2) {
		t.Fatalf("invalid json map result, actual: %v", m)
	}
}
