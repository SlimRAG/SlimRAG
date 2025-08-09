package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/cockroachdb/errors"
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
		&cli.IntFlag{Name: "retrieval-limit", Value: 100, Usage: "Number of chunks to retrieve from vector search"},
		&cli.IntFlag{Name: "selected-limit", Value: 10, Usage: "Number of chunks for LLM to select and use for final answer"},
		&cli.BoolFlag{
			Name:    "vector-only",
			Aliases: []string{"vc", "vec"},
			Usage:   "Only return vector search results without LLM processing",
		},
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
		retrievalLimit := command.Int("retrieval-limit")
		selectedLimit := command.Int("selected-limit")
		vectorOnly := command.Bool("vector-only")
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
					return ask(ctx, &r, item.Query, retrievalLimit, selectedLimit, vectorOnly)
				})
			}
			return g.Wait()
		}

		return ask(ctx, &r, query, retrievalLimit, selectedLimit, vectorOnly)
	},
}

type queryItem struct {
	Query string `json:"query"`
}

func ask(ctx context.Context, r *rag.RAG, query string, retrievalLimit int, selectedLimit int, vectorOnly bool) error {
	// 第一阶段：向量检索并显示检索到的块
	retrievedChunks, err := r.QueryDocumentChunks(ctx, query, retrievalLimit)
	if err != nil {
		return err
	}

	fmt.Printf("Retrieved %d chunks from vector search:\n", len(retrievedChunks))
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Chunk ID", "Document", "File Path"})
	for _, chunk := range retrievedChunks {
		tw.AppendRow(table.Row{chunk.ID, chunk.Document, chunk.FilePath})
	}
	fmt.Println(tw.Render())

	// 如果是 vector-only 模式，直接返回
	if vectorOnly {
		return nil
	}

	// 第二阶段：LLM 选择最相关的块
	selectedChunks, err := r.Rerank(ctx, query, retrievedChunks, selectedLimit)
	if err != nil {
		return err
	}

	fmt.Printf("\nLLM selected %d most relevant chunks:\n", len(selectedChunks))
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"Chunk ID", "Document", "File Path"})
	for _, chunk := range selectedChunks {
		tw2.AppendRow(table.Row{chunk.ID, chunk.Document, chunk.FilePath})
	}
	fmt.Println(tw2.Render())

	// 第三阶段：基于选择的块生成答案
	fmt.Println("\nThe answer is:")

	prompt := rag.BuildPrompt(query, selectedChunks)
	c, err := r.AssistantClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: r.AssistantModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return err
	}

	if len(c.Choices) > 0 {
		answer := string(c.Choices[0].Message.Content)
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

	return errors.New("no choices returned from chat completion")
}
