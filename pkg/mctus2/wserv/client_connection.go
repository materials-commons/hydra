package wserv

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcssh/mc"
)

// Message types
const (
	MsgUploadStart  = "UPLOAD_START"
	MsgUploadPause  = "UPLOAD_PAUSE"
	MsgUploadResume = "UPLOAD_RESUME"
	MsgUploadCancel = "UPLOAD_CANCEL"
	MsgGetStatus    = "GET_STATUS"
	MsgHeartbeat    = "HEARTBEAT"

	MsgClientConnected        = "CLIENT_CONNECTED"
	MsgClientDisconnected     = "CLIENT_DISCONNECTED"
	MsgUploadProgress         = "UPLOAD_PROGRESS"
	MsgUploadComplete         = "UPLOAD_COMPLETE"
	MsgUploadFailed           = "UPLOAD_FAILED"
	MsgClientStatus           = "CLIENT_STATUS"
	MsgListProjects           = "LIST_PROJECTS"
	MsgTransferInit           = "TRANSFER_INIT"
	MsgTransferAccept         = "TRANSFER_ACCEPT"
	MsgTransferReject         = "TRANSFER_REJECT"
	MsgChunkAck               = "CHUNK_ACK"
	MsgTransferComplete       = "TRANSFER_COMPLETE"
	MsgTransferFinalize       = "TRANSFER_FINALIZE"
	MsgTransferResume         = "TRANSFER_RESUME"
	MsgTransferResumeResponse = "TRANSFER_RESUME_RESPONSE"
	MsgTransferCancel         = "TRANSFER_CANCEL"
)

type Message struct {
	Command   string    `json:"command"`
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	ClientID  string    `json:"clientId"`
	Payload   any       `json:"payload"`
}

type ClientConnection struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan Message
	Hub      *Hub
	Type     string // "ui" or "python"
	Hostname string
	User     *mcmodel.User
	Projects []*mcmodel.Project
	mu       sync.Mutex

	// File transfer state
	activeTransfers map[string]*FileTransfer
	transferMu      sync.RWMutex
}

func (c *ClientConnection) readPump() {
	defer func() {
		c.Hub.unregister <- c
		_ = c.Conn.Close()
	}()

	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		switch messageType {

		case websocket.TextMessage:
			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error unmarshalling message: %v", err)
				continue
			}
			msg.Timestamp = time.Now()
			log.Printf("Received message: command=%s from=%s", msg.Command, c.ID)
			c.handleMessage(msg)

		case websocket.BinaryMessage:
			c.handleFileChunk(message)
		}

	}
}

type ChunkHeader struct {
	TransferID string `json:"transfer_id"`
	Sequence   int    `json:"sequence"`
	Size       int    `json:"size"`
	IsLast     bool   `json:"is_last"`
}

func (c *ClientConnection) handleFileChunk(msg []byte) {
	// Parse header. The first part of the mesage is a newline terminated JSON string. After
	// that comes the bytes for the file.
	newLineIdx := bytes.IndexByte(msg, '\n')
	if newLineIdx == -1 {
		log.Printf("Error parsing chunk header: %s", msg)
		return
	}
	headerBytes := msg[:newLineIdx]
	chunkBytes := msg[newLineIdx+1:]

	var header ChunkHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		log.Printf("Error parsing chunk header: %v", err)
		return
	}

	// Get the transfer state
	c.transferMu.RLock()
	transfer, ok := c.activeTransfers[header.TransferID]
	c.transferMu.RUnlock()
	if !ok {
		log.Printf("Received chunk for unknown transfer %s", header.TransferID)
		c.sendChunkError(header.TransferID, header.Sequence, "transfer not found")
		return
	}

	// Write chunk
	if err := transfer.writeChunk(header.Sequence, chunkBytes); err != nil {
		log.Printf("Error writing chunk: %v", err)
		c.sendChunkError(header.TransferID, header.Sequence, "transfer not found")
		return
	}

	// transfer.updateProgressIfNeeded()

	c.Send <- Message{
		Command:   MsgChunkAck,
		ID:        header.TransferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id":    header.TransferID,
			"chunk_sequence": header.Sequence,
			"bytes_received": transfer.BytesWritten,
			"next_sequence":  transfer.NextChunkSeq,
		},
	}

	// Broadcast progress to UI clients (every 10 chunks or so)
	//if header.Sequence % 10 == 0 {
	//	c.broadcastProgress(transfer)
	//}
}

