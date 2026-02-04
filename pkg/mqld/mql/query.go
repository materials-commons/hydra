package mql

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mqld/mql/fileindex"
	"github.com/materials-commons/hydra/pkg/mql/mqldb"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

type Query struct {
	db        *mqldb.DB
	fileIndex *fileindex.TextFileIndex
}

type ShowOptions struct {
	Columns []string
	Format  string // "list", "table", "csv", "json"
	Headers []string
	Width   int
}

func NewQuery(projectID int, db *gorm.DB) *Query {
	q := &Query{
		db: mqldb.NewDB(projectID, db),
	}
	return q
}

func (q *Query) RegisterCommands(i *feather.Interp) error {
	if err := q.db.Load(); err != nil {
		return err
	}

	// Structured data query commands
	i.RegisterCommand("attr", q.attrCommand)
	i.RegisterCommand("field", q.fieldCommand)
	i.RegisterCommand("any-state", q.anyStateCommand)
	i.RegisterCommand("all-states", q.allStatesCommand)
	i.RegisterCommand("has-activity", q.hasProcessCommand)
	i.RegisterCommand("has-sample", q.hasSampleCommand)
	i.RegisterCommand("has-attribute", q.hasAttributeCommand)
	i.RegisterCommand("contains", q.containsCommand)
	i.RegisterCommand("starts-with", q.startsWithCommand)
	i.RegisterCommand("ends-with", q.endsWithCommand)
	i.RegisterCommand("query", q.queryCommand)

	// Simple file search commands (beginner level)
	i.RegisterCommand("index-files", q.indexFilesCommand)
	i.RegisterCommand("find-in-files", q.findInFilesCommand)
	i.RegisterCommand("show", q.showCommand)
	i.RegisterCommand("preview", q.previewCommand)

	return nil
}

func (q *Query) attrCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 2 {
		return feather.Error(fmt.Errorf("attr <attr-name>"))
	}
	attrName := args[0].String()

	// Check if there is any context in which we determine the attribute
	contextType := i.GetVar("_ctx_type")

	if contextType == "process" {
		return q.getActivityAttributeValue(i, attrName)
	}

	// Entity type, check if there is a state we should pull from
	if stateIDStr := i.GetVar("_ctx_state_id"); stateIDStr != "" {
		stateID, _ := strconv.Atoi(stateIDStr)
		return q.getEntityStateAttributeValue(i, stateID, attrName)
	}

	// No state, so get the first matching attribute across states.
	return q.getEntityAttributeValue(i, attrName)
}

func (q *Query) getActivityAttributeValue(i *feather.Interp, attrName string) feather.Result {
	activityIDStr := i.GetVar("_ctx_activity_id")
	if activityIDStr == "" {
		return feather.OK("")
	}

	activityID, _ := strconv.Atoi(activityIDStr)
	if attrs, ok := q.db.ProcessAttributesByProcessID[activityID]; ok {
		if attr, ok := attrs[attrName]; ok {
			if len(attr.AttributeValues) > 0 {
				return q.attributeValueToFeather(attr.AttributeValues[0])
			}
		}
	}
	return feather.OK("")
}

func (q *Query) getEntityStateAttributeValue(i *feather.Interp, stateID int, attrName string) feather.Result {
	entityIDStr := i.GetVar("_ctx_sample_id")
	if entityIDStr == "" {
		return feather.OK("")
	}

	entityID, _ := strconv.Atoi(entityIDStr)

	if states, ok := q.db.SampleAttributesBySampleIDAndStates[entityID]; ok {
		// Entity exists, and we have the states map
		if stateAttrs, ok := states[stateID]; ok {
			// state exists for that entity, and we have the attributes maps
			if attr, ok := stateAttrs[attrName]; ok {
				// attribute exists for that state, lets check if it has a value
				if len(attr.AttributeValues) > 0 {
					return q.attributeValueToFeather(attr.AttributeValues[0])
				}
			}
		}
	}
	return feather.OK("")
}

