package cache

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
)

// stubCache is a configurable in-memory Cache implementation used to exercise the package level
// helpers without depending on any external service.
type stubCache struct {
	closed   bool
	closeErr error

	hasValue bool
	hasErr   error

	getResult Result

	setErr error

	getSetResult Result

	delCount int64
	delErr   error

	incrIntValue   int64
	incrIntErr     error
	incrFloatValue float64
	incrFloatErr   error
	decrIntValue   int64
	decrIntErr     error
	decrFloatValue float64
	decrFloatErr   error

	prefix string
	client any
}

func (c *stubCache) Has(ctx context.Context, key string) (bool, error) {
	return c.hasValue, c.hasErr
}

func (c *stubCache) Get(ctx context.Context, key string, def ...any) Result {
	return c.getResult
}

func (c *stubCache) Set(ctx context.Context, key string, value any, expiration ...time.Duration) error {
	return c.setErr
}

func (c *stubCache) GetSet(ctx context.Context, key string, fn SetValueFunc) Result {
	return c.getSetResult
}

func (c *stubCache) Delete(ctx context.Context, keys ...string) (int64, error) {
	return c.delCount, c.delErr
}

func (c *stubCache) IncrInt(ctx context.Context, key string, value int64) (int64, error) {
	return c.incrIntValue, c.incrIntErr
}

func (c *stubCache) IncrFloat(ctx context.Context, key string, value float64) (float64, error) {
	return c.incrFloatValue, c.incrFloatErr
}

func (c *stubCache) DecrInt(ctx context.Context, key string, value int64) (int64, error) {
	return c.decrIntValue, c.decrIntErr
}

func (c *stubCache) DecrFloat(ctx context.Context, key string, value float64) (float64, error) {
	return c.decrFloatValue, c.decrFloatErr
}

func (c *stubCache) AddPrefix(key string) string {
	return c.prefix + key
}

func (c *stubCache) Client() any {
	return c.client
}

func (c *stubCache) Close() error {
	c.closed = true
	return c.closeErr
}

// TestSetCacheIgnoresNil verifies that a nil cache never replaces or closes the installed one.
func TestSetCacheIgnoresNil(t *testing.T) {
	current := &stubCache{}
	globalCache = current
	defer func() { globalCache = nil }()

	SetCache(nil)

	if GetCache() != current {
		t.Error("SetCache(nil) must not replace the installed cache")
	}
	if current.closed {
		t.Error("SetCache(nil) must not close the installed cache")
	}
}

// TestSetCacheInstallsAndReplaces verifies the install and replacement behaviour of SetCache.
func TestSetCacheInstallsAndReplaces(t *testing.T) {
	globalCache = nil
	defer func() { globalCache = nil }()

	first := &stubCache{}
	SetCache(first)

	if GetCache() != first {
		t.Error("GetCache must return the installed cache")
	}
	if first.closed {
		t.Error("a freshly installed cache must not be closed")
	}

	second := &stubCache{}
	SetCache(second)

	if GetCache() != second {
		t.Error("SetCache must replace the previously installed cache")
	}
	if !first.closed {
		t.Error("SetCache must close the previously installed cache")
	}
}

// TestSetCacheCloseError verifies that a failing close does not prevent the replacement.
func TestSetCacheCloseError(t *testing.T) {
	globalCache = nil
	defer func() { globalCache = nil }()

	failing := &stubCache{closeErr: errors.New("close failed")}
	SetCache(failing)

	replacement := &stubCache{}
	SetCache(replacement)

	if GetCache() != replacement {
		t.Error("SetCache must install the new cache even when closing the old one fails")
	}
	if !failing.closed {
		t.Error("SetCache must attempt to close the old cache")
	}
}

