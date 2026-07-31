package server

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/jtprogru/bear-sre-test-app/internal/config"
)

// Server — HTTP-сервер приложения.
type Server struct {
	cfg       *config.Config
	srv       *http.Server
	cancelCtx context.CancelFunc
}

func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg}

	ctx, cancelCtx := context.WithCancel(context.Background())
	s.cancelCtx = cancelCtx

	s.srv = &http.Server{
		Addr:    cfg.Addr,
		Handler: s.routes(),
		BaseContext: func(l net.Listener) context.Context {
			return context.WithValue(ctx, keyServerAddr, l.Addr().String())
		},
	}
	return s
}

// Handler отдаёт маршрутизатор — нужен тестам, чтобы поднимать httptest без
// реального листенера.
func (s *Server) Handler() http.Handler { return s.srv.Handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	get := func(path string, h http.Handler) {
		mux.Handle("GET "+path, h)
		mux.Handle(path, methodNotAllowed(http.MethodGet))
	}

	get("/{$}", http.HandlerFunc(s.handleRoot))
	get("/ping", http.HandlerFunc(s.handlePing))
	get("/public", http.HandlerFunc(s.handlePublic))
	get("/secret", http.HandlerFunc(s.handleSecret))
	get("/healthz", http.HandlerFunc(s.handleHealthz))
	get("/readyz", http.HandlerFunc(s.handleReadyz))
	mux.HandleFunc("/", s.handleNotFound)

	return withLogging(mux)
}

// Start поднимает сервер и блокируется, пока тот не остановится.
func (s *Server) Start() error {
	log.Info().Str("addr", s.cfg.Addr).Msg("server is starting")

	err := s.srv.ListenAndServe()
	s.cancelCtx()

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
