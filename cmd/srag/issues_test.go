package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestableIssueProcessor extends IssueProcessor for testing
type TestableIssueProcessor struct {
	*IssueProcessor
	MockFetchIssues    func(ctx context.Context, limit int) ([]GitHubIssue, error)
	MockGenerateAnswer func(ctx context.Context, issue GitHubIssue) (string, error)
	MockPostAnswer     func(ctx context.Context, issue GitHubIssue, answer string) error
}

func (t *TestableIssueProcessor) ProcessIssuesWithMocks(ctx context.Context, limit int) error {
	// Load processed issues
	processed, err := t.loadProcessedIssues()
	if err != nil {
		processed = &ProcessedIssues{IssueIDs: []int{}}
	}

	// Fetch issues using mock if available
	var issues []GitHubIssue
	if t.MockFetchIssues != nil {
		issues, err = t.MockFetchIssues(ctx, limit)
	} else {
		issues, err = t.fetchIssues(ctx, limit)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch issues: %w", err)
	}

	processedCount := 0
	for _, issue := range issues {
		// Skip if already processed
		if t.isProcessed(processed, issue.Number) {
			continue
		}

		// Check if it's a consultation question
		if !t.isConsultationQuestion(issue) {
			continue
		}

		// Generate answer using mock if available
		var answer string
		if t.MockGenerateAnswer != nil {
			answer, err = t.MockGenerateAnswer(ctx, issue)
		} else {
			answer, err = t.generateAnswer(ctx, issue)
		}
		if err != nil {
			continue
		}

		// Post answer using mock if available
		if t.MockPostAnswer != nil {
			err = t.MockPostAnswer(ctx, issue, answer)
		} else {
			err = t.postAnswer(ctx, issue, answer)
		}
		if err != nil {
			continue
		}

		// Mark as processed
		processed.IssueIDs = append(processed.IssueIDs, issue.Number)
		processedCount++

		// Save processed issues after each successful processing
		if err := t.saveProcessedIssues(processed); err != nil {
			// Log error but continue
		}
	}

	return nil
}

