package quic

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	quicgo "github.com/quic-go/quic-go"
)

func testCredentials(t testing.TB) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

func testServer(t testing.TB, opts ...ServerOption) (*server, string) {
	t.Helper()
	cert, key := testCredentials(t)
	base := []ServerOption{WithServerAddr("127.0.0.1:0"), WithServerCredentials(cert, key),
		WithServerHeartbeatInterval(0), WithServerCloseTimeout(300 * time.Millisecond)}
	s := NewServer(append(base, opts...)...).(*server)
	t.Cleanup(func() { _ = s.Stop() })
	return s, cert
}

func startServer(t testing.TB, s *server) {
	t.Helper()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
}

func dialClient(t testing.TB, s *server, cert string, opts ...ClientOption) *conn {
	t.Helper()
	base := []ClientOption{WithClientAddr(s.Addr()), WithClientCredentials(cert, "localhost"),
		WithClientHeartbeatInterval(0), WithClientCloseTimeout(300 * time.Millisecond)}
	cl := NewClient(append(base, opts...)...)
	c, err := cl.Dial()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(true) })
	return c.(*conn)
}

func await[T any](t testing.TB, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
		var zero T
		return zero
	}
}

func message(t testing.TB, data []byte) buffer.Buffer {
	t.Helper()
	buf, err := packet.PackMessage(&packet.Message{Route: 1, Buffer: data})
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestWelcomeWithoutPeriodicHeartbeat(t *testing.T) {
	s, cert := testServer(t)
	var order atomic.Int32
	s.OnConnect(func(c network.Conn) {
		buf := message(t, []byte("welcome"))
		if err := c.Push(buf); err != nil {
			buf.Release()
			t.Error(err)
		}
	})
	startServer(t, s)
	received := make(chan string, 1)
	cl := NewClient(WithClientAddr(s.Addr()), WithClientCredentials(cert, "localhost"), WithClientHeartbeatInterval(0), WithClientDialTimeout(0))
	cl.OnConnect(func(network.Conn) { order.Store(1) })
	cl.OnReceive(func(c network.Conn, buf buffer.Buffer) {
		defer buf.Release()
		if order.Load() != 1 {
			t.Error("receive preceded connect")
		}
		msg, err := packet.UnpackMessage(buf)
		if err != nil {
			t.Error(err)
			return
		}
		received <- string(msg.Buffer)
		_ = c.Close(true)
	})
	disconnected := make(chan struct{}, 1)
	cl.OnDisconnect(func(network.Conn) { disconnected <- struct{}{} })
	c, err := cl.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(true)
	if got := await(t, received); got != "welcome" {
		t.Fatal(got)
	}
	await(t, disconnected)
}

func TestGracefulCloseDeliversAcceptedMessages(t *testing.T) {
	s, cert := testServer(t, WithServerCloseTimeout(time.Second))
	const count = 100
	received := make(chan []byte, count)
	s.OnReceive(func(_ network.Conn, buf buffer.Buffer) {
		defer buf.Release()
		msg, err := packet.UnpackMessage(buf)
		if err != nil {
			t.Error(err)
			return
		}
		received <- bytes.Clone(msg.Buffer)
	})
	startServer(t, s)
	c := dialClient(t, s, cert, WithClientCloseTimeout(time.Second))
	for i := range count {
		buf := message(t, []byte{byte(i)})
		if err := c.Push(buf); err != nil {
			buf.Release()
			t.Fatal(err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	for i := range count {
		if got := await(t, received); !bytes.Equal(got, []byte{byte(i)}) {
			t.Fatalf("message %d: %v", i, got)
		}
	}
	await(t, c.done)
}

func TestHeartbeatCallbacks(t *testing.T) {
	s, cert := testServer(t, WithServerHeartbeatInterval(100*time.Millisecond))
	serverHB, clientHB := make(chan struct{}, 10), make(chan struct{}, 10)
	s.OnHeartbeat(func(network.Conn, int64) {
		select {
		case serverHB <- struct{}{}:
		default:
		}
	})
	startServer(t, s)
	cl := NewClient(WithClientAddr(s.Addr()), WithClientCredentials(cert, "localhost"), WithClientHeartbeatInterval(100*time.Millisecond))
	cl.OnHeartbeat(func(network.Conn, int64) {
		select {
		case clientHB <- struct{}{}:
		default:
		}
	})
	c, err := cl.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(true)
	await(t, serverHB)
	await(t, clientHB)
}

func TestRestartAndCallbackStop(t *testing.T) {
	s, cert := testServer(t)
	stopped := make(chan error, 1)
	s.OnConnect(func(network.Conn) { stopped <- s.Stop() })
	for range 3 {
		startServer(t, s)
		c := dialClient(t, s, cert)
		if err := await(t, stopped); err != nil {
			t.Fatal(err)
		}
		await(t, c.done)
	}
}

func rawDial(t testing.TB, s *server, cert string) *quicgo.Conn {
	t.Helper()
	config, err := makeClientTLSConfig(cert, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	config.NextProtos = []string{alpn}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	qc, err := quicgo.DialAddr(ctx, s.Addr(), config, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = qc.CloseWithError(0, "test cleanup") })
	return qc
}

func TestPendingStreamLimitAndTimeout(t *testing.T) {
	s, cert := testServer(t, WithServerMaxConnNum(1), WithServerHandshakeTimeout(500*time.Millisecond))
	startServer(t, s)
	first := rawDial(t, s, cert)
	s.mu.Lock()
	manager := s.run.manager
	s.mu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for manager.total.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if manager.total.Load() != 1 {
		t.Fatal("pending stream not counted")
	}
	second := rawDial(t, s, cert)
	await(t, second.Context().Done())
	await(t, first.Context().Done())
	deadline = time.Now().Add(2 * time.Second)
	for manager.total.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if manager.total.Load() != 0 {
		t.Fatal("pending slot leaked")
	}
}

type trackedBuffer struct {
	data     []byte
	releases atomic.Int32
}

func (b *trackedBuffer) Len() int                             { return len(b.data) }
func (b *trackedBuffer) Nodes() int                           { return 1 }
func (b *trackedBuffer) Bytes() []byte                        { return b.data }
func (b *trackedBuffer) VisitBytes(fn func([]byte) bool) bool { return fn(b.data) }
func (b *trackedBuffer) Delay(int)                            {}
func (b *trackedBuffer) Release()                             { b.releases.Add(1) }

func TestFullQueueCloseAndOwnership(t *testing.T) {
	s, cert := testServer(t)
	startServer(t, s)
	qc := rawDial(t, s, cert)
	stream, err := qc.OpenStreamSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately leave the writer unstarted to make queue saturation deterministic.
	c := newConn(1, qc, stream, connOptions{queueSize: 1, closeTimeout: time.Second})
	first, second := &trackedBuffer{data: []byte{1}}, &trackedBuffer{data: []byte{2}}
	if err := c.Push(first); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- c.Push(second) }()
	if err := c.Close(true); err != nil {
		t.Fatal(err)
	}
	if err := await(t, result); !errors.Is(err, errors.ErrConnectionClosed) {
		t.Fatal(err)
	}
	if second.releases.Load() != 0 {
		t.Fatal("rejected buffer was released")
	}
	for buf := range c.queue {
		buf.Release()
	}
	if first.releases.Load() != 1 {
		t.Fatal("accepted buffer not released once")
	}
}

func TestWriteTimeoutAndForceCloseUnderBackpressure(t *testing.T) {
	for _, timeout := range []time.Duration{0, 100 * time.Millisecond} {
		t.Run(timeout.String(), func(t *testing.T) {
			s, cert := testServer(t)
			blocked := make(chan struct{})
			defer close(blocked)
			s.OnConnect(func(network.Conn) { <-blocked })
			startServer(t, s)
			c := dialClient(t, s, cert, WithClientWriteTimeout(timeout), WithClientWriteQueueSize(1))
			buf := &trackedBuffer{data: make([]byte, 32<<20)}
			if err := c.Push(buf); err != nil {
				t.Fatal(err)
			}
			if timeout == 0 {
				time.Sleep(30 * time.Millisecond)
				closed := make(chan error, 1)
				go func() { closed <- c.Close(true) }()
				if err := await(t, closed); err != nil {
					t.Fatal(err)
				}
			}
			await(t, c.done)
			if buf.releases.Load() != 1 {
				t.Fatalf("release count %d", buf.releases.Load())
			}
		})
	}
}

func TestConcurrentPushCloseAndStaleReference(t *testing.T) {
	s, cert := testServer(t)
	startServer(t, s)
	c := dialClient(t, s, cert)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				buf := packet.PackHeartbeat()
				if err := c.Push(buf); err != nil {
					buf.Release()
					return
				}
			}
		}()
	}
	_ = c.Close()
	_ = c.Close(true)
	wg.Wait()
	await(t, c.done)
	fresh := dialClient(t, s, cert)
	if err := c.Bind(123); !errors.Is(err, errors.ErrConnectionClosed) {
		t.Fatal(err)
	}
	_ = c.Close(true)
	if fresh.State() != network.ConnOpened {
		t.Fatal("old reference closed new connection")
	}
}

