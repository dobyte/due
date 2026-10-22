package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/bridge"
	"github.com/dobyte/due/v2/internal/transporter/node"
)

func TestBridgeResolverFallbackAndRebuild(t *testing.T) {
	s, _ := newTestServer(t)
	target := bridge.Target(s.Endpoint().String(), s.Metadata(nil)[bridge.MetadataKey])
	for _, disabled := range []bool{false, true} {
		opts := newTestClientOptions("gate")
		opts.ConnNum = 1
		opts.DisableLocal = disabled
		b := node.NewBuilder(opts)
		t.Cleanup(b.Close)
		c, err := b.Resolve(target)
		if err != nil {
			t.Fatal(err)
		}
		_, remote := c.(*node.Client)
		if remote != disabled {
			t.Fatalf("disabled=%v remote=%v", disabled, remote)
		}
		if state, err := c.GetState(context.Background()); err != nil || state != cluster.Work {
			t.Fatalf("disabled=%v state=%v err=%v", disabled, state, err)
		}
		b.Close()
		if _, err := b.Resolve(target); !errors.Is(err, errors.ErrClientClosed) {
			t.Fatal(err)
		}
	}
	b := node.NewBuilder(&node.ClientOptions{Kind: cluster.Gate, CallTimeout: time.Second})
	defer b.Close()
	old, err := b.Resolve(target)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Stop()
	fresh, err := node.NewServer(&provider{}, &node.ServerOptions{DisableRemote: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Start(); err != nil {
		t.Fatal(err)
	}
	defer fresh.Stop()
	if _, err := old.GetState(context.Background()); !errors.Is(err, errors.ErrServerClosed) {
		t.Fatal("old provider still accessible", err)
	}
	newTarget := bridge.Target(fresh.Endpoint().String(), fresh.Metadata(nil)[bridge.MetadataKey])
	current, err := b.Resolve(newTarget)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := current.GetState(context.Background()); err != nil {
		t.Fatal(err)
	}
}
