package rag

import (
	"context"
	"database/sql"
	"encoding/hex"
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
	ID          string               `gorm:"primaryKey"`
	Document    string               `gorm:"not null"`
	RawDocument string               `gorm:"not null"`
	Text        string               `gorm:"not null" json:"text,omitzero"`
	Embedding   *pgvector.HalfVector `gorm:"type:halfvec(4000)" json:"embedding,omitzero"`
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

type RAG struct {
	DB              *gorm.DB
	OSS             *minio.Client
	EmbeddingClient *openai.Client
	EmbeddingModel  string
	RerankerClient  *InfinityClient
	RerankerModel   string
	AssistantClient *openai.Client
	AssistantModel  string
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

	err = db.AutoMigrate(&DocumentChunk{})
	if err != nil {
		return errors.Wrap(err, "Failed to migrate document chunks")
	}

	//err = db.Exec(`SET maintenance_work_mem = '8GB'`).Error
	//if err != nil {
	//	return errors.Wrap(err, "Failed to set maintenance work mem")
	//}
	//
	//err = db.Exec(`SET max_parallel_maintenance_workers = 32`).Error
	//if err != nil {
	//	return errors.Wrap(err, "Failed to set max_parallel_maintenance_workers")
	//}

	//err = db.Exec(`CREATE INDEX IF NOT EXISTS ON document_chunks USING hnsw (embedding halfvec_l2_ops)`).Error
	//if err != nil {
	//	return errors.Wrap(err, "Failed to create index on document_chunks")
	//}

	return nil
}

func (r *RAG) UpsertDocumentChunks(document *Document) error {
	if len(document.Chunks) == 0 {
		return nil
	}

	counts := make(map[string]int)
	for _, chunk := range document.Chunks {
		counts[chunk.ID]++
	}

	chunks := make([]*DocumentChunk, 0, len(document.Chunks))
	for _, c := range document.Chunks {
		count := counts[c.ID]
		if count == 1 {
			chunks = append(chunks, c)
		} else {
			counts[c.ID] = count - 1
		}
	}

	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&chunks).Error
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
			rsp, err = r.EmbeddingClient.Embeddings.New(ctx, openai.EmbeddingNewParams{
				Model: r.EmbeddingModel,
				Input: openai.EmbeddingNewParamsInputUnion{
					OfString: openai.String(chunk.Text),
				},
				Dimensions:     openai.Int(4000),
				EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
			})
			if err != nil {
				log.Error().Err(err).Stack().Str("chunk_id", chunk.ID).Msg("Compute embedding")
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

// TODO: use PCA for cast down
func castDown(e []float64) []float32 {
	x := make([]float32, 4000)
	for i := range x {
		x[i] = float32(e[i])
	}
	return x
}

func (r *RAG) QueryDocumentChunks(ctx context.Context, query string, limit int) ([]DocumentChunk, error) {
	rsp, err := r.EmbeddingClient.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: r.EmbeddingModel,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(query),
		},
	})
	if err != nil {
		return nil, err
	}
	queryEmbedding := castDown(rsp.Data[0].Embedding)

	var chunks []DocumentChunk
	err = r.DB.Clauses(clause.OrderBy{
		Expression: clause.Expr{
			SQL:  "embedding <-> ?",
			Vars: []interface{}{pgvector.NewVector(queryEmbedding)},
		}},
	).Select([]string{"id", "document", "raw_document"}).
		Limit(limit).
		Find(&chunks).Error
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

func (r *RAG) DeleteChunk(id string) error {
	return r.DB.Where("id = ?", id).Delete(&DocumentChunk{}).Error
}

func (r *RAG) Rerank(query string, chunks []DocumentChunk, topN int) ([]DocumentChunk, error) {
	m := make(map[string]int)
	docs := make([]string, len(chunks))
	for i, c := range chunks {
		docs[i] = c.Text
		m[hashString(c.Text)] = i
	}

	rsp, err := r.RerankerClient.Rerank(&RerankRequest{
		Model:           r.RerankerModel,
		Query:           query,
		Documents:       docs,
		TopN:            topN,
		ReturnDocuments: true, // TODO:	maybe we don't need this
	})
	if err != nil {
		return nil, err
	}

	cs := make([]DocumentChunk, len(rsp.Results))
	for i, x := range rsp.Results {
		cs[i] = chunks[m[hashString(x.Document)]]
	}
	return cs, nil
}

func (r *RAG) Ask(ctx context.Context, query string, chunks []DocumentChunk) (string, error) {
	prompt := BuildPrompt(query, chunks)
	c, err := r.EmbeddingClient.Completions.New(ctx, openai.CompletionNewParams{
		Model:  openai.CompletionNewParamsModel(r.AssistantModel),
		Prompt: openai.CompletionNewParamsPromptUnion{OfString: openai.String(prompt)},
	})
	if err != nil {
		return "", err
	}
	if len(c.Choices) > 0 {
		return c.Choices[0].Text, nil
	}
	return "", errors.New("no choices returned from completion")
}