func (c *ClientConnection) broadcastProgress(transfer *FileTransfer) {
	progressMsg := Message{
		Command:   MsgUploadProgress,
		ID:        transfer.TransferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id": transfer.TransferID,
			//"file_name":     transfer.FileName,
			"bytes_written": transfer.BytesWritten,
			"expected_size": transfer.ExpectedSize,
			"progress_pct":  float64(transfer.BytesWritten) / float64(transfer.ExpectedSize) * 100,
		},
	}

	_ = progressMsg
	// Broadcast to all UI clients for this user
	//c.Hub.broadcastToUserClients(c.User.ID, "ui", progressMsg)
}

func (c *ClientConnection) sendChunkError(transferID string, sequence int, reason string) {
	c.Send <- Message{
		Command:   "CHUNK_ERROR",
		ID:        transferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id":    transferID,
			"chunk_sequence": sequence,
			"error":          reason,
		},
	}
}

func (c *ClientConnection) handleTransferComplete(msg Message) {
	payload := msg.Payload.(map[string]interface{})
	transferID, _ := payload["transfer_id"].(string)

	// Get transfer
	c.transferMu.Lock()
	transfer, exists := c.activeTransfers[transferID]
	if !exists {
		c.transferMu.Unlock()
		c.sendTransferError(transferID, "transfer not found")
		return
	}
	delete(c.activeTransfers, transferID)
	c.transferMu.Unlock()

	// Finalize the file
	if err := c.finalizeTransfer(transfer); err != nil {
		log.Printf("Error finalizing transfer %s: %v", transferID, err)
		c.sendTransferError(transferID, err.Error())
		return
	}

	// Send success response
	c.Send <- Message{
		Command:   MsgTransferFinalize,
		ID:        msg.ID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id":   transferID,
			"status":        "complete",
			"bytes_written": transfer.BytesWritten,
			"file_name":     transfer.FileName,
		},
	}

	// Notify UI clients
	completeMsg := Message{
		Command:   MsgUploadComplete,
		ID:        transferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id": transferID,
			"file_name":   transfer.FileName,
			"file_size":   transfer.BytesWritten,
		},
	}
	c.Hub.broadcastToUserClients(c.User.ID, "ui", completeMsg)

	log.Printf("Transfer completed: %s (%s, %.2f MB)",
		transferID, transfer.FileName, float64(transfer.BytesWritten)/1024/1024)
}

func (c *ClientConnection) finalizeTransfer(transfer *FileTransfer) error {
	transfer.mu.Lock()
	defer transfer.mu.Unlock()

	// Flush and close file
	if err := transfer.File.Sync(); err != nil {
		return fmt.Errorf("sync error: %v", err)
	}
	transfer.File.Close()

	// Verify file size
	fileInfo, err := os.Stat(transfer.FilePath)
	if err != nil {
		return fmt.Errorf("stat error: %v", err)
	}

	if fileInfo.Size() != transfer.ExpectedSize {
		return fmt.Errorf("size mismatch: expected %d, got %d",
			transfer.ExpectedSize, fileInfo.Size())
	}

	// Optional: Calculate SHA256 hash
	hash, err := calculateFileSHA256(transfer.FilePath)
	if err != nil {
		log.Printf("Warning: could not calculate hash: %v", err)
	}

	// Update DB record to "complete"
	if err := c.Hub.partialTransferFileStor.MarkComplete(transfer.TransferID, hash); err != nil {
		return fmt.Errorf("database error: %v", err)
	}

	// Create entry in main files table (for Laravel UI)
	file := &mcmodel.File{
		Name:        transfer.FileName,
		Path:        transfer.FilePath,
		Size:        uint64(fileInfo.Size()),
		OwnerID:     c.User.ID,
		ProjectID:   transfer.ProjectID,
		DirectoryID: transfer.DirectoryID,
		//TransferID:   transfer.TransferID,
		//UploadStatus: "complete",
		MimeType: mc.DetectMimeType(transfer.FilePath),
		Checksum: hash,
	}

	_ = file

	_, err = c.Hub.fileStor.CreateFile(transfer.FileName, transfer.ProjectID, transfer.DirectoryID, c.User.ID, mc.DetectMimeType(transfer.FilePath))

	if err != nil {
		return fmt.Errorf("file creation error: %v", err)
	}

	return nil
}

func calculateFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (c *ClientConnection) sendTransferError(transferID, reason string) {
	c.Send <- Message{
		Command:   MsgUploadFailed,
		ID:        transferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id": transferID,
			"error":       reason,
		},
	}
}

func (c *ClientConnection) handleTransferResume(msg Message) {
	payload := msg.Payload.(map[string]interface{})
	transferID, _ := payload["transfer_id"].(string)

	// Check if the transfer is already active in memory
	c.transferMu.RLock()
	if _, exists := c.activeTransfers[transferID]; exists {
		c.transferMu.RUnlock()
		// Already active, just return current state
		c.sendResumeResponse(transferID, c.activeTransfers[transferID])
		return
	}
	c.transferMu.RUnlock()

	// Load from database
	partialFile, err := c.Hub.partialTransferFileStor.GetPartialTransferFileByID(transferID)
	if err != nil {
		c.sendTransferReject(transferID, "transfer not found")
		return
	}

	// Verify this transfer belongs to this user
	if partialFile.UserID != c.User.ID {
		c.sendTransferReject(transferID, "unauthorized")
		return
	}

	// Check if already complete
	if partialFile.Status == "complete" {
		c.sendTransferReject(transferID, "already completed")
		return
	}

	// Verify file exists on disk
	fileInfo, err := os.Stat(partialFile.FilePath)
	if err != nil {
		c.sendTransferReject(transferID, "file not found on disk")
		return
	}

	// Open file for writing
	file, err := os.OpenFile(partialFile.FilePath, os.O_WRONLY, 0644)
	if err != nil {
		c.sendTransferReject(transferID, "cannot open file")
		return
	}

	// Calculate where to resume from
	actualSize := fileInfo.Size()
	nextChunkSeq := int(actualSize / int64(partialFile.ChunkSize))

	// Create in-memory transfer state
	transfer := &FileTransfer{
		TransferID:   transferID,
		ProjectID:    partialFile.ProjectID,
		DirectoryID:  partialFile.DirectoryID,
		FileName:     partialFile.FileName,
		FilePath:     partialFile.FilePath,
		File:         file,
		ExpectedSize: partialFile.ExpectedSize,
		BytesWritten: actualSize,
		ChunkSize:    partialFile.ChunkSize,
		NextChunkSeq: nextChunkSeq,
		LastActivity: time.Now(),
		lastDBUpdate: time.Now(),
	}

	c.transferMu.Lock()
	if c.activeTransfers == nil {
		c.activeTransfers = make(map[string]*FileTransfer)
	}
	c.activeTransfers[transferID] = transfer
	c.transferMu.Unlock()

	// Send resume response
	c.sendResumeResponse(transferID, transfer)

	log.Printf("Transfer resumed: %s (from byte %d / %d, %.1f%%)",
		transferID, actualSize, transfer.ExpectedSize,
		float64(actualSize)/float64(transfer.ExpectedSize)*100)
}

func (c *ClientConnection) sendResumeResponse(transferID string, transfer *FileTransfer) {
	c.Send <- Message{
		Command:   MsgTransferResumeResponse,
		ID:        transferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id":       transferID,
			"can_resume":        true,
			"resume_from_byte":  transfer.BytesWritten,
			"resume_from_chunk": transfer.NextChunkSeq,
			"bytes_received":    transfer.BytesWritten,
			"expected_size":     transfer.ExpectedSize,
		},
	}
}

