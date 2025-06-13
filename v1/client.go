package rag

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Client struct {
	client *openai.Client
	model  string
}

func NewClient(baseURL string, model string) *Client {
	client := openai.NewClient(option.WithBaseURL(baseURL))
	return &Client{
		client: &client,
		model:  model,
	}
}

func (e *Client) GetEmbedding(ctx context.Context, s string) ([]openai.Embedding, error) {
	rsp, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
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
