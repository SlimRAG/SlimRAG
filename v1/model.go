package rag

import (
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/negrel/assert"
)

type DocumentChunk struct {
	ID          string    `gorm:"primaryKey"`
	Document    string    `gorm:"not null"`
	RawDocument string    `gorm:"not null"`
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
	var filePath string
	if d.FilePath != "" {
		filePath = d.FilePath
	} else if d.FileName != "" {
		// 尝试获取绝对路径
		if absPath, err := filepath.Abs(d.FileName); err == nil {
			filePath = absPath
		} else {
			filePath = d.FileName
		}
	}

	// 使用文件路径生成 document_id
	d.Document = GenerateDocumentID(filePath)
	d.RawDocument = d.FileName
	for _, chunk := range d.Chunks {
		chunk.Fix(d)
	}
}
