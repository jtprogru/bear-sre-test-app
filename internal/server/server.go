package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/jtprogru/bear-sre-test-app/internal/config"
	"github.com/jtprogru/bear-sre-test-app/internal/upstream"
)

// Server — HTTP-сервер приложения.
type Server struct {
	cfg      *config.Config
	srv      *http.Server
	upstream *upstream.Client

	// ready снимается перед началом остановки, чтобы балансировщик успел
	// увести трафик до того, как сервер перестанет принимать соединения.
	ready atomic.Bool
}

// New собирает сервер со всеми таймаутами. Без них сервис держит соединение
// сколько угодно и ложится от Slowloris (gosec G112).
func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg}
	s.ready.Store(true)

	if cfg.Upstream.Enabled() {
		s.upstream = upstream.New(upstream.Options{
			URL:              cfg.Upstream.URL,
			Timeout:          cfg.Upstream.Timeout,
			MaxAttempts:      cfg.Upstream.MaxAttempts,
			BackoffBase:      cfg.Upstream.BackoffBase,
			FailureThreshold: cfg.Upstream.FailureThreshold,
			OpenFor:          cfg.Upstream.OpenFor,
		})
	}

	// Базовый контекст фиксируем один раз. Раньше здесь переприсваивалась
	// захваченная переменная — гонка данных при нескольких листенерах.
	baseCtx := context.Background()

	s.srv = &http.Server{
		Addr:              cfg.Addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
		BaseContext: func(l net.Listener) context.Context {
			return context.WithValue(baseCtx, keyServerAddr, l.Addr().String())
		},
	}
	return s
}

// Handler отдаёт маршрутизатор — нужен тестам, чтобы поднимать httptest без
// реального листенера.
func (s *Server) Handler() http.Handler { return s.srv.Handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// get регистрирует маршрут дважды: с методом и без него. ServeMux отдаёт
	// предпочтение более специфичному шаблону, поэтому GET уходит в хендлер,
	// а любой другой метод — в 405 с заголовком Allow. Без парного шаблона
	// такой запрос провалился бы в catch-all и получил 404.
	get := func(path string, h http.Handler) {
		mux.Handle("GET "+path, h)
		mux.Handle(path, methodNotAllowed(http.MethodGet))
	}

	// Шаблон "/{$}" матчит только сам "/", а "/" — всё остальное. Благодаря
	// этому неизвестный путь получает честный 404, а не 200 с домашней
	// страницей, как было с прежним catch-all.
	get("/{$}", http.HandlerFunc(s.handleRoot))
	get("/ping", http.HandlerFunc(s.handlePing))
	get("/public", http.HandlerFunc(s.handlePublic))
	get("/secret", http.HandlerFunc(s.handleSecret))
	get("/healthz", http.HandlerFunc(s.handleHealthz))
	get("/readyz", http.HandlerFunc(s.handleReadyz))
	get("/upstream", http.HandlerFunc(s.handleUpstream))
	get("/metrics", promhttp.Handler())
	mux.HandleFunc("/", s.handleNotFound)

	return withRecover(withObservability(mux))
}

// Run поднимает сервер и блокируется до отмены ctx, после чего гасит его
// мягко: перестаёт принимать новые соединения и ждёт завершения текущих.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		log.Info().Str("addr", s.cfg.Addr).Msg("server is starting")
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
	}

	// Сначала снимаем readiness, потом закрываем. Порядок важен: балансировщик
	// должен увидеть 503 на /readyz раньше, чем оборвутся соединения.
	s.ready.Store(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed, forcing close")
		return errors.Join(err, s.srv.Close())
	}

	log.Info().Msg("server stopped gracefully")
	return <-errCh
}
