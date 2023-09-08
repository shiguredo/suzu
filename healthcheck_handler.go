package suzu

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (s *Server) healthcheckHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"version": s.config.Version,
	})
}
