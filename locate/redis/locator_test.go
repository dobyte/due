package redis_test

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/locate/redis"
	"strconv"
	"testing"
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
		err := locator.Set(context.Background(), int64(i), cluster.Node, strconv.Itoa(i))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocator_Watch(t *testing.T) {
	locator.Watch(context.Background(), cluster.Node)
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
