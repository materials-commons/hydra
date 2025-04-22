package webapi

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// ResumableUploadController handles file uploads with support for unlimited size and restartable uploads.
type ResumableUploadController struct {
	fileStor stor.FileStor
	mu       sync.Mutex
	chunks   map[int]map[int]string // map[fileID]map[chunkIndex]chunkPath
}

// NewResumableUploadController creates a new ResumableUploadController.
func NewResumableUploadController(fileStor stor.FileStor) *ResumableUploadController {
	return &ResumableUploadController{
		fileStor: fileStor,
		chunks:   make(map[int]map[int]string),
	}
}

// UploadRequest contains the information needed to upload a file.
type UploadRequest struct {
	ProjectID       int    `json:"project_id"`
	DestinationPath string `json:"destination_path"`
	ChunkIndex      int    `json:"chunk_index"`
	TotalChunks     int    `json:"total_chunks"`
	TotalSize       int64  `json:"total_size"`
}

// Upload handles file uploads with support for multiple chunks being sent simultaneously.
// It reads the file data from the request body and writes it to a chunk file.
// Each chunk is stored separately and will be combined when the upload is finalized.
func (c *ResumableUploadController) Upload(ctx echo.Context) error {
	// Parse the request parameters
	projectID, err := strconv.Atoi(ctx.QueryParam("project_id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid project ID"})
	}

	destinationPath := ctx.QueryParam("destination_path")
	if destinationPath == "" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Destination path is required"})
	}

	chunkIndex, err := strconv.Atoi(ctx.QueryParam("chunk_index"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid chunk index"})
	}

	totalChunks, err := strconv.Atoi(ctx.QueryParam("total_chunks"))
	if err != nil || totalChunks <= 0 {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid total chunks"})
	}

	// Get the user from the context
	user := ctx.Get("user").(*mcmodel.User)
	if user == nil {
		return ctx.JSON(http.StatusUnauthorized, map[string]string{"error": "User not authenticated"})
	}

	// Get or create the directory for the file
	dirPath := filepath.Dir(destinationPath)
	dir, err := c.fileStor.GetOrCreateDirPath(projectID, user.ID, dirPath)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create directory"})
	}

	// Get or create the file
	fileName := filepath.Base(destinationPath)
	file, err := c.getOrCreateFile(projectID, user.ID, dir.ID, fileName)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create file"})
	}

	// Create a temporary directory for chunks if it doesn't exist
	chunksDir := filepath.Join(c.fileStor.Root(), "chunks", fmt.Sprintf("file_%d", file.ID))
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create chunks directory"})
	}

	// Create a file for this chunk
	chunkPath := filepath.Join(chunksDir, fmt.Sprintf("chunk_%d", chunkIndex))
	chunkFile, err := os.Create(chunkPath)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create chunk file"})
	}
	defer chunkFile.Close()

	// Copy the request body to the chunk file
	written, err := io.Copy(chunkFile, ctx.Request().Body)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to write to chunk file"})
	}

	// Store the chunk path in the chunks map
	c.mu.Lock()
	if _, ok := c.chunks[file.ID]; !ok {
		c.chunks[file.ID] = make(map[int]string)
	}
	c.chunks[file.ID][chunkIndex] = chunkPath
	c.mu.Unlock()

	// Return the chunk info
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"file_id":       file.ID,
		"file_uuid":     file.UUID,
		"chunk_index":   chunkIndex,
		"total_chunks":  totalChunks,
		"bytes_written": written,
	})
}

