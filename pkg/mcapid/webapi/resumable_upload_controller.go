package webapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
)

// These type aliases are here to make the chunks map more readable. They help to define each point of the map.
type userID = int            // The user ID of the user doing an upload
type fileID = int            // The file ID of the file chunks are being uploaded for
type currentChunkIndex = int // The last chunk index successfully uploaded.

type ResumableUploadInstanceState struct {
	LastChunkIndexUploaded int
	ExpectedSize           int64
	ExpectedChecksum       string
}

// ResumableUploadController handles file uploads with support for unlimited size and restartable uploads.
type ResumableUploadController struct {
	fileStor stor.FileStor
	mu       sync.Mutex
	chunks   map[userID]map[fileID]currentChunkIndex
}

// NewResumableUploadController creates a new ResumableUploadController.
func NewResumableUploadController(fileStor stor.FileStor) *ResumableUploadController {
	return &ResumableUploadController{
		fileStor: fileStor,
		chunks:   make(map[userID]map[fileID]currentChunkIndex),
	}
}

// UploadRequest contains the information needed to upload a file.
type UploadRequest struct {
	ProjectID  int `json:"project_id"`
	FileID     int `json:"file_id"`
	ChunkIndex int `json:"chunk_index"`
}

func (c *ResumableUploadController) StartUpload(ctx echo.Context) error {
	var (
		req struct {
			ProjectID       int    `json:"project_id"`
			DestinationPath string `json:"destination_path"`
			TotalChunks     int    `json:"total_chunks"`
			TotalSize       int64  `json:"total_size"`
			FileID          *int   `json:"file_id,omitempty"`
		}
	)

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	// Get the user from the context
	user := ctx.Get("user").(*mcmodel.User)
	if user == nil {
		return errorResponse(ctx, http.StatusUnauthorized, "User not authenticated")
	}

	// Get or create the directory for the file
	dirPath := filepath.Dir(req.DestinationPath)
	dir, err := c.fileStor.GetOrCreateDirPath(req.ProjectID, user.ID, dirPath)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, "Failed to create directory")
	}

	// Get or create the Materials Commons file instance
	fileName := filepath.Base(req.DestinationPath)
	file, err := c.getOrCreateMCFile(req.ProjectID, user.ID, dir.ID, req.FileID, fileName)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, "Failed to create file")
	}

	// Create the chunk temporary directory if it doesn't exist
	_, err = c.getAndCreateChunkDirPath(req.ProjectID, user.ID, file.ID)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, "Failed to create chunks directory")
	}

	// Get or retrieve the current upload state for this file
	state := c.getOrCreateUploadState(req.ProjectID, user.ID, file.ID)
	_ = state

	startingChunk := c.computeStartingChunk(req.ProjectID, user.ID, file.ID)

	// TODO: Compute the starting chunk index based on what has already been uploaded.
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"file_id":        file.ID,
		"starting_chunk": startingChunk,
	})
}

// computeStartingChunk computes the starting chunk index based on the number of chunks already uploaded.
func (c *ResumableUploadController) computeStartingChunk(projectID, userID, fileID int) int {
	// TODO: Compute the starting chunk index based on what has already been uploaded.
	return 1
}

// UploadChunk handles file uploads with support for multiple chunks being sent simultaneously.
// It reads the file data from the request body and writes it to a chunk file.
// Each chunk is stored separately and will be combined when the upload is finalized.
func (c *ResumableUploadController) UploadChunk(ctx echo.Context) error {
	var uploadRequest UploadRequest
	if err := bindUploadRequest(ctx, &uploadRequest); err != nil {
		return err
	}

	// Get the user from the context
	user := ctx.Get("user").(*mcmodel.User)
	if user == nil {
		return errorResponse(ctx, http.StatusUnauthorized, "User not authenticated")
	}

	//// Get or create the directory for the file
	//dirPath := filepath.Dir(uploadRequest.DestinationPath)
	//dir, err := c.fileStor.GetOrCreateDirPath(uploadRequest.ProjectID, user.ID, dirPath)
	//if err != nil {
	//	return errorResponse(ctx, http.StatusInternalServerError, "Failed to create directory")
	//}
	//
	//// Get or create the file
	//fileName := filepath.Base(uploadRequest.DestinationPath)
	//file, err := c.getOrCreateMCFile(uploadRequest.ProjectID, user.ID, dir.ID, uploadRequest.FileID, fileName)
	//if err != nil {
	//	return errorResponse(ctx, http.StatusInternalServerError, "Failed to create file")
	//}

	// Create a temporary directory for chunks if it doesn't exist
	//_, err = c.getAndCreateChunkDirPath(uploadRequest.ProjectID, user.ID, file.ID)
	//if err != nil {
	//	return errorResponse(ctx, http.StatusInternalServerError, "Failed to create chunks directory")
	//}

	// Create a file for this chunk
	chunkPath := c.getChunkPath(uploadRequest.ProjectID, user.ID, uploadRequest.FileID, uploadRequest.ChunkIndex)
	chunkFile, err := os.Create(chunkPath)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, "Failed to create chunk file")
	}
	defer chunkFile.Close()

	// Copy the request body to the chunk file
	written, err := io.Copy(chunkFile, ctx.Request().Body)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, "Failed to write to chunk file")
	}

	// Store the last chunk written in the chunk's state map
	c.mu.Lock()
	if _, ok := c.chunks[uploadRequest.FileID]; !ok {
		c.chunks[user.ID][uploadRequest.FileID] = uploadRequest.ChunkIndex
	}
	c.mu.Unlock()

	// Return the chunk info
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"chunk_index":   uploadRequest.ChunkIndex,
		"bytes_written": written,
	})
}

