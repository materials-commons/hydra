package parser

import (
	"fmt"
	"strconv"

	"github.com/materials-commons/hydra/internal/mql"
	"github.com/materials-commons/hydra/internal/mql/ast"
	"github.com/materials-commons/hydra/internal/mql/lexer"
	"github.com/materials-commons/hydra/internal/mql/token"
)

// Precendence from lowest to highest
const (
	_ int = iota
	LOWEST
	EQUALS      // =
	LOGICAL     // and, or, not
	LESSGREATER // > or < or <= or >=
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X
	CALL        // func(x)
	INDEX       // array[index]

)

var precendences = map[token.TokenType]int{
	token.EQUAL: EQUALS,
	token.NOTEQ: EQUALS,
	token.LT:    LESSGREATER,
	token.LTEQ:  LESSGREATER,
	token.GT:    LESSGREATER,
	token.GTEQ:  LESSGREATER,
	token.AND:   LOGICAL,
	token.NOT:   LOGICAL,
	token.OR:    LOGICAL,
	token.PLUS:  SUM,
	token.MINUS: SUM,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l              *lexer.Lexer
	errors         []string
	curToken       token.Token
	peekToken      token.Token
	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.infixParseFns = make(map[token.TokenType]infixParseFn)

	// Prefix
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.TRUE, p.parseBoolLiteral)
	p.registerPrefix(token.FALSE, p.parseBoolLiteral)

	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.SAMPLES, p.parseSamplesLiteral)
	p.registerPrefix(token.PROCESSES, p.parseProcessesLiteral)
	p.registerPrefix(token.SAMPLE_HAS_ATTRIBUTE_FUNC, p.parseSampleHasAttributeFunc)
	p.registerPrefix(token.SAMPLE_HAS_PROCESS_FUNC, p.parseSampleHasProcessFunc)
	p.registerPrefix(token.PROCESS_HAS_ATTRIBUTE_FUNC, p.parseProcessHasAttributeFunc)
	p.registerPrefix(token.PROCESS_HAS_SAMPLE_FUNC, p.parseProcessHasSampleFunc)
	p.registerPrefix(token.SAMPLE_ATTR, p.parseSampleAttrFunc)
	p.registerPrefix(token.PROCESS_ATTR, p.parseProcessAttrFunc)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)

	// Infix
	p.registerInfix(token.EQUAL, p.parseInfixExpression)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.LTEQ, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.GTEQ, p.parseInfixExpression)

	// Read two tokens so that currentToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) ParseMQL() *ast.MQL {
	mqlProgram := &ast.MQL{}
	mqlProgram.Statements = []ast.Statement{}
	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			mqlProgram.Statements = append(mqlProgram.Statements, stmt)
		}
		p.nextToken()
	}

	return mqlProgram
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.SELECT:
		return p.parseSelectStatement()
	//case token.SEMICOLON:
	//	return
	default:
		// Allow expression parsing, at least for now
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseSelectStatement() ast.Statement {
	statement := &ast.SelectStatement{Token: p.curToken, SelectionStatements: []ast.Statement{}}
	if !p.peekTokenIs(token.SAMPLES) && !p.peekTokenIs(token.PROCESSES) {
		return nil
	}
	p.nextToken()
	statement.SelectionStatements = p.parseSelectionStatement()
	if p.curTokenIs(token.WHERE) {
		statement.WhereStatement = p.parseWhereStatement()
	}

	return statement
}

func (p *Parser) parseSelectionStatement() []ast.Statement {
	var selectionStatements []ast.Statement
	for {
		switch {
		case p.curTokenIs(token.SAMPLES):
			selectionStatements = append(selectionStatements, &ast.SamplesSelectionStatement{Token: p.curToken})
		case p.curTokenIs(token.PROCESSES):
			selectionStatements = append(selectionStatements, &ast.ProcessesSelectionStatement{Token: p.curToken})
		case p.curTokenIs(token.COMMA):
			// skip over to next token
		default:
			return selectionStatements
		}
		p.nextToken()
	}
}