// GetUploadStatus returns the current status of a file upload, including the file size and chunk information.
func (c *ResumableUploadController) GetUploadStatus(ctx echo.Context) error {
	// Parse the request parameters
	fileID, err := strconv.Atoi(ctx.QueryParam("file_id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid file ID"})
	}

	// Get the file from the database
	file, err := c.fileStor.GetFileByID(fileID)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, map[string]string{"error": "File not found"})
	}

	// Get the file size if it exists
	filePath := file.ToUnderlyingFilePath(c.fileStor.Root())
	fileExists := false
	var fileSize int64 = 0

	fileInfo, err := os.Stat(filePath)
	if err == nil {
		fileExists = true
		fileSize = fileInfo.Size()
	}

	// Get chunk information
	c.mu.Lock()
	chunkMap, hasChunks := c.chunks[fileID]
	chunkCount := 0
	if hasChunks {
		chunkCount = len(chunkMap)
	}
	c.mu.Unlock()

	// Return the file status
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"file_id":     file.ID,
		"file_uuid":   file.UUID,
		"file_size":   fileSize,
		"exists":      fileExists,
		"has_chunks":  hasChunks,
		"chunk_count": chunkCount,
	})
}

// FinalizeUpload combines all chunks for a file into a single file in the correct order.
func (c *ResumableUploadController) FinalizeUpload(ctx echo.Context) error {
	// Parse the request parameters
	fileID, err := strconv.Atoi(ctx.QueryParam("file_id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid file ID"})
	}

	totalChunks, err := strconv.Atoi(ctx.QueryParam("total_chunks"))
	if err != nil || totalChunks <= 0 {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid total chunks"})
	}

	// Get the file from the database
	file, err := c.fileStor.GetFileByID(fileID)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, map[string]string{"error": "File not found"})
	}

	// Check if we have all the chunks
	c.mu.Lock()
	chunkMap, hasChunks := c.chunks[fileID]
	if !hasChunks {
		c.mu.Unlock()
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "No chunks found for this file"})
	}

	if len(chunkMap) != totalChunks {
		c.mu.Unlock()
		return ctx.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Expected %d chunks, but found %d", totalChunks, len(chunkMap)),
		})
	}

	// Get all chunk indices and sort them
	chunkIndices := make([]int, 0, len(chunkMap))
	for idx := range chunkMap {
		chunkIndices = append(chunkIndices, idx)
	}
	sort.Ints(chunkIndices)
	c.mu.Unlock()

	// Create the final file
	filePath := file.ToUnderlyingFilePath(c.fileStor.Root())
	finalFile, err := os.Create(filePath)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create final file"})
	}
	defer finalFile.Close()

	// Combine all chunks in order
	var totalSize int64 = 0
	for _, idx := range chunkIndices {
		c.mu.Lock()
		chunkPath := chunkMap[idx]
		c.mu.Unlock()

		// Read the chunk file
		chunkData, err := ioutil.ReadFile(chunkPath)
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to read chunk %d: %v", idx, err),
			})
		}

		// Write the chunk to the final file
		written, err := finalFile.Write(chunkData)
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to write chunk %d to final file: %v", idx, err),
			})
		}

		totalSize += int64(written)
	}

	// Clean up the chunks
	c.mu.Lock()
	chunksDir := filepath.Join(c.fileStor.Root(), "chunks", fmt.Sprintf("file_%d", fileID))
	delete(c.chunks, fileID)
	c.mu.Unlock()

	// Remove the chunks directory
	os.RemoveAll(chunksDir)

	// Return the final file info
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"file_id":    file.ID,
		"file_uuid":  file.UUID,
		"file_size":  totalSize,
		"chunks":     totalChunks,
		"finalized":  true,
	})
}

// getOrCreateFile gets an existing file or creates a new one if it doesn't exist.
func (c *ResumableUploadController) getOrCreateFile(projectID, ownerID, dirID int, fileName string) (*mcmodel.File, error) {
	// Get the directory
	dir, err := c.fileStor.GetFileByID(dirID)
	if err != nil {
		return nil, err
	}

	// Try to get the file first
	filePath := filepath.Join(dir.Path, fileName)
	file, err := c.fileStor.GetFileByPath(projectID, filePath)
	if err == nil {
		return file, nil
	}

	// Create the file if it doesn't exist
	return c.fileStor.CreateFile(fileName, projectID, ownerID, dirID, getMimeType(fileName))
}

// getMimeType returns the MIME type for a file based on its extension.
func getMimeType(fileName string) string {
	ext := filepath.Ext(fileName)
	switch ext {
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}