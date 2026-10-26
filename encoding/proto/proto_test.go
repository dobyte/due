package proto_test

import (
	"bytes"
	"testing"

	dueencoding "github.com/dobyte/due/v2/encoding"
	dueproto "github.com/dobyte/due/v2/encoding/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ dueencoding.Appender = dueproto.DefaultCodec

// TestAppend verifies that Append matches Marshal and reuses dst capacity.
func TestAppend(t *testing.T) {
	message := wrapperspb.String("hello")

	want, err := dueproto.Marshal(message)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	got, err := dueproto.DefaultCodec.Append(nil, message)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Append = %v, want %v", got, want)
	}

	dst := make([]byte, 2, len(want)+16)
	dst[0], dst[1] = 'x', 'y'

	got, err = dueproto.DefaultCodec.Append(dst, message)
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

// TestAppendInvalidValue verifies that Append rejects values that are not proto messages.
func TestAppendInvalidValue(t *testing.T) {
	if _, err := dueproto.DefaultCodec.Append(nil, struct{}{}); err == nil {
		t.Fatal("expect an error for a non-proto value")
	}
}

// BenchmarkMarshal measures encoding a 1KB message into a new slice.
func BenchmarkMarshal(b *testing.B) {
	message := wrapperspb.Bytes(make([]byte, 1024))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := dueproto.Marshal(message); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAppend measures encoding a 1KB message into a reusable buffer.
func BenchmarkAppend(b *testing.B) {
	message := wrapperspb.Bytes(make([]byte, 1024))
	dst := make([]byte, 0, 2048)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var err error
		dst, err = dueproto.DefaultCodec.Append(dst[:0], message)
		if err != nil {
			b.Fatal(err)
		}
	}
}