func (q *Query) getEntityAttributeValue(i *feather.Interp, attrName string) feather.Result {
	entityIDStr := i.GetVar("_ctx_sample_id")
	if entityIDStr == "" {
		return feather.OK("")
	}

	entityID, _ := strconv.Atoi(entityIDStr)
	if states, ok := q.db.SampleAttributesBySampleIDAndStates[entityID]; ok {
		for _, stateAttrs := range states {
			if attr, ok := stateAttrs[attrName]; ok {
				if len(attr.AttributeValues) > 0 {
					return q.attributeValueToFeather(attr.AttributeValues[0])
				}
			}
		}
	}

	return feather.OK("")
}

func (q *Query) attributeValueToFeather(val mcmodel.AttributeValue) feather.Result {
	switch val.ValueType {
	case mcmodel.ValueTypeInt:
		return feather.OK(val.ValueInt)
	case mcmodel.ValueTypeFloat:
		return feather.OK(val.ValueFloat)
	case mcmodel.ValueTypeString:
		return feather.OK(val.ValueString)
	default:
		return feather.OK("")
	}
}

func (q *Query) fieldCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("field <field-name>"))
	}
	fieldName := args[0].String()

	switch i.GetVar("_ctx_type") {
	case "sample":
		return q.getEntityFieldValue(i, fieldName)
	case "process":
		return q.getActivityFieldValue(i, fieldName)
	default:
		return feather.Error(fmt.Errorf("field command not implemented for type '%s'", i.GetVar("_ctx_type")))
	}
}

func (q *Query) getEntityFieldValue(i *feather.Interp, fieldName string) feather.Result {
	entityIDStr := i.GetVar("_ctx_sample_id")
	if entityIDStr == "" {
		return feather.Error(fmt.Errorf("no sample ID set in context"))
	}

	sampleID, _ := strconv.Atoi(entityIDStr)
	entity := q.findEntityByID(sampleID)
	if entity == nil {
		return feather.Error(fmt.Errorf("no sample with ID %d found", sampleID))
	}

	switch fieldName {
	case "id":
		return feather.OK(entity.ID)
	case "name":
		return feather.OK(entity.Name)
	case "category":
		return feather.OK(entity.Category)
	case "description":
		return feather.OK(entity.Description)
	default:
		return feather.Error(fmt.Errorf("unknown field '%s'", fieldName))
	}
}

func (q *Query) getActivityFieldValue(i *feather.Interp, fieldName string) feather.Result {
	activityIDStr := i.GetVar("_ctx_activity_id")
	activityID, _ := strconv.Atoi(activityIDStr)
	activity := q.findActivityByID(activityID)
	if activity == nil {
		return feather.Error(fmt.Errorf("no activity with ID %d found", activityID))
	}
	switch fieldName {
	case "id":
		return feather.OK(activity.ID)
	case "name":
		return feather.OK(activity.Name)
	case "description":
		return feather.OK(activity.Description)
	default:
		return feather.Error(fmt.Errorf("unknown field '%s'", fieldName))
	}
}

func (q *Query) anyStateCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("any-state {condition}"))
	}

	condition := args[0].String()
	sampleIDStr := i.GetVar("_ctx_sample_id")
	if sampleIDStr == "" {
		// No sample to matching against, return false
		return feather.OK(false)
	}

	sampleID, _ := strconv.Atoi(sampleIDStr)

	// Find the sample matching the ID
	entity := q.findEntityByID(sampleID)

	// Check if there is a match or if the sample has any states
	if entity == nil || len(entity.EntityStates) == 0 {
		return feather.OK(false)
	}

	saveStateIDStr := i.GetVar("_ctx_state_id")

	// Check if all state match condition
	for _, state := range entity.EntityStates {
		i.SetVar("_ctx_state_id", state.ID)

		exprCmd := fmt.Sprintf("expr {%s}", condition)
		result, err := i.Eval(exprCmd)

		if err != nil || isTrue(result) {
			// Found match, return true.
			if saveStateIDStr != "" {
				i.SetVar("_ctx_state_id", saveStateIDStr)
			}
			return feather.OK(true)
		}
	}

	if saveStateIDStr != "" {
		i.SetVar("_ctx_state_id", saveStateIDStr)
	}

	return feather.OK(false)
}

