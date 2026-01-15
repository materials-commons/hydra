package wserv

import (
	"log"
	"sync"
)

// WebSocketManager manages WebSocket client connections and their lifecycle.
// It handles registration, unregistration, and message broadcasting to WebSocket clients.
type WebSocketManager struct {
	// clients maps client IDs to their connections
	clients map[string]*ClientConnection

	// clientsByUserID groups clients by user ID for efficient user-based broadcasting
	clientsByUserID map[int][]*ClientConnection

	// register is used to add new client connections
	register chan *ClientConnection

	// unregister is used to remove client connections
	unregister chan *ClientConnection

	// broadcast sends a message to a specific client by ID
	broadcast chan Message

	// userBroadcast sends a message to all clients for a specific user
	userBroadcast chan UserMessage

	// mu protects the clients and clientsByUserID maps
	mu sync.RWMutex
}

// NewWebSocketManager creates a new WebSocketManager with initialized channels and maps.
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:         make(map[string]*ClientConnection),
		clientsByUserID: make(map[int][]*ClientConnection),
		register:        make(chan *ClientConnection),
		unregister:      make(chan *ClientConnection),
		broadcast:       make(chan Message),
		userBroadcast:   make(chan UserMessage, 100), // Buffered for better throughput
	}
}

// Register returns the channel used to register new client connections.
func (wsm *WebSocketManager) Register() chan<- *ClientConnection {
	return wsm.register
}

// Unregister returns the channel used to unregister client connections.
func (wsm *WebSocketManager) Unregister() chan<- *ClientConnection {
	return wsm.unregister
}

// Broadcast returns the channel used to send messages to specific clients.
func (wsm *WebSocketManager) Broadcast() chan<- Message {
	return wsm.broadcast
}

// UserBroadcast returns the channel used to send messages to all clients for a user.
func (wsm *WebSocketManager) UserBroadcast() chan<- UserMessage {
	return wsm.userBroadcast
}

// HandleRegister processes a client registration request.
func (wsm *WebSocketManager) HandleRegister(client *ClientConnection) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	wsm.clients[client.ID] = client
	wsm.clientsByUserID[client.User.ID] = append(wsm.clientsByUserID[client.User.ID], client)

	log.Printf("ClientConnection registered: %s (type: %s), (host: %s), (userID: %d)",
		client.ID, client.Type, client.Hostname, client.User.ID)
	log.Printf("With Projects:")
	for _, p := range client.Projects {
		log.Printf("  %s (id: %d)", p.Name, p.ID)
	}
}

// HandleUnregister processes a client unregistration request.
func (wsm *WebSocketManager) HandleUnregister(client *ClientConnection) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	if _, ok := wsm.clients[client.ID]; ok {
		delete(wsm.clients, client.ID)
		close(client.Send)

		// Remove from clientsByUserID
		if userClients, ok := wsm.clientsByUserID[client.User.ID]; ok {
			for i, c := range userClients {
				if c.ID == client.ID {
					// Delete the entry at index i
					wsm.clientsByUserID[client.User.ID] = append(userClients[:i], userClients[i+1:]...)
					break
				}
			}

			// Clean up the map key if the user has no more clients
			if len(wsm.clientsByUserID[client.User.ID]) == 0 {
				delete(wsm.clientsByUserID, client.User.ID)
			}
		}
	}

	log.Printf("ClientConnection unregistered: %s", client.ID)
}

// HandleBroadcast processes a broadcast message to a specific client.
func (wsm *WebSocketManager) HandleBroadcast(message Message) {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	targetID := message.ClientID
	if client, ok := wsm.clients[targetID]; ok {
		select {
		case client.Send <- message:
		default:
			// Channel full, client will be cleaned up
			log.Printf("Warning: could not send to client %s (channel full)", client.ID)
		}
	}
}

// HandleUserBroadcast processes a broadcast message to all clients for a user.
func (wsm *WebSocketManager) HandleUserBroadcast(userMsg UserMessage) {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	userClients, ok := wsm.clientsByUserID[userMsg.UserID]
	if !ok {
		return
	}

	for _, client := range userClients {
		if userMsg.ClientType == "" || client.Type == userMsg.ClientType {
			select {
			case client.Send <- userMsg.Message:
			default:
				// Channel full, skip this client
				log.Printf("Warning: could not send to client %s (channel full)", client.ID)
			}
		}
	}
}

// BroadcastToUser sends a message to all WebSocket clients for a specific user.
// The clientType parameter can be used to filter by client type (empty string = all types).
func (wsm *WebSocketManager) BroadcastToUser(userID int, clientType string, msg Message) {
	wsm.userBroadcast <- UserMessage{
		UserID:     userID,
		ClientType: clientType,
		Message:    msg,
	}
}

// GetClient retrieves a client connection by ID.
// Returns nil if the client doesn't exist.
func (wsm *WebSocketManager) GetClient(clientID string) *ClientConnection {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()
	return wsm.clients[clientID]
}

// GetClientsForUser retrieves all client connections for a specific user.
func (wsm *WebSocketManager) GetClientsForUser(userID int) []*ClientConnection {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	clients := wsm.clientsByUserID[userID]
	if clients == nil {
		return []*ClientConnection{}
	}

	// Return a copy to avoid concurrent modification issues
	result := make([]*ClientConnection, len(clients))
	copy(result, clients)
	return result
}

// GetAllClients returns a copy of all connected clients.
func (wsm *WebSocketManager) GetAllClients() map[string]*ClientConnection {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	// Return a copy to avoid concurrent modification issues
	result := make(map[string]*ClientConnection, len(wsm.clients))
	for k, v := range wsm.clients {
		result[k] = v
	}
	return result
}
