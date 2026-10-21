package main

import (
	"testing"
	"time"

	"due-benchmark/internal/securitytest"
	"github.com/dobyte/due/crypto/ecc/v2"
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/encoding/json"
)

// TestSecurityFlag exercises the same client construction used by the command-line benchmark.
func TestSecurityFlag(t *testing.T) {
	for _, secure := range []bool{false, true} {
		name := "plaintext"
		if secure {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			signer := ecc.NewSigner(ecc.WithSignerPrivateKey("../gate/resource/key.pem"))
			verifier := ecc.NewSigner(ecc.WithSignerPublicKey("resource/key.pub.pem"))
			server := ws.NewServer(ws.WithServerAddr("127.0.0.1:0"), ws.WithServerHeartbeatInterval(0), ws.WithServerWriteTimeout(time.Second), ws.WithServerCloseTimeout(time.Second))
			securitytest.StartEcho(t, server, signer, secure)
			cfg := config{security: secure, addr: "ws://" + server.Addr() + "/", duration: 100 * time.Millisecond, warmup: 20 * time.Millisecond, requests: 20, rate: 20}
			component := newBenchmarkClient(cfg, verifier, client.WithCodec(json.DefaultCodec))
			r := &runner{cfg: cfg, proxy: component.Proxy()}
			component.Proxy().AddRouteHandler(greet, r.handleReply)
			component.Init()
			component.Start()
			t.Cleanup(func() { component.Close(); component.Destroy() })
			for _, mode := range []string{modeThroughput, modeLatency, modeRate} {
				t.Run(mode, func(t *testing.T) {
					r.cfg.mode = mode
					res, err := r.runRound(2, 64, 1)
					if err != nil {
						t.Fatal(err)
					}
					if res.failure != "" || !res.drained || res.recv == 0 || res.recv != res.sent {
						t.Fatalf("failed round: failure=%q drained=%v sent=%d recv=%d", res.failure, res.drained, res.sent, res.recv)
					}
					if mode != modeThroughput && res.samples != res.recv {
						t.Fatalf("samples=%d recv=%d", res.samples, res.recv)
					}
				})
			}
		})
	}
}
