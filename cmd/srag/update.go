package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gobwas/glob"
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
			Name:    "chunker-config",
			Aliases: []string{"c"},
			Usage:   "Chunker configuration file path",
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
		&cli.StringFlag{
			Name:    "glob",
			Aliases: []string{"g"},
			Usage:   "Glob pattern to filter files",
			Value:   "*.md",
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
		embeddingDimension := int64(command.Int("embedding-dimension"))
		chunkerConfigPath := command.String("chunker-config")
		workers := command.Int("workers")
		force := command.Bool("force")
		globStr := command.String("glob")
		if globStr == "" {
			globStr = "*.md"
			log.Warn().Str("glob", globStr).
				Msg("No glob pattern to filter files, default to *.md")
		}
		fileGlob := glob.MustCompile(globStr)

		// Open database
		db, err := rag.OpenDuckDB(dsn, embeddingDimension)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()

		// Validate or set embedding dimension
		embeddingDimension = rag.GetStoredEmbeddingDimension(db, embeddingDimension)

		// Create RAG instance
		embeddingClient := openai.NewClient(option.WithBaseURL(baseURL))
		r := rag.RAG{
			DB:                  db,
			EmbeddingClient:     &embeddingClient,
			EmbeddingModel:      embeddingModel,
			EmbeddingDimensions: embeddingDimension,
		}

		// Create chunking config
		var config *rag.ChunkingConfig
		config, err = rag.LoadChunkingConfig(chunkerConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load chunking config: %w", err)
		}

		// Create document chunker
		chunker, err := rag.NewDocumentChunker(config, path)
		if err != nil {
			return fmt.Errorf("failed to create chunker: %w", err)
		}

		// Find files to process
		var toProcessFiles []string
		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			if fileGlob.Match(info.Name()) {
				toProcessFiles = append(toProcessFiles, filePath)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}
		log.Info().Int("total_files", len(toProcessFiles)).Msg("Found markdown files")

		// find files to process
		filesToProcess, err := r.FindFilesToProcess(toProcessFiles, force)
		if err != nil {
			return err
		}

		// Process files
		bar := progressbar.Default(int64(len(toProcessFiles)))
		for _, fileInfo := range filesToProcess {
			filePath := fileInfo.FilePath

			// Chunk document
			doc, err := chunker.GetDocumentChunks(filePath)
			if err != nil {
				log.Error().Err(err).Str("file_path", filePath).
					Msg("Failed to chunk document")
				continue
			}

			// Insert new chunks
			err = r.UpsertDocumentChunks(doc)
			if err != nil {
				log.Error().Err(err).Str("file_path", filePath).
					Msg("Failed to upsert document chunks")
				continue
			}

			// Update file hash record
			err = r.UpdateProcessedFileHash(fileInfo.FilePath, fileInfo.FileHash)
			if err != nil {
				log.Error().Err(err).Str("file", filePath).
					Msg("Failed to update file hash")
				continue
			}

			_ = bar.Add(1)
			log.Info().Str("file", filePath).Int("chunks", len(doc.Chunks)).
				Msg("Processed file")
		}

		_ = bar.Finish()

		// Compute embeddings for new chunks
		log.Info().Msg("Computing embeddings for new chunks")
		err = r.ComputeEmbeddings(ctx, true, workers, func() {})
		if err != nil {
			return fmt.Errorf("failed to compute embeddings: %w", err)
		}
		log.Info().Msg("Embedding computation completed")

		return nil
	},
}
