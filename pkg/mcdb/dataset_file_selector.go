package mcdb

import (
	"github.com/materials-commons/gomcdb/mcmodel"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
)

type DatasetFileSelector struct {
	DatasetID    int
	IncludeFiles map[string]bool
	ExcludeFiles map[string]bool
	IncludeDirs  map[string]bool
	ExcludeDirs  map[string]bool
	EntityFiles  map[string]bool
}

func NewDatasetFileSelector(dataset mcmodel.Dataset) *DatasetFileSelector {
	fs, _ := dataset.GetFileSelection()
	return &DatasetFileSelector{
		DatasetID:    dataset.ID,
		IncludeFiles: createSelectionEntries(fs.IncludeFiles),
		ExcludeFiles: createSelectionEntries(fs.ExcludeFiles),
		IncludeDirs:  createSelectionEntries(fs.IncludeDirs),
		ExcludeDirs:  createSelectionEntries(fs.ExcludeDirs),
		EntityFiles:  make(map[string]bool),
	}
}

func (s *DatasetFileSelector) LoadEntityFiles(db *gorm.DB) error {
	ds := mcmodel.Dataset{ID: s.DatasetID}
	entities, err := ds.GetEntitiesFromTemplate(db)
	if err != nil {
		return err
	}

	for _, entity := range entities {
		for _, file := range entity.Files {
			s.EntityFiles[file.FullPath()] = true
		}
	}
	return nil
}

func createSelectionEntries(filePaths []string) map[string]bool {
	m := make(map[string]bool)
	for _, filePath := range filePaths {
		m[filePath] = true
	}
	return m
}

func (s *DatasetFileSelector) IsIncludedFile(filePath string) bool {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return false
	}

	filePath = filepath.Clean(filePath)
	if _, ok := s.IncludeFiles[filePath]; ok {
		return true
	}

	if _, ok := s.ExcludeFiles[filePath]; ok {
		return false
	}

	if _, ok := s.EntityFiles[filePath]; ok {
		return true
	}

	return s.IsIncludedDir(filepath.Dir(filePath))
}

func (s *DatasetFileSelector) IsIncludedDir(dirPath string) bool {
	if _, ok := s.IncludeDirs[dirPath]; ok {
		return true
	}

	if _, ok := s.ExcludeDirs[dirPath]; ok {
		return false
	}

	dirPath = filepath.Dir(dirPath)
	for {
		if dirPath == "" {
			return false
		}

		if _, ok := s.IncludeDirs[dirPath]; ok {
			return true
		}

		if _, ok := s.ExcludeDirs[dirPath]; ok {
			return false
		}

		if dirPath == "/" {
			return false
		}

		dirPath = filepath.Dir(dirPath)
	}
}
