package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gobwas/glob"
	"github.com/goccy/go-json"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/joho/godotenv"
	"github.com/negrel/assert"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v3"

	"github.com/fioepq9/pzlog"

	"github.com/fanyang89/rag/v1"
)

var cmd = &cli.Command{
	Name: "rag",
	Commands: []*cli.Command{
		serveCmd,
		scanCmd,
		computeCmd,
		askCmd,
		getChunkCmd,
		generateScriptCmd,
		cleanupCmd,
	},
}

var trimSpace = cli.StringConfig{TrimSpace: true}

var cleanupCmd = &cli.Command{
	Name: "cleanup",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "dsn",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_DSN")),
		},
		&cli.BoolFlag{
			Name:    "delete",
			Aliases: []string{"d"},
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		dsn := command.String("dsn")
		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}
		r := rag.RAG{DB: db}

		ids := make([]uint64, 0)

		tw := table.NewWriter()
		tw.AppendHeader(table.Row{"ID", "Raw document", "Text", "Embedding"})
		err = r.FindInvalidChunks(ctx, func(chunk *rag.DocumentChunk) {
			tw.AppendRow(table.Row{chunk.ID, chunk.RawDocument, chunk.Text, chunk.Embedding})
			ids = append(ids, chunk.ID)
		})
		if err != nil {
			return err
		}
		fmt.Println(tw.Render())

		if command.Bool("delete") {
			log.Info().Int("count", len(ids)).Msg("Deleting invalid chunks")
			for _, id := range ids {
				err = r.DeleteChunk(id)
				if err != nil {
					log.Error().Err(err).Uint64("chunk_id", id).Msgf("Delete chunk failed")
				}
			}
		}

		return nil
	},
}

var generateScriptCmd = &cli.Command{
	Name:    "generate-script",
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

var serveCmd = &cli.Command{
	Name:  "serve",
	Usage: "start rag server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "bind",
			Aliases: []string{"a", "l"},
			Value:   ":5000",
		},
		&cli.StringFlag{
			Name:    "dsn",
			Sources: cli.ValueSourceChain{Chain: []cli.ValueSource{cli.EnvVar("RAG_DSN")}},
		},
		&cli.StringFlag{
			Name:    "base_url",
			Sources: cli.ValueSourceChain{Chain: []cli.ValueSource{cli.EnvVar("EMBEDDING_BASE_URL")}},
		},
		&cli.StringFlag{
			Name:    "model",
			Sources: cli.ValueSourceChain{Chain: []cli.ValueSource{cli.EnvVar("EMBEDDING_MODEL")}},
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		baseURL := command.String("base_url")
		model := command.String("model")
		dsn := command.String("dsn")

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		client := openai.NewClient(option.WithBaseURL(baseURL))
		r := &rag.RAG{DB: db, EmbeddingClient: &client, EmbeddingModel: model}

		s := rag.NewServer(r)
		go func() {
			select {
			case <-ctx.Done():
				closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = s.Shutdown(closeCtx)
			}
		}()
		err = s.Start(command.String("bind"))
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	},
}

var scanCmd = &cli.Command{
	Name:  "scan",
	Usage: "Scan directories for files and upsert them into the database",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "path"},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "glob",
			Aliases: []string{"g"},
			Value:   "*.md.chunks.json",
		},
		&cli.StringFlag{
			Name: "dsn",
			Sources: cli.ValueSourceChain{
				Chain: []cli.ValueSource{
					cli.EnvVar("RAG_DSN"),
				},
			},
		},
		&cli.BoolFlag{
			Name: "dry-run",
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		dryRun := command.Bool("dry-run")
		dsn := command.String("dsn")
		if dsn == "" {
			return errors.New("dsn is required")
		}
		path := command.StringArg("path")
		if path == "" {
			return errors.New("path is required")
		}

		g, err := glob.Compile(command.String("glob"))
		if err != nil {
			return err
		}

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		r := rag.RAG{DB: db}

		pathList := make([]string, 0)
		err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && g.Match(d.Name()) {
				pathList = append(pathList, path)
			}
			return nil
		})

		bar := progressbar.New(len(pathList))
		bar.Describe("Uploading chunks")
		defer func() { _ = bar.Finish() }()

		for _, path := range pathList {
			_ = bar.Add(1)

			buf, err := os.ReadFile(path)
			if err != nil {
				log.Error().Err(err).Stack().Str("path", path).Msg("Read file")
				continue
			}

			decoder := json.NewDecoder(bytes.NewReader(buf))
			//decoder.DisallowUnknownFields()
			var chunks rag.Document
			err = decoder.Decode(&chunks)
			if err != nil {
				log.Error().Err(err).Stack().Str("path", path).Msg("Decode")
				continue
			}
			chunks.Fix()

			if dryRun {
				log.Info().Str("path", path).Msg("Skipped chunks uploading due to dry-run")
				continue
			}

			err = r.UpsertDocumentChunks(&chunks)
			if err != nil {
				log.Error().Err(err).Stack().Str("path", path).Msg("Upsert chunks")
			}
		}

		return nil
	},
}

