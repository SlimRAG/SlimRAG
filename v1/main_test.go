package rag

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

func TestOpenOSS(t *testing.T) {
	endpoint := os.Getenv("OSS_ENDPOINT")
	accessKeyID := os.Getenv("OSS_ACCESS_KEY")
	secretAccessKey := os.Getenv("OSS_SECRET_ACCESS_KEY")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		t.FailNow()
	}
	for object := range client.ListObjects(context.TODO(), "my-bucket", minio.ListObjectsOptions{}) {
		fmt.Println(object.Key)
	}
}
