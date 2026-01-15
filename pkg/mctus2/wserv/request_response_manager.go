package wserv

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// PendingRequest represents an HTTP request waiting for a response from a WebSocket client.
// This is used when an HTTP endpoint needs to send a command to a WebSocket client and
// wait for a response to send back to the HTTP caller.
type PendingRequest struct {
	// RequestID is the unique identifier for this request
	RequestID string

	// ClientID is the ID of the WebSocket client this request is targeting
	ClientID string

	// UserID is the ID of the user making the request
	UserID int

	// Command is the command being sent to the WebSocket client
	Command string

	// ResponseChan is used to send the response back to the waiting HTTP handler
	ResponseChan chan Message

	// CreatedAt is when this request was created
	CreatedAt time.Time

	// Timeout is how long to wait for a response before giving up
	Timeout time.Duration

	// ctx is the context for this request (used for cancellation)
	ctx context.Context

	// cancel is the cancel function for the context
	cancel context.CancelFunc
}

// RequestResponseManager manages temporary channels for HTTP requests that need
// responses from WebSocket clients. This allows HTTP handlers to send commands
// to WebSocket clients and wait for responses.
type RequestResponseManager struct {
	// pendingRequests maps request IDs to their pending request info
	pendingRequests map[string]*PendingRequest

	// requestsByClient indexes requests by client ID for fast lookup
	requestsByClient map[string]map[string]*PendingRequest

	// mu protects the maps
	mu sync.RWMutex

	// cleanupInterval is how often to run cleanup of timed-out requests
	cleanupInterval time.Duration

	// defaultTimeout is the default timeout for requests
	defaultTimeout time.Duration
}

// NewRequestResponseManager creates a new RequestResponseManager.
func NewRequestResponseManager(defaultTimeout time.Duration) *RequestResponseManager {
	rrm := &RequestResponseManager{
		pendingRequests:  make(map[string]*PendingRequest),
		requestsByClient: make(map[string]map[string]*PendingRequest),
		cleanupInterval:  30 * time.Second,
		defaultTimeout:   defaultTimeout,
	}

	// Start background cleanup goroutine
	go rrm.cleanupLoop()

	return rrm
}

// CreateRequest creates a new pending request and returns the request ID.
// The caller should send the command to the WebSocket client and then wait
// on the ResponseChan for the response.
func (rrm *RequestResponseManager) CreateRequest(clientID string, userID int, command string, timeout time.Duration) (*PendingRequest, error) {
	if timeout == 0 {
		timeout = rrm.defaultTimeout
	}

	requestID := fmt.Sprintf("req-%s-%d", clientID, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	req := &PendingRequest{
		RequestID:    requestID,
		ClientID:     clientID,
		UserID:       userID,
		Command:      command,
		ResponseChan: make(chan Message, 1), // Buffered to avoid blocking
		CreatedAt:    time.Now(),
		Timeout:      timeout,
		ctx:          ctx,
		cancel:       cancel,
	}

	rrm.mu.Lock()
	rrm.pendingRequests[requestID] = req

	if rrm.requestsByClient[clientID] == nil {
		rrm.requestsByClient[clientID] = make(map[string]*PendingRequest)
	}
	rrm.requestsByClient[clientID][requestID] = req
	rrm.mu.Unlock()

	log.Printf("Created pending request %s for client %s (command: %s, timeout: %v)",
		requestID, clientID, command, timeout)

	return req, nil
}

// SendResponse sends a response to a pending request.
// Returns an error if the request doesn't exist or has timed out.
func (rrm *RequestResponseManager) SendResponse(requestID string, msg Message) error {
	rrm.mu.RLock()
	req, exists := rrm.pendingRequests[requestID]
	rrm.mu.RUnlock()

	if !exists {
		return errors.New("request not found")
	}

	select {
	case <-req.ctx.Done():
		return errors.New("request timed out")
	case req.ResponseChan <- msg:
		log.Printf("Response sent for request %s", requestID)
		return nil
	}
}

// WaitForResponse waits for a response to a pending request.
// Returns the response message or an error if the request times out.
func (rrm *RequestResponseManager) WaitForResponse(req *PendingRequest) (Message, error) {
	defer rrm.RemoveRequest(req.RequestID)

	select {
	case <-req.ctx.Done():
		return Message{}, errors.New("request timed out")
	case msg := <-req.ResponseChan:
		return msg, nil
	}
}

// RemoveRequest removes a pending request from the manager.
func (rrm *RequestResponseManager) RemoveRequest(requestID string) {
	rrm.mu.Lock()
	defer rrm.mu.Unlock()

	if req, exists := rrm.pendingRequests[requestID]; exists {
		// Cancel the context
		req.cancel()

		// Remove from maps
		delete(rrm.pendingRequests, requestID)

		if clientReqs, ok := rrm.requestsByClient[req.ClientID]; ok {
			delete(clientReqs, requestID)
			if len(clientReqs) == 0 {
				delete(rrm.requestsByClient, req.ClientID)
			}
		}

		log.Printf("Removed pending request %s", requestID)
	}
}

// GetPendingRequestsForClient returns all pending requests for a specific client.
func (rrm *RequestResponseManager) GetPendingRequestsForClient(clientID string) []*PendingRequest {
	rrm.mu.RLock()
	defer rrm.mu.RUnlock()

	clientReqs := rrm.requestsByClient[clientID]
	if clientReqs == nil {
		return []*PendingRequest{}
	}

	result := make([]*PendingRequest, 0, len(clientReqs))
	for _, req := range clientReqs {
		result = append(result, req)
	}
	return result
}

// CancelRequestsForClient cancels all pending requests for a specific client.
// This should be called when a client disconnects.
func (rrm *RequestResponseManager) CancelRequestsForClient(clientID string) {
	rrm.mu.Lock()
	defer rrm.mu.Unlock()

	clientReqs := rrm.requestsByClient[clientID]
	if clientReqs == nil {
		return
	}

	for requestID, req := range clientReqs {
		req.cancel()
		close(req.ResponseChan)
		delete(rrm.pendingRequests, requestID)
	}

	delete(rrm.requestsByClient, clientID)

	log.Printf("Cancelled all pending requests for client %s", clientID)
}

// cleanupLoop runs periodically to clean up timed-out requests.
func (rrm *RequestResponseManager) cleanupLoop() {
	ticker := time.NewTicker(rrm.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rrm.cleanup()
	}
}

// cleanup removes timed-out requests.
func (rrm *RequestResponseManager) cleanup() {
	rrm.mu.Lock()
	defer rrm.mu.Unlock()

	now := time.Now()
	for requestID, req := range rrm.pendingRequests {
		if now.Sub(req.CreatedAt) > req.Timeout {
			log.Printf("Cleaning up timed-out request %s", requestID)

			req.cancel()

			// Remove from maps
			delete(rrm.pendingRequests, requestID)
			if clientReqs, ok := rrm.requestsByClient[req.ClientID]; ok {
				delete(clientReqs, requestID)
				if len(clientReqs) == 0 {
					delete(rrm.requestsByClient, req.ClientID)
				}
			}
		}
	}
}

// GetStats returns statistics about pending requests.
func (rrm *RequestResponseManager) GetStats() map[string]interface{} {
	rrm.mu.RLock()
	defer rrm.mu.RUnlock()

	return map[string]interface{}{
		"total_pending_requests": len(rrm.pendingRequests),
		"clients_with_requests":  len(rrm.requestsByClient),
	}
}
