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
)
