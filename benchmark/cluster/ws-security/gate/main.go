package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"due-benchmark/cluster/ws-security/internal/profile"
	"github.com/dobyte/due/crypto/ecc/v2"
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/component/pprof"
	"github.com/gorilla/websocket"
)

func main() {
	suiteName := flag.String("suite", "aesgcm", "aesgcm or chacha20poly1305")
	privateKey := flag.String("private-key", "./etc/key.pem", "ECDSA private key file")
	publicKey := flag.String("public-key", "./etc/key.pub.pem", "ECDSA public key file")
	pprofAddr := flag.String("pprof", "", "override pprof address (empty uses etc.toml)")
	flag.Parse()
	suite, err := profile.Suite(*suiteName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	signer := ecc.NewSigner(
		ecc.WithSignerPrivateKey(*privateKey),
		ecc.WithSignerPublicKey(*publicKey),
	)
	// Check key loading before starting network components.
	probe := []byte("ws-security benchmark")
	signature, err := signer.Sign(probe)
	if err == nil {
		var valid bool
		valid, err = signer.Verify(probe, signature)
		if err == nil && !valid {
			err = fmt.Errorf("signing key pair does not match")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "signing keys:", err)
		os.Exit(1)
	}
	profile.ConfigurePacket()
	server := ws.NewServer()
	server.OnUpgrade(func(w http.ResponseWriter, r *http.Request) bool {
		if websocket.IsWebSocketUpgrade(r) {
			return true
		}
		http.Error(w, "websocket connection required", http.StatusUpgradeRequired)
		return false
	})
	component := gate.NewGate(
		gate.WithServer(server),
		gate.WithLocator(redis.NewLocator()),
		gate.WithRegistry(nacos.NewRegistry()),
		gate.WithSessionSecurity(signer, suite),
	)
	fmt.Printf("WS session security: %s\n", suite.Name())
	profiler := pprof.NewPProf()
	if *pprofAddr != "" {
		profiler = pprof.NewPProf(pprof.WithAddr(*pprofAddr))
	}
	container := due.NewContainer()
	container.Add(component, profiler)
	container.Serve()
}
