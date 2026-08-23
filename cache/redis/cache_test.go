package redis_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dobyte/due/cache/redis/v2"
)

var cache = redis.NewCache(
	redis.WithAddrs("localhost:6379"),
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

func TestCache_Has(t *testing.T) {
	ctx := context.Background()

	if err := cache.Set(ctx, "key", "value", time.Second); err != nil {
		t.Fatal(err)
	}

	ok, err := cache.Has(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(ok)
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
