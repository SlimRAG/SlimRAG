package rag

import (
	"github.com/cockroachdb/errors"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

type DocumentChunk struct {
	ID           uint            `gorm:"primaryKey;autoIncrement"`
	DocumentName string          `gorm:"not null"`
	ChunkID      uint            `gorm:"not null" json:"index"`
	Text         string          `gorm:"not null" json:"text"`
	Embedding    pgvector.Vector `gorm:"type:vector(1024)"`
}

type Document struct {
	FileName string          `json:"file_name"`
	Chunks   []DocumentChunk `json:"chunks"`
}

func Migrate(db *gorm.DB) error {
	err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error
	if err != nil {
		return errors.Wrap(err, "Failed to create vector extension")
	}
	return db.AutoMigrate(&DocumentChunk{})
}

func UpsertDocumentChunks(db *gorm.DB, document *Document) error {
	err := db.Delete(&DocumentChunk{}, "document_name = ?", document.FileName).Error
	if err != nil {
		return err
	}

	zero := pgvector.NewVector(make([]float32, 1024))
	for i := range document.Chunks {
		c := &document.Chunks[i]
		c.DocumentName = document.FileName
		c.Embedding = zero
	}

	return db.Create(&document.Chunks).Error
}