func (c *ClientConnection) handleTransferCancel(msg Message) {
	payload := msg.Payload.(map[string]interface{})
	transferID, _ := payload["transfer_id"].(string)

	// Remove from active transfers
	c.transferMu.Lock()
	transfer, exists := c.activeTransfers[transferID]
	if exists {
		delete(c.activeTransfers, transferID)
	}
	c.transferMu.Unlock()

	if !exists {
		// Not active, but might exist in DB
		c.Hub.partialTransferFileStor.MarkFailed(transferID, "cancelled by user")
		return
	}

	// Close and delete file
	transfer.mu.Lock()
	transfer.File.Close()
	filePath := transfer.FilePath
	transfer.mu.Unlock()

	if err := os.Remove(filePath); err != nil {
		log.Printf("Error removing cancelled transfer file: %v", err)
	}

	// Update DB
	c.Hub.partialTransferFileStor.MarkFailed(transferID, "cancelled by user")

	// Send confirmation
	c.Send <- Message{
		Command:   "TRANSFER_CANCELLED",
		ID:        transferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id": transferID,
		},
	}

	log.Printf("Transfer cancelled: %s", transferID)
}

func (c *ClientConnection) handleTransferInit(msg Message) {
	payload := msg.Payload.(map[string]interface{})

	transferID, _ := payload["transfer_id"].(string)
	fileName, _ := payload["file_name"].(string)
	fileSize, _ := payload["file_size"].(float64) // JSON numbers are float64
	chunkSize, _ := payload["chunk_size"].(float64)
	projectID, _ := payload["project_id"].(float64)
	directoryID, _ := payload["directory_id"].(float64)

	// Validate
	if transferID == "" || fileName == "" || fileSize <= 0 {
		c.sendTransferReject(transferID, "invalid parameters")
		return
	}

	// Check if user has access to project
	//if !c.hasAccessToProject(int(projectID)) {
	//	c.sendTransferReject(transferID, "no access to project")
	//	return
	//}

	// Create file path (your storage convention)
	// Example: /storage/projects/{projectID}/uploads/{transferID}_{fileName}
	filePath := "" // c.Hub.buildUploadPath(int(projectID), transferID, fileName)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		c.sendTransferReject(transferID, "cannot create directory")
		return
	}

	// Create the file (pre-allocated)
	file, err := os.Create(filePath)
	if err != nil {
		c.sendTransferReject(transferID, "cannot create file")
		return
	}

	// Optional: pre-allocate disk space
	if err := file.Truncate(int64(fileSize)); err != nil {
		file.Close()
		c.sendTransferReject(transferID, "cannot allocate space")
		return
	}

	// Create DB record
	partialFile := &mcmodel.PartialTransferFile{
		TransferID:   transferID,
		UserID:       c.User.ID,
		ProjectID:    int(projectID),
		DirectoryID:  int(directoryID),
		FileName:     fileName,
		FilePath:     filePath,
		ExpectedSize: int64(fileSize),
		ChunkSize:    int(chunkSize),
		Status:       "uploading",
	}

	if _, err := c.Hub.partialTransferFileStor.CreatePartialTransferFile(partialFile); err != nil {
		file.Close()
		os.Remove(filePath)
		c.sendTransferReject(transferID, "database error")
		return
	}

	// Create in-memory transfer state
	transfer := &FileTransfer{
		TransferID: transferID,
		//ProjectID:    int(projectID),
		//DirectoryID:  int(directoryID),
		//FileName:     fileName,
		//FilePath:     filePath,
		File:         file,
		ExpectedSize: int64(fileSize),
		BytesWritten: 0,
		ChunkSize:    int(chunkSize),
		NextChunkSeq: 0,
		LastActivity: time.Now(),
		lastDBUpdate: time.Now(),
	}

	c.transferMu.Lock()
	if c.activeTransfers == nil {
		c.activeTransfers = make(map[string]*FileTransfer)
	}
	c.activeTransfers[transferID] = transfer
	c.transferMu.Unlock()

	// Send acceptance
	c.Send <- Message{
		Command:   MsgTransferAccept,
		ID:        msg.ID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id":     transferID,
			"chunk_size":      int(chunkSize),
			"expected_chunks": int(fileSize) / int(chunkSize),
		},
	}

	log.Printf("Transfer initialized: %s (%s, %.2f MB)", transferID, fileName, fileSize/1024/1024)
}

