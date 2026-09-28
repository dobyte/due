package link

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/registry"
)

// triggerRecord 记录一次事件触发的入参
type triggerRecord struct {
	gid   string
	cid   int64
	uid   int64
	event cluster.Event
}

// mockProvider 捕获事件触发的测试服务提供者
type mockProvider struct {
	ch chan triggerRecord
}

func (p *mockProvider) Trigger(ctx context.Context, gid string, cid, uid int64, event cluster.Event) error {
	p.ch <- triggerRecord{gid: gid, cid: cid, uid: uid, event: event}
	return nil
}

func (p *mockProvider) Deliver(ctx context.Context, gid, nid string, cid, uid int64, buf buffer.Buffer) error {
	buf.Release()
	return nil
}

func (p *mockProvider) GetState() (cluster.State, error) { return cluster.Work, nil }

func (p *mockProvider) SetState(state cluster.State) error { return nil }

// listenFreeAddr 获取一个空闲的本地监听地址
func listenFreeAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

// newTestNodeLinker 创建测试用的节点链接器
func newTestNodeLinker() *NodeLinker {
	return NewNodeLinker(context.Background(), &Options{
		ID:                  "test-gate",
		Kind:                cluster.Gate,
		Dispatch:            cluster.Random,
		ConnNum:             1,
		CallTimeout:         3 * time.Second,
		DialTimeout:         500 * time.Millisecond,
		DialRetryTimes:      1,
		FaultRecoveryTime:   5 * time.Second,
		CommandQueueSize:    128,
		CommandWriteTimeout: time.Second,
	})
}

func TestNodeLinkerTriggerNotFoundEvent(t *testing.T) {
	linker := newTestNodeLinker()

	err := linker.Trigger(context.Background(), &TriggerArgs{Event: cluster.Connect, CID: 100, UID: 200})
	if !errors.Is(err, errors.ErrNotFoundEvent) {
		t.Fatalf("expect ErrNotFoundEvent, got %v", err)
	}
}

func TestNodeLinkerTrigger(t *testing.T) {
	provider := &mockProvider{ch: make(chan triggerRecord, 1)}

	addr := listenFreeAddr(t)

	server, err := node.NewServer(provider, &node.ServerOptions{Addr: addr})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}
	if err = server.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	defer server.Stop()

	linker := newTestNodeLinker()

	// 同时注册一个可达节点与一个不可达节点：
	// 不可达节点的触发失败不得阻塞调用方，也不得影响可达节点的事件投递
	linker.dispatcher.ReplaceServices(
		&registry.ServiceInstance{
			ID:       "test-node",
			Name:     cluster.Node.String(),
			Kind:     cluster.Node.String(),
			State:    cluster.Work.String(),
			Events:   []int{int(cluster.Connect)},
			Endpoint: "drpc://" + addr,
		},
		&registry.ServiceInstance{
			ID:       "dead-node",
			Name:     cluster.Node.String(),
			Kind:     cluster.Node.String(),
			State:    cluster.Work.String(),
			Events:   []int{int(cluster.Connect)},
			Endpoint: "drpc://127.0.0.1:1",
		},
	)

	start := time.Now()

	if err = linker.Trigger(context.Background(), &TriggerArgs{Event: cluster.Connect, CID: 100, UID: 200}); err != nil {
		t.Fatalf("trigger failed: %v", err)
	}

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("trigger should return promptly, took %v", elapsed)
	}

	select {
	case rec := <-provider.ch:
		if rec.gid != "test-gate" || rec.cid != 100 || rec.uid != 200 || rec.event != cluster.Connect {
			t.Fatalf("unexpected trigger record: %+v", rec)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not receive the event in time")
	}
}
