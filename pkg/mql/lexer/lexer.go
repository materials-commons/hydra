package lexer

import (
	"github.com/materials-commons/hydra/pkg/mql/token"
)

type Lexer struct {
	input        string
	curPosition  int // current position in input (points to current char)
	readPosition int // current reading position, but not current position so this is "peeking" ahead
	ch           byte
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()

	switch l.ch {
	case '=':
		tok = newToken(token.EQUAL, l.ch)
	case '<':
		switch l.peekChar() {
		case '=':
			l.readChar() // advance
			tok = newTokenStr(token.LTEQ, "<=")
		case '>':
			l.readChar() // advance
			tok = newTokenStr(token.NOTEQ, "<>")
		default:
			tok = newToken(token.LT, l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar() // advance
			tok = newTokenStr(token.GTEQ, ">=")
		} else {
			tok = newToken(token.GT, l.ch)
		}
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '[':
		tok = newToken(token.LBRACKET, l.ch)
	case ']':
		tok = newToken(token.RBRACKET, l.ch)
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case '"':
		tok = newTokenStr(token.STRING, l.readString())
	case '\'':
		tok = newTokenStr(token.IDENT, l.readQuotedIdentifier())
	case '-':
		tok = newToken(token.MINUS, l.ch)
	case '+':
		tok = newToken(token.PLUS, l.ch)
	case '!':
		tok = newToken(token.BANG, l.ch)
	case 0:
		tok = newTokenStr(token.EOF, "")
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			// TODO: Add support for float point type
			// TODO: Add support for units
			return newTokenStr(token.INT, l.readNumber())
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	l.readChar() // advance
	return tok
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}

	// Advance current
	l.curPosition = l.readPosition
	l.readPosition += 1 // Peek ahead - Could advance past end of input
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}

	return l.input[l.readPosition]
}

func (l *Lexer) readNumber() string {
	position := l.curPosition
	for isDigit(l.ch) {
		l.readChar()
	}

	return l.input[position:l.curPosition]
}

func (l *Lexer) readString() string {
	position := l.curPosition + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.curPosition]
}

func (l *Lexer) readIdentifier() string {
	position := l.curPosition
	for {
		l.readChar()
		if l.ch == ':' {
			l.readChar()
			break
		}

		if !l.isUnquotedIdentifierChar(l.ch) {
			break
		}
	}
	return l.input[position:l.curPosition]
}

func (l *Lexer) isUnquotedIdentifierChar(ch byte) bool {
	if isLetter(ch) {
		return true
	}

	if isDigit(ch) {
		return true
	}

	if ch == '-' {
		return true
	}

	return false
}

func (l *Lexer) readQuotedIdentifier() string {
	// Advance past first quote
	l.readChar()
	position := l.curPosition
	for {
		l.readChar()
		if l.ch == '\'' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.curPosition]
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}

func newTokenStr(tokenType token.TokenType, s string) token.Token {
	return token.Token{Type: tokenType, Literal: s}
}
