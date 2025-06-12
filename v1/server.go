package rag

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo) {
	e.GET("/", getHome)
}

func getHome(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, RAG!")
}
