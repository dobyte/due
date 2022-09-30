package gate

import (
	"context"
	"errors"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/locator"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/dobyte/due/cluster/internal/code"
	"github.com/dobyte/due/cluster/internal/pb"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/router"
)

var (
	ErrNotFoundUserSource = errors.New("not found user source")
)

type proxy struct {
	gate    *Gate    // 网关服
	sources sync.Map // 用户来源
}

func newProxy(gate *Gate) *proxy {
	return &proxy{gate: gate}
}

// 绑定用户与网关间的关系
func (p *proxy) bindGate(ctx context.Context, uid int64) error {
	err := p.gate.opts.locator.Set(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		return err
	}

	err = p.trigger(ctx, cluster.Reconnect, uid)
	if err != nil && err != ErrNotFoundUserSource && err != router.ErrNotFoundEndpoint {
		log.Errorf("trigger event failed, gid: %s, uid: %d, event: %v, err: %v", p.gate.opts.id, uid, cluster.Reconnect, err)
	}

	return nil
}

// 解绑用户与网关间的关系
func (p *proxy) unbindGate(ctx context.Context, uid int64) error {
	err := p.gate.opts.locator.Rem(ctx, uid, cluster.Gate, p.gate.opts.id)
	if err != nil {
		return err
	}

	err = p.trigger(ctx, cluster.Disconnect, uid)
	if err != nil && err != ErrNotFoundUserSource && err != router.ErrNotFoundEndpoint {
		log.Errorf("trigger event failed, gid: %s, uid: %d, event: %v, err: %v", p.gate.opts.id, uid, cluster.Disconnect, err)
	}

	return nil
}

// 触发事件
func (p *proxy) trigger(ctx context.Context, event cluster.Event, uid int64) error {
	var (
		err    error
		nid    string
		prev   string
		client pb.NodeClient
		ep     *router.Endpoint
	)

	for i := 0; i < 2; i++ {
		if nid, err = p.locateNode(ctx, uid); err != nil {
			return err
		}
		if nid == prev {
			return err
		}
		prev = nid

		if ep, err = p.gate.router.FindNodeEndpoint(nid); err != nil {
			return err
		}

		if client, err = p.newNodeClient(ep); err != nil {
			return err
		}

		_, err = client.Trigger(ctx, &pb.TriggerRequest{
			GID:   p.gate.opts.id,
			UID:   uid,
			Event: int32(event),
		})
		if status.Code(err) == code.NotFoundSession {
			p.sources.Delete(uid)
			continue
		}

		break
	}

	return err
}

// 投递消息
func (p *proxy) deliver(ctx context.Context, cid, uid int64, message *packet.Message) error {
	_, err := p.doNodeRPC(ctx, message.Route, uid, func(ctx context.Context, client pb.NodeClient) (bool, interface{}, error) {
		reply, err := client.Deliver(ctx, &pb.DeliverRequest{
			GID: p.gate.opts.id,
			CID: cid,
			UID: uid,
			Message: &pb.Message{
				Seq:    message.Seq,
				Route:  message.Route,
				Buffer: message.Buffer,
			},
		})
		return status.Code(err) == code.NotFoundSession, reply, err
	})

	return err
}

// 执行RPC调用
func (p *proxy) doNodeRPC(ctx context.Context, route int32, uid int64, fn func(ctx context.Context, client pb.NodeClient) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		nid       string
		prev      string
		client    pb.NodeClient
		entity    *router.Route
		ep        *router.Endpoint
		continued bool
		reply     interface{}
	)

	if entity, err = p.gate.router.FindNodeRoute(route); err != nil {
		return nil, err
	}

	for i := 0; i < 2; i++ {
		if entity.Stateful() {
			if nid, err = p.locateNode(ctx, uid); err != nil {
				return nil, err
			}
			if nid == prev {
				return reply, err
			}
			prev = nid
		}

		if ep, err = entity.FindEndpoint(nid); err != nil {
			return nil, err
		}

		if client, err = p.newNodeClient(ep); err != nil {
			return nil, err
		}

		if continued, reply, err = fn(ctx, client); continued {
			p.sources.Delete(uid)
			continue
		}

		break
	}

	return reply, err
}

// 定位用户所在节点
func (p *proxy) locateNode(ctx context.Context, uid int64) (string, error) {
	if val, ok := p.sources.Load(uid); ok {
		if nid := val.(string); nid != "" {
			return nid, nil
		}
	}

	nid, err := p.gate.opts.locator.Get(ctx, uid, cluster.Node)
	if err != nil {
		return "", err
	}

	if nid == "" {
		return "", ErrNotFoundUserSource
	}

	p.sources.Store(uid, nid)

	return nid, nil
}

// 新建节点RPC客户端
func (p *proxy) newNodeClient(ep *router.Endpoint) (pb.NodeClient, error) {
	conn, err := grpc.Dial(ep.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return pb.NewNodeClient(conn), nil
}

// 监听
func (p *proxy) watch(ctx context.Context) {
	watcher, err := p.gate.opts.locator.Watch(ctx, cluster.Node)
	if err != nil {
		log.Fatalf("user locate event watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
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
				case locator.SetLocation:
					p.sources.Store(event.UID, event.InsID)
				case locator.RemLocation:
					p.sources.Delete(event.UID)
				}
			}
		}
	}()
}
