package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func TestAuditLogger(t *testing.T) {
	// Create a temporary directory for audit logs
	tempDir := t.TempDir()

	// Create audit logger
	auditLogger := NewAuditLogger(true, tempDir)

	// Test logging an API call
	testRequest := map[string]interface{}{
		"model": "test-model",
		"input": "test input",
	}

	testResponse := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"embedding": []float64{0.1, 0.2, 0.3},
			},
		},
	}

	auditLogger.LogAPICall(
		context.Background(),
		"embeddings",
		"test-model",
		testRequest,
		testResponse,
		nil,
		time.Millisecond*100,
		"test-request-id",
	)

	// Check if the audit log file was created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read audit directory: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No audit log files were created")
	}

	// Check the content of the audit log file
	content, err := os.ReadFile(filepath.Join(tempDir, files[0].Name()))
	if err != nil {
		t.Fatalf("Failed to read audit log file: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "API Call Audit Log") {
		t.Error("Audit log does not contain expected header")
	}
	if !contains(contentStr, "embeddings") {
		t.Error("Audit log does not contain API type")
	}
	if !contains(contentStr, "test-model") {
		t.Error("Audit log does not contain model name")
	}
}

func TestAuditEmbeddingsClient(t *testing.T) {
	// Create a mock OpenAI client
	client := openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL("https://api.openai.com/v1"),
	)

	// Create audit logger
	auditLogger := NewAuditLogger(true, t.TempDir())

	// Create audit embeddings client
	auditClient := NewAuditEmbeddingsClient(&client, auditLogger, "test-model")

	// Test the client interface
	if auditClient == nil {
		t.Fatal("Audit embeddings client is nil")
	}

	// Note: We can't test actual API calls without a real API key,
	// but we can verify the client is properly structured
}

func TestAuditChatClient(t *testing.T) {
	// Create a mock OpenAI client
	client := openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL("https://api.openai.com/v1"),
	)

	// Create audit logger
	auditLogger := NewAuditLogger(true, t.TempDir())

	// Create audit chat client
	auditClient := NewAuditChatClient(&client, auditLogger, "test-model")

	// Test the client interface
	if auditClient == nil {
		t.Fatal("Audit chat client is nil")
	}

	// Test completions interface
	completions := auditClient.Completions()
	if completions == nil {
		t.Fatal("Chat completions client is nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
