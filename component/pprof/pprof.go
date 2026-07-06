package pprof

import (
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
)

var _ component.Component = &PProf{}

type PProf struct {
	component.Base
	opts *options
}

func NewPProf(opts ...Option) *PProf {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &PProf{opts: o}
}

func (*PProf) Name() string {
	return "pprof"
}

func (p *PProf) Start() {
	listenAddr, exposeAddr, err := xnet.ParseAddr(p.opts.addr)
	if err != nil {
		log.Fatalf("pprof addr parse failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	go func() {
		if err := http.ListenAndServe(listenAddr, mux); err != nil {
			log.Fatalf("pprof server start failed: %v", err)
		}
	}()

	info.PrintBoxInfo("PProf",
		fmt.Sprintf("Url: http://%s/debug/pprof/", exposeAddr),
	)
}
