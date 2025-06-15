package rag

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/fioepq9/pzlog"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = zerolog.New(pzlog.NewPtermWriter()).With().Timestamp().Caller().Stack().Logger()

	err := godotenv.Load(".env")
	if err != nil {
		log.Warn().Err(err).Msg(".env file not found")
	}

	rc := m.Run()
	os.Exit(rc)
}
