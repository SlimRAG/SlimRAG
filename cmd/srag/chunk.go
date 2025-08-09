package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	rag "github.com/fanyang89/rag/v1"
)

var chunkCmd = &cli.Command{
	Name:    "chunk",
	Usage:   "Chunk markdown files using Go native chunker",
	Aliases: []string{"c"},
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "input", Config: trimSpace},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output JSON file path (default: input.chunks.json)",
			Config:  trimSpace,
		},
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
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		inputPath := command.StringArg("input")
		if inputPath == "" {
			return fmt.Errorf("input file path is required")
		}

		// 获取输出路径
		outputPath := command.String("output")
		if outputPath == "" {
			outputPath = inputPath + ".chunks.json"
		}

		// 获取配置文件路径
		configPath := command.String("config")

		// 如果没有指定配置文件，使用命令行参数创建临时配置
		if configPath == "" {
			config := &rag.ChunkingConfig{
				MaxChunkSize:        command.Int("max-size"),
				MinChunkSize:        command.Int("min-size"),
				OverlapSize:         command.Int("overlap"),
				SentenceWindow:      3,
				Strategy:            command.String("strategy"),
				Language:            command.String("language"),
				PreserveSections:    true,
				SimilarityThreshold: 0.7,
			}

			// 创建分块器
			chunker, err := rag.NewDocumentChunker(config, "")
			if err != nil {
				return fmt.Errorf("failed to create chunker: %w", err)
			}

			// 读取并分块文档
			content, err := os.ReadFile(inputPath)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}

			fileName := filepath.Base(inputPath)
			doc, err := chunker.ChunkDocument(string(content), fileName)
			if err != nil {
				return fmt.Errorf("failed to chunk document: %w", err)
			}
			doc.FilePath = inputPath
			doc.Fix()

			// 保存结果
			data, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}

			err = os.WriteFile(outputPath, data, 0644)
			if err != nil {
				return fmt.Errorf("failed to save output: %w", err)
			}

			fmt.Printf("Successfully chunked document: %s -> %s (%d chunks)\n", inputPath, outputPath, len(doc.Chunks))
			return nil
		}

		// 使用配置文件
		err := rag.ChunkMarkdownFile(inputPath, configPath, outputPath)
		if err != nil {
			return fmt.Errorf("failed to chunk document: %w", err)
		}

		fmt.Printf("Successfully chunked document: %s -> %s\n", inputPath, outputPath)
		return nil
	},
}
