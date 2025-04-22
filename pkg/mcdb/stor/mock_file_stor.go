package stor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// MockFileStor implements the FileStor interface for testing
type MockFileStor struct {
	files       map[int]*mcmodel.File
	filesByPath map[string]*mcmodel.File
	dirs        map[int]*mcmodel.File
	nextID      int
	root        string
}

// NewMockFileStor creates a new MockFileStor with a temporary directory for testing
func NewMockFileStor() *MockFileStor {
	// Create a temporary directory for testing
	tempDir, _ := ioutil.TempDir("", "mock-file-stor-test")

	return &MockFileStor{
		files:       make(map[int]*mcmodel.File),
		filesByPath: make(map[string]*mcmodel.File),
		dirs:        make(map[int]*mcmodel.File),
		nextID:      1,
		root:        tempDir,
	}
}

// Cleanup removes the temporary directory created for testing
func (m *MockFileStor) Cleanup() {
	os.RemoveAll(m.root)
}

// GetFileByID retrieves a file by its ID
func (m *MockFileStor) GetFileByID(fileID int) (*mcmodel.File, error) {
	file, ok := m.files[fileID]
	if !ok {
		return nil, fmt.Errorf("file not found")
	}
	return file, nil
}

// GetFileByUUID retrieves a file by its UUID
func (m *MockFileStor) GetFileByUUID(fileUUID string) (*mcmodel.File, error) {
	for _, file := range m.files {
		if file.UUID == fileUUID {
			return file, nil
		}
	}
	return nil, fmt.Errorf("file not found")
}

// GetFileByPath retrieves a file by its path and project ID
func (m *MockFileStor) GetFileByPath(projectID int, path string) (*mcmodel.File, error) {
	key := fmt.Sprintf("%d:%s", projectID, path)
	file, ok := m.filesByPath[key]
	if !ok {
		return nil, fmt.Errorf("file not found")
	}
	return file, nil
}

// GetOrCreateDirPath gets an existing directory or creates a new one if it doesn't exist
func (m *MockFileStor) GetOrCreateDirPath(projectID, ownerID int, path string) (*mcmodel.File, error) {
	// Check if directory already exists
	for _, dir := range m.dirs {
		if dir.ProjectID == projectID && dir.Path == path {
			return dir, nil
		}
	}

	// Create new directory
	dir := &mcmodel.File{
		ID:        m.nextID,
		UUID:      fmt.Sprintf("dir-%d", m.nextID),
		ProjectID: projectID,
		OwnerID:   ownerID,
		Path:      path,
		Name:      filepath.Base(path),
		MimeType:  "directory",
	}
	m.nextID++
	m.dirs[dir.ID] = dir
	return dir, nil
}

// CreateFile creates a new file in the specified directory
func (m *MockFileStor) CreateFile(name string, projectID, directoryID, ownerID int, mimeType string) (*mcmodel.File, error) {
	dir, ok := m.dirs[directoryID]
	if !ok {
		return nil, fmt.Errorf("directory not found")
	}

	file := &mcmodel.File{
		ID:          m.nextID,
		UUID:        fmt.Sprintf("file-%d", m.nextID),
		ProjectID:   projectID,
		OwnerID:     ownerID,
		DirectoryID: directoryID,
		Directory:   dir,
		Name:        name,
		Path:        filepath.Join(dir.Path, name),
		MimeType:    mimeType,
	}
	m.nextID++
	m.files[file.ID] = file

	key := fmt.Sprintf("%d:%s", projectID, file.Path)
	m.filesByPath[key] = file

	return file, nil
}

// Root returns the root directory for the mock file system
func (m *MockFileStor) Root() string {
	return m.root
}

// Implement other methods of the FileStor interface with minimal functionality

// UpdateMetadataForFileAndProject updates file metadata
func (m *MockFileStor) UpdateMetadataForFileAndProject(file *mcmodel.File, checksum string, totalBytes int64) error {
	return nil
}

// UpdateFile updates a file with the provided updates
func (m *MockFileStor) UpdateFile(file, updates *mcmodel.File) (*mcmodel.File, error) {
	return file, nil
}

// SetUsesToNull sets the uses fields to null
func (m *MockFileStor) SetUsesToNull(file *mcmodel.File) (*mcmodel.File, error) {
	return file, nil
}

// SetFileAsCurrent marks a file as current
func (m *MockFileStor) SetFileAsCurrent(file *mcmodel.File) (*mcmodel.File, error) {
	return file, nil
}

// GetDirByPath retrieves a directory by its path
func (m *MockFileStor) GetDirByPath(projectID int, path string) (*mcmodel.File, error) {
	for _, dir := range m.dirs {
		if dir.ProjectID == projectID && dir.Path == path {
			return dir, nil
		}
	}
	return nil, fmt.Errorf("directory not found")
}

// CreateDirectory creates a new directory
func (m *MockFileStor) CreateDirectory(parentDirID, projectID, ownerID int, path, name string) (*mcmodel.File, error) {
	return m.GetOrCreateDirPath(projectID, ownerID, filepath.Join(path, name))
}

// CreateDirIfNotExists creates a directory if it doesn't exist
func (m *MockFileStor) CreateDirIfNotExists(parentDirID int, path, name string, projectID, ownerID int) (*mcmodel.File, error) {
	return m.GetOrCreateDirPath(projectID, ownerID, filepath.Join(path, name))
}

// ListDirectoryByPath lists files in a directory
func (m *MockFileStor) ListDirectoryByPath(projectID int, path string) ([]mcmodel.File, error) {
	return []mcmodel.File{}, nil
}

// UpdateFileUses updates the uses fields of a file
func (m *MockFileStor) UpdateFileUses(file *mcmodel.File, uuid string, fileID int) error {
	return nil
}

// PointAtExistingIfExists points at an existing file if it exists
func (m *MockFileStor) PointAtExistingIfExists(file *mcmodel.File) (bool, error) {
	return false, nil
}

// DoneWritingToFile marks a file as done writing
func (m *MockFileStor) DoneWritingToFile(file *mcmodel.File, checksum string, size int64, conversionStore ConversionStor) (bool, error) {
	return false, nil
}
