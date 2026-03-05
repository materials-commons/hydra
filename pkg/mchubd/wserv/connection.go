package wserv

import (
	"time"
)

// Connection represents a websocket connection with additional methods for setting deadlines and handling pongs.
// This allows us to provide other implementations of Connection for testing.
type Connection interface {
	ReadMessage() (messageType int, p []byte, err error)
	SetReadDeadline(t time.Time) error
	SetPongHandler(h func(appData string) error)

	WriteMessage(messageType int, data []byte) error
	WriteJSON(v any) error
	SetWriteDeadline(t time.Time) error

	Close() error
}
