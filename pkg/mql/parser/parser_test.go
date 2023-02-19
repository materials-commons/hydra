package parser

import (
	"fmt"
	"github.com/materials-commons/hydra/pkg/mql/ast"
	"github.com/materials-commons/hydra/pkg/mql/lexer"
	"testing"

	"github.com/materials-commons/hydra/internal/mqldb"
)

func TestSimpleSelectStatement(t *testing.T) {
	input := `
	select samples;
	select processes;
	select samples, processes`

	mql := parseForTest(t, input, 3)

	tests := []struct {
		isSamplesSelection   bool
		isProcessesSelection bool
	}{
		{isSamplesSelection: true, isProcessesSelection: false},
		{isSamplesSelection: false, isProcessesSelection: true},
		{isSamplesSelection: true, isProcessesSelection: true},
	}
	for i, tt := range tests {
		stmt := mql.Statements[i]
		if !testSimpleSelectStatement(t, stmt, tt.isSamplesSelection, tt.isProcessesSelection) {
			return
		}
	}
}

func testSimpleSelectStatement(t *testing.T, s ast.Statement, isSamplesSelection, isProcessesSelection bool) bool {
	if s.TokenLiteral() != "select" {
		t.Errorf("s.TokenLiteral not 'select', got=%q", s.TokenLiteral())
		return false
	}
	selectStatement, ok := s.(*ast.SelectStatement)
	if !ok {
		t.Errorf("statement not *ast.SelectStatement, got=%T", selectStatement)
		return false
	}
	for _, selection := range selectStatement.SelectionStatements {
		switch st := selection.(type) {
		case *ast.SamplesSelectionStatement:
			if !isSamplesSelection {
				t.Errorf("Found samples selection, but was not expecting it")
				return false
			}
		case *ast.ProcessesSelectionStatement:
			if !isProcessesSelection {
				t.Errorf("Found processes selection, but was not expecting it")
				return false
			}
		default:
			t.Errorf("Expected *ast.{SamplesSelectionStatement,ProcessesSelectionStatement, got %T", st)
			return false
		}
	}
	return true
}

func TestSelectWithWhereStatement(t *testing.T) {
	input := `
	select samples where;
	select processes where;
	select samples, processes where;
`
	mql := parseForTest(t, input, 3)

	for _, s := range mql.Statements {
		if s.TokenLiteral() != "select" {
			t.Fatalf("s.TokenLiteral not 'select', got=%q", s.TokenLiteral())
		}
		selectStatement, ok := s.(*ast.SelectStatement)
		if !ok {
			t.Fatalf("statement not *ast.SelectStatement, got=%T", selectStatement)
		}
		if selectStatement.WhereStatement == nil {
			t.Fatalf("Statement should have had a where statement, but didn't")
		}
	}
}

func TestSampleIdentifierExpression(t *testing.T) {
	input := `sa:hardness;
		sample-attr:hardness;
		sample:hardness;
		s:hardness;
		sample:'with space';`

	tests := []struct {
		token      string
		identifier string
	}{
		{"sa:", "hardness"},
		{"sample-attr:", "hardness"},
		{"sample:", "hardness"},
		{"s:", "hardness"},
		{"sample:", "with space"},
	}

	mql := parseForTest(t, input, len(tests))

	for i, test := range tests {
		testSampleIdentifierExpression(t, mql.Statements[i], test.token, test.identifier)
	}

}

func testSampleIdentifierExpression(t *testing.T, s ast.Statement, token, identifier string) {
	es, ok := s.(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("Expected *ast.ExpressionStatement, got %T", s)
	}

	if es.TokenLiteral() != token {
		t.Fatalf("Expect token %s, got %s", token, es.TokenLiteral())
	}

	sampleAttribute, ok := es.Expression.(*ast.SampleAttributeIdentifier)
	if !ok {
		t.Fatalf("Expected *ast.SampleAttributeIdentifier, got %T", es.Expression)
	}

	if sampleAttribute.Value != identifier {
		t.Errorf("Expected value '%s', got %s", identifier, sampleAttribute.Value)
	}

	if sampleAttribute.TokenLiteral() != identifier {
		t.Errorf("Expected token literal '%s', got %s", identifier, sampleAttribute.Value)
	}
}

