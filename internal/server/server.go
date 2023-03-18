package server

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config interface {
	Addr() string
}

type server struct {
	cancelCtx context.CancelFunc
	srv       *http.Server
}

func New(cfg Config) *server {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	mux := http.NewServeMux()
	mux.HandleFunc("/", getRoot)
	mux.HandleFunc("/ping", getPing)
	mux.HandleFunc("/public", getPublic)
	mux.HandleFunc("/secret", getSecret)

	ctx, cancelCtx := context.WithCancel(context.Background())
	log.Info().Msg("new server is created")

	return &server{
		cancelCtx: cancelCtx,
		srv: &http.Server{
			Addr:    cfg.Addr(),
			Handler: mux,
			BaseContext: func(l net.Listener) context.Context {
				ctx = context.WithValue(ctx, keyServerAddr, l.Addr().String())
				return ctx
			},
		},
	}
}

func (s *server) Start() {
	err := s.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		log.Info().Msg("server one closed\n")
	} else if err != nil {
		log.Error().AnErr("err", err).Msg("error listening")
	}
	s.cancelCtx()
}
