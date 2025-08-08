package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
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

func (r *RAG) Rerank(ctx context.Context, query string, chunks []DocumentChunk, topN int) ([]DocumentChunk, error) {
	prompt := MustGetPrompt("reranker")
	prompt = strings.ReplaceAll(prompt, "{{.Query}}", query)

	var documents string
	for i, chunk := range chunks {
		documents += fmt.Sprintf("Document %d: %s\n", i+1, chunk.Text)
	}
	prompt = strings.ReplaceAll(prompt, "{{.Documents}}", documents)

	c, err := r.AssistantClient.Completions.New(ctx, openai.CompletionNewParams{
		Model:  openai.CompletionNewParamsModel(r.AssistantModel),
		Prompt: openai.CompletionNewParamsPromptUnion{OfString: openai.String(prompt)},
		Stop:   openai.CompletionNewParamsStopUnion{OfStringArray: []string{"\n"}},
		N:      openai.Int(1),
	})
	if err != nil {
		return nil, errors.Wrap(err, "CreateCompletion")
	}

	if len(c.Choices) == 0 {
		// return original chunks if no choice
		if len(chunks) > topN {
			return chunks[:topN], nil
		}
		return chunks, nil
	}

	choice := c.Choices[0].Text
	log.Info().Str("choice", choice).Msg("Reranked")

	// parse the response and pick the top N chunks
	// The response is a list of document indices, separated by commas.
	// e.g. "1, 3, 2"
	parts := strings.Split(choice, ",")
	var rerankedChunks []DocumentChunk
	seen := make(map[int]struct{})
	for _, part := range parts {
		index, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			log.Warn().Err(err).Str("part", part).Msg("Failed to parse rerank index")
			continue
		}

		// index is 1-based
		if index-1 < 0 || index-1 >= len(chunks) {
			log.Warn().Int("index", index).Msg("Rerank index out of bounds")
			continue
		}

		if _, ok := seen[index]; ok {
			log.Warn().Int("index", index).Msg("Duplicate rerank index")
			continue
		}

		rerankedChunks = append(rerankedChunks, chunks[index-1])
		seen[index] = struct{}{} // mark as seen
	}

	// Add the remaining chunks that were not picked by the reranker
	for i, chunk := range chunks {
		if _, ok := seen[i+1]; !ok {
			rerankedChunks = append(rerankedChunks, chunk)
		}
	}

	if len(rerankedChunks) > topN {
		return rerankedChunks[:topN], nil
	}

	return rerankedChunks, nil
}

func (r *RAG) Ask(ctx context.Context, p *AskParameter) (string, error) {
	chunks, err := r.QueryDocumentChunks(ctx, p.Query, p.Limit)
	if err != nil {
		return "", err
	}

	chunks, err = r.Rerank(ctx, p.Query, chunks, p.Limit)
	if err != nil {
		return "", err
	}

	prompt := buildPrompt(p.Query, chunks)
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