func TestProcessIdentifierExpression(t *testing.T) {
	input := `pa:hardness;
		process-attr:hardness;
		process:hardness;
		p:hardness;
		process:'with space';`

	tests := []struct {
		token      string
		identifier string
	}{
		{"pa:", "hardness"},
		{"process-attr:", "hardness"},
		{"process:", "hardness"},
		{"p:", "hardness"},
		{"process:", "with space"},
	}

	mql := parseForTest(t, input, len(tests))

	for i, test := range tests {
		testProcessIdentifierExpression(t, mql.Statements[i], test.token, test.identifier)
	}

}

func testProcessIdentifierExpression(t *testing.T, s ast.Statement, token, identifier string) {
	es, ok := s.(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("Expected *ast.ExpressionStatement, got %T", s)
	}

	if es.TokenLiteral() != token {
		t.Fatalf("Expect token %s, got %s", token, es.TokenLiteral())
	}

	processAttribute, ok := es.Expression.(*ast.ProcessAttributeIdentifier)
	if !ok {
		t.Fatalf("Expected *ast.ProcessAttributeIdentifier, got %T", es.Expression)
	}

	if processAttribute.Value != identifier {
		t.Errorf("Expected value '%s', got %s", identifier, processAttribute.Value)
	}

	if processAttribute.TokenLiteral() != identifier {
		t.Errorf("Expected token literal '%s', got %s", identifier, processAttribute.Value)
	}
}

func TestIntegerLiteralExpression(t *testing.T) {
	input := "5;"
	mql := parseForTest(t, input, 1)

	statement, ok := mql.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("statement not *ast.ExpressionStatement, got=%T", statement)
	}

	literal, ok := statement.Expression.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("literal not *ast.IntegerLiteral, got %T", literal)
	}

	if literal.Value != 5 {
		t.Fatalf("expected 5 for value got %d", literal.Value)
	}

	if literal.TokenLiteral() != "5" {
		t.Fatalf("expected token '5' got '%s'", literal.TokenLiteral())
	}
}

func TestBooleanLiteralExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, test := range tests {
		mql := parseForTest(t, test.input, 1)
		statement := checkForExpressionStatement(t, mql.Statements[0])
		b := checkForBooleanLiteral(t, statement.Expression)
		if b.Value != test.expected {
			t.Fatalf("Expected boolean literal %s to be %t, got %t", b.TokenLiteral(), test.expected, b.Value)
		}
	}
}

func TestParsingPrefixExpressions(t *testing.T) {
	tests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"-5", "-", 5},
	}

	for _, test := range tests {
		mql := parseForTest(t, test.input, 1)
		expression := checkForExpressionStatement(t, mql.Statements[0])
		prefixExpression := checkForPrefixExpression(t, expression.Expression)
		if prefixExpression.Operator != test.operator {
			t.Fatalf("expected prefixExpression.Operator '%s', got '%s'", test.operator, prefixExpression.Operator)
		}
		testLiteralExpression(t, prefixExpression.Right, test.value)
	}
}

func TestParsingInfixExpressions(t *testing.T) {
	tests := []struct {
		input    string
		left     interface{}
		operator string
		right    interface{}
	}{
		{"true = true;", true, "=", true},
		{"5+5;", 5, "+", 5},
	}

	for _, test := range tests {
		mql := parseForTest(t, test.input, 1)
		expression := checkForExpressionStatement(t, mql.Statements[0])
		testInfixExpression(t, expression.Expression, test.left, test.operator, test.right)
	}
}

