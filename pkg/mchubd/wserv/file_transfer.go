package wserv

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// FileTransfer represents a file transfer in progress.
type FileTransfer struct {
	TransferID           string
	File                 *os.File
	ProjectID            int
	DirectoryID          int
	FileID               int
	OwnerID              int
	remoteClientTransfer *mcmodel.RemoteClientTransfer
	FileName             string
	RemoteFilePath       string
	ProjectFilePath      string
	ExpectedSize         int64
	BytesWritten         int64
	ChunkSize            int
	NextChunkSeq         int
	LastActivity         time.Time

	// For periodic DB updates
	chunksSinceUpdate int
	lastDBUpdate      time.Time

	mu sync.Mutex
}

// writeChunk writes a chunk of data to the file.
func (tf *FileTransfer) writeChunk(seq int, chunk []byte) error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Enforce sequential chunks
	if seq != tf.NextChunkSeq {
		return fmt.Errorf("expected chunk %d, got %d", tf.NextChunkSeq, seq)
	}

	// Write at correct offset
	offset := int64(seq) * int64(tf.ChunkSize)
	n, err := tf.File.WriteAt(chunk, offset)
	if err != nil {
		return fmt.Errorf("write error: %v", err)
	}

	tf.BytesWritten += int64(n)
	tf.NextChunkSeq++
	tf.LastActivity = time.Now()
	//tf.chunksSinceUpdate++

	return nil
}

// updateProgressIfNeeded updates the progress in the DB if needed.
func (tf *FileTransfer) updateProgressIfNeeded(stor stor.GormPartialTransferFileStor) {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	// Update DB every 100 chunks or 30 seconds
	shouldUpdate := tf.chunksSinceUpdate >= 100 ||
		time.Since(tf.lastDBUpdate) > 30*time.Second

	if shouldUpdate {
		if err := stor.UpdateFileSize(tf.TransferID, tf.BytesWritten); err != nil {
			log.Printf("Error updating file size: %v", err)
		}
		tf.chunksSinceUpdate = 0
		tf.lastDBUpdate = time.Now()
	}
}
