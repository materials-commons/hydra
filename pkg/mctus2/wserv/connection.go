package wserv

import (
	"time"
)

type Connection interface {
	ReadMessage() (messageType int, p []byte, err error)
	SetReadDeadline(t time.Time) error
	SetPongHandler(h func(appData string) error)

	WriteMessage(messageType int, data []byte) error
	WriteJSON(v any) error
	SetWriteDeadline(t time.Time) error

	Close() error
}
