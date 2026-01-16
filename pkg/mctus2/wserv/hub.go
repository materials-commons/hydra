package wserv

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"gorm.io/gorm"
)

type Hub struct {
	// Connection managers
	WSManager  *WebSocketManager
	sseManager *SSEManager
	rrManager  *RequestResponseManager

	// Database storage interfaces
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
	ClientType string  `json:"client_type"` // Filter by client type (blank = all, sse, ws)
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
		// Initialize connection managers
		WSManager:  NewWebSocketManager(),
		sseManager: NewSSEManager(),
		rrManager:  NewRequestResponseManager(30 * time.Second), // 30s default timeout

		// Initialize storage interfaces
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
		case client := <-h.WSManager.register:
			fmt.Println("Registering client trying to get lock!")
			h.WSManager.HandleRegister(client)

			// Notify SSE clients about the new registration
			h.sseManager.BroadcastToUser(client.User.ID, Message{
				Command: "register",
			})

		case client := <-h.WSManager.unregister:
			h.WSManager.HandleUnregister(client)

			// Cancel any pending requests for this client
			h.rrManager.CancelRequestsForClient(client.ID)

			// Notify SSE clients about the unregistration
			h.sseManager.BroadcastToUser(client.User.ID, Message{
				Command: "unregister",
			})

		case message := <-h.WSManager.broadcast:
			h.WSManager.HandleBroadcast(message)

		case userMessage := <-h.WSManager.userBroadcast:
			fmt.Println("User broadcast!")
			h.WSManager.HandleUserBroadcast(userMessage)
			h.sseManager.BroadcastToUser(userMessage.UserID, userMessage.Message)
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// We authenticate via bearer token. This being the
		// case, the origin check is not critical for security.
		return true
	},
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
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
		Hub:          h,
	}

	client.Hub.WSManager.Register() <- client
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

// broadcastToUserClients sends a message to all clients (WebSocket and SSE) for a specific user.
func (h *Hub) broadcastToUserClients(userID int, clientType string, msg Message) {
	// Send to WebSocket clients via the user broadcast channel
	h.WSManager.UserBroadcast() <- UserMessage{
		UserID:     userID,
		ClientType: clientType,
		Message:    msg,
	}
}

/////////////////// REST API Handlers ///////////////////

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

	// ensure the client exists and is associated with the userID
	c := h.WSManager.GetClient(req.ClientID)
	if c == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		fmt.Println("Client not found")
		return
	}

	if c.User.ID != req.UserID {
		http.Error(w, "User not allowed", http.StatusForbidden)
		fmt.Println("User not allowed")
		return
	}

	msg := Message{
		Command:   req.Command,
		ID:        "system",
		Timestamp: time.Now(),
		ClientID:  req.ClientID,
		Payload:   req.Payload,
	}

	h.WSManager.Broadcast() <- msg
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

	allClients := h.WSManager.GetAllClients()
	clients := make([]ListClientsResp, 0, len(allClients))
	for id, client := range allClients {
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

func (h *Hub) HandleUploadFile(w http.ResponseWriter, r *http.Request) {
	fmt.Println("HandleUploadFile")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
			"file_path":    "/home/gtarcea/proj/Aging/random_250MiB.bin",
			"project_path": "/random_250MiB.bin",
			"project_id":   438,
		},
	}

	if h.WSManager.GetClient(clientID) == nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		fmt.Println("Client not found")
		return
	}

	h.WSManager.Broadcast() <- msg
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

	clientsForUser := h.WSManager.GetClientsForUser(userID)
	clients := make([]ListClientsResp, 0, len(clientsForUser))
	for _, client := range clientsForUser {
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

	// Delegate to SSE manager
	h.sseManager.HandleSSE(w, r, user)
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

func (h *Hub) GetConnectedClientsForUser(userID int) []ListClientsResp {
	clientsForUser := h.WSManager.GetClientsForUser(userID)
	clients := make([]ListClientsResp, 0, len(clientsForUser))
	for _, client := range clientsForUser {
		cr := ListClientsResp{
			ClientID: client.ID,
			Type:     client.Type,
			Hostname: client.Hostname,
			UserID:   client.User.ID,
			Projects: getProjectIds(client.Projects),
		}
		clients = append(clients, cr)
	}

	return clients
}
