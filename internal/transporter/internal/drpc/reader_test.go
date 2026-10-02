package drpc

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

// newTCPPair returns the two ends of a loopback TCP connection. The first end is intended for the
// code under test and the second one for the peer that feeds bytes.
func newTCPPair(t *testing.T) (client, server *net.TCPConn) {
	t.Helper()

	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	accepted := make(chan *net.TCPConn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := ln.AcceptTCP()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()

	client, err = net.DialTCP("tcp", nil, ln.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	select {
	case server = <-accepted:
		t.Cleanup(func() { _ = server.Close() })
	case err = <-acceptErr:
		t.Fatalf("accept failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("accept timed out")
	}

	return client, server
}

// dataFrame builds a full data frame carrying the given route, sequence and payload.
func dataFrame(route uint8, seq uint64, payload []byte) []byte {
	frame := make([]byte, def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes+len(payload))
	binary.BigEndian.PutUint32(frame[:def.SizeBytes], uint32(len(frame)-def.SizeBytes))
	frame[def.SizeBytes] = def.DataBit
	frame[def.SizeBytes+def.HeaderBytes] = route
	binary.BigEndian.PutUint64(frame[def.SizeBytes+def.HeaderBytes+def.RouteBytes:], seq)
	copy(frame[def.SizeBytes+def.HeaderBytes+def.RouteBytes+def.SeqBytes:], payload)

	return frame
}

// heartbeatFrame builds a full heartbeat frame.
func heartbeatFrame() []byte {
	return []byte{0, 0, 0, def.HeaderBytes, def.HeartbeatBit}
}

// headerFrame builds a frame that only contains the size and header fields.
func headerFrame(size uint32, header uint8) []byte {
	frame := make([]byte, def.SizeBytes+def.HeaderBytes)
	binary.BigEndian.PutUint32(frame[:def.SizeBytes], size)
	frame[def.SizeBytes] = header

	return frame
}

// TestReaderRead verifies every branch of the frame reader with a controllable peer.
func TestReaderRead(t *testing.T) {
	tests := []struct {
		name    string
		frame   []byte
		wantHB  bool
		wantRt  uint8
		wantSeq uint64
		wantMsg string
		wantErr error
	}{
		{
			name:   "heartbeat",
			frame:  heartbeatFrame(),
			wantHB: true,
		},
		{
			name:    "heartbeat with an invalid size",
			frame:   headerFrame(2, def.HeartbeatBit),
			wantErr: errors.ErrInvalidMessage,
		},
		{
			name:    "frame smaller than the minimum size",
			frame:   headerFrame(def.MinFrameSize-def.SizeBytes-1, def.DataBit),
			wantErr: errors.ErrInvalidMessage,
		},
		{
			name:    "frame larger than the maximum size",
			frame:   headerFrame(def.MaxFrameSize, def.DataBit),
			wantErr: errors.ErrMessageTooLarge,
		},
		{
			name:    "data with a payload",
			frame:   dataFrame(route.Bind, 9, []byte("hello")),
			wantRt:  route.Bind,
			wantSeq: 9,
			wantMsg: "hello",
		},
		{
			name:    "data with an empty payload",
			frame:   dataFrame(route.GetState, 1, nil),
			wantRt:  route.GetState,
			wantSeq: 1,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			client, server := newTCPPair(t)
			r := newReader(client)

			if _, err := server.Write(c.frame); err != nil {
				t.Fatalf("write frame failed: %v", err)
			}

			hb, rt, seq, buf, err := r.read()
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("read error = %v, want %v", err, c.wantErr)
			}
			if c.wantErr != nil {
				return
			}

			if hb != c.wantHB || rt != c.wantRt || seq != c.wantSeq {
				t.Fatalf("read = (%v, %d, %d), want (%v, %d, %d)", hb, rt, seq, c.wantHB, c.wantRt, c.wantSeq)
			}
			if !hb {
				defer buf.Release()
				if got := string(buf.Bytes()); got != c.wantMsg {
					t.Fatalf("payload = %q, want %q", got, c.wantMsg)
				}
			}
		})
	}
}

// TestReaderReadClosed verifies that a closed peer surfaces as a read error.
func TestReaderReadClosed(t *testing.T) {
	client, server := newTCPPair(t)
	r := newReader(client)

	_ = server.Close()

	if _, _, _, _, err := r.read(); err == nil {
		t.Fatal("expect an error when the peer is closed")
	}
}

// TestReaderReadTruncated verifies that a truncated body surfaces as a read error.
func TestReaderReadTruncated(t *testing.T) {
	client, server := newTCPPair(t)
	r := newReader(client)

	// Announce a body of nine bytes but only send the five-byte header.
	frame := dataFrame(route.Bind, 1, nil)
	if _, err := server.Write(frame[:def.SizeBytes+def.HeaderBytes]); err != nil {
		t.Fatalf("write header failed: %v", err)
	}
	_ = server.Close()

	if _, _, _, _, err := r.read(); err == nil {
		t.Fatal("expect an error for a truncated body")
	}
}
