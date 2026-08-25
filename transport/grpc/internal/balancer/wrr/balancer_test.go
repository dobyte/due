package wrr

import (
	"testing"

	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
)

type mockSubConn struct {
	balancer.SubConn
}

func TestPickerNoSubConn(t *testing.T) {
	p := &Picker{err: balancer.ErrNoSubConnAvailable}
	if _, err := p.Pick(balancer.PickInfo{}); err != balancer.ErrNoSubConnAvailable {
		t.Fatalf("want ErrNoSubConnAvailable, got %v", err)
	}
}

func TestPickerSingleSubConn(t *testing.T) {
	sc := &mockSubConn{}
	p := &Picker{subConns: []*weightedSubConn{{sc: sc, weight: 5}}}

	for i := 0; i < 100; i++ {
		res, err := p.Pick(balancer.PickInfo{})
		if err != nil {
			t.Fatal(err)
		}
		if res.SubConn != sc {
			t.Fatal("should always pick the only subconn")
		}
	}
}

func TestPickerWeightedDistribution(t *testing.T) {
	subs := []*weightedSubConn{
		{sc: &mockSubConn{}, weight: 3},
		{sc: &mockSubConn{}, weight: 1},
	}
	p := &Picker{subConns: subs}

	const total = 60000
	counts := make(map[balancer.SubConn]int)
	for i := 0; i < total; i++ {
		res, err := p.Pick(balancer.PickInfo{})
		if err != nil {
			t.Fatal(err)
		}
		counts[res.SubConn]++
	}

	// 权重比 3:1，60000 次抽样应接近 45000:15000
	want := total * 3 / 4
	if got := counts[subs[0].sc]; got < want-500 || got > want+500 {
		t.Errorf("weighted distribution mismatch: first=%d want ~%d", got, want)
	}
	if got := counts[subs[1].sc]; got < total/4-500 || got > total/4+500 {
		t.Errorf("weighted distribution mismatch: second=%d want ~%d", got, total/4)
	}
}

func TestPickerConcurrentSafe(t *testing.T) {
	subs := []*weightedSubConn{
		{sc: &mockSubConn{}, weight: 2},
		{sc: &mockSubConn{}, weight: 2},
	}
	p := &Picker{subConns: subs}

	const goroutines = 8
	const picks = 20000
	done := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < picks; j++ {
				if _, err := p.Pick(balancer.PickInfo{}); err != nil {
					t.Errorf("unexpected pick error: %v", err)
					return
				}
			}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func TestGetWeight(t *testing.T) {
	// 无属性时默认权重为1
	if w := getWeight(resolver.Address{}); w != 1 {
		t.Errorf("want default weight 1, got %d", w)
	}

	// 读取解析器附加的权重属性
	addr := resolver.Address{Attributes: attributes.New(WeightAttrKey, uint32(5))}
	if w := getWeight(addr); w != 5 {
		t.Errorf("want weight 5, got %d", w)
	}

	// 权重为0时兜底为1
	addr = resolver.Address{Attributes: attributes.New(WeightAttrKey, uint32(0))}
	if w := getWeight(addr); w != 1 {
		t.Errorf("want weight 1 for zero weight, got %d", w)
	}
}
