package wserv

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"gorm.io/gorm"
)

type Hub struct {
	clients    map[string]*ClientConnection
	register   chan *ClientConnection
	unregister chan *ClientConnection
	broadcast  chan Message
	mu         sync.RWMutex
	userStor   stor.UserStor
}

func NewHub(db *gorm.DB) *Hub {
	return &Hub{
		clients:    make(map[string]*ClientConnection),
		register:   make(chan *ClientConnection),
		unregister: make(chan *ClientConnection),
		broadcast:  make(chan Message),
		userStor:   stor.NewGormUserStor(db),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			log.Printf("ClientConnection registered: %s (type: %s)", client.ID, client.Type)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("ClientConnection unregistered: %s", client.ID)

		case message := <-h.broadcast:
			h.mu.RLock()
			targetID := message.ClientID
			if client, ok := h.clients[targetID]; ok {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client.ID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) validateToken(token string) (clientID string, clientType string, valid bool) {
	// TODO: Implement your token validation logic here
	// This should validate against your database or token store
	// Return clientID, clientType ("ui" or "python"), and whether token is valid

	fmt.Println("Validating token:", token)
	// Example placeholder:
	if token == "" {
		return "", "", false
	}

	user, err := h.userStor.GetUserByAPIToken(token)
	if err != nil {
		return "", "", false
	}

	_ = user

	// You would typically:
	// 1. Query database to verify token exists
	// 2. Check if token is expired
	// 3. Get associated client ID and type
	// 4. Return the information

	return "client-123", "python", true
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin validation
		return true
	},
}

func (h *Hub) ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// Extract bearer token from Authorization header
	fmt.Println("Connection!")
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		fmt.Println("Missing Authorization header")
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		fmt.Println("Invalid Authorization header format")
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}

	token := parts[1]

	// Validate token and get client info
	clientID, clientType, valid := h.validateToken(token)
	if !valid {
		fmt.Println("Invalid or expired token")
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Upgrade error")
		log.Printf("Upgrade error: %v", err)
		return
	}

	fmt.Println("Creating client!")
	client := &ClientConnection{
		ID:   clientID,
		Conn: conn,
		Send: make(chan Message, 256),
		Hub:  hub,
		Type: clientType,
	}

	client.Hub.register <- client

	// Send connection acknowledgment
	connectMsg := Message{
		Type:      "CONNECTED",
		ID:        "system",
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload:   map[string]interface{}{"status": "connected"},
	}
	client.Send <- connectMsg

	go client.writePump()
	go client.readPump()
}