// allStatesCommand evaluate the condition against all states of the current sample
func (q *Query) allStatesCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("all-states {condition}"))
	}

	condition := args[0].String()
	sampleIDStr := i.GetVar("_ctx_sample_id")
	if sampleIDStr == "" {
		// No sample to matching against, return false
		return feather.OK(false)
	}

	sampleID, _ := strconv.Atoi(sampleIDStr)

	// Find the sample matching the ID
	entity := q.findEntityByID(sampleID)

	// Check if there is a match or if the sample has any states
	if entity == nil || len(entity.EntityStates) == 0 {
		return feather.OK(false)
	}

	saveStateIDStr := i.GetVar("_ctx_state_id")

	// Check if all state match condition
	for _, state := range entity.EntityStates {
		i.SetVar("_ctx_state_id", state.ID)

		exprCmd := fmt.Sprintf("expr {%s}", condition)
		result, err := i.Eval(exprCmd)

		if err != nil || !isTrue(result) {
			// Found a state that doesn't match, return false
			if saveStateIDStr != "" {
				i.SetVar("_ctx_state_id", saveStateIDStr)
			}
			return feather.OK(false)
		}
	}

	if saveStateIDStr != "" {
		i.SetVar("_ctx_state_id", saveStateIDStr)
	}

	return feather.OK(true)
}

func (q *Query) findEntityByID(id int) *mcmodel.Entity {
	for _, sample := range q.db.Samples {
		if sample.ID == id {
			return &sample
		}
	}
	return nil
}

func (q *Query) findActivityByID(id int) *mcmodel.Activity {
	for _, activity := range q.db.Processes {
		if activity.ID == id {
			return &activity
		}
	}
	return nil
}

// hasProcess checks if a sample has any process matching the condition
func (q *Query) hasProcessCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("has-activity requires {condition}"))
	}

	condition := args[0].String()

	sampleIDStr := i.GetVar("_ctx_sample_id")
	if sampleIDStr == "" {
		return feather.OK(false)
	}

	sampleID, _ := strconv.Atoi(sampleIDStr)

	activities, ok := q.db.SampleProcesses[sampleID]
	if !ok {
		return feather.OK(false)
	}

	// Save context
	savedSampleID := sampleIDStr
	savedType := i.GetVar("_ctx_type")

	// Check if ANY activity matches
	for _, activity := range activities {
		i.SetVar("_ctx_activity_id", activity.ID)
		i.SetVar("_ctx_type", "activity")

		exprCmd := fmt.Sprintf("expr {%s}", condition)
		result, err := i.Eval(exprCmd)

		if err == nil && isTrue(result) {
			// Found match. Restore context and return true.
			i.SetVar("_ctx_sample_id", savedSampleID)
			i.SetVar("_ctx_type", savedType)
			return feather.OK(true)
		}
	}

	// No matches found. Restore context and return false.
	i.SetVar("_ctx_sample_id", savedSampleID)
	i.SetVar("_ctx_type", savedType)

	return feather.OK(false)
}

// hasSample checks if a process has any sample matching the condition.
func (q *Query) hasSampleCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 1 {
		return feather.Error(fmt.Errorf("has-sample <sample-name>"))
	}

	condition := args[0].String()

	// Get the activity we are checking for the sample in.
	activityIDStr := i.GetVar("_ctx_activity_id")
	if activityIDStr == "" {
		// No activity ID set, cannot check for sample
		return feather.OK(false)
	}

	activityID, _ := strconv.Atoi(activityIDStr)

	samples, ok := q.db.ProcessSamples[activityID]
	if !ok {
		return feather.OK(false)
	}

	savedActivityIDStr := activityIDStr
	savedType := i.GetVar("_ctx_type")

	for _, sample := range samples {
		i.SetVar("_ctx_sample_id", sample.ID)
		i.SetVar("_ctx_type", "sample")

		exprCmd := fmt.Sprintf("expr {%s}", condition)
		result, err := i.Eval(exprCmd)

		// Found matching sample.
		if err == nil && isTrue(result) {
			// Restore context
			i.SetVar("_ctx_activity_id", savedActivityIDStr)
			i.SetVar("_ctx_type", savedType)
			return feather.OK(true)
		}
	}

	// No sample matching the condition was found. Restore context and return false.
	i.SetVar("_ctx_activity_id", savedActivityIDStr)
	i.SetVar("_ctx_type", savedType)
	return feather.OK(false)
}

func (q *Query) hasAttributeCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	return feather.Error(fmt.Errorf("has-attribute command not implemented"))
}

