// Package metrics содержит RED-метрики приложения.
//
// Регистрация идёт в дефолтный registry prometheus, поэтому вместе с
// прикладными метриками наружу уезжают go_* и process_* коллекторы. Это не
// косметика: process_open_fds — единственный способ увидеть утечку файловых
// дескрипторов, не заходя на хост.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal — счётчик запросов. Лейбл path берётся из шаблона маршрута,
	// а не из сырого URI, иначе кардинальность взлетает на первом же сканере.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests by route, method and status code.",
	}, []string{"method", "path", "code"})

	// RequestDuration — гистограмма латентности, из неё считается p99.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency by route and method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// RequestsInFlight показывает, сколько запросов обрабатывается прямо сейчас.
	// По нему видно, дренирует ли сервис соединения при graceful shutdown.
	RequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})

	// UpstreamRequestsTotal — исходы обращений к внешнему сервису.
	// result: success | failure | circuit_open.
	UpstreamRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "upstream_requests_total",
		Help: "Total number of upstream calls by outcome.",
	}, []string{"result"})

	// UpstreamAttemptsTotal считает попытки, включая ретраи.
	UpstreamAttemptsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "upstream_attempts_total",
		Help: "Total number of upstream HTTP attempts including retries.",
	})

	// UpstreamDuration — длительность всего вызова апстрима вместе с ретраями.
	UpstreamDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "upstream_request_duration_seconds",
		Help:    "Upstream call latency including retries.",
		Buckets: prometheus.DefBuckets,
	})

	// CircuitBreakerState: 0 — closed, 1 — half-open, 2 — open.
	CircuitBreakerState = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "upstream_circuit_breaker_state",
		Help: "Circuit breaker state: 0=closed, 1=half-open, 2=open.",
	})
)
