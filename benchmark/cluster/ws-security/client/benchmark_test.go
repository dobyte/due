package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"due-benchmark/cluster/ws-security/internal/profile"
	"github.com/dobyte/due/crypto/ecc/v2"
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

func newPeer(window int) *peer {
	return &peer{
		replies:  make(chan response, window),
		overflow: make(chan struct{}, 1),
	}
}

func TestRunPeersDistributesAndBoundsRequests(t *testing.T) {
	opts := options{requests: 101, connections: 3, window: 4}
	peers := make([]*peer, opts.connections)
	for i := range peers {
		p := newPeer(opts.window)
		p.push = func(m *cluster.Message) error {
			// Reply synchronously so a worker can fill its entire window before receiving.
			p.replies <- response{seq: m.Seq, at: time.Now()}
			return nil
		}
		peers[i] = p
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runPeers(ctx, peers, opts, "hello"); err != nil {
		t.Fatal(err)
	}
	for i, p := range peers {
		want := []int{34, 34, 33}[i]
		if p.attempted != want || p.sent != want || len(p.latencies) != want {
			t.Fatalf("peer %d: attempted=%d sent=%d replies=%d want=%d",
				i, p.attempted, p.sent, len(p.latencies), want)
		}
	}
}

func TestPeerRejectsUnexpectedSequence(t *testing.T) {
	p := newPeer(1)
	p.push = func(m *cluster.Message) error {
		p.replies <- response{seq: m.Seq + 1, at: time.Now()}
		return nil
	}
	if err := p.run(context.Background(), 70000, 1, 1, "hello"); err == nil {
		t.Fatal("accepted wrong sequence")
	}
	if len(p.latencies) != 0 {
		t.Fatal("invalid reply counted as success")
	}
}

func TestRunPeersCancelsOnPushFailure(t *testing.T) {
	errPush := errors.New("test push error")
	p := newPeer(1)
	p.push = func(*cluster.Message) error { return errPush }
	if err := runPeers(context.Background(), []*peer{p}, options{requests: 10, window: 1}, ""); !errors.Is(err, errPush) {
		t.Fatalf("expected push error, got %v", err)
	}
	if p.attempted != 1 || p.sent != 0 || len(p.latencies) != 0 {
		t.Fatal("push failure accounting is incorrect")
	}
}

func TestPeerStopsWhenReplyIsLost(t *testing.T) {
	p := newPeer(2)
	p.push = func(*cluster.Message) error { return nil }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := p.run(ctx, 1, 100, 2, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	if p.sent != 2 || len(p.latencies) != 0 {
		t.Fatal("worker exceeded the window or counted missing replies")
	}
}

// TestEncryptedWS exercises real WebSocket framing, ECC verification, both AEAD suites,
// and the benchmark receive/worker paths without external registry or locator services.
func TestEncryptedWS(t *testing.T) {
	profile.ConfigurePacket()
	for _, suiteName := range []string{"aesgcm", "chacha20poly1305"} {
		t.Run(suiteName, func(t *testing.T) {
			suite, err := profile.Suite(suiteName)
			if err != nil {
				t.Fatal(err)
			}
			server, serverErrors := startEchoServer(t, suite)
			defer server.Stop()
			for _, size := range []int{0, profile.MaxMessageSize} {
				message := strings.Repeat("a", size)
				component := client.NewClient(
					client.WithCodec(json.DefaultCodec),
					client.WithClient(ws.NewClient(
						ws.WithClientUrl("ws://"+server.Addr()+"/"),
						ws.WithClientHeartbeatInterval(0),
					)),
					client.WithSessionSecurity(ecc.NewVerifier("./etc/key.pub.pem"), suite),
				)
				component.Proxy().AddRouteHandler(profile.Greet, receive(message))
				component.Init()
				component.Start()
				err = benchmark(context.Background(), component.Proxy(), options{
					connections: 3, requests: 101, window: 4, size: size, timeout: 10 * time.Second,
				}, message)
				component.Close()
				component.Destroy()
				if err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-serverErrors:
				t.Fatal(err)
			default:
			}
		})
	}
}

type echoSession struct {
	pending   *handshake.Pending
	confirmed *handshake.Session
}

func startEchoServer(t *testing.T, suite aead.Suite) (network.Server, <-chan error) {
	t.Helper()
	signer := ecc.NewSigner(
		ecc.WithSignerPrivateKey("../gate/etc/key.pem"),
		ecc.WithSignerPublicKey("../gate/etc/key.pub.pem"),
	)
	security := handshake.NewServer(signer, suite)
	server := ws.NewServer(ws.WithServerAddr("127.0.0.1:0"), ws.WithServerHeartbeatInterval(0))
	serverErrors := make(chan error, 1)
	report := func(err error) {
		if err == nil {
			return
		}
		select {
		case serverErrors <- err:
		default:
		}
	}
	server.OnConnect(func(conn network.Conn) {
		conn.Attr().Set("session", &echoSession{})
	})
	server.OnHandshake(func(conn network.Conn, buf buffer.Buffer) {
		defer buf.Release()
		value, ok := conn.Attr().Get("session")
		if !ok {
			report(fmt.Errorf("missing echo session"))
			return
		}
		state := value.(*echoSession)
		frame, err := packet.ExtractHandshake(buf)
		if err != nil {
			report(err)
			return
		}
		if state.pending != nil {
			state.confirmed, err = security.Confirm(state.pending, frame)
			report(err)
			return
		}
		var hello []byte
		state.pending, hello, err = security.Accept(frame)
		if err != nil {
			report(err)
			return
		}
		packed, err := packet.PackHandshake(hello)
		if err == nil {
			err = conn.Push(packed)
			if err != nil {
				packed.Release()
			}
		}
		report(err)
	})
	server.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		report(echo(conn, buf))
		buf.Release()
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	return server, serverErrors
}

func echo(conn network.Conn, buf buffer.Buffer) error {
	value, ok := conn.Attr().Get("session")
	if !ok || value.(*echoSession).confirmed == nil {
		return fmt.Errorf("business message before confirmation")
	}
	session := value.(*echoSession).confirmed
	route, seq, payload, err := packet.ExtractMessage(buf)
	if err != nil {
		return err
	}
	plaintext, err := session.OpenMessage(nil, payload, handshake.ClientToServer, route, seq)
	if err != nil {
		return err
	}
	// Ensure JSON encoding and the maximal packet budget match the Node's echo contract.
	var req profile.Greeting
	if err = json.Unmarshal(plaintext, &req); err != nil {
		return err
	}
	sealed, err := session.SealMessage(nil, plaintext, handshake.ServerToClient, route, seq)
	if err != nil {
		return err
	}
	packed, err := packet.PackMessage(&packet.Message{Route: route, Seq: seq, Buffer: sealed})
	if err != nil {
		return err
	}
	if err = conn.Push(packed); err != nil {
		packed.Release()
	}
	return err
}