func bindUploadRequest(ctx echo.Context, uploadRequest *UploadRequest) error {
	var (
		err error
	)
	// Parse the request parameters
	uploadRequest.ProjectID, err = strconv.Atoi(ctx.QueryParam("project_id"))
	if err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "Invalid project ID")
	}

	//uploadRequest.DestinationPath = ctx.QueryParam("destination_path")
	//if uploadRequest.DestinationPath == "" {
	//	return errorResponse(ctx, http.StatusBadRequest, "Destination path is required")
	//}

	uploadRequest.ChunkIndex, err = strconv.Atoi(ctx.QueryParam("chunk_index"))
	if err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "Invalid chunk index")
	}

	//uploadRequest.TotalChunks, err = strconv.Atoi(ctx.QueryParam("total_chunks"))
	//if err != nil || uploadRequest.TotalChunks <= 0 {
	//	return errorResponse(ctx, http.StatusBadRequest, "Invalid total chunks")
	//}

	//fileID := ctx.QueryParam("file_id")
	//if fileID != "" {
	//	*uploadRequest.FileID, err = strconv.Atoi(fileID)
	//	if err != nil {
	//		return errorResponse(ctx, http.StatusBadRequest, "Invalid file ID")
	//	}
	//}

	return nil
}

func errorResponse(ctx echo.Context, httpError int, msg string) error {
	return ctx.JSON(httpError, map[string]string{"error": msg})
}

// GetUploadStatus returns the current status of a file upload, including the file size and chunk information.
func (c *ResumableUploadController) GetUploadStatus(ctx echo.Context) error {
	// Parse the request parameters
	fileID, err := strconv.Atoi(ctx.QueryParam("file_id"))
	if err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "Invalid file ID")
	}

	// Get the file from the database
	file, err := c.fileStor.GetFileByID(fileID)
	if err != nil {
		return errorResponse(ctx, http.StatusNotFound, "File not found")
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
	// Get the user from the context
	user := ctx.Get("user").(*mcmodel.User)
	if user == nil {
		return errorResponse(ctx, http.StatusUnauthorized, "User not authenticated")
	}

	// Parse the request parameters
	fileID, err := strconv.Atoi(ctx.QueryParam("file_id"))
	if err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "Invalid file ID")
	}

	totalChunks, err := strconv.Atoi(ctx.QueryParam("total_chunks"))
	if err != nil || totalChunks <= 0 {
		return errorResponse(ctx, http.StatusBadRequest, "Invalid total chunks")
	}

	// Get the file from the database
	file, err := c.fileStor.GetFileByID(fileID)
	if err != nil {
		return errorResponse(ctx, http.StatusNotFound, "Failed not found")
	}

	// Check if we have all the chunks
	c.mu.Lock()

	// Check if the user has an upload
	userUploadsMap, hasUserUploads := c.chunks[user.ID]
	if !hasUserUploads {
		c.mu.Unlock()
		return errorResponse(ctx, http.StatusBadRequest, "No chunks found for this user")
	}

	_, hasChunks := userUploadsMap[fileID]
	if !hasChunks {
		c.mu.Unlock()
		return errorResponse(ctx, http.StatusBadRequest, "No chunks found for this file")
	}

	c.mu.Unlock()

	chunkIndices := c.createSortedListOfChunkIds(file.ProjectID, file.OwnerID, fileID)

	if len(chunkIndices) != totalChunks {
		return errorResponse(ctx, http.StatusBadRequest, fmt.Sprintf("Expected %d chunks, but found %d", totalChunks, len(chunkIndices)))
	}

	// Create the final file
	filePath := file.ToUnderlyingFilePath(c.fileStor.Root())
	finalFile, err := os.Create(filePath)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, fmt.Sprintf("Failed to create final file: %v", err))
	}
	defer finalFile.Close()

	// Combine all chunks in order
	var totalSize int64 = 0
	for _, idx := range chunkIndices {

		chunkPath := c.getChunkPath(file.ProjectID, file.OwnerID, fileID, idx)
		// Read the chunk file
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			return errorResponse(ctx, http.StatusInternalServerError, fmt.Sprintf("Failed to read chunk %d: %v", idx, err))
		}

		// Write the chunk to the final file
		written, err := finalFile.Write(chunkData)
		if err != nil {
			return errorResponse(ctx, http.StatusInternalServerError, fmt.Sprintf("Failed to write chunk %d to final file: %v", idx, err))
		}

		totalSize += int64(written)
	}

	// Clean up the chunks
	c.mu.Lock()
	delete(c.chunks[user.ID], fileID)
	// Check if the user has any more upload requests
	if len(c.chunks[user.ID]) == 0 {
		delete(c.chunks, user.ID)
	}
	c.mu.Unlock()

	// Remove the chunks directory
	chunksDir := filepath.Join(c.fileStor.Root(), "chunks", fmt.Sprintf("file_%d", fileID))
	os.RemoveAll(chunksDir)

	// Return the final file info
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"file_id":   file.ID,
		"file_uuid": file.UUID,
		"file_size": totalSize,
		"chunks":    totalChunks,
		"finalized": true,
	})
}

