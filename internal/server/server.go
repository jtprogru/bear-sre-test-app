package server

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/sirupsen/logrus"
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

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.SetLevel(logrus.DebugLevel)

	log.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
}

func New(cfg Config) *server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", getRoot)
	mux.HandleFunc("/ping", getPing)

	ctx, cancelCtx := context.WithCancel(context.Background())
	log.Info("new server is created")

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
		log.Info("server one closed\n")
	} else if err != nil {
		log.Infof("error listening: %s\n", err)
	}
	s.cancelCtx()
}
