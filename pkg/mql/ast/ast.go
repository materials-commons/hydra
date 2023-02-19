package ast

import (
	"bytes"
	"fmt"
	"github.com/materials-commons/hydra/pkg/mql/token"
	"strings"
)

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

/////////////////////////////////////////

type MQL struct {
	Statements []Statement
}

func (m *MQL) TokenLiteral() string {
	if len(m.Statements) > 0 {
		return m.Statements[0].TokenLiteral()
	}

	return ""
}

func (m *MQL) String() string {
	var out bytes.Buffer

	for _, s := range m.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

/////////////////////////////////////////

type SelectStatement struct {
	Token               token.Token
	SelectionStatements []Statement
	WhereStatement      *WhereStatement
}

func (s *SelectStatement) statementNode() {
}

func (s *SelectStatement) TokenLiteral() string {
	return s.Token.Literal
}

func (s *SelectStatement) String() string {
	var out bytes.Buffer

	out.WriteString(" select ")
	for _, st := range s.SelectionStatements {
		out.WriteString(st.String())
	}

	if s.WhereStatement != nil {
		out.WriteString(s.WhereStatement.String())
	}

	return out.String()
}

type SamplesSelectionStatement struct {
	Token token.Token
}

func (s *SamplesSelectionStatement) statementNode() {
}

func (s *SamplesSelectionStatement) TokenLiteral() string {
	return s.Token.Literal
}

func (s *SamplesSelectionStatement) String() string {
	return " samples "
}

type ProcessesSelectionStatement struct {
	Token token.Token
}

func (s *ProcessesSelectionStatement) statementNode() {
}

func (s *ProcessesSelectionStatement) TokenLiteral() string {
	return s.Token.Literal
}

func (s *ProcessesSelectionStatement) String() string {
	return " processes "
}

/////////////////////////////////////////

type WhereStatement struct {
	Token      token.Token
	Expression Expression
}

func (s *WhereStatement) statementNode() {
}

func (s *WhereStatement) TokenLiteral() string {
	return s.Token.Literal
}

func (s *WhereStatement) String() string {
	var out bytes.Buffer

	out.WriteString(" where ")
	out.WriteString(s.Expression.String())

	return out.String()
}

/////////////////////////////////////////

type SampleAttributeIdentifier struct {
	Token     token.Token
	Attribute string
	Operator  string
	Value     string
}

func (i *SampleAttributeIdentifier) expressionNode() {
}

func (i *SampleAttributeIdentifier) TokenLiteral() string {
	return i.Token.Literal
}

func (i *SampleAttributeIdentifier) String() string {
	if strings.Contains(i.Token.Literal, " ") {
		return fmt.Sprintf("sample:'%s' %s %s", i.Attribute, i.Operator, i.Value)
	}
	return fmt.Sprintf("sample:%s %s %s", i.Attribute, i.Operator, i.Value)
}

/////////////////////////////////////////

type ProcessAttributeIdentifier struct {
	Token     token.Token
	Attribute string
	Operator  string
	Value     string
}

func (i *ProcessAttributeIdentifier) expressionNode() {
}

func (i *ProcessAttributeIdentifier) TokenLiteral() string {
	return i.Token.Literal
}

func (i *ProcessAttributeIdentifier) String() string {
	if strings.Contains(i.Token.Literal, " ") {
		return fmt.Sprintf("process:'%s' %s %s", i.Attribute, i.Operator, i.Value)
	}
	return fmt.Sprintf("process:%s %s %s", i.Attribute, i.Operator, i.Value)
}

/////////////////////////////////////////

type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (l *IntegerLiteral) expressionNode() {
}

func (l *IntegerLiteral) TokenLiteral() string {
	return l.Token.Literal
}
func (l *IntegerLiteral) String() string {
	return l.Token.Literal
}

/////////////////////////////////////////

type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (l *FloatLiteral) expressionNode() {
}

func (l *FloatLiteral) TokenLiteral() string {
	return l.Token.Literal
}
func (l *FloatLiteral) String() string {
	return l.Token.Literal
}

/////////////////////////////////////////

type StringLiteral struct {
	Token token.Token
	Value string
}

func (l *StringLiteral) expressionNode() {
}

func (l *StringLiteral) TokenLiteral() string {
	return l.Token.Literal
}

func (l *StringLiteral) String() string {
	return l.Token.Literal
}

/////////////////////////////////////////

type BooleanLiteral struct {
	Token token.Token
	Value bool
}

func (l *BooleanLiteral) expressionNode() {
}

func (l *BooleanLiteral) TokenLiteral() string {
	return l.Token.Literal
}

func (l *BooleanLiteral) String() string {
	return l.Token.Literal
}

/////////////////////////////////////////

type ExpressionStatement struct {
	Token      token.Token
	Expression Expression
}

func (e *ExpressionStatement) statementNode() {
}

func (e *ExpressionStatement) TokenLiteral() string {
	return e.Token.Literal
}

func (e *ExpressionStatement) String() string {
	if e.Expression != nil {
		return e.Expression.String()
	}

	return fmt.Sprintf(" expression %s ", e.Token.Literal)
}

////////////////////////////////////////

type PrefixExpression struct {
	Token    token.Token
	Operator string
	Right    Expression
}

func (e *PrefixExpression) expressionNode() {
}

func (e *PrefixExpression) TokenLiteral() string {
	return e.Token.Literal

}

func (e *PrefixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(e.Operator)
	out.WriteString(e.Right.String())
	out.WriteString(")")

	return out.String()
}

/////////////////////////////////////////

type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (e *InfixExpression) expressionNode() {
}

func (e *InfixExpression) TokenLiteral() string {
	return e.Token.Literal

}

func (e *InfixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(e.Left.String())
	out.WriteString(" " + e.Operator + " ")
	out.WriteString(e.Right.String())
	out.WriteString(")")

	return out.String()
}

/////////////////////////////////////////

type ExecuteStatement struct {
	Token token.Token
}

func (s *ExecuteStatement) statementNode() {

}

func (s *ExecuteStatement) TokenLiteral() string {
	return s.Token.Literal
}

func (s *ExecuteStatement) String() string {
	return ";"
}