func TestTLSConfigValidation(t *testing.T) {
	s, cert := testServer(t)
	startServer(t, s)
	config, err := makeClientTLSConfig(cert, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	config.NextProtos = []string{"custom"}
	cl := NewClient(WithClientAddr(s.Addr()), WithClientTLSConfig(config))
	c, err := cl.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(true)
	if len(config.NextProtos) != 1 || config.NextProtos[0] != "custom" {
		t.Fatal("mutated caller TLS config")
	}
	bad := NewClient(WithClientAddr(s.Addr()), WithClientCredentials(filepath.Join(t.TempDir(), "missing"), "localhost"))
	if _, err := bad.Dial(); err == nil {
		t.Fatal("ignored invalid CA")
	}
	secure := NewClient(WithClientAddr(s.Addr()))
	if _, err := secure.Dial(); err == nil {
		t.Fatal("accepted an untrusted certificate")
	}
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(b []byte) (int, error) { return w.Buffer.Write(b[:min(2, len(b))]) }

func TestCompositeShortWrite(t *testing.T) {
	buf := buffer.NewNocopyBuffer([]byte("abc"), []byte("def"), []byte("ghi"))
	defer buf.Release()
	var w shortWriter
	if err := writeBuffer(&w, buf); err != nil {
		t.Fatal(err)
	}
	if w.String() != "abcdefghi" {
		t.Fatal(w.String())
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func TestZeroWriteFails(t *testing.T) {
	buf := buffer.NewBytes([]byte("data"))
	defer buf.Release()
	if err := writeBuffer(zeroWriter{}, buf); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

func TestAuthorizationTimerReset(t *testing.T) {
	s, cert := testServer(t, WithServerAuthorizeTimeout(150*time.Millisecond))
	connected := make(chan *conn, 1)
	s.OnConnect(func(c network.Conn) { connected <- c.(*conn) })
	startServer(t, s)
	clientConn := dialClient(t, s, cert)
	c := await(t, connected)
	if err := c.Bind(42); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if c.State() != network.ConnOpened {
		t.Fatal("bound connection expired")
	}
	if err := c.Unbind(); err != nil {
		t.Fatal(err)
	}
	await(t, c.done)
	await(t, clientConn.done)
}

func TestQueueTimeoutPreservesOwnership(t *testing.T) {
	c := newConn(1, nil, nil, connOptions{queueSize: 1, writeTimeout: 10 * time.Millisecond})
	first, second := &trackedBuffer{data: []byte{1}}, &trackedBuffer{data: []byte{2}}
	if err := c.Push(first); err != nil {
		t.Fatal(err)
	}
	if err := c.Push(second); !errors.Is(err, errors.ErrWriteTimeout) {
		t.Fatal(err)
	}
	if second.releases.Load() != 0 {
		t.Fatal("enqueue timeout consumed ownership")
	}
	(<-c.queue).Release()
	second.Release()
}

func TestServerStopClosesPendingStream(t *testing.T) {
	s, cert := testServer(t)
	startServer(t, s)
	qc := rawDial(t, s, cert)
	stopped := make(chan error, 1)
	go func() { stopped <- s.Stop() }()
	if err := await(t, stopped); err != nil {
		t.Fatal(err)
	}
	await(t, qc.Context().Done())
}

func BenchmarkWriteBuffer(b *testing.B) {
	for _, nodes := range []int{1, 3} {
		name := "single"
		if nodes > 1 {
			name = "composite"
		}
		b.Run(name, func(b *testing.B) {
			var buf buffer.Buffer = buffer.NewBytes(make([]byte, 1024))
			if nodes > 1 {
				buf.Release()
				buf = buffer.NewNocopyBuffer(make([]byte, 16), make([]byte, 1000), make([]byte, 8))
			}
			defer buf.Release()
			writer := newBufferWriter(io.Discard)
			b.ReportAllocs()
			b.SetBytes(int64(buf.Len()))
			b.ResetTimer()
			for b.Loop() {
				if err := writer.write(buf); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkQUICEcho(b *testing.B) {
	s, cert := testServer(b)
	s.OnReceive(func(c network.Conn, buf buffer.Buffer) {
		if err := c.Push(buf); err != nil {
			buf.Release()
		}
	})
	startServer(b, s)
	received := make(chan struct{}, 1)
	cl := NewClient(WithClientAddr(s.Addr()), WithClientCredentials(cert, "localhost"), WithClientHeartbeatInterval(0))
	cl.OnReceive(func(_ network.Conn, buf buffer.Buffer) { buf.Release(); received <- struct{}{} })
	c, err := cl.Dial()
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close(true)
	data := make([]byte, 1024)
	b.ReportAllocs()
	b.SetBytes(1024)
	b.ResetTimer()
	for b.Loop() {
		buf, err := packet.PackMessage(&packet.Message{Route: 1, Buffer: data})
		if err != nil {
			b.Fatal(err)
		}
		if err := c.Push(buf); err != nil {
			buf.Release()
			b.Fatal(err)
		}
		<-received
	}
}
