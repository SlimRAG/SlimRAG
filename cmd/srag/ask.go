package main

import (
	"context"
	"fmt"

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
		flagDSN,
		flagEmbeddingBaseURL,
		flagEmbeddingModel,
		flagRerankerBaseURL,
		flagRerankerModel,
		flagAssistantBaseURL,
		flagAssistantModel,
		&cli.IntFlag{Name: "limit", Value: 10},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		query, err := getArgumentQuery(command)
		if err != nil {
			return err
		}

		dsn := command.String("dsn")
		embeddingBaseURL := command.String("embedding-base-url")
		embeddingModel := command.String("embedding-model")
		rerankerBaseURL := command.String("reranker-base-url")
		rerankerModel := command.String("reranker-model")
		limit := command.Int("limit")

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		client := openai.NewClient(option.WithBaseURL(embeddingBaseURL))
		rerankerClient := rag.NewInfinityClient(rerankerBaseURL)
		r := rag.RAG{
			DB:              db,
			EmbeddingClient: &client,
			EmbeddingModel:  embeddingModel,
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

		fmt.Println("The answer is:")
		fmt.Println(answer)
		return nil
	},
}
