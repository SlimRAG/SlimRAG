package main

import (
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

var flagDSN = &cli.StringFlag{
	Name:    "dsn",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_DSN")),
}

var flagEmbeddingBaseURL = &cli.StringFlag{
	Name:    "embedding-base-url",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_EMBEDDING_BASE_URL")),
}

var flagEmbeddingModel = &cli.StringFlag{
	Name:    "embedding-model",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_EMBEDDING_MODEL")),
}

var flagEmbeddingDimension = &cli.StringFlag{
	Name:    "embedding-dimension",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_EMBEDDING_DIMENSION")),
}

var flagAssistantBaseURL = &cli.StringFlag{
	Name:    "assistant-base-url",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_ASSISTANT_BASE_URL")),
}

var flagAssistantModel = &cli.StringFlag{
	Name:    "assistant-model",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_ASSISTANT_MODEL")),
}

func getArgumentQuery(command *cli.Command) (string, error) {
	query := command.StringArg("query")
	if query == "" {
		cli.SubcommandHelpTemplate = strings.Replace(cli.SubcommandHelpTemplate,
			"[arguments...]", "[QUERY]", 1)
		_ = cli.ShowSubcommandHelp(command)
		return "", errors.New("query is required")
	}
	return query, nil
}

func getArgumentPath(command *cli.Command) (string, error) {
	path := command.StringArg("path")
	if path == "" {
		cli.SubcommandHelpTemplate = strings.Replace(cli.SubcommandHelpTemplate,
			"[arguments...]", "[PATH]", 1)
		_ = cli.ShowSubcommandHelp(command)
		return "", errors.New("path is required")
	}
	return path, nil
}
