package parser

import (
	"errors"
	"strings"

	"github.com/materials-commons/hydra/pkg/mql/ast"
)

var ErrNoSelectionStatement = errors.New("no selection statement")
var ErrInvalidWhereStatement = errors.New("invalid where statement")

func AST2Selection(query *ast.MQL) (*Selection, error) {
	var selection Selection
	ss, ok := query.Statements[0].(*ast.SelectStatement)
	if !ok {
		return nil, ErrNoSelectionStatement
	}

	for _, s := range ss.SelectionStatements {
		switch s.(type) {
		case *ast.ProcessesSelectionStatement:
			selection.SelectProcesses = true
		case *ast.SamplesSelectionStatement:
			selection.SelectSamples = true
		}
	}

	selection.Statement = convertAstExpression(ss.WhereStatement.Expression)

	return &selection, nil
}

func convertAstExpression(expression ast.Expression) Statement {
	switch e := expression.(type) {
	case *ast.InfixExpression:
		return convertAstInfixExpression(e)
	case *ast.SampleAttributeIdentifier:
		return sampleAttributeIdentifier2MatchStatement(e)
	case *ast.ProcessAttributeIdentifier:
		return processAttributeIdentifier2MatchStatement(e)
	default:
		return nil
	}
}

func processAttributeIdentifier2MatchStatement(ai *ast.ProcessAttributeIdentifier) MatchStatement {
	m := MatchStatement{}
	switch ai.Attribute {
	case "name":
		m.FieldType = ProcessFieldType
	default:
		m.FieldType = ProcessAttributeFieldType
	}

	m.FieldName = ai.Attribute
	m.Value = ai.Value
	m.Operation = ai.Operator

	return m
}

func sampleAttributeIdentifier2MatchStatement(ai *ast.SampleAttributeIdentifier) MatchStatement {
	m := MatchStatement{}
	switch ai.Attribute {
	case "name":
		m.FieldType = SampleFieldType
	default:
		m.FieldType = SampleAttributeFieldType
	}

	m.FieldName = ai.Attribute
	m.Value = ai.Value
	m.Operation = ai.Operator

	return m
}

func convertAstInfixExpression(ie *ast.InfixExpression) Statement {
	switch strings.ToLower(ie.Operator) {
	case "and":
		statement := AndStatement{}
		statement.Left = convertAstExpression(ie.Left)
		statement.Right = convertAstExpression(ie.Right)
		return statement
	case "or":
		statement := OrStatement{}
		statement.Left = convertAstExpression(ie.Left)
		statement.Right = convertAstExpression(ie.Right)
		return statement
	default:
		return nil
	}
}
