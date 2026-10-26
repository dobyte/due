package protocol_test

import (
	"bytes"
	"testing"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/packet"
)

func TestEncodePushReqBuffer(t *testing.T) {
	payload := []byte("hello world")

	want := protocol.EncodePushReq(1, cluster.User, 3, true, buffer.NewBytes(append([]byte(nil), payload...)))
	defer want.Release()
	wantBytes := want.Bytes()

	message := buffer.MallocBytesWithHeadroom(protocol.PushReqHeadroom(), len(payload))
	if message == nil {
		t.Fatal("MallocBytesWithHeadroom returned nil")
	}
	defer message.Release()

	copy(message.Bytes(), payload)

	got := protocol.EncodePushReqBuffer(1, cluster.User, 3, true, message)
	if got != message {
		t.Fatal("expect the pooled buffer to be reused in place")
	}
	if nodes := got.Nodes(); nodes != 1 {
		t.Fatalf("Nodes() = %d, want 1", nodes)
	}
	if !bytes.Equal(got.Bytes(), wantBytes) {
		t.Errorf("frame = %v, want %v", got.Bytes(), wantBytes)
	}

	data := buffer.NewBytes(got.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	kind, target, disconnect, messageData, err := protocol.DecodePushReq(data)
	if err != nil {
		t.Fatalf("DecodePushReq failed: %v", err)
	}
	if kind != cluster.User || target != 3 || !disconnect {
		t.Errorf("kind/target/disconnect = %v/%d/%v, want user/3/true", kind, target, disconnect)
	}
	if !bytes.Equal(messageData.Bytes(), payload) {
		t.Errorf("payload = %q, want %q", messageData.Bytes(), payload)
	}
}

func TestEncodePushReqBufferFallback(t *testing.T) {
	payload := []byte("hello world")

	want := protocol.EncodePushReq(1, cluster.User, 3, true, buffer.NewBytes(append([]byte(nil), payload...)))
	defer want.Release()
	wantBytes := want.Bytes()

	message := buffer.MallocBytes(len(payload))
	if message == nil {
		t.Fatal("MallocBytes returned nil")
	}
	copy(message.Bytes(), payload)

	got := protocol.EncodePushReqBuffer(1, cluster.User, 3, true, message)
	defer got.Release()

	if _, ok := got.(*buffer.NocopyBuffer); !ok {
		t.Errorf("expect a segmented buffer without headroom, got %T", got)
	}
	if !bytes.Equal(got.Bytes(), wantBytes) {
		t.Errorf("frame = %v, want %v", got.Bytes(), wantBytes)
	}
}

// BenchmarkEncodePushReq measures the segmented push encoding.
func BenchmarkEncodePushReq(b *testing.B) {
	payload := make([]byte, 1024)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := protocol.EncodePushReq(1, cluster.User, 3, false, buffer.MallocBytes(len(payload)))
		req.Release()
	}
}

// BenchmarkEncodePushReqBuffer measures the in-place push encoding.
func BenchmarkEncodePushReqBuffer(b *testing.B) {
	payload := make([]byte, 1024)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		message := buffer.MallocBytesWithHeadroom(protocol.PushReqHeadroom(), len(payload))
		protocol.EncodePushReqBuffer(1, cluster.User, 3, false, message).Release()
	}
}

func TestEncodePushReq(t *testing.T) {
	message, err := packet.PackMessage(1, 2, []byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}

	buf := protocol.EncodePushReq(1, cluster.User, 3, true, buffer.NewNocopyBuffer(message))

	t.Log(buf.Bytes())
}

func TestDecodePushReq(t *testing.T) {
	message, err := packet.PackMessage(1, 2, []byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}

	req := protocol.EncodePushReq(1, cluster.User, 3, true, buffer.NewNocopyBuffer(message))
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	kind, target, disconnect, data, err := protocol.DecodePushReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("kind: %v", kind)
	t.Logf("target: %v", target)
	t.Logf("disconnect: %v", disconnect)
	t.Logf("message: %v", string(data.Bytes()))
}

func TestEncodePushRes(t *testing.T) {
	buffer := protocol.EncodePushRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodePushRes(t *testing.T) {
	req := protocol.EncodePushRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodePushRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
