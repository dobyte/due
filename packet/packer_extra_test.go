package packet_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// stubPacker records the calls made through the package-level helpers.
type stubPacker struct {
	readCalls      int
	packCalls      int
	extractCalls   int
	unpackCalls    int
	heartbeatCalls int
}

// Read implements Packer.
func (p *stubPacker) Read(io.Reader) (bool, int64, buffer.Buffer, error) {
	p.readCalls++
	return true, 1, nil, errors.ErrInvalidMessage
}

// PackMessage implements Packer.
func (p *stubPacker) PackMessage(*packet.Message) (buffer.Buffer, error) {
	p.packCalls++
	return nil, errors.ErrInvalidMessage
}

// ExtractRouteSeq implements Packer.
func (p *stubPacker) ExtractRouteSeq(buffer.Buffer) (int32, int32, error) {
	p.extractCalls++
	return 1, 2, nil
}

// UnpackMessage implements Packer.
func (p *stubPacker) UnpackMessage(buffer.Buffer) (int32, int32, buffer.Buffer, error) {
	p.unpackCalls++
	return 1, 2, nil, nil
}

// PackHeartbeat implements Packer.
func (p *stubPacker) PackHeartbeat(...bool) buffer.Buffer {
	p.heartbeatCalls++
	return nil
}

// frame builds a raw packet header with the given size, header flag and body bytes.
func frame(size uint32, flag byte, body ...byte) []byte {
	buf := make([]byte, 0, 4+1+len(body))
	buf = binary.BigEndian.AppendUint32(buf, size)
	buf = append(buf, flag)
	buf = append(buf, body...)
	return buf
}

// newBuffer wraps data into a buffer for unpacking tests.
func newBuffer(data []byte) *buffer.Writer {
	w := buffer.NewWriter(make([]byte, len(data)), true)
	w.WriteBytes(data...)
	return w
}

// readerFactory builds a fresh reader over data, covering both the buffered and the pooled paths.
type readerFactory struct {
	name string
	make func(data []byte) io.Reader
}

var readerFactories = []readerFactory{
	{"pooled", func(data []byte) io.Reader { return bytes.NewReader(data) }},
	{"buffered", func(data []byte) io.Reader { return bufio.NewReader(bytes.NewReader(data)) }},
}

// TestExtraGlobalPacker verifies the package-level helpers delegate to the global packer and that
// the global packer can be replaced.
func TestExtraGlobalPacker(t *testing.T) {
	original := packet.GetPacker()
	t.Cleanup(func() { packet.SetPacker(original) })

	if packet.GetPacker() == nil {
		t.Fatalf("GetPacker() = nil, want non-nil")
	}

	packed, err := packet.PackMessage(&packet.Message{Seq: 1, Route: 1, Buffer: []byte("hello")})
	if err != nil {
		t.Fatalf("PackMessage() error = %v", err)
	}
	defer packed.Release()

	route, seq, err := packet.ExtractRouteSeq(packed)
	if err != nil {
		t.Fatalf("ExtractRouteSeq() error = %v", err)
	}
	if route != 1 || seq != 1 {
		t.Errorf("ExtractRouteSeq() = (%d, %d), want (1, 1)", route, seq)
	}

	isHeartbeat, heartbeatTime, body, err := packet.Read(bytes.NewReader(packed.Bytes()))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if isHeartbeat {
		t.Errorf("Read() isHeartbeat = true, want false")
	}
	if heartbeatTime != 0 {
		t.Errorf("Read() heartbeatTime = %d, want 0", heartbeatTime)
	}
	defer body.Release()

	route, seq, unpacked, err := packet.UnpackMessage(body)
	if err != nil {
		t.Fatalf("UnpackMessage() error = %v", err)
	}
	if route != 1 || seq != 1 {
		t.Errorf("UnpackMessage() = (%d, %d), want (1, 1)", route, seq)
	}
	if got := string(unpacked.Bytes()); got != "hello" {
		t.Errorf("UnpackMessage() body = %q, want %q", got, "hello")
	}

	if hb := packet.PackHeartbeat(); hb == nil {
		t.Errorf("PackHeartbeat() = nil, want non-nil")
	}

	stub := &stubPacker{}
	packet.SetPacker(stub)
	if packet.GetPacker() != stub {
		t.Errorf("GetPacker() did not return the injected packer")
	}

	packet.Read(nil)
	packet.PackMessage(nil)
	packet.ExtractRouteSeq(nil)
	packet.UnpackMessage(nil)
	packet.PackHeartbeat()

	if stub.readCalls != 1 || stub.packCalls != 1 || stub.extractCalls != 1 || stub.unpackCalls != 1 || stub.heartbeatCalls != 1 {
		t.Errorf("wrapper calls = (%d, %d, %d, %d, %d), want all 1",
			stub.readCalls, stub.packCalls, stub.extractCalls, stub.unpackCalls, stub.heartbeatCalls)
	}
}

