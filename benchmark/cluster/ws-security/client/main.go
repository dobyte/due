package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"due-benchmark/cluster/ws-security/internal/profile"
	"github.com/dobyte/due/crypto/ecc/v2"
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/encoding/json"
)

func main() {
	var opts options
	flag.IntVar(&opts.connections, "c", 100, "number of connections")
	flag.IntVar(&opts.requests, "n", 1000000, "total number of requests")
	flag.IntVar(&opts.size, "size", 1024, "ASCII message bytes, excluding JSON and framing")
	flag.IntVar(&opts.window, "window", 16, "maximum outstanding requests per connection")
	flag.DurationVar(&opts.timeout, "timeout", 2*time.Minute, "business phase deadline")
	url := flag.String("url", "ws://127.0.0.1:3554/", "gateway WebSocket URL")
	suiteName := flag.String("suite", "aesgcm", "aesgcm or chacha20poly1305")
	publicKey := flag.String("public-key", "./etc/key.pub.pem", "gateway ECDSA public key file")
	flag.Parse()
	if err := opts.validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	suite, err := profile.Suite(*suiteName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	profile.ConfigurePacket()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	component := client.NewClient(
		client.WithContext(ctx),
		client.WithCodec(json.DefaultCodec),
		client.WithClient(ws.NewClient(
			ws.WithClientUrl(*url),
			ws.WithClientDialTimeout(10*time.Second),
			ws.WithClientWriteTimeout(10*time.Second),
			ws.WithClientCompression(false),
		)),
		client.WithSessionSecurity(ecc.NewVerifier(*publicKey), suite),
	)
	message := strings.Repeat("a", opts.size)
	component.Proxy().AddRouteHandler(profile.Greet, receive(message))
	fmt.Printf("target=%s suite=%s connections=%d requests=%d size=%d window=%d\n",
		*url, suite.Name(), opts.connections, opts.requests, opts.size, opts.window)
	component.Init()
	component.Start()
	err = benchmark(ctx, component.Proxy(), opts, message)
	component.Close()
	component.Destroy()
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
