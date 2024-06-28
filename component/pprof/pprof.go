package pprof

import (
	"fmt"
	"github.com/dobyte/due/v2/component"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/internal/info"
	"github.com/dobyte/due/v2/log"
	"net/http"
	_ "net/http/pprof"
)

var _ component.Component = &pprof{}

type pprof struct {
	component.Base
	opts *options
}

func NewPProf(opts ...Option) *pprof {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &pprof{opts: o}
}

func (*pprof) Name() string {
	return "pprof"
}

func (p *pprof) Start() {
	listenAddr, exposeAddr, err := xnet.ParseAddr(p.opts.addr)
	if err != nil {
		log.Fatalf("pprof addr listen failed: %v", err)
	}

	go func() {
		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			log.Fatalf("pprof server start failed: %v", err)
		}
	}()

	info.PrintBoxInfo("PProf",
		fmt.Sprintf("Url: http://%s/debug/pprof/", exposeAddr),
	)
}
