package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/goccy/go-json"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

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
		flagAssistantBaseURL,
		flagAssistantModel,
		flagAssistantAPIKey,
		&cli.IntFlag{Name: "limit", Value: 40},
		&cli.IntFlag{Name: "top-n", Value: 10},
		&cli.IntFlag{Name: "jobs", Value: 4},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		query, err := getArgumentQuery(command)
		if err != nil {
			return err
		}

		dsn := command.String("dsn")
		embeddingBaseURL := command.String("embedding-base-url")
		embeddingModel := command.String("embedding-model")
		assistantBaseURL := command.String("assistant-base-url")
		assistantModel := command.String("assistant-model")
		assistantAPIKey := command.String("assistant-api-key")
		limit := command.Int("limit")
		topN := command.Int("top-n")
		jobs := command.Int("jobs")

		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}

		embeddingClient := openai.NewClient(option.WithBaseURL(embeddingBaseURL))
		assistantClient := openai.NewClient(option.WithBaseURL(assistantBaseURL), option.WithAPIKey(assistantAPIKey))
		r := rag.RAG{
			DB:              db,
			EmbeddingClient: &embeddingClient,
			EmbeddingModel:  embeddingModel,
			AssistantClient: &assistantClient,
			AssistantModel:  assistantModel,
		}

		if strings.HasSuffix(query, ".ndjson") {
			f, err := os.Open(query)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			g, ctx := errgroup.WithContext(ctx)
			g.SetLimit(jobs)

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				g.Go(func() error {
					var item queryItem
					err = json.Unmarshal([]byte(line), &item)
					if err != nil {
						return err
					}
					return ask(ctx, &r, item.Query, limit, topN)
				})
			}
			return g.Wait()
		}

		return ask(ctx, &r, query, limit, topN)
	},
}

type queryItem struct {
	Query string `json:"query"`
}

func ask(ctx context.Context, r *rag.RAG, query string, limit int, topN int) error {
	chunks, err := r.QueryDocumentChunks(ctx, query, limit)
	if err != nil {
		return err
	}

	chunks, err = r.Rerank(ctx, query, chunks, topN)
	if err != nil {
		return err
	}

	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Chunk ID", "Document", "File Path"})
	for _, chunk := range chunks {
		tw.AppendRow(table.Row{chunk.ID, chunk.Document, chunk.FilePath})
	}
	fmt.Println(tw.Render())

	answer, err := r.Ask(ctx, &rag.AskParameter{Query: query, Limit: limit})
	if err != nil {
		return err
	}

	fmt.Println("The answer is:")

	// Use glamour to render markdown
	rendered, err := glamour.Render(answer, "dark")
	if err != nil {
		fmt.Printf("Error rendering markdown: %v\n", err)
		fmt.Println(answer)
		return nil
	}
	fmt.Println(rendered)
	return nil
}
