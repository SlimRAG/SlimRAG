package rag

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClientTestSuite struct {
	suite.Suite
}

func TestClient(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}

func (s *ClientTestSuite) TestGetEmbedding() {
	baseURL := os.Getenv("EMBEDDING_BASE_URL")
	model := os.Getenv("EMBEDDING_MODEL")
	s.Require().True(len(baseURL) > 0 && len(model) > 0, "EMBEDDING_BASE_URL and EMBEDDING_MODEL must be set")
	e := NewClient(ClientConfig{
		EmbeddingBaseURL: baseURL,
		EmbeddingModel:   model,
	})
	embeddings, err := e.GetEmbedding(context.TODO(), "hello world")
	s.Nil(err)
	s.True(len(embeddings) > 0)
}
