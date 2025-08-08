package main

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var computeCmd = &cli.Command{
	Name:  "compute",
	Usage: "Compute embeddings for files in the database",
	Flags: []cli.Flag{
		flagDSN,
		flagEmbeddingBaseURL,
		flagEmbeddingModel,
		&cli.BoolFlag{
			Name:  "force",
			Value: false,
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"j"},
			Value:   3,
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		dsn := command.String("dsn")
		baseURL := command.String("embedding-base-url")
		embeddingModel := command.String("embedding-model")
		force := command.Bool("force")
		workers := command.Int("workers")

		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}

		embeddingClient := openai.NewClient(option.WithBaseURL(baseURL))
		r := rag.RAG{
			DB:              db,
			EmbeddingClient: &embeddingClient,
			EmbeddingModel:  embeddingModel,
		}

		return r.ComputeEmbeddings(ctx, !force, workers)
	},
}
