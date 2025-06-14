package rag

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type ClientConfig struct {
	EmbeddingBaseURL string
	EmbeddingModel   string
}

type Client struct {
	embeddingClient *openai.Client
	model           string
}

func NewClient(config ClientConfig) *Client {
	client := openai.NewClient(option.WithBaseURL(config.EmbeddingBaseURL))
	return &Client{
		embeddingClient: &client,
		model:           config.EmbeddingModel,
	}
}

func (e *Client) GetEmbedding(ctx context.Context, s string) ([]openai.Embedding, error) {
	rsp, err := e.embeddingClient.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: e.model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(s),
		},
	})
	if err != nil {
		return nil, err
	}
	return rsp.Data, nil
}
