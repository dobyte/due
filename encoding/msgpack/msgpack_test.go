package msgpack_test

import (
	"bytes"
	"testing"

	dueencoding "github.com/dobyte/due/v2/encoding"
	duemsgpack "github.com/dobyte/due/v2/encoding/msgpack"
)

var _ dueencoding.Appender = duemsgpack.DefaultCodec

// TestAppend verifies that Append matches Marshal and appends to dst in place when dst has enough
// capacity.
func TestAppend(t *testing.T) {
	value := map[string]string{"message": "hello"}

	want, err := duemsgpack.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	got, err := duemsgpack.DefaultCodec.Append(nil, value)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Append = %v, want %v", got, want)
	}

	dst := make([]byte, 2, len(want)+16)
	dst[0], dst[1] = 'x', 'y'

	got, err = duemsgpack.DefaultCodec.Append(dst, value)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if !bytes.Equal(got[:2], []byte("xy")) || !bytes.Equal(got[2:], want) {
		t.Errorf("Append with prefix = %v, want %v", got, append([]byte("xy"), want...))
	}
	if &got[0] != &dst[0] {
		t.Error("Append reallocated despite sufficient capacity")
	}
}

// TestAppendError verifies that Append reports encoding failures.
func TestAppendError(t *testing.T) {
	if _, err := duemsgpack.DefaultCodec.Append(nil, make(chan int)); err == nil {
		t.Fatal("expect an error for an unsupported value")
	}
}

// BenchmarkMarshal measures encoding a 1KB value into a new slice.
func BenchmarkMarshal(b *testing.B) {
	value := map[string]string{"message": string(make([]byte, 1024))}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := duemsgpack.Marshal(value); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAppend measures encoding a 1KB value into a reusable buffer.
func BenchmarkAppend(b *testing.B) {
	value := map[string]string{"message": string(make([]byte, 1024))}
	dst := make([]byte, 0, 2048)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var err error
		dst, err = duemsgpack.DefaultCodec.Append(dst[:0], value)
		if err != nil {
			b.Fatal(err)
		}
	}
}
