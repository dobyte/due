package wrr

import (
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
)

// WeightAttrKey 服务实例权重属性键，由解析器在地址上附加
const WeightAttrKey = "weight"

var _ balancer.Balancer = &Balancer{}

// Balancer 加权轮询负载均衡器
// 基于平滑加权轮询算法在就绪子连接间按权重分配请求
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

	readyConns := make([]*weightedSubConn, 0, len(b.subConns))
	for sc, addr := range b.subConns {
		if b.scStates[sc] == connectivity.Ready {
			readyConns = append(readyConns, &weightedSubConn{
				sc:     sc,
				weight: getWeight(addr),
			})
		}
	}

	if len(readyConns) == 0 {
		b.picker = &Picker{err: balancer.ErrNoSubConnAvailable}
		return
	}

	b.picker = &Picker{subConns: readyConns}
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

// getWeight 从地址属性中读取服务实例权重，未配置或缺省时默认为1
// @param addr resolver.Address 地址信息
// @return @1 int 权重值
func getWeight(addr resolver.Address) int {
	if addr.Attributes != nil {
		if v, ok := addr.Attributes.Value(WeightAttrKey).(uint32); ok && v > 0 {
			return int(v)
		}
	}
	return 1
}

// weightedSubConn 加权子连接，保存平滑加权轮询所需的权重状态
type weightedSubConn struct {
	sc            balancer.SubConn
	weight        int // 静态权重
	currentWeight int // 平滑加权轮询动态权重
}

// Picker 加权轮询选择器
type Picker struct {
	subConns []*weightedSubConn
	err      error
	mu       sync.Mutex
}

var _ balancer.Picker = &Picker{}

// Pick 选择目标子连接
// 采用平滑加权轮询算法，每次选取当前动态权重最大的子连接
// @param _ balancer.PickInfo 请求信息
// @return @1 balancer.PickResult 选择结果
// @return @2 error 错误信息
func (p *Picker) Pick(_ balancer.PickInfo) (balancer.PickResult, error) {
	if p.err != nil {
		return balancer.PickResult{}, p.err
	}

	if len(p.subConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 平滑加权轮询：每次选取当前动态权重最大的子连接
	var (
		best  *weightedSubConn
		total int
	)
	for _, sc := range p.subConns {
		sc.currentWeight += sc.weight
		total += sc.weight
		if best == nil || sc.currentWeight > best.currentWeight {
			best = sc
		}
	}
	best.currentWeight -= total

	return balancer.PickResult{SubConn: best.sc}, nil
}
