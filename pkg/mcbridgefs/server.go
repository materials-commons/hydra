package mcbridgefs

import (
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcbridgefs/web/api"
	"gorm.io/gorm"
)

type Server struct {
	g  *echo.Group
	db *gorm.DB
	c  *api.BridgeController
}

func NewServer(g *echo.Group, db *gorm.DB) *Server {
	return &Server{
		g:  g,
		db: db,
		c:  api.NewBridgeController(db),
	}
}

func (s *Server) Init() error {
	s.g.POST("/start-bridge", s.c.StartBridgeController)
	s.g.GET("/list-active-bridges", s.c.ListActiveBridgesController)
	s.g.POST("/stop-bridge", s.c.StopBridgeController)

	return nil
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}
