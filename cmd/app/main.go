package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

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
		// Раньше здесь был log.Fatal без текста ошибки: невозможно было
		// отличить «файла нет» от «нет прав» и от «битый YAML».
		log.Error().
			Err(err).
			Strs("search_paths", config.SearchPaths()).
			Msg("can't load config")
		os.Exit(1)
	}

	// signal.NotifyContext отменяет контекст по SIGINT/SIGTERM — отсюда
	// начинается graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.New(cfg).Run(ctx); err != nil {
		log.Error().Err(err).Msg("server stopped with error")
		os.Exit(1)
	}

	log.Info().Msg("testapp stopped")
}
