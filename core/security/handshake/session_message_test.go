package handshake

import (
	"bytes"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/aead/chacha20poly1305"
	"github.com/dobyte/due/v2/errors"
)

// TestMessageBuffers checks destination reuse and compatibility with the original wire encoding.
func TestMessageBuffers(t *testing.T) {
	for _, suite := range []aead.Suite{aesgcm.Suite, chacha20poly1305.Suite} {
		t.Run(suite.Name(), func(t *testing.T) {
			client, server := establishSession(t, suite)
			for _, data := range [][]byte{nil, []byte("message")} {
				dst := make([]byte, 3, 3+len(data)+client.Overhead())
				copy(dst, "pre")
				sealed, err := client.SealMessage(dst, data, ClientToServer, -10, 42)
				if err != nil {
					t.Fatal(err)
				}
				if &sealed[0] != &dst[0] || string(sealed[:3]) != "pre" {
					t.Fatal("seal did not reuse destination and preserve its prefix")
				}
				original := bytes.Clone(sealed)
				plain, err := server.Open(sealed[3:], MakeAAD(ClientToServer, 1, -10, 42))
				if err != nil || !bytes.Equal(plain, data) || !bytes.Equal(original, sealed) {
					t.Fatalf("old Open compatibility: plaintext=%q err=%v", plain, err)
				}

				sealed, err = server.Seal(data, MakeAAD(ServerToClient, 1, 10, -42))
				if err != nil {
					t.Fatal(err)
				}
				original = bytes.Clone(sealed)
				out := make([]byte, 3, 3+len(data))
				copy(out, "pre")
				plain, err = client.OpenMessage(out, sealed, ServerToClient, 10, -42)
				if err != nil || string(plain[:3]) != "pre" || !bytes.Equal(plain[3:], data) {
					t.Fatalf("new Open compatibility: plaintext=%q err=%v", plain, err)
				}
				if &plain[0] != &out[0] || !bytes.Equal(original, sealed) {
					t.Fatal("open must reuse destination without modifying separate ciphertext")
				}
			}

			// An undersized destination must grow, preserving the caller's prefix.
			sealed, err := client.SealMessage([]byte("pre"), []byte("grow"), ClientToServer, 1, 2)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := server.OpenMessage([]byte("pre"), sealed[3:], ClientToServer, 1, 2)
			if err != nil || string(plain) != "pregrow" {
				t.Fatalf("growing destination: plaintext=%q err=%v", plain, err)
			}
		})
	}
}

// TestMessageAuthentication verifies metadata binding and that failed authentication cannot
// advance replay state. Neither an authentication failure nor a replay may return plaintext.
func TestMessageAuthentication(t *testing.T) {
	for _, suite := range []aead.Suite{aesgcm.Suite, chacha20poly1305.Suite} {
		t.Run(suite.Name(), func(t *testing.T) {
			sender, receiver := establishSession(t, suite)
			sealed, err := sender.SealMessage(nil, []byte("secret"), ClientToServer, 10, 20)
			if err != nil {
				t.Fatal(err)
			}
			for _, metadata := range []struct {
				direction  Direction
				route, seq int32
			}{{ServerToClient, 10, 20}, {ClientToServer, 11, 20}, {ClientToServer, 10, 21}} {
				plain, err := receiver.OpenMessage(make([]byte, 0, 32), sealed,
					metadata.direction, metadata.route, metadata.seq)
				if !errors.Is(err, errors.ErrInvalidMessage) || plain != nil {
					t.Fatalf("modified metadata returned plaintext=%q err=%v", plain, err)
				}
			}
			wrongEpoch, forgedCounter, wrongTag := bytes.Clone(sealed), bytes.Clone(sealed), bytes.Clone(sealed)
			wrongEpoch[0]++
			binary.BigEndian.PutUint64(forgedCounter[1:9], 1<<63)
			wrongTag[len(wrongTag)-1] ^= 1
			for _, invalid := range [][]byte{nil, sealed[:8], wrongEpoch, forgedCounter, wrongTag} {
				if plain, err := receiver.OpenMessage(nil, invalid, ClientToServer, 10, 20); err == nil || plain != nil {
					t.Fatalf("invalid envelope returned plaintext=%q err=%v", plain, err)
				}
			}
			plain, err := receiver.OpenMessage(nil, sealed, ClientToServer, 10, 20)
			if err != nil || string(plain) != "secret" {
				t.Fatalf("valid message after invalid attempts: plaintext=%q err=%v", plain, err)
			}
			if plain, err = receiver.OpenMessage(nil, sealed, ClientToServer, 10, 20); !errors.Is(err, ErrReplayMessage) || plain != nil {
				t.Fatalf("replay returned plaintext=%q err=%v", plain, err)
			}
			sealed, err = sender.SealMessage(nil, []byte("in-place"), ClientToServer, 10, 21)
			if err != nil {
				t.Fatal(err)
			}
			plain, err = receiver.OpenMessage(sealed[9:9], sealed, ClientToServer, 10, 21)
			if err != nil || string(plain) != "in-place" {
				t.Fatalf("in-place open: plaintext=%q err=%v", plain, err)
			}
		})
	}
}

// TestMessageConcurrent protects per-operation scratch ownership, nonce uniqueness and atomic
// replay rejection. The batch fits the 64-message replay window even when goroutines reorder it.
func TestMessageConcurrent(t *testing.T) {
	for _, suite := range []aead.Suite{aesgcm.Suite, chacha20poly1305.Suite} {
		t.Run(suite.Name(), func(t *testing.T) {
			sender, receiver := establishSession(t, suite)
			sealed := make([][]byte, 32)
			var wg sync.WaitGroup
			for i := range sealed {
				wg.Go(func() {
					var err error
					sealed[i], err = sender.SealMessage(nil, []byte{byte(i)}, ClientToServer, 10, int32(i))
					if err != nil {
						t.Error(err)
					}
				})
			}
			wg.Wait()
			if t.Failed() {
				return
			}
			seen := make(map[uint64]bool)
			for _, frame := range sealed {
				counter := binary.BigEndian.Uint64(frame[1:9])
				if seen[counter] {
					t.Fatal("nonce counter reused")
				}
				seen[counter] = true
			}
			for i := range sealed {
				wg.Go(func() {
					plain, err := receiver.OpenMessage(nil, sealed[i], ClientToServer, 10, int32(i))
					if err != nil || !bytes.Equal(plain, []byte{byte(i)}) {
						t.Errorf("concurrent open: plaintext=%v err=%v", plain, err)
					}
				})
			}
			wg.Wait()
			frame, err := sender.SealMessage(nil, []byte("once"), ClientToServer, 10, 100)
			if err != nil {
				t.Fatal(err)
			}
			var successes atomic.Int32
			for range 16 {
				wg.Go(func() {
					_, err := receiver.OpenMessage(nil, frame, ClientToServer, 10, 100)
					if err == nil {
						successes.Add(1)
					} else if !errors.Is(err, ErrReplayMessage) {
						t.Error(err)
					}
				})
			}
			wg.Wait()
			if successes.Load() != 1 {
				t.Fatalf("duplicate frame accepted %d times", successes.Load())
			}
		})
	}
}
