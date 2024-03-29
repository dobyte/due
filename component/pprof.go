package component

import (
	"github.com/symsimmy/due/config"
	"github.com/symsimmy/due/log"
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
			// open url: http://addr/debug/pprof/
			log.Infof("pprof server starting at : %v", addr)
			err := http.ListenAndServe(addr, nil)
			if err != nil {
				log.Errorf("pprof server start failed: %v", err)
			}
		}()
	}
}