func (q *Query) containsCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 2 {
		return feather.Error(fmt.Errorf("contains str substr"))
	}
	str := args[0].String()
	substr := args[1].String()
	return feather.OK(strings.Contains(str, substr))
}

func (q *Query) startsWithCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 2 {
		return feather.Error(fmt.Errorf("starts-with str prefix"))
	}
	str := args[0].String()
	prefix := args[1].String()
	return feather.OK(strings.HasPrefix(str, prefix))
}

func (q *Query) endsWithCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	if len(args) != 2 {
		return feather.Error(fmt.Errorf("ends-with str prefix"))
	}
	str := args[0].String()
	suffix := args[1].String()
	return feather.OK(strings.HasSuffix(str, suffix))
}

func (q *Query) queryCommand(i *feather.Interp, o *feather.Obj, args []*feather.Obj) feather.Result {
	// Parse: query [select {columns}] <type> where {condition} [show {options}]
	var (
		selectColumns []string
		queryType     string
		condition     string
		showOptions   *ShowOptions
	)

	usageErr := fmt.Errorf("query [select {columns}] <type> where {condition} [show {options}]")

	// argIdx is the index of the current argument being parsed we advance it
	// we walk through the args array.
	argIdx := 0

	// Check for the optional 'select' clause
	if argIdx < len(args) && args[argIdx].String() == "select" {
		argIdx++
		// Check for required columns
		if argIdx >= len(args) {
			// If the argument index is greater than the number of args then
			// the columns to select are missing
			return feather.Error(usageErr)
		}
		// Parse columns
		selectColumns = q.parseColumnList(args[argIdx])
		argIdx++
	}

	// Check for the query type
	if argIdx > len(args) {
		return feather.Error(usageErr)
	}

	queryType = args[argIdx].String()
	argIdx++

	// Expect keyword 'where'
	if argIdx >= len(args) || args[argIdx].String() != "where" {
		return feather.Error(usageErr)
	}

	// Get the where condition
	if argIdx > len(args) {
		return feather.Error(usageErr)
	}

	condition = args[argIdx].String()
	argIdx++

	// Check for optional show clause
	if argIdx < len(args) && args[argIdx].String() == "show" {
		// Has a show keyword, let's make sure the clause is included and parse it.
		if argIdx >= len(args) {
			return feather.Error(usageErr)
		}

		showOptions = q.parseShowOptions(args[argIdx])
	}

	// Set select columns if not set
	if len(selectColumns) == 0 && showOptions != nil && len(showOptions.Columns) > 0 {
		// There was no select, but the user did specify columns in the show clause.
		selectColumns = showOptions.Columns
	}

	// Now execute the query
	switch queryType {
	case "samples":
		samples, err := q.executeSamplesQuery(i, condition)
		if err != nil {
			return feather.Error(err)
		}
		return q.formatSamplesOutput(samples, selectColumns, showOptions)

	case "processes":
		processes, err := q.executeProcessesQuery(i, condition)
		if err != nil {
			return feather.Error(err)
		}
		return q.formatProcessesOutput(processes, selectColumns, showOptions)
	default:
		return feather.Error(fmt.Errorf("unknown query type '%s'", queryType))
	}
}

func (q *Query) parseColumnList(arg *feather.Obj) []string {
	// Convert to string to handle string list and TCL list
	argStr := arg.String()

	// Check if it's a *, indicating all columns
	if argStr == "*" {
		return []string{"*"}
	}

	// Not a *, so let's get the individual columns

	// First see if it's a list
	if list, err := arg.List(); err == nil {
		var columns []string
		for _, item := range list {
			columns = append(columns, item.String())
		}
		return columns
	}

	// Not a list, so split the string
	return strings.Fields(argStr)
}

