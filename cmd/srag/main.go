package main

import (
	"context"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/fioepq9/pzlog"
)

var cmd = &cli.Command{
	Name:  "SilmRAG",
	Usage: "RAG for minimalists",
	Commands: []*cli.Command{
		generateCmd,
		scanCmd,
		computeCmd,
		cleanupCmd,
		serveCmd,
		askCmd,
		getChunkCmd,
		healthCmd,
	},
}

var trimSpace = cli.StringConfig{TrimSpace: true}

func main() {
	_ = godotenv.Load(".env")

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = zerolog.New(pzlog.NewPtermWriter()).With().Timestamp().Caller().Stack().Logger()

	//ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	//defer stop()

	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		log.Error().Err(err).Msg("Unexpected error")
	}
}
