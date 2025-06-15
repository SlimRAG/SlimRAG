package rag

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/openai/openai-go"
	"github.com/pgvector/pgvector-go"
	gormzerolog "github.com/vitaliy-art/gorm-zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type DocumentChunk struct {
	ID          uint             `gorm:"primaryKey;autoIncrement"`
	Document    string           `gorm:"not null"`
	RawDocument string           `gorm:"not null"`
	ChunkID     uint             `gorm:"not null" json:"index"`
	Text        string           `gorm:"not null" json:"text"`
	Embedding   *pgvector.Vector `gorm:"type:vector(1024)"`
}

type Document struct {
	Document    string          `json:"document"`
	RawDocument string          `json:"raw_document"`
	Chunks      []DocumentChunk `json:"chunks"`
}

func OpenDB(dsn string) (*gorm.DB, error) {
	logger := gormzerolog.NewGormLogger()
	logger.IgnoreRecordNotFoundError(true)
	logger.LogMode(gormlogger.Warn)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger})
	if err != nil {
		return nil, err
	}

	err = Migrate(db)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error
	if err != nil {
		return errors.Wrap(err, "Failed to create vector extension")
	}
	return db.AutoMigrate(&DocumentChunk{})
}

func UpsertDocumentChunks(db *gorm.DB, document *Document) error {
	if len(document.Chunks) == 0 {
		return nil
	}

	maxChunkID := document.Chunks[0].ChunkID

	for i := range document.Chunks {
		c := &document.Chunks[i]
		c.Document = document.Document
		c.RawDocument = document.RawDocument

		u := db.Model(&c).Where("document = ? AND chunk_id = ?", c.Document, c.ChunkID).Updates(&c)
		if err := u.Error; err != nil {
			return err
		}
		if u.RowsAffected == 0 {
			db.Create(&c)
		}
		maxChunkID = max(maxChunkID, c.ChunkID)
	}

	var restChunks []DocumentChunk
	err := db.Model(&DocumentChunk{}).
		Select("id").
		Where("chunk_id > ?", maxChunkID).
		Find(&restChunks).Error
	if err != nil {
		return err
	}

	if len(restChunks) > 0 {
		return db.Delete(&restChunks).Error
	}

	return nil
}

func ComputeEmbeddings(ctx context.Context, db *gorm.DB, client *openai.Client, model string) error {
	rows, err := db.Model(&DocumentChunk{}).Where("embedding IS NULL").Rows()
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var chunk DocumentChunk
		err = db.ScanRows(rows, &chunk)
		if err != nil {
			return err
		}

		var rsp *openai.CreateEmbeddingResponse
		rsp, err = client.Embeddings.New(ctx, openai.EmbeddingNewParams{
			Model: model,
			Input: openai.EmbeddingNewParamsInputUnion{
				OfString: openai.String(chunk.Text),
			},
			EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
		})
		if err != nil {
			return errors.Wrapf(err, "compute embedding for chunk %d", chunk.ID)
		}

		embedding := rsp.Data[0].Embedding
		x := make([]float32, len(embedding))
		for i, v := range embedding {
			x[i] = float32(v)
		}

		v := pgvector.NewVector(x)
		chunk.Embedding = &v

		db.Save(&chunk)
	}

	return nil
}
