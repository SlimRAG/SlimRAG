package rag

import (
	"context"
	"database/sql"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/minio/minio-go/v7"
	"github.com/openai/openai-go"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/sourcegraph/conc/pool"
)

type RAG struct {
	DB              *sql.DB
	OSS             *minio.Client
	EmbeddingClient *openai.Client
	EmbeddingModel  string
	RerankerClient  *InfinityClient
	RerankerModel   string
	AssistantClient *openai.Client
	AssistantModel  string
}

func (r *RAG) UpsertDocumentChunks(document *Document) error {
	if len(document.Chunks) == 0 {
		return nil
	}

	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO document_chunks (id, document_id, text, start_offset, end_offset, embedding)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			embedding = EXCLUDED.embedding;
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, chunk := range document.Chunks {
		_, err = stmt.Exec(
			chunk.ID,
			chunk.Document,
			chunk.Text,
			0, // start_offset is not used
			0, // end_offset is not used
			chunk.Embedding,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *RAG) ComputeEmbeddings(ctx context.Context, onlyEmpty bool, workers int) error {
	var err error
	var rows *sql.Rows
	if onlyEmpty {
		rows, err = r.DB.QueryContext(ctx, "SELECT id, text, embedding FROM document_chunks WHERE embedding IS NULL")
	} else {
		rows, err = r.DB.QueryContext(ctx, "SELECT id, text, embedding FROM document_chunks")
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
		var embedding []float32
		err = rows.Scan(&chunk.ID, &chunk.Text, &embedding)
		if err != nil {
			return err
		}

		if embedding != nil || len(chunk.Text) == 0 {
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
				Dimensions:     openai.Int(dims),
				EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
			})
			if err != nil {
				log.Error().Err(err).Stack().Str("chunk_id", chunk.ID).Msg("Compute embedding")
				return
			}

			chunk.Embedding = toFloat32Slice(rsp.Data[0].Embedding)

			_, err = r.DB.ExecContext(ctx, "UPDATE document_chunks SET embedding = ? WHERE id = ?", chunk.Embedding, chunk.ID)
			if err != nil {
				log.Error().Err(err).Stack().Str("chunk_id", chunk.ID).Msg("Update embedding")
			}
		})
	}

	wg.Wait()
	p.Wait()
	return nil
}

func toFloat32Slice(v []float64) []float32 {
	x := make([]float32, len(v))
	for i, f := range v {
		x[i] = float32(f)
	}
	return x
}

func (r *RAG) QueryDocumentChunks(ctx context.Context, query string, limit int) ([]DocumentChunk, error) {
	rsp, err := r.EmbeddingClient.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: r.EmbeddingModel,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(query),
		},
		Dimensions: openai.Int(dims),
	})
	if err != nil {
		return nil, err
	}
	queryEmbedding := toFloat32Slice(rsp.Data[0].Embedding)

	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, document_id, text, embedding
		FROM document_chunks
		ORDER BY array_distance(embedding, ?::FLOAT[384])
		LIMIT ?;
	`, queryEmbedding, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chunks []DocumentChunk
	for rows.Next() {
		var chunk DocumentChunk
		var embedding []float32
		err = rows.Scan(&chunk.ID, &chunk.Document, &chunk.Text, &embedding)
		if err != nil {
			return nil, err
		}
		chunk.Embedding = embedding
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

func (r *RAG) GetDocumentChunk(id string) (*DocumentChunk, error) {
	row := r.DB.QueryRow("SELECT id, document_id, text, embedding FROM document_chunks WHERE id = ?", id)

	var chunk DocumentChunk
	var embedding []float32
	err := row.Scan(&chunk.ID, &chunk.Document, &chunk.Text, &embedding)
	if err != nil {
		return nil, err
	}
	chunk.Embedding = embedding
	return &chunk, nil
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

func (r *RAG) FindInvalidChunks(ctx context.Context, f func(chunk *DocumentChunk)) error {
	rows, err := r.DB.QueryContext(ctx, "SELECT id, document_id, text, embedding FROM document_chunks WHERE embedding IS NULL")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var chunk DocumentChunk
		var embedding []float32
		err = rows.Scan(&chunk.ID, &chunk.Document, &chunk.Text, &embedding)
		if err != nil {
			return err
		}
		chunk.Embedding = embedding
		f(&chunk)
	}
	return nil
}

func (r *RAG) DeleteChunk(id string) error {
	_, err := r.DB.Exec("DELETE FROM document_chunks WHERE id = ?", id)
	return err
}

func (r *RAG) Ask(ctx context.Context, query string, chunks []DocumentChunk) (string, error) {
	prompt := buildPrompt(query, chunks)
	c, err := r.AssistantClient.Completions.New(ctx, openai.CompletionNewParams{
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
