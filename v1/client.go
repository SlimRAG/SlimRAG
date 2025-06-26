package rag

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"resty.dev/v3"
)

type InfinityClient struct {
	client *resty.Client
}

func NewInfinityClient(baseURL string) *InfinityClient {
	return &InfinityClient{
		client: resty.New().SetBaseURL(baseURL),
	}
}

func (c *InfinityClient) Close() (err error) {
	if c.client != nil {
		err = c.client.Close()
	}
	return
}

type HealthResponse struct {
	Unix float64 `json:"unix"`
}

func (c *InfinityClient) GetHealth() (*HealthResponse, error) {
	var response HealthResponse
	rsp, err := c.client.R().SetResult(&response).Get("/health")
	if err != nil {
		return nil, err
	}
	if code := rsp.StatusCode(); code != http.StatusOK {
		return nil, errors.Newf("status code: %d", code)
	}
	return &response, nil
}

type RerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n"`
	ReturnDocuments bool     `json:"return_documents"`
	RawScores       bool     `json:"raw_scores"`
}

type RerankResponse struct {
	Id      string `json:"id"`
	Model   string `json:"model"`
	Created int    `json:"created"`
	Object  string `json:"object"`

	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`

	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       string  `json:"document"`
	} `json:"results"`
}

func (c *InfinityClient) Rerank(req *RerankRequest) (*RerankResponse, error) {
	var response RerankResponse
	rsp, err := c.client.R().SetBody(req).SetResult(&response).Post("/rerank")
	if err != nil {
		return nil, err
	}
	if code := rsp.StatusCode(); code != http.StatusOK {
		return nil, errors.Newf("status code: %d, response: '%s'", code, rsp.String())
	}
	return &response, nil
}
