package mqldb

import (
	"fmt"
	"github.com/materials-commons/hydra/pkg/mql"
	"testing"
)

func TestSimpleProcessQueries(t *testing.T) {
	db := createTestDB()
	processMatchStatement := mql.MatchStatement{
		FieldType: mql.ProcessFieldType,
		FieldName: "name",
		Operation: "=",
		Value:     "Texture",
	}

	selection := selectAllProcesses()
	matchingProcesses, _ := EvalStatement(db, selection, processMatchStatement)
	if len(matchingProcesses) != 2 {
		t.Fatalf("Expected 2 matches on name = 'Texture', but got %d", len(matchingProcesses))
	}

	////////////////////////////////////

	processAttributeMatchStatement := mql.MatchStatement{
		FieldType: mql.ProcessAttributeFieldType,
		FieldName: "frames per second",
		Operation: ">",
		Value:     3,
	}

	matchingProcesses, _ = EvalStatement(db, selection, processAttributeMatchStatement)
	if len(matchingProcesses) != 1 {
		t.Fatalf("Expected 1 match on attribute 'frames per second' > 2, but got %d", len(matchingProcesses))
	}

	orStatement := mql.OrStatement{
		Left:  processMatchStatement,
		Right: processAttributeMatchStatement,
	}

	matchingProcesses, _ = EvalStatement(db, selection, orStatement)
	if len(matchingProcesses) != 3 {
		t.Fatalf("Expected 3 matches on: name = 'Texture' or attribute 'frames per second' > 2, but got %d", len(matchingProcesses))
	}

	////////////////////////////////////

	andStatement := mql.AndStatement{
		Left: mql.MatchStatement{
			FieldType: mql.ProcessAttributeFieldType,
			FieldName: "note",
			Operation: "=",
			Value:     "ignore these results",
		},
		Right: mql.MatchStatement{
			FieldType: mql.ProcessFieldType,
			FieldName: "name",
			Operation: "=",
			Value:     "Texture",
		},
	}

	matchingProcesses, _ = EvalStatement(db, selection, andStatement)
	if len(matchingProcesses) != 1 {
		t.Fatalf("Expected 1 match on: process name = 'Texture' and process attribute 'note' = 'ignore these results', but got %d", len(matchingProcesses))
	}

	orStatement.Left = andStatement
	orStatement.Right = mql.MatchStatement{
		FieldType: mql.ProcessAttributeFieldType,
		FieldName: "Beam Type",
		Operation: "=",
		Value:     "Wide",
	}

	matchingProcesses, _ = EvalStatement(db, selection, orStatement)
	if len(matchingProcesses) != 2 {
		t.Fatalf("Expected 2 matches on: (process name = 'Texture' and process attribute 'note' = 'ignore these results') or process attribute 'Beam Type' = 'Wide', but got %d", len(matchingProcesses))
	}
}

func TestComplexAndOrStatementProcessQuery(t *testing.T) {
	db := createTestDB()
	leftSideOfOrStatement := mql.AndStatement{
		Left: mql.MatchStatement{
			FieldType: mql.ProcessAttributeFieldType,
			FieldName: "note",
			Operation: "=",
			Value:     "ignore these results",
		},
		Right: mql.MatchStatement{
			FieldType: mql.ProcessFieldType,
			FieldName: "name",
			Operation: "=",
			Value:     "Texture",
		},
	}

	rightSideOfOrStatement := mql.OrStatement{
		Left: mql.MatchStatement{
			FieldType: mql.ProcessAttributeFieldType,
			FieldName: "Beam Type",
			Operation: "=",
			Value:     "Wide",
		},
		Right: mql.MatchStatement{
			FieldType: mql.ProcessAttributeFieldType,
			FieldName: "frames per second",
			Operation: "=",
			Value:     3,
		},
	}

	orStatement := mql.OrStatement{
		Left:  leftSideOfOrStatement,
		Right: rightSideOfOrStatement,
	}

	selection := selectAllProcesses()

	matchingProcesses, _ := EvalStatement(db, selection, orStatement)
	if len(matchingProcesses) != 3 {
		fmt.Printf("matching = %+v\n", matchingProcesses)
		t.Fatalf(`Expected 3 matches on: 
(process name = 'Texture' and process attribute 'note' = 'ignore these results') or
(process attribute 'Beam Type' = 'Wide' or process attribute 'frames per second' = 3), but got %d`, len(matchingProcesses))
	}
}

