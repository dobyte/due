package json_test

import (
	"testing"

	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/encoding/json"
)

type payload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TestRoundTrip verifies that the backend selected by the build tags encodes and decodes the
// same payload.
func TestRoundTrip(t *testing.T) {
	data, err := json.Marshal(payload{Name: "due", Count: 2})
	if err != nil {
		t.Fatal(err)
	}

	var got payload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got != (payload{Name: "due", Count: 2}) {
		t.Errorf("unexpected payload: %+v", got)
	}
}

// TestCaseInsensitiveNames verifies that field names match regardless of case on both backends.
func TestCaseInsensitiveNames(t *testing.T) {
	var got payload
	if err := json.Unmarshal([]byte(`{"NAME":"due","COUNT":2}`), &got); err != nil {
		t.Fatal(err)
	}

	if got != (payload{Name: "due", Count: 2}) {
		t.Errorf("unexpected payload: %+v", got)
	}
}

// TestDuplicateNames verifies that duplicate object members keep the last value on both backends.
func TestDuplicateNames(t *testing.T) {
	var got payload
	if err := json.Unmarshal([]byte(`{"name":"a","name":"b"}`), &got); err != nil {
		t.Fatal(err)
	}

	if got.Name != "b" {
		t.Errorf("expect the last member to win, actual: %s", got.Name)
	}
}

// TestAppend verifies that Append produces the same bytes as Marshal and appends to dst in place
// when dst has enough capacity.
func TestAppend(t *testing.T) {
	appender, ok := any(json.DefaultCodec).(encoding.Appender)
	if !ok {
		t.Fatal("json codec does not implement encoding.Appender")
	}

	v := payload{Name: "due", Count: 2}

	want, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	got, err := appender.Append(nil, v)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Append = %q, want %q", got, want)
	}

	dst := make([]byte, 0, 4096)

	got, err = appender.Append(dst, v)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Append into capacity = %q, want %q", got, want)
	}
	if &got[0] != &dst[:1][0] {
		t.Errorf("Append reallocated despite sufficient capacity: dst=%p got=%p cap=%d", &dst[:1][0], &got[0], cap(dst))
	}

	prefix := []byte("prefix:")
	dst = append(make([]byte, 0, len(prefix)+len(want)+16), prefix...)

	got, err = appender.Append(dst, v)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if string(got) != string(prefix)+string(want) {
		t.Errorf("Append with prefix = %q, want %q", got, string(prefix)+string(want))
	}
}

// TestInvalidUTF8 verifies that invalid UTF-8 neither fails the encoding nor the decoding on both
// backends. The backends keep the raw bytes differently, so only the absence of an error is
// asserted here.
func TestInvalidUTF8(t *testing.T) {
	data, err := json.Marshal(map[string]string{"name": "\xff"})
	if err != nil {
		t.Fatalf("marshal failed, err: %v", err)
	}

	var got map[string]string
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed, err: %v", err)
	}

	if len(got["name"]) == 0 {
		t.Error("expect the invalid UTF-8 value to be kept")
	}
}
