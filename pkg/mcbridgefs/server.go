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

	s.closeExistingGlobusTransfers()
	return nil
}

// closeExistingGlobusTransfers will mark all existing globus transfers and transfer requests
// as closed, so they can be cleaned up. When the server starts it performs this action to
// remove old requests that no longer have a bridge associated with them.
func (s *Server) closeExistingGlobusTransfers() {
	_ = s.db.Exec("update globus_transfers set state = ?", "closed").Error
	_ = s.db.Exec("update transfer_requests set state = ?", "closed").Error
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}
