package rag

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/minio/minio-go/v7"
	"github.com/openai/openai-go"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/sourcegraph/conc/pool"
)

type RAG struct {
	DB                  *sql.DB
	OSS                 *minio.Client
	EmbeddingClient     interface{}
	EmbeddingModel      string
	EmbeddingDimensions int64
	AssistantClient     interface{}
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

func (r *RAG) ComputeEmbeddings(ctx context.Context, onlyEmpty bool, workers int, callback func()) error {
	var err error
	var rows *sql.Rows
	if onlyEmpty {
		rows, err = r.DB.QueryContext(ctx,
			"SELECT id, text, embedding FROM document_chunks WHERE embedding IS NULL")
	} else {
		rows, err = r.DB.QueryContext(ctx,
			"SELECT id, text, embedding FROM document_chunks")
	}
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	bar := progressbar.Default(-1)
	bar.Describe("Computing embeddings")
	defer func() { _ = bar.Finish() }()

	p := pool.New().WithMaxGoroutines(workers)
	var wg sync.WaitGroup // wait for all jobs finished

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
			embeddingClient := ToEmbeddingClient(r.EmbeddingClient)
			if embeddingClient == nil {
				log.Error().Msg("Failed to get embedding client")
				return
			}
			rsp, err = embeddingClient.New(ctx, openai.EmbeddingNewParams{
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
			} else {
				callback()
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
	embeddingClient := ToEmbeddingClient(r.EmbeddingClient)
	if embeddingClient == nil {
		return nil, errors.New("failed to get embedding client")
	}

	rsp, err := embeddingClient.New(ctx, openai.EmbeddingNewParams{
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

	rows, err := r.DB.QueryContext(ctx, fmt.Sprintf(`SELECT id, document_id, file_path, text, embedding FROM document_chunks
		ORDER BY array_distance(embedding, ?::FLOAT[%d]) LIMIT ?;
	`, r.EmbeddingDimensions), queryEmbedding, limit)
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

// Rerank uses LLM to select the most relevant document chunks for the query
func (r *RAG) Rerank(ctx context.Context, query string, chunks []DocumentChunk, selectedLimit int) ([]DocumentChunk, error) {
	if len(chunks) <= selectedLimit {
		return chunks, nil
	}

	// Build LLM selection prompt
	selectionPrompt := r.buildSelectionPrompt(query, chunks, selectedLimit)

	chatClient := ToChatClient(r.AssistantClient)
	if chatClient == nil {
		return nil, errors.New("failed to get chat client")
	}

	c, err := chatClient.Completions().New(ctx, openai.ChatCompletionNewParams{
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

	// Parse LLM returned index selection
	selectedIndices, err := r.parseSelectedIndices(string(c.Choices[0].Message.Content), len(chunks))
	if err != nil {
		return nil, err
	}

	// Return document chunks based on selected indices
	var selectedChunks []DocumentChunk
	for _, idx := range selectedIndices {
		if idx < len(chunks) {
			selectedChunks = append(selectedChunks, chunks[idx])
		}
	}

	log.Info().Int("total_chunks", len(chunks)).Int("selected_chunks", len(selectedChunks)).Msg("LLM-based chunk selection completed")
	return selectedChunks, nil
}

// buildSelectionPrompt builds the prompt for LLM to select document chunks
func (r *RAG) buildSelectionPrompt(query string, chunks []DocumentChunk, selectedLimit int) string {
	var b strings.Builder
	b.WriteString("You are an intelligent document retrieval assistant. Please select the most relevant ")
	b.WriteString(fmt.Sprintf("%d", selectedLimit))
	b.WriteString(" chunks from the following document blocks. Please only return index numbers, one per line, sorted by relevance from highest to lowest.\n\n")
	b.WriteString("User query: ")
	b.WriteString(query)
	b.WriteString("\n\nDocument block list:\n\n")

	for i, chunk := range chunks {
		b.WriteString(fmt.Sprintf("[%d] %s\n\n", i, chunk.Text))
	}

	b.WriteString(fmt.Sprintf("\nPlease select the most relevant %d chunks, return only index numbers: ", selectedLimit))
	return b.String()
}

// parseSelectedIndices parses the index selection returned by LLM
func (r *RAG) parseSelectedIndices(content string, maxIndex int) ([]int, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var indices []int

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract numbers
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
	// Phase 1: Vector retrieval of large number of document chunks
	retrievedChunks, err := r.QueryDocumentChunks(ctx, p.Query, p.RetrievalLimit)
	if err != nil {
		return "", err
	}

	// Phase 2: LLM selects the most relevant document chunks
	selectedChunks, err := r.Rerank(ctx, p.Query, retrievedChunks, p.SelectedLimit)
	if err != nil {
		return "", err
	}

	// Phase 3: Generate final answer based on selected document chunks
	var prompt string
	if p.SystemPrompt != "" {
		prompt = BuildPromptWithSystem(p.Query, selectedChunks, p.SystemPrompt)
	} else {
		prompt = BuildPrompt(p.Query, selectedChunks)
	}

	chatClient := ToChatClient(r.AssistantClient)
	if chatClient == nil {
		return "", errors.New("failed to get chat client")
	}

	c, err := chatClient.Completions().New(ctx, openai.ChatCompletionNewParams{
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
		if os.IsNotExist(err) {
			return "", nil
		}
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

func (r *RAG) GetProcessedFileHash(filePath string) (string, error) {
	var storedHash string
	err := r.DB.
		QueryRow("SELECT file_hash FROM processed_files WHERE file_path = ?", filePath).
		Scan(&storedHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
		return "", err
	}
	return storedHash, nil
}

// IsFileProcessed checks if a file has been processed and if its hash matches
func (r *RAG) IsFileProcessed(filePath, currentHash string) (bool, error) {
	var storedHash string
	err := r.DB.QueryRow("SELECT file_hash FROM processed_files WHERE file_path = ?", filePath).Scan(&storedHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil // File not processed yet
		}
		return false, err
	}
	return storedHash == currentHash, nil
}

// UpdateProcessedFileHash updates or inserts the file hash record
func (r *RAG) UpdateProcessedFileHash(filePath, fileHash string) error {
	_, err := r.DB.Exec(`
		INSERT INTO processed_files (file_path, file_hash, processed_at)
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

func (r *RAG) RemoveDocumentChunksByFileHash(fileHash string) error {
	_, err := r.DB.Exec("DELETE FROM document_chunks WHERE file_hash = ?", fileHash)
	return err
}

func (r *RAG) RemoveDocumentChunksByFilePath(filePath string) error {
	// find last file hash
	storedHash, err := r.GetProcessedFileHash(filePath)
	if err != nil {
		return err
	}

	if storedHash != "" {
		err = r.RemoveDocumentChunksByFileHash(storedHash)
		if err != nil {
			return err
		}
	}

	_, err = r.DB.Exec("DELETE FROM processed_files WHERE file_hash = ?", storedHash)
	return err
}

// FindFilesToProcess returns all file paths from the processed_files table
// Delete non-exist files, update changed files
func (r *RAG) FindFilesToProcess(filePathListForNow []string, force bool) ([]FileInfo, error) {
	infos := make([]FileInfo, 0)
	if force {
		for _, filePath := range filePathListForNow {
			var h string
			h, err := CalculateFileHash(filePath)
			if err != nil {
				return nil, err
			}

			infos = append(infos, FileInfo{
				FilePath: filePath,
				FileName: filepath.Base(filePath),
				FileHash: h,
			})
		}
		return infos, nil
	}

	filesMap := make(map[string]struct{})
	for _, file := range filePathListForNow {
		filesMap[file] = struct{}{}
	}

	rows, err := r.DB.Query("SELECT file_path, file_name, file_hash FROM processed_files")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	toDeleteFilePathList := make([]string, 0)

	for rows.Next() {
		fileInfo := FileInfo{}
		err = rows.Scan(&fileInfo.FilePath, &fileInfo.FileName, &fileInfo.FileHash)
		if err != nil {
			return nil, err
		}

		processed := false
		_, processed = filesMap[fileInfo.FilePath]
		if !processed {
			infos = append(infos, fileInfo)
			continue
		}

		var h string
		h, err = CalculateFileHash(fileInfo.FilePath)
		if err != nil {
			return nil, err
		}
		if h == "" {
			// file deleted
			toDeleteFilePathList = append(toDeleteFilePathList, fileInfo.FilePath)
			continue
		}

		if h != fileInfo.FileHash {
			// file changed
			infos = append(infos, fileInfo)
		}
	}

	for _, filePath := range toDeleteFilePathList {
		err = r.RemoveDocumentChunksByFilePath(filePath)
		if err != nil {
			return nil, err
		}
	}

	return infos, nil
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
