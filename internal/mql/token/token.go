package token

import "fmt"

type TokenType int

const (
	EOF     TokenType = 0
	ILLEGAL           = -1

	// Identifiers and Literals
	IDENT  = 0x100
	INT    = 0x101
	FLOAT  = 0x102
	STRING = 0x103

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

	// built-in functions
	SAMPLE_HAS_PROCESS_FUNC    = 0x400 // s-has-process:
	SAMPLE_HAS_ATTRIBUTE_FUNC  = 0x401 // s-has-attribute:
	PROCESS_HAS_SAMPLE_FUNC    = 0x402 // p-has-sample:
	PROCESS_HAS_ATTRIBUTE_FUNC = 0x403 // p-has-attribute:

	// Keywords
	SAMPLE       = 0x700 // s:
	PROCESS      = 0x701 // p:
	ATTR         = 0x702 // a:
	SELECT       = 0x703 // select
	WHERE        = 0x704 // where
	NULL         = 0x705 // null
	SAMPLES      = 0x706 // samples
	PROCESSES    = 0x707 // processes
	PROCESS_ATTR = 0x708 // pa:
	SAMPLE_ATTR  = 0x709 // sa:
	TRUE         = 0x710 // true
	FALSE        = 0x711 // false

	// Elements
	LBRACKET  = 0x800 // [
	RBRACKET  = 0x801 // ]
	LPAREN    = 0x802 // (
	RPAREN    = 0x803 // )
	COMMA     = 0x804 // ,
	QUOTE     = 0x805 // "
	COLON     = 0x806 // :
	SEMICOLON = 0x807 // ;
	MINUS     = 0x808 // -
	BANG      = 0x809 // !
	PLUS      = 0x810 // +
)

type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]TokenType{
	"select": SELECT,
	"where":  WHERE,

	"s:":           SAMPLE_ATTR,
	"sa:":          SAMPLE_ATTR,
	"sample-attr:": SAMPLE_ATTR,
	"sample:":      SAMPLE_ATTR,

	"pa:":           PROCESS_ATTR,
	"process-attr:": PROCESS_ATTR,
	"process:":      PROCESS_ATTR,
	"p:":            PROCESS_ATTR,

	"samples":   SAMPLES,
	"processes": PROCESSES,
	"and":       AND,
	"or":        OR,
	"not":       NOT,
	"null":      NULL,

	"s-has-process:":  SAMPLE_HAS_PROCESS_FUNC,
	"s-has-attribute": SAMPLE_HAS_ATTRIBUTE_FUNC,
	"p-has-sample":    PROCESS_HAS_SAMPLE_FUNC,
	"p-has-attribute": PROCESS_HAS_ATTRIBUTE_FUNC,

	"true":  TRUE,
	"false": FALSE,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}

	return IDENT
}

var tokenToStr = map[TokenType]string{
	EQUAL:                      "EQUAL: =",
	LTEQ:                       "LTEQ: <=",
	NOTEQ:                      "NOTEQ: <>",
	LT:                         "LT: <",
	GTEQ:                       "GTEQ: >=",
	GT:                         "GT: >",
	COMMA:                      "COMMA: ,",
	LBRACKET:                   "LBRACKET: [",
	RBRACKET:                   "RBRACKET: ]",
	LPAREN:                     "LPAREN: (",
	RPAREN:                     "RPAREN: )",
	SEMICOLON:                  "SEMICOLON: ;",
	WHERE:                      "WHERE: where",
	AND:                        "AND: and",
	NOT:                        "NOT: not",
	NULL:                       "NULL: null",
	SAMPLE_HAS_PROCESS_FUNC:    "SAMPLE_HAS_PROCESS_FUNC: s-has-process:",
	SAMPLE_HAS_ATTRIBUTE_FUNC:  "SAMPLE_HAS_ATTRIBUTE_FUNC: s-has-attribute:",
	PROCESS_HAS_SAMPLE_FUNC:    "PROCESS_HAS_SAMPLE_FUNC: p-has-sample:",
	PROCESS_HAS_ATTRIBUTE_FUNC: "PROCESS_HAS_ATTRIBUTE_FUNC: p-has-attribute:",
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
