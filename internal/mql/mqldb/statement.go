package mqldb

const (
	ProcessFieldType          = 1
	SampleFieldType           = 2
	ProcessAttributeFieldType = 3
	SampleAttributeFieldType  = 4
	ProcessFuncType           = 5
	SampleFuncType            = 6
)

type Statement interface {
	statementNode()
}

type AndStatement struct {
	// Ignored field that is here to distinguish json from "OrStatement"
	And   int       `json:"and"`
	Left  Statement `json:"left"`
	Right Statement `json:"right"`
}

func (s AndStatement) statementNode() {
}

type OrStatement struct {
	// Ignored field that is here to distinguish json from "AndStatement"
	Or    int       `json:"or"`
	Left  Statement `json:"left"`
	Right Statement `json:"right"`
}

func (s OrStatement) statementNode() {
}

type MatchStatement struct {
	FieldType int         `json:"field_type"`
	FieldName string      `json:"field_name"`
	Operation string      `json:"operation"`
	Value     interface{} `json:"value"`
}

func (s MatchStatement) statementNode() {
}

func hasProcessMatchStatement(statement Statement) bool {
	switch s := statement.(type) {
	case MatchStatement:
		switch s.FieldType {
		case ProcessFieldType:
			return true
		case ProcessAttributeFieldType:
			return true
		case ProcessFuncType:
			return true
		default:
			return false
		}

	case AndStatement:
		if hasProcessMatchStatement(s.Left) {
			return true
		}
		if hasProcessMatchStatement(s.Right) {
			return true
		}
		return false

	case OrStatement:
		if hasProcessMatchStatement(s.Left) {
			return true
		}
		if hasProcessMatchStatement(s.Right) {
			return true
		}
		return false
	}

	return false
}

func hasSampleMatchStatement(statement Statement) bool {
	switch s := statement.(type) {
	case MatchStatement:
		switch s.FieldType {
		case SampleFieldType:
			return true
		case SampleAttributeFieldType:
			return true
		case SampleFuncType:
			return true
		default:
			return false
		}

	case AndStatement:
		if hasSampleMatchStatement(s.Left) {
			return true
		}
		if hasSampleMatchStatement(s.Right) {
			return true
		}
		return false

	case OrStatement:
		if hasSampleMatchStatement(s.Left) {
			return true
		}
		if hasSampleMatchStatement(s.Right) {
			return true
		}
		return false
	}

	return false
}
