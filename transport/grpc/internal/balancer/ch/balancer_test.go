package ch

import (
	"context"
	"strconv"
	"testing"

	"google.golang.org/grpc/balancer"
)

type mockSubConn struct {
	balancer.SubConn
}

func TestPickerError(t *testing.T) {
	p := &Picker{err: balancer.ErrNoSubConnAvailable}
	if _, err := p.Pick(balancer.PickInfo{}); err != balancer.ErrNoSubConnAvailable {
		t.Fatalf("want ErrNoSubConnAvailable, got %v", err)
	}
}

func TestPickerSingleSubConn(t *testing.T) {
	sc := &mockSubConn{}
	p := &Picker{ring: newConsistentRing([]*hashSubConn{{sc: sc, key: "127.0.0.1:8011"}})}

	for _, method := range []string{"/a.Service/Foo", "/b.Service/Bar", "/c.Service/Baz"} {
		for i := 0; i < 10; i++ {
			res, err := p.Pick(balancer.PickInfo{FullMethodName: method})
			if err != nil {
				t.Fatal(err)
			}
			if res.SubConn != sc {
				t.Fatal("should always pick the only subconn")
			}
		}
	}
}

func TestPickerDeterministic(t *testing.T) {
	subs := []*hashSubConn{
		{sc: &mockSubConn{}, key: "127.0.0.1:8011"},
		{sc: &mockSubConn{}, key: "127.0.0.1:8012"},
		{sc: &mockSubConn{}, key: "127.0.0.1:8013"},
	}
	p := &Picker{ring: newConsistentRing(subs)}

	// 同一哈希键多次 Pick 结果一致（粘性路由）
	for _, method := range []string{"/a.Service/Foo", "/b.Service/Bar", "/c.Service/Baz", "/d.Service/Qux"} {
		first, err := p.Pick(balancer.PickInfo{FullMethodName: method})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100; i++ {
			res, err := p.Pick(balancer.PickInfo{FullMethodName: method})
			if err != nil {
				t.Fatal(err)
			}
			if res.SubConn != first.SubConn {
				t.Fatalf("method %s not sticky: got different subconn", method)
			}
		}
	}
}

func TestPickerContextKey(t *testing.T) {
	subs := []*hashSubConn{
		{sc: &mockSubConn{}, key: "127.0.0.1:8011"},
		{sc: &mockSubConn{}, key: "127.0.0.1:8012"},
	}
	p := &Picker{ring: newConsistentRing(subs)}

	// 同一哈希键在不同方法下路由到同一节点
	ctx := WithHashKey(context.Background(), "user-1001")
	var first balancer.SubConn
	for _, method := range []string{"/a.Service/Foo", "/b.Service/Bar", "/c.Service/Baz"} {
		res, err := p.Pick(balancer.PickInfo{Ctx: ctx, FullMethodName: method})
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = res.SubConn
		} else if res.SubConn != first {
			t.Fatalf("hash key user-1001 routed to different subconns across methods")
		}
	}
}

func TestPickerDistribution(t *testing.T) {
	subs := []*hashSubConn{
		{sc: &mockSubConn{}, key: "127.0.0.1:8011"},
		{sc: &mockSubConn{}, key: "127.0.0.1:8012"},
		{sc: &mockSubConn{}, key: "127.0.0.1:8013"},
	}
	p := &Picker{ring: newConsistentRing(subs)}

	// 大量不同键应覆盖全部节点
	counts := make(map[balancer.SubConn]int)
	for i := 0; i < 3000; i++ {
		res, err := p.Pick(balancer.PickInfo{FullMethodName: "/svc.Method" + strconv.Itoa(i)})
		if err != nil {
			t.Fatal(err)
		}
		counts[res.SubConn]++
	}

	if len(counts) != len(subs) {
		t.Errorf("not all subconns received requests: %v", counts)
	}
}

func TestPickerRebalance(t *testing.T) {
	a, b, c := &mockSubConn{}, &mockSubConn{}, &mockSubConn{}
	subs := []*hashSubConn{
		{sc: a, key: "127.0.0.1:8011"},
		{sc: b, key: "127.0.0.1:8012"},
		{sc: c, key: "127.0.0.1:8013"},
	}
	p := &Picker{ring: newConsistentRing(subs)}

	// 记录一批键的初始路由
	keys := make([]string, 0, 1000)
	for i := 0; i < 1000; i++ {
		keys = append(keys, "key-"+strconv.Itoa(i))
	}
	initial := make(map[string]balancer.SubConn)
	for _, k := range keys {
		res, err := p.Pick(balancer.PickInfo{FullMethodName: k})
		if err != nil {
			t.Fatal(err)
		}
		initial[k] = res.SubConn
	}

	// 移除节点 c 后重建环，仅受影响键迁移
	p2 := &Picker{ring: newConsistentRing(subs[:2])}
	moved := 0
	for _, k := range keys {
		res, err := p2.Pick(balancer.PickInfo{FullMethodName: k})
		if err != nil {
			t.Fatal(err)
		}
		if res.SubConn != initial[k] {
			moved++
		}
	}

	// 移除 1/3 节点，理论迁移约 1/3 的键
	if moved < 200 || moved > 450 {
		t.Errorf("rebalance ratio unexpected: moved=%d/1000", moved)
	}
}

func TestGetHashKey(t *testing.T) {
	if key := getHashKey(context.Background()); key != "" {
		t.Errorf("want empty key for nil ctx, got %q", key)
	}

	ctx := WithHashKey(context.Background(), "user-2002")
	if key := getHashKey(ctx); key != "user-2002" {
		t.Errorf("want user-2002, got %q", key)
	}
}
