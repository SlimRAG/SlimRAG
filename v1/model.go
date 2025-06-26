package rag

import (
	"encoding/hex"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/negrel/assert"
	"github.com/pgvector/pgvector-go"
)

const dims = 2560

var zeroVector = pgvector.NewHalfVector(make([]float32, dims))

type DocumentChunk struct {
	ID          string               `gorm:"primaryKey"`
	Document    string               `gorm:"not null"`
	RawDocument string               `gorm:"not null"`
	Text        string               `gorm:"not null" json:"text,omitzero"`
	Embedding   *pgvector.HalfVector `gorm:"type:halfvec(2560)" json:"embedding,omitzero"`
	Index       int                  `gorm:"-:all" json:"index"`
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
	Document    string           `json:"document"`
	RawDocument string           `json:"raw_document"`
	Chunks      []*DocumentChunk `json:"chunks"`
}

func (d *Document) Fix() {
	d.Document = strings.TrimSuffix(d.FileName, ".md")
	d.RawDocument = d.FileName
	for _, chunk := range d.Chunks {
		chunk.Fix(d)
	}
}
