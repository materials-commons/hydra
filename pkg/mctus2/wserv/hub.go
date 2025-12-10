package wserv

import (
	"encoding/json"
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

type HubCommandRequest struct {
	ClientID string                 `json:"client_id"`
	Command  string                 `json:"command"`
	UserID   int                    `json:"user_id"`
	Payload  map[string]interface{} `json:"payload"`
}

type HubCommandResponse struct {
	ClientID string `json:"client_id"`
	Command  string `json:"command"`
	UserID   int    `json:"user_id"`
	Status   string `json:"status"`
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
		Command:   "CONNECTED",
		ID:        "system",
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload:   map[string]interface{}{"status": "connected"},
	}
	client.Send <- connectMsg

	go client.writePump()
	go client.readPump()
}

// ****** NOTE ******
// All REST endpoints are listening on localhost. The PHP app sends requests to them. The user is already
// authenticated, so these endpoints don't need to do any authentication. However, we do check that the
// userID is associated with the clientID that comes across in the request.

func (h *Hub) HandleSendCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HubCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		http.Error(w, "Missing user_id", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}

	// ensure the client exists, and is associated with the userID
	h.mu.RLock()
	c, exists := h.clients[req.ClientID]
	h.mu.RUnlock()

	if !exists {
		_ = sendCommandResponse(w, HubCommandResponse{}, http.StatusNotFound)
	}

	if c.User.ID != req.UserID {
		_ = sendCommandResponse(w, HubCommandResponse{}, http.StatusForbidden)
	}

	msg := Message{
		Command:   req.Command,
		ID:        "system",
		Timestamp: time.Now(),
		ClientID:  req.ClientID,
		Payload:   req.Payload,
	}

	h.broadcast <- msg
	_ = sendCommandResponse(w, HubCommandResponse{Command: req.Command, Status: "ok"}, http.StatusOK)
}

type ClientResp struct {
	ClientID string `json:"client_id"`
	Type     string `json:"type"`
	Hostname string `json:"hostname"`
	UserID   int    `json:"user_id"`
}

func (h *Hub) HandleListClients(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	clients := make([]ClientResp, 0, len(h.clients))
	for id, client := range h.clients {
		cr := ClientResp{ClientID: id, Type: client.Type, Hostname: client.Hostname, UserID: client.User.ID}
		clients = append(clients, cr)
	}
	defer h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(clients)
}

// private utility methods

func sendCommandResponse(w http.ResponseWriter, resp HubCommandResponse, httpStatus int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	return json.NewEncoder(w).Encode(resp)
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
