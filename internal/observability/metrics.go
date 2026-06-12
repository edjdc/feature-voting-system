package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		RequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests partitioned by method, path, and status.",
		}, []string{"method", "path", "status"}),
		RequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		RequestsInFlight: factory.NewGauge(prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of in-flight HTTP requests.",
		}),
	}
}
