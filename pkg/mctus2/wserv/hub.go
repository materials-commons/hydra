package wserv

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	projectStor     stor.ProjectStor
}

type ConnectionAttributes struct {
	ClientID string `json:"client_id"`
	Type     string `json:"type"`
	Hostname string `json:"hostname"`
	Projects string `json:"projects"`
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
		projectStor:     stor.NewGormProjectStor(db),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			fmt.Println("Registering client trying to get lock!")
			h.mu.Lock()
			h.clients[client.ID] = client
			h.clientsByUserID[client.User.ID] = append(h.clientsByUserID[client.User.ID], client)
			h.mu.Unlock()
			log.Printf("ClientConnection registered: %s (type: %s), (host: %s), (userID: %d)", client.ID, client.Type, client.Hostname, client.User.ID)
			log.Printf("With Projects:")
			for _, p := range client.Projects {
				log.Printf("  %s (id: %d)", p.Name, p.ID)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)

				// Remove from clientsByUserID
				if userClients, ok := h.clientsByUserID[client.User.ID]; ok {
					for i, c := range userClients {
						if c.ID == client.ID {
							// Delete the entry at index i
							h.clientsByUserID[client.User.ID] = append(userClients[:i], userClients[i+1:]...)
							break
						}
					}

					// Clean up the map key if the user has no more clients
					if len(h.clientsByUserID[client.User.ID]) == 0 {
						delete(h.clientsByUserID, client.User.ID)
					}
				}
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

	// Validate token and get client info
	user, err := h.validateAuthAndGetUser(r)
	if err != nil {
		fmt.Println("Invalid or expired token")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	clientConnectionAttrs := getClientConnectionAttributes(r)

	switch {
	case clientConnectionAttrs.ClientID == "":
		fmt.Println("Missing MC-Client-ID header")
		http.Error(w, "Missing MC-Client-ID header or client_id param", http.StatusBadRequest)
		return

	case clientConnectionAttrs.Hostname == "":
		fmt.Println("Missing MC-Client-Hostname header")
		http.Error(w, "Missing MC-Client-Hostname header or hostname param", http.StatusBadRequest)
		return

	case clientConnectionAttrs.Type == "":
		fmt.Println("Missing MC-Connection-Type header or connection_type param")
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
		ID:       clientConnectionAttrs.ClientID,
		Hostname: clientConnectionAttrs.Hostname,
		Type:     clientConnectionAttrs.Type,
		User:     user,
		Projects: h.commaSeparatedProjectIDsToProjects(clientConnectionAttrs.Projects),
		Conn:     conn,
		Send:     make(chan Message, 256),
		Hub:      hub,
	}

	client.Hub.register <- client
	fmt.Println("Client registered!")

	// Send connection acknowledgment
	connectMsg := Message{
		Command:   "connected",
		ID:        "system",
		Timestamp: time.Now(),
		ClientID:  clientConnectionAttrs.ClientID,
		Payload:   map[string]interface{}{"status": "connected", "user_id": user.ID},
	}
	client.Send <- connectMsg

	fmt.Println("Client connectMsg sent!")

	go client.writePump()
	go client.readPump()
	fmt.Println("Client readPump and writePump started!")
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

type ListClientsResp struct {
	ClientID string `json:"client_id"`
	Type     string `json:"type"`
	Hostname string `json:"hostname"`
	UserID   int    `json:"user_id"`
	Projects []int  `json:"projects"`
}

func (h *Hub) HandleListClients(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]ListClientsResp, 0, len(h.clients))
	for id, client := range h.clients {

		cr := ListClientsResp{
			ClientID: id,
			Type:     client.Type,
			Hostname: client.Hostname,
			UserID:   client.User.ID,
			Projects: getProjectIds(client.Projects),
		}
		clients = append(clients, cr)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(clients)
}

func (h *Hub) HandleListClientsForUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.PathValue("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	clientsForUser, found := h.clientsByUserID[userID]
	if !found {
		empty := make([]ListClientsResp, 0)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(empty)
		return
	}
	clients := make([]ListClientsResp, 0, len(clientsForUser))
	for _, client := range clientsForUser {
		if client.User.ID != userID {
			continue
		}

		cr := ListClientsResp{
			ClientID: client.ID,
			Type:     client.Type,
			Hostname: client.Hostname,
			UserID:   client.User.ID,
			Projects: getProjectIds(client.Projects),
		}
		clients = append(clients, cr)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(clients)
}

func getProjectIds(projects []*mcmodel.Project) []int {
	ids := make([]int, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	return ids
}

// private utility methods

func sendCommandResponse(w http.ResponseWriter, resp HubCommandResponse, httpStatus int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	return json.NewEncoder(w).Encode(resp)
}

func (h *Hub) commaSeparatedProjectIDsToProjects(commaSeparatedIDs string) []*mcmodel.Project {
	var projects []*mcmodel.Project

	if commaSeparatedIDs == "" {
		return projects
	}

	for _, projectID := range strings.Split(commaSeparatedIDs, ",") {
		idAsInt, err := strconv.Atoi(strings.TrimSpace(projectID))
		if err != nil {
			continue
		}
		project, err := h.projectStor.GetProjectByID(idAsInt)
		if err != nil {
			continue
		}
		projects = append(projects, project)
	}

	return projects
}

func (h *Hub) validateAuthAndGetUser(r *http.Request) (*mcmodel.User, error) {
	token, err := h.getAuthToken(r)
	if err != nil {
		return nil, err
	}

	return h.userStor.GetUserByAPIToken(token)
}

func (h *Hub) getAuthToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		return h.extractTokenFromAuthHeader(authHeader)
	}

	// If we are here, then the token was passed in the query string
	token := r.URL.Query().Get("api_token")
	if token == "" {
		return "", fmt.Errorf("api_token is empty")
	}

	return token, nil
}

func (h *Hub) extractTokenFromAuthHeader(authHeader string) (string, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		fmt.Println("Invalid Authorization header format")
		return "", fmt.Errorf("invalid authorization header format")
	}

	token := parts[1]
	if token == "" {
		fmt.Println("Bearer token is empty")
		return "", fmt.Errorf("bearer token is empty")
	}

	return token, nil
}

func getClientConnectionAttributes(r *http.Request) *ConnectionAttributes {
	return &ConnectionAttributes{
		ClientID: getClientConnectionAttr(r, "MC-Client-ID", "client_id"),
		Type:     getClientConnectionAttr(r, "MC-Connection-Type", "connection_type"),
		Hostname: getClientConnectionAttr(r, "MC-Client-Hostname", "hostname"),
		Projects: getClientConnectionAttr(r, "MC-Client-Projects", "projects"),
	}
}

func getClientConnectionAttr(r *http.Request, headerAttr string, attr string) string {
	val := r.Header.Get(headerAttr)
	if val == "" {
		return r.URL.Query().Get(attr)
	}
	return val
}
