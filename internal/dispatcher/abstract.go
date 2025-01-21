package dispatcher

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"
)

type serviceEndpoint struct {
	insID    string
	state    string
	endpoint *endpoint.Endpoint
}

type abstract struct {
	counter    atomic.Uint64
	dispatcher *Dispatcher
	endpoints1 []*serviceEndpoint          // 所有端口（包含work、busy、hang、shut状态的实例）
	endpoints2 map[string]*serviceEndpoint // 所有端口（包含work、busy、hang、shut状态的实例）
	endpoints3 []*serviceEndpoint          // 所有端口（包含work、busy状态的实例）
	endpoints4 map[string]*serviceEndpoint // 所有端口（包含work、busy状态的实例）
	// 加权轮询相关字段
	currentQueue *wrrQueue  // 当前队列
	nextQueue    *wrrQueue  // 下一个队列
	step         int        // GCD步长
	wrrMu        sync.Mutex // 加权轮询锁
}

// 加权轮询队列节点
type wrrEntry struct {
	weight    int // 当前权重
	orgWeight int // 原始权重
	endpoint  *serviceEndpoint
	next      *wrrEntry
}

// 加权轮询队列
type wrrQueue struct {
	head *wrrEntry
	tail *wrrEntry
}

// FindEndpoint 查询路由服务端点
func (a *abstract) FindEndpoint(insID ...string) (*endpoint.Endpoint, error) {
	if len(insID) == 0 || insID[0] == "" {
		switch a.dispatcher.strategy {
		case RoundRobin:
			return a.roundRobinDispatch()
		case WeightRoundRobin:
			return a.weightRoundRobinDispatch()
		default:
			return a.randomDispatch()
		}
	}

	return a.directDispatch(insID[0])
}

// IterateEndpoint 迭代服务端口
func (a *abstract) IterateEndpoint(fn func(insID string, ep *endpoint.Endpoint) bool) {
	for _, se := range a.endpoints1 {
		if fn(se.insID, se.endpoint) == false {
			break
		}
	}
}

// 添加服务端点
func (a *abstract) addEndpoint(insID string, state string, endpoint *endpoint.Endpoint) {
	if se, ok := a.endpoints2[insID]; ok {
		se.state = state
		se.endpoint = endpoint
	} else {
		se = &serviceEndpoint{insID: insID, state: state, endpoint: endpoint}
		a.endpoints1 = append(a.endpoints1, se)
		a.endpoints2[insID] = se
	}

	switch state {
	case cluster.Work.String(), cluster.Busy.String():
		if se, ok := a.endpoints4[insID]; ok {
			se.state = state
			se.endpoint = endpoint
		} else {
			se = &serviceEndpoint{insID: insID, state: state, endpoint: endpoint}
			a.endpoints3 = append(a.endpoints3, se)
			a.endpoints4[insID] = se
		}
	case cluster.Hang.String():
		if _, ok := a.endpoints4[insID]; ok {
			delete(a.endpoints4, insID)

			for i, se := range a.endpoints3 {
				if se.insID == insID {
					a.endpoints3 = append(a.endpoints3[:i], a.endpoints3[i+1:]...)
					break
				}
			}
		}
	}
}

// 直接分配
func (a *abstract) directDispatch(insID string) (*endpoint.Endpoint, error) {
	sep, ok := a.endpoints2[insID]
	if !ok {
		return nil, errors.ErrNotFoundEndpoint
	}

	return sep.endpoint, nil
}

// 随机分配
func (a *abstract) randomDispatch() (*endpoint.Endpoint, error) {
	if n := len(a.endpoints3); n > 0 {
		return a.endpoints3[rand.IntN(n)].endpoint, nil
	}

	return nil, errors.ErrNotFoundEndpoint
}

// 轮询分配
func (a *abstract) roundRobinDispatch() (*endpoint.Endpoint, error) {
	if len(a.endpoints3) == 0 {
		return nil, errors.ErrNotFoundEndpoint
	}

	index := int(a.counter.Add(1) % uint64(len(a.endpoints3)))

	return a.endpoints3[index].endpoint, nil
}

// 加权轮询分配
func (a *abstract) weightRoundRobinDispatch() (*endpoint.Endpoint, error) {
	a.wrrMu.Lock()
	defer a.wrrMu.Unlock()

	// 如果当前队列为空，交换当前队列和下一个队列
	if a.currentQueue.isEmpty() {
		a.currentQueue, a.nextQueue = a.nextQueue, a.currentQueue
	}

	// 从当前队列中取出一个节点
	entry := a.currentQueue.pop()
	if entry == nil {
		return nil, errors.ErrNotFoundEndpoint
	}

	// 减少当前权重
	entry.weight -= a.step

	// 如果权重大于0，放回当前队列
	if entry.weight > 0 {
		a.currentQueue.push(entry)
	} else {
		// 重置权重并放入下一个队列
		entry.weight = entry.orgWeight
		a.nextQueue.push(entry)
	}

	return entry.endpoint.endpoint, nil
}

// 初始化 WRR 队列
func (a *abstract) initWRRQueue() {
	a.currentQueue = &wrrQueue{}
	a.nextQueue = &wrrQueue{}

	// 计算最大公约数作为步长
	a.step = 0
	for _, sep := range a.endpoints4 {
		weight := a.dispatcher.instances[sep.insID].Weight
		if a.step == 0 {
			a.step = weight
		} else {
			a.step = gcd(a.step, weight)
		}

		// 创建队列节点
		entry := &wrrEntry{
			weight:    weight,
			orgWeight: weight,
			endpoint:  sep,
		}
		a.currentQueue.push(entry)
	}
}

// 判断队列是否为空
func (q *wrrQueue) isEmpty() bool {
	return q.head == nil
}

// 将节点加入队列尾部
func (q *wrrQueue) push(entry *wrrEntry) {
	entry.next = nil

	if q.tail == nil {
		// 空队列
		q.head = entry
		q.tail = entry
		return
	}

	// 添加到队列尾部
	q.tail.next = entry
	q.tail = entry
}

// 从队列头部取出节点
func (q *wrrQueue) pop() *wrrEntry {
	if q.head == nil {
		return nil
	}

	entry := q.head
	q.head = entry.next

	if q.head == nil {
		// 队列已空
		q.tail = nil
	}

	entry.next = nil
	return entry
}

// 计算最大公约数
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
