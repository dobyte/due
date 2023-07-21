package component

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/log"
	"net/http"
	_ "net/http/pprof"
)

var _ Component = &pprof{}

type pprof struct {
	Base
}

func NewPProf() *pprof {
	return &pprof{}
}

func (*pprof) Name() string {
	return "pprof"
}

func (*pprof) Start() {
	if addr := config.Get("config.pprof.addr").String(); addr != "" {
		go func() {
			err := http.ListenAndServe(addr, nil)
			if err != nil {
				log.Errorf("pprof server start failed: %v", err)
			}
		}()
	}
}
