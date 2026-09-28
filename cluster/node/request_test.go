package node

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

// newTestRequestNode 创建仅初始化请求对象池的测试节点
// @return @1 *Node 测试节点实例
func newTestRequestNode() *Node {
	n := &Node{}
	n.reqPool = &sync.Pool{New: func() any { return &request{node: n} }}
	return n
}

func TestRequestCloneBytes(t *testing.T) {
	n := newTestRequestNode()

	r := &request{node: n, message: []byte("hello")}
	c := r.Clone().(*request)

	msg, ok := c.message.([]byte)
	if !ok {
		t.Fatalf("expect []byte message, got %T", c.message)
	}
	if string(msg) != "hello" {
		t.Fatalf("expect cloned message %q, got %q", "hello", string(msg))
	}

	// 克隆体必须持有独立切片，修改互不影响
	msg[0] = 'x'
	if r.message.([]byte)[0] == 'x' {
		t.Fatal("clone should own an independent slice")
	}
}

func TestRequestCloneBuffer(t *testing.T) {
	n := newTestRequestNode()

	buf := buffer.NewBytes([]byte("hello"))
	defer buf.Release()

	r := &request{node: n, message: buf}
	c := r.Clone().(*request)

	msg, ok := c.message.([]byte)
	if !ok {
		t.Fatalf("expect []byte message, got %T", c.message)
	}
	if string(msg) != "hello" {
		t.Fatalf("expect cloned message %q, got %q", "hello", string(msg))
	}
}

// countJSONMarshaler 统计序列化次数的测试消息类型
type countJSONMarshaler struct {
	n *atomic.Int32
}

func (c countJSONMarshaler) MarshalJSON() ([]byte, error) {
	c.n.Add(1)
	return []byte(`{"a":1}`), nil
}

func TestRequestCloneMarshalOnce(t *testing.T) {
	n := newTestRequestNode()

	var count atomic.Int32
	r := &request{node: n, message: countJSONMarshaler{n: &count}}

	c1 := r.Clone().(*request)
	c2 := r.Clone().(*request)

	// 底层sonic库单次Marshal可能多次调用MarshalJSON，此处断言第二次克隆命中缓存、序列化次数不再增长
	if got := count.Load(); got == 0 {
		t.Fatal("expect marshal at least once")
	}
	first := count.Load()
	_ = r.Clone()
	if got := count.Load(); got != first {
		t.Fatalf("expect cache reuse, marshal count changed from %d to %d", first, got)
	}

	m1, ok := c1.message.([]byte)
	if !ok || string(m1) != `{"a":1}` {
		t.Fatalf("unexpected cloned message: %v", c1.message)
	}
	m2, ok := c2.message.([]byte)
	if !ok || string(m2) != `{"a":1}` {
		t.Fatalf("unexpected cloned message: %v", c2.message)
	}

	// 克隆体之间持有独立切片，修改互不影响
	m1[0] = 'x'
	if m2[0] == 'x' {
		t.Fatal("clones should own independent slices")
	}
}

func TestRequestReleaseResetCache(t *testing.T) {
	n := newTestRequestNode()

	var count atomic.Int32
	r := &request{node: n, message: countJSONMarshaler{n: &count}}
	_ = r.Clone()

	if r.cache == nil {
		t.Fatal("cache should be filled after clone")
	}

	r.release()

	if r.cache != nil {
		t.Fatal("cache should be reset after release")
	}

	recycled := n.reqPool.Get().(*request)
	if recycled.cache != nil {
		t.Fatal("recycled request should not carry stale cache")
	}
}
