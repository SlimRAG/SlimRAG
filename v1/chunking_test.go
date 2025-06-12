package rag

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ChunkingTestSuite struct {
	suite.Suite
}

func TestChunking(t *testing.T) {
	suite.Run(t, new(ChunkingTestSuite))
}

func (s *ChunkingTestSuite) TestChunking() {
	err := chunk()
	s.NoError(err)
}
