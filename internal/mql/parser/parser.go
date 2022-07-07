package parser

import (
	"fmt"
	"log"
	"strconv"

	"github.com/materials-commons/hydra/internal/mql/ast"
	"github.com/materials-commons/hydra/internal/mql/lexer"
	"github.com/materials-commons/hydra/internal/mql/token"
)

// Precendence from lowest to highest
const (
	_ int = iota
	LOWEST
	EQUALS      // =
	LESSGREATER // > or < or <= or >=
	BOOLEAN
)

var precendences = map[token.TokenType]int{
	token.EQUAL: EQUALS,
	token.NOTEQ: EQUALS,
	token.LT:    LESSGREATER,
	token.LTEQ:  LESSGREATER,
	token.GT:    LESSGREATER,
	token.GTEQ:  LESSGREATER,
	token.AND:   BOOLEAN,
	token.NOT:   BOOLEAN,
	token.OR:    BOOLEAN,
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
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)

	// Read two tokens so that currentToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) appendError(msg string, args ...interface{}) {
	fmt.Sprintf(msg, args...)
	p.errors = append(p.errors, msg)
}

func (p *Parser) registerPrefix(t token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[t] = fn
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

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefixFn := p.prefixParseFns[p.curToken.Type]
	if prefixFn == nil {
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

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseMQL() *ast.MQL {
	mql := &ast.MQL{}
	mql.Statements = []ast.Statement{}
	for !p.curTokenIs(token.EOF) {
		statement := p.parseStatement()
		if statement != nil {
			mql.Statements = append(mql.Statements, statement)
		}
		p.nextToken()
	}
	return mql
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.SELECT:
		return p.parseSelectStatement()
	//case token.SEMICOLON:
	//	return
	default:
		// error here for now
		log.Fatalf("Top level statement can only be a select")
		return nil
	}
}

func (p *Parser) parseSelectStatement() ast.Statement {
	statement := &ast.SelectStatement{Token: p.curToken, SelectionStatements: []ast.Statement{}}
	p.nextToken()
	statement.SelectionStatements = p.parseSelectionStatements()
	if !p.expectPeek(token.WHERE) {
		return statement
	}
	return nil
}

func (p *Parser) parseSelectionStatements() []ast.Statement {
	return nil
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
