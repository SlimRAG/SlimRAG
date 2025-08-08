package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/negrel/assert"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	rag "github.com/fanyang89/rag/v1"
)

var generateCmd = &cli.Command{
	Name:    "generate",
	Usage:   "Generate .md from PDFs and split them into text chunks",
	Aliases: []string{"gen"},
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "path", Config: trimSpace},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "rag-tools",
			Aliases:  []string{"t", "tools"},
			Required: true,
			Config:   trimSpace,
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "-",
			Config:  trimSpace,
		},
		&cli.BoolFlag{
			Name:  "chonkie",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "mineru",
			Value: true,
		},
		&cli.StringFlag{
			Name: "chonkie-recipe",
		},
		&cli.BoolFlag{
			Name:  "use-go-chunker",
			Value: false,
			Usage: "Use Go native chunker instead of Chonkie",
		},
		&cli.StringFlag{
			Name:  "chunker-config",
			Usage: "Path to chunker configuration file",
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		path, err := getArgumentPath(command)
		if err != nil {
			return err
		}

		toolPath := command.String("rag-tools")
		chunkingRecipe := command.String("chonkie-recipe")
		if chunkingRecipe == "" {
			chunkingRecipe = filepath.Join(toolPath, "chonkie-recipes", "default_zh.json")
		}
		outputPath := command.String("output")
		enableMinerU := command.Bool("mineru")
		enableChonkie := command.Bool("chonkie")
		useGoChunker := command.Bool("use-go-chunker")
		chunkerConfig := command.String("chunker-config")

		var w io.Writer
		if outputPath == "-" {
			w = os.Stdout
		} else {
			f, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			w = f
		}

		_, err = fmt.Fprintln(w, "#!/usr/bin/env bash\ntrap 'exit' INT")
		if err != nil {
			return err
		}

		return filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".pdf") ||
				strings.HasSuffix(path, "_layout.pdf") ||
				strings.HasSuffix(path, "_origin.pdf") ||
				strings.HasSuffix(path, "_span.pdf") {
				return nil
			}

			path, err = filepath.Abs(path)
			if err != nil {
				panic(err)
			}
			baseDir := filepath.Dir(path)
			fileNameExt := filepath.Base(path)
			fileName := strings.TrimSuffix(fileNameExt, filepath.Ext(fileNameExt))
			markdownFilePath := filepath.Join(baseDir, fileName, "auto", fileName+".md")

			toolArg := " "
			recipeArg := " "
			ragCliPath := "rag.py"
			if toolPath != "" {
				toolArg = fmt.Sprintf(" --project %s ", toolPath)
				recipeArg = fmt.Sprintf(" -r '%s' ", chunkingRecipe)
				ragCliPath = filepath.Join(toolPath, "rag.py")
			}

			_, err = os.Stat(filepath.Join(baseDir, fileName))
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				if enableMinerU {
					_, err = fmt.Fprintf(w, "pueue add -- \"uv run%smineru --source modelscope -p '%s' -o '%s'\"\n",
						toolArg, path, baseDir)
					assert.NoError(err)
				}
			}

			_, err = os.Stat(markdownFilePath)
			if err == nil && (enableChonkie || useGoChunker) {
				outputPath := fmt.Sprintf("%s.chunks.json", markdownFilePath)
				_, err = os.Stat(outputPath)
				if err != nil {
					if !errors.Is(err, fs.ErrNotExist) {
						return err
					}
					if useGoChunker {
						// 使用Go原生分块器
						err = rag.ChunkMarkdownFile(markdownFilePath, chunkerConfig, outputPath)
						if err != nil {
							log.Error().Err(err).Str("path", markdownFilePath).Msg("Failed to chunk with Go chunker")
							return err
						}
						log.Info().Str("input", markdownFilePath).Str("output", outputPath).Msg("Chunked with Go chunker")
					} else {
						// 使用Chonkie
						_, err = fmt.Fprintf(w, "pueue add -- \"uv run%s%s chunking '%s'%s--output '%s'\"\n",
							toolArg, ragCliPath, markdownFilePath, recipeArg, outputPath)
						assert.NoError(err)
					}
				}
			} else {
				log.Info().Str("path", path).Msg("Skipped chunking since the markdown doesn't exist")
			}

			return nil
		})
	},
}