func (q *Query) parseShowOptions(arg *feather.Obj) *ShowOptions {
	opts := &ShowOptions{
		Format: "list", // default
	}

	// Try to parse as a dictionary
	dict, err := arg.Dict()
	if err != nil {
		// If not a dict, treat as a column list
		opts.Columns = q.parseColumnList(arg)
		return opts
	}

	// Parse dictionary options
	if columnsObj, ok := dict.Items["columns"]; ok {
		opts.Columns = q.parseColumnList(columnsObj)
	}

	if formatObj, ok := dict.Items["format"]; ok {
		opts.Format = formatObj.String()
	}

	if headersObj, ok := dict.Items["headers"]; ok {
		if list, err := headersObj.List(); err == nil {
			for _, item := range list {
				opts.Headers = append(opts.Headers, item.String())
			}
		}
	}

	if widthObj, ok := dict.Items["width"]; ok {
		opts.Width, _ = strconv.Atoi(widthObj.String())
	}

	return opts
}

func (q *Query) executeSamplesQuery(i *feather.Interp, condition string) ([]mcmodel.Entity, error) {
	var matched []mcmodel.Entity

	for _, sample := range q.db.Samples {
		i.SetVar("_ctx_sample_id", sample.ID)
		i.SetVar("_ctx_type", "sample")

		exprCmd := fmt.Sprintf("expr {%s}", condition)
		result, err := i.Eval(exprCmd)

		if err != nil {
			log.Warnf("Error evaluating condition for sample %d: %v", sample.ID, err)
			continue
		}

		if isTrue(result) {
			matched = append(matched, sample)
		}

	}

	return matched, nil
}

func (q *Query) formatSamplesOutput(samples []mcmodel.Entity, columns []string, options *ShowOptions) feather.Result {
	// Determine columns to display
	if len(columns) == 0 {
		// Default columns
		columns = []string{"id", "name", "category"}
	} else if len(columns) == 1 && columns[0] == "*" {
		// All available fields and common attributes
		columns = []string{"id", "name", "category", "description"}
	}

	// Determine format
	format := "list"
	if options != nil && options.Format != "" {
		format = options.Format
	}

	switch format {
	case "table":
		return q.formatSamplesAsTable(samples, columns, options)
	case "csv":
		return q.formatSamplesAsCSV(samples, columns)
	case "json":
		return q.formatSamplesAsJSON(samples, columns)
	default: // "list"
		return q.formatSamplesAsList(samples, columns)
	}
}

func (q *Query) formatSamplesAsTable(samples []mcmodel.Entity, columns []string, options *ShowOptions) feather.Result {
	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	defer table.Close()

	// Configure table style
	//table.SetAutoWrapText(false)
	//table.SetAutoFormatHeaders(true)
	//table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	//table.SetAlignment(tablewriter.ALIGN_LEFT)
	//table.SetBorder(true)
	//table.SetCenterSeparator("|")
	//table.SetColumnSeparator("|")
	//table.SetRowSeparator("-")

	// Set headers
	headers := columns
	if options != nil && len(options.Headers) > 0 {
		headers = options.Headers
	} else {
		// Auto-capitalize column names for headers
		headers = make([]string, len(columns))
		for i, col := range columns {
			headers[i] = q.formatColumnHeader(col)
		}
	}
	table.Header(headers)

	// Add rows
	for _, sample := range samples {
		row := make([]string, len(columns))
		for i, col := range columns {
			row[i] = q.getSampleColumnValue(&sample, col)
		}
		table.Append(row)
	}

	// Add footer with count
	footer := make([]string, len(columns))
	footer[0] = fmt.Sprintf("Total: %d", len(samples))
	for i := 1; i < len(columns); i++ {
		footer[i] = ""
	}
	table.Footer(footer)

	buf.WriteString("\n")
	table.Render()

	return feather.OK(buf.String())
}

func (q *Query) formatColumnHeader(col string) string {
	// Convert snake_case or camelCase to Title Case
	words := strings.FieldsFunc(col, func(r rune) bool {
		return r == '_' || r == '-'
	})

	for i, word := range words {
		//words[i] = strings.Title(strings.ToLower(word))
		words[i] = cases.Title(language.English).String(strings.ToLower(word))
	}

	return strings.Join(words, " ")
}

func (q *Query) formatSamplesAsCSV(samples []mcmodel.Entity, columns []string) feather.Result {
	var lines []string

	// Header
	lines = append(lines, strings.Join(columns, ","))

	// Rows
	for _, sample := range samples {
		row := make([]string, len(columns))
		for i, col := range columns {
			value := q.getSampleColumnValue(&sample, col)
			// Escape commas and quotes
			if strings.Contains(value, ",") || strings.Contains(value, "\"") {
				value = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\"\""))
			}
			row[i] = value
		}
		lines = append(lines, strings.Join(row, ","))
	}

	return feather.OK(strings.Join(lines, "\n"))
}

