package webapi

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupEchoContext creates a test Echo context with the given request
func setupEchoContext(t *testing.T, method, target string, body []byte, queryParams map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()

	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	// Add query parameters
	q := req.URL.Query()
	for key, value := range queryParams {
		q.Add(key, value)
	}
	req.URL.RawQuery = q.Encode()

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Add user to context
	user := &mcmodel.User{
		ID: 1,
	}
	c.Set("user", user)

	return c, rec
}

// TestUpload tests the UploadChunk method
func TestUpload(t *testing.T) {
	// Setup
	mockFileStor := stor.NewMockFileStor()
	defer mockFileStor.Cleanup()

	controller := NewResumableUploadController(mockFileStor)

	// Create test directory
	_, err := mockFileStor.GetOrCreateDirPath(1, 1, "/test")
	require.NoError(t, err)

	t.Run("SuccessfulUpload", func(t *testing.T) {
		// Setup request
		queryParams := map[string]string{
			"project_id":       "1",
			"destination_path": "/test/file.txt",
			"chunk_index":      "0",
			"total_chunks":     "3",
		}

		body := []byte("This is chunk 0")
		ctx, rec := setupEchoContext(t, http.MethodPost, "/resumable-upload/upload", body, queryParams)

		// Execute
		err := controller.UploadChunk(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify response
		response := rec.Body.String()
		assert.Contains(t, response, "file_id")
		assert.Contains(t, response, "chunk_index")
		assert.Contains(t, response, "total_chunks")
		assert.Contains(t, response, "bytes_written")

		// Verify chunk was stored
		fileID := 0
		if strings.Contains(response, `"file_id":2`) {
			fileID = 2
		}
		assert.NotEqual(t, 0, fileID, "File ID should be extracted from response")

		// Check if chunk exists in the controller's chunks map
		controller.mu.Lock()
		chunks, ok := controller.chunks[fileID]
		controller.mu.Unlock()

		assert.True(t, ok, "Chunks map should contain an entry for the file ID")
		assert.Equal(t, 1, len(chunks), "There should be one chunk stored")
		assert.Contains(t, chunks, 0, "Chunk index 0 should be stored")
	})

	t.Run("MissingParameters", func(t *testing.T) {
		// Test with missing project_id
		queryParams := map[string]string{
			"destination_path": "/test/file.txt",
			"chunk_index":      "0",
			"total_chunks":     "3",
		}

		body := []byte("This is chunk 0")
		ctx, rec := setupEchoContext(t, http.MethodPost, "/resumable-upload/upload", body, queryParams)

		err := controller.UploadChunk(ctx)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Invalid project ID")

		// Test with missing destination_path
		queryParams = map[string]string{
			"project_id":   "1",
			"chunk_index":  "0",
			"total_chunks": "3",
		}

		ctx, rec = setupEchoContext(t, http.MethodPost, "/resumable-upload/upload", body, queryParams)

		err = controller.UploadChunk(ctx)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Destination path is required")
	})
}

// TestGetUploadStatus tests the GetUploadStatus method
func TestGetUploadStatus(t *testing.T) {
	// Setup
	mockFileStor := stor.NewMockFileStor()
	defer mockFileStor.Cleanup()

	controller := NewResumableUploadController(mockFileStor)

	// Create test directory and file
	dir, err := mockFileStor.GetOrCreateDirPath(1, 1, "/test")
	require.NoError(t, err)

	file, err := mockFileStor.CreateFile("file.txt", 1, dir.ID, 1, "text/plain")
	require.NoError(t, err)

	t.Run("FileWithNoChunks", func(t *testing.T) {
		// Setup request
		queryParams := map[string]string{
			"file_id": strconv.Itoa(file.ID),
		}

		ctx, rec := setupEchoContext(t, http.MethodGet, "/resumable-upload/status", nil, queryParams)

		// Execute
		err := controller.GetUploadStatus(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify response
		response := rec.Body.String()
		assert.Contains(t, response, `"file_id":`)
		assert.Contains(t, response, `"file_uuid":`)
		assert.Contains(t, response, `"has_chunks":false`)
		assert.Contains(t, response, `"chunk_count":0`)
	})

	t.Run("FileWithChunks", func(t *testing.T) {
		// Add chunks to the controller
		controller.mu.Lock()
		controller.chunks[file.ID] = map[int]string{
			0: "chunk0",
			1: "chunk1",
		}
		controller.mu.Unlock()

		// Setup request
		queryParams := map[string]string{
			"file_id": strconv.Itoa(file.ID),
		}

		ctx, rec := setupEchoContext(t, http.MethodGet, "/resumable-upload/status", nil, queryParams)

		// Execute
		err := controller.GetUploadStatus(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify response
		response := rec.Body.String()
		assert.Contains(t, response, `"file_id":`)
		assert.Contains(t, response, `"file_uuid":`)
		assert.Contains(t, response, `"has_chunks":true`)
		assert.Contains(t, response, `"chunk_count":2`)
	})

	t.Run("InvalidFileID", func(t *testing.T) {
		// Setup request with invalid file ID
		queryParams := map[string]string{
			"file_id": "invalid",
		}

		ctx, rec := setupEchoContext(t, http.MethodGet, "/resumable-upload/status", nil, queryParams)

		// Execute
		err := controller.GetUploadStatus(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Invalid file ID")
	})

	t.Run("NonexistentFile", func(t *testing.T) {
		// Setup request with nonexistent file ID
		queryParams := map[string]string{
			"file_id": "999",
		}

		ctx, rec := setupEchoContext(t, http.MethodGet, "/resumable-upload/status", nil, queryParams)

		// Execute
		err := controller.GetUploadStatus(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "File not found")
	})
}

// TestFinalizeUpload tests the FinalizeUpload method
func TestFinalizeUpload(t *testing.T) {
	// Setup
	mockFileStor := stor.NewMockFileStor()
	defer mockFileStor.Cleanup()

	controller := NewResumableUploadController(mockFileStor)

	// Create test directory and file
	dir, err := mockFileStor.GetOrCreateDirPath(1, 1, "/test")
	require.NoError(t, err)

	file, err := mockFileStor.CreateFile("file.txt", 1, dir.ID, 1, "text/plain")
	require.NoError(t, err)

	// Create temporary chunk files
	chunksDir := filepath.Join(mockFileStor.Root(), "chunks", fmt.Sprintf("file_%d", file.ID))
	os.MkdirAll(chunksDir, 0755)

	chunk0Path := filepath.Join(chunksDir, "chunk_0")
	chunk1Path := filepath.Join(chunksDir, "chunk_1")
	chunk2Path := filepath.Join(chunksDir, "chunk_2")

	_ = os.WriteFile(chunk0Path, []byte("Chunk 0 data"), 0644)
	_ = os.WriteFile(chunk1Path, []byte("Chunk 1 data"), 0644)
	_ = os.WriteFile(chunk2Path, []byte("Chunk 2 data"), 0644)

	// Add chunks to the controller
	controller.mu.Lock()
	controller.chunks[file.ID] = map[int]string{
		0: chunk0Path,
		1: chunk1Path,
		2: chunk2Path,
	}
	controller.mu.Unlock()

	t.Run("SuccessfulFinalization", func(t *testing.T) {
		// Setup request
		queryParams := map[string]string{
			"file_id":      strconv.Itoa(file.ID),
			"total_chunks": "3",
		}

		ctx, rec := setupEchoContext(t, http.MethodPost, "/resumable-upload/finalize", nil, queryParams)

		// Execute
		err := controller.FinalizeUpload(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify response
		response := rec.Body.String()
		assert.Contains(t, response, `"file_id":`)
		assert.Contains(t, response, `"file_uuid":`)
		assert.Contains(t, response, `"file_size":`)
		assert.Contains(t, response, `"chunks":3`)
		assert.Contains(t, response, `"finalized":true`)

		// Verify chunks were removed from the controller
		controller.mu.Lock()
		_, exists := controller.chunks[file.ID]
		controller.mu.Unlock()
		assert.False(t, exists, "Chunks should be removed after finalization")

		// Verify the final file exists
		finalPath := file.ToUnderlyingFilePath(mockFileStor.Root())
		_, err = os.Stat(finalPath)
		assert.NoError(t, err, "Final file should exist")

		// Verify the chunks directory was removed
		_, err = os.Stat(chunksDir)
		assert.True(t, os.IsNotExist(err), "Chunks directory should be removed")
	})

	t.Run("MissingChunks", func(t *testing.T) {
		// Create a new file for this test
		newFile, err := mockFileStor.CreateFile("missing_chunks.txt", 1, dir.ID, 1, "text/plain")
		require.NoError(t, err)

		// Setup request
		queryParams := map[string]string{
			"file_id":      strconv.Itoa(newFile.ID),
			"total_chunks": "3",
		}

		ctx, rec := setupEchoContext(t, http.MethodPost, "/resumable-upload/finalize", nil, queryParams)

		// Execute
		err = controller.FinalizeUpload(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "No chunks found for this file")
	})

	t.Run("IncorrectChunkCount", func(t *testing.T) {
		// Create a new file for this test
		newFile, err := mockFileStor.CreateFile("incorrect_count.txt", 1, dir.ID, 1, "text/plain")
		require.NoError(t, err)

		// Create temporary chunk files
		newChunksDir := filepath.Join(mockFileStor.Root(), "chunks", fmt.Sprintf("file_%d", newFile.ID))
		os.MkdirAll(newChunksDir, 0755)

		newChunk0Path := filepath.Join(newChunksDir, "chunk_0")
		newChunk1Path := filepath.Join(newChunksDir, "chunk_1")

		_ = os.WriteFile(newChunk0Path, []byte("Chunk 0 data"), 0644)
		_ = os.WriteFile(newChunk1Path, []byte("Chunk 1 data"), 0644)

		// Add chunks to the controller
		controller.mu.Lock()
		controller.chunks[newFile.ID] = map[int]string{
			0: newChunk0Path,
			1: newChunk1Path,
		}
		controller.mu.Unlock()

		// Setup request with incorrect total_chunks
		queryParams := map[string]string{
			"file_id":      strconv.Itoa(newFile.ID),
			"total_chunks": "3", // We only have 2 chunks
		}

		ctx, rec := setupEchoContext(t, http.MethodPost, "/resumable-upload/finalize", nil, queryParams)

		// Execute
		err = controller.FinalizeUpload(ctx)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Expected 3 chunks, but found 2")
	})
}

// TestGetOrCreateFile tests the getOrCreateMCFile helper method
func TestGetOrCreateFile(t *testing.T) {
	// Setup
	mockFileStor := stor.NewMockFileStor()
	defer mockFileStor.Cleanup()

	controller := NewResumableUploadController(mockFileStor)

	// Create test directory
	dir, err := mockFileStor.GetOrCreateDirPath(1, 1, "/test")
	require.NoError(t, err)

	t.Run("CreateNewFile", func(t *testing.T) {
		// Execute
		file, err := controller.getOrCreateMCFile(1, 1, dir.ID, "newfile.txt")

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, file)
		assert.Equal(t, "newfile.txt", file.Name)
		assert.Equal(t, 1, file.ProjectID)
		assert.Equal(t, 1, file.OwnerID)
		assert.Equal(t, dir.ID, file.DirectoryID)
	})

	t.Run("GetExistingFile", func(t *testing.T) {
		// Create a file first
		existingFile, err := mockFileStor.CreateFile("existing.txt", 1, dir.ID, 1, "text/plain")
		require.NoError(t, err)

		// Execute - should get the existing file
		file, err := controller.getOrCreateMCFile(1, 1, dir.ID, "existing.txt")

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, file)
		assert.Equal(t, existingFile.ID, file.ID)
		assert.Equal(t, "existing.txt", file.Name)
	})
}
