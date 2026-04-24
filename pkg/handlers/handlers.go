package handlers

import "github.com/labstack/echo/v5"

// Handler handles one or more HTTP routes.
type Handler interface {
	Routes(g *echo.Group)
}