func (q *Query) formatSamplesAsJSON(samples []mcmodel.Entity, columns []string) feather.Result {
	var items []map[string]string

	for _, sample := range samples {
		item := make(map[string]string)
		for _, col := range columns {
			item[col] = q.getSampleColumnValue(&sample, col)
		}
		items = append(items, item)
	}

	// Simple JSON formatting (could use json.Marshal for production)
	var jsonLines []string
	jsonLines = append(jsonLines, "[")
	for i, item := range items {
		var pairs []string
		for k, v := range item {
			pairs = append(pairs, fmt.Sprintf("  \"%s\": \"%s\"", k, v))
		}
		line := "  {" + strings.Join(pairs, ", ") + "}"
		if i < len(items)-1 {
			line += ","
		}
		jsonLines = append(jsonLines, line)
	}
	jsonLines = append(jsonLines, "]")

	return feather.OK(strings.Join(jsonLines, "\n"))
}

// formatSamplesAsList outputs the samples as a list of dictionaries.
func (q *Query) formatSamplesAsList(samples []mcmodel.Entity, columns []string) feather.Result {
	var items []string

	for _, sample := range samples {
		var parts []string
		for _, col := range columns {
			value := q.getSampleColumnValue(&sample, col)
			parts = append(parts, fmt.Sprintf("%s: %s", col, value))
		}
		items = append(items, strings.Join(parts, " "))
	}

	return feather.OK(items)
}

func (q *Query) getSampleColumnValue(sample *mcmodel.Entity, column string) string {
	// Check if it's a built-in field
	switch column {
	case "id":
		return strconv.Itoa(sample.ID)
	case "name":
		return sample.Name
	case "description":
		return sample.Description
	case "category":
		return sample.Category
	case "owner_id":
		return strconv.Itoa(sample.OwnerID)
	case "project_id":
		return strconv.Itoa(sample.ProjectID)
	case "created_at":
		return sample.CreatedAt.Format(time.RFC3339)
	}

	// Otherwise, treat as attribute - search across all states
	if states, ok := q.db.SampleAttributesBySampleIDAndStates[sample.ID]; ok {
		for _, stateAttrs := range states {
			if attr, ok := stateAttrs[column]; ok {
				if len(attr.AttributeValues) > 0 {
					return q.formatAttributeValue(attr.AttributeValues[0])
				}
			}
		}
	}

	return "-"
}

func (q *Query) executeProcessesQuery(i *feather.Interp, condition string) ([]mcmodel.Activity, error) {
	var matched []mcmodel.Activity

	for _, activity := range q.db.Processes {
		i.SetVar("_ctx_activity_id", activity.ID)
		i.SetVar("_ctx_type", "activity")

		exprCmd := fmt.Sprintf("expr {%s}", condition)
		result, err := i.Eval(exprCmd)

		if err != nil {
			log.Warnf("Error evaluating condition for activity %d: %v", activity.ID, err)
			continue
		}

		if isTrue(result) {
			matched = append(matched, activity)
		}
	}

	return matched, nil
}

func (q *Query) formatProcessesOutput(activities []mcmodel.Activity, columns []string, options *ShowOptions) feather.Result {
	// Similar to formatSamplesOutput but for activities
	if len(columns) == 0 {
		columns = []string{"id", "name", "description"}
	}

	format := "list"
	if options != nil && options.Format != "" {
		format = options.Format
	}

	switch format {
	case "table":
		return q.formatActivitiesAsTable(activities, columns, options)
	case "csv":
		return q.formatActivitiesAsCSV(activities, columns)
	case "json":
		return q.formatActivitiesAsJSON(activities, columns)
	default:
		return q.formatActivitiesAsList(activities, columns)
	}
}