func testInfixExpression(t *testing.T, e ast.Expression, left interface{}, operator string, right interface{}) {
	operatorExpression := checkForInfixExpression(t, e)

	testLiteralExpression(t, operatorExpression.Left, left)
	if operatorExpression.Operator != operator {
		t.Fatalf("Expected operator %s, got %s", operator, operatorExpression.Operator)
	}
	testLiteralExpression(t, operatorExpression.Right, right)
}

func TestComplexQueryExpression(t *testing.T) {
	//input := `select samples where sample:hardness = 5 and sample:color = 8`
	//mql := parseForTest(t, input, 1)
	//fmt.Println(mql)

	input := `select samples where (sample:hardness = 5 and (sample:'max size' > 5 or process:color = "blue"));`
	mql := parseForTest(t, input, 1)
	fmt.Println(mql)
	s, _ := mqldb.AST2Selection(mql)
	fmt.Printf("%+v\n", s)
}

func TestSimpleQueryExpression(t *testing.T) {
	input := `select samples where sample:hardness = 5`
	mql := parseForTest(t, input, 1)
	fmt.Println(mql)
	s, _ := mqldb.AST2Selection(mql)
	fmt.Printf("%+v\n", s)
}

func checkForExpressionStatement(t *testing.T, statement ast.Statement) *ast.ExpressionStatement {
	s, ok := statement.(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("statement not *ast.ExpressionStatement, got=%T", s)
	}

	return s
}

func checkForBooleanLiteral(t *testing.T, e ast.Expression) *ast.BooleanLiteral {
	b, ok := e.(*ast.BooleanLiteral)
	checkOk(t, "*ast.BooleanLiteral", b, ok)
	return b
}

func checkForPrefixExpression(t *testing.T, expression ast.Expression) *ast.PrefixExpression {
	e, ok := expression.(*ast.PrefixExpression)
	checkOk(t, "*ast.ExpressionStatement", e, ok)
	return e
}

func checkForInfixExpression(t *testing.T, expression ast.Expression) *ast.InfixExpression {
	e, ok := expression.(*ast.InfixExpression)
	checkOk(t, "*ast.InfixExpression", expression, ok)
	return e
}

func checkOk(t *testing.T, expectedType string, item interface{}, ok bool) {
	if !ok {
		t.Fatalf("statement not %s, got %T", expectedType, item)
	}
}

//func checkForIdentifier(t *testing.T, expression ast.Expression) *ast.Identifier {
//	i, ok := expression.(*ast.Identifier)
//	if
//}

func testLiteralExpression(t *testing.T, expression ast.Expression, expected interface{}) {
	switch v := expected.(type) {
	case int:
		testIntegerLiteral(t, expression, int64(v))
	}
}

func testIntegerLiteral(t *testing.T, expression ast.Expression, value int64) {
	i, ok := expression.(*ast.IntegerLiteral)
	checkOk(t, "*ast.IntegerLiteral", expression, ok)

	if i.Value != value {
		t.Fatalf("Expected value %d, got %d", value, i.Value)
	}

	if i.TokenLiteral() != fmt.Sprintf("%d", value) {
		t.Fatalf("Expected literal %s, got %s", fmt.Sprintf("%d", value), i.TokenLiteral())
	}
}

func testFloatLiteral(t *testing.T, expression ast.Expression, value float64) {

}

func testStringLiteral(t *testing.T, expression ast.Expression, value string) {

}

func testBooleanLiteral(t *testing.T, expression ast.Expression, value bool) {

}

func parseForTest(t *testing.T, input string, length int) *ast.MQL {
	l := lexer.New(input)
	p := New(l)

	mql := p.ParseMQL()
	if mql == nil {
		t.Fatalf("ParseMQL returned nil")
	}

	if len(mql.Statements) != length {
		t.Fatalf("program has wrong number of statements. Expected %d, got = %d", length, len(mql.Statements))
	}
	return mql
}
