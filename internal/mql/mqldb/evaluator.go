package mqldb

import (
	"fmt"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

// EvalStatement runs a query and returns the results. At the moment selection is a simple boolean flag
// on whether to return samples and/or processes from the matches.
func EvalStatement(db *DB, selection Selection, statement Statement) ([]mcmodel.Activity, []mcmodel.Entity) {
	var (
		matchingProcesses []mcmodel.Activity
		matchingSamples   []mcmodel.Entity
	)
	switch {
	case selection.ProcessSelection.All && selection.SampleSelection.All:
		matchingProcesses, matchingSamples = evalSelectProcessesAndSamples(db, statement)
	case selection.SampleSelection.All:
		matchingSamples = evalSelectSamples(db, statement)
	case selection.ProcessSelection.All:
		matchingProcesses = evalSelectProcesses(db, statement)
	}

	return matchingProcesses, matchingSamples
}

// evalSelectProcessesAndSamples runs the match against both processes and samples.
func evalSelectProcessesAndSamples(db *DB, statement Statement) ([]mcmodel.Activity, []mcmodel.Entity) {
	processes := evalSelectProcesses(db, statement)
	samples := evalSelectSamples(db, statement)
	return processes, samples
}

// evalSelectSamples will only return matching samples. This method checks if there are sample or process
// matching statements, and runs matches against samples and/or processes. If there is a process run it
// then takes the results from the processes and filters it down to just the unique samples associated
// the process.
func evalSelectSamples(db *DB, statement Statement) []mcmodel.Entity {
	var matchingSamples []mcmodel.Entity
	var matchingProcesses []mcmodel.Activity

	if hasSampleMatchStatement(statement) {
		matchingSamples = evalMatchingSamples(db, statement)
	}

	if hasProcessMatchStatement(statement) {
		matchingProcesses = evalMatchingProcesses(db, statement)
	}

	processSamples := uniqueSamplesForProcesses(db, matchingProcesses)

	// We now have two lists of samples - samples that matched from evalMatchingSamples, and samples that
	// were matched by finding the processes and then the samples for those processes. These two lists of
	// samples may share the same sample. Since we want to return just the unique list of matches we go
	// through each of these lists to identify the samples that are unique across the lists. We do this
	// by creating a hash table of sample ids. Then taking those unique samples and creating a list of
	// the unique samples to return.
	var samplesToReturn []mcmodel.Entity
	uniqueSamples := make(map[int]mcmodel.Entity)
	for _, sample := range matchingSamples {
		uniqueSamples[sample.ID] = sample
	}

	for _, sample := range processSamples {
		uniqueSamples[sample.ID] = sample
	}

	for _, sample := range uniqueSamples {
		samplesToReturn = append(samplesToReturn, sample)
	}

	return samplesToReturn
}

// uniqueSamplesForProcesses takes a list of matching processes, finds all the samples that those
// processes refer to, and then filters out all duplicates.
func uniqueSamplesForProcesses(db *DB, processes []mcmodel.Activity) []mcmodel.Entity {
	var samples []mcmodel.Entity
	uniqueSamples := make(map[int]*mcmodel.Entity)
	for _, process := range processes {
		for _, sample := range db.ProcessSamples[process.ID] {
			uniqueSamples[sample.ID] = sample
		}
	}

	for _, sample := range uniqueSamples {
		samples = append(samples, *sample)
	}

	return samples
}

// evalSelectProcesses will only return matching processes. This method checks if there are sample or process
// matching statements, and runs matches against samples and/or processes. If there is a sample run it
// then takes the results from the sample and filters it down to just the unique processes associated with the
// samples.
func evalSelectProcesses(db *DB, statement Statement) []mcmodel.Activity {
	var matchingProcesses []mcmodel.Activity
	var matchingSamples []mcmodel.Entity

	if hasProcessMatchStatement(statement) {
		matchingProcesses = evalMatchingProcesses(db, statement)
	}

	if hasSampleMatchStatement(statement) {
		matchingSamples = evalMatchingSamples(db, statement)
	}

	sampleProcesses := uniqueProcessesForSamples(db, matchingSamples)

	// We now have two lists of processes - processes that matched from evalMatchingProcesses, and processes that
	// were matched by finding the samples and then the processes for those samples. These two lists of
	// processes may share the same process. Since we want to return just the unique list of matches we go
	// through each of these lists to identify the processes that are unique across the lists. We do this
	// by creating a hash table of process ids. Then taking those unique processes and creating a list of
	// the unique processes to return.
	var processesToReturn []mcmodel.Activity
	uniqueProcesses := make(map[int]mcmodel.Activity)
	for _, process := range matchingProcesses {
		uniqueProcesses[process.ID] = process
	}

	for _, process := range sampleProcesses {
		uniqueProcesses[process.ID] = process
	}

	for _, sample := range uniqueProcesses {
		processesToReturn = append(processesToReturn, sample)
	}

	return processesToReturn
}

// uniqueProcessesForSamples takes a list of matching samples, finds all the processes that those
// samples are associated with, and then filters out all duplicates.
func uniqueProcessesForSamples(db *DB, samples []mcmodel.Entity) []mcmodel.Activity {
	var processes []mcmodel.Activity
	uniqueProcesses := make(map[int]*mcmodel.Activity)
	for _, sample := range samples {
		for _, process := range db.SampleProcesses[sample.ID] {
			uniqueProcesses[process.ID] = process
		}
	}

	for _, process := range uniqueProcesses {
		processes = append(processes, *process)
	}

	return processes
}

// evalMatchingProcesses finds all the matching processes with a statement
func evalMatchingProcesses(db *DB, statement Statement) []mcmodel.Activity {
	var matchingProcesses []mcmodel.Activity
	uniqueProcessMatches := make(map[int]mcmodel.Activity)
	for _, process := range db.Processes {
		if eval(db, &process, nil, statement) {
			uniqueProcessMatches[process.ID] = process
		}
	}

	for _, process := range uniqueProcessMatches {
		matchingProcesses = append(matchingProcesses, process)
	}

	return matchingProcesses
}

// SampleState is the state associated with a sample. The IDs contained in the structures
// are used to look up items in the hash tables in the DB.
type SampleState struct {
	sample        *mcmodel.Entity
	EntityStateID int
}

// evalMatchingSamples finds all the matching samples for a statement. This method must iterate through
// the states associated with a sample. Once it finds a match in a sample state it will stop searching
// and ignore the other sample states.
func evalMatchingSamples(db *DB, statement Statement) []mcmodel.Entity {
	var matchingSamples []mcmodel.Entity
	uniqueSampleMatches := make(map[int]mcmodel.Entity)
	for _, sample := range db.Samples {
		for _, entityState := range sample.EntityStates {
			sampleState := SampleState{&sample, entityState.ID}
			if eval(db, nil, &sampleState, statement) {
				// Found a match on the sample, no need to check other sample states so break out of the state loop
				uniqueSampleMatches[sample.ID] = sample
				break
			}
		}
	}

	for _, sample := range uniqueSampleMatches {
		matchingSamples = append(matchingSamples, sample)
	}

	return matchingSamples
}

// eval is the heart of the statement evaluation. It handles individual matches as well as complex
// statements.
func eval(db *DB, process *mcmodel.Activity, sampleState *SampleState, statement Statement) bool {
	switch s := statement.(type) {
	case MatchStatement:
		return evalMatchStatement(db, process, sampleState, s)
	case AndStatement:
		return evalAndStatement(db, process, sampleState, s)
	case OrStatement:
		return evalOrStatement(db, process, sampleState, s)
	default:
		return false
	}
}

// evalAndStatement evaluates an AndStatement. It short circuits its check by returning false if the left side
// evaluates to false.
func evalAndStatement(db *DB, process *mcmodel.Activity, sampleState *SampleState, statement AndStatement) bool {
	if !eval(db, process, sampleState, statement.Left) {
		return false
	}

	return eval(db, process, sampleState, statement.Right)
}

// evalOrStatement evaluates an OrStatement. It short circuits its check by returning if the left evaluates
// to true.
func evalOrStatement(db *DB, process *mcmodel.Activity, sampleState *SampleState, statement OrStatement) bool {
	if eval(db, process, sampleState, statement.Left) {
		return true
	}

	return eval(db, process, sampleState, statement.Right)
}

// evalMatchStatement evaluates a MatchStatment which is a leaf node matching against a specific type of item such
// as a process or sample attribute, or similar.
func evalMatchStatement(db *DB, process *mcmodel.Activity, sampleState *SampleState, match MatchStatement) bool {
	switch match.FieldType {
	case ProcessFieldType:
		return evalProcessFieldMatch(process, match)
	case ProcessAttributeFieldType:
		// There are two contexts in which to evaluate a process attribute - A sample or a process context. When in
		// the sample context we need to find the processes associated with a sample and then evaluate the attributes.
		// The context is determined by checking if sampleState is nil. If sampleState is not nil, then we are in a
		// sample context.
		if sampleState != nil {
			return evalProcessAttributeFieldMatchForSampleState(sampleState, db, match)
		}
		return evalProcessAttributeFieldMatch(process, db, match)
	case SampleFieldType:
		return evalSampleFieldMatch(sampleState, match)
	case SampleAttributeFieldType:
		// There are two contexts in which to evaluate a sample attribute - A sample or a process context. When in
		// the process context we need to find the samples associated with the process and then evaluate the attributes.
		// The context is determined by checking if process is nil. If process is not nil, then we are in a process
		// context.
		if process != nil {
			return evalSampleAttributeFieldMatchForProcess(process, db, match)
		}
		return evalSampleAttributeFieldMatch(sampleState, db, match)
	case ProcessFuncType:
		return evalProcessFuncMatch(process, db, match)
	case SampleFuncType:
		return evalSampleFuncMatch(sampleState, db, match)
	}

	return false
}

// evalSampleFuncMatch is called when the user as specified one of the built in sample matching functions. It determines
// the function being called and performs the evaluation.
func evalSampleFuncMatch(state *SampleState, db *DB, match MatchStatement) bool {
	if state == nil {
		return false
	}

	switch {
	case match.Operation == "has-process":
		// matching samples that are used in the given process
		return evalSampleFuncMatchHasProcess(state, db, match.Value.(string))
	case match.Operation == "has-attribute":
		return evalSampleFuncMatchHasAttribute(state, db, match.Value.(string))
	}
	return false
}

// evalSampleFuncMatchHasProcess implements the has-process function for samples. The has-process function
// for samples determines if a sample went through a particular process.
func evalSampleFuncMatchHasProcess(state *SampleState, db *DB, processType string) bool {
	processes, ok := db.SampleProcesses[state.sample.ID]
	if !ok {
		// weird... sample doesn't have any processes... (shouldn't happen)
		return false
	}

	for _, process := range processes {
		if process.Name == processType {
			return true
		}
	}

	return false
}

// evalSampleFuncMatchHasAttribute implements the has-attribute function for samples. The has-attribute function
// for samples determines if a sample has a particular attribute in any of it's state.
func evalSampleFuncMatchHasAttribute(sampleState *SampleState, db *DB, attributeName string) bool {
	// Sanity check, make sure sampleState isn't nil
	if sampleState == nil {
		return false
	}

	// Get all the states for the sample
	states, ok := db.SampleAttributesBySampleIDAndStates[sampleState.sample.ID]
	if !ok {
		return false
	}

	// Get all the attributes associated with the specific sample and sample state
	attributes, ok := states[sampleState.EntityStateID]
	if !ok {
		fmt.Printf("     Sample %d/%s with state %d has no attributes\n", sampleState.sample.ID, sampleState.sample.Name, sampleState.EntityStateID)
		return false
	}

	// From that list of attributes, check if it contains the specific attribute
	_, ok = attributes[attributeName]
	return ok
}

// evalProcessFuncMatch when a user has specified a process level built in function.
func evalProcessFuncMatch(process *mcmodel.Activity, db *DB, match MatchStatement) bool {
	if process == nil {
		return false
	}

	switch {
	case match.Operation == "has-sample":
		// matching samples that are used in the given process
		return evalProcessFuncMatchHasSample(process, db, match.Value.(string))
	case match.Operation == "has-attribute":
		return evalProcessFuncMatchHasAttribute(process, db, match.Value.(string))
	}
	return false
}

// evalProcessFuncMatchHasSample implements the has-sample process function. The has-sample process function
// determines if a process has a given sample name.
func evalProcessFuncMatchHasSample(process *mcmodel.Activity, db *DB, sampleName string) bool {
	samples, ok := db.ProcessSamples[process.ID]
	if !ok {
		// Process has no samples
		return false
	}

	for _, sample := range samples {
		if sample.Name == sampleName {
			return true
		}
	}

	return false
}

// evalProcessFuncMatchHasAttribute implements the has-attribute process function. The has-attribute process
// function determines if a process has a given process attribute.
func evalProcessFuncMatchHasAttribute(process *mcmodel.Activity, db *DB, attributeName string) bool {
	attributes, ok := db.ProcessAttributesByProcessID[process.ID]
	if !ok {
		// Process has no attributes
		return false
	}

	_, ok = attributes[attributeName]
	return ok
}

// evalProcessAttributeFieldMatchForSampleState evaluates a process attribute match in the context of a sample. To do
// this it uses the sample to look up all the processes associated with the sample and then evaluates them, stopping
// if one of them evaluates to true.
func evalProcessAttributeFieldMatchForSampleState(sampleState *SampleState, db *DB, match MatchStatement) bool {
	// Get the processes associated with the sample
	processes, ok := db.SampleProcesses[sampleState.sample.ID]
	if !ok {
		// No processes to evaluate!
		return false
	}

	// Loop through the processes looking for a match on the specified attribute
	for _, process := range processes {
		if evalProcessAttributeFieldMatch(process, db, match) {
			return true
		}
	}
	return false
}

// evalProcessAttributeFieldMatch evaluates the match statement against the given process. It checks the
// process for the attribute in the match statement and if it exists evaluates the match statement against
// that attribute.
func evalProcessAttributeFieldMatch(process *mcmodel.Activity, db *DB, match MatchStatement) bool {
	if process == nil {
		// There are contexts in which a null process may be passed in. When that happens just return false
		// (no match)
		return false
	}

	// Get the attributes for the process
	attributes, ok := db.ProcessAttributesByProcessID[process.ID]
	if !ok {
		fmt.Printf("    Process %d/%s has no attributes\n", process.ID, process.Name)
		return false
	}

	// Get the given attribute in the match for the process
	attribute, ok := attributes[match.FieldName]
	if !ok {
		return false
	}

	// An attribute may have a list of values, so evaluate the match statement against each
	// stopping if one matches.
	for _, value := range attribute.AttributeValues {
		switch value.ValueType {
		case mcmodel.ValueTypeInt:
			return tryEvalAttributeIntMatch(value.ValueInt, match)
		case mcmodel.ValueTypeFloat:
			return tryEvalAttributeFloatMatch(value.ValueFloat, match)
		case mcmodel.ValueTypeString:
			return tryEvalAttributeStringMatch(value.ValueString, match)
		}
	}

	return false
}

// evalSampleAttributeFieldMatchForProcess evaluates a sample field in a process context. It looks up the samples
// for a given process and then runs an evaluation again each of them.
func evalSampleAttributeFieldMatchForProcess(process *mcmodel.Activity, db *DB, match MatchStatement) bool {
	// Get the list of samples associated with the process
	samples, ok := db.ProcessSamples[process.ID]
	if !ok {
		return false
	}

	// Loop through each sample and its various states checking for a match.
	for _, sample := range samples {
		for _, state := range sample.EntityStates {
			sampleState := &SampleState{
				sample:        sample,
				EntityStateID: state.ID,
			}
			if evalSampleAttributeFieldMatch(sampleState, db, match) {
				return true
			}
		}
	}

	return false
}

// evalSampleAttributeFieldMatch evaluates a attribute match against a sample in a specific sample state.
func evalSampleAttributeFieldMatch(sampleState *SampleState, db *DB, match MatchStatement) bool {
	// Sanity check, make sure sampleState isn't nil
	if sampleState == nil {
		return false
	}

	// Get all the states for the sample
	states, ok := db.SampleAttributesBySampleIDAndStates[sampleState.sample.ID]
	if !ok {
		return false
	}

	// Get all the attributes associated with the specific sample and sample state
	attributes, ok := states[sampleState.EntityStateID]
	if !ok {
		fmt.Printf("     Sample %d/%s with state %d has no attributes\n", sampleState.sample.ID, sampleState.sample.Name, sampleState.EntityStateID)
		return false
	}

	// From that list of attributes, check if it contains the specific attribute
	attribute, ok := attributes[match.FieldName]
	if !ok {
		return false
	}

	// Attributes can have multiple values, loop through each value, stopping if a match evaluates to true.
	for _, value := range attribute.AttributeValues {
		switch value.ValueType {
		case mcmodel.ValueTypeInt:
			return tryEvalAttributeIntMatch(value.ValueInt, match)
		case mcmodel.ValueTypeFloat:
			return tryEvalAttributeFloatMatch(value.ValueFloat, match)
		case mcmodel.ValueTypeString:
			return tryEvalAttributeStringMatch(value.ValueString, match)
		}
	}

	return false
}
