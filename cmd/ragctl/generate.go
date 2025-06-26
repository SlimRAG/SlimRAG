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
)

var generateScriptCmd = &cli.Command{
	Name:    "generate-script",
	Usage:   "Generate .md from PDFs",
	Aliases: []string{"gen", "generate"},
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "path", Config: trimSpace},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "rag-tools", Aliases: []string{"t", "tools"}, Config: trimSpace},
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "-", Config: trimSpace},
		&cli.BoolFlag{Name: "chunking", Value: true},
		&cli.BoolFlag{Name: "mineru", Value: true},
		&cli.StringFlag{Name: "chunking-recipe"},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		var err error

		path := command.StringArg("path")
		if path == "" {
			return errors.New("path is required")
		}
		toolPath := command.String("rag-tools")
		chunkingRecipe := command.String("chunking-recipe")
		if chunkingRecipe == "" {
			chunkingRecipe = filepath.Join(toolPath, "chonkie-recipes", "default_zh.json")
		}

		var w io.Writer
		if outputPath := command.String("output"); outputPath == "-" {
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
				if command.Bool("mineru") {
					_, err = fmt.Fprintf(w, "pueue add -- \"uv run%smineru --source modelscope -p '%s' -o '%s'\"\n",
						toolArg, path, baseDir)
					assert.NoError(err)
				}
			}

			_, err = os.Stat(markdownFilePath)
			if err == nil && command.Bool("chunking") {
				outputPath := fmt.Sprintf("%s.chunks.json", markdownFilePath)
				_, err = os.Stat(outputPath)
				if err != nil {
					if !errors.Is(err, fs.ErrNotExist) {
						return err
					}
					_, err = fmt.Fprintf(w, "pueue add -- \"uv run%s%s chunking '%s'%s--output '%s'\"\n",
						toolArg, ragCliPath, markdownFilePath, recipeArg, outputPath)
					assert.NoError(err)
				}
			} else {
				log.Info().Str("path", path).Msg("Skipped chunking since the markdown doesn't exist")
			}

			return nil
		})
	},
}
