package rag

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type Server struct {
	e *echo.Echo
	r *RAG
}

func NewServer(r *RAG) *Server {
	s := &Server{r: r}
	e := echo.New()
	s.e = e

	e.GET("/", s.homeHandler)
	e.POST("/v1/search", s.searchHandler)
	return s
}

func (s *Server) Start(bind string) error {
	return s.e.Start(bind)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.e.Shutdown(ctx)
}

type SearchParam struct {
	Query string `json:"query" validate:"required"`
	Limit int
}

func (p *SearchParam) WithDefaults(limitStr string) {
	limit, err := strconv.Atoi(limitStr)
	if limit <= 0 || err != nil {
		p.Limit = 10
	} else {
		p.Limit = limit
	}
}

func (s *Server) searchHandler(c echo.Context) error {
	var p SearchParam
	err := c.Bind(&p)
	if err != nil {
		return err
	}
	p.WithDefaults(c.QueryParam("limit"))

	chunks, err := s.r.QueryDocumentChunks(context.TODO(), p.Query, p.Limit)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, echo.Map{
		"count":  len(chunks),
		"chunks": chunks,
	})
}

func (s *Server) homeHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{
		"name":    "Simple RAG Server",
		"version": "1.0.0",
		"URL":     "https://github.com/fanyang89/rag",
	})
}
