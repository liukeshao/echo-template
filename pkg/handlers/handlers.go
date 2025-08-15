package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/services"
)

var handlers []Handler

// Handler handles one or more HTTP routes
type Handler interface {
	// Routes allows for self-registration of HTTP routes on the router
	Routes(g *echo.Group)

	// Init provides the service container to initialize
	Init(c *services.Container) error
}

// Register registers a handler
func Register(h Handler) {
	handlers = append(handlers, h)
}

// GetHandlers returns all handlers
func GetHandlers() []Handler {
	return handlers
}

// Success 成功响应
func Success(c echo.Context, data any) error {
	response := errors.NewSuccessResponse(c, data)
	return c.JSON(http.StatusOK, response)
}
