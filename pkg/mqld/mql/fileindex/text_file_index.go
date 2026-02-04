package fileindex

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// TextFileIndex provides fast in-memory searching for small text files
type TextFileIndex struct {
	projectID    int
	indexedFiles map[int]*IndexedTextFile
	lastUpdate   time.Time
	maxFileSize  int64 // Max file size to index (default 1MB)
}

// IndexedTextFile represents a text file cached in memory for fast searching
type IndexedTextFile struct {
	FileID     int
	Path       string
	Name       string
	Size       int64
	Category   string // "log", "results", "metadata", "notebook", "other"
	LineIndex  []string
	ModifiedAt time.Time
}

// SearchResult represents a single search match
type SearchResult struct {
	FileID      int
	FilePath    string
	FileName    string
	Category    string
	LineNum     int
	Line        string
	ContextBefore []string
	ContextAfter  []string
}

// NewTextFileIndex creates a new text file index for a project
func NewTextFileIndex(projectID int) *TextFileIndex {
	return &TextFileIndex{
		projectID:    projectID,
		indexedFiles: make(map[int]*IndexedTextFile),
		maxFileSize:  1 * 1024 * 1024, // 1MB default
	}
}

// SetMaxFileSize sets the maximum file size to index (in bytes)
func (tfi *TextFileIndex) SetMaxFileSize(size int64) {
	tfi.maxFileSize = size
}

// IndexFiles indexes the provided text files for searching
func (tfi *TextFileIndex) IndexFiles(files []mcmodel.File, mcfsDir string) error {
	indexed := 0
	skipped := 0

	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}

		// Skip large files
		if int64(file.Size) > tfi.maxFileSize {
			skipped++
			continue
		}

		// Only index text files
		if !isTextFile(file) {
			skipped++
			continue
		}

		// Read file content
		content, err := readFileContent(file, mcfsDir)
		if err != nil {
			log.Warnf("Failed to read file %s (id: %d): %v", file.Name, file.ID, err)
			skipped++
			continue
		}

		// Split into lines for indexing
		lines := strings.Split(content, "\n")

		// Classify the file
		category := classifyFile(file)

		// Store in index
		tfi.indexedFiles[file.ID] = &IndexedTextFile{
			FileID:     file.ID,
			Path:       file.Path,
			Name:       file.Name,
			Size:       int64(file.Size),
			Category:   category,
			LineIndex:  lines,
			ModifiedAt: file.UpdatedAt,
		}

		indexed++
	}

	tfi.lastUpdate = time.Now()
	log.Infof("Indexed %d text files, skipped %d files", indexed, skipped)

	return nil
}

// Search performs a simple text search across all indexed files
func (tfi *TextFileIndex) Search(searchText string) []SearchResult {
	return tfi.SearchWithContext(searchText, 2)
}

// SearchWithContext performs a text search and includes context lines
func (tfi *TextFileIndex) SearchWithContext(searchText string, contextLines int) []SearchResult {
	var results []SearchResult

	for _, indexed := range tfi.indexedFiles {
		for lineNum, line := range indexed.LineIndex {
			if strings.Contains(line, searchText) {
				result := SearchResult{
					FileID:   indexed.FileID,
					FilePath: indexed.Path + "/" + indexed.Name,
					FileName: indexed.Name,
					Category: indexed.Category,
					LineNum:  lineNum + 1,
					Line:     line,
				}

				// Add context lines
				result.ContextBefore = getContextBefore(indexed.LineIndex, lineNum, contextLines)
				result.ContextAfter = getContextAfter(indexed.LineIndex, lineNum, contextLines)

				results = append(results, result)
			}
		}
	}

	return results
}

// SearchWithRegex performs a regex search across all indexed files
func (tfi *TextFileIndex) SearchWithRegex(pattern string) ([]SearchResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	var results []SearchResult

	for _, indexed := range tfi.indexedFiles {
		for lineNum, line := range indexed.LineIndex {
			if re.MatchString(line) {
				result := SearchResult{
					FileID:   indexed.FileID,
					FilePath: indexed.Path + "/" + indexed.Name,
					FileName: indexed.Name,
					Category: indexed.Category,
					LineNum:  lineNum + 1,
					Line:     line,
				}

				// Add context lines
				result.ContextBefore = getContextBefore(indexed.LineIndex, lineNum, 2)
				result.ContextAfter = getContextAfter(indexed.LineIndex, lineNum, 2)

				results = append(results, result)
			}
		}
	}

	return results, nil
}

// SearchInCategory searches only within a specific category of files
func (tfi *TextFileIndex) SearchInCategory(searchText, category string) []SearchResult {
	var results []SearchResult

	for _, indexed := range tfi.indexedFiles {
		// Skip if not in the requested category
		if category != "all" && indexed.Category != category {
			continue
		}

		for lineNum, line := range indexed.LineIndex {
			if strings.Contains(line, searchText) {
				result := SearchResult{
					FileID:   indexed.FileID,
					FilePath: indexed.Path + "/" + indexed.Name,
					FileName: indexed.Name,
					Category: indexed.Category,
					LineNum:  lineNum + 1,
					Line:     line,
				}

				result.ContextBefore = getContextBefore(indexed.LineIndex, lineNum, 2)
				result.ContextAfter = getContextAfter(indexed.LineIndex, lineNum, 2)

				results = append(results, result)
			}
		}
	}

	return results
}

// GetIndexedFile retrieves an indexed file by ID
func (tfi *TextFileIndex) GetIndexedFile(fileID int) (*IndexedTextFile, bool) {
	file, ok := tfi.indexedFiles[fileID]
	return file, ok
}

// GetFileCategories returns a summary of files by category
func (tfi *TextFileIndex) GetFileCategories() map[string]int {
	categories := make(map[string]int)

	for _, indexed := range tfi.indexedFiles {
		categories[indexed.Category]++
	}

	return categories
}

// GetIndexedFileCount returns the number of indexed files
func (tfi *TextFileIndex) GetIndexedFileCount() int {
	return len(tfi.indexedFiles)
}

// Clear clears the index
func (tfi *TextFileIndex) Clear() {
	tfi.indexedFiles = make(map[int]*IndexedTextFile)
	tfi.lastUpdate = time.Time{}
}

// Helper functions

func getContextBefore(lines []string, lineNum, contextLines int) []string {
	start := lineNum - contextLines
	if start < 0 {
		start = 0
	}
	return lines[start:lineNum]
}

func getContextAfter(lines []string, lineNum, contextLines int) []string {
	end := lineNum + 1 + contextLines
	if end > len(lines) {
		end = len(lines)
	}
	return lines[lineNum+1 : end]
}
