package tcp

import (
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/registry"
	"google.golang.org/grpc"
	"sync"
)

type Builder struct {
	err      error
	opts     *Options
	dialOpts []grpc.DialOption
	pools    sync.Map
}

type Options struct {
	Discovery registry.Discovery
}

func NewBuilder() *Builder {
	b := &Builder{}
	return b
}

// Build 构建连接
func (b *Builder) Build(target string) (*Client, error) {
	if b.err != nil {
		return nil, b.err
	}

	val, ok := b.pools.Load(target)
	if ok {
		client := val.(*Client)
		if !val.(*Client).Connected {
			log.Warnf("client[-> %+v] offline.new one tcp client.", client.Target)
		} else {
			return val.(*Client), nil
		}
	}

	client, err := NewClient(target)
	if err != nil {
		return nil, err
	}

	if client != nil {
		client.DisconnectHandler = b.OnClientDisconnected
	}
	b.pools.Store(target, client)

	return client, nil
}

func (b *Builder) OnClientDisconnected(target string) {
	if _, ok := b.pools.Load(target); ok {
		b.pools.Delete(target)
		log.Infof("client disconnected.delete target:%+v from pool.", target)
	}
}

func (b *Builder) Update(endpoints map[string]bool) {
	b.pools.Range(func(key, value any) bool {
		target := key.(string)
		if _, ok := endpoints[target]; !ok {
			log.Infof("watch server node offline.delete target:%+v from pool", target)
			b.pools.Delete(target)
		}
		return true
	})
}
