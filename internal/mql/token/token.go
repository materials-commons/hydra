package token

import "fmt"

type TokenType int

const (
	EOF     TokenType = 0
	ILLEGAL           = -1

	// Identifiers and Literals
	IDENT        = 0x100
	INT          = 0x101
	FLOAT        = 0x102
	STRING       = 0x103
	BOOL         = 0x104
	QUOTED_IDENT = 0x105

	// Operators
	EQUAL = 0x200 // =
	LT    = 0x201 // <
	LTEQ  = 0x202 // <=
	GT    = 0x203 // >
	GTEQ  = 0x204 // >=
	NOTEQ = 0x205 // <>

	// Logical Operators
	AND = 0x300 // and
	OR  = 0x301 // or
	NOT = 0x302 // not

	// build-in functions
	HAS_PROCESS = 0x400 // has-process:

	// keywords
	SAMPLE  = 0x700 // s:
	PROCESS = 0x701 // p:
	ATTR    = 0x702 // a:
	SELECT  = 0x703 // select
	WHERE   = 0x704 // where
	NULL    = 0x705 // null

	// Elements
	LBRACKET  = 0x800 // [
	RBRACKET  = 0x801 // ]
	LPAREN    = 0x802 // (
	RPAREN    = 0x803 // )
	COMMA     = 0x804 // ,
	QUOTE     = 0x805 // "
	COLON     = 0x806 // :
	SEMICOLON = 0x807 // ;
)

type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]TokenType{
	"select":       SELECT,
	"where":        WHERE,
	"a:":           ATTR,
	"p:":           PROCESS,
	"s:":           SAMPLE,
	"and":          AND,
	"or":           OR,
	"not":          NOT,
	"null":         NULL,
	"has-process:": HAS_PROCESS,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}

	return IDENT
}

var tokenToStr = map[TokenType]string{
	EQUAL:       "EQUAL: =",
	LTEQ:        "LTEQ: <=",
	NOTEQ:       "NOTEQ: <>",
	LT:          "LT: <",
	GTEQ:        "GTEQ: >=",
	GT:          "GT: >",
	COMMA:       "COMMA: ,",
	LBRACKET:    "LBRACKET: [",
	RBRACKET:    "RBRACKET: ]",
	LPAREN:      "LPAREN: (",
	RPAREN:      "RPAREN: )",
	SEMICOLON:   "SEMICOLON: ;",
	WHERE:       "WHERE: where",
	AND:         "AND: and",
	NOT:         "NOT: not",
	NULL:        "NULL: null",
	HAS_PROCESS: "HAS_PROCESS: has-process:",
}

func TokenToStr(token TokenType) string {
	if token == INT {
		return "int"
	}

	if token == EOF {
		return "EOF"
	}

	if token == IDENT {
		return "IDENT"
	}

	if str, ok := tokenToStr[token]; ok {
		return str
	}

	return fmt.Sprintf("(%d) unknown", token)
}