// TestGlobalCacheWithoutInstance verifies that every global helper degrades gracefully when no
// cache instance is installed.
func TestGlobalCacheWithoutInstance(t *testing.T) {
	globalCache = nil
	defer func() { globalCache = nil }()

	ctx := context.Background()

	t.Run("Has", func(t *testing.T) {
		ok, err := Has(ctx, "key")
		if ok {
			t.Error("Has must report false without a cache instance")
		}
		if err != errors.ErrMissingCacheInstance {
			t.Errorf("Has error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("Get", func(t *testing.T) {
		if err := Get(ctx, "key").Err(); err != errors.ErrMissingCacheInstance {
			t.Errorf("Get error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("Set", func(t *testing.T) {
		if err := Set(ctx, "key", "value"); err != errors.ErrMissingCacheInstance {
			t.Errorf("Set error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("GetSet", func(t *testing.T) {
		fn := func() (any, error) { return "value", nil }
		if err := GetSet(ctx, "key", fn).Err(); err != errors.ErrMissingCacheInstance {
			t.Errorf("GetSet error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		n, err := Delete(ctx, "key")
		if n != 0 {
			t.Errorf("Delete count = %d, want 0", n)
		}
		if err != errors.ErrMissingCacheInstance {
			t.Errorf("Delete error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("IncrInt", func(t *testing.T) {
		n, err := IncrInt(ctx, "key", 1)
		if n != 0 {
			t.Errorf("IncrInt value = %d, want 0", n)
		}
		if err != errors.ErrMissingCacheInstance {
			t.Errorf("IncrInt error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("IncrFloat", func(t *testing.T) {
		n, err := IncrFloat(ctx, "key", 1)
		if n != 0 {
			t.Errorf("IncrFloat value = %v, want 0", n)
		}
		if err != errors.ErrMissingCacheInstance {
			t.Errorf("IncrFloat error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("DecrInt", func(t *testing.T) {
		n, err := DecrInt(ctx, "key", 1)
		if n != 0 {
			t.Errorf("DecrInt value = %d, want 0", n)
		}
		if err != errors.ErrMissingCacheInstance {
			t.Errorf("DecrInt error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("DecrFloat", func(t *testing.T) {
		n, err := DecrFloat(ctx, "key", 1)
		if n != 0 {
			t.Errorf("DecrFloat value = %v, want 0", n)
		}
		if err != errors.ErrMissingCacheInstance {
			t.Errorf("DecrFloat error = %v, want %v", err, errors.ErrMissingCacheInstance)
		}
	})

	t.Run("AddPrefix", func(t *testing.T) {
		if got := AddPrefix("key"); got != "" {
			t.Errorf("AddPrefix = %q, want empty", got)
		}
	})

	t.Run("Client", func(t *testing.T) {
		if got := Client(); got != nil {
			t.Errorf("Client = %v, want nil", got)
		}
	})

	t.Run("Close", func(t *testing.T) {
		if err := Close(); err != nil {
			t.Errorf("Close error = %v, want nil", err)
		}
	})
}

// TestGlobalCacheWithInstance verifies that every global helper delegates to the installed cache.
func TestGlobalCacheWithInstance(t *testing.T) {
	stub := &stubCache{
		hasValue:       true,
		getResult:      NewResult("get"),
		getSetResult:   NewResult("getset"),
		delCount:       3,
		incrIntValue:   11,
		incrFloatValue: 1.5,
		decrIntValue:   7,
		decrFloatValue: 0.5,
		prefix:         "due:",
		client:         "client",
	}
	globalCache = stub
	defer func() { globalCache = nil }()

	ctx := context.Background()

	t.Run("Has", func(t *testing.T) {
		ok, err := Has(ctx, "key")
		if !ok || err != nil {
			t.Errorf("Has = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		if got := Get(ctx, "key"); got != stub.getResult {
			t.Errorf("Get = %v, want %v", got, stub.getResult)
		}
	})

	t.Run("Set", func(t *testing.T) {
		if err := Set(ctx, "key", "value"); err != nil {
			t.Errorf("Set error = %v, want nil", err)
		}
	})

	t.Run("GetSet", func(t *testing.T) {
		fn := func() (any, error) { return "value", nil }
		if got := GetSet(ctx, "key", fn); got != stub.getSetResult {
			t.Errorf("GetSet = %v, want %v", got, stub.getSetResult)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		n, err := Delete(ctx, "key")
		if n != 3 || err != nil {
			t.Errorf("Delete = (%d, %v), want (3, nil)", n, err)
		}
	})

	t.Run("IncrInt", func(t *testing.T) {
		n, err := IncrInt(ctx, "key", 1)
		if n != 11 || err != nil {
			t.Errorf("IncrInt = (%d, %v), want (11, nil)", n, err)
		}
	})

	t.Run("IncrFloat", func(t *testing.T) {
		n, err := IncrFloat(ctx, "key", 1)
		if n != 1.5 || err != nil {
			t.Errorf("IncrFloat = (%v, %v), want (1.5, nil)", n, err)
		}
	})

	t.Run("DecrInt", func(t *testing.T) {
		n, err := DecrInt(ctx, "key", 1)
		if n != 7 || err != nil {
			t.Errorf("DecrInt = (%d, %v), want (7, nil)", n, err)
		}
	})

	t.Run("DecrFloat", func(t *testing.T) {
		n, err := DecrFloat(ctx, "key", 1)
		if n != 0.5 || err != nil {
			t.Errorf("DecrFloat = (%v, %v), want (0.5, nil)", n, err)
		}
	})

	t.Run("AddPrefix", func(t *testing.T) {
		if got := AddPrefix("key"); got != "due:key" {
			t.Errorf("AddPrefix = %q, want %q", got, "due:key")
		}
	})

	t.Run("Client", func(t *testing.T) {
		if got := Client(); got != "client" {
			t.Errorf("Client = %v, want %q", got, "client")
		}
	})

	t.Run("Close", func(t *testing.T) {
		if err := Close(); err != nil {
			t.Errorf("Close error = %v, want nil", err)
		}
		if !stub.closed {
			t.Error("Close must close the installed cache")
		}
	})
}
