package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fioepq9/pzlog"
	"github.com/gobwas/glob"
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
		scanCmd,
		computeCmd,
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

var scanCmd = &cli.Command{
	Name:  "scan",
	Usage: "Scan directories for files and upsert them into the database",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "path"},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "glob",
			Aliases: []string{"g"},
			Value:   "*.pdf",
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		path := command.StringArg("path")
		if path == "" {
			return errors.New("path is required")
		}
		g, err := glob.Compile(command.String("glob"))
		if err != nil {
			return err
		}
		return filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !g.Match(d.Name()) {
				return nil
			}
			log.Info().Str("file", path).Msg("Found file")
			return nil
		})
	},
}

var computeCmd = &cli.Command{
	Name:  "compute",
	Usage: "Compute embeddings for files in the database",
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
