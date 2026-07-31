package server

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/jtprogru/bear-sre-test-app/internal/metrics"
)

// statusRecorder запоминает код ответа: сам http.ResponseWriter его не отдаёт,
// а без кода не собрать ни лог, ни метрику.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.written {
		return // защита от повторного WriteHeader
	}
	r.status = code
	r.written = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

// withObservability пишет структурный лог и RED-метрики по каждому запросу.
func withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		metrics.RequestsInFlight.Inc()
		defer metrics.RequestsInFlight.Dec()

		next.ServeHTTP(rec, r)

		route := routeLabel(r)
		elapsed := time.Since(start)

		metrics.RequestDuration.WithLabelValues(r.Method, route).Observe(elapsed.Seconds())
		metrics.RequestsTotal.WithLabelValues(r.Method, route, statusText(rec.status)).Inc()

		log.Info().
			Str("server_addr", serverAddr(r)).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Str("method", r.Method).
			Str("route", route).
			Str("uri", r.RequestURI).
			Int("status", rec.status).
			Dur("elapsed", elapsed).
			Msg("request handled")
	})
}

// withRecover не даёт панике в хендлере уронить обработку и превращает её
// в честный 500. Без него паника рвёт соединение без ответа и без метрики.
func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Interface("panic", rec).
					Str("uri", r.RequestURI).
					Msg("recovered from panic in handler")
				writeJSON(w, http.StatusInternalServerError, errorResponse{Msg: "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// routeLabel возвращает шаблон маршрута, а не сырой путь: лейбл из URI
// раздувает кардинальность метрик на первом же сканере портов.
func routeLabel(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return "unmatched"
}

func statusText(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}

// serverAddr достаёт адрес листенера из контекста. Ключа может не быть —
// например, когда хендлер вызван из httptest в обход BaseContext, поэтому
// приведение типа с проверкой, а не голое .(string).
func serverAddr(r *http.Request) string {
	v, ok := r.Context().Value(keyServerAddr).(string)
	if !ok {
		return "unknown"
	}
	return v
}