var computeCmd = &cli.Command{
	Name:  "compute",
	Usage: "Compute embeddings for files in the database",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "dsn",
			Sources: cli.ValueSourceChain{Chain: []cli.ValueSource{cli.EnvVar("RAG_DSN")}},
		},
		&cli.StringFlag{
			Name:    "base_url",
			Sources: cli.ValueSourceChain{Chain: []cli.ValueSource{cli.EnvVar("EMBEDDING_BASE_URL")}},
		},
		&cli.StringFlag{
			Name:    "model",
			Sources: cli.ValueSourceChain{Chain: []cli.ValueSource{cli.EnvVar("EMBEDDING_MODEL")}},
		},
		&cli.BoolFlag{
			Name:  "force",
			Value: false,
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"j"},
			Value:   3,
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		baseURL := command.String("base_url")
		model := command.String("model")
		dsn := command.String("dsn")
		force := command.Bool("force")
		workers := command.Int("workers")

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		client := openai.NewClient(option.WithBaseURL(baseURL))
		r := rag.RAG{DB: db, EmbeddingClient: &client, EmbeddingModel: model}

		return r.ComputeEmbeddings(ctx, !force, workers)
	},
}

var askCmd = &cli.Command{
	Name:  "ask",
	Usage: "Search documents and ask the LLM",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "query", Config: cli.StringConfig{TrimSpace: true}},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "dsn",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_DSN")),
		},
		&cli.StringFlag{
			Name:    "base_url",
			Sources: cli.NewValueSourceChain(cli.EnvVar("EMBEDDING_BASE_URL")),
		},
		&cli.StringFlag{
			Name:    "model",
			Sources: cli.NewValueSourceChain(cli.EnvVar("EMBEDDING_MODEL")),
		},
		&cli.StringFlag{
			Name:    "reranker_base_url",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RERANKER_BASE_URL")),
		},
		&cli.StringFlag{
			Name:    "reranker_model",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RERANKER_MODEL")),
		},
		&cli.IntFlag{
			Name:  "limit",
			Value: 10,
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		query := command.StringArg("query")
		if query == "" {
			return errors.New("query is required")
		}

		baseURL := command.String("base_url")
		model := command.String("model")
		rerankerBaseURL := command.String("reranker_base_url")
		rerankerModel := command.String("reranker_model")
		dsn := command.String("dsn")
		limit := command.Int("limit")

		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		client := openai.NewClient(option.WithBaseURL(baseURL))
		rerankerClient := rag.NewInfinityClient(rerankerBaseURL)
		r := rag.RAG{
			DB:              db,
			EmbeddingClient: &client,
			EmbeddingModel:  model,
			RerankerClient:  rerankerClient,
			RerankerModel:   rerankerModel,
		}

		chunks, err := r.QueryDocumentChunks(ctx, query, limit)
		if err != nil {
			return err
		}

		chunks, err = r.Rerank(query, chunks, limit)
		if err != nil {
			return err
		}

		tw := table.NewWriter()
		tw.AppendHeader(table.Row{"ID", "Raw document"})
		for _, chunk := range chunks {
			tw.AppendRow(table.Row{chunk.ID, chunk.Document})
		}
		fmt.Println(tw.Render())

		answer, err := r.Ask(ctx, query, chunks)
		if err != nil {
			return err
		}
		fmt.Println(answer)
		return nil
	},
}

var getChunkCmd = &cli.Command{
	Name: "get",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "id", Config: cli.StringConfig{TrimSpace: true}},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "dsn",
			Sources: cli.ValueSourceChain{Chain: []cli.ValueSource{cli.EnvVar("RAG_DSN")}},
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		id := command.StringArg("id")
		if id == "" {
			return errors.New("id is required")
		}
		dsn := command.String("dsn")
		db, err := rag.OpenDB(dsn)
		if err != nil {
			return err
		}

		r := rag.RAG{DB: db}
		c, err := r.GetDocumentChunk(id)
		if err != nil {
			return err
		}
		fmt.Printf("id=%v document='%s' raw_document='%s'\n", c.ID, c.Document, c.RawDocument)
		fmt.Println(c.Text)
		return nil
	},
}

func main() {
	_ = godotenv.Load(".env")

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = zerolog.New(pzlog.NewPtermWriter()).With().Timestamp().Caller().Stack().Logger()

	//ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	//defer stop()

	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		log.Error().Err(err).Msg("Unexpected error")
	}
}
