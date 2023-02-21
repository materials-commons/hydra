package api

import (
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcft/ft"
	"gorm.io/gorm"
)

type WSController struct {
	db       *gorm.DB
	upgrader websocket.Upgrader
}

func NewWSController(db *gorm.DB) *WSController {
	return &WSController{
		db:       db,
		upgrader: websocket.Upgrader{},
	}
}

func (c *WSController) HandleUploadDownloadConnection(ctx echo.Context) error {
	ws, err := c.upgrader.Upgrade(ctx.Response(), ctx.Request(), nil)
	if err != nil {
		return err
	}

	fileTransferHandler := ft.NewFileTransferHandler(ws, c.db)
	defer func() {
		_ = ws.Close()
	}()

	if err := fileTransferHandler.Run(); err != nil {
		status := ft.Error2Status(err)
		_ = ws.WriteJSON(status)
	}

	return nil
}
