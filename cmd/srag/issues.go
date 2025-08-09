package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var issueBotCmd = &cli.Command{
	Name:  "issue-bot",
	Usage: "Scan GitHub issues and answer consultation questions using RAG",
	Flags: []cli.Flag{
		flagDSN,
		flagEmbeddingBaseURL,
		flagEmbeddingModel,
		flagAssistantBaseURL,
		flagAssistantModel,
		&cli.StringFlag{
			Name:     "repo",
			Usage:    "GitHub repository in format 'owner/repo'",
			Required: true,
		},
		&cli.StringFlag{
			Name:    "token",
			Usage:   "GitHub access token (optional, uses GITHUB_TOKEN env var if not provided)",
			Sources: cli.NewValueSourceChain(cli.EnvVar("GITHUB_TOKEN")),
		},
		&cli.StringFlag{
			Name:  "processed-file",
			Usage: "File to store processed issue IDs",
			Value: "processed_issues.json",
		},
		&cli.IntFlag{
			Name:  "limit",
			Usage: "Maximum number of issues to process",
			Value: 10,
		},
		&cli.IntFlag{
			Name:  "rag-limit",
			Usage: "Number of document chunks to retrieve for RAG",
			Value: 20,
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		repo := command.String("repo")
		token := command.String("token")
		processedFile := command.String("processed-file")
		limit := command.Int("limit")
		ragLimit := command.Int("rag-limit")

		dsn := command.String("dsn")
		embeddingBaseURL := command.String("embedding-base-url")
		embeddingModel := command.String("embedding-model")
		assistantBaseURL := command.String("assistant-base-url")
		assistantModel := command.String("assistant-model")

		// Initialize RAG
		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		embeddingClient := openai.NewClient(option.WithBaseURL(embeddingBaseURL))
		assistantClient := openai.NewClient(option.WithBaseURL(assistantBaseURL))

		r := &rag.RAG{
			DB:              db,
			EmbeddingClient: &embeddingClient,
			EmbeddingModel:  embeddingModel,
			AssistantClient: &assistantClient,
			AssistantModel:  assistantModel,
		}

		// Initialize issue processor
		processor := &IssueProcessor{
			RAG:           r,
			Repo:          repo,
			Token:         token,
			ProcessedFile: processedFile,
			RAGLimit:      ragLimit,
		}

		return processor.ProcessIssues(ctx, limit)
	},
}

// GitHubIssue represents a GitHub issue
type GitHubIssue struct {
	ID     int    `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProcessedIssues stores the IDs of issues that have been processed
type ProcessedIssues struct {
	IssueIDs []int `json:"issue_ids"`
}

// IssueProcessor handles GitHub issue processing
type IssueProcessor struct {
	RAG           *rag.RAG
	Repo          string
	Token         string
	ProcessedFile string
	RAGLimit      int
}

// ProcessIssues scans and processes GitHub issues
func (p *IssueProcessor) ProcessIssues(ctx context.Context, limit int) error {
	// Load processed issues
	processed, err := p.loadProcessedIssues()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load processed issues, starting fresh")
		processed = &ProcessedIssues{IssueIDs: []int{}}
	}

	// Fetch open issues from GitHub
	issues, err := p.fetchIssues(ctx, limit)
	if err != nil {
		return fmt.Errorf("failed to fetch issues: %w", err)
	}

	log.Info().Int("total_issues", len(issues)).Msg("Fetched issues from GitHub")

	processedCount := 0
	for _, issue := range issues {
		// Skip if already processed
		if p.isProcessed(processed, issue.Number) {
			log.Debug().Int("issue_number", issue.Number).Msg("Issue already processed, skipping")
			continue
		}

		// Check if it's a consultation question
		if !p.isConsultationQuestion(issue) {
			log.Info().Int("issue_number", issue.Number).Msg("Not a consultation question, skipping")
			continue
		}

		log.Info().Int("issue_number", issue.Number).Str("title", issue.Title).Msg("Processing consultation question")

		// Generate answer using RAG
		answer, err := p.generateAnswer(ctx, issue)
		if err != nil {
			log.Error().Err(err).Int("issue_number", issue.Number).Msg("Failed to generate answer")
			continue
		}

		// Post answer as comment (simulated for now)
		err = p.postAnswer(ctx, issue, answer)
		if err != nil {
			log.Error().Err(err).Int("issue_number", issue.Number).Msg("Failed to post answer")
			continue
		}

		// Mark as processed
		processed.IssueIDs = append(processed.IssueIDs, issue.Number)
		processedCount++

		// Save processed issues after each successful processing
		if err := p.saveProcessedIssues(processed); err != nil {
			log.Error().Err(err).Msg("Failed to save processed issues")
		}
	}

	log.Info().Int("processed_count", processedCount).Msg("Issue processing completed")
	return nil
}

// fetchIssues retrieves issues from GitHub API
func (p *IssueProcessor) fetchIssues(ctx context.Context, limit int) ([]GitHubIssue, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues?state=open&per_page=%d", p.Repo, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if p.Token != "" {
		req.Header.Set("Authorization", "token "+p.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var issues []GitHubIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, err
	}

	return issues, nil
}

// isConsultationQuestion determines if an issue is a consultation question using LLM
func (p *IssueProcessor) isConsultationQuestion(issue GitHubIssue) bool {
	ctx := context.Background()

	// Create a prompt for LLM to analyze the issue
	determinationPrompt := fmt.Sprintf(`You are an expert at analyzing GitHub issues.
Your task is to determine if the given issue is a consultation question that seeks help, advice, or information.

Consultation questions are typically:
- Asking for help with usage or implementation
- Seeking advice or best practices
- Requesting explanations or documentation
- Looking for tutorials or examples
- General "how to" questions

NOT consultation questions:
- Bug reports (reporting errors, crashes, unexpected behavior)
- Feature requests (asking for new functionality)
- Documentation updates or fixes
- Code contributions or pull request discussions

Analyze this GitHub issue:

Title: %s
Body: %s

Respond with only "YES" if this is a consultation question, or "NO" if it is not.`, issue.Title, issue.Body)

	// Use the RAG's assistant client to make the determination
	if p.RAG == nil || p.RAG.AssistantClient == nil {
		// Fallback to simple keyword-based detection if RAG is not available
		return p.isConsultationQuestionFallback(issue)
	}

	chatClient := rag.ToChatClient(p.RAG.AssistantClient)
	if chatClient == nil {
		// Fallback to simple keyword-based detection if RAG client is not available
		return p.isConsultationQuestionFallback(issue)
	}

	resp, err := chatClient.Completions().New(ctx, openai.ChatCompletionNewParams{
		Model: p.RAG.AssistantModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(determinationPrompt),
		},
		MaxTokens: openai.Int(10), // We only need a short YES/NO response
	})

	if err != nil {
		log.Warn().Err(err).Msg("Failed to use LLM for issue classification, falling back to keyword detection")
		return p.isConsultationQuestionFallback(issue)
	}

	if len(resp.Choices) == 0 {
		log.Warn().Msg("No response from LLM for issue classification, falling back to keyword detection")
		return p.isConsultationQuestionFallback(issue)
	}

	response := strings.TrimSpace(strings.ToUpper(resp.Choices[0].Message.Content))
	return response == "YES"
}

// isConsultationQuestionFallback provides keyword-based fallback detection
func (p *IssueProcessor) isConsultationQuestionFallback(issue GitHubIssue) bool {
	// Keywords that indicate consultation questions
	consultationKeywords := []string{
		"how to", "how do", "how can", "how should",
		"what is", "what are", "what does", "what should",
		"why does", "why is", "why are",
		"when should", "when to",
		"where can", "where should",
		"which is", "which should",
		"help", "question", "ask", "advice",
		"guidance", "recommendation", "best practice",
		"tutorial", "example", "documentation",
	}

	// Bug report indicators (to exclude)
	bugKeywords := []string{
		"bug", "error", "crash", "fail", "broken",
		"not working", "doesn't work", "issue with",
		"problem with", "exception", "stack trace",
	}

	// Feature request indicators (to exclude)
	featureKeywords := []string{
		"feature request", "enhancement", "add support",
		"implement", "new feature", "would like",
	}

	text := strings.ToLower(issue.Title + " " + issue.Body)
	title := strings.ToLower(issue.Title)

	// Check for bug reports or feature requests first (exclude these)
	for _, keyword := range bugKeywords {
		if strings.Contains(text, keyword) {
			return false
		}
	}

	for _, keyword := range featureKeywords {
		if strings.Contains(text, keyword) {
			return false
		}
	}

	// Check for consultation question patterns
	// Use more specific matching to avoid false positives
	for _, keyword := range consultationKeywords {
		// For single words, ensure they are whole words
		if keyword == "help" || keyword == "question" || keyword == "ask" || keyword == "advice" || keyword == "guidance" || keyword == "tutorial" || keyword == "example" || keyword == "documentation" {
			// Check if it's a whole word (surrounded by word boundaries)
			if strings.Contains(text, " "+keyword+" ") || strings.HasPrefix(text, keyword+" ") || strings.HasSuffix(text, " "+keyword) || text == keyword {
				// Additional check: if it's "documentation", make sure it's asking about it, not updating it
				if keyword == "documentation" {
					if strings.Contains(text, "update documentation") || strings.Contains(text, "fix documentation") {
						continue
					}
				}
				return true
			}
		} else {
			// For phrases, use direct contains check
			if strings.Contains(text, keyword) {
				return true
			}
		}
	}

	// Check if title ends with question mark
	if strings.HasSuffix(strings.TrimSpace(title), "?") {
		return true
	}

	// Check if title starts with question words
	questionWords := []string{"how", "what", "why", "when", "where", "which", "who", "can"}
	for _, word := range questionWords {
		if strings.HasPrefix(title, word+" ") {
			return true
		}
	}

	return false
}

// generateAnswer uses RAG to generate an answer for the issue
// It includes confidence evaluation and will honestly express uncertainty when confidence is low
func (p *IssueProcessor) generateAnswer(ctx context.Context, issue GitHubIssue) (string, error) {
	if p.RAG == nil {
		return "", fmt.Errorf("RAG system is not initialized")
	}

	// Get chat client for confidence evaluation
	chatClient := rag.ToChatClient(p.RAG.AssistantClient)
	if chatClient == nil {
		return "", fmt.Errorf("chat client is not initialized")
	}

	query := issue.Title
	if issue.Body != "" {
		query += "\n" + issue.Body
	}

	param := &rag.AskParameter{
		Query:          query,
		RetrievalLimit: p.RAGLimit * 2, // Retrieve more chunks for LLM selection
		SelectedLimit:  p.RAGLimit,
	}

	answer, err := p.RAG.Ask(ctx, param)
	if err != nil {
		return "", err
	}

	// Evaluate confidence in the generated answer
	confidencePrompt := fmt.Sprintf(`You are an expert evaluator. Analyze the following question and answer pair to determine if the answer is confident and accurate.

Question: %s

Answer: %s

Evaluate the answer based on:
1. Does it directly address the question?
2. Is it specific and detailed enough?
3. Does it show uncertainty or vagueness?
4. Is it based on relevant information?

Respond with only "HIGH" if the answer is confident and likely accurate, or "LOW" if the answer shows uncertainty, is vague, or may not be reliable.`, query, answer)

	confidenceResp, err := chatClient.Completions().New(ctx, openai.ChatCompletionNewParams{
		Model: p.RAG.AssistantModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(confidencePrompt),
		},
		MaxTokens: openai.Int(10),
	})
	if err != nil {
		// If confidence evaluation fails, return the original answer with a disclaimer
		return fmt.Sprintf("%s\n\n*Note: I generated this answer but couldn't evaluate my confidence level. Please verify the information.*", answer), nil
	}

	if len(confidenceResp.Choices) == 0 {
		return fmt.Sprintf("%s\n\n*Note: I generated this answer but couldn't evaluate my confidence level. Please verify the information.*", answer), nil
	}

	confidenceLevel := strings.TrimSpace(confidenceResp.Choices[0].Message.Content)
	if strings.ToUpper(confidenceLevel) == "LOW" {
		return "I'm not confident enough to provide a reliable answer to this question based on the available documentation. The question might require more specific context, recent updates, or domain expertise that I don't have sufficient information about. I'd recommend:\n\n1. Checking the official documentation or recent updates\n2. Asking in community forums or discussions\n3. Consulting with maintainers or experienced users\n\nI want to be honest rather than potentially misleading you with an uncertain answer.", nil
	}

	return answer, nil
}

// postAnswer posts the answer as a comment (simulated for testing)
func (p *IssueProcessor) postAnswer(ctx context.Context, issue GitHubIssue, answer string) error {
	// For now, just log the answer instead of actually posting to GitHub
	// In a real implementation, this would use GitHub API to post a comment
	log.Info().Int("issue_number", issue.Number).Str("answer", answer).Msg("Generated answer for issue")

	// TODO: Implement actual GitHub comment posting
	// This would require:
	// 1. POST to https://api.github.com/repos/{owner}/{repo}/issues/{issue_number}/comments
	// 2. With body: {"body": answer}
	// 3. With proper authentication

	return nil
}

// loadProcessedIssues loads the list of processed issue IDs
func (p *IssueProcessor) loadProcessedIssues() (*ProcessedIssues, error) {
	if _, err := os.Stat(p.ProcessedFile); os.IsNotExist(err) {
		return &ProcessedIssues{IssueIDs: []int{}}, nil
	}

	data, err := os.ReadFile(p.ProcessedFile)
	if err != nil {
		return nil, err
	}

	var processed ProcessedIssues
	if err := json.Unmarshal(data, &processed); err != nil {
		return nil, err
	}

	return &processed, nil
}

// saveProcessedIssues saves the list of processed issue IDs
func (p *IssueProcessor) saveProcessedIssues(processed *ProcessedIssues) error {
	// Ensure directory exists
	dir := filepath.Dir(p.ProcessedFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(processed, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(p.ProcessedFile, data, 0644)
}

// isProcessed checks if an issue has already been processed
func (p *IssueProcessor) isProcessed(processed *ProcessedIssues, issueNumber int) bool {
	for _, id := range processed.IssueIDs {
		if id == issueNumber {
			return true
		}
	}
	return false
}
