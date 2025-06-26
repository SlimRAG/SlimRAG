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
		&cli.StringFlag{
			Name:    "dsn",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_DSN")),
		},
		&cli.StringFlag{
			Name:    "base_url",
			Sources: cli.NewValueSourceChain(cli.EnvVar("EMBEDDING_BASE_URL")),
		},
		&cli.StringFlag{
			Name:    "model",
			Sources: cli.NewValueSourceChain(cli.EnvVar("EMBEDDING_MODEL")),
		},
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
		baseURL := command.String("base_url")
		model := command.String("model")
		dsn := command.String("dsn")
		force := command.Bool("force")
		workers := command.Int("workers")

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		client := openai.NewClient(option.WithBaseURL(baseURL))
		r := rag.RAG{DB: db, EmbeddingClient: &client, EmbeddingModel: model}

		return r.ComputeEmbeddings(ctx, !force, workers)
	},
}
