package link

import (
	"context"
	"sync"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/dispatcher"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/registry"
	"golang.org/x/sync/errgroup"
)

type NodeLinker struct {
	ctx        context.Context             // 上下文
	opts       *Options                    // 参数项
	builder    *node.Builder               // 构建器
	dispatcher *dispatcher.Dispatcher      // 分发器
	rw         sync.RWMutex                // 锁
	sources    map[int64]map[string]string // 用户来源节点
}

func NewNodeLinker(ctx context.Context, opts *Options) *NodeLinker {
	l := &NodeLinker{
		ctx:        ctx,
		opts:       opts,
		builder:    node.NewBuilder(&node.Options{InsID: opts.InsID, InsKind: opts.InsKind}),
		dispatcher: dispatcher.NewDispatcher(opts.Dispatch),
		sources:    make(map[int64]map[string]string),
	}

	return l
}

// HasNode 检测是否存在某个节点
func (l *NodeLinker) HasNode(nid string) bool {
	_, err := l.dispatcher.FindEndpoint(nid)
	return err == nil
}

// AskNode 检测用户是否在给定的节点上
func (l *NodeLinker) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	if l.opts.Locator == nil {
		return "", false, errors.ErrNotFoundLocator
	}

	if insID, ok := l.doLoadSource(uid, name); ok {
		return insID, insID == nid, nil
	}

	insID, err := l.opts.Locator.LocateNode(ctx, uid, name)
	if err != nil {
		return "", false, err
	}

	if insID == "" {
		return "", false, errors.ErrNotFoundUserLocation
	}

	l.doSaveSource(uid, name, insID)

	return insID, insID == nid, nil
}

// LocateNode 定位用户所在节点
func (l *NodeLinker) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if l.opts.Locator == nil {
		return "", errors.ErrNotFoundLocator
	}

	nid, ok := l.doLoadSource(uid, name)
	if ok {
		return nid, nil
	}

	nid, err := l.opts.Locator.LocateNode(ctx, uid, name)
	if err != nil {
		return "", err
	}

	if nid == "" {
		return "", errors.ErrNotFoundUserLocation
	}

	l.doSaveSource(uid, name, nid)

	return nid, nil
}

// LocateNodes 定位用户所在节点列表
func (l *NodeLinker) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if l.opts.Locator == nil {
		return nil, errors.ErrNotFoundLocator
	}

	return l.opts.Locator.LocateNodes(ctx, uid)
}

// BindNode 绑定节点
// 单个用户可以绑定到多个节点服务器上，相同名称的节点服务器只能绑定一个，多次绑定会到相同名称的节点服务器会覆盖之前的绑定。
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (l *NodeLinker) BindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.opts.Locator == nil {
		return errors.ErrNotFoundLocator
	}

	if err := l.opts.Locator.BindNode(ctx, uid, name, nid); err != nil {
		return err
	}

	l.doSaveSource(uid, name, nid)

	return nil
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (l *NodeLinker) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.opts.Locator == nil {
		return errors.ErrNotFoundLocator
	}

	err := l.opts.Locator.UnbindNode(ctx, uid, name, nid)
	if err != nil {
		return err
	}

	l.doDeleteSource(uid, name, nid)

	return nil
}

// FetchNodeList 拉取节点列表
func (l *NodeLinker) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	services, err := l.opts.Registry.Services(ctx, cluster.Node.String())
	if err != nil {
		return nil, err
	}

	if len(states) == 0 {
		return services, nil
	}

	mp := make(map[string]struct{}, len(states))
	for _, state := range states {
		mp[state.String()] = struct{}{}
	}

	list := make([]*registry.ServiceInstance, 0, len(services))
	for i := range services {
		if _, ok := mp[services[i].State]; ok {
			list = append(list, services[i])
		}
	}

	return list, nil
}

// Deliver 投递消息给节点处理
func (l *NodeLinker) Deliver(ctx context.Context, args *DeliverArgs) error {
	var (
		err       error
		buf       buffer.Buffer
		isDeliver bool
	)

	switch b := args.Buffer.(type) {
	case []byte:
		buf = buffer.NewNocopyBuffer(b)
	case buffer.Buffer:
		buf = b
	case *Message:
		if buf, err = l.PackMessage(b, false); err != nil {
			return err
		}
	default:
		return errors.ErrInvalidMessage
	}

	if args.NID != "" {
		client, err := l.doBuildClient(args.NID)
		if err != nil {
			buf.Release()
			return err
		}

		return client.Deliver(ctx, args.CID, args.UID, buf)
	} else {
		if _, err = l.doRPC(ctx, args.Route, args.UID, func(ctx context.Context, client *node.Client) (bool, any, error) {
			isDeliver = true

			return false, nil, client.Deliver(ctx, args.CID, args.UID, buf)
		}); err != nil {
			if !isDeliver {
				buf.Release()
			}

			if !errors.Is(err, errors.ErrNotFoundUserLocation) {
				return err
			}
		}

		return nil
	}
}

// Trigger 触发事件
func (l *NodeLinker) Trigger(ctx context.Context, args *TriggerArgs) error {
	event, err := l.dispatcher.FindEvent(int(args.Event))
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	event.VisitEndpoints(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			client, err := l.builder.Build(ep.Address())
			if err != nil {
				return err
			}

			return client.Trigger(ctx, args.Event, args.CID, args.UID)
		})

		return true
	})

	return eg.Wait()
}

