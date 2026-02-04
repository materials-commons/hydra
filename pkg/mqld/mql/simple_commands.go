package mql

import (
	"fmt"
	"strings"

	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mqld/mql/fileindex"
)

// findInFilesCommand searches for text across indexed files
// Usage: find-in-files "search text" [in "category"]
func (q *Query) findInFilesCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) < 1 {
		return feather.Error(fmt.Errorf("find-in-files search-text [in category]"))
	}

	searchText := args[0].String()
	category := "all"

	// Check for optional "in category" syntax
	if len(args) >= 3 && args[1].String() == "in" {
		category = args[2].String()
	}

	// Ensure index is built
	if err := q.ensureFileIndexBuilt(i); err != nil {
		return feather.Error(err)
	}

	// Search
	var results []fileindex.SearchResult
	if category == "all" {
		results = q.fileIndex.SearchWithContext(searchText, 2)
	} else {
		results = q.fileIndex.SearchInCategory(searchText, category)
	}

	// Store results in context for later use
	i.SetVar("_found_results", fileindex.SerializeResults(results))
	i.SetVar("_found_search_text", searchText)

	// Format output
	if len(results) == 0 {
		return feather.OK(fmt.Sprintf("No matches found for '%s'", searchText))
	}

	summary := fmt.Sprintf("Found %d matches in %d files\n",
		len(results), len(fileindex.GetUniqueFiles(results)))

	return feather.OK(summary + "\n" + fileindex.FormatSearchResultsAsText(results))
}

// showCommand displays various information about searchable files and results
// Usage: show searchable-files | found-data | file-categories | presets
func (q *Query) showCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("show <what>"))
	}

	what := args[0].String()

	switch what {
	case "searchable-files":
		return q.showSearchableFiles(i)
	case "found-data":
		return q.showFoundData(i)
	case "file-categories":
		return q.showFileCategories(i)
	case "presets":
		return q.showPresets(i)
	default:
		return feather.Error(fmt.Errorf("don't know how to show '%s'. Try: searchable-files, found-data, file-categories, presets", what))
	}
}

func (q *Query) showSearchableFiles(i *feather.Interp) feather.Result {
	if err := q.ensureFileIndexBuilt(i); err != nil {
		return feather.Error(err)
	}

	categories := q.fileIndex.GetFileCategories()
	total := q.fileIndex.GetIndexedFileCount()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d searchable text files:\n\n", total))

	if count, ok := categories["log"]; ok && count > 0 {
		output.WriteString(fmt.Sprintf("  📋 %d log files\n", count))
	}
	if count, ok := categories["results"]; ok && count > 0 {
		output.WriteString(fmt.Sprintf("  📊 %d result files\n", count))
	}
	if count, ok := categories["notebook"]; ok && count > 0 {
		output.WriteString(fmt.Sprintf("  📓 %d lab notebooks\n", count))
	}
	if count, ok := categories["metadata"]; ok && count > 0 {
		output.WriteString(fmt.Sprintf("  📝 %d metadata files\n", count))
	}
	if count, ok := categories["other"]; ok && count > 0 {
		output.WriteString(fmt.Sprintf("  📄 %d other text files\n", count))
	}

	output.WriteString("\nTry: find-in-files \"your search text\"\n")
	output.WriteString("Or:  find-in-files \"text\" in logs\n")

	return feather.OK(output.String())
}

