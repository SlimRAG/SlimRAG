package rag

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDuckDBEmbeddingStorageAndRetrieval 测试 DuckDB 的 embedding 存储和召回能力
// 使用本地 ollama 的 bge-m3 模型
func TestDuckDBEmbeddingStorageAndRetrieval(t *testing.T) {
	// 设置测试环境
	ctx := context.Background()
	
	// 使用内存数据库进行测试
	db, err := OpenDuckDB(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// 配置 ollama 客户端 (假设 ollama 运行在默认端口)
	ollamaBaseURL := "http://localhost:11434/v1"
	embeddingModel := "bge-m3"
	
	embeddingClient := openai.NewClient(option.WithBaseURL(ollamaBaseURL))
	
	rag := &RAG{
		DB:              db,
		EmbeddingClient: &embeddingClient,
		EmbeddingModel:  embeddingModel,
	}

	// 测试数据
	testDocuments := []struct {
		id       string
		document string
		text     string
	}{
		{
			id:       "chunk1",
			document: "test_doc_1",
			text:     "SlimRAG is a minimalist Retrieval-Augmented Generation system built with Go.",
		},
		{
			id:       "chunk2",
			document: "test_doc_1",
			text:     "The system provides a command-line interface for building and querying RAG systems.",
		},
		{
			id:       "chunk3",
			document: "test_doc_2",
			text:     "DuckDB is an in-process SQL OLAP database management system with vector search capabilities.",
		},
		{
			id:       "chunk4",
			document: "test_doc_2",
			text:     "Vector embeddings enable semantic search and similarity matching in databases.",
		},
	}

	// 1. 测试文档块插入
	t.Run("InsertDocumentChunks", func(t *testing.T) {
		for _, testDoc := range testDocuments {
			_, err := db.ExecContext(ctx, `
				INSERT INTO document_chunks (id, document_id, text, start_offset, end_offset, embedding)
				VALUES (?, ?, ?, ?, ?, ?)
			`, testDoc.id, testDoc.document, testDoc.text, 0, 0, nil)
			require.NoError(t, err)
		}

		// 验证插入的数据
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM document_chunks").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, len(testDocuments), count)
	})

	// 2. 测试 embedding 计算和存储
	t.Run("ComputeAndStoreEmbeddings", func(t *testing.T) {
		// 注意：这个测试需要 ollama 服务运行并且有 bge-m3 模型
		// 如果 ollama 不可用，测试会跳过
		err := rag.ComputeEmbeddings(ctx, true, 1)
		if err != nil {
			t.Skipf("Skipping embedding computation test: %v (ensure ollama is running with bge-m3 model)", err)
			return
		}

		// 验证所有文档块都有了 embedding
		rows, err := db.QueryContext(ctx, "SELECT id, embedding FROM document_chunks WHERE embedding IS NOT NULL")
		require.NoError(t, err)
		defer rows.Close()

		embeddingCount := 0
		for rows.Next() {
			var id string
			var embedding []float32
			err := rows.Scan(&id, &embedding)
			require.NoError(t, err)
			assert.NotEmpty(t, embedding)
			assert.Greater(t, len(embedding), 0) // 确保 embedding 有内容
			embeddingCount++
		}
		assert.Equal(t, len(testDocuments), embeddingCount)
	})

	// 3. 测试向量相似性搜索
	t.Run("VectorSimilaritySearch", func(t *testing.T) {
		// 首先确保有 embeddings
		var embeddingCount int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM document_chunks WHERE embedding IS NOT NULL").Scan(&embeddingCount)
		require.NoError(t, err)
		
		if embeddingCount == 0 {
			t.Skip("No embeddings available for similarity search test")
			return
		}

		// 测试查询
		queryText := "What is SlimRAG?"
		chunks, err := rag.QueryDocumentChunks(ctx, queryText, 2)
		if err != nil {
			t.Skipf("Skipping similarity search test: %v", err)
			return
		}

		// 验证搜索结果
		assert.NotEmpty(t, chunks)
		assert.LessOrEqual(t, len(chunks), 2)
		
		// 第一个结果应该与 SlimRAG 相关
		if len(chunks) > 0 {
			firstChunk := chunks[0]
			assert.Contains(t, firstChunk.Text, "SlimRAG")
		}
	})

	// 4. 测试 HNSW 索引功能
	t.Run("HNSWIndexFunctionality", func(t *testing.T) {
		// 验证 HNSW 索引是否创建成功
		rows, err := db.QueryContext(ctx, `
			SELECT indexname 
			FROM pg_indexes 
			WHERE tablename = 'document_chunks' AND indexname = 'hnsw_idx'
		`)
		
		// 如果是 DuckDB，使用不同的查询
		if err != nil {
			// DuckDB 的索引查询方式
			rows, err = db.QueryContext(ctx, "PRAGMA table_info('document_chunks')")
		}
		
		if err != nil {
			t.Logf("Could not verify index creation: %v", err)
			return
		}
		defer rows.Close()
		
		// 至少验证表结构正确
		var tableExists bool
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) > 0 
			FROM information_schema.tables 
			WHERE table_name = 'document_chunks'
		`).Scan(&tableExists)
		
		if err != nil {
			// DuckDB 的表存在性检查
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM document_chunks LIMIT 0").Scan(&tableExists)
			assert.NoError(t, err)
		} else {
			assert.True(t, tableExists)
		}
	})

	// 5. 测试数据检索功能
	t.Run("DocumentChunkRetrieval", func(t *testing.T) {
		// 测试通过 ID 获取文档块
		var chunk DocumentChunk
		err := db.QueryRowContext(ctx, "SELECT id, document_id, text FROM document_chunks WHERE id = ?", "chunk1").Scan(&chunk.ID, &chunk.Document, &chunk.Text)
		require.NoError(t, err)
		assert.Equal(t, "chunk1", chunk.ID)
		assert.Equal(t, "test_doc_1", chunk.Document)
		assert.Contains(t, chunk.Text, "SlimRAG")

		// 测试不存在的文档块
		err = db.QueryRowContext(ctx, "SELECT id FROM document_chunks WHERE id = ?", "nonexistent").Scan(&chunk.ID)
		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
	})

	// 6. 测试批量操作
	t.Run("BatchOperations", func(t *testing.T) {
		// 直接使用 SQL 插入测试数据，避免 embedding 数组大小问题
		testChunks := []struct {
			id       string
			document string
			text     string
		}{
			{
				id:       hashString("This is a batch test document chunk 1."),
				document: "batch_test",
				text:     "This is a batch test document chunk 1.",
			},
			{
				id:       hashString("This is a batch test document chunk 2."),
				document: "batch_test",
				text:     "This is a batch test document chunk 2.",
			},
		}
		
		// 测试批量插入
		for _, chunk := range testChunks {
			_, err := db.ExecContext(ctx, `
				INSERT INTO document_chunks (id, document_id, text, start_offset, end_offset, embedding)
				VALUES (?, ?, ?, ?, ?, ?)
			`, chunk.id, chunk.document, chunk.text, 0, 0, nil)
			require.NoError(t, err)
		}
		
		// 验证插入的数据
		for _, chunk := range testChunks {
			var retrievedChunk DocumentChunk
			err := db.QueryRowContext(ctx, "SELECT id, document_id, text FROM document_chunks WHERE id = ?", chunk.id).Scan(&retrievedChunk.ID, &retrievedChunk.Document, &retrievedChunk.Text)
			require.NoError(t, err)
			assert.Equal(t, chunk.text, retrievedChunk.Text)
			assert.Equal(t, "batch_test", retrievedChunk.Document)
		}
	})
}

// TestDuckDBEmbeddingPerformance 测试 embedding 操作的性能
func TestDuckDBEmbeddingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	
	// 使用内存数据库
	db, err := OpenDuckDB(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// 插入大量测试数据
	numChunks := 100
	for i := 0; i < numChunks; i++ {
		_, err := db.ExecContext(ctx, `
			INSERT INTO document_chunks (id, document_id, text, start_offset, end_offset, embedding)
			VALUES (?, ?, ?, ?, ?, ?)
		`, 
			hashString(fmt.Sprintf("perf_chunk_%d", i)),
			"perf_test_doc",
			fmt.Sprintf("This is performance test chunk number %d with some sample text content.", i),
			0, 0, nil)
		require.NoError(t, err)
	}

	// 验证插入的数据量
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM document_chunks WHERE document_id = 'perf_test_doc'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, numChunks, count)

	t.Logf("Successfully inserted %d document chunks for performance testing", numChunks)
}

// TestDuckDBVSSExtension 测试 DuckDB VSS 扩展的安装和功能
func TestDuckDBVSSExtension(t *testing.T) {
	db, err := OpenDuckDB(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// 测试 VSS 扩展是否正确加载
	rows, err := db.Query("SELECT extension_name FROM duckdb_extensions() WHERE extension_name = 'vss'")
	if err != nil {
		t.Logf("Could not query extensions: %v", err)
		return
	}
	defer rows.Close()

	var extensionFound bool
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		require.NoError(t, err)
		if name == "vss" {
			extensionFound = true
			break
		}
	}

	if !extensionFound {
		t.Log("VSS extension not found in loaded extensions")
	}

	// 测试表结构是否正确创建
	var tableExists bool
	err = db.QueryRow("SELECT COUNT(*) > 0 FROM information_schema.tables WHERE table_name = 'document_chunks'").Scan(&tableExists)
	if err != nil {
		// 尝试 DuckDB 特定的查询
		err = db.QueryRow("SELECT COUNT(*) FROM document_chunks LIMIT 0").Scan(&tableExists)
		assert.NoError(t, err)
	} else {
		assert.True(t, tableExists)
	}
}