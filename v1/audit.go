package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/openai/openai-go"
	"github.com/rs/zerolog/log"
)

type AuditLogger struct {
	enabled bool
	logDir  string
}

type APIRequest struct {
	Timestamp time.Time     `json:"timestamp"`
	APIType   string        `json:"api_type"`
	Model     string        `json:"model"`
	Request   interface{}   `json:"request"`
	Response  interface{}   `json:"response"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration_ms"`
	RequestID string        `json:"request_id,omitempty"`
}

func NewAuditLogger(enabled bool, logDir string) *AuditLogger {
	if enabled && logDir == "" {
		logDir = "./audit_logs"
	}

	return &AuditLogger{
		enabled: enabled,
		logDir:  logDir,
	}
}

func (a *AuditLogger) LogAPICall(ctx context.Context, apiType, model string, request, response interface{}, err error, duration time.Duration, requestID string) {
	if !a.enabled {
		return
	}

	// Create log directory if it doesn't exist
	if a.logDir != "" {
		if err := os.MkdirAll(a.logDir, 0755); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log directory")
			return
		}
	}

	// Create audit record
	auditRecord := APIRequest{
		Timestamp: time.Now(),
		APIType:   apiType,
		Model:     model,
		Request:   request,
		Response:  response,
		Duration:  duration,
		RequestID: requestID,
	}

	if err != nil {
		auditRecord.Error = err.Error()
	}

	// Convert to JSON for validation (not used directly)
	_, err = json.MarshalIndent(auditRecord, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal audit record")
		return
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("api_call_%s_%d.md",
		strings.ToLower(apiType),
		time.Now().UnixNano())

	if a.logDir != "" {
		filename = filepath.Join(a.logDir, filename)
	}

	// Write to file in markdown format
	file, err := os.Create(filename)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create audit log file")
		return
	}
	defer file.Close()

	// Write markdown header
	markdownContent := fmt.Sprintf(`# API Call Audit Log

**Timestamp:** %s  
**API Type:** %s  
**Model:** %s  
**Duration:** %v  
**Request ID:** %s  

## Request

`+"```json\n%s\n```\n\n"+`## Response

`+"```json\n%s\n```\n\n"+`%s
`,
		auditRecord.Timestamp.Format("2006-01-02 15:04:05.000"),
		auditRecord.APIType,
		auditRecord.Model,
		auditRecord.Duration.Round(time.Millisecond),
		auditRecord.RequestID,
		prettyJSON(auditRecord.Request),
		prettyJSON(auditRecord.Response),
		a.getErrorMarkdown(auditRecord.Error),
	)

	if _, err := file.WriteString(markdownContent); err != nil {
		log.Error().Err(err).Msg("Failed to write audit log file")
		return
	}

	log.Debug().Str("filename", filename).Msg("API call audit log created")
}

func (a *AuditLogger) getErrorMarkdown(errMsg string) string {
	if errMsg == "" {
		return ""
	}
	return fmt.Sprintf("## Error\n\n"+"```\n%s\n```", errMsg)
}

func prettyJSON(v interface{}) string {
	if v == nil {
		return "null"
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling JSON: %v", err)
	}
	return string(data)
}

// AuditClient wraps OpenAI client to add audit logging
type AuditClient struct {
	*openai.Client
	auditLogger *AuditLogger
	model       string
	apiType     string
}

func NewAuditClient(client *openai.Client, auditLogger *AuditLogger, model, apiType string) *AuditClient {
	return &AuditClient{
		Client:      client,
		auditLogger: auditLogger,
		model:       model,
		apiType:     apiType,
	}
}

func (c *AuditClient) WithAuditLogger(logger *AuditLogger) *AuditClient {
	return &AuditClient{
		Client:      c.Client,
		auditLogger: logger,
		model:       c.model,
		apiType:     c.apiType,
	}
}

func (c *AuditClient) WithModel(model string) *AuditClient {
	return &AuditClient{
		Client:      c.Client,
		auditLogger: c.auditLogger,
		model:       model,
		apiType:     c.apiType,
	}
}

func (c *AuditClient) WithAPIType(apiType string) *AuditClient {
	return &AuditClient{
		Client:      c.Client,
		auditLogger: c.auditLogger,
		model:       c.model,
		apiType:     apiType,
	}
}
