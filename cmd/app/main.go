package main

import (
	"github.com/jtprogru/bear-sre-test-app/internal/config"
	"github.com/jtprogru/bear-sre-test-app/internal/server"
)

func main() {

	cfg := config.New()
	srv := server.New(cfg)
	srv.Start()

}
