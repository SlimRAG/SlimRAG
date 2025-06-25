package rag

import (
	"context"
	"database/sql"
	"strings"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/cockroachdb/errors"
	"github.com/minio/minio-go/v7"
	"github.com/negrel/assert"
	"github.com/openai/openai-go"
	"github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/sourcegraph/conc/pool"
	gormzerolog "github.com/vitaliy-art/gorm-zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

type DocumentChunk struct {
	ID          uint64               `gorm:"primaryKey"`
	Document    string               `gorm:"not null"`
	RawDocument string               `gorm:"not null"`
	Text        string               `gorm:"not null" json:"text,omitzero"`
	Embedding   *pgvector.HalfVector `gorm:"type:halfvec(4000)" json:"embedding,omitzero"`
}

func hashString(s string) uint64 {
	h := xxhash.New()
	_, err := h.Write([]byte(s))
	assert.NoError(err)
	return h.Sum64()
}

func (c *DocumentChunk) Fix() {
	c.Text = strings.ReplaceAll(c.Text, "\u0000", "")
	c.ID = hashString(c.Text)
}

type Document struct {
	FileName    string          `json:"file_name"`
	Document    string          `json:"document"`
	RawDocument string          `json:"raw_document"`
	Chunks      []DocumentChunk `json:"chunks"`
}

func (d *Document) Fix() {
	for _, chunk := range d.Chunks {
		chunk.Fix()
	}
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
	logger.LogMode(gormlogger.Error)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger,
	})
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

	for i := range document.Chunks {
		c := &document.Chunks[i]
		c.Document = document.Document
		c.RawDocument = document.RawDocument
	}

	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"document", "raw_document"}),
	}).Create(&document.Chunks).Error
}

func (r *RAG) ComputeEmbeddings(ctx context.Context, onlyEmpty bool, workers int) error {
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

	bar := progressbar.Default(-1)
	bar.Describe("Computing embeddings")
	defer func() { _ = bar.Finish() }()

	p := pool.New().WithMaxGoroutines(workers)
	var wg sync.WaitGroup

	for rows.Next() {
		var chunk DocumentChunk
		err = r.DB.ScanRows(rows, &chunk)
		if err != nil {
			return err
		}

		if chunk.Embedding != nil || len(chunk.Text) == 0 {
			_ = bar.Add(1)
			continue
		}

		p.Go(func() {
			wg.Add(1)
			defer func() {
				_ = bar.Add(1)
				wg.Done()
			}()

			var rsp *openai.CreateEmbeddingResponse
			rsp, err = r.Client.Embeddings.New(ctx, openai.EmbeddingNewParams{
				Model: r.Model,
				Input: openai.EmbeddingNewParamsInputUnion{
					OfString: openai.String(chunk.Text),
				},
				Dimensions:     openai.Int(4000),
				EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
			})
			if err != nil {
				log.Error().Err(err).Stack().Uint64("chunk_id", chunk.ID).Msg("Compute embedding")
				return
			}

			embedding := rsp.Data[0].Embedding
			v := pgvector.NewHalfVector(castDown(embedding))
			chunk.Embedding = &v

			r.DB.Save(&chunk)
		})
	}

	wg.Wait()
	p.Wait()
	return nil
}

func castDown(e []float64) []float32 {
	x := make([]float32, 4000)
	for i := range x {
		x[i] = float32(e[i])
	}
	return x
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

func (r *RAG) FindInvalidChunks(ctx context.Context, cb func(chunk *DocumentChunk)) error {
	bar := progressbar.Default(-1)
	bar.Describe("Cleaning up chunks")
	defer func() { _ = bar.Finish() }()

	zeroVector := pgvector.NewHalfVector(make([]float32, 4000))

	var err error
	var rows *sql.Rows
	rows, err = r.DB.
		Model(&DocumentChunk{}).
		Where("embedding IS NULL OR (text = '') IS NOT FALSE OR embedding <-> ? = 0", zeroVector).
		Rows()
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var chunk DocumentChunk
		err = r.DB.ScanRows(rows, &chunk)
		if err != nil {
			return err
		}
		cb(&chunk)
	}

	return nil
}

func (r *RAG) DeleteChunk(id uint64) error {
	return r.DB.Where("id = ?", id).Delete(&DocumentChunk{}).Error
}
