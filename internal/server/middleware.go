package server

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// withLogging пишет структурный лог по каждому запросу.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		log.Info().
			Str("server_addr", serverAddr(r)).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Str("method", r.Method).
			Str("uri", r.RequestURI).
			Dur("elapsed", time.Since(start)).
			Msg("request handled")
	})
}

// serverAddr достаёт адрес листенера из контекста.
func serverAddr(r *http.Request) string {
	v, ok := r.Context().Value(keyServerAddr).(string)
	if !ok {
		return "unknown"
	}
	return v
}