func (p *Parser) parseWhereStatement() *ast.WhereStatement {
	whereStatement := &ast.WhereStatement{Token: p.curToken}

	// move past where
	p.nextToken()

	whereStatement.Expression = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return whereStatement
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	statement := &ast.ExpressionStatement{Token: p.curToken}
	statement.Expression = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return statement
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	//fmt.Printf("parseExpression p.curToken.Type = %d/%s\n", p.curToken.Type, p.curToken.Literal)
	prefixFn := p.prefixParseFns[p.curToken.Type]
	if prefixFn == nil {
		fmt.Printf("No function found for %d/%s\n", p.curToken.Type, p.curToken.Literal)
		p.appendError("no prefix parse function for %s found", token.TokenToStr(p.curToken.Type))
		return nil
	}

	leftExp := prefixFn()

	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infixFn := p.infixParseFns[p.peekToken.Type]
		if infixFn == nil {
			return leftExp
		}

		p.nextToken()
		leftExp = infixFn(leftExp)
	}

	return leftExp
}

func (p *Parser) parseSampleAttrFunc() ast.Expression {
	attribute, operator, value := p.parseAttribute()
	return &ast.SampleAttributeIdentifier{Token: p.curToken, Attribute: attribute, Operator: operator, Value: value}
}

func (p *Parser) parseProcessAttrFunc() ast.Expression {
	attribute, operator, value := p.parseAttribute()
	return &ast.ProcessAttributeIdentifier{Token: p.curToken, Attribute: attribute, Operator: operator, Value: value}
}

func (p *Parser) parseAttribute() (string, string, string) {
	p.nextToken()
	attribute := p.curToken.Literal
	p.nextToken()
	operator := p.curToken.Literal
	p.nextToken()
	value := p.curToken.Literal
	return attribute, operator, value
}

func (p *Parser) appendError(msg string, args ...interface{}) {
	p.errors = append(p.errors, fmt.Sprintf(msg, args...))
}

func (p *Parser) registerPrefix(t token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[t] = fn
}

func (p *Parser) registerInfix(t token.TokenType, fn infixParseFn) {
	p.infixParseFns[t] = fn
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	var err error
	literal := &ast.IntegerLiteral{Token: p.curToken}
	if literal.Value, err = strconv.ParseInt(p.curToken.Literal, 0, 64); err != nil {
		p.appendError("could not parse %q as integer", p.curToken.Literal)
		return nil
	}

	return literal
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	var err error
	literal := &ast.FloatLiteral{Token: p.curToken}
	if literal.Value, err = strconv.ParseFloat(p.curToken.Literal, 64); err != nil {
		p.appendError("could not parse %q as float", p.curToken.Literal)
		return nil
	}

	return literal
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBoolLiteral() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{Token: p.curToken, Operator: p.curToken.Literal}

	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{Token: p.curToken, Operator: p.curToken.Literal, Left: left}
	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)
	return expression
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	//fmt.Printf("parseGroupedExpression called\n")
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) peekPrecedence() int {
	return p.getPrecedence(p.peekToken.Type)
}

func (p *Parser) curPrecedence() int {
	return p.getPrecedence(p.curToken.Type)
}

func (p *Parser) getPrecedence(t token.TokenType) int {
	if p, ok := precendences[t]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) peekError(t token.TokenType) {
	p.errors = append(p.errors, fmt.Sprintf("Expect next token to be %s, got %s instead", token.TokenToStr(t),
		token.TokenToStr(p.peekToken.Type)))
}

func (p *Parser) parseAndExpression(expression ast.Expression) ast.Expression {
	s := mql.AndStatement{
		Left:  nil,
		Right: nil,
	}
	_ = s
	return nil
}

func (p *Parser) parseSamplesLiteral() ast.Expression {
	return nil
}

func (p *Parser) parseProcessesLiteral() ast.Expression {
	return nil
}

func (p *Parser) parseSampleHasAttributeFunc() ast.Expression {
	return nil
}

func (p *Parser) parseSampleHasProcessFunc() ast.Expression {
	return nil
}

func (p *Parser) parseProcessHasAttributeFunc() ast.Expression {
	return nil
}

func (p *Parser) parseProcessHasSampleFunc() ast.Expression {
	return nil
}
