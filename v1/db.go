package rag

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/minio/minio-go/v7"
	"github.com/openai/openai-go"
	"github.com/pgvector/pgvector-go"
	gormzerolog "github.com/vitaliy-art/gorm-zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

type DocumentChunk struct {
	ID          uint             `gorm:"primaryKey;autoIncrement"`
	Document    string           `gorm:"not null"`
	RawDocument string           `gorm:"not null"`
	ChunkID     uint             `gorm:"not null" json:"index"`
	Text        string           `gorm:"not null" json:"text,omitzero"`
	Embedding   *pgvector.Vector `gorm:"type:vector(1024)" json:"embedding,omitzero"`
}

type Document struct {
	FileName    string          `json:"file_name"`
	Document    string          `json:"document"`
	RawDocument string          `json:"raw_document"`
	Chunks      []DocumentChunk `json:"chunks"`
}

type RAG struct {
	DB     *gorm.DB
	Client *openai.Client
	Model  string
	OSS    *minio.Client
}

func OpenDB(dsn string) (*gorm.DB, error) {
	logger := gormzerolog.NewGormLogger()
	logger.IgnoreRecordNotFoundError(true)
	logger.LogMode(gormlogger.Warn)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger})
	if err != nil {
		return nil, err
	}

	err = migrate(db)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func migrate(db *gorm.DB) error {
	err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error
	if err != nil {
		return errors.Wrap(err, "Failed to create vector extension")
	}
	//db.Exec("CREATE INDEX ON items USING hnsw (embedding vector_l2_ops)")
	return db.AutoMigrate(&DocumentChunk{})
}

func (r *RAG) UpsertDocumentChunks(document *Document) error {
	if len(document.Chunks) == 0 {
		return nil
	}

	maxChunkID := document.Chunks[0].ChunkID

	for i := range document.Chunks {
		c := &document.Chunks[i]
		c.Document = document.Document
		c.RawDocument = document.RawDocument

		u := r.DB.Model(&c).Where("document = ? AND chunk_id = ?", c.Document, c.ChunkID).Updates(&c)
		if err := u.Error; err != nil {
			return err
		}
		if u.RowsAffected == 0 {
			r.DB.Create(&c)
		}
		maxChunkID = max(maxChunkID, c.ChunkID)
	}

	var restChunks []DocumentChunk
	err := r.DB.Model(&DocumentChunk{}).
		Select("id").
		Where("chunk_id > ?", maxChunkID).
		Find(&restChunks).Error
	if err != nil {
		return err
	}

	if len(restChunks) > 0 {
		return r.DB.Delete(&restChunks).Error
	}

	return nil
}

func (r *RAG) ComputeEmbeddings(ctx context.Context, onlyEmpty bool) error {
	var err error
	var rows *sql.Rows
	if onlyEmpty {
		rows, err = r.DB.Model(&DocumentChunk{}).Where("embedding IS NULL").Rows()
	} else {
		rows, err = r.DB.Model(&DocumentChunk{}).Rows()
	}
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var chunk DocumentChunk
		err = r.DB.ScanRows(rows, &chunk)
		if err != nil {
			return err
		}

		var rsp *openai.CreateEmbeddingResponse
		rsp, err = r.Client.Embeddings.New(ctx, openai.EmbeddingNewParams{
			Model: r.Model,
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

		r.DB.Save(&chunk)
	}

	return nil
}

func (r *RAG) QueryDocuments(ctx context.Context, query string, limit int) ([]DocumentChunk, error) {
	rsp, err := r.Client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: r.Model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(query),
		},
	})
	if err != nil {
		return nil, err
	}

	e := rsp.Data[0].Embedding
	queryEmbedding := make([]float32, len(e))
	for i := range rsp.Data[0].Embedding {
		queryEmbedding[i] = float32(e[i])
	}

	var chunks []DocumentChunk
	err = r.DB.Clauses(clause.OrderBy{
		Expression: clause.Expr{
			SQL:  "embedding <-> ?",
			Vars: []interface{}{pgvector.NewVector(queryEmbedding)},
		}},
	).Select("id", "document", "raw_document", "chunk_id").Limit(limit).Find(&chunks).Error
	if err != nil {
		return nil, err
	}
	return chunks, nil
}

func (r *RAG) GetDocumentChunk(id string) (*DocumentChunk, error) {
	var c DocumentChunk
	err := r.DB.Model(&DocumentChunk{}).Where("id = ?", id).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}