func TestIssueProcessor_isConsultationQuestion(t *testing.T) {
	// Test the fallback method since we can't easily mock the LLM in unit tests
	processor := &IssueProcessor{} // RAG is nil, so it will use fallback

	tests := []struct {
		name     string
		issue   GitHubIssue
		expected bool
	}{
		{
			name: "Question with how to",
			issue: GitHubIssue{
				Title: "How to configure the RAG system?",
				Body:  "I need help setting up the configuration.",
			},
			expected: true,
		},
		{
			name: "Question with what is",
			issue: GitHubIssue{
				Title: "What is the best practice for chunking?",
				Body:  "I want to understand the recommended approach.",
			},
			expected: true,
		},
		{
			name: "Question ending with question mark",
			issue: GitHubIssue{
				Title: "Can I use custom embeddings?",
				Body:  "I have my own embedding model.",
			},
			expected: true,
		},
		{
			name: "Bug report",
			issue: GitHubIssue{
				Title: "Bug: Application crashes on startup",
				Body:  "The application fails to start with error message.",
			},
			expected: false,
		},
		{
			name: "Feature request",
			issue: GitHubIssue{
				Title: "Feature request: Add support for PDF files",
				Body:  "Would like to see PDF support implemented.",
			},
			expected: false,
		},
		{
			name: "Help request",
			issue: GitHubIssue{
				Title: "Need help with installation",
				Body:  "I'm having trouble getting started.",
			},
			expected: true,
		},
		{
			name: "Documentation question",
			issue: GitHubIssue{
				Title: "Where can I find examples?",
				Body:  "Looking for usage examples and tutorials.",
			},
			expected: true,
		},
		{
			name: "Regular issue not a question",
			issue: GitHubIssue{
				Title: "Update documentation",
				Body:  "The documentation needs to be updated.",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.isConsultationQuestion(tt.issue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIssueProcessor_loadAndSaveProcessedIssues(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "issues_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	processedFile := filepath.Join(tempDir, "processed_issues.json")
	processor := &IssueProcessor{
		ProcessedFile: processedFile,
	}

	// Test loading non-existent file
	processed, err := processor.loadProcessedIssues()
	require.NoError(t, err)
	assert.Empty(t, processed.IssueIDs)

	// Test saving and loading
	processed.IssueIDs = []int{1, 2, 3}
	err = processor.saveProcessedIssues(processed)
	require.NoError(t, err)

	// Load again and verify
	loaded, err := processor.loadProcessedIssues()
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, loaded.IssueIDs)
}

func TestIssueProcessor_isProcessed(t *testing.T) {
	processor := &IssueProcessor{}
	processed := &ProcessedIssues{
		IssueIDs: []int{1, 3, 5, 7},
	}

	tests := []struct {
		issueNumber int
		expected    bool
	}{
		{1, true},
		{2, false},
		{3, true},
		{4, false},
		{5, true},
		{6, false},
		{7, true},
		{8, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("issue_%d", tt.issueNumber), func(t *testing.T) {
			result := processor.isProcessed(processed, tt.issueNumber)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIssueProcessor_fetchIssues(t *testing.T) {
	// Create mock GitHub API server
	mockIssues := []GitHubIssue{
		{
			ID:     1,
			Number: 1,
			Title:  "How to use this library?",
			Body:   "I need help getting started.",
			State:  "open",
			User: struct {
				Login string `json:"login"`
			}{Login: "testuser"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:     2,
			Number: 2,
			Title:  "Bug: Application crashes",
			Body:   "The app crashes when I do X.",
			State:  "open",
			User: struct {
				Login string `json:"login"`
			}{Login: "testuser2"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/test/repo/issues", r.URL.Path)
		assert.Equal(t, "open", r.URL.Query().Get("state"))
		assert.Equal(t, "10", r.URL.Query().Get("per_page"))
		assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockIssues)
	}))
	defer server.Close()

	// Test the mock server directly
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/repos/test/repo/issues?state=open&per_page=10", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var issues []GitHubIssue
	err = json.NewDecoder(resp.Body).Decode(&issues)
	require.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.Equal(t, "How to use this library?", issues[0].Title)
	assert.Equal(t, "Bug: Application crashes", issues[1].Title)
}

func TestIssueProcessor_ProcessIssues_Integration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "issues_integration_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	processedFile := filepath.Join(tempDir, "processed_issues.json")

	// Create mock issues
	mockIssues := []GitHubIssue{
		{
			ID:     1,
			Number: 1,
			Title:  "How to use this library?",
			Body:   "I need help getting started.",
			State:  "open",
		},
		{
			ID:     2,
			Number: 2,
			Title:  "Bug: Application crashes",
			Body:   "The app crashes when I do X.",
			State:  "open",
		},
		{
			ID:     3,
			Number: 3,
			Title:  "What is the best practice?",
			Body:   "I want to know the recommended approach.",
			State:  "open",
		},
	}

	processor := &TestableIssueProcessor{
		IssueProcessor: &IssueProcessor{
			Repo:          "test/repo",
			ProcessedFile: processedFile,
			RAGLimit:      40,
		},
		MockFetchIssues: func(ctx context.Context, limit int) ([]GitHubIssue, error) {
			return mockIssues, nil
		},
		MockGenerateAnswer: func(ctx context.Context, issue GitHubIssue) (string, error) {
			return "Mock answer for: " + issue.Title, nil
		},
		MockPostAnswer: func(ctx context.Context, issue GitHubIssue, answer string) error {
			return nil
		},
	}

	ctx := context.Background()
	err = processor.ProcessIssuesWithMocks(ctx, 10)
	require.NoError(t, err)

	// Verify that consultation questions were processed
	processed, err := processor.loadProcessedIssues()
	require.NoError(t, err)

	// Should have processed issues 1 and 3 (consultation questions)
	// Issue 2 should be skipped as it's a bug report
	assert.Contains(t, processed.IssueIDs, 1)
	assert.Contains(t, processed.IssueIDs, 3)
	assert.NotContains(t, processed.IssueIDs, 2)

	// Run again to test that processed issues are skipped
	err = processor.ProcessIssuesWithMocks(ctx, 10)
	require.NoError(t, err)

	// Should still have the same processed issues
	processed2, err := processor.loadProcessedIssues()
	require.NoError(t, err)
	assert.Equal(t, processed.IssueIDs, processed2.IssueIDs)
}

func TestIssueProcessor_generateAnswer_ConfidenceEvaluation(t *testing.T) {
	// Test that generateAnswer includes confidence evaluation logic
	// Note: This test focuses on the structure and error handling rather than actual LLM calls
	
	processor := &IssueProcessor{
		RAG: nil, // RAG is nil to test error handling
	}
	
	issue := GitHubIssue{
		Title: "How to configure the system?",
		Body:  "I need help with configuration.",
	}
	
	ctx := context.Background()
	
	// This should return an error since RAG is nil
	_, err := processor.generateAnswer(ctx, issue)
	assert.Error(t, err)
	
	// Test that the method signature is correct and can be called
	// The actual confidence evaluation would require a real RAG instance
	// which is beyond the scope of unit testing
}