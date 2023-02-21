package mcft

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcft/web/api"
	"gorm.io/gorm"
)

type Server struct {
	e *echo.Echo
	c *api.WSController
}

func NewServer(e *echo.Echo, db *gorm.DB) *Server {
	return &Server{
		e: e,
		c: api.NewWSController(db),
	}
}

func (s *Server) Init() error {
	s.e.GET("/ws", s.c.HandleUploadDownloadConnection)
	return nil
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}