func (c *ResumableUploadController) createSortedListOfChunkIds(projectID, userID, fileID int) []int {
	chunkDirEntries, err := os.ReadDir(c.getChunkDirPath(projectID, userID, fileID))
	if err != nil {
		return nil
	}
	chunkIds := make([]int, 0, len(chunkDirEntries))
	for _, entry := range chunkDirEntries {
		if entry.Name() == "upload-state.json" {
			continue
		}
		chunkIndex, _ := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".chunk"))
		chunkIds = append(chunkIds, chunkIndex)
	}

	sort.Ints(chunkIds)
	return chunkIds
}

func (c *ResumableUploadController) getChunkDirPath(projectID, userID, fileID int) string {
	return filepath.Join(c.fileStor.Root(), "__chunks", fmt.Sprintf("%d", projectID),
		fmt.Sprintf("%d", userID),
		fmt.Sprintf("%d", fileID))
}

func (c *ResumableUploadController) getChunkPath(projectID, userID, fileID, chunkIndex int) string {
	return filepath.Join(c.getChunkDirPath(projectID, userID, fileID), fmt.Sprintf("%d.chunk", chunkIndex))
}

func (c *ResumableUploadController) getAndCreateChunkDirPath(projectID, userID, fileID int) (string, error) {
	chunkDirPath := c.getChunkDirPath(projectID, userID, fileID)
	if err := os.MkdirAll(chunkDirPath, 0755); err != nil {
		return "", err
	}
	return chunkDirPath, nil
}

// getOrCreateMCFile gets an existing file or creates a new one if it doesn't exist.
func (c *ResumableUploadController) getOrCreateMCFile(projectID, ownerID, dirID int, fileID *int, fileName string) (*mcmodel.File, error) {
	if fileID != nil {
		// if fileID is set, try to get the file
		file, err := c.fileStor.GetFileByID(*fileID)
		if err == nil {
			return file, nil
		}

		// If we couldn't get the file, then return an error, as the user is trying to upload to an existing file
		// upload instance.
		return nil, fmt.Errorf("no such file ID %d", *fileID)
	}

	// If fileID is not set, then we are creating a new file.

	// 1. Make sure the directory exists
	dir, err := c.fileStor.GetFileByID(dirID)
	if err != nil {
		return nil, err
	}

	// 2. Create the file.
	//   Directory exists, and we know the ownerID and projectID are good because the middleware already checked them.
	//   The created file will have the current entry set to false. We will set it to true when the upload is finalized.
	return c.fileStor.CreateFile(fileName, projectID, ownerID, dir.ID, getMimeType(fileName))
}

func (c *ResumableUploadController) getOrCreateUploadState(projectID int, ownerID int, fileID int) *ResumableUploadInstanceState {
	//finfo, err := os.Stat(c.getUploadState(projectID, ownerID, fileID))
	return nil
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
