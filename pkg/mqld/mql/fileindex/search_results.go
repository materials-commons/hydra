package fileindex

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractedValue represents a value extracted from a file
type ExtractedValue struct {
	FileID    int
	FilePath  string
	LineNum   int
	FieldName string
	Value     string
	ValueType string // "string", "int", "float"
	Unit      string // Optional unit (e.g., "C", "MPa", "mbar")
}

// SerializeResults converts search results to a JSON string for storage in context
func SerializeResults(results []SearchResult) string {
	data, err := json.Marshal(results)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// DeserializeResults converts a JSON string back to search results
func DeserializeResults(data string) []SearchResult {
	var results []SearchResult
	if err := json.Unmarshal([]byte(data), &results); err != nil {
		return []SearchResult{}
	}
	return results
}

// SerializeExtractedValues converts extracted values to JSON string
func SerializeExtractedValues(values []ExtractedValue) string {
	data, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// DeserializeExtractedValues converts JSON string back to extracted values
func DeserializeExtractedValues(data string) []ExtractedValue {
	var values []ExtractedValue
	if err := json.Unmarshal([]byte(data), &values); err != nil {
		return []ExtractedValue{}
	}
	return values
}

// FormatSearchResultsAsText formats search results as human-readable text
func FormatSearchResultsAsText(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d matches:\n\n", len(results)))

	// Group by file
	fileGroups := groupResultsByFile(results)

	for filePath, fileResults := range fileGroups {
		output.WriteString(fmt.Sprintf("📄 %s (%d matches)\n", filePath, len(fileResults)))
		for _, result := range fileResults {
			output.WriteString(fmt.Sprintf("  Line %d: %s\n", result.LineNum, strings.TrimSpace(result.Line)))
		}
		output.WriteString("\n")
	}

	return output.String()
}

// FormatSearchResultsAsList formats search results as a TCL list
func FormatSearchResultsAsList(results []SearchResult) []string {
	var items []string

	for _, result := range results {
		item := fmt.Sprintf("file: %q line: %d text: %q category: %s",
			result.FilePath,
			result.LineNum,
			strings.TrimSpace(result.Line),
			result.Category)
		items = append(items, item)
	}

	return items
}

// FormatSearchResultsWithContext formats search results with context lines
func FormatSearchResultsWithContext(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d matches:\n\n", len(results)))

	for _, result := range results {
		output.WriteString(fmt.Sprintf("📄 %s (line %d)\n", result.FilePath, result.LineNum))

		// Context before
		if len(result.ContextBefore) > 0 {
			for _, line := range result.ContextBefore {
				output.WriteString(fmt.Sprintf("    %s\n", strings.TrimSpace(line)))
			}
		}

		// Matching line (highlighted)
		output.WriteString(fmt.Sprintf("  → %s\n", strings.TrimSpace(result.Line)))

		// Context after
		if len(result.ContextAfter) > 0 {
			for _, line := range result.ContextAfter {
				output.WriteString(fmt.Sprintf("    %s\n", strings.TrimSpace(line)))
			}
		}

		output.WriteString("\n")
	}

	return output.String()
}

// GroupResultsByCategory groups search results by file category
func GroupResultsByCategory(results []SearchResult) map[string][]SearchResult {
	groups := make(map[string][]SearchResult)

	for _, result := range results {
		groups[result.Category] = append(groups[result.Category], result)
	}

	return groups
}

// groupResultsByFile groups search results by file path
func groupResultsByFile(results []SearchResult) map[string][]SearchResult {
	groups := make(map[string][]SearchResult)

	for _, result := range results {
		groups[result.FilePath] = append(groups[result.FilePath], result)
	}

	return groups
}

// FilterResultsByCategory filters search results to only include a specific category
func FilterResultsByCategory(results []SearchResult, category string) []SearchResult {
	if category == "all" || category == "" {
		return results
	}

	var filtered []SearchResult
	for _, result := range results {
		if result.Category == category {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// GetUniqueFiles returns the unique file paths from search results
func GetUniqueFiles(results []SearchResult) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, result := range results {
		if !seen[result.FilePath] {
			seen[result.FilePath] = true
			unique = append(unique, result.FilePath)
		}
	}

	return unique
}
