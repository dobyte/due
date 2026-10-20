package handshake

import (
	"strconv"
	"testing"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/aead/chacha20poly1305"
)

// BenchmarkSession measures authenticated traffic with a fresh counter on every message.
func BenchmarkSession(b *testing.B) {
	for _, suite := range []aead.Suite{aesgcm.Suite, chacha20poly1305.Suite} {
		for _, size := range []int{0, 64, 256, 1024, 4096} {
			b.Run(suite.Name()+"/"+strconv.Itoa(size), func(b *testing.B) {
				b.Run("MessageSeal", func(b *testing.B) {
					sender, _ := establishSession(b, suite)
					data := make([]byte, size)
					dst := make([]byte, 0, size+sender.Overhead())
					b.SetBytes(int64(size))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := sender.SealMessage(dst, data, ClientToServer, 10, 1); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("MessageRoundTrip", func(b *testing.B) {
					sender, receiver := establishSession(b, suite)
					data := make([]byte, size)
					sealedBuffer := make([]byte, 0, size+sender.Overhead())
					plainBuffer := make([]byte, 0, size)
					b.SetBytes(int64(size))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						sealed, err := sender.SealMessage(sealedBuffer, data, ClientToServer, 10, 1)
						if err != nil {
							b.Fatal(err)
						}
						if _, err = receiver.OpenMessage(plainBuffer, sealed, ClientToServer, 10, 1); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("ParallelMessageSeal", func(b *testing.B) {
					sender, _ := establishSession(b, suite)
					data := make([]byte, size)
					b.SetBytes(int64(size))
					b.ReportAllocs()
					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						dst := make([]byte, 0, size+sender.Overhead())
						for pb.Next() {
							if _, err := sender.SealMessage(dst, data, ClientToServer, 10, 1); err != nil {
								b.Error(err)
								return
							}
						}
					})
				})
				b.Run("Seal", func(b *testing.B) {
					sender, _ := establishSession(b, suite)
					data := make([]byte, size)
					b.SetBytes(int64(size))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := sender.Seal(data, MakeAAD(ClientToServer, 1, 10, 1)); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("RoundTrip", func(b *testing.B) {
					sender, receiver := establishSession(b, suite)
					data := make([]byte, size)
					b.SetBytes(int64(size))
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						aad := MakeAAD(ClientToServer, 1, 10, 1)
						sealed, err := sender.Seal(data, aad)
						if err != nil {
							b.Fatal(err)
						}
						if _, err = receiver.Open(sealed, aad); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("ParallelSeal", func(b *testing.B) {
					sender, _ := establishSession(b, suite)
					data := make([]byte, size)
					b.SetBytes(int64(size))
					b.ReportAllocs()
					b.ResetTimer()
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							if _, err := sender.Seal(data, MakeAAD(ClientToServer, 1, 10, 1)); err != nil {
								b.Error(err)
								return
							}
						}
					})
				})
			})
		}
	}
}
