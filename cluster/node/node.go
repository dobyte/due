package node

import (
	"context"
	"sync"
	"time"

	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/cluster/internal/pb"
	"github.com/dobyte/due/internal/xnet"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/router"

	"github.com/google/uuid"

	"github.com/dobyte/due/encoding/proto"

	"github.com/dobyte/due/component"
	"github.com/dobyte/due/encoding"
	"github.com/dobyte/due/log"
)

const (
	defaultNodeName  = "node"          // 默认节点名称
	defaultCodecName = proto.Name      // 默认编解码器
	defaultTimeout   = 3 * time.Second // 默认超时时间
)

type RouteHandler func(req Request)

type EventHandler func(gid string, uid int64)

type routeEntity struct {
	route    int32
	stateful bool
	handler  RouteHandler
}

type eventEntity struct {
	event cluster.Event
	gid   string
	uid   int64
}

type Node struct {
	component.Base
	opts                *options
	ctx                 context.Context
	cancel              context.CancelFunc
	chEvent             chan *eventEntity
	chRequest           chan *request
	rw                  sync.RWMutex
	routes              map[int32]routeEntity
	defaultRouteHandler RouteHandler
	events              map[cluster.Event]EventHandler
	proxy               *proxy
	router              *router.Router
	instance            *registry.ServiceInstance
}

func NewNode(opts ...Option) *Node {
	o := &options{
		ctx:     context.Background(),
		name:    defaultNodeName,
		codec:   encoding.Invoke(defaultCodecName),
		timeout: defaultTimeout,
	}
	if id, err := uuid.NewUUID(); err == nil {
		o.id = id.String()
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.id == "" {
		log.Fatal("the node instance ID is not registered.")
	}
	if o.redis == nil {
		log.Fatal("the redis client is not registered.")
	}
	if o.grpc == nil {
		log.Fatal("the grpc server is not registered.")
	}
	if o.registry == nil {
		log.Fatal("the registry is not registered.")
	}

	n := &Node{}
	n.opts = o
	n.routes = make(map[int32]routeEntity)
	n.events = make(map[cluster.Event]EventHandler)
	n.chEvent = make(chan *eventEntity, 1024)
	n.chRequest = make(chan *request, 1024)
	n.proxy = newProxy(n)
	n.router = router.NewRouter()
	n.ctx, n.cancel = context.WithCancel(o.ctx)

	return n
}

// Name 组件名称
func (n *Node) Name() string {
	return n.opts.name
}

// Init 初始化节点
func (n *Node) Init() {
	n.buildInstance()
}

// Start 启动节点
func (n *Node) Start() {
	n.startGRPC()

	n.startProxy()

	n.dispatch()

	n.registry()

	n.debugPrint()
}

// Destroy 销毁网关服务器
func (n *Node) Destroy() {
	close(n.chEvent)
	close(n.chRequest)

	n.cancel()

	n.stopGRPC()
}

// Proxy 获取节点代理
func (n *Node) Proxy() Proxy {
	return n.proxy
}

// 分发处理消息
func (n *Node) dispatch() {
	go func() {
		for {
			select {
			case entity, ok := <-n.chEvent:
				if !ok {
					return
				}

				handler, ok := n.events[entity.event]
				if !ok {
					log.Warnf("event does not register handler function, event: %v", entity.event)
					continue
				}

				handler(entity.gid, entity.uid)
			case req, ok := <-n.chRequest:
				if !ok {
					return
				}

				n.rw.RLock()
				route, ok := n.routes[req.route]
				n.rw.RUnlock()

				if ok {
					route.handler(req)
				} else if n.defaultRouteHandler != nil {
					n.defaultRouteHandler(req)
				} else {
					log.Warnf("message routing does not register handler function, route: %v", req.route)
				}
			}
		}
	}()
}

// 启动GRPC服务
func (n *Node) startGRPC() {
	go func() {
		n.opts.grpc.RegisterService(&pb.Node_ServiceDesc, &endpoint{node: n})
		if err := n.opts.grpc.Start(); err != nil {
			log.Fatalf("the grpc server startup failed: %v", err)
		}
	}()
}

// 停止GRPC服务
func (n *Node) stopGRPC() {
	if err := n.opts.registry.Deregister(n.instance); err != nil {
		log.Errorf("the node service instance deregister failed: %v", err)
	}

	if err := n.opts.grpc.Stop(); err != nil {
		log.Errorf("the grpc server stop failed: %v", err)
	}
}

// 启动实例代理
func (n *Node) startProxy() {
	go n.proxy.listen(n.ctx)
}

// 注册服务实例
func (n *Node) registry() {
	if err := n.opts.registry.Register(n.instance); err != nil {
		log.Fatalf("the node service instance register failed: %v", err)
	}

	watcher, err := n.opts.registry.Watch(context.Background(), string(cluster.Gate))
	if err != nil {
		log.Fatalf("the gate service watch failed: %v", err)
	}

	go func() {
		for {
			services, err := watcher.Next()
			if err != nil {
				continue
			}
			n.router.ReplaceServices(services...)
		}
	}()
}

// 构建服务实例
func (n *Node) buildInstance() {
	n.rw.RLock()
	defer n.rw.RUnlock()

	routes := make([]registry.Route, 0, len(n.routes))
	for _, entity := range n.routes {
		routes = append(routes, registry.Route{
			ID:       entity.route,
			Stateful: entity.stateful,
		})
	}

	n.instance = &registry.ServiceInstance{
		ID:       n.opts.id,
		Name:     string(cluster.Node),
		Routes:   routes,
		Endpoint: n.opts.grpc.Endpoint().String(),
	}
}

// 添加路由处理器
func (n *Node) addRouteHandler(route int32, stateful bool, handler RouteHandler) {
	n.rw.Lock()
	defer n.rw.Unlock()

	n.routes[route] = routeEntity{
		route:    route,
		stateful: stateful,
		handler:  handler,
	}
}

// 是否为有状态路由
func (n *Node) isStatefulRoute(route int32) (bool, bool) {
	n.rw.Lock()
	defer n.rw.Unlock()

	if entity, ok := n.routes[route]; ok {
		return entity.stateful, ok
	}

	return false, n.defaultRouteHandler != nil
}

// 添加事件处理器
func (n *Node) addEventListener(event cluster.Event, handler EventHandler) {
	n.events[event] = handler
}

// trigger 触发事件
func (n *Node) trigger(event cluster.Event, gid string, uid int64) {
	n.chEvent <- &eventEntity{
		event: event,
		gid:   gid,
		uid:   uid,
	}
}

// 投递消息给当前节点处理
func (n *Node) deliver(gid, nid string, cid, uid int64, route int32, buffer interface{}) {
	n.chRequest <- &request{
		gid:    gid,
		nid:    nid,
		cid:    cid,
		uid:    uid,
		route:  route,
		buffer: buffer,
		node:   n,
	}
}

func (n *Node) debugPrint() {
	log.Debugf("The node server startup successful")
	log.Debugf("GRPC server, listen: %s protocol: %s", xnet.FulfillAddr(n.opts.grpc.Addr()), n.opts.grpc.Scheme())
}
