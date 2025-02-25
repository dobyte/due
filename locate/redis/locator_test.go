package redis_test

import (
	"context"
	"fmt"
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/utils/xuuid"
	"testing"
	"time"
)

var locator = redis.NewLocator(
	redis.WithAddrs("127.0.0.1:6379"),
)

func TestLocator_BindGate(t *testing.T) {
	ctx := context.Background()
	uid := int64(1)
	gid := xuuid.UUID()

	if err := locator.BindGate(ctx, uid, gid); err != nil {
		t.Fatal(err)
	}
}

func TestLocator_BindNode(t *testing.T) {
	ctx := context.Background()
	uid := int64(1)
	nid := xuuid.UUID()
	name := "node1"

	if err := locator.BindNode(ctx, uid, name, nid); err != nil {
		t.Fatal(err)
	}
}

func TestLocator_UnbindGate(t *testing.T) {
	ctx := context.Background()
	uid := int64(1)
	gid := xuuid.UUID()

	if err := locator.BindGate(ctx, uid, gid); err != nil {
		t.Fatal(err)
	}

	if err := locator.UnbindGate(ctx, uid, gid); err != nil {
		t.Fatal(err)
	}
}

func TestLocator_UnbindNode(t *testing.T) {
	ctx := context.Background()
	uid := int64(1)
	nid1 := xuuid.UUID()
	nid2 := xuuid.UUID()
	name1 := "node1"
	name2 := "node2"

	if err := locator.BindNode(ctx, uid, name1, nid1); err != nil {
		t.Fatal(err)
	}

	if err := locator.BindNode(ctx, uid, name2, nid2); err != nil {
		t.Fatal(err)
	}

	if err := locator.UnbindNode(ctx, uid, name2, nid2); err != nil {
		t.Fatal(err)
	}
}

func TestLocator_Watch(t *testing.T) {
	watcher1, err := locator.Watch(context.Background(), cluster.Gate.String(), cluster.Node.String())
	if err != nil {
		t.Fatal(err)
	}

	watcher2, err := locator.Watch(context.Background(), cluster.Gate.String())
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			events, err := watcher1.Next()
			if err != nil {
				t.Errorf("goroutine 1: %v", err)
				return
			}

			fmt.Println("goroutine 1: new event entity")

			for _, event := range events {
				t.Logf("goroutine 1: %+v", event)
			}
		}
	}()

	go func() {
		for {
			events, err := watcher2.Next()
			if err != nil {
				t.Errorf("goroutine 2: %v", err)
				return
			}

			fmt.Println("goroutine 2: new event entity")

			for _, event := range events {
				t.Logf("goroutine 2: %+v", event)
			}
		}
	}()

	time.Sleep(60 * time.Second)
}
