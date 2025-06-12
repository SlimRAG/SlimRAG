package rag

import (
	_ "embed"
	"os"
	"testing"

	"github.com/fioepq9/pzlog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/suite"
)

type ChunkingTestSuite struct {
	suite.Suite
	c *Chunker
}

func TestChunking(t *testing.T) {
	suite.Run(t, new(ChunkingTestSuite))
}

func (s *ChunkingTestSuite) SetupTest() {
	var err error
	s.c, err = NewChunker()
	s.Require().NoError(err)
}

func (s *ChunkingTestSuite) TearDownTest() {
	err := s.c.Close()
	s.NoError(err)
}

func (s *ChunkingTestSuite) TestChunking() {
	for i := 0; i < 10; i++ {
		chunks, err := s.c.Split(`hello rag`)
		s.NoError(err)
		s.Len(chunks, 1)
	}
}

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = zerolog.New(pzlog.NewPtermWriter()).With().Timestamp().Caller().Stack().Logger()
	os.Exit(m.Run())
}
