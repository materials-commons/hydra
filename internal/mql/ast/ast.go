package ast

import (
	"bytes"

	"github.com/materials-commons/hydra/internal/mql/token"
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
	WhereStatement      WhereStatement
}

func (s *SelectStatement) statementNode() {
}

func (s *SelectStatement) TokenLiteral() string {
	return s.Token.Literal
}

func (s *SelectStatement) String() string {
	var out bytes.Buffer

	for _, st := range s.SelectionStatements {
		out.WriteString(st.String())
	}

	out.WriteString(s.WhereStatement.String())

	return out.String()
}

/////////////////////////////////////////

type WhereStatement struct {
	Token      token.Token
	Statements []Statement
}

func (s *WhereStatement) statementNode() {
}

func (s *WhereStatement) TokenLiteral() string {
	return s.Token.Literal
}

func (s *WhereStatement) String() string {
	var out bytes.Buffer

	for _, st := range s.Statements {
		out.WriteString(st.String())
	}

	return out.String()
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
