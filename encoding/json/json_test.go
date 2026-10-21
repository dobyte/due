package json_test

import (
	"testing"

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