// GetState 获取节点状态
func (l *NodeLinker) GetState(ctx context.Context, nid string) (cluster.State, error) {
	client, err := l.doBuildClient(nid)
	if err != nil {
		return cluster.Shut, err
	}

	return client.GetState(ctx)
}

// SetState 设置节点状态
func (l *NodeLinker) SetState(ctx context.Context, nid string, state cluster.State) error {
	client, err := l.doBuildClient(nid)
	if err != nil {
		return err
	}

	return client.SetState(ctx, state)
}

// 执行节点RPC调用
func (l *NodeLinker) doRPC(ctx context.Context, routeID int32, uid int64, fn func(ctx context.Context, client *node.Client) (bool, any, error)) (any, error) {
	var (
		err       error
		nid       string
		prev      string
		route     *dispatcher.Route
		client    *node.Client
		ep        *endpoint.Endpoint
		continued bool
		reply     any
	)

	if route, err = l.dispatcher.FindRoute(routeID); err != nil {
		return nil, err
	}

	if uid == 0 && (route.Stateful() || route.Authorized()) {
		return nil, errors.ErrIllegalRequest
	}

	if l.opts.InsKind == cluster.Gate && route.Internal() {
		return nil, errors.ErrIllegalRequest
	}

	for range 2 {
		if route.Stateful() {
			if nid, err = l.LocateNode(ctx, uid, route.Group()); err != nil {
				return nil, err
			}

			if nid == prev {
				return reply, err
			}

			prev = nid
		}

		if ep, err = route.FindEndpoint(nid); err != nil {
			return nil, err
		}

		if client, err = l.builder.Build(ep.Address()); err != nil {
			return nil, err
		}

		if continued, reply, err = fn(ctx, client); continued {
			if route.Stateful() {
				l.doDeleteSource(uid, route.Group(), prev)
			}
			continue
		}

		break
	}

	return reply, err
}

// 构建节点客户端
func (l *NodeLinker) doBuildClient(nid string) (*node.Client, error) {
	if nid == "" {
		return nil, errors.ErrInvalidNID
	}

	ep, err := l.dispatcher.FindEndpoint(nid)
	if err != nil {
		return nil, err
	}

	return l.builder.Build(ep.Address())
}

// 打包消息
func (l *NodeLinker) PackMessage(message *Message, encrypt bool) (*buffer.NocopyBuffer, error) {
	buffer, err := l.PackBuffer(message.Data, encrypt)
	if err != nil {
		return nil, err
	}

	return packet.PackBuffer(&packet.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: buffer,
	})
}

// 消息转buffer
func (l *NodeLinker) PackBuffer(message any, encrypt bool) ([]byte, error) {
	if message == nil {
		return nil, nil
	}

	if v, ok := message.([]byte); ok {
		return v, nil
	}

	data, err := l.opts.Codec.Marshal(message)
	if err != nil {
		return nil, err
	}

	if encrypt && l.opts.Encryptor != nil {
		return l.opts.Encryptor.Encrypt(data)
	}

	return data, nil
}

// 保存用户节点来源
func (l *NodeLinker) doSaveSource(uid int64, name, nid string) {
	l.rw.Lock()
	defer l.rw.Unlock()

	sources, ok := l.sources[uid]
	if !ok {
		sources = make(map[string]string)
		l.sources[uid] = sources
	}
	sources[name] = nid
}

// 删除用户节点来源
func (l *NodeLinker) doDeleteSource(uid int64, name, nid string) {
	l.rw.Lock()
	defer l.rw.Unlock()

	sources, ok := l.sources[uid]
	if !ok {
		return
	}

	oldNID, ok := sources[name]
	if !ok {
		return
	}

	// ignore mismatched NID
	if oldNID != nid {
		return
	}

	if len(sources) == 1 {
		delete(l.sources, uid)
	} else {
		delete(sources, name)
	}
}

// 加载用户节点来源
func (l *NodeLinker) doLoadSource(uid int64, name string) (string, bool) {
	l.rw.RLock()
	defer l.rw.RUnlock()

	if sources, ok := l.sources[uid]; ok {
		if nid, ok := sources[name]; ok {
			return nid, ok
		}
	}

	return "", false
}

// WatchUserLocate 监听用户定位
func (l *NodeLinker) WatchUserLocate() {
	if l.opts.Locator == nil {
		return
	}

	ctx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	watcher, err := l.opts.Locator.Watch(ctx, cluster.Node.String())
	cancel()
	if err != nil {
		log.Fatalf("user locate event watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-l.ctx.Done():
				return
			default:
				// exec watch
			}

			events, err := watcher.Next()
			if err != nil {
				continue
			}

			for _, event := range events {
				switch event.Type {
				case locate.BindNode:
					l.doSaveSource(event.UID, event.InsName, event.InsID)
				case locate.UnbindNode:
					l.doDeleteSource(event.UID, event.InsName, event.InsID)
				default:
					// ignore
				}
			}
		}
	}()
}

// WatchClusterInstance 监听集群实例
func (l *NodeLinker) WatchClusterInstance() {
	ctx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	watcher, err := l.opts.Registry.Watch(ctx, cluster.Node.String())
	cancel()
	if err != nil {
		log.Fatalf("the cluster instance watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-l.ctx.Done():
				return
			default:
				// exec watch
			}

			services, err := watcher.Next()
			if err != nil {
				continue
			}

			l.dispatcher.ReplaceServices(services...)
		}
	}()
}
