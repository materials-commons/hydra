package wserv

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// SSEManager manages Server-Sent Events (SSE) connections for users.
// It handles connection lifecycle and message broadcasting to SSE clients.
type SSEManager struct {
	// sseConnections maps user IDs to their active SSE connections
	// Each user can have multiple SSE connections, identified by connection ID
	sseConnections map[int]map[string]chan Message

	// mu protects the sseConnections map
	mu sync.RWMutex
}

// NewSSEManager creates a new SSEManager with initialized maps.
func NewSSEManager() *SSEManager {
	return &SSEManager{
		sseConnections: make(map[int]map[string]chan Message),
	}
}

// RegisterConnection creates and registers a new SSE connection for a user.
// Returns the connection ID and event channel.
func (s *SSEManager) RegisterConnection(userID int) (string, chan Message) {
	eventChan := make(chan Message, 256)
	connectionID := fmt.Sprintf("sse-%d-%d", userID, time.Now().UnixNano())

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sseConnections[userID] == nil {
		s.sseConnections[userID] = make(map[string]chan Message)
	}
	s.sseConnections[userID][connectionID] = eventChan

	return connectionID, eventChan
}

// UnregisterConnection removes an SSE connection for a user.
func (s *SSEManager) UnregisterConnection(userID int, connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conns, ok := s.sseConnections[userID]; ok {
		if ch, exists := conns[connectionID]; exists {
			close(ch)
			delete(conns, connectionID)
		}

		// Clean up user entry if no connections remain
		if len(s.sseConnections[userID]) == 0 {
			delete(s.sseConnections, userID)
		}
	}

	log.Printf("SSE connection %s closed for user %d", connectionID, userID)
}

// BroadcastToUser sends a message to all SSE connections for a specific user.
func (s *SSEManager) BroadcastToUser(userID int, msg Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sseConns, ok := s.sseConnections[userID]
	if !ok {
		return
	}

	for _, sseChan := range sseConns {
		select {
		case sseChan <- msg:
		default:
			// SSE channel full, skip this client
			log.Printf("Warning: could not send to SSE client %d (channel full)", userID)
		}
	}
}

// HandleSSE handles an incoming SSE connection request.
// This method manages the entire lifecycle of an SSE connection.
func (s *SSEManager) HandleSSE(w http.ResponseWriter, r *http.Request, user *mcmodel.User) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Make sure we can do streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Register the connection
	connectionID, eventChan := s.RegisterConnection(user.ID)

	// Cleanup on disconnect
	defer s.UnregisterConnection(user.ID, connectionID)

	// Send the initial connection acknowledgment
	_, _ = fmt.Fprintf(w, "data: {\"event\":\"connected\",\"user_id\":%d}\n\n", user.ID)
	flusher.Flush()

	// Setup keep alive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return // Client disconnected

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
			// Send keep-alive
			_, _ = fmt.Fprintf(w, "data: {\"event\":\"keepalive\"}\n\n")
			flusher.Flush()
		}
	}
}

// GetConnectionCount returns the number of active SSE connections for a user.
func (s *SSEManager) GetConnectionCount(userID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if conns, ok := s.sseConnections[userID]; ok {
		return len(conns)
	}
	return 0
}

// GetAllConnectionCounts returns a map of user IDs to their connection counts.
func (s *SSEManager) GetAllConnectionCounts() map[int]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[int]int)
	for userID, conns := range s.sseConnections {
		result[userID] = len(conns)
	}
	return result
}
