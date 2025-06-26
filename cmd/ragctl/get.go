package main

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

var getChunkCmd = &cli.Command{
	Name:  "get",
	Usage: "Get document chunk by ID",
	Arguments: []cli.Argument{
		&cli.StringArg{Name: "id", Config: trimSpace},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "dsn",
			Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_DSN")),
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
