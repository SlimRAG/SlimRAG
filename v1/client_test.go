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
	s.Require().True(len(baseURL) > 0)
	e := NewClient(baseURL, "Qwen/Qwen3-Embedding-0.6B")
	embeddings, err := e.GetEmbedding(context.TODO(), "hello world")
	s.Nil(err)
	s.True(len(embeddings) > 0)
}
