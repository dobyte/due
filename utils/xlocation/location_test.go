package xlocation

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/location"
	"github.com/dobyte/due/v2/errors"
)

// mockResolver 用于测试的解析器实现
type mockResolver struct {
	name   string
	result *location.Result
	err    error
	delay  time.Duration
}

// Name 获取解析器名称
func (m *mockResolver) Name() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}

// Resolve 解析 IP 地址
func (m *mockResolver) Resolve(ctx context.Context, ip string) (*location.Result, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.result, m.err
}

func TestParse(t *testing.T) {
	want := &location.Result{
		IP:       "1.2.3.4",
		Country:  "中国",
		Province: "广东省",
		City:     "深圳市",
		ISP:      "电信",
	}
	globalLocation = location.NewLocation(&mockResolver{result: want})

	got, err := Parse(context.Background(), "1.2.3.4")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got == nil || *got != *want {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseNoResolvers(t *testing.T) {
	globalLocation = location.NewLocation()

	got, err := Parse(context.Background(), "1.2.3.4")
	if !errors.Is(err, errors.ErrNotFoundIPAddress) {
		t.Fatalf("Parse() error = %v, want %v", err, errors.ErrNotFoundIPAddress)
	}
	if got != nil {
		t.Fatalf("Parse() = %#v, want nil", got)
	}
}

func TestParseAllResolversFail(t *testing.T) {
	globalLocation = location.NewLocation(
		&mockResolver{err: errors.New("resolver1 failed")},
		&mockResolver{err: errors.New("resolver2 failed")},
	)

	got, err := Parse(context.Background(), "1.2.3.4")
	if !errors.Is(err, errors.ErrNotFoundIPAddress) {
		t.Fatalf("Parse() error = %v, want %v", err, errors.ErrNotFoundIPAddress)
	}
	if got != nil {
		t.Fatalf("Parse() = %#v, want nil", got)
	}
}

func TestParseFirstSuccessWins(t *testing.T) {
	fast := &location.Result{IP: "1.1.1.1"}
	slow := &location.Result{IP: "2.2.2.2"}

	globalLocation = location.NewLocation(
		&mockResolver{result: fast},
		&mockResolver{result: slow, delay: 100 * time.Millisecond},
	)

	got, err := Parse(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got == nil || got.IP != fast.IP {
		t.Fatalf("Parse() IP = %v, want %v", got, fast.IP)
	}
}

func TestParseContextCanceled(t *testing.T) {
	globalLocation = location.NewLocation(
		&mockResolver{result: &location.Result{IP: "1.2.3.4"}},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := Parse(ctx, "1.2.3.4")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse() error = %v, want %v", err, context.Canceled)
	}
	if got != nil {
		t.Fatalf("Parse() = %#v, want nil", got)
	}
}
