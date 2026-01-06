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
	clients                  map[string]*ClientConnection
	clientsByUserID          map[int][]*ClientConnection
	register                 chan *ClientConnection
	unregister               chan *ClientConnection
	broadcast                chan Message
	mu                       sync.RWMutex
	userBroadcast            chan UserMessage
	sseConnections           map[int]map[string]chan Message // UserID -> ConnectionID -> channel
	sseConnectionsMu         sync.RWMutex
	userStor                 stor.UserStor
	projectStor              stor.ProjectStor
	fileStor                 stor.FileStor
	remoteClientStor         stor.RemoteClientStor
	remoteClientTransferStor stor.RemoteClientTransferStor
	conversionStor           stor.ConversionStor
	partialTransferFileStor  *stor.GormPartialTransferFileStor // TODO: Make this an interface
}

type UserMessage struct {
	UserID     int     `json:"user_id"`
	ClientType string  `json:"client_type"` // Filter by client type (blank = all)
	Message    Message `json:"message"`
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

func NewHub(db *gorm.DB, mcfsDir string) *Hub {
	return &Hub{
		clients:                  make(map[string]*ClientConnection),
		clientsByUserID:          make(map[int][]*ClientConnection),
		sseConnections:           make(map[int]map[string]chan Message),
		register:                 make(chan *ClientConnection),
		unregister:               make(chan *ClientConnection),
		broadcast:                make(chan Message),
		userBroadcast:            make(chan UserMessage, 100), // Buffered
		userStor:                 stor.NewGormUserStor(db),
		projectStor:              stor.NewGormProjectStor(db),
		remoteClientStor:         stor.NewGormRemoteClientStor(db),
		fileStor:                 stor.NewGormFileStor(db, mcfsDir),
		remoteClientTransferStor: stor.NewGormRemoteClientTransferStor(db),
		conversionStor:           stor.NewGormConversionStor(db),
		partialTransferFileStor:  stor.NewGormPartialTransferFileStor(db),
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

		case userMessage := <-h.userBroadcast:
			fmt.Println("User broadcast!")
			h.broadcastToUserWSClients(userMessage.UserID, userMessage.ClientType, userMessage.Message)
			h.broadcastToUserSSEConnections(userMessage)
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

	remoteClient, err := h.getOrCreateRemoteClient(clientConnectionAttrs, user)
	if err != nil {
		fmt.Println("Error getting or creating remote client")
		log.Printf("Error getting or creating remote client: %v", err)
		return
	}

	fmt.Println("Creating client!")
	client := &ClientConnection{
		ID:           clientConnectionAttrs.ClientID,
		Hostname:     clientConnectionAttrs.Hostname,
		Type:         clientConnectionAttrs.Type,
		RemoteClient: remoteClient,
		User:         user,
		Projects:     h.commaSeparatedProjectIDsToProjects(clientConnectionAttrs.Projects),
		Conn:         conn,
		Send:         make(chan Message, 256),
		Hub:          hub,
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

func (h *Hub) broadcastToUserClients(userID int, clientType string, msg Message) {
	h.userBroadcast <- UserMessage{
		UserID:     userID,
		ClientType: clientType,
		Message:    msg,
	}
}

func (h *Hub) broadcastToUserWSClients(userID int, clientType string, msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userClients, ok := h.clientsByUserID[userID]
	if !ok {
		return
	}

	for _, client := range userClients {
		if clientType == "" || client.Type == clientType {
			select {
			case client.Send <- msg:
			default:
				// Channel full, skip this client
				log.Printf("Warning: could not send to client %s (channel full)", client.ID)
			}
		}
	}
}

func (h *Hub) broadcastToUserSSEConnections(userMsg UserMessage) {
	h.sseConnectionsMu.RLock()
	defer h.sseConnectionsMu.RUnlock()

	sseConns, ok := h.sseConnections[userMsg.UserID]
	if !ok {
		return
	}

	for _, sseChan := range sseConns {
		select {
		case sseChan <- userMsg.Message:
		default:
			// SSE channel full, skip this client
			log.Printf("Warning: could not send to SSE client %d (channel full)", userMsg.UserID)
		}
	}
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

/////////////////// REST API Handlers ///////////////////

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

func (h *Hub) HandleSubmitTestUpload(w http.ResponseWriter, r *http.Request) {
	fmt.Println("HandleSubmitTestUpload")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.PathValue("client_id")
	fmt.Printf("  clientID: %s\n", clientID)
	msg := Message{
		Command:   "UPLOAD_FILE",
		ID:        "ui", // Should this be the ID of the initiating client (Web UI)?
		Timestamp: time.Now(),
		ClientID:  clientID,
		Payload: map[string]any{
			"file_path":    "/home/gtarcea/uploadme.txt",
			"project_path": "/uploadme.txt",
			"project_id":   438,
		},
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[clientID]
	if !ok {
		http.Error(w, "Client not found", http.StatusNotFound)
		fmt.Println("Client not found")
		return
	}

	h.broadcast <- msg
	_ = sendCommandResponse(w, HubCommandResponse{Command: "UPLOAD_FILE", Status: "ok"}, http.StatusOK)
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

func (h *Hub) HandleSSE(w http.ResponseWriter, r *http.Request) {
	fmt.Println("HandleSSE")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ensure the user is authenticated
	user, err := h.validateAuthAndGetUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// CORS for now
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Make sure we can do streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create the event channel for this SSE connection
	eventChan := make(chan Message, 256)
	connectionID := fmt.Sprintf("sse-%d-%d", user.ID, time.Now().UnixNano())

	// Register the connection
	h.sseConnectionsMu.Lock()
	if h.sseConnections[user.ID] == nil {
		h.sseConnections[user.ID] = make(map[string]chan Message)
	}
	h.sseConnections[user.ID][connectionID] = eventChan
	h.sseConnectionsMu.Unlock()

	// Cleanup state on disconnect
	defer func() {
		h.sseConnectionsMu.Lock()
		delete(h.sseConnections[user.ID], connectionID)
		h.sseConnectionsMu.Unlock()
		// Remove from sseConnections if the user has no more connections
		if len(h.sseConnections[user.ID]) == 0 {
			delete(h.sseConnections, user.ID)
		}
		close(eventChan)
		log.Printf("SSE connection %s closed for user %d", connectionID, user.ID)
	}()

	// Send the initial connection acknowledgment
	_, _ = fmt.Fprintf(w, "data: {\"event\":\"connected\",\"user_id\":%d}\n\n", user.ID)
	flusher.Flush()

	// Setup keep alive, then loop on select
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := r.Context() // detect if we are done
	for {
		select {
		case <-ctx.Done():
			return // disconnect

		case msg := <-eventChan:
			// Send the message
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Error marshalling SSE message: %v", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-ticker.C:
			// Keep alive!
			_, _ = fmt.Fprintf(w, "data: {\"event\":\"keepalive\"}\n\n")
			flusher.Flush()
		}
	}
}

/////////////////// Utility functions/methods ///////////////////

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

func (h *Hub) getOrCreateRemoteClient(attrs *ConnectionAttributes, user *mcmodel.User) (*mcmodel.RemoteClient, error) {
	remoteClient, err := h.remoteClientStor.GetRemoteClientByClientID(attrs.ClientID)
	if err == nil {
		// Found the remote client

		// TODO: Update LastSeenAt when we find an existing client.
		return remoteClient, nil
	}

	// remote client not found, create it
	remoteClient = &mcmodel.RemoteClient{
		ClientID:   attrs.ClientID,
		Hostname:   attrs.Hostname,
		Name:       attrs.Hostname,
		Type:       attrs.Type,
		OwnerID:    user.ID,
		LastSeenAt: time.Now(),
	}

	return h.remoteClientStor.CreateRemoteClient(remoteClient)
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
