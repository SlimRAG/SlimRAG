package rag

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		INSERT INTO document_chunks (id, document_id, file_path, text, start_offset, end_offset)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			text = EXCLUDED.text,
			document_id = EXCLUDED.document_id,
			file_path = EXCLUDED.file_path;
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, chunk := range document.Chunks {
		_, err = stmt.Exec(
			chunk.ID,
			chunk.Document,
			chunk.FilePath,
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

	rows, err := r.DB.QueryContext(ctx, `SELECT id, document_id, file_path, text, embedding FROM document_chunks
		ORDER BY array_distance(embedding, ?::FLOAT[1024]) LIMIT ?;
	`, queryEmbedding, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var chunks []DocumentChunk
	for rows.Next() {
		var chunk DocumentChunk
		var embeddingInterface interface{}
		err = rows.Scan(&chunk.ID, &chunk.Document, &chunk.FilePath, &chunk.Text, &embeddingInterface)
		if err != nil {
			return nil, err
		}

		// Convert embedding from []interface{} to []float32
		if embeddingInterface != nil {
			if embeddingSlice, ok := embeddingInterface.([]interface{}); ok {
				chunk.Embedding = make([]float32, len(embeddingSlice))
				for i, v := range embeddingSlice {
					if f, ok := v.(float64); ok {
						chunk.Embedding[i] = float32(f)
					}
				}
			}
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

func (r *RAG) GetDocumentChunk(id string) (*DocumentChunk, error) {
	row := r.DB.QueryRow("SELECT id, document_id, file_path, text, embedding FROM document_chunks WHERE id = ?", id)

	var chunk DocumentChunk
	var embeddingInterface interface{}
	err := row.Scan(&chunk.ID, &chunk.Document, &chunk.FilePath, &chunk.Text, &embeddingInterface)
	if err != nil {
		return nil, err
	}

	// Convert embedding from []interface{} to []float32
	if embeddingInterface != nil {
		if embeddingSlice, ok := embeddingInterface.([]interface{}); ok {
			chunk.Embedding = make([]float32, len(embeddingSlice))
			for i, v := range embeddingSlice {
				if f, ok := v.(float64); ok {
					chunk.Embedding[i] = float32(f)
				}
			}
		}
	}

	return &chunk, nil
}

// Rerank 使用 LLM 来选择与查询最相关的文档块
func (r *RAG) Rerank(ctx context.Context, query string, chunks []DocumentChunk, selectedLimit int) ([]DocumentChunk, error) {
	if len(chunks) <= selectedLimit {
		return chunks, nil
	}

	// 构建 LLM 选择提示
	selectionPrompt := r.buildSelectionPrompt(query, chunks, selectedLimit)

	c, err := r.AssistantClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: r.AssistantModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(selectionPrompt),
		},
	})
	if err != nil {
		return nil, err
	}

	if len(c.Choices) == 0 {
		return nil, errors.New("no choices returned from LLM selection")
	}

	// 解析 LLM 返回的索引选择
	selectedIndices, err := r.parseSelectedIndices(string(c.Choices[0].Message.Content), len(chunks))
	if err != nil {
		return nil, err
	}

	// 根据选择的索引返回文档块
	var selectedChunks []DocumentChunk
	for _, idx := range selectedIndices {
		if idx < len(chunks) {
			selectedChunks = append(selectedChunks, chunks[idx])
		}
	}

	log.Info().Int("total_chunks", len(chunks)).Int("selected_chunks", len(selectedChunks)).Msg("LLM-based chunk selection completed")
	return selectedChunks, nil
}

// buildSelectionPrompt 构建用于 LLM 选择文档块的提示
func (r *RAG) buildSelectionPrompt(query string, chunks []DocumentChunk, selectedLimit int) string {
	var b strings.Builder
	b.WriteString("你是一个智能文档检索助手。请根据用户查询，从下面的文档块中选择最相关的 ")
	b.WriteString(fmt.Sprintf("%d", selectedLimit))
	b.WriteString(" 个块。请只返回索引编号，每行一个，按相关性从高到低排序。\n\n")
	b.WriteString("用户查询：")
	b.WriteString(query)
	b.WriteString("\n\n文档块列表：\n\n")

	for i, chunk := range chunks {
		b.WriteString(fmt.Sprintf("[%d] %s\n\n", i, chunk.Text))
	}

	b.WriteString(fmt.Sprintf("\n请选择最相关的 %d 个块，只返回索引编号：", selectedLimit))
	return b.String()
}

// parseSelectedIndices 解析 LLM 返回的索引选择
func (r *RAG) parseSelectedIndices(content string, maxIndex int) ([]int, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var indices []int

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 提取数字
		var idx int
		n, err := fmt.Sscanf(line, "%d", &idx)
		if err != nil || n != 1 {
			continue
		}

		if idx >= 0 && idx < maxIndex {
			indices = append(indices, idx)
		}
	}

	if len(indices) == 0 {
		return nil, errors.New("no valid indices found in LLM response")
	}

	return indices, nil
}

func (r *RAG) Ask(ctx context.Context, p *AskParameter) (string, error) {
	// 第一阶段：向量检索大量文档块
	retrievedChunks, err := r.QueryDocumentChunks(ctx, p.Query, p.RetrievalLimit)
	if err != nil {
		return "", err
	}

	// 第二阶段：LLM 选择最相关的文档块
	selectedChunks, err := r.Rerank(ctx, p.Query, retrievedChunks, p.SelectedLimit)
	if err != nil {
		return "", err
	}

	// 第三阶段：基于选择的文档块生成最终答案
	prompt := BuildPrompt(p.Query, selectedChunks)
	c, err := r.AssistantClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: r.AssistantModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", err
	}
	if len(c.Choices) > 0 {
		return string(c.Choices[0].Message.Content), nil
	}
	return "", errors.New("no choices returned from chat completion")
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
