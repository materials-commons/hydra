package fileindex

import (
	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

// FileLoader handles lazy loading of files based on user-specified patterns
type FileLoader struct {
	projectID int
	db        *gorm.DB
}

// NewFileLoader creates a new file loader
func NewFileLoader(projectID int, db *gorm.DB) *FileLoader {
	return &FileLoader{
		projectID: projectID,
		db:        db,
	}
}

// FindFilesByPattern finds files matching a path pattern
// This uses SQL queries rather than loading all files into memory
// Pattern examples: "*/logs/*.txt", "*/results/*", "*.log"
func (fl *FileLoader) FindFilesByPattern(pattern string) ([]mcmodel.File, error) {
	var files []mcmodel.File

	// Convert glob pattern to SQL LIKE pattern
	sqlPattern := convertGlobToSQL(pattern)

	// Query for files matching the pattern
	// Only load files, not directories
	err := fl.db.Preload("Directory").
		Where("project_id = ?", fl.projectID).
		Where("current = ?", true).
		Where("mime_type != ?", "directory").
		Where("path LIKE ? OR name LIKE ?", sqlPattern, sqlPattern).
		Find(&files).Error

	if err != nil {
		return nil, err
	}

	log.Infof("Found %d files matching pattern '%s'", len(files), pattern)
	return files, nil
}

// FindTextFiles finds all text files in the project up to a limit
// This prevents loading millions of files
func (fl *FileLoader) FindTextFiles(limit int) ([]mcmodel.File, error) {
	var files []mcmodel.File

	// Build a query for common text file extensions
	textExtensions := []string{
		"%.txt", "%.log", "%.csv", "%.dat",
		"%.md", "%.json", "%.xml", "%.yaml",
	}

	query := fl.db.Preload("Directory").
		Where("project_id = ?", fl.projectID).
		Where("current = ?", true).
		Where("mime_type != ?", "directory")

	// Add OR conditions for text files
	orCond := fl.db.Where("mime_type LIKE ?", "text/%")
	for _, ext := range textExtensions {
		orCond = orCond.Or("name LIKE ?", ext)
	}

	err := query.Where(orCond).
		Limit(limit).
		Find(&files).Error

	if err != nil {
		return nil, err
	}

	log.Infof("Found %d text files (limit: %d)", len(files), limit)
	return files, nil
}

// FindFilesByCategory finds files in a specific directory pattern
// category: "logs", "results", "data", etc.
func (fl *FileLoader) FindFilesByCategory(category string) ([]mcmodel.File, error) {
	var files []mcmodel.File

	var pathPattern string
	switch category {
	case "logs", "log":
		pathPattern = "%/logs/%"
	case "results":
		pathPattern = "%/results/%"
	case "data":
		pathPattern = "%/data/%"
	case "output":
		pathPattern = "%/output/%"
	case "notebooks":
		pathPattern = "%/notebooks/%"
	default:
		pathPattern = "%/" + category + "/%"
	}

	err := fl.db.Preload("Directory").
		Where("project_id = ?", fl.projectID).
		Where("current = ?", true).
		Where("mime_type != ?", "directory").
		Where("path LIKE ?", pathPattern).
		Find(&files).Error

	if err != nil {
		return nil, err
	}

	log.Infof("Found %d files in category '%s'", len(files), category)
	return files, nil
}

// FindFilesInPath finds all files under a specific path
func (fl *FileLoader) FindFilesInPath(path string, recursive bool) ([]mcmodel.File, error) {
	var files []mcmodel.File

	var pathPattern string
	if recursive {
		pathPattern = path + "%"
	} else {
		pathPattern = path
	}

	err := fl.db.Preload("Directory").
		Where("project_id = ?", fl.projectID).
		Where("current = ?", true).
		Where("mime_type != ?", "directory").
		Where("path LIKE ?", pathPattern).
		Find(&files).Error

	if err != nil {
		return nil, err
	}

	log.Infof("Found %d files in path '%s' (recursive: %v)", len(files), path, recursive)
	return files, nil
}

// CountTextFiles returns the count of text files without loading them
func (fl *FileLoader) CountTextFiles() (int64, error) {
	var count int64

	textExtensions := []string{
		"%.txt", "%.log", "%.csv", "%.dat",
		"%.md", "%.json", "%.xml", "%.yaml",
	}

	query := fl.db.Model(&mcmodel.File{}).
		Where("project_id = ?", fl.projectID).
		Where("current = ?", true).
		Where("mime_type != ?", "directory")

	orCond := fl.db.Where("mime_type LIKE ?", "text/%")
	for _, ext := range textExtensions {
		orCond = orCond.Or("name LIKE ?", ext)
	}

	err := query.Where(orCond).Count(&count).Error
	return count, err
}

// convertGlobToSQL converts a simple glob pattern to SQL LIKE pattern
func convertGlobToSQL(glob string) string {
	// Simple conversion: * -> %, ? -> _
	// This is a basic implementation, may need enhancement
	result := glob
	result = replace(result, "*", "%")
	result = replace(result, "?", "_")
	return result
}

func replace(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i < len(s)-len(old)+1 && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}
