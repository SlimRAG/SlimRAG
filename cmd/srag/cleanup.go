package main

import (
	"context"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var cleanupCmd = &cli.Command{
	Name:  "cleanup",
	Usage: "Clean up invalid document chunks",
	Flags: []cli.Flag{
		flagDSN,
		&cli.BoolFlag{Name: "delete", Aliases: []string{"d"}},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		dsn := command.String("dsn")
		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}
		r := rag.RAG{DB: db}

		ids := make([]string, 0)

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
					log.Error().Err(err).Str("chunk_id", id).Msgf("Delete chunk failed")
				}
			}
		}

		return nil
	},
}
