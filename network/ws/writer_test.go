package ws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/gorilla/websocket"
)

func TestWriteBuffer(t *testing.T) {
	tests := []struct {
		name string
		buf  buffer.Buffer
		want []byte
	}{
		{
			name: "single node",
			buf:  buffer.NewBytes([]byte("single"), true),
			want: []byte("single"),
		},
		{
			name: "segmented",
			buf:  &segmentedWriteBuffer{blocks: [][]byte{[]byte("packet "), []byte("header"), []byte(" + ciphertext")}},
			want: []byte("packet header + ciphertext"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, received := newWriterTestPeer(t)
			if err := writeBuffer(client, tt.buf); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-received:
				if !bytes.Equal(got, tt.want) {
					t.Fatalf("message = %q, want %q", got, tt.want)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("timeout waiting for WebSocket message")
			}
		})
	}
}

// segmentedWriteBuffer panics from Bytes so the test proves that the segmented path only visits
// the original blocks.
type segmentedWriteBuffer struct {
	blocks [][]byte
}

func (b *segmentedWriteBuffer) Len() int {
	var size int
	for _, block := range b.blocks {
		size += len(block)
	}
	return size
}

func (b *segmentedWriteBuffer) Nodes() int     { return len(b.blocks) }
func (b *segmentedWriteBuffer) Delay(int)      {}
func (b *segmentedWriteBuffer) Bytes() []byte  { panic("segmented buffer was flattened") }
func (b *segmentedWriteBuffer) Release()       {}
func (b *segmentedWriteBuffer) Slide(int) bool { return false }
func (b *segmentedWriteBuffer) VisitBytes(fn func([]byte) bool) bool {
	for _, block := range b.blocks {
		if !fn(block) {
			return false
		}
	}
	return true
}

func newWriterTestPeer(t *testing.T) (*websocket.Conn, <-chan []byte) {
	t.Helper()

	received := make(chan []byte, 1)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err == nil {
			received <- data
		}
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, received
}
