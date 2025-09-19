package webapi

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcft/wire"
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

	defer ws.Close()

	for {
		msg, err := wire.ReadTypedMessage(ws)
		if err != nil {
			break
		}

		switch m := msg.(type) {
		case *wire.InitUploadMsg:
		case *wire.UploadChunkMsg:
		case *wire.FinalizeUploadMsg:
		default:
			_ = m
		}
	}

	return nil
}

func writeJSON(ws *websocket.Conn, v any) error {
	_ = ws.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return ws.WriteJSON(v)
}

func readMsg(ws *websocket.Conn) (int, []byte, error) {
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Minute))
	return ws.ReadMessage()
}
