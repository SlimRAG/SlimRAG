package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var updateCmd = &cli.Command{
	Name:  "update",
	Usage: "Update documents and make embeddings",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "path"},
	},
	Flags: []cli.Flag{
		flagDSN,
		flagEmbeddingBaseURL,
		flagEmbeddingModel,
		flagEmbeddingDimension,
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Chunker configuration file path",
			Config:  trimSpace,
		},
		&cli.StringFlag{
			Name:    "strategy",
			Aliases: []string{"s"},
			Usage:   "Chunking strategy: fixed, semantic, sentence, adaptive",
			Value:   "adaptive",
			Config:  trimSpace,
		},
		&cli.IntFlag{
			Name:    "max-size",
			Aliases: []string{"max"},
			Usage:   "Maximum chunk size in characters",
			Value:   1000,
		},
		&cli.IntFlag{
			Name:    "min-size",
			Aliases: []string{"min"},
			Usage:   "Minimum chunk size in characters",
			Value:   100,
		},
		&cli.IntFlag{
			Name:  "overlap",
			Usage: "Overlap size in characters",
			Value: 50,
		},
		&cli.StringFlag{
			Name:    "language",
			Aliases: []string{"l"},
			Usage:   "Language: zh, en, auto",
			Value:   "auto",
			Config:  trimSpace,
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"j"},
			Usage:   "Number of workers for embedding computation",
			Value:   3,
		},
		&cli.BoolFlag{
			Name:  "force",
			Usage: "Force reprocess all files regardless of hash",
			Value: false,
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		path, err := getArgumentPath(command)
		if err != nil {
			return err
		}

		dsn := command.String("dsn")
		baseURL := command.String("embedding-base-url")
		embeddingModel := command.String("embedding-model")
		embeddingDimensions := command.Int64("embedding-dimension")
		configPath := command.String("config")
		strategy := command.String("strategy")
		maxSize := command.Int("max-size")
		minSize := command.Int("min-size")
		overlap := command.Int("overlap")
		language := command.String("language")
		workers := command.Int("workers")
		force := command.Bool("force")

		// Open database
		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}
		defer db.Close()

		// Create RAG instance
		embeddingClient := openai.NewClient(option.WithBaseURL(baseURL))
		r := rag.RAG{
			DB:                  db,
			EmbeddingClient:     &embeddingClient,
			EmbeddingModel:      embeddingModel,
			EmbeddingDimensions: embeddingDimensions,
		}

		// Create chunking config
		var config *rag.ChunkingConfig
		if configPath != "" {
			config, err = rag.LoadChunkingConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load chunking config: %w", err)
			}
		} else {
			config = &rag.ChunkingConfig{
				MaxChunkSize:        maxSize,
				MinChunkSize:        minSize,
				OverlapSize:         overlap,
				SentenceWindow:      3,
				Strategy:            strategy,
				Language:            language,
				PreserveSections:    true,
				SimilarityThreshold: 0.7,
			}
		}

		// Create document chunker
		chunker, err := rag.NewDocumentChunker(config)
		if err != nil {
			return fmt.Errorf("failed to create chunker: %w", err)
		}

		// Find all .md files
		var mdFiles []string
		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				mdFiles = append(mdFiles, filePath)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}

		log.Info().Int("total_files", len(mdFiles)).Msg("Found markdown files")

		// Clean up deleted files from database
		var removedCount int
		processedFiles, err := r.GetAllProcessedFiles()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get processed files")
		} else {
			for _, processedFile := range processedFiles {
				if _, err := os.Stat(processedFile); os.IsNotExist(err) {
					err = r.RemoveFileRecord(processedFile)
					if err != nil {
						log.Error().Err(err).Str("file", processedFile).Msg("Failed to remove deleted file record")
					} else {
						removedCount++
						log.Info().Str("file", processedFile).Msg("Removed deleted file from database")
					}
				}
			}
			if removedCount > 0 {
				log.Info().Int("removed", removedCount).Msg("Cleaned up deleted files")
			}
		}

		// Process files
		var processedCount, skippedCount int
		bar := progressbar.Default(int64(len(mdFiles)))

		for _, filePath := range mdFiles {
			// Calculate file hash
			currentHash, err := rag.CalculateFileHash(filePath)
			if err != nil {
				log.Error().Err(err).Str("file", filePath).Msg("Failed to calculate file hash")
				continue
			}

			// Check if file needs processing
			if !force {
				alreadyProcessed, err := r.IsFileProcessed(filePath, currentHash)
				if err != nil {
					log.Error().Err(err).Str("file", filePath).Msg("Failed to check file processing status")
					continue
				}
				if alreadyProcessed {
					skippedCount++
					bar.Add(1)
					continue
				}
			}

			// Read and process file
			content, err := os.ReadFile(filePath)
			if err != nil {
				log.Error().Err(err).Str("file", filePath).Msg("Failed to read file")
				continue
			}

			// Chunk document
			fileName := filepath.Base(filePath)
			doc, err := chunker.ChunkDocument(string(content), fileName)
			if err != nil {
				log.Error().Err(err).Str("file", filePath).Msg("Failed to chunk document")
				continue
			}

			// Remove old chunks for this document if it was previously processed
			documentID := rag.GenerateDocumentID(filePath)
			err = r.RemoveDocumentChunks(documentID)
			if err != nil {
				log.Error().Err(err).Str("document_id", documentID).Msg("Failed to remove old chunks")
				// Continue anyway, as this might be a new document
			}

			// Insert new chunks
			err = r.UpsertDocumentChunks(doc)
			if err != nil {
				log.Error().Err(err).Str("file", filePath).Msg("Failed to upsert document chunks")
				continue
			}

			// Update file hash record
			err = r.UpdateFileHash(filePath, currentHash)
			if err != nil {
				log.Error().Err(err).Str("file", filePath).Msg("Failed to update file hash")
				// Continue anyway, as the chunks are already inserted
			}

			processedCount++
			_ = bar.Add(1)
			log.Info().Str("file", filePath).Int("chunks", len(doc.Chunks)).Msg("Processed file")
		}

		_ = bar.Finish()
		log.Info().Int("processed", processedCount).Int("skipped", skippedCount).Msg("File processing completed")

		// Compute embeddings for new chunks
		if processedCount > 0 {
			log.Info().Msg("Computing embeddings for new chunks")
			err = r.ComputeEmbeddings(ctx, true, workers) // Only compute for empty embeddings
			if err != nil {
				return fmt.Errorf("failed to compute embeddings: %w", err)
			}
			log.Info().Msg("Embedding computation completed")
		}

		return nil
	},
}
