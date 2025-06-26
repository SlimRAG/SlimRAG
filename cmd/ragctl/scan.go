package main

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gobwas/glob"
	"github.com/goccy/go-json"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var scanCmd = &cli.Command{
	Name:  "scan",
	Usage: "Scan directories for files and upsert them into the database",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "path"},
	},
	Flags: []cli.Flag{
		flagDSN,
		&cli.StringFlag{
			Name:    "glob",
			Aliases: []string{"g"},
			Value:   "*.md.chunks.json",
		},
		&cli.BoolFlag{
			Name: "dry-run",
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		path, err := getArgumentPath(command)
		if err != nil {
			return err
		}
		dsn := command.String("dsn")
		dryRun := command.Bool("dry-run")
		globStr := command.String("glob")

		g, err := glob.Compile(globStr)
		if err != nil {
			return err
		}

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		r := rag.RAG{DB: db}

		pathList := make([]string, 0)
		err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && g.Match(d.Name()) {
				pathList = append(pathList, path)
			}
			return nil
		})

		bar := progressbar.New(len(pathList))
		bar.Describe("Uploading chunks")
		defer func() { _ = bar.Finish() }()

		for _, path := range pathList {
			_ = bar.Add(1)

			buf, err := os.ReadFile(path)
			if err != nil {
				log.Error().Err(err).Stack().Str("path", path).Msg("Read file")
				continue
			}

			decoder := json.NewDecoder(bytes.NewReader(buf))
			decoder.DisallowUnknownFields()
			var chunks rag.Document
			err = decoder.Decode(&chunks)
			if err != nil {
				log.Error().Err(err).Stack().Str("path", path).Msg("Decode")
				continue
			}
			chunks.Fix()

			if dryRun {
				log.Info().Str("path", path).Msg("Skipped chunks uploading due to dry-run")
				continue
			}

			err = r.UpsertDocumentChunks(&chunks)
			if err != nil {
				log.Error().Err(err).Stack().Str("path", path).Msg("Upsert chunks")
			}
		}

		return nil
	},
}
