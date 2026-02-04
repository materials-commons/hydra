# Hub Manager Architecture

## Overview

The Hub has been refactored to use three specialized managers for different connection types:

1. **WebSocketManager** - Manages WebSocket client connections
2. **SSEManager** - Manages Server-Sent Events connections
3. **RequestResponseManager** - Manages HTTP requests waiting for WebSocket responses

## WebSocketManager

Handles WebSocket client lifecycle and message broadcasting.

### Usage Examples

```go
// Get a specific client
client := hub.wsManager.GetClient(clientID)

// Get all clients for a user
clients := hub.wsManager.GetClientsForUser(userID)

// Broadcast to a specific client
hub.wsManager.Broadcast() <- Message{
    Command:   "SOME_COMMAND",
    ClientID:  clientID,
    Payload:   data,
}

// Broadcast to all clients for a user
hub.wsManager.BroadcastToUser(userID, "ui", Message{
    Command: "UPDATE",
    Payload: data,
})
```

## SSEManager

Handles Server-Sent Events connection lifecycle and broadcasting.

### Usage Examples

```go
// In your HTTP handler, just delegate to the SSE manager
func (h *Hub) HandleSSE(w http.ResponseWriter, r *http.Request) {
    user, err := h.validateAuthAndGetUser(r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }

    h.sseManager.HandleSSE(w, r, user)
}

// Broadcast to all SSE connections for a user
hub.sseManager.BroadcastToUser(userID, Message{
    Command: "NOTIFICATION",
    Payload: data,
})
```

## RequestResponseManager

Manages temporary channels for HTTP requests that need responses from WebSocket clients.

### Use Case

When an HTTP endpoint needs to:
1. Send a command to a WebSocket client
2. Wait for the client's response
3. Return that response to the HTTP caller

### Usage Example

```go
// Example: HTTP handler that sends a command to a WS client and waits for response
func (h *Hub) HandleGetClientInfo(w http.ResponseWriter, r *http.Request) {
    clientID := r.PathValue("client_id")
    userID := getUserIDFromAuth(r)

    // Create a pending request (30 second timeout)
    req, err := h.rrManager.CreateRequest(clientID, userID, "GET_CLIENT_INFO", 30*time.Second)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Send command to WebSocket client
    h.wsManager.Broadcast() <- Message{
        Command:   "GET_CLIENT_INFO",
        ID:        req.RequestID,
        ClientID:  clientID,
        Payload:   map[string]interface{}{"request_id": req.RequestID},
    }

    // Wait for response (blocks until response or timeout)
    response, err := h.rrManager.WaitForResponse(req)
    if err != nil {
        http.Error(w, "Request timed out", http.StatusRequestTimeout)
        return
    }

    // Send response back to HTTP caller
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response.Payload)
}
```

### WebSocket Client Response Handler

The WebSocket client needs to send responses with the request ID:

```go
// In client_connection.go handleMessage
case "CLIENT_INFO_RESPONSE":
    // Extract request ID from payload
    requestID := msg.Payload["request_id"].(string)

    // Send response back through the request-response manager
    err := c.Hub.rrManager.SendResponse(requestID, msg)
    if err != nil {
        log.Printf("Failed to send response: %v", err)
    }
```

## Benefits of This Architecture

### Separation of Concerns
- Each manager handles one type of connection/pattern
- Hub.go is much cleaner and easier to understand
- Easy to test individual managers

### Scalability
- Adding new channel types is straightforward (create a new manager)
- Managers can be optimized independently
- Clear boundaries between different concerns

### Request-Response Pattern
- HTTP handlers can now easily communicate with WebSocket clients
- Built-in timeout handling
- Automatic cleanup of expired requests
- Thread-safe request tracking

## Migration Notes

Old code that accessed hub fields directly needs to be updated:

```go
// OLD
hub.clients[clientID]
hub.broadcast <- msg
hub.userBroadcast <- UserMessage{...}

// NEW
hub.wsManager.GetClient(clientID)
hub.wsManager.Broadcast() <- msg
hub.wsManager.UserBroadcast() <- UserMessage{...}
```

## Future Enhancements

The RequestResponseManager can be extended to support:
- Priority queues for requests
- Request batching
- Streaming responses (multiple messages)
- Request cancellation via context
- Metrics and monitoring
