package gate

import (
	"context"
	"errors"
	"fmt"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/cluster/internal/enum"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/dobyte/due/cluster/internal/code"
	"github.com/dobyte/due/cluster/internal/pb"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/packet"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/third/redis"
)

var (
	ErrNotFoundUserSource = errors.New("not found user source")
)

type proxy struct {
	gate    *Gate              // 网关服
	kind    string             // 代理类型
	channel string             // 发布订阅通道
	sources sync.Map           // 用户来源
	sfg     singleflight.Group // singleFlight
}

func newProxy(gate *Gate) *proxy {
	return &proxy{
		gate: gate,
		kind: string(cluster.Gate),
	}
}

// 绑定用户与网关间的关系
func (p *proxy) bindGate(ctx context.Context, uid int64) error {
	key := fmt.Sprintf(enum.UserSourcesKey, uid)
	err := p.gate.opts.redis.HSet(ctx, key, p.kind, p.gate.opts.id).Err()
	if err != nil {
		return err
	}

	p.synchronize(ctx, uid, enum.BindAction)

	err = p.trigger(ctx, cluster.Reconnect, uid)
	if err != nil && err != ErrNotFoundUserSource && err != router.ErrNotFoundEndpoint {
		log.Errorf("trigger event failed, gid: %s, uid: %d, event: %v, err: %v", p.gate.opts.id, uid, cluster.Reconnect, err)
	}

	return nil
}

// 解绑用户与网关间的关系
func (p *proxy) unbindGate(ctx context.Context, uid int64) error {
	key := fmt.Sprintf(enum.UserSourcesKey, uid)
	err := p.gate.opts.redis.HDel(ctx, key, p.kind).Err()
	if err != nil {
		return err
	}

	p.synchronize(ctx, uid, enum.UnbindAction)

	err = p.trigger(ctx, cluster.Disconnect, uid)
	if err != nil && err != ErrNotFoundUserSource && err != router.ErrNotFoundEndpoint {
		log.Errorf("trigger event failed, gid: %s, uid: %d, event: %v, err: %v", p.gate.opts.id, uid, cluster.Disconnect, err)
	}

	return nil
}

// 广播
func (p *proxy) synchronize(ctx context.Context, uid int64, action string) {
	msg := fmt.Sprintf("%d@%s@%s@%s", uid, p.kind, p.gate.opts.id, action)
	channel := fmt.Sprintf(enum.UserSourcesBroadcastKey, cluster.Gate)
	err := p.gate.opts.redis.Publish(ctx, channel, msg).Err()
	if err != nil {
		log.Errorf("the user source broadcast failed: %v", err)
	}
}

// 监听
func (p *proxy) listen(ctx context.Context) {
	channel := fmt.Sprintf(enum.UserSourcesBroadcastKey, string(cluster.Node))
	sub := p.gate.opts.redis.Subscribe(ctx, channel)

	for {
		iface, err := sub.Receive(ctx)
		if err != nil {
			return
		}

		switch v := iface.(type) {
		case *redis.Subscription:
			log.Debugf("channel subscribe succeeded, %s", channel)
		case *redis.Message:
			slice := strings.Split(v.Payload, "@")
			if len(slice) != 4 {
				log.Errorf("invalid broadcast payload, %s", v.Payload)
				continue
			}

			uid, err := strconv.ParseInt(slice[0], 10, 64)
			if err != nil {
				log.Errorf("invalid broadcast payload, %s", v.Payload)
				continue
			}

			switch slice[3] {
			case enum.BindAction:
				p.sources.Store(uid, slice[2])
			case enum.UnbindAction:
				p.sources.Delete(uid)
			}
		case *redis.Pong:
			log.Debugf("channel received pong, %s", channel)
		default:
			// handle error
		}
	}
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
			GID:    p.gate.opts.id,
			CID:    cid,
			UID:    uid,
			Route:  message.Route,
			Buffer: message.Buffer,
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
		if insID := val.(string); insID != "" {
			return insID, nil
		}
	}

	key := fmt.Sprintf(enum.UserSourcesKey, uid)
	val, err, _ := p.sfg.Do(key, func() (interface{}, error) {
		val, err := p.gate.opts.redis.HGet(ctx, key, string(cluster.Node)).Result()
		if err != nil && err != redis.Nil {
			return "", err
		}

		if val == "" {
			return "", ErrNotFoundUserSource
		}

		p.sources.Store(uid, val)

		return val, nil
	})
	if err != nil {
		return "", err
	}

	return val.(string), nil
}

// 新建节点RPC客户端
func (p *proxy) newNodeClient(ep *router.Endpoint) (pb.NodeClient, error) {
	conn, err := grpc.Dial(ep.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return pb.NewNodeClient(conn), nil
}
