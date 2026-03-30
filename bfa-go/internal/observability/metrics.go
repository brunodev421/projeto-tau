package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	RequestLatency        *prometheus.HistogramVec
	RequestErrors         *prometheus.CounterVec
	DownstreamCalls       *prometheus.CounterVec
	RetryCount            *prometheus.CounterVec
	CircuitBreakerEvents  *prometheus.CounterVec
	CacheLookups          *prometheus.CounterVec
	ConcurrencySaturation *prometheus.CounterVec
}

func NewMetrics(registry *prometheus.Registry) *Metrics {
	m := &Metrics{
		RequestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bfa_http_request_latency_seconds",
			Help:    "Latency by route and status code.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "status"}),
		RequestErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_http_request_errors_total",
			Help: "HTTP errors by route and error code.",
		}, []string{"route", "error_code"}),
		DownstreamCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_downstream_calls_total",
			Help: "Downstream calls by dependency and outcome.",
		}, []string{"dependency", "outcome"}),
		RetryCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_retry_attempts_total",
			Help: "Retry attempts by dependency.",
		}, []string{"dependency"}),
		CircuitBreakerEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_circuit_breaker_events_total",
			Help: "Circuit breaker state changes by dependency and state.",
		}, []string{"dependency", "state"}),
		CacheLookups: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_cache_lookups_total",
			Help: "Cache hits and misses by cache name.",
		}, []string{"cache", "result"}),
		ConcurrencySaturation: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_bulkhead_saturation_total",
			Help: "Bulkhead saturation events by dependency.",
		}, []string{"dependency"}),
	}

	registry.MustRegister(
		m.RequestLatency,
		m.RequestErrors,
		m.DownstreamCalls,
		m.RetryCount,
		m.CircuitBreakerEvents,
		m.CacheLookups,
		m.ConcurrencySaturation,
	)

	return m
}

func Handler(registry *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func (m *Metrics) ObserveRequest(route string, status int, duration time.Duration) {
	m.RequestLatency.WithLabelValues(route, strconv.Itoa(status)).Observe(duration.Seconds())
}