func TestSimpleSampleQueries(t *testing.T) {
	db := createTestDB()
	// Test simple match on sample name
	sampleNameMatchStatement := mql.MatchStatement{
		FieldType: mql.SampleFieldType,
		FieldName: "name",
		Operation: "=",
		Value:     "S1",
	}

	selection := selectAllSamples()
	_, matchingSamples := EvalStatement(db, selection, sampleNameMatchStatement)
	if len(matchingSamples) != 1 {
		t.Fatalf("Expected 1 match on: name = 'S1', got %d", len(matchingSamples))
	}

	// Test simple match on sample attribute
	sampleAttributeMatchStatement := mql.MatchStatement{
		FieldType: mql.SampleAttributeFieldType,
		FieldName: "alloy",
		Operation: "=",
		Value:     "zn45",
	}

	_, matchingSamples = EvalStatement(db, selection, sampleAttributeMatchStatement)
	if len(matchingSamples) != 1 {
		t.Fatalf("Expected 1 match on: sample attribute 'alloy' = 'zn45', got %d", len(matchingSamples))
	}

	// Test simple or statement using the above two statements
	orStatement := mql.OrStatement{
		Left:  sampleNameMatchStatement,
		Right: sampleAttributeMatchStatement,
	}

	_, matchingSamples = EvalStatement(db, selection, orStatement)
	if len(matchingSamples) != 2 {
		t.Fatalf(`Expected 1 match on: 
name = 'S1' or
sample attribute 'alloy' = 'zn45', got %d`, len(matchingSamples))
	}
}

func TestComplexAndOrStatementSampleQuery(t *testing.T) {
	db := createTestDB()
	// Matches sample S1 in entity state 2 attributes
	leftSideOfOrStatement := mql.AndStatement{
		Left: mql.MatchStatement{
			FieldType: mql.SampleAttributeFieldType,
			FieldName: "zn",
			Operation: "=",
			Value:     0.5,
		},
		Right: mql.MatchStatement{
			FieldType: mql.SampleAttributeFieldType,
			FieldName: "mg",
			Operation: "=",
			Value:     0.5,
		},
	}

	// Matches S2, entity state 3 for Left, and matches nothing on right
	rightSideOfOrStatement := mql.OrStatement{
		Left: mql.MatchStatement{
			FieldType: mql.SampleAttributeFieldType,
			FieldName: "ductility",
			Operation: "=",
			Value:     0.81,
		},
		Right: mql.MatchStatement{
			FieldType: mql.SampleAttributeFieldType,
			FieldName: "no-such",
			Operation: "=",
			Value:     0.5,
		},
	}

	orStatement := mql.OrStatement{
		Left:  leftSideOfOrStatement,
		Right: rightSideOfOrStatement,
	}

	selection := selectAllSamples()
	_, matchingSamples := EvalStatement(db, selection, orStatement)
	if len(matchingSamples) != 2 {
		t.Fatalf(`Expected x matches on:
(sample attribute 'zn' = 0.5 and sample attribute 'mg' = 0.5) or
(sample attribute 'ductility' = 0.81 or sample attribute 'no-such' = 0.5)
, got %d`, len(matchingSamples))
	}
}

func TestSimpleSelectProcessesThroughSamplesQuery(t *testing.T) {
	db := createTestDB()
	selection := selectAllProcesses()
	matchStatement := mql.MatchStatement{
		FieldType: mql.SampleAttributeFieldType,
		FieldName: "alloy",
		Operation: "=",
		Value:     "zn45",
	}

	matchingProcesses, matchingSamples := EvalStatement(db, selection, matchStatement)
	if len(matchingSamples) != 0 {
		t.Fatalf("Expected matchingSamples length = 0, got %d", len(matchingSamples))
	}

	if len(matchingProcesses) != 2 {
		t.Fatalf("Expected matchingProcesses length = 2, got %d", len(matchingProcesses))
	}
}

func TestSimpleSelectSamplesThroughProcessesQuery(t *testing.T) {
	db := createTestDB()
	selection := selectAllSamples()
	matchStatement := mql.MatchStatement{
		FieldType: mql.ProcessAttributeFieldType,
		FieldName: "Beam Type",
		Operation: "=",
		Value:     "Wide",
	}

	matchingProcesses, matchingSamples := EvalStatement(db, selection, matchStatement)
	if len(matchingProcesses) != 0 {
		t.Fatalf("Expected matchingProcesses length = 0, got %d", len(matchingProcesses))
	}

	if len(matchingSamples) != 2 {
		t.Fatalf("Expected matchingSamples length = 2, got %d", len(matchingSamples))
	}
}
