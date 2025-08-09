package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/fioepq9/pzlog"
)

var cmd = &cli.Command{
	Name:  "SlimRAG",
	Usage: "RAG for minimalists",
	Commands: []*cli.Command{
		serveCmd,
		askCmd,
		getChunkCmd,
		healthCmd,
		chunkCmd,
		updateCmd,
		issueBotCmd,
		botCmd,
	},
}

var trimSpace = cli.StringConfig{TrimSpace: true}

func main() {
	_ = godotenv.Load(".env")

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = zerolog.New(pzlog.NewPtermWriter()).With().Timestamp().Caller().Stack().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err := cmd.Run(ctx, os.Args)
	if err != nil {
		log.Error().Err(err).Msg("Unexpected error")
	}
}
