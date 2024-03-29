package prommetrics

import (
	"context"
	"github.com/symsimmy/due/config"
)

const (
	defaultAddr    = ":8664"
	defaultHandler = "/metrics"
	defaultEnable  = false
)

const (
	defaultAddrKey    = "config.metrics.prometheus.addr"
	defaultHandlerKey = "config.metrics.prometheus.handler"
	defaultEnableKey  = "config.metrics.prometheus.enable"
)

type Option func(o *options)

type options struct {
	ctx context.Context

	addr        string
	handlerPath string
	enable      bool
}

func defaultOptions() *options {
	return &options{
		ctx:         context.Background(),
		addr:        config.Get(defaultAddrKey, defaultAddr).String(),
		handlerPath: config.Get(defaultHandlerKey, defaultHandler).String(),
		enable:      config.Get(defaultEnableKey, defaultEnable).Bool(),
	}
}
