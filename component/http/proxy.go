package http

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/transport"
)

type Proxy struct {
	http *Http
}

func newProxy(h *Http) *Proxy {
	return &Proxy{http: h}
}

// Router 获取路由器
func (p *Proxy) Router() Router {
	return &router{app: p.http.app}
}

// NewMeshClient 新建微服务客户端
// target参数可分为两种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	if p.http.opts.transporter == nil {
		return nil, errors.ErrMissTransporter
	}

	return p.http.opts.transporter.NewClient(target)
}
