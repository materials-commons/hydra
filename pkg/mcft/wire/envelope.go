package wire

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// Envelope contains only the discriminator used to pick the concrete type.
type Envelope struct {
	MsgType string `json:"msg_type"`
}

var msgRegistry = map[string]func() any{
	"init_upload":     func() any { return InitUploadMsg{} },
	"upload_chunk":    func() any { return UploadChunkMsg{} },
	"finalize_upload": func() any { return FinalizeUploadMsg{} },
}

// ReadTypedMessage reads a single WebSocket text/binary message, inspects msg_type,
// and unmarshals into the appropriate struct using a registry of msg_type => reflect.Type.
//
// - conn: an established *websocket.Conn
// - registry: map of msg_type -> reflect.Type (type must be a struct type from this package)
//
// Returns the concrete message as interface{} (you can type-assert or type-switch).
func ReadTypedMessage(conn *websocket.Conn) (interface{}, error) {
	// Optional hardening before auth/handshake phases:
	// conn.SetReadLimit(8 << 10) // 8KB
	// _ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	mt, data, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	// We only handle text/binary JSON messages here
	if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
		return nil, fmt.Errorf("unsupported websocket message type: %d", mt)
	}

	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	if env.MsgType == "" {
		return nil, fmt.Errorf("missing msg_type")
	}

	f, ok := msgRegistry[env.MsgType]
	if !ok {
		return nil, fmt.Errorf("unknown msg_type: %q", env.MsgType)
	}

	// Create a pointer to the struct type so json can unmarshal into it.
	v := f()
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("decode %s: %w", env.MsgType, err)
	}

	return v, nil
}

// ReadTypedMessageWithDeadline is a safe wrapper that sets a temporary read deadline for this read.
// Useful if you want per-message timeouts.
func ReadTypedMessageWithDeadline(conn *websocket.Conn, timeout time.Duration) (interface{}, error) {
	prevDeadline := time.Time{}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	msg, err := ReadTypedMessage(conn)
	_ = conn.SetReadDeadline(prevDeadline) // clear deadline
	return msg, err
}
