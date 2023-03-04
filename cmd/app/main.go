package main

import (
	"github.com/jtprogru/bear-sre-test-app/internal/config"
	"github.com/jtprogru/bear-sre-test-app/internal/server"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.SetLevel(logrus.DebugLevel)

	log.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
}

func main() {
	log.Infof("testapp is started...")
	cfg := config.New()
	srv := server.New(cfg)
	srv.Start()

}
