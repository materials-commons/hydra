package stor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type FakeFileStor struct {
	files  []mcmodel.File
	lastID int
}

func NewFakeFileStor(files []mcmodel.File) *FakeFileStor {
	return &FakeFileStor{files: files, lastID: 10000}
}

func (s *FakeFileStor) GetFileByID(fileID int) (*mcmodel.File, error) {
	for _, f := range s.files {
		if f.ID == fileID {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("no such file")
}

func (s *FakeFileStor) GetFileByUUID(fileUUID string) (*mcmodel.File, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *FakeFileStor) UpdateMetadataForFileAndProject(file *mcmodel.File, checksum string, totalBytes int64) error {
	for _, f := range s.files {
		if f.ID == file.ID {
			f.Checksum = checksum
			return nil
		}
	}
	return fmt.Errorf("no such file: %d", file.ID)
}

func (s *FakeFileStor) CreateFile(name string, projectID, directoryID, ownerID int, mimeType string) (*mcmodel.File, error) {
	f := mcmodel.File{
		ID:          s.lastID,
		ProjectID:   projectID,
		DirectoryID: directoryID,
		OwnerID:     ownerID,
		MimeType:    mimeType,
		Name:        name,
	}
	s.lastID = s.lastID + 1
	s.files = append(s.files, f)
	return &f, nil
}

func (s *FakeFileStor) GetDirByPath(projectID int, path string) (*mcmodel.File, error) {
	for _, f := range s.files {
		if f.IsDir() {
			if f.ProjectID == projectID && f.Path == path {
				return &f, nil
			}
		}
	}
	return nil, fmt.Errorf("no such dir")
}

func (s *FakeFileStor) CreateDirectory(parentDirID, projectID, ownerID int, path, name string) (*mcmodel.File, error) {
	d := mcmodel.File{
		ID:          s.lastID,
		Path:        path,
		ProjectID:   projectID,
		DirectoryID: parentDirID,
		OwnerID:     ownerID,
		MimeType:    "directory",
		Name:        name,
	}
	s.lastID = s.lastID + 1
	s.files = append(s.files, d)
	return &d, nil
}

func (s *FakeFileStor) CreateDirIfNotExists(parentDirID int, path, name string, projectID, ownerID int) (*mcmodel.File, error) {
	d, err := s.GetDirByPath(projectID, path)
	if err == nil {
		return d, nil
	}
	return s.CreateDirectory(parentDirID, projectID, ownerID, path, name)
}

func (s *FakeFileStor) ListDirectoryByPath(projectID int, path string) ([]mcmodel.File, error) {
	var files []mcmodel.File
	dir, err := s.GetDirByPath(projectID, path)
	if err != nil {
		return files, err
	}
	for _, f := range s.files {
		if f.DirectoryID == dir.ID {
			files = append(files, f)
		}
	}
	return files, nil
}

func (s *FakeFileStor) GetOrCreateDirPath(projectID, ownerID int, path string) (*mcmodel.File, error) {
	dir, err := s.GetDirByPath(projectID, path)
	if err == nil {
		return dir, nil
	}

	parentPath := filepath.Dir(path)
	parentDir, err := s.GetDirByPath(projectID, parentPath)
	if err == nil {
		// Ok, the parent exists, so just create the child of the parent (ie, the complete path) and return
		// the created directory.
		return s.CreateDirectory(parentDir.ID, projectID, ownerID, path, filepath.Base(path))
	}

	pathParts := strings.Split(path, "/")
	currentPath := "/"
	for _, pathPart := range pathParts[1:] {
		currentPath = filepath.Join(currentPath, pathPart)
		dir, err = s.CreateDirIfNotExists(parentDir.ID, currentPath, filepath.Base(currentPath), projectID, ownerID)
		if err != nil {
			return nil, err
		}
		parentDir = dir
	}

	return dir, nil
}

func (s *FakeFileStor) GetFileByPath(projectID int, path string) (*mcmodel.File, error) {
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	dir, err := s.GetDirByPath(projectID, dirPath)
	if err != nil {
		return nil, err
	}

	for _, f := range s.files {
		if f.DirectoryID == dir.ID && f.Name == fileName {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("no such file")
}

func (s *FakeFileStor) UpdateFileUses(file *mcmodel.File, uuid string, fileID int) error {
	for _, f := range s.files {
		if f.ID == file.ID {
			f.UsesUUID = uuid
			f.UsesID = fileID
			return nil
		}
	}
	return fmt.Errorf("no such file")
}

func (s *FakeFileStor) PointAtExistingIfExists(file *mcmodel.File) (bool, error) {
	// Do nothing, don't switch
	return false, nil
}

func (s *FakeFileStor) DoneWritingToFile(file *mcmodel.File, checksum string, size int64, conversionStore ConversionStor) (bool, error) {
	// Do nothing, don't switch
	return false, nil
}
