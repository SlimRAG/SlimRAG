package rag

import (
	"context"
	"database/sql"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/cockroachdb/errors"
	"github.com/minio/minio-go/v7"
	"github.com/openai/openai-go"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/sourcegraph/conc/pool"
)

type RAG struct {
	DB                  *sql.DB
	OSS                 *minio.Client
	EmbeddingClient     *openai.Client
	EmbeddingModel      string
	EmbeddingDimensions int64
	AssistantClient     *openai.Client
	AssistantModel      string
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
		INSERT INTO document_chunks (id, document_id, text, start_offset, end_offset)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			text = EXCLUDED.text,
			document_id = EXCLUDED.document_id;
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
		var embeddingBytes interface{}
		err = rows.Scan(&chunk.ID, &chunk.Text, &embeddingBytes)
		if err != nil {
			return err
		}

		// Skip if embedding already exists or text is empty
		if embeddingBytes != nil || len(chunk.Text) == 0 {
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
				Dimensions:     openai.Int(r.EmbeddingDimensions),
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
		Dimensions: openai.Int(r.EmbeddingDimensions),
	})
	if err != nil {
		return nil, err
	}
	queryEmbedding := toFloat32Slice(rsp.Data[0].Embedding)

	rows, err := r.DB.QueryContext(ctx, `SELECT id, document_id, text, embedding FROM document_chunks
		ORDER BY array_distance(embedding, ?::FLOAT[1024]) LIMIT ?;
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

// Rerank simplifies the chunk selection by returning the top N chunks
// This is a simplified version that doesn't use LLM for reranking
// but simply filters the most relevant chunks based on the initial retrieval order
func (r *RAG) Rerank(ctx context.Context, query string, chunks []DocumentChunk, topN int) ([]DocumentChunk, error) {
	// Simply return the top N chunks from the initial retrieval
	// The chunks are already ordered by relevance from the vector search
	if len(chunks) <= topN {
		return chunks, nil
	}

	log.Info().Int("total_chunks", len(chunks)).Int("selected_chunks", topN).Msg("Filtering top chunks")
	return chunks[:topN], nil
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

// CalculateFileHash calculates xxh64 hash of a file
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := xxhash.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// GenerateDocumentID generates a unique document ID using xxh64 hash of file path + filename
func GenerateDocumentID(filePath string) string {
	h := xxhash.New()
	h.Write([]byte(filePath))
	pathHash := hex.EncodeToString(h.Sum(nil))
	fileName := filepath.Base(filePath)
	return pathHash + ":" + fileName
}

// IsFileProcessed checks if a file has been processed and if its hash matches
func (r *RAG) IsFileProcessed(filePath, currentHash string) (bool, error) {
	var storedHash string
	err := r.DB.QueryRow("SELECT file_hash FROM processed_files WHERE file_path = ?", filePath).Scan(&storedHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // File not processed yet
		}
		return false, err
	}
	return storedHash == currentHash, nil
}

// UpdateFileHash updates or inserts the file hash record
func (r *RAG) UpdateFileHash(filePath, fileHash string) error {
	_, err := r.DB.Exec(`INSERT INTO processed_files (file_path, file_hash, processed_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (file_path) DO UPDATE SET
			file_hash = EXCLUDED.file_hash,
			processed_at = EXCLUDED.processed_at`, filePath, fileHash)
	return err
}

// RemoveDocumentChunks removes all chunks for a specific document
func (r *RAG) RemoveDocumentChunks(documentID string) error {
	_, err := r.DB.Exec("DELETE FROM document_chunks WHERE document_id = ?", documentID)
	return err
}

// GetAllProcessedFiles returns all file paths from the processed_files table
func (r *RAG) GetAllProcessedFiles() ([]string, error) {
	rows, err := r.DB.Query("SELECT file_path FROM processed_files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filePaths []string
	for rows.Next() {
		var filePath string
		err := rows.Scan(&filePath)
		if err != nil {
			return nil, err
		}
		filePaths = append(filePaths, filePath)
	}
	return filePaths, nil
}

// RemoveFileRecord removes a file record and its associated document chunks
func (r *RAG) RemoveFileRecord(filePath string) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get document ID from file path using hash + filename
	documentID := GenerateDocumentID(filePath)

	// Remove document chunks
	_, err = tx.Exec("DELETE FROM document_chunks WHERE document_id = ?", documentID)
	if err != nil {
		return err
	}

	// Remove file record
	_, err = tx.Exec("DELETE FROM processed_files WHERE file_path = ?", filePath)
	if err != nil {
		return err
	}

	return tx.Commit()
}
