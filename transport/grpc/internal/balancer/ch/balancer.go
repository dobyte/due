package ch

import (
	"context"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

// virtualNodes 每个子连接在哈希环上的虚拟节点数
const virtualNodes = 150

var _ balancer.Balancer = &Balancer{}

type Balancer struct {
	cc       balancer.ClientConn
	opts     balancer.BuildOptions
	subConns map[balancer.SubConn]resolver.Address
	scStates map[balancer.SubConn]connectivity.State
	cse      balancer.ConnectivityStateEvaluator
	picker   balancer.Picker
	mu       sync.Mutex
}

func (b *Balancer) UpdateClientConnState(s balancer.ClientConnState) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	addrsSet := make(map[string]struct{})
	for _, addr := range s.ResolverState.Addresses {
		addrsSet[addr.Addr] = struct{}{}
	}

	for sc, addr := range b.subConns {
		if _, ok := addrsSet[addr.Addr]; !ok {
			b.cse.RecordTransition(b.scStates[sc], connectivity.Shutdown)
			sc.Shutdown()
			delete(b.subConns, sc)
			delete(b.scStates, sc)
		}
	}

	for _, addr := range s.ResolverState.Addresses {
		exists := false
		for _, existingAddr := range b.subConns {
			if existingAddr.Addr == addr.Addr {
				exists = true
				break
			}
		}
		if exists {
			continue
		}

		sc, err := b.cc.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{})
		if err != nil {
			continue
		}

		b.subConns[sc] = addr
		b.scStates[sc] = connectivity.Idle
		b.cse.RecordTransition(connectivity.Shutdown, connectivity.Idle)
		sc.Connect()
	}

	b.updatePicker()

	b.cc.UpdateState(balancer.State{
		ConnectivityState: b.cse.CurrentState(),
		Picker:            b.picker,
	})

	return nil
}

func (b *Balancer) updatePicker() {
	if len(b.subConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	readyConns := make([]*hashSubConn, 0, len(b.subConns))
	for sc, addr := range b.subConns {
		if b.scStates[sc] == connectivity.Ready {
			readyConns = append(readyConns, &hashSubConn{sc: sc, key: addr.Addr})
		}
	}

	if len(readyConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	b.picker = &Picker{ring: newConsistentRing(readyConns)}
}

func (b *Balancer) ResolverError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.picker = &Picker{err: err}
	b.cc.UpdateState(balancer.State{
		ConnectivityState: connectivity.TransientFailure,
		Picker:            b.picker,
	})
}

func (b *Balancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subConns[sc]; !ok {
		return
	}

	oldState := b.scStates[sc]
	b.scStates[sc] = state.ConnectivityState

	switch state.ConnectivityState {
	case connectivity.Idle:
		sc.Connect()
	case connectivity.Shutdown:
		delete(b.subConns, sc)
		delete(b.scStates, sc)
	}

	if oldState != state.ConnectivityState {
		b.cse.RecordTransition(oldState, state.ConnectivityState)
	}

	b.updatePicker()

	b.cc.UpdateState(balancer.State{
		ConnectivityState: b.cse.CurrentState(),
		Picker:            b.picker,
	})
}

func (b *Balancer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sc := range b.subConns {
		sc.Shutdown()
	}

	b.subConns = nil
	b.scStates = nil
	b.picker = nil
}

func (b *Balancer) ExitIdle() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sc, state := range b.scStates {
		if state == connectivity.Idle {
			sc.Connect()
		}
	}
}

// hashSubConn 待哈希的子连接
type hashSubConn struct {
	sc  balancer.SubConn
	key string // 哈希节点标识（服务地址）
}

// ringNode 哈希环节点
type ringNode struct {
	hash uint32
	sc   balancer.SubConn
}

// consistentRing 一致性哈希环，按哈希值升序排列
type consistentRing struct {
	nodes []ringNode
}

// hashKey 计算字符串哈希值，FNV-1a 具有更好的分布特性
func hashKey(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

// newConsistentRing 构建哈希环，每个子连接生成若干虚拟节点以均衡分布
func newConsistentRing(subConns []*hashSubConn) *consistentRing {
	nodes := make([]ringNode, 0, len(subConns)*virtualNodes)
	for _, sc := range subConns {
		for i := 0; i < virtualNodes; i++ {
			key := sc.key + "#" + strconv.Itoa(i)
			nodes = append(nodes, ringNode{hash: hashKey(key), sc: sc.sc})
		}
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].hash < nodes[j].hash })

	return &consistentRing{nodes: nodes}
}

// get 根据哈希键返回子连接
func (r *consistentRing) get(key string) (balancer.SubConn, bool) {
	if len(r.nodes) == 0 {
		return nil, false
	}

	idx := sort.Search(len(r.nodes), func(i int) bool { return r.nodes[i].hash >= hashKey(key) })
	if idx == len(r.nodes) {
		idx = 0
	}

	return r.nodes[idx].sc, true
}

// hashKeyCtxKey 一致性哈希键上下文键
type hashKeyCtxKey struct{}

// WithHashKey 将一致性哈希键注入上下文，未设置时默认使用 RPC 方法名作为哈希键
func WithHashKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, hashKeyCtxKey{}, key)
}

func getHashKey(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if key, ok := ctx.Value(hashKeyCtxKey{}).(string); ok {
		return key
	}

	return ""
}

type Picker struct {
	ring *consistentRing
	err  error
}

var _ balancer.Picker = &Picker{}

func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if p.err != nil {
		return balancer.PickResult{}, p.err
	}

	key := getHashKey(info.Ctx)
	if key == "" {
		key = info.FullMethodName
	}

	if sc, ok := p.ring.get(key); ok {
		return balancer.PickResult{SubConn: sc}, nil
	}

	return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
}
