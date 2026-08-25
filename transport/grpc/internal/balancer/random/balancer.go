package random

import (
	"math/rand"
	"sync"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

var _ balancer.Balancer = &Balancer{}

// Balancer 随机负载均衡器
// 从就绪子连接中随机选取一个处理请求
type Balancer struct {
	cc       balancer.ClientConn
	opts     balancer.BuildOptions
	subConns map[balancer.SubConn]resolver.Address
	scStates map[balancer.SubConn]connectivity.State
	cse      balancer.ConnectivityStateEvaluator
	picker   balancer.Picker
	mu       sync.Mutex
}

// UpdateClientConnState 更新客户端连接状态
// 同步解析器下发的地址列表，移除失效子连接并新建缺失子连接
// @param s balancer.ClientConnState 客户端连接状态
// @return @1 error 错误信息
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

	readyConns := make([]balancer.SubConn, 0, len(b.subConns))
	for sc, state := range b.scStates {
		if state == connectivity.Ready {
			readyConns = append(readyConns, sc)
		}
	}

	if len(readyConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	b.picker = &Picker{
		subConns: readyConns,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ResolverError 处理解析器错误
// 将错误透传给选择器并更新连接状态为瞬态失败
// @param err error 解析器错误
func (b *Balancer) ResolverError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.picker = &Picker{err: err}
	b.cc.UpdateState(balancer.State{
		ConnectivityState: connectivity.TransientFailure,
		Picker:            b.picker,
	})
}

// UpdateSubConnState 更新子连接状态
// 空闲连接触发重新连接，关闭连接从映射中移除，并同步更新选择器
// @param sc balancer.SubConn 子连接
// @param state balancer.SubConnState 子连接状态
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

// Close 关闭负载均衡器，关闭全部子连接并清空状态
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

// ExitIdle 退出空闲状态，触发全部空闲子连接建立连接
func (b *Balancer) ExitIdle() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sc, state := range b.scStates {
		if state == connectivity.Idle {
			sc.Connect()
		}
	}
}

// Picker 随机选择器，从子连接列表中随机选取一个
type Picker struct {
	subConns []balancer.SubConn
	rng      *rand.Rand
	mu       sync.Mutex
	err      error
}

var _ balancer.Picker = &Picker{}

// Pick 选择目标子连接
// 单个子连接时直接返回，多个子连接时随机选取
// @param info balancer.PickInfo 请求信息
// @return @1 balancer.PickResult 选择结果
// @return @2 error 错误信息
func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if p.err != nil {
		return balancer.PickResult{}, p.err
	}

	if len(p.subConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	if len(p.subConns) == 1 {
		return balancer.PickResult{SubConn: p.subConns[0]}, nil
	}

	p.mu.Lock()
	idx := p.rng.Intn(len(p.subConns))
	p.mu.Unlock()

	return balancer.PickResult{SubConn: p.subConns[idx]}, nil
}
