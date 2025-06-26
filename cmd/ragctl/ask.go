package main

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var askCmd = &cli.Command{
	Name:  "ask",
	Usage: "Search documents and ask the LLM",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "query", Config: trimSpace},
	},
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
		&cli.StringFlag{
			Name:    "reranker_base_url",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RERANKER_BASE_URL")),
		},
		&cli.StringFlag{
			Name:    "reranker_model",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RERANKER_MODEL")),
		},
		&cli.IntFlag{
			Name:  "limit",
			Value: 10,
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		query := command.StringArg("query")
		if query == "" {
			return errors.New("query is required")
		}

		baseURL := command.String("base_url")
		model := command.String("model")
		rerankerBaseURL := command.String("reranker_base_url")
		rerankerModel := command.String("reranker_model")
		dsn := command.String("dsn")
		limit := command.Int("limit")

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		client := openai.NewClient(option.WithBaseURL(baseURL))
		rerankerClient := rag.NewInfinityClient(rerankerBaseURL)
		r := rag.RAG{
			DB:              db,
			EmbeddingClient: &client,
			EmbeddingModel:  model,
			RerankerClient:  rerankerClient,
			RerankerModel:   rerankerModel,
		}

		chunks, err := r.QueryDocumentChunks(ctx, query, limit)
		if err != nil {
			return err
		}

		chunks, err = r.Rerank(query, chunks, limit)
		if err != nil {
			return err
		}

		tw := table.NewWriter()
		tw.AppendHeader(table.Row{"ID", "Raw document"})
		for _, chunk := range chunks {
			tw.AppendRow(table.Row{chunk.ID, chunk.Document})
		}
		fmt.Println(tw.Render())

		answer, err := r.Ask(ctx, query, chunks)
		if err != nil {
			return err
		}
		fmt.Println(answer)
		return nil
	},
}