func (q *Query) showFoundData(i *feather.Interp) feather.Result {
	resultsStr := i.GetVar("_found_results")
	if resultsStr == "" {
		return feather.OK("No search results found. Use 'find-in-files' first to search for data.")
	}

	searchText := i.GetVar("_found_search_text")
	results := fileindex.DeserializeResults(resultsStr)

	if len(results) == 0 {
		return feather.OK("No results stored.")
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for '%s':\n\n", searchText))
	output.WriteString(fileindex.FormatSearchResultsWithContext(results))
	output.WriteString("\nNext steps:\n")
	output.WriteString("  - Extract samples: make-samples from found-data using pattern \"Sample-(\\w+)\"\n")
	output.WriteString("  - Extract values: extract-from-files pattern \"Temp: (\\d+)C\" as Temperature\n")

	return feather.OK(output.String())
}

func (q *Query) showFileCategories(i *feather.Interp) feather.Result {
	if err := q.ensureFileIndexBuilt(i); err != nil {
		return feather.Error(err)
	}

	categories := q.fileIndex.GetFileCategories()

	var output strings.Builder
	output.WriteString("File categories:\n\n")
	output.WriteString("  log       - Log files (*.log, */logs/*)\n")
	output.WriteString("  results   - Result files (*/results/*, */data/*)\n")
	output.WriteString("  notebook  - Lab notebooks (*notes.txt, */notebooks/*)\n")
	output.WriteString("  metadata  - Metadata files (metadata*.txt, readme*)\n")
	output.WriteString("  other     - Other text files\n\n")

	output.WriteString("Files in your project:\n")
	for cat, count := range categories {
		output.WriteString(fmt.Sprintf("  %s: %d files\n", cat, count))
	}

	return feather.OK(output.String())
}

func (q *Query) showPresets(i *feather.Interp) feather.Result {
	presets := fileindex.ListAvailablePresets()

	var output strings.Builder
	output.WriteString("Available project presets:\n\n")

	for _, preset := range presets {
		output.WriteString(fmt.Sprintf("📦 %s - %s\n", preset.Type, preset.Description))
		output.WriteString("   Common patterns:\n")
		for _, extraction := range preset.CommonExtractions {
			output.WriteString(fmt.Sprintf("     • %s: %s\n", extraction.Name, extraction.Description))
		}
		if len(preset.HelpExamples) > 0 {
			output.WriteString("   Example:\n")
			output.WriteString(fmt.Sprintf("     %s\n", preset.HelpExamples[0]))
		}
		output.WriteString("\n")
	}

	output.WriteString("To use a preset: setup-file-search <preset-type>\n")

	return feather.OK(output.String())
}

// previewCommand is an alias for "show found-data"
func (q *Query) previewCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	return q.showFoundData(i)
}

// indexFilesCommand allows users to explicitly index files by pattern or category
// Usage: index-files pattern "*/logs/*.txt" | category "logs" | path "/data" | all [limit]
func (q *Query) indexFilesCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) < 2 {
		return feather.Error(fmt.Errorf("index-files <pattern|category|path|all> <value> [limit]"))
	}

	command := args[0].String()
	value := args[1].String()
	limit := 1000 // Default limit

	if len(args) >= 3 {
		if l, err := args[2].Int(); err == nil {
			limit = int(l)
		}
	}

	// Initialize index if needed
	if q.fileIndex == nil {
		q.fileIndex = fileindex.NewTextFileIndex(q.db.ProjectID)
	}

	// Clear existing index
	q.fileIndex.Clear()

	// Create file loader
	fileLoader := fileindex.NewFileLoader(q.db.ProjectID, q.db.GetDB())

	var files []mcmodel.File
	var err error

	switch command {
	case "pattern":
		files, err = fileLoader.FindFilesByPattern(value)
	case "category":
		files, err = fileLoader.FindFilesByCategory(value)
	case "path":
		recursive := true // default to recursive
		files, err = fileLoader.FindFilesInPath(value, recursive)
	case "all":
		files, err = fileLoader.FindTextFiles(limit)
	default:
		return feather.Error(fmt.Errorf("unknown index command '%s'. Use: pattern, category, path, or all", command))
	}

	if err != nil {
		return feather.Error(fmt.Errorf("failed to find files: %v", err))
	}

	// Apply limit if we have too many files
	if len(files) > limit {
		files = files[:limit]
	}

	// Index the files
	mcfsDir := "/mcfs" // TODO: Get from configuration
	if err := q.fileIndex.IndexFiles(files, mcfsDir); err != nil {
		return feather.Error(fmt.Errorf("failed to index files: %v", err))
	}

	return feather.OK(fmt.Sprintf("Indexed %d files. Use 'find-in-files' to search.", q.fileIndex.GetIndexedFileCount()))
}

// ensureFileIndexBuilt makes sure the file index is built
func (q *Query) ensureFileIndexBuilt(i *feather.Interp) error {
	// Check if already built
	if q.fileIndex != nil && q.fileIndex.GetIndexedFileCount() > 0 {
		return nil
	}

	// Build the index
	return q.buildFileIndex()
}

// buildFileIndex builds the file index from the database
// This uses lazy loading - only loads files matching user criteria
func (q *Query) buildFileIndex() error {
	if q.fileIndex == nil {
		q.fileIndex = fileindex.NewTextFileIndex(q.db.ProjectID)
	}

	// Check if already indexed
	if q.fileIndex.GetIndexedFileCount() > 0 {
		return nil
	}

	// Create file loader for lazy loading
	fileLoader := fileindex.NewFileLoader(q.db.ProjectID, q.db.GetDB())

	// Only load a reasonable number of text files (limit to 1000 by default)
	// Users can be more specific with patterns if they have more files
	files, err := fileLoader.FindTextFiles(1000)
	if err != nil {
		return err
	}

	// Get mcfs directory (you may need to pass this in or get from config)
	// For now, using a placeholder - this should be properly configured
	mcfsDir := "/mcfs" // TODO: Get from configuration

	return q.fileIndex.IndexFiles(files, mcfsDir)
}
