package rag

import (
	"encoding/hex"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/negrel/assert"
)

type DocumentChunk struct {
	ID          string    `gorm:"primaryKey"`
	Document    string    `gorm:"not null"`
	RawDocument string    `gorm:"not null"`
	FilePath    string    `gorm:"not null"`
	Text        string    `gorm:"not null" json:"text,omitzero"`
	Embedding   []float32 `gorm:"type:float[]" json:"embedding,omitzero"`
	Index       int       `gorm:"-:all" json:"index"`
}

func hashString(s string) string {
	h := xxhash.New()
	_, err := h.Write([]byte(s))
	assert.NoError(err)
	b := h.Sum(nil)
	return hex.EncodeToString(b)
}

func (c *DocumentChunk) Fix(d *Document) {
	c.Text = strings.ReplaceAll(c.Text, "\u0000", "")
	c.ID = hashString(c.Text)
	c.Document = d.Document
	c.RawDocument = d.RawDocument
	c.FilePath = d.FilePath
}

type Document struct {
	FileName    string           `json:"file_name"`
	FilePath    string           `json:"file_path"`
	Document    string           `json:"document"`
	RawDocument string           `json:"raw_document"`
	Chunks      []*DocumentChunk `json:"chunks"`
}

type AskParameter struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (d *Document) Fix() {
	// 使用所有 chunks 的文本内容生成 document_id
	var fullContent strings.Builder
	for _, chunk := range d.Chunks {
		fullContent.WriteString(chunk.Text)
	}

	d.Document = hashString(fullContent.String())
	d.RawDocument = d.FileName
	for _, chunk := range d.Chunks {
		chunk.Fix(d)
	}
}
