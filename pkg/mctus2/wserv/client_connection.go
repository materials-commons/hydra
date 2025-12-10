package wserv

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// Message types
const (
	// Control messages (Laravel → Python ClientConnection)
	MsgUploadStart  = "UPLOAD_START"
	MsgUploadPause  = "UPLOAD_PAUSE"
	MsgUploadResume = "UPLOAD_RESUME"
	MsgUploadCancel = "UPLOAD_CANCEL"
	MsgGetStatus    = "GET_STATUS"
	MsgHeartbeat    = "HEARTBEAT"

	// Status messages (Python ClientConnection → Laravel)
	MsgClientConnected    = "CLIENT_CONNECTED"
	MsgClientDisconnected = "CLIENT_DISCONNECTED"
	MsgUploadProgress     = "UPLOAD_PROGRESS"
	MsgUploadComplete     = "UPLOAD_COMPLETE"
	MsgUploadFailed       = "UPLOAD_FAILED"
	MsgClientStatus       = "CLIENT_STATUS"
)

type Message struct {
	Command   string                 `json:"command"`
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	ClientID  string                 `json:"clientId"`
	Payload   map[string]interface{} `json:"payload"`
}

type ClientConnection struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan Message
	Hub      *Hub
	Type     string // "ui" or "python"
	Hostname string
	User     *mcmodel.User
	mu       sync.Mutex
}

func (c *ClientConnection) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		msg.Timestamp = time.Now()
		log.Printf("Received message: command=%s from=%s", msg.Command, c.ID)

		// Handle message based on type
		c.handleMessage(msg)
	}
}

func (c *ClientConnection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *ClientConnection) handleMessage(msg Message) {
	switch msg.Command {
	case MsgUploadStart, MsgUploadPause, MsgUploadResume, MsgUploadCancel, MsgGetStatus:
		// Forward control messages to target Python client
		c.Hub.broadcast <- msg

	case MsgUploadProgress, MsgUploadComplete, MsgUploadFailed, MsgClientStatus:
		// Forward status messages to Laravel UI
		c.Hub.broadcast <- msg

	case MsgHeartbeat:
		// Respond to heartbeat
		response := Message{
			Command:   "HEARTBEAT_ACK",
			ID:        msg.ID,
			Timestamp: time.Now(),
			ClientID:  msg.ClientID,
		}
		c.Send <- response

	case MsgClientConnected:
		log.Printf("ClientConnection %s connected", msg.ClientID)

	case MsgClientDisconnected:
		log.Printf("ClientConnection %s disconnected", msg.ClientID)
	}
}
