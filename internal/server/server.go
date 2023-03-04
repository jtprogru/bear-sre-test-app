package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

type Config interface {
	Addr() string
}

type Generator interface {
	Gen() string
}

type server struct {
	cancelCtx context.CancelFunc
	srv       *http.Server
}

type str string

const keyServerAddr str = "serverAddr"

func New(cfg Config) *server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", getRoot)
	mux.HandleFunc("/ping", getPing)
	ctx, cancelCtx := context.WithCancel(context.Background())
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
		fmt.Printf("server one closed\n")
	} else if err != nil {
		fmt.Printf("error listening for server one: %s\n", err)
	}
	s.cancelCtx()
}
