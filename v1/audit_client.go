package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
)

// AuditEmbeddingsClient wraps OpenAI embeddings client with audit logging
type AuditEmbeddingsClient struct {
	embeddings  openai.EmbeddingService
	auditLogger *AuditLogger
	model       string
}

func NewAuditEmbeddingsClient(client *openai.Client, auditLogger *AuditLogger, model string) *AuditEmbeddingsClient {
	return &AuditEmbeddingsClient{
		embeddings:  client.Embeddings,
		auditLogger: auditLogger,
		model:       model,
	}
}

func (c *AuditEmbeddingsClient) New(ctx context.Context, params openai.EmbeddingNewParams) (*openai.CreateEmbeddingResponse, error) {
	start := time.Now()

	// Log request
	request := map[string]interface{}{
		"model":           params.Model,
		"input":           params.Input,
		"dimensions":      params.Dimensions,
		"encoding_format": params.EncodingFormat,
	}

	// Make the API call
	response, err := c.embeddings.New(ctx, params)
	duration := time.Since(start)

	// Log the call
	c.auditLogger.LogAPICall(ctx, "embeddings", c.model, request, response, err, duration, "")

	return response, err
}

// AuditChatClient wraps OpenAI chat client with audit logging
type AuditChatClient struct {
	chat        openai.ChatService
	auditLogger *AuditLogger
	model       string
}

func NewAuditChatClient(client *openai.Client, auditLogger *AuditLogger, model string) *AuditChatClient {
	return &AuditChatClient{
		chat:        client.Chat,
		auditLogger: auditLogger,
		model:       model,
	}
}

func (c *AuditChatClient) Completions() ChatCompletionsInterface {
	return &AuditChatCompletionsClient{
		completions: c.chat.Completions,
		auditLogger: c.auditLogger,
		model:       c.model,
	}
}

// AuditChatCompletionsClient wraps OpenAI chat completions client with audit logging
type AuditChatCompletionsClient struct {
	completions openai.ChatCompletionService
	auditLogger *AuditLogger
	model       string
}

func (c *AuditChatCompletionsClient) New(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	start := time.Now()

	// Log request (sanitize messages for privacy)
	request := map[string]interface{}{
		"model":             params.Model,
		"messages":          sanitizeMessages(params.Messages),
		"temperature":       params.Temperature,
		"max_tokens":        params.MaxTokens,
		"top_p":             params.TopP,
		"frequency_penalty": params.FrequencyPenalty,
		"presence_penalty":  params.PresencePenalty,
	}

	// Make the API call
	response, err := c.completions.New(ctx, params)
	duration := time.Since(start)

	// Log the call
	c.auditLogger.LogAPICall(ctx, "chat", c.model, request, response, err, duration, "")

	return response, err
}

// sanitizeMessages removes or masks sensitive content from messages for logging
func sanitizeMessages(messages []openai.ChatCompletionMessageParamUnion) []map[string]interface{} {
	sanitized := make([]map[string]interface{}, len(messages))

	for i, msg := range messages {
		sanitizedMsg := map[string]interface{}{
			"role": "unknown",
		}

		// Use string representation for logging to avoid type complexity
		msgStr := fmt.Sprintf("%v", msg)
		sanitizedMsg["content"] = fmt.Sprintf("[Message type: %T, Length: %d chars]", msg, len(msgStr))

		// Try to extract role information
		if strings.Contains(msgStr, "user") {
			sanitizedMsg["role"] = "user"
		} else if strings.Contains(msgStr, "assistant") {
			sanitizedMsg["role"] = "assistant"
		} else if strings.Contains(msgStr, "system") {
			sanitizedMsg["role"] = "system"
		}

		sanitized[i] = sanitizedMsg
	}

	return sanitized
}
