package memcache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dobyte/due/cache/memcache/v2"
)

var cache = memcache.NewCache(
	memcache.WithAddrs("localhost:11211"),
)

func TestCache_Get(t *testing.T) {
	ctx := context.Background()

	if err := cache.Set(ctx, "key", "value", time.Second); err != nil {
		t.Fatal(err)
	}

	value, err := cache.Get(ctx, "key").String()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(value)
}

func TestCache_Incr(t *testing.T) {
	ctx := context.Background()

	value, err := cache.IncrInt(ctx, "key", 1)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(value)

	value, err = cache.IncrInt(ctx, "key", 5)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(value)
}

func TestCache_Decr(t *testing.T) {
	ctx := context.Background()

	value, err := cache.DecrInt(ctx, "key", 20)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(value)
}

func TestCache_Delete(t *testing.T) {
	ctx := context.Background()

	total, err := cache.Delete(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(total)
}

func TestCache_GetSet(t *testing.T) {
	ctx := context.Background()

	value, err := cache.GetSet(ctx, "key", func() (any, error) {
		return "new value", nil
	}).Result()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(value)
}
