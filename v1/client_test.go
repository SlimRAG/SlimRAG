package rag

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfinityClient_Rerank(t *testing.T) {
	client := NewInfinityClient(os.Getenv("RERANKER_BASE_URL"))
	defer func() { _ = client.Close() }()

	_, err := client.GetHealth()
	require.NoError(t, err)

	rsp, err := client.Rerank(&RerankRequest{
		Model:     os.Getenv("RERANKER_MODEL"),
		Query:     "query",
		Documents: []string{"doc1", "doc2", "doc3"},
		TopN:      3,
	})
	require.NoError(t, err)
	fmt.Printf("%+v\n", rsp)
}
