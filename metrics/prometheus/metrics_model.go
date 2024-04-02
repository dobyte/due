package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	GateServerTotalOnlinePlayerGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: GateServerTotalOnlinePlayer,
		},
		[]string{"gate_instance_id"},
	)
)
