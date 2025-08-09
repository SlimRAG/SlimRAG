package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	Usage: "Search documents and ask the LLM (supports direct query or file input)",
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
		&cli.IntFlag{Name: "retrieval-limit", Value: 40, Usage: "Number of chunks to retrieve from vector search"},
		&cli.IntFlag{Name: "selected-limit", Value: 10, Usage: "Number of chunks for LLM to select and use for final answer"},
		&cli.BoolFlag{
			Name:    "vector-only",
			Aliases: []string{"vc", "vec"},
			Usage:   "Only return vector search results without LLM processing",
		},
		&cli.StringFlag{
			Name:    "system-prompt",
			Aliases: []string{"sp"},
			Usage:   "Custom system prompt file path",
		},
		&cli.StringFlag{
			Name:    "system-text",
			Aliases: []string{"st"},
			Usage:   "Custom system prompt text (overrides --system-prompt)",
		},
		&cli.BoolFlag{
			Name:  "trace",
			Usage: "Enable API call tracing and audit logging",
		},
		&cli.StringFlag{
			Name:  "audit-log-dir",
			Usage: "Directory for audit log files (default: ./audit_logs)",
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
		systemPromptFile := command.String("system-prompt")
		systemPromptText := command.String("system-text")
		traceEnabled := command.Bool("trace")
		auditLogDir := command.String("audit-log-dir")
		jobs := command.Int("jobs")

		// Handle system prompt
		var systemPrompt string
		if systemPromptText != "" {
			systemPrompt = systemPromptText
		} else if systemPromptFile != "" {
			content, err := os.ReadFile(systemPromptFile)
			if err != nil {
				return fmt.Errorf("failed to read system prompt file: %w", err)
			}
			systemPrompt = string(content)
		}

		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}

		// Create audit logger if trace is enabled
		auditLogger := rag.NewAuditLogger(traceEnabled, auditLogDir)

		// Create OpenAI clients
		embeddingClient := openai.NewClient(option.WithBaseURL(embeddingBaseURL))
		assistantClient := openai.NewClient(option.WithBaseURL(assistantBaseURL), option.WithAPIKey(assistantAPIKey))

		// Wrap clients with audit logging if enabled
		var embeddingClientInterface interface{} = &embeddingClient
		var assistantClientInterface interface{} = &assistantClient

		if traceEnabled {
			embeddingClientInterface = rag.NewAuditEmbeddingsClient(&embeddingClient, auditLogger, embeddingModel)
			assistantClientInterface = rag.NewAuditChatClient(&assistantClient, auditLogger, assistantModel)
		}

		r := rag.RAG{
			DB:              db,
			EmbeddingClient: embeddingClientInterface,
			EmbeddingModel:  embeddingModel,
			AssistantClient: assistantClientInterface,
			AssistantModel:  assistantModel,
		}

		// Check if query is a file path
		if _, err := os.Stat(query); err == nil {
			return processQueryFile(ctx, &r, query, retrievalLimit, selectedLimit, vectorOnly, systemPrompt, jobs)
		}

		return ask(ctx, &r, query, retrievalLimit, selectedLimit, vectorOnly, systemPrompt)
	},
}

type queryItem struct {
	Query string `json:"query"`
}

func ask(ctx context.Context, r *rag.RAG, query string, retrievalLimit int, selectedLimit int, vectorOnly bool, systemPrompt string) error {
	// Phase 1: Vector retrieval and display retrieved chunks
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

	// If vector-only mode, return directly
	if vectorOnly {
		return nil
	}

	// Phase 2: LLM selects the most relevant chunks
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

	// Phase 3: Generate answer based on selected chunks
	fmt.Println("\nThe answer is:")

	// Use the RAG's Ask method which handles the client interface properly
	answer, err := r.Ask(ctx, &rag.AskParameter{
		Query:          query,
		RetrievalLimit: retrievalLimit,
		SelectedLimit:  selectedLimit,
		SystemPrompt:   systemPrompt,
	})
	if err != nil {
		return err
	}

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

// processQueryFile handles reading queries from different file formats
func processQueryFile(ctx context.Context, r *rag.RAG, filePath string, retrievalLimit int, selectedLimit int, vectorOnly bool, systemPrompt string, jobs int) error {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".ndjson", ".jsonl":
		return processNdjsonFile(ctx, r, filePath, retrievalLimit, selectedLimit, vectorOnly, systemPrompt, jobs)
	case ".txt":
		return processTextFile(ctx, r, filePath, retrievalLimit, selectedLimit, vectorOnly, systemPrompt)
	default:
		return fmt.Errorf("unsupported file format: %s. Supported formats: .ndjson, .jsonl, .txt", ext)
	}
}

// processNdjsonFile processes NDJSON files with query items
func processNdjsonFile(ctx context.Context, r *rag.RAG, filePath string, retrievalLimit int, selectedLimit int, vectorOnly bool, systemPrompt string, jobs int) error {
	f, err := os.Open(filePath)
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
			return ask(ctx, r, item.Query, retrievalLimit, selectedLimit, vectorOnly, systemPrompt)
		})
	}
	return g.Wait()
}

// processTextFile processes plain text files with one query per line
func processTextFile(ctx context.Context, r *rag.RAG, filePath string, retrievalLimit int, selectedLimit int, vectorOnly bool, systemPrompt string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	queryCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		queryCount++
		fmt.Printf("Processing query %d: %s\n", queryCount, line)

		err := ask(ctx, r, line, retrievalLimit, selectedLimit, vectorOnly, systemPrompt)
		if err != nil {
			fmt.Printf("Error processing query '%s': %v\n", line, err)
			continue
		}

		fmt.Println("---\n")
	}

	if queryCount == 0 {
		fmt.Println("No queries found in the file")
	}

	return nil
}
