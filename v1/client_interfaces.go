package rag

import (
	"context"

	"github.com/openai/openai-go"
)

// EmbeddingClientInterface defines the interface for embedding clients
type EmbeddingClientInterface interface {
	New(ctx context.Context, params openai.EmbeddingNewParams) (*openai.CreateEmbeddingResponse, error)
}

// ChatClientInterface defines the interface for chat clients
type ChatClientInterface interface {
	Completions() ChatCompletionsInterface
}

// ChatCompletionsInterface defines the interface for chat completions
type ChatCompletionsInterface interface {
	New(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error)
}

// OriginalOpenAIEmbeddingClient wraps the original OpenAI embedding client
type OriginalOpenAIEmbeddingClient struct {
	embeddings openai.EmbeddingService
}

func (c *OriginalOpenAIEmbeddingClient) New(ctx context.Context, params openai.EmbeddingNewParams) (*openai.CreateEmbeddingResponse, error) {
	return c.embeddings.New(ctx, params)
}

// OriginalOpenAIChatClient wraps the original OpenAI chat client
type OriginalOpenAIChatClient struct {
	chat openai.ChatService
}

func (c *OriginalOpenAIChatClient) Completions() ChatCompletionsInterface {
	return &OriginalOpenAIChatCompletionsClient{completions: c.chat.Completions}
}

// OriginalOpenAIChatCompletionsClient wraps the original OpenAI chat completions client
type OriginalOpenAIChatCompletionsClient struct {
	completions openai.ChatCompletionService
}

func (c *OriginalOpenAIChatCompletionsClient) New(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	return c.completions.New(ctx, params)
}

// ToEmbeddingClient converts an *openai.Client to EmbeddingClientInterface
func ToEmbeddingClient(client interface{}) EmbeddingClientInterface {
	switch c := client.(type) {
	case *openai.Client:
		return &OriginalOpenAIEmbeddingClient{embeddings: c.Embeddings}
	case *AuditEmbeddingsClient:
		return c
	case EmbeddingClientInterface:
		return c
	default:
		// Fallback - try to cast to *openai.Client
		if openaiClient, ok := client.(*openai.Client); ok {
			return &OriginalOpenAIEmbeddingClient{embeddings: openaiClient.Embeddings}
		}
		return nil
	}
}

// ToChatClient converts an *openai.Client to ChatClientInterface
func ToChatClient(client interface{}) ChatClientInterface {
	switch c := client.(type) {
	case *openai.Client:
		return &OriginalOpenAIChatClient{chat: c.Chat}
	case *AuditChatClient:
		return c
	case ChatClientInterface:
		return c
	default:
		// Fallback - try to cast to *openai.Client
		if openaiClient, ok := client.(*openai.Client); ok {
			return &OriginalOpenAIChatClient{chat: openaiClient.Chat}
		}
		return nil
	}
}
