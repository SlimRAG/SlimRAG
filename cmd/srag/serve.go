package main

import (
	"context"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var serveCmd = &cli.Command{
	Name:  "serve",
	Usage: "Start HTTP server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "bind",
			Aliases: []string{"a", "l"},
			Value:   ":5000",
		},
		flagDSN,
		flagEmbeddingBaseURL,
		flagEmbeddingModel,
		flagEmbeddingDimension,
		flagAssistantBaseURL,
		flagAssistantModel,
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		dsn := command.String("dsn")
		embeddingBaseURL := command.String("embedding-base-url")
		embeddingModel := command.String("embedding-model")
		bind := command.String("bind")

		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}

		client := openai.NewClient(option.WithBaseURL(embeddingBaseURL))
		r := &rag.RAG{DB: db, EmbeddingClient: &client, EmbeddingModel: embeddingModel}

		s := rag.NewServer(r)
		go func() {
			select {
			case <-ctx.Done():
				closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = s.Shutdown(closeCtx)
			}
		}()
		err = s.Start(bind)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	},
}
