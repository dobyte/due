package protocol_test

import (
	"bytes"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
)

func TestEncodeDeliverReq(t *testing.T) {
	buffer := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewNocopyBuffer([]byte("hello world")))

	t.Log(buffer.Bytes())
}

func TestEncodeDeliverReqBuffer(t *testing.T) {
	payload := []byte("hello world")

	want := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewBytes(append([]byte(nil), payload...)))
	defer want.Release()
	wantBytes := want.Bytes()

	message := buffer.MallocBytesWithHeadroom(protocol.DeliverReqHeadroom(), len(payload))
	if message == nil {
		t.Fatal("MallocBytesWithHeadroom returned nil")
	}
	defer message.Release()

	copy(message.Bytes(), payload)

	got := protocol.EncodeDeliverReqBuffer(1, 2, 3, message)
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
	cid, uid, messageData, err := protocol.DecodeDeliverReq(data)
	if err != nil {
		t.Fatalf("DecodeDeliverReq failed: %v", err)
	}
	if cid != 2 || uid != 3 {
		t.Errorf("cid/uid = %d/%d, want 2/3", cid, uid)
	}
	if !bytes.Equal(messageData.Bytes(), payload) {
		t.Errorf("payload = %q, want %q", messageData.Bytes(), payload)
	}
}

func TestEncodeDeliverReqBufferFallback(t *testing.T) {
	payload := []byte("hello world")

	want := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewBytes(append([]byte(nil), payload...)))
	defer want.Release()
	wantBytes := want.Bytes()

	message := buffer.MallocBytes(len(payload))
	if message == nil {
		t.Fatal("MallocBytes returned nil")
	}
	copy(message.Bytes(), payload)

	got := protocol.EncodeDeliverReqBuffer(1, 2, 3, message)
	defer got.Release()

	if _, ok := got.(*buffer.NocopyBuffer); !ok {
		t.Errorf("expect a segmented buffer without headroom, got %T", got)
	}
	if !bytes.Equal(got.Bytes(), wantBytes) {
		t.Errorf("frame = %v, want %v", got.Bytes(), wantBytes)
	}
}

func TestDecodeDeliverReq(t *testing.T) {
	req := protocol.EncodeDeliverReq(1, 2, 3, buffer.NewNocopyBuffer([]byte("hello world")))
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	cid, uid, data, err := protocol.DecodeDeliverReq(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("cid: %v", cid)
	t.Logf("uid: %v", uid)
	t.Logf("message: %v", string(data.Bytes()))
}

func TestEncodeDeliverRes(t *testing.T) {
	buffer := protocol.EncodeDeliverRes(1, codes.OK)

	t.Log(buffer.Bytes())
}

func TestDecodeDeliverRes(t *testing.T) {
	req := protocol.EncodeDeliverRes(1, codes.OK)
	buf := buffer.NewBytes(req.Bytes()[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:])
	defer req.Release()

	code, err := protocol.DecodeDeliverRes(buf)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("code: %v", code)
}