// TestExtraPackerRoundTrip verifies packing and unpacking across every supported route/seq size
// and a little-endian byte order.
func TestExtraPackerRoundTrip(t *testing.T) {
	for _, routeBytes := range []int{1, 2, 4} {
		for _, seqBytes := range []int{0, 1, 2, 4} {
			t.Run(fmt.Sprintf("route%d/seq%d", routeBytes, seqBytes), func(t *testing.T) {
				p := packet.NewPacker(
					packet.WithByteOrder(binary.LittleEndian),
					packet.WithRouteBytes(routeBytes),
					packet.WithSeqBytes(seqBytes),
					packet.WithBufferBytes(1024),
					packet.WithHeartbeatTime(false),
				)

				packed, err := p.PackMessage(&packet.Message{Seq: 100, Route: 100, Buffer: []byte("payload")})
				if err != nil {
					t.Fatalf("PackMessage() error = %v", err)
				}
				defer packed.Release()

				route, seq, err := p.ExtractRouteSeq(packed)
				if err != nil {
					t.Fatalf("ExtractRouteSeq() error = %v", err)
				}
				if route != 100 {
					t.Errorf("ExtractRouteSeq() route = %d, want 100", route)
				}
				if seqBytes > 0 && seq != 100 {
					t.Errorf("ExtractRouteSeq() seq = %d, want 100", seq)
				}

				isHeartbeat, _, body, err := p.Read(bytes.NewReader(packed.Bytes()))
				if err != nil {
					t.Fatalf("Read() error = %v", err)
				}
				if isHeartbeat {
					t.Errorf("Read() isHeartbeat = true, want false")
				}
				defer body.Release()

				route, seq, unpacked, err := p.UnpackMessage(body)
				if err != nil {
					t.Fatalf("UnpackMessage() error = %v", err)
				}
				if route != 100 {
					t.Errorf("UnpackMessage() route = %d, want 100", route)
				}
				if seqBytes > 0 && seq != 100 {
					t.Errorf("UnpackMessage() seq = %d, want 100", seq)
				}
				if got := string(unpacked.Bytes()); got != "payload" {
					t.Errorf("UnpackMessage() body = %q, want %q", got, "payload")
				}
			})
		}
	}
}

// TestExtraReadHeartbeat verifies a heartbeat without a timestamp is read as an empty body.
func TestExtraReadHeartbeat(t *testing.T) {
	p := packet.NewPacker()
	hb := p.PackHeartbeat()

	for _, rf := range readerFactories {
		t.Run(rf.name, func(t *testing.T) {
			isHeartbeat, heartbeatTime, body, err := p.Read(rf.make(hb.Bytes()))
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if !isHeartbeat {
				t.Errorf("Read() isHeartbeat = false, want true")
			}
			if heartbeatTime != 0 {
				t.Errorf("Read() heartbeatTime = %d, want 0", heartbeatTime)
			}
			if body != nil {
				body.Release()
				t.Errorf("Read() body = %v, want nil", body)
			}
		})
	}
}

// TestExtraReadHeartbeatWithTime verifies a server-side heartbeat carries a non-zero timestamp.
func TestExtraReadHeartbeatWithTime(t *testing.T) {
	p := packet.NewPacker(packet.WithHeartbeatTime(true))
	hb := p.PackHeartbeat(true)
	defer hb.Release()

	for _, rf := range readerFactories {
		t.Run(rf.name, func(t *testing.T) {
			isHeartbeat, heartbeatTime, body, err := p.Read(rf.make(hb.Bytes()))
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if !isHeartbeat {
				t.Errorf("Read() isHeartbeat = false, want true")
			}
			if heartbeatTime == 0 {
				t.Errorf("Read() heartbeatTime = 0, want non-zero")
			}
			if body != nil {
				body.Release()
				t.Errorf("Read() body = %v, want nil", body)
			}
		})
	}
}

// TestExtraReadBufferedData verifies a data message read through the buffered fast path.
func TestExtraReadBufferedData(t *testing.T) {
	p := packet.NewPacker()

	packed, err := p.PackMessage(&packet.Message{Route: 1, Seq: 1, Buffer: []byte("body")})
	if err != nil {
		t.Fatalf("PackMessage() error = %v", err)
	}
	defer packed.Release()

	isHeartbeat, _, body, err := p.Read(bufio.NewReader(bytes.NewReader(packed.Bytes())))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if isHeartbeat {
		t.Errorf("Read() isHeartbeat = true, want false")
	}
	defer body.Release()

	_, _, unpacked, err := p.UnpackMessage(body)
	if err != nil {
		t.Fatalf("UnpackMessage() error = %v", err)
	}
	if got := string(unpacked.Bytes()); got != "body" {
		t.Errorf("Read() body = %q, want %q", got, "body")
	}
}