//func (c *ClientConnection) handleTransferResume(transferID string) {
//	// Load from DB
//	partial, err := c.Hub.partialTransferFileStor.GetPartialTransferFileByID(transferID)
//	if err != nil {
//		c.sendTransferReject(transferID, "not found")
//		return
//	}
//
//	// Check actual file size on disk
//	fileInfo, err := os.Stat(partial.FilePath)
//	if err != nil {
//		c.sendTransferReject(transferID, "file missing")
//		return
//	}
//
//	// Open file in append mode
//	file, err := os.OpenFile(partial.FilePath, os.O_WRONLY, 0644)
//	if err != nil {
//		c.sendTransferReject(transferID, "cannot open file")
//		return
//	}
//
//	// Create in-memory transfer state
//	transfer := &FileTransfer{
//		TransferID:   transferID,
//		File:         file,
//		ExpectedSize: partial.ExpectedSize,
//		BytesWritten: fileInfo.Size(),
//		NextChunkSeq: int(fileInfo.Size() / int64(partial.ChunkSize)),
//	}
//
//	c.activeTransfers[transferID] = transfer
//
//	// Tell client where to resume from
//	c.Send <- Message{
//		Command: MsgTransferResumeResponse,
//		Payload: map[string]interface{}{
//			"transfer_id":       transferID,
//			"resume_from_byte":  fileInfo.Size(),
//			"resume_from_chunk": transfer.NextChunkSeq,
//		},
//	}
//}

func (c *ClientConnection) sendTransferReject(transferID, reason string) {
	c.Send <- Message{
		Command:   MsgTransferReject,
		ID:        transferID,
		Timestamp: time.Now(),
		ClientID:  c.ID,
		Payload: map[string]interface{}{
			"transfer_id": transferID,
			"reason":      reason,
		},
	}
}

func (c *ClientConnection) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *ClientConnection) handleMessage(msg Message) {
	fmt.Println("clienConnection::handleMessage", msg.Command)

	switch msg.Command {
	case MsgUploadStart, MsgUploadPause, MsgUploadResume, MsgUploadCancel, MsgGetStatus:
		// Forward control messages to target Python client
		c.Hub.broadcast <- msg

	case MsgUploadProgress, MsgUploadComplete, MsgUploadFailed, MsgClientStatus:
		// Forward status messages to Laravel UI
		c.Hub.broadcast <- msg

	case MsgListProjects:
		c.handleListProjects(msg)

	case MsgHeartbeat:
		// Respond to heartbeat
		response := Message{
			Command:   "HEARTBEAT_ACK",
			ID:        msg.ID,
			Timestamp: time.Now(),
			ClientID:  msg.ClientID,
		}
		c.Send <- response

	case MsgTransferInit:
		c.handleTransferInit(msg)

	case MsgTransferComplete:
		c.handleTransferComplete(msg)

	case MsgTransferResume:
		c.handleTransferResume(msg)

	case MsgTransferCancel:
		c.handleTransferCancel(msg)

	case MsgClientConnected:
		log.Printf("ClientConnection %s connected", msg.ClientID)

	case MsgClientDisconnected:
		log.Printf("ClientConnection %s disconnected", msg.ClientID)
	}
}

type ProjectItem struct {
	Directory string `json:"directory"`
	ProjectID int    `json:"project_id"`
}

func (c *ClientConnection) handleListProjects(msg Message) {
	//fmt.Printf("handleListProjects: %+v\n", msg.Payload)
	projectsList := msg.Payload.([]interface{})
	for _, projectItem := range projectsList {
		projectItem := toProjectItem(projectItem.(map[string]interface{}))
		_ = projectItem
		//fmt.Printf("projectItem: %+v\n", projectItem)
	}
}

func toProjectItem(project map[string]interface{}) ProjectItem {
	return ProjectItem{
		Directory: project["directory"].(string),
		ProjectID: int(project["project_id"].(float64)),
	}
}

func (c *ClientConnection) handleHeartbeat(msg Message) error {
	response := Message{
		Command:   "HEARTBEAT_ACK",
		ID:        msg.ID,
		Timestamp: time.Now(),
		ClientID:  msg.ClientID,
	}
	c.Send <- response

	return nil
}
