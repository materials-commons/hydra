package fileindex

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// FileCategories organizes files by type
type FileCategories struct {
	LogFiles      []mcmodel.File
	ResultFiles   []mcmodel.File
	MetadataFiles []mcmodel.File
	LabNotebooks  []mcmodel.File
	OtherText     []mcmodel.File
}

// FileClassifier classifies files based on patterns and project type
type FileClassifier struct {
	ProjectType string // "materials", "chemistry", "biology", "general"
}

// NewFileClassifier creates a new file classifier
func NewFileClassifier(projectType string) *FileClassifier {
	return &FileClassifier{
		ProjectType: projectType,
	}
}

// ClassifyFiles categorizes files into different types
func (fc *FileClassifier) ClassifyFiles(files []mcmodel.File) FileCategories {
	categories := FileCategories{
		LogFiles:      []mcmodel.File{},
		ResultFiles:   []mcmodel.File{},
		MetadataFiles: []mcmodel.File{},
		LabNotebooks:  []mcmodel.File{},
		OtherText:     []mcmodel.File{},
	}

	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}

		// Skip non-text files
		if !isTextFile(file) {
			continue
		}

		// Classify based on patterns
		category := classifyFile(file)

		switch category {
		case "log":
			categories.LogFiles = append(categories.LogFiles, file)
		case "results":
			categories.ResultFiles = append(categories.ResultFiles, file)
		case "metadata":
			categories.MetadataFiles = append(categories.MetadataFiles, file)
		case "notebook":
			categories.LabNotebooks = append(categories.LabNotebooks, file)
		default:
			categories.OtherText = append(categories.OtherText, file)
		}
	}

	return categories
}

// classifyFile determines the category of a file based on its name and path
func classifyFile(file mcmodel.File) string {
	name := strings.ToLower(file.Name)
	path := strings.ToLower(file.Path)

	// Log files
	if strings.HasSuffix(name, ".log") ||
		strings.Contains(path, "/logs/") ||
		strings.Contains(name, "log") {
		return "log"
	}

	// Result files
	if strings.Contains(path, "/results/") ||
		strings.Contains(path, "/data/") ||
		strings.Contains(path, "/output/") ||
		strings.HasPrefix(name, "results_") ||
		strings.HasPrefix(name, "output_") {
		return "results"
	}

	// Metadata files
	if strings.HasPrefix(name, "metadata") ||
		strings.HasSuffix(name, "_metadata.txt") ||
		strings.HasSuffix(name, "_meta.txt") ||
		strings.Contains(name, "readme") {
		return "metadata"
	}

	// Lab notebooks
	if strings.Contains(name, "notebook") ||
		strings.Contains(name, "notes") ||
		strings.HasSuffix(name, "_notes.txt") ||
		strings.Contains(path, "/notebooks/") {
		return "notebook"
	}

	return "other"
}

// isTextFile determines if a file is a text file based on mime type and extension
func isTextFile(file mcmodel.File) bool {
	// Check by mime type first
	if strings.HasPrefix(file.MimeType, "text/") {
		return true
	}

	// Check by extension
	name := strings.ToLower(file.Name)
	textExtensions := []string{
		".txt", ".log", ".csv", ".tsv", ".dat",
		".md", ".rst", ".tex",
		".json", ".xml", ".yaml", ".yml",
		".ini", ".conf", ".cfg", ".config",
		".sh", ".bash", ".py", ".r", ".m",
	}

	for _, ext := range textExtensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}

	return false
}

// readFileContent reads the content of a file from the mcfs storage
func readFileContent(file mcmodel.File, mcfsDir string) (string, error) {
	filePath := file.ToUnderlyingFilePath(mcfsDir)

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Read file content
	content, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	// Check if it's valid UTF-8
	if !utf8.Valid(content) {
		// Try to convert or skip non-text files
		return "", io.EOF
	}

	return string(content), nil
}

// GetTextFilesInPath returns all text files under a given path pattern
func GetTextFilesInPath(files []mcmodel.File, pathPattern string) []mcmodel.File {
	var matching []mcmodel.File

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !isTextFile(file) {
			continue
		}

		// Simple glob-style matching
		matched, _ := filepath.Match(pathPattern, file.Path+"/"+file.Name)
		if matched || strings.Contains(file.Path+"/"+file.Name, pathPattern) {
			matching = append(matching, file)
		}
	}

	return matching
}
