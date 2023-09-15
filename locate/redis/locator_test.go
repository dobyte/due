package redis_test

import (
	"context"
	"fmt"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/locate/redis"
	"strconv"
	"testing"
	"time"
)

var locator = redis.NewLocator(
	redis.WithAddrs(
		"127.0.0.1:7000",
		"127.0.0.1:7001",
		"127.0.0.1:7002",
		"127.0.0.1:7003",
		"127.0.0.1:7004",
		"127.0.0.1:7005",
	),
)

func TestLocator_Set(t *testing.T) {
	for i := 1; i <= 6; i++ {
		var kind cluster.Kind

		if i%2 == 0 {
			kind = cluster.Node
		} else {
			kind = cluster.Gate
		}

		err := locator.Set(context.Background(), int64(i), kind, strconv.Itoa(i))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocator_Watch(t *testing.T) {
	watcher1, err := locator.Watch(context.Background(), cluster.Gate, cluster.Node)
	if err != nil {
		t.Fatal(err)
	}

	watcher2, err := locator.Watch(context.Background(), cluster.Gate)
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

func TestLocator_Get(t *testing.T) {
	for i := 1; i <= 6; i++ {
		insID, err := locator.Get(context.Background(), int64(i), cluster.Node)
		if err != nil {
			t.Fatal(err)
		}

		t.Log(insID)
	}
}

func TestLocator_Rem(t *testing.T) {
	for i := 1; i <= 6; i++ {
		err := locator.Rem(context.Background(), int64(i), cluster.Node, strconv.Itoa(i))
		if err != nil {
			t.Fatal(err)
		}
	}
}
