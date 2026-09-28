package node

import (
	"context"
	"sync"

	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"golang.org/x/sync/singleflight"
)

type ClientOptions = drpc.ClientOptions

type Builder struct {
	sfg     singleflight.Group
	opts    *ClientOptions
	clients sync.Map
}

func NewBuilder(opts *ClientOptions) *Builder {
	return &Builder{
		opts: opts,
	}
}

// Build 构建客户端
func (b *Builder) Build(addr string) (*Client, error) {
	return b.BuildContext(context.Background(), addr)
}

// BuildContext 构建客户端
// 拨号建立连接受限于上下文超时，避免对不可达端点无限重试
// @param ctx context.Context 上下文
// @param addr string 节点地址
// @return @1 *Client 客户端实例
// @return @2 error 错误信息
func (b *Builder) BuildContext(ctx context.Context, addr string) (*Client, error) {
	if cli, ok := b.clients.Load(addr); ok {
		return cli.(*Client), nil
	}

	cli, err, _ := b.sfg.Do(addr, func() (any, error) {
		if cli, ok := b.clients.Load(addr); ok {
			return cli.(*Client), nil
		}

		c, err := drpc.NewClient(addr, b.opts)
		if err != nil {
			return nil, err
		}

		if err = c.Establish(ctx); err != nil {
			_ = c.Close()

			return nil, err
		}

		cli := NewClient(c)

		b.clients.Store(addr, cli)

		return cli, nil
	})
	if err != nil {
		return nil, err
	}

	return cli.(*Client), nil
}
