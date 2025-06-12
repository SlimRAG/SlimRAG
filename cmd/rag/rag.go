package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/fioepq9/pzlog"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var cmd = &cli.Command{
	Name: "rag",
	Commands: []*cli.Command{
		serveCmd,
	},
}

var serveCmd = &cli.Command{
	Name:  "serve",
	Usage: "start rag server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "bind",
			Aliases: []string{"a", "l"},
			Value:   ":5000",
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		e := echo.New()
		rag.RegisterRoutes(e)
		return e.Start(command.StringArg("bind"))
	},
}

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = zerolog.New(pzlog.NewPtermWriter()).With().Timestamp().Caller().Stack().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err := cmd.Run(ctx, os.Args)
	if err != nil {
		log.Error().Err(err).Msg("Unexpected error")
	}
}
