package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	SpotInterruptions *prometheus.CounterVec
)

func init() {
	SpotInterruptions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace, // e.g. "linea"
			Subsystem: metricSubsystem, // e.g. "controller"
			Name:      "spot_interruptions_total",
			Help:      "Total number of spot reclaim interruptions (SIGUSR1) received by the controller.",
		},
		[]string{"instance_type"},
	)
	prometheus.MustRegister(SpotInterruptions)
}

func IncSpotInterruption(instanceType string) {
	SpotInterruptions.WithLabelValues(instanceType).Inc()
}
