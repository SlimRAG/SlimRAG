package main

import (
	"context"
	"fmt"

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
		_, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}

		// TODO: Implement cleanup logic without FindInvalidChunks and DeleteChunk
		fmt.Println("Cleanup command is not yet implemented.")

		return nil
	},
}