func (q *Query) formatActivitiesAsTable(activities []mcmodel.Activity, columns []string, options *ShowOptions) feather.Result {
	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	defer table.Close()

	//table.SetAutoWrapText(false)
	//table.SetAutoFormatHeaders(true)
	//table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	//table.SetAlignment(tablewriter.ALIGN_LEFT)
	//table.SetBorder(true)

	headers := columns
	if options != nil && len(options.Headers) > 0 {
		headers = options.Headers
	} else {
		headers = make([]string, len(columns))
		for i, col := range columns {
			headers[i] = q.formatColumnHeader(col)
		}
	}
	table.Header(headers)

	for _, activity := range activities {
		row := make([]string, len(columns))
		for i, col := range columns {
			row[i] = q.getActivityColumnValue(&activity, col)
		}
		table.Append(row)
	}

	footer := make([]string, len(columns))
	footer[0] = fmt.Sprintf("Total: %d", len(activities))
	for i := 1; i < len(columns); i++ {
		footer[i] = ""
	}
	table.Footer(footer)

	buf.WriteString("\n")
	table.Render()

	return feather.OK(buf.String())
}

func (q *Query) formatActivitiesAsCSV(activities []mcmodel.Activity, columns []string) feather.Result {
	var lines []string

	// Header
	lines = append(lines, strings.Join(columns, ","))

	// Rows
	for _, activity := range activities {
		row := make([]string, len(columns))
		for i, col := range columns {
			value := q.getActivityColumnValue(&activity, col)
			// Escape commas and quotes
			if strings.Contains(value, ",") || strings.Contains(value, "\"") {
				value = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\"\""))
			}
			row[i] = value
		}
		lines = append(lines, strings.Join(row, ","))
	}

	return feather.OK(strings.Join(lines, "\n"))
}

func (q *Query) formatActivitiesAsJSON(activities []mcmodel.Activity, columns []string) feather.Result {
	var items []map[string]string

	for _, activity := range activities {
		item := make(map[string]string)
		for _, col := range columns {
			item[col] = q.getActivityColumnValue(&activity, col)
		}
		items = append(items, item)
	}

	// Simple JSON formatting (could use json.Marshal for production)
	var jsonLines []string
	jsonLines = append(jsonLines, "[")
	for i, item := range items {
		var pairs []string
		for k, v := range item {
			pairs = append(pairs, fmt.Sprintf("  \"%s\": \"%s\"", k, v))
		}
		line := "  {" + strings.Join(pairs, ", ") + "}"
		if i < len(items)-1 {
			line += ","
		}
		jsonLines = append(jsonLines, line)
	}
	jsonLines = append(jsonLines, "]")

	return feather.OK(strings.Join(jsonLines, "\n"))
}

func (q *Query) formatActivitiesAsList(activities []mcmodel.Activity, columns []string) feather.Result {
	var items []string

	for _, activity := range activities {
		var parts []string
		for _, col := range columns {
			value := q.getActivityColumnValue(&activity, col)
			parts = append(parts, fmt.Sprintf("%s: %s", col, value))
		}
		items = append(items, strings.Join(parts, " "))
	}

	return feather.OK(items)
}

func (q *Query) getActivityColumnValue(activity *mcmodel.Activity, column string) string {
	switch column {
	case "id":
		return strconv.Itoa(activity.ID)
	case "name":
		return activity.Name
	case "description":
		return activity.Description
	case "owner_id":
		return strconv.Itoa(activity.OwnerID)
	case "project_id":
		return strconv.Itoa(activity.ProjectID)
	case "created_at":
		return activity.CreatedAt.Format(time.RFC3339)
	}

	// Check activity attributes
	if attrs, ok := q.db.ProcessAttributesByProcessID[activity.ID]; ok {
		if attr, ok := attrs[column]; ok {
			if len(attr.AttributeValues) > 0 {
				return q.formatAttributeValue(attr.AttributeValues[0])
			}
		}
	}

	return "-"
}

// formatAttributeValue converts attribute value to string
func (q *Query) formatAttributeValue(val mcmodel.AttributeValue) string {
	switch val.ValueType {
	case mcmodel.ValueTypeInt:
		return strconv.FormatInt(val.ValueInt, 10)
	case mcmodel.ValueTypeFloat:
		return strconv.FormatFloat(val.ValueFloat, 'f', -1, 64)
	case mcmodel.ValueTypeString:
		return val.ValueString
	default:
		return ""
	}
}

func isTrue(result *feather.Obj) bool {
	val := result.String()
	if val == "1" || val == "true" {
		return true
	}
	return false
}
