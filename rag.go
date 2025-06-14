package main

import (
	"context"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cockroachdb/errors"
	"github.com/gobwas/glob"
	"github.com/goccy/go-json"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	gormzerolog "github.com/vitaliy-art/gorm-zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/fioepq9/pzlog"

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
			Value:   "*.chunks.json",
		},
		&cli.StringFlag{
			Name: "dsn",
			Sources: cli.ValueSourceChain{
				Chain: []cli.ValueSource{
					cli.EnvVar("RAG_DSN"),
				},
			},
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

		logger := gormzerolog.NewGormLogger()
		logger.IgnoreRecordNotFoundError(true)
		logger.LogMode(gormlogger.Warn)
		db, err := gorm.Open(postgres.Open(command.String("dsn")), &gorm.Config{
			Logger: logger,
		})
		if err != nil {
			return err
		}

		err = rag.Migrate(db)
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

			buf, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var chunks rag.Document
			err = json.Unmarshal(buf, &chunks)
			if err != nil {
				return err
			}

			return rag.UpsertDocumentChunks(db, &chunks)
		})
	},
}

var computeCmd = &cli.Command{
	Name:  "compute",
	Usage: "Compute embeddings for files in the database",
	Action: func(ctx context.Context, command *cli.Command) error {
		return nil
	},
}

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
