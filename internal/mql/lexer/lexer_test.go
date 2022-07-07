package lexer

import (
	"testing"

	"github.com/materials-commons/hydra/internal/mql/token"
)

func TestNextToken(t *testing.T) {
	input := `select p:[name, a:time], s:[a:'metal hardness'] where and not null`
	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.SELECT, "select"},
		{token.PROCESS, "p:"},
		{token.LBRACKET, "["},
		{token.IDENT, "name"},
		{token.COMMA, ","},
		{token.ATTR, "a:"},
		{token.IDENT, "time"},
		{token.RBRACKET, "]"},
		{token.COMMA, ","},
		{token.SAMPLE, "s:"},
		{token.LBRACKET, "["},
		{token.ATTR, "a:"},
		{token.IDENT, "metal hardness"},
		{token.RBRACKET, "]"},
		{token.WHERE, "where"},
		{token.AND, "and"},
		{token.NOT, "not"},
		{token.NULL, "null"},
	}

	l := New(input)
	for i, test := range tests {
		tok := l.NextToken()
		if tok.Type != test.expectedType {
			t.Fatalf("tests[%d] - Token Type wrong. Expected='%s', got='%s': %s", i,
				token.TokenToStr(test.expectedType), token.TokenToStr(tok.Type), tok.Literal)
		}

		if tok.Literal != test.expectedLiteral {
			t.Fatalf("tests[%d] - Literal wrong. Expected=%q, got=%q", i, test.expectedLiteral, tok.Literal)
		}
	}
}
