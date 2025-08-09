package main

import (
	"os"
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

var flagEmbeddingDimension = &cli.IntFlag{
	Name:    "embedding-dimension",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_EMBEDDING_DIMENSION")),
	Value:   1024,
}

var flagAssistantBaseURL = &cli.StringFlag{
	Name:    "assistant-base-url",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_ASSISTANT_BASE_URL")),
}

var flagAssistantModel = &cli.StringFlag{
	Name:    "assistant-model",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_ASSISTANT_MODEL")),
}

var flagAssistantAPIKey = &cli.StringFlag{
	Name:    "assistant-api-key",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_ASSISTANT_API_KEY")),
}

var flagTrace = &cli.BoolFlag{
	Name:    "trace",
	Usage:   "Enable OpenAI API call tracing for debugging",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_TRACE")),
}

var flagAuditLogDir = &cli.StringFlag{
	Name:    "audit-log-dir",
	Usage:   "Directory to store API call audit logs",
	Value:   "./audit_logs",
	Sources: cli.NewValueSourceChain(cli.EnvVar("RAG_AUDIT_LOG_DIR")),
}

func getArgumentQuery(command *cli.Command) (string, error) {
	query := command.StringArg("query")
	if query == "" {
		cli.SubcommandHelpTemplate = strings.Replace(cli.SubcommandHelpTemplate,
			"[arguments...]", "[QUERY]", 1)
		_ = cli.ShowSubcommandHelp(command)
		return "", errors.New("query is required")
	}
	_, err := os.Stat(query)
	if err != nil {
		return query, nil
	}
	var queryBuf []byte
	queryBuf, err = os.ReadFile(query)
	if err != nil {
		return query, nil
	}
	return string(queryBuf), nil
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
