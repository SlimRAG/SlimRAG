package rag

import (
	"encoding/hex"
	"time"

	"github.com/cespare/xxhash"
	"github.com/negrel/assert"
)

type FileInfo struct {
	FilePath    string
	FileName    string
	FileHash    string
	ProcessedAt time.Time
}

type DocumentChunk struct {
	ID         string
	DocumentID string
	Text       string
	Embedding  []float32
	Index      int
}

func hashString(s string) string {
	h := xxhash.New()
	_, err := h.Write([]byte(s))
	assert.NoError(err)
	b := h.Sum(nil)
	return hex.EncodeToString(b)
}

type Document struct {
	FileName   string           `json:"file_name"`
	FilePath   string           `json:"file_path"`
	DocumentID string           `json:"document_id"`
	Chunks     []*DocumentChunk `json:"chunks"`
}

type AskParameter struct {
	Query          string          `json:"query"`
	SelectedChunks []DocumentChunk `json:"selected_chunks"`
	RetrievalLimit int             `json:"retrieval_limit"` // Number of vector retrievals, e.g., 100
	SelectedLimit  int             `json:"selected_limit"`  // Number of LLM selections, e.g., 10
	SystemPrompt   string          `json:"system_prompt"`   // Custom system prompt
}
