package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/jtprogru/bear-sre-test-app/internal/config"
	"github.com/jtprogru/bear-sre-test-app/internal/server"
)

func main() {
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Msg("testapp is starting...")

	cfg, err := config.New()
	if err != nil {
		log.Error().Msg("can't load config")
		os.Exit(1)
	}

	if err := server.New(cfg).Start(); err != nil {
		log.Error().Err(err).Msg("server stopped with error")
		os.Exit(1)
	}
}
