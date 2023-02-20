package mql

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mql/web/api"
	"gorm.io/gorm"
)

type Server struct {
	g  *echo.Group
	db *gorm.DB
}

func NewServer(g *echo.Group, db *gorm.DB) *Server {
	return &Server{
		g:  g,
		db: db,
	}
}

func (s *Server) Init() error {
	api.Init(s.db)
	s.g.POST("/load-project", api.LoadProjectController)
	s.g.POST("/reload-project", api.ReloadProjectController)
	s.g.POST("/execute-query", api.ExecuteQueryController)

	return nil
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}
