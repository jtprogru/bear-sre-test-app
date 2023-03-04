package main

import (
	"flag"

	"github.com/jtprogru/bear-sre-test-app/internal/config"
	"github.com/jtprogru/bear-sre-test-app/internal/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	debug := flag.Bool("debug", false, "sets log level to debug")

	flag.Parse()

	// Default level for this example is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Msg("testapp is started...")
	log.Debug().Msg("debug mode is enabled...")
	cfg := config.New()
	srv := server.New(cfg)
	srv.Start()

}
