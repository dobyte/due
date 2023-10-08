package pprof

import (
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"net/http"
	_ "net/http/pprof"
)

var _ component.Component = &pprof{}

type pprof struct {
	component.Base
}

func NewPProf() *pprof {
	return &pprof{}
}

func (*pprof) Name() string {
	return "pprof"
}

func (*pprof) Start() {
	if addr := etc.Get("etc.pprof.addr").String(); addr != "" {
		go func() {
			err := http.ListenAndServe(addr, nil)
			if err != nil {
				log.Errorf("pprof server start failed: %v", err)
			}
		}()
	}
}