// TestExtraReadErrors verifies the error branches of Read on both reader paths.
func TestExtraReadErrors(t *testing.T) {
	p := packet.NewPacker(packet.WithBufferBytes(1024))

	cases := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{"short header", []byte{0, 0, 0}, nil},
		{"heartbeat time wrong size", frame(5, 0xC0), errors.ErrInvalidMessage},
		{"heartbeat time short body", frame(9, 0xC0), nil},
		{"zero size", frame(0, 0x00), errors.ErrInvalidMessage},
		{"data too large", frame(2000, 0x00), errors.ErrMessageTooLarge},
		{"short data body", frame(10, 0x00), nil},
	}

	for _, rf := range readerFactories {
		for _, c := range cases {
			t.Run(rf.name+"/"+c.name, func(t *testing.T) {
				_, _, body, err := p.Read(rf.make(c.data))
				if body != nil {
					body.Release()
					t.Errorf("Read() body = %v, want nil", body)
				}
				if err == nil {
					t.Fatalf("Read() error = nil, want error")
				}
				if c.wantErr != nil && !errors.Is(err, c.wantErr) {
					t.Errorf("Read() error = %v, want %v", err, c.wantErr)
				}
			})
		}
	}
}

// TestExtraPackMessageErrors verifies the validation branches of PackMessage.
func TestExtraPackMessageErrors(t *testing.T) {
	cases := []struct {
		name string
		opts []packet.Option
		msg  *packet.Message
		want error
	}{
		{"route overflow", []packet.Option{packet.WithRouteBytes(1)}, &packet.Message{Route: 128}, errors.ErrRouteOverflow},
		{"seq overflow", []packet.Option{packet.WithSeqBytes(1)}, &packet.Message{Route: 1, Seq: 128}, errors.ErrSeqOverflow},
		{"message too large", []packet.Option{packet.WithBufferBytes(4)}, &packet.Message{Route: 1, Buffer: []byte("12345")}, errors.ErrMessageTooLarge},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := packet.NewPacker(c.opts...)

			buf, err := p.PackMessage(c.msg)
			if buf != nil {
				buf.Release()
				t.Errorf("PackMessage() buffer = %v, want nil", buf)
			}
			if !errors.Is(err, c.want) {
				t.Errorf("PackMessage() error = %v, want %v", err, c.want)
			}
		})
	}
}

// TestExtraUnpackErrors verifies the validation branches of ExtractRouteSeq and UnpackMessage.
func TestExtraUnpackErrors(t *testing.T) {
	p := packet.NewPacker()

	shortData := []byte{0, 0, 0, 0, 0}

	sizeMismatch := make([]byte, 9)
	binary.BigEndian.PutUint32(sizeMismatch, 100)

	heartbeat := make([]byte, 9)
	binary.BigEndian.PutUint32(heartbeat, 5)
	heartbeat[4] = 0x80

	cases := []struct {
		name string
		data []byte
	}{
		{"too short", shortData},
		{"size mismatch", sizeMismatch},
		{"heartbeat", heartbeat},
	}

	for _, c := range cases {
		t.Run("ExtractRouteSeq/"+c.name, func(t *testing.T) {
			route, seq, err := p.ExtractRouteSeq(newBuffer(append([]byte(nil), c.data...)))
			if route != 0 || seq != 0 {
				t.Errorf("ExtractRouteSeq() = (%d, %d), want (0, 0)", route, seq)
			}
			if !errors.Is(err, errors.ErrInvalidMessage) {
				t.Errorf("ExtractRouteSeq() error = %v, want %v", err, errors.ErrInvalidMessage)
			}
		})

		t.Run("UnpackMessage/"+c.name, func(t *testing.T) {
			route, seq, buf, err := p.UnpackMessage(newBuffer(append([]byte(nil), c.data...)))
			if buf != nil {
				buf.Release()
				t.Errorf("UnpackMessage() buffer = %v, want nil", buf)
			}
			if route != 0 || seq != 0 {
				t.Errorf("UnpackMessage() = (%d, %d), want (0, 0)", route, seq)
			}
			if !errors.Is(err, errors.ErrInvalidMessage) {
				t.Errorf("UnpackMessage() error = %v, want %v", err, errors.ErrInvalidMessage)
			}
		})
	}
}

// TestExtraPackHeartbeat verifies every branch of PackHeartbeat.
func TestExtraPackHeartbeat(t *testing.T) {
	cases := []struct {
		name              string
		heartbeatTime     bool
		server            bool
		wantHeartbeatTime bool
		releasable        bool
	}{
		{"disabled client", false, false, false, false},
		{"disabled server", false, true, false, false},
		{"enabled client", true, false, false, false},
		{"enabled server", true, true, true, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := packet.NewPacker(packet.WithHeartbeatTime(c.heartbeatTime))

			hb := p.PackHeartbeat(c.server)
			if hb == nil {
				t.Fatalf("PackHeartbeat() = nil, want non-nil")
			}
			if c.releasable {
				defer hb.Release()
			}

			isHeartbeat, heartbeatTime, body, err := p.Read(bytes.NewReader(hb.Bytes()))
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if !isHeartbeat {
				t.Errorf("Read() isHeartbeat = false, want true")
			}
			if got := heartbeatTime != 0; got != c.wantHeartbeatTime {
				t.Errorf("Read() heartbeatTime = %d, want non-zero = %v", heartbeatTime, c.wantHeartbeatTime)
			}
			if body != nil {
				body.Release()
			}
		})
	}
}
