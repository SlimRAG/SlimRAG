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

	// Create a detailed response for audit logging
	var auditResponse interface{}
	if response != nil && len(response.Choices) > 0 {
		// Extract the actual response content
		auditResponse = map[string]interface{}{
			"id":      response.ID,
			"object":  response.Object,
			"created": response.Created,
			"model":   response.Model,
			"choices": []map[string]interface{}{
				{
					"index":         response.Choices[0].Index,
					"finish_reason": response.Choices[0].FinishReason,
					"message": map[string]interface{}{
						"role":    response.Choices[0].Message.Role,
						"content": response.Choices[0].Message.Content,
					},
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     response.Usage.PromptTokens,
				"completion_tokens": response.Usage.CompletionTokens,
				"total_tokens":      response.Usage.TotalTokens,
			},
		}
	} else {
		auditResponse = response
	}

	// Log the call with detailed response
	c.auditLogger.LogAPICall(ctx, "chat", c.model, request, auditResponse, err, duration, "")

	return response, err
}

// sanitizeMessages removes or masks sensitive content from messages for logging
func sanitizeMessages(messages []openai.ChatCompletionMessageParamUnion) []map[string]interface{} {
	sanitized := make([]map[string]interface{}, len(messages))

	for i, msg := range messages {
		sanitizedMsg := map[string]interface{}{
			"role":    "chat message",
			"content": "[Message content sanitized for privacy]",
		}

		// Get the string representation to analyze message structure
		msgStr := fmt.Sprintf("%v", msg)

		// Try to determine role based on field positions in the struct
		// Based on the pattern: {<nil> <nil> content <nil> <nil> <nil> {{<nil>}}}
		fields := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(msgStr, "{", ""), "}", ""))

		// Count non-nil fields to infer content presence
		nonNilCount := 0
		for _, field := range fields {
			if field != "<nil>" && !strings.Contains(field, "0x") {
				nonNilCount++
			}
		}

		// For user messages, sanitize for privacy
		if nonNilCount > 1 {
			sanitizedMsg["content"] = fmt.Sprintf("[User message: %d characters]", len(msgStr))
		} else if nonNilCount == 1 {
			sanitizedMsg["content"] = fmt.Sprintf("[User message: %d characters]", len(msgStr))
		} else {
			sanitizedMsg["content"] = fmt.Sprintf("[User message: empty or placeholder]")
		}

		sanitized[i] = sanitizedMsg
	}

	return sanitized
}
