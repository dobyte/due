package gate

import (
	"strconv"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// discardSecurityConn models a successful write without adding a capture copy to the benchmark.
type discardSecurityConn struct {
	*securityConn
}

func (c *discardSecurityConn) Push(buf buffer.Buffer) error {
	buf.Release()
	return nil
}

// benchmarkSecurityConn establishes a session outside the measured message path.
func benchmarkSecurityConn(b *testing.B) network.Conn {
	b.Helper()
	signer := &securitySigner{key: []byte("gate-security-benchmark")}
	g := NewGate(WithSessionSecurity(signer))
	conn := &securityConn{pushed: make(chan []byte, 1)}
	wrapped := g.security.wrap(&discardSecurityConn{securityConn: conn})
	client := handshake.NewClient(signer, aesgcm.Suite)
	hello, err := client.Hello()
	if err != nil {
		b.Fatal(err)
	}
	g.security.advanceHandshake(conn, hello)
	payload, err := packet.ExtractHandshake(buffer.NewBytes(<-conn.pushed, true))
	if err != nil {
		b.Fatal(err)
	}
	_, finished, err := client.Handle(payload)
	if err != nil {
		b.Fatal(err)
	}
	g.security.advanceHandshake(conn, finished)
	return wrapped
}

// BenchmarkSecurityPush includes extraction, encryption, packing and buffer release.
func BenchmarkSecurityPush(b *testing.B) {
	for _, size := range []int{64, 1024, 4096} {
		for _, recipients := range []int{1, 8} {
			b.Run(strconv.Itoa(size)+"/recipients="+strconv.Itoa(recipients), func(b *testing.B) {
				conns := make([]network.Conn, recipients)
				for i := range conns {
					conns[i] = benchmarkSecurityConn(b)
				}
				data := make([]byte, size)
				b.SetBytes(int64(size * recipients))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					buf, err := packet.PackMessage(&packet.Message{Route: 10, Seq: 1, Buffer: data})
					if err != nil {
						b.Fatal(err)
					}
					buf.Delay(recipients)
					for _, conn := range conns {
						if err = conn.Push(buf); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}

// BenchmarkSecurityReceive includes sender encryption, packing and gate decryption. The gate is
// shut so delivery releases the plaintext immediately, without measuring routing or network I/O.
func BenchmarkSecurityReceive(b *testing.B) {
	for _, size := range []int{0, 64, 1024, 4096} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			g, conn, _, sender := newSecurityTestPeer(b, 1)
			data := make([]byte, size)
			sealedBuffer := make([]byte, 0, size+sender.Overhead())
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sealed, err := sender.SealMessage(sealedBuffer, data, handshake.ClientToServer, 10, 1)
				if err != nil {
					b.Fatal(err)
				}
				buf, err := packet.PackMessage(&packet.Message{Route: 10, Seq: 1, Buffer: sealed})
				if err != nil {
					b.Fatal(err)
				}
				g.security.decrypt(conn, buf)
			}
			b.StopTimer()
			if conn.closed.Load() {
				b.Fatal("gate rejected authenticated traffic")
			}
		})
	}
}
