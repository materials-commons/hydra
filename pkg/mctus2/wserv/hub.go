package wserv

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"gorm.io/gorm"
)

type Hub struct {
	clients         map[string]*ClientConnection
	clientsByUserID map[int][]*ClientConnection
	register        chan *ClientConnection
	unregister      chan *ClientConnection
	broadcast       chan Message
	mu              sync.RWMutex
	userStor        stor.UserStor
}

func NewHub(db *gorm.DB) *Hub {
	return &Hub{
		clients:         make(map[string]*ClientConnection),
		clientsByUserID: make(map[int][]*ClientConnection),
		register:        make(chan *ClientConnection),
		unregister:      make(chan *ClientConnection),
		broadcast:       make(chan Message),
		userStor:        stor.NewGormUserStor(db),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.clientsByUserID[client.User.ID] = append(h.clientsByUserID[client.User.ID], client)
			h.mu.Unlock()
			log.Printf("ClientConnection registered: %s (type: %s), (host: %s), (userID: %d)", client.ID, client.Type, client.Hostname, client.User.ID)

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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin validation
		return true
	},
}

func (h *Hub) ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// Extract bearer token from the Authorization header
	fmt.Println("Connection!")
	authHeader := r.Header.Get("Authorization")

	// Validate token and get client info
	err, user := h.validateAuthorizationAndUser(authHeader)
	if err != nil {
		fmt.Println("Invalid or expired token")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	clientID := r.Header.Get("MC-Client-ID")
	clientHostname := r.Header.Get("MC-Client-Hostname")
	clientType := r.Header.Get("MC-Connection-Type")

	switch {
	case clientID == "":
		fmt.Println("Missing MC-Client-ID header")
		http.Error(w, "Missing MC-Client-ID header", http.StatusBadRequest)
		return

	case clientHostname == "":
		fmt.Println("Missing MC-Client-Hostname header")
		http.Error(w, "Missing MC-Client-Hostname header", http.StatusBadRequest)
		return

	case clientType == "":
		fmt.Println("Missing MC-Connection-Type header")
		http.Error(w, "Missing MC-Connection-Type header", http.StatusBadRequest)
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
		ID:       clientID,
		Hostname: clientHostname,
		Type:     clientType,
		User:     user,
		Conn:     conn,
		Send:     make(chan Message, 256),
		Hub:      hub,
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

func (h *Hub) validateAuthorizationAndUser(authHeader string) (error, *mcmodel.User) {
	if authHeader == "" {
		fmt.Println("Missing Authorization header")
		return fmt.Errorf("missing authorization header"), nil
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		fmt.Println("Invalid Authorization header format")
		return fmt.Errorf("invalid authorization header format"), nil
	}

	token := parts[1]
	if token == "" {
		fmt.Println("Bearer token is empty")
		return fmt.Errorf("bearer token is empty"), nil
	}

	user, err := h.userStor.GetUserByAPIToken(token)
	if err != nil {
		fmt.Printf("No user found for token: %s\n", token)
	}

	return err, user
}
