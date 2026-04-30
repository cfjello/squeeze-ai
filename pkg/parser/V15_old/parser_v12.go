// Package parser implements a hand-written recursive-descent parser for the
// Squeeze language (V12), following the plan in specifications/parser_v3.md.
//
// This file is structured in phases:
//
//	Phase 1 — Token types & Lexer          (this file, complete)
//	Phase 2 — AST node types               (this file, complete for 01_definitions)
//	Phase 3 — Recursive-descent parser     (this file, 01_definitions rules)
//	Phase 4 — Directive processor          (TODO: full wiring in subsequent files)
package parser

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// =============================================================================
// PHASE 1 — TOKEN TYPES (V12)
// =============================================================================

// V12TokenType identifies the category of a lexed token.
type V12TokenType int

const (
	// ---- Synthetic / control tokens ----
	V12_BOF     V12TokenType = iota // Beginning of file (first token always emitted)
	V12_EOF                         // End of file (last token always emitted)
	V12_NL                          // Newline sequence  /([ \t]*[\r\n]+)+/ or BOF
	V12_ILLEGAL                     // Unrecognised character

	// ---- Literal value tokens ----
	V12_INTEGER // unsigned digit sequence  e.g. 42  (sign is a separate token)
	V12_DECIMAL // unsigned decimal  e.g. 3.14
	V12_STRING  // single/double/template quoted  '…' "…" `…` (delimiters included)
	V12_REGEXP  // regexp literal  /pattern/flags

	// ---- Identifier tokens ----
	V12_IDENT    // plain identifier  (Unicode letters, digits, _, embedded spaces)
	V12_AT_IDENT // @-prefixed identifier  @name

	// ---- Boolean / null keywords ----
	V12_TRUE  // true
	V12_FALSE // false
	V12_NULL  // null

	// ---- Cardinality upper-bound keyword ----
	V12_MANY // many | Many

	// ---- Built-in type keywords ----
	V12_NAN      // NaN
	V12_INFINITY // Infinity | +Infinity | -Infinity  (sign emitted as separate tok)

	// ---- Duration-unit keywords ----
	V12_MS  // ms
	V12_SEC // s
	V12_MIN // m
	V12_HR  // h
	V12_DAY // d
	V12_WK  // w

	// ---- Parser directive keywords ----
	V12_UNIQUE     // UNIQUE
	V12_RANGE_KW   // RANGE
	V12_TYPE_OF    // TYPE_OF
	V12_VALUE_OF   // VALUE_OF
	V12_ADDRESS_OF // ADDRESS_OF
	V12_RETURN_DIR // RETURN  (directive form — distinct from <- return-statement)
	V12_UNIFORM    // UNIFORM
	V12_INFER      // INFER
	V12_CAST       // CAST
	V12_EXTEND     // EXTEND
	V12_MERGE      // MERGE
	V12_SQZ        // SQZ
	V12_CODE       // CODE

	// ---- Multi-character operators (max-munch) ----
	V12_DOTDOT        // ..    range separator
	V12_DOTDOLLAR     // .$    JSONPath entry shorthand
	V12_ARROW         // ->    data-chain connector
	V12_STREAM        // >>    stream connector
	V12_STORE         // =>    store/deps operator
	V12_RETURN_STMT   // <-    return statement marker
	V12_INC           // ++    inline increment
	V12_DEC           // --    inline decrement
	V12_POW           // **    exponentiation
	V12_IADD_IMM      // +:    incremental immutable assign +
	V12_ISUB_IMM      // -:    incremental immutable assign -
	V12_IMUL_IMM      // *:    incremental immutable assign *
	V12_IDIV_IMM      // /:    incremental immutable assign /
	V12_IADD_MUT      // +~    incremental mutable assign +
	V12_ISUB_MUT      // -~    incremental mutable assign -
	V12_IMUL_MUT      // *~    incremental mutable assign *
	V12_IDIV_MUT      // /~    incremental mutable assign /
	V12_READONLY      // :~    read-only reference assign
	V12_EXTEND_ASSIGN // +=    incremental mutable assign + / extend-scope operator
	V12_ISUB_ASSIGN   // -=    incremental mutable assign -
	V12_IMUL_ASSIGN   // *=    incremental mutable assign *
	V12_IDIV_ASSIGN   // /=    incremental mutable assign /
	V12_GEQ           // >=    compare
	V12_LEQ           // <=    compare
	V12_NEQ           // !=    compare (not-equal)
	V12_EQEQ          // ==    equality compare
	V12_MATCH_OP      // =~    string-regexp compare
	V12_AMP_AMP       // &&    logical and
	V12_PIPE_PIPE     // ||    logical or
	V12_EMPTY_ARR     // []    empty array initialiser
	V12_EMPTY_OBJ     // {}    empty object initialiser
	V12_EMPTY_STR_D   // ""    empty double-quoted string
	V12_EMPTY_STR_S   // ''    empty single-quoted string
	V12_EMPTY_STR_T   // ``    empty template-quoted string
	V12_REGEXP_DECL   // //    empty regexp initialiser

	// ---- Single-character operators and punctuation ----
	V12_PLUS      // +
	V12_MINUS     // -
	V12_STAR      // *
	V12_SLASH     // /   (division)
	V12_PERCENT   // %
	V12_TILDE     // ~   mutable assign base
	V12_COLON     // :   immutable assign base
	V12_EQ        // =
	V12_GT        // >
	V12_LT        // <
	V12_BANG      // !   logical not
	V12_AMP       // &
	V12_PIPE      // |
	V12_CARET     // ^
	V12_DOT       // .
	V12_COMMA     // ,
	V12_SEMICOLON // ;
	V12_LPAREN    // (
	V12_RPAREN    // )
	V12_LBRACKET  // [
	V12_RBRACKET  // ]
	V12_LBRACE    // {
	V12_RBRACE    // }
	V12_DOLLAR    // $   self-reference / JSONPath root marker
	V12_SECTION   // §   section marker
	V12_AT        // @   (standalone before @-ident is consumed)
	V12_QUESTION  // ?   JSONPath filter shorthand
	V12_ANY_TYPE  // @?  any_type — matches any value of any type
)

// V12tokenNames maps each V12TokenType to its display name.
var V12tokenNames = map[V12TokenType]string{
	V12_BOF: "BOF", V12_EOF: "EOF", V12_NL: "NL", V12_ILLEGAL: "ILLEGAL",
	V12_INTEGER: "INTEGER", V12_DECIMAL: "DECIMAL", V12_STRING: "STRING", V12_REGEXP: "REGEXP",
	V12_IDENT: "IDENT", V12_AT_IDENT: "AT_IDENT",
	V12_TRUE: "true", V12_FALSE: "false", V12_NULL: "null", V12_MANY: "many",
	V12_NAN: "NaN", V12_INFINITY: "Infinity",
	V12_MS: "ms", V12_SEC: "s", V12_MIN: "m", V12_HR: "h", V12_DAY: "d", V12_WK: "w",
	V12_UNIQUE: "UNIQUE", V12_RANGE_KW: "RANGE", V12_TYPE_OF: "TYPE_OF",
	V12_VALUE_OF: "VALUE_OF", V12_ADDRESS_OF: "ADDRESS_OF",
	V12_RETURN_DIR: "RETURN", V12_UNIFORM: "UNIFORM", V12_INFER: "INFER",
	V12_CAST: "CAST", V12_EXTEND: "EXTEND", V12_MERGE: "MERGE",
	V12_SQZ: "SQZ", V12_CODE: "CODE",
	V12_DOTDOT: "..", V12_DOTDOLLAR: ".$", V12_ARROW: "->", V12_STREAM: ">>",
	V12_STORE: "=>", V12_RETURN_STMT: "<-",
	V12_INC: "++", V12_DEC: "--", V12_POW: "**",
	V12_IADD_IMM: "+:", V12_ISUB_IMM: "-:", V12_IMUL_IMM: "*:", V12_IDIV_IMM: "/:",
	V12_IADD_MUT: "+~", V12_ISUB_MUT: "-~", V12_IMUL_MUT: "*~", V12_IDIV_MUT: "/~",
	V12_READONLY: ":~", V12_EXTEND_ASSIGN: "+=", V12_ISUB_ASSIGN: "-=", V12_IMUL_ASSIGN: "*=", V12_IDIV_ASSIGN: "/=",
	V12_GEQ: ">=", V12_LEQ: "<=", V12_NEQ: "!=", V12_EQEQ: "==", V12_MATCH_OP: "=~",
	V12_AMP_AMP: "&&", V12_PIPE_PIPE: "||",
	V12_EMPTY_ARR: "[]", V12_EMPTY_OBJ: "{}", V12_EMPTY_STR_D: `""`,
	V12_EMPTY_STR_S: "''", V12_EMPTY_STR_T: "``", V12_REGEXP_DECL: "//",
	V12_PLUS: "+", V12_MINUS: "-", V12_STAR: "*", V12_SLASH: "/", V12_PERCENT: "%",
	V12_TILDE: "~", V12_COLON: ":", V12_EQ: "=", V12_GT: ">", V12_LT: "<",
	V12_BANG: "!", V12_AMP: "&", V12_PIPE: "|", V12_CARET: "^",
	V12_DOT: ".", V12_COMMA: ",", V12_SEMICOLON: ";",
	V12_LPAREN: "(", V12_RPAREN: ")", V12_LBRACKET: "[",
	V12_RBRACKET: "]", V12_LBRACE: "{", V12_RBRACE: "}",
	V12_DOLLAR: "$", V12_SECTION: "§", V12_AT: "@", V12_QUESTION: "?",
	V12_ANY_TYPE: "@?",
}

// String returns the display name of a V12TokenType.
func (t V12TokenType) String() string {
	if s, ok := V12tokenNames[t]; ok {
		return s
	}
	return fmt.Sprintf("V12TOKEN(%d)", int(t))
}

// =============================================================================
// PHASE 1 — TOKEN (V12)
// =============================================================================

// V12Token represents a single lexeme produced by the V12 lexer.
type V12Token struct {
	Type  V12TokenType
	Value string
	Line  int
	Col   int
}

// String returns a compact, readable representation for diagnostics.
func (t V12Token) String() string {
	return fmt.Sprintf("V12Token{%s %q L%d:C%d}", t.Type, t.Value, t.Line, t.Col)
}

// =============================================================================
// PHASE 1 — LEXER (V12)
// =============================================================================

// V12keywords maps literal source strings to their corresponding V12TokenType.
var V12keywords = map[string]V12TokenType{
	"true":       V12_TRUE,
	"false":      V12_FALSE,
	"null":       V12_NULL,
	"many":       V12_MANY,
	"Many":       V12_MANY,
	"NaN":        V12_NAN,
	"Infinity":   V12_INFINITY,
	"ms":         V12_MS,
	"UNIQUE":     V12_UNIQUE,
	"RANGE":      V12_RANGE_KW,
	"TYPE_OF":    V12_TYPE_OF,
	"VALUE_OF":   V12_VALUE_OF,
	"ADDRESS_OF": V12_ADDRESS_OF,
	"RETURN":     V12_RETURN_DIR,
	"UNIFORM":    V12_UNIFORM,
	"INFER":      V12_INFER,
	"CAST":       V12_CAST,
	"EXTEND":     V12_EXTEND,
	"MERGE":      V12_MERGE,
	"SQZ":        V12_SQZ,
	"CODE":       V12_CODE,
}

// V12durationKeywords are single-character duration unit keywords.
// They only become duration-unit tokens when parsing is inside a duration context;
// elsewhere they are plain TOK_IDENT.  The lexer always checks here and emits
// the specialised type — the parser uses context to disambiguate.
var V12durationUnits = map[string]V12TokenType{
	"s": V12_SEC,
	"m": V12_MIN,
	"h": V12_HR,
	"d": V12_DAY,
	"w": V12_WK,
}

// V12prevIsValue returns true when tok is a "value-ending" token —
// i.e. the previous token closes a value expression, meaning a following `/`
// is division rather than the start of a regexp literal.
func V12prevIsValue(t V12TokenType) bool {
	switch t {
	case V12_INTEGER, V12_DECIMAL, V12_STRING, V12_REGEXP,
		V12_IDENT, V12_AT_IDENT, V12_TRUE, V12_FALSE, V12_NULL,
		V12_NAN, V12_INFINITY,
		V12_RPAREN, V12_RBRACKET, V12_RBRACE,
		V12_INC, V12_DEC,
		V12_EMPTY_STR_D, V12_EMPTY_STR_S, V12_EMPTY_STR_T,
		V12_EMPTY_ARR, V12_EMPTY_OBJ:
		return true
	}
	return false
}

// V12Lexer holds the mutable state of the V12 scanner.
type V12Lexer struct {
	input []rune
	pos   int
	line  int
	col   int
	last  V12TokenType // last significant (non-NL) token type for disambiguation
}

// NewV12Lexer constructs a fresh V12Lexer for the given source string.
func NewV12Lexer(src string) *V12Lexer {
	return &V12Lexer{
		input: []rune(src),
		pos:   0,
		line:  1,
		col:   0,
		last:  V12_BOF,
	}
}

// V12Tokenize scans the entire input and returns the full token slice.
// The slice always begins with V12_BOF and ends with V12_EOF.
// An error is returned if an illegal character is encountered; partial results
// up to that point are also returned.
func (l *V12Lexer) V12Tokenize() ([]V12Token, error) {
	tokens := []V12Token{l.V12makeTok(V12_BOF, "", 1, 0)}

	for {
		tok, err := l.V12scan()
		if err != nil {
			return tokens, err
		}
		if tok.Type != V12_NL {
			l.last = tok.Type
		}
		tokens = append(tokens, tok)
		if tok.Type == V12_EOF {
			break
		}
	}
	return tokens, nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

func (l *V12Lexer) V12peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *V12Lexer) V12peek2() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *V12Lexer) V12advance() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
	return ch
}

func (l *V12Lexer) V12skipHorizWS() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' {
			l.V12advance()
		} else {
			break
		}
	}
}

func (l *V12Lexer) V12makeTok(typ V12TokenType, val string, line, col int) V12Token {
	return V12Token{Type: typ, Value: val, Line: line, Col: col}
}

// --------------------------------------------------------------------------
// scan — top-level dispatch
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scan() (V12Token, error) {
	l.V12skipHorizWS()

	if l.pos >= len(l.input) {
		return l.V12makeTok(V12_EOF, "", l.line, l.col), nil
	}

	line, col := l.line, l.col
	ch := l.V12peek()

	// Newlines → NL token
	if ch == '\r' || ch == '\n' {
		return l.V12scanNL(line, col)
	}

	// (* …  *) nested comments — emitted as NL (whitespace-like)
	if ch == '(' && l.V12peek2() == '*' {
		return l.V12scanComment(line, col)
	}

	// Unicode letter or underscore → identifier or keyword
	if unicode.IsLetter(ch) || ch == '_' {
		return l.V12scanIdentOrKeyword(line, col), nil
	}

	// @identifier
	if ch == '@' {
		return l.V12scanAtIdent(line, col)
	}

	// Digit → number
	if unicode.IsDigit(ch) {
		return l.V12scanNumber(line, col), nil
	}

	// Quoted strings
	switch ch {
	case '"':
		return l.V12scanDoubleQuoted(line, col)
	case '\'':
		return l.V12scanSingleQuoted(line, col)
	case '`':
		return l.V12scanTemplateQuoted(line, col)
	}

	// Operators and punctuation
	return l.V12scanOperator(line, col)
}

// --------------------------------------------------------------------------
// Newline scanner  NL = /([ \t]*[\r\n]+)+/
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanNL(line, col int) (V12Token, error) {
	var sb strings.Builder

	for l.pos < len(l.input) {
		wsStart := l.pos
		for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t') {
			l.V12advance()
		}
		if l.pos >= len(l.input) || (l.input[l.pos] != '\r' && l.input[l.pos] != '\n') {
			l.pos = wsStart
			break
		}
		for i := wsStart; i < l.pos; i++ {
			sb.WriteRune(l.input[i])
		}
		consumed := false
		for l.pos < len(l.input) && (l.input[l.pos] == '\r' || l.input[l.pos] == '\n') {
			ch := l.V12advance()
			sb.WriteRune(ch)
			if ch == '\r' && l.pos < len(l.input) && l.input[l.pos] == '\n' {
				sb.WriteRune(l.V12advance())
			}
			consumed = true
		}
		if !consumed {
			break
		}
	}

	return l.V12makeTok(V12_NL, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Nested comment scanner  comment = "(*"  comment_txt { comment }  "*)"
// Comments are treated as whitespace (emitted as V12_NL).
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanComment(line, col int) (V12Token, error) {
	depth := 0
	var sb strings.Builder

	for l.pos < len(l.input) {
		ch := l.V12peek()
		ch2 := l.V12peek2()

		if ch == '(' && ch2 == '*' {
			depth++
			sb.WriteRune(l.V12advance())
			sb.WriteRune(l.V12advance())
			continue
		}
		if ch == '*' && ch2 == ')' {
			sb.WriteRune(l.V12advance())
			sb.WriteRune(l.V12advance())
			depth--
			if depth == 0 {
				return l.V12makeTok(V12_NL, sb.String(), line, col), nil
			}
			continue
		}
		sb.WriteRune(l.V12advance())
	}

	return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated comment", line, col)
}

// --------------------------------------------------------------------------
// Identifier / keyword scanner
// Grammar: ident_name = /(?<value>[\p{L}][\p{L}0-9_]*)/
// Spaces are NOT part of an identifier (V11 and later).
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanIdentOrKeyword(line, col int) V12Token {
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.V12advance())
		} else {
			break
		}
	}
	value := sb.String()

	// Check explicit keyword map first.
	if tt, ok := V12keywords[value]; ok {
		return l.V12makeTok(tt, value, line, col)
	}
	// Single-character duration unit keywords (s m h d w) — emitted as specialised types
	// so the parser can recognise them in a duration context.
	if tt, ok := V12durationUnits[value]; ok {
		return l.V12makeTok(tt, value, line, col)
	}
	return l.V12makeTok(V12_IDENT, value, line, col)
}

// --------------------------------------------------------------------------
// @-prefixed identifier scanner  @name  @type  …
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanAtIdent(line, col int) (V12Token, error) {
	l.V12advance() // consume '@'
	// @? → any_type token.
	if l.pos < len(l.input) && l.input[l.pos] == '?' {
		l.V12advance()
		return l.V12makeTok(V12_ANY_TYPE, "@?", line, col), nil
	}
	// Standalone '@' (e.g. JSONPath current-node '@') — emit V12_AT.
	if l.pos >= len(l.input) || !unicode.IsLetter(l.input[l.pos]) {
		return l.V12makeTok(V12_AT, "@", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('@')
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.V12advance())
		} else {
			break
		}
	}
	return l.V12makeTok(V12_AT_IDENT, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Number scanner  →  V12_INTEGER | V12_DECIMAL
// The lexer emits unsigned digit sequences; signs (+/-) are separate tokens.
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanNumber(line, col int) V12Token {
	var sb strings.Builder
	for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
		sb.WriteRune(l.V12advance())
	}
	// Decimal: digits "." digits — but NOT the ".." range operator.
	if l.pos < len(l.input) && l.input[l.pos] == '.' &&
		l.pos+1 < len(l.input) && l.input[l.pos+1] != '.' {
		sb.WriteRune(l.V12advance()) // '.'
		for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
			sb.WriteRune(l.V12advance())
		}
		return l.V12makeTok(V12_DECIMAL, sb.String(), line, col)
	}
	return l.V12makeTok(V12_INTEGER, sb.String(), line, col)
}

// --------------------------------------------------------------------------
// String scanners
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanDoubleQuoted(line, col int) (V12Token, error) {
	l.V12advance() // consume opening "
	if l.V12peek() == '"' {
		l.V12advance()
		return l.V12makeTok(V12_EMPTY_STR_D, `""`, line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('"')
	for l.pos < len(l.input) {
		ch := l.V12advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.V12advance())
		case ch == '"':
			return l.V12makeTok(V12_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
		}
	}
	return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
}

func (l *V12Lexer) V12scanSingleQuoted(line, col int) (V12Token, error) {
	l.V12advance() // consume opening '
	if l.V12peek() == '\'' {
		l.V12advance()
		return l.V12makeTok(V12_EMPTY_STR_S, "''", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('\'')
	for l.pos < len(l.input) {
		ch := l.V12advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.V12advance())
		case ch == '\'':
			return l.V12makeTok(V12_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
		}
	}
	return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
}

// V12scanTemplateQuoted handles `…` strings with optional $(...) interpolation.
// Grammar: tmpl_quoted = "`" ( tmpl_text | tmpl_expr ) { ( tmpl_text | tmpl_expr ) } "`"
// The lexer emits the entire template literal as a single V12_STRING token,
// preserving all interpolation syntax verbatim.  The parser reconstructs the
// tmpl_expr segments from the raw value when building AST nodes.
func (l *V12Lexer) V12scanTemplateQuoted(line, col int) (V12Token, error) {
	l.V12advance() // consume opening `
	if l.V12peek() == '`' {
		l.V12advance()
		return l.V12makeTok(V12_EMPTY_STR_T, "``", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('`')
	depth := 0 // nesting depth inside $( ... )
	for l.pos < len(l.input) {
		ch := l.V12peek()
		ch2 := l.V12peek2()

		if ch == '$' && ch2 == '(' && depth == 0 {
			sb.WriteRune(l.V12advance()) // $
			sb.WriteRune(l.V12advance()) // (
			depth++
			continue
		}
		if ch == '(' && depth > 0 {
			depth++
			sb.WriteRune(l.V12advance())
			continue
		}
		if ch == ')' && depth > 0 {
			depth--
			sb.WriteRune(l.V12advance())
			continue
		}
		if ch == '`' && depth == 0 {
			sb.WriteRune(l.V12advance())
			return l.V12makeTok(V12_STRING, sb.String(), line, col), nil
		}
		if ch == 0 {
			return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated template string", line, col)
		}
		sb.WriteRune(l.V12advance())
	}
	return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated template string", line, col)
}

// --------------------------------------------------------------------------
// Regexp scanner
// grammar: regexp = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags { regexp_flags } ]
// Called when '/' is encountered and context indicates a regexp, not division.
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanRegexp(line, col int) (V12Token, error) {
	l.V12advance() // consume opening /
	var sb strings.Builder
	sb.WriteRune('/')

	for l.pos < len(l.input) {
		ch := l.V12advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.V12advance())
		case ch == '/':
			// Consume optional flags: g i m s u y x n A
			for l.pos < len(l.input) {
				f := l.input[l.pos]
				if f == 'g' || f == 'i' || f == 'm' || f == 's' ||
					f == 'u' || f == 'y' || f == 'x' || f == 'n' || f == 'A' {
					sb.WriteRune(l.V12advance())
				} else {
					break
				}
			}
			return l.V12makeTok(V12_REGEXP, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
		}
	}
	return l.V12makeTok(V12_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
}

// --------------------------------------------------------------------------
// Operator scanner — max-munch, all single and multi-character operators.
// --------------------------------------------------------------------------

func (l *V12Lexer) V12scanOperator(line, col int) (V12Token, error) {
	ch := l.V12peek()
	ch2 := l.V12peek2()

	switch ch {

	// . .. .$
	case '.':
		if ch2 == '.' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_DOTDOT, "..", line, col), nil
		}
		if ch2 == '$' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_DOTDOLLAR, ".$", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_DOT, ".", line, col), nil

	// - -> -- -: -~
	case '-':
		switch ch2 {
		case '>':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_ARROW, "->", line, col), nil
		case '-':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_DEC, "--", line, col), nil
		case ':':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_ISUB_IMM, "-:", line, col), nil
		case '~':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_ISUB_MUT, "-~", line, col), nil
		case '=':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_ISUB_ASSIGN, "-=", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_MINUS, "-", line, col), nil

	// > >> >=
	case '>':
		switch ch2 {
		case '>':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_STREAM, ">>", line, col), nil
		case '=':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_GEQ, ">=", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_GT, ">", line, col), nil

	// = => =~
	case '=':
		switch ch2 {
		case '>':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_STORE, "=>", line, col), nil
		case '~':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_MATCH_OP, "=~", line, col), nil
		case '=':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_EQEQ, "==", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_EQ, "=", line, col), nil

	// < <- <=
	case '<':
		switch ch2 {
		case '-':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_RETURN_STMT, "<-", line, col), nil
		case '=':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_LEQ, "<=", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_LT, "<", line, col), nil

	// + ++ +: +~ +=
	case '+':
		switch ch2 {
		case '+':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_INC, "++", line, col), nil
		case ':':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IADD_IMM, "+:", line, col), nil
		case '~':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IADD_MUT, "+~", line, col), nil
		case '=':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_EXTEND_ASSIGN, "+=", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_PLUS, "+", line, col), nil

	// * ** *: *~
	case '*':
		switch ch2 {
		case '*':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_POW, "**", line, col), nil
		case ':':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IMUL_IMM, "*:", line, col), nil
		case '~':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IMUL_MUT, "*~", line, col), nil
		case '=':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IMUL_ASSIGN, "*=", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_STAR, "*", line, col), nil

	// / // /: /~  or regexp /…/flags
	case '/':
		switch ch2 {
		case '/':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_REGEXP_DECL, "//", line, col), nil
		case ':':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IDIV_IMM, "/:", line, col), nil
		case '~':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IDIV_MUT, "/~", line, col), nil
		case '=':
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_IDIV_ASSIGN, "/=", line, col), nil
		}
		if V12prevIsValue(l.last) {
			l.V12advance()
			return l.V12makeTok(V12_SLASH, "/", line, col), nil
		}
		return l.V12scanRegexp(line, col)

	// : :~
	case ':':
		if ch2 == '~' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_READONLY, ":~", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_COLON, ":", line, col), nil

	// ! !=
	case '!':
		if ch2 == '=' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_NEQ, "!=", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_BANG, "!", line, col), nil

	// & &&
	case '&':
		if ch2 == '&' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_AMP_AMP, "&&", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_AMP, "&", line, col), nil

	// | ||
	case '|':
		if ch2 == '|' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_PIPE_PIPE, "||", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_PIPE, "|", line, col), nil

	// [ []
	case '[':
		if ch2 == ']' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_EMPTY_ARR, "[]", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_LBRACKET, "[", line, col), nil

	// { {}
	case '{':
		if ch2 == '}' {
			l.V12advance()
			l.V12advance()
			return l.V12makeTok(V12_EMPTY_OBJ, "{}", line, col), nil
		}
		l.V12advance()
		return l.V12makeTok(V12_LBRACE, "{", line, col), nil

	// $ dollar — self-reference / JSONPath root
	case '$':
		l.V12advance()
		return l.V12makeTok(V12_DOLLAR, "$", line, col), nil

	// § section marker (U+00A7)
	case '§':
		l.V12advance()
		return l.V12makeTok(V12_SECTION, "§", line, col), nil

	// ? question mark
	case '?':
		l.V12advance()
		return l.V12makeTok(V12_QUESTION, "?", line, col), nil

	// Single-character punctuation
	case '~':
		l.V12advance()
		return l.V12makeTok(V12_TILDE, "~", line, col), nil
	case '%':
		l.V12advance()
		return l.V12makeTok(V12_PERCENT, "%", line, col), nil
	case '^':
		l.V12advance()
		return l.V12makeTok(V12_CARET, "^", line, col), nil
	case ',':
		l.V12advance()
		return l.V12makeTok(V12_COMMA, ",", line, col), nil
	case ';':
		l.V12advance()
		return l.V12makeTok(V12_SEMICOLON, ";", line, col), nil
	case '(':
		l.V12advance()
		return l.V12makeTok(V12_LPAREN, "(", line, col), nil
	case ')':
		l.V12advance()
		return l.V12makeTok(V12_RPAREN, ")", line, col), nil
	case ']':
		l.V12advance()
		return l.V12makeTok(V12_RBRACKET, "]", line, col), nil
	case '}':
		l.V12advance()
		return l.V12makeTok(V12_RBRACE, "}", line, col), nil
	}

	bad := l.V12advance()
	return l.V12makeTok(V12_ILLEGAL, string(bad), line, col),
		fmt.Errorf("L%d:C%d: unexpected character %q", line, col, bad)
}

// =============================================================================
// PHASE 2 — AST NODE TYPES  (01_definitions.sqg)
// =============================================================================

// V12Node is the common interface for all AST nodes.
type V12Node interface {
	V12NodePos() (line, col int)
}

// V12BaseNode carries source position, embedded in every AST node.
type V12BaseNode struct {
	Line int
	Col  int
}

func (n V12BaseNode) V12NodePos() (int, int) { return n.Line, n.Col }

// ---------- Numeric primitives ----------

// V12SignPrefixNode  sign_prefix = [ "+" | "-" ]
// Sign is "", "+", or "-"; empty means the prefix was absent.
type V12SignPrefixNode struct {
	V12BaseNode
	Sign string // "", "+", or "-"
}

// V12IntegerNode  integer = sign_prefix digits
type V12IntegerNode struct {
	V12BaseNode
	SignPrefix *V12SignPrefixNode
	Digits     string // raw digit string
}

// V12DecimalNode  decimal = sign_prefix digits "." digits
type V12DecimalNode struct {
	V12BaseNode
	SignPrefix *V12SignPrefixNode
	Integral   string // digits before "."
	Frac       string // digits after "."
}

// V12NumericConstNode  numeric_const = integer | decimal
type V12NumericConstNode struct {
	V12BaseNode
	Value V12Node // *V12IntegerNode | *V12DecimalNode
}

// ---------- Typed integer / unsigned integer nodes ----------

// V12IntTypeNode represents a range-constrained signed integer (int8..int128)
// or unsigned integer (byte, uint8..uint128).
// The Kind field carries the type name ("int8", "uint32", etc.).
type V12IntTypeNode struct {
	V12BaseNode
	Kind  string // "int8", "byte", "uint64", …
	Value *V12IntegerNode
}

// ---------- Float nodes ----------

// V12NanNode  nan = "NaN"
type V12NanNode struct{ V12BaseNode }

// V12InfinityNode  infinity = "+Infinity" | "-Infinity" | "Infinity"
type V12InfinityNode struct {
	V12BaseNode
	Sign string // "", "+", or "-"
}

// V12FloatNode  float32 | float64  (TYPE_OF self-referential)
// At parse time this is just a wrapper around decimal | nan | infinity.
type V12FloatNode struct {
	V12BaseNode
	Kind  string  // "float32" or "float64"
	Value V12Node // *V12DecimalNode | *V12NanNode | *V12InfinityNode
}

// ---------- Decimal number types ----------

// V12DecimalTypeNode  decimalN = TYPE_OF intN<integer> "." digits
type V12DecimalTypeNode struct {
	V12BaseNode
	Kind     string // "decimal8", "decimal16", …
	Integral *V12IntegerNode
	Frac     string // raw fractional digits
}

// V12DecimalNumNode  decimal_num = decimal8 | … | decimal128
type V12DecimalNumNode struct {
	V12BaseNode
	Value *V12DecimalTypeNode
}

// ---------- CAST directive record ----------

// V12CastDirective records a CAST<T> = A | B | … widening chain.
// Not an AST value node — stored in V12Parser.CastChains.
type V12CastDirective struct {
	Source  string   // e.g. "int8"
	Targets []string // e.g. ["int16","int32","int64","int128"]
}

// ---------- String nodes ----------

// StringKind distinguishes the three quoted string varieties.
type V12StringKind int

const (
	V12StringSingle   V12StringKind = iota // '…'
	V12StringDouble                        // "…"
	V12StringTemplate                      // `…` (may contain $(...) interpolations)
)

// V12StringNode  single_quoted | double_quoted | tmpl_quoted
type V12StringNode struct {
	V12BaseNode
	Kind  V12StringKind
	Raw   string        // verbatim source text including delimiters
	Parts []V12TmplPart // non-nil only for template strings
}

// V12StringQuotedNode  string_quoted = single_quoted | double_quoted
// A non-template string — guaranteed to be single- or double-quoted.
type V12StringQuotedNode struct {
	V12BaseNode
	Value *V12StringNode // Kind is always V12StringSingle or V12StringDouble
}

// V12TmplPart is one segment of a template string:
// either literal text or a $(expr) interpolation.
type V12TmplPart struct {
	IsExpr bool   // false → literal text, true → $(...) expression
	Text   string // raw text (when !IsExpr) or inner expression source (when IsExpr)
}

// V12CharNode  char = "'" /(?<value>[\s\S])/ "'"
type V12CharNode struct {
	V12BaseNode
	Value rune
}

// ---------- Date / Time nodes ----------

// V12DateNode  date = date_year [ ["-"] date_month [ ["-"] date_day ] ]
type V12DateNode struct {
	V12BaseNode
	Year  string
	Month string // empty if omitted
	Day   string // empty if omitted
}

// V12TimeNode  time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
type V12TimeNode struct {
	V12BaseNode
	Hour   string
	Minute string
	Second string
	Millis string
}

// V12DateTimeNode  date_time = ( date [ [" "] time ] ) | time
type V12DateTimeNode struct {
	V12BaseNode
	Date *V12DateNode // nil if time-only
	Time *V12TimeNode // nil if date-only
}

// V12TimeStampNode  time_stamp = date [" "] time
type V12TimeStampNode struct {
	V12BaseNode
	Date *V12DateNode
	Time *V12TimeNode
}

// ---------- Duration ----------

// V12DurationSegment is one "quantity unit" pair, e.g. "30" "ms"
type V12DurationSegment struct {
	Digits string
	Unit   string
}

// V12DurationNode  duration = digits duration_unit { digits duration_unit }
type V12DurationNode struct {
	V12BaseNode
	Segments []V12DurationSegment
}

// ---------- Regexp ----------

// V12RegexpNode  regexp = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags ]
type V12RegexpNode struct {
	V12BaseNode
	Pattern string // everything between the slashes
	Flags   string // zero or more flag characters
}

// ---------- Boolean / null ----------

// V12BoolNode  boolean = "true" | "false"
type V12BoolNode struct {
	V12BaseNode
	Value bool
}

// V12NullNode  null = "null"
type V12NullNode struct{ V12BaseNode }

// V12AnyTypeNode  any_type = "@?"
// Matches a value of any type — used as a wildcard type annotation.
type V12AnyTypeNode struct{ V12BaseNode }

// ---------- Cardinality / range ----------

// V12CardinalityNode  cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
type V12CardinalityNode struct {
	V12BaseNode
	Lo string
	Hi string // raw upper bound token value
}

// V12RangeNode  range = integer ".." integer
type V12RangeNode struct {
	V12BaseNode
	Lo *V12IntegerNode
	Hi *V12IntegerNode
}

// ---------- UUID ----------

// V12UUIDNode  uuid  (standard 8-4-4-4-12)
type V12UUIDNode struct {
	V12BaseNode
	Value string // e.g. "550e8400-e29b-41d4-a716-446655440000"
}

// V12UUIDV7Node  uuid_v7
type V12UUIDV7Node struct {
	V12BaseNode
	Value string
}

// ---------- URL / file path ----------

// V12URLNode  http_url
type V12URLNode struct {
	V12BaseNode
	Value string
}

// V12FilePathNode  file_path
type V12FilePathNode struct {
	V12BaseNode
	Value string
}

// ---------- Top-level constant ----------

// V12ConstantNode  constant = numeric_const | string | char | regexp | boolean | null |
//
//	date | time | date_time | time_stamp | uuid | http_url | file_path
type V12ConstantNode struct {
	V12BaseNode
	Value V12Node
}

// =============================================================================
// PHASE 3 — RECURSIVE-DESCENT PARSER  (01_definitions.sqg)
// =============================================================================

// V12ParseError records a parse failure with location information.
type V12ParseError struct {
	Line    int
	Col     int
	Message string
}

func (e *V12ParseError) Error() string {
	return fmt.Sprintf("L%d:C%d: %s", e.Line, e.Col, e.Message)
}

// V12Parser holds the parser state: the token stream and the current position.
type V12Parser struct {
	tokens []V12Token
	pos    int
	// CastChains stores every CAST<T> = … directive encountered during parsing.
	CastChains []V12CastDirective
}

// NewV12Parser creates a V12Parser from an already-tokenised slice.
// The caller should have run V12Lexer.V12Tokenize() first.
func NewV12Parser(tokens []V12Token) *V12Parser {
	return &V12Parser{tokens: tokens, pos: 0}
}

// ParseV12FromSource is a convenience wrapper: lex + parse in one call.
func ParseV12FromSource(src string) (*V12Parser, error) {
	lex := NewV12Lexer(src)
	toks, err := lex.V12Tokenize()
	if err != nil {
		return nil, err
	}
	return NewV12Parser(toks), nil
}

// --------------------------------------------------------------------------
// Parser helpers
// --------------------------------------------------------------------------

// cur returns the current token (skipping NL and BOF tokens).
func (p *V12Parser) cur() V12Token {
	i := p.pos
	for i < len(p.tokens) && (p.tokens[i].Type == V12_NL || p.tokens[i].Type == V12_BOF) {
		i++
	}
	if i >= len(p.tokens) {
		return V12Token{Type: V12_EOF}
	}
	return p.tokens[i]
}

// curRaw returns the current token WITHOUT skipping NL.
func (p *V12Parser) curRaw() V12Token {
	if p.pos >= len(p.tokens) {
		return V12Token{Type: V12_EOF}
	}
	return p.tokens[p.pos]
}

// advance moves past the current token (NL/BOF-aware: skips leading NL/BOF tokens first).
func (p *V12Parser) advance() {
	// Skip any leading NL/BOF tokens.
	for p.pos < len(p.tokens) && (p.tokens[p.pos].Type == V12_NL || p.tokens[p.pos].Type == V12_BOF) {
		p.pos++
	}
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// advanceRaw moves past exactly one token regardless of type.
func (p *V12Parser) advanceRaw() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// skipNL advances past all NL tokens at the current position.
func (p *V12Parser) skipNL() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == V12_NL {
		p.pos++
	}
}

// peek returns the token n positions ahead (NL-transparent).
func (p *V12Parser) peek(n int) V12Token {
	count := 0
	i := p.pos
	for i < len(p.tokens) {
		if p.tokens[i].Type != V12_NL {
			if count == n {
				return p.tokens[i]
			}
			count++
		}
		i++
	}
	return V12Token{Type: V12_EOF}
}

// expect consumes the current token if it matches typ; otherwise returns an error.
func (p *V12Parser) expect(typ V12TokenType) (V12Token, error) {
	tok := p.cur()
	if tok.Type != typ {
		return tok, &V12ParseError{
			Line:    tok.Line,
			Col:     tok.Col,
			Message: fmt.Sprintf("expected %s, got %s %q", typ, tok.Type, tok.Value),
		}
	}
	p.advance()
	return tok, nil
}

// expectValue consumes a V12_STRING or V12_INTEGER or V12_DECIMAL token if the
// current token's Value field matches the literal str.
func (p *V12Parser) expectLit(str string) (V12Token, error) {
	tok := p.cur()
	if tok.Value != str {
		return tok, &V12ParseError{
			Line:    tok.Line,
			Col:     tok.Col,
			Message: fmt.Sprintf("expected %q, got %s %q", str, tok.Type, tok.Value),
		}
	}
	p.advance()
	return tok, nil
}

// errAt builds a V12ParseError at the current token's location.
func (p *V12Parser) errAt(msg string) *V12ParseError {
	tok := p.cur()
	return &V12ParseError{Line: tok.Line, Col: tok.Col, Message: msg}
}

// --------------------------------------------------------------------------
// Numeric primitive parsers
// --------------------------------------------------------------------------

// ParseDigits parses the next token as an unsigned digit sequence.
// The lexer always emits unsigned values; this returns the raw string.
func (p *V12Parser) ParseDigits() (string, error) {
	tok := p.cur()
	if tok.Type != V12_INTEGER {
		return "", p.errAt(fmt.Sprintf("expected digits (INTEGER), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return tok.Value, nil
}

// ParseSignPrefix parses:  sign_prefix = [ "+" | "-" ]
// Always succeeds — returns an empty Sign when no sign token is present.
func (p *V12Parser) ParseSignPrefix() *V12SignPrefixNode {
	tok := p.cur()
	node := &V12SignPrefixNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}}
	if tok.Type == V12_PLUS {
		node.Sign = "+"
		p.advance()
	} else if tok.Type == V12_MINUS {
		node.Sign = "-"
		p.advance()
	}
	return node
}

// ParseInteger parses:  integer = sign_prefix digits
func (p *V12Parser) ParseInteger() (*V12IntegerNode, error) {
	line, col := p.cur().Line, p.cur().Col
	sp := p.ParseSignPrefix()
	digitTok, err := p.expect(V12_INTEGER)
	if err != nil {
		return nil, err
	}
	return &V12IntegerNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		SignPrefix:  sp,
		Digits:      digitTok.Value,
	}, nil
}

// ParseDecimal parses:  decimal = sign_prefix digits "." digits
// The lexer emits the entire "3.14" as a single V12_DECIMAL token, so we
// split it here for the AST.
func (p *V12Parser) ParseDecimal() (*V12DecimalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	sp := p.ParseSignPrefix()
	tok := p.cur()
	if tok.Type != V12_DECIMAL {
		return nil, p.errAt(fmt.Sprintf("expected decimal literal, got %s %q", tok.Type, tok.Value))
	}
	p.advance()

	// Split "NNN.FFF" on the first '.'.
	parts := strings.SplitN(tok.Value, ".", 2)
	integral := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	return &V12DecimalNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		SignPrefix:  sp,
		Integral:    integral,
		Frac:        frac,
	}, nil
}

// ParseNumericConst parses:  numeric_const = integer | decimal
// Looks ahead: if a decimal token is present (with optional sign) use ParseDecimal,
// otherwise use ParseInteger.
func (p *V12Parser) ParseNumericConst() (*V12NumericConstNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Determine whether the upcoming value is a decimal.
	isDecSign := tok.Type == V12_PLUS || tok.Type == V12_MINUS
	nextTok := p.peek(1)
	if isDecSign {
		if nextTok.Type == V12_DECIMAL {
			d, err := p.ParseDecimal()
			if err != nil {
				return nil, err
			}
			return &V12NumericConstNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: d}, nil
		}
		// Fall through to integer
		i, err := p.ParseInteger()
		if err != nil {
			return nil, err
		}
		return &V12NumericConstNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: i}, nil
	}

	if tok.Type == V12_DECIMAL {
		d, err := p.ParseDecimal()
		if err != nil {
			return nil, err
		}
		return &V12NumericConstNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: d}, nil
	}

	i, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	return &V12NumericConstNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: i}, nil
}

// --------------------------------------------------------------------------
// NaN / Infinity parsers
// --------------------------------------------------------------------------

// ParseNan parses:  nan = "NaN"
func (p *V12Parser) ParseNan() (*V12NanNode, error) {
	tok := p.cur()
	if tok.Type != V12_NAN {
		return nil, p.errAt(fmt.Sprintf("expected NaN, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V12NanNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// ParseInfinity parses:  infinity = "+Infinity" | "-Infinity" | "Infinity"
func (p *V12Parser) ParseInfinity() (*V12InfinityNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	sign := ""

	if tok.Type == V12_PLUS {
		sign = "+"
		p.advance()
		tok = p.cur()
	} else if tok.Type == V12_MINUS {
		sign = "-"
		p.advance()
		tok = p.cur()
	}

	if tok.Type != V12_INFINITY {
		return nil, p.errAt(fmt.Sprintf("expected Infinity, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V12InfinityNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Sign: sign}, nil
}

// --------------------------------------------------------------------------
// CAST directive parser
// Grammar:  CAST<T> = A | B | … ;
// This is a grammar-level directive, not a source-level expression.
// --------------------------------------------------------------------------

// ParseCastDirective parses:  CAST "<" ident ">" "=" ident { "|" ident } ";"
// and appends the result to p.CastChains.
func (p *V12Parser) ParseCastDirective() (*V12CastDirective, error) {
	if _, err := p.expect(V12_CAST); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_LT); err != nil {
		return nil, err
	}
	srcTok, err := p.expect(V12_IDENT)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_EQ); err != nil {
		return nil, err
	}

	var targets []string
	firstTok, err := p.expect(V12_IDENT)
	if err != nil {
		return nil, err
	}
	targets = append(targets, firstTok.Value)

	for p.cur().Type == V12_PIPE {
		p.advance()
		t, err := p.expect(V12_IDENT)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t.Value)
	}

	// Optional EOL / semicolon
	if p.cur().Type == V12_SEMICOLON {
		p.advance()
	}

	cd := &V12CastDirective{Source: srcTok.Value, Targets: targets}
	p.CastChains = append(p.CastChains, *cd)
	return cd, nil
}

// --------------------------------------------------------------------------
// String / char parsers
// --------------------------------------------------------------------------

// ParseString parses:  string = single_quoted | double_quoted | tmpl_quoted
// The raw token value includes delimiters.  For template strings the Parts
// slice is populated by splitting on $( … ) segments.
func (p *V12Parser) ParseString() (*V12StringNode, error) {
	tok := p.cur()
	if tok.Type != V12_STRING && tok.Type != V12_EMPTY_STR_D &&
		tok.Type != V12_EMPTY_STR_S && tok.Type != V12_EMPTY_STR_T {
		return nil, p.errAt(fmt.Sprintf("expected string literal, got %s %q", tok.Type, tok.Value))
	}

	line, col := tok.Line, tok.Col
	p.advance()

	raw := tok.Value
	var kind V12StringKind
	var parts []V12TmplPart

	switch {
	case strings.HasPrefix(raw, "'"):
		kind = V12StringSingle
	case strings.HasPrefix(raw, "\""):
		kind = V12StringDouble
	case strings.HasPrefix(raw, "`"):
		kind = V12StringTemplate
		parts = v12splitTemplateParts(raw)
	default:
		kind = V12StringDouble
	}

	return &V12StringNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Kind:        kind,
		Raw:         raw,
		Parts:       parts,
	}, nil
}

// ParseStringQuoted parses:  string_quoted = single_quoted | double_quoted
// Template strings are explicitly rejected — use ParseString for those.
func (p *V12Parser) ParseStringQuoted() (*V12StringQuotedNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Only single- or double-quoted tokens qualify.
	isQuoted := (tok.Type == V12_STRING && !strings.HasPrefix(tok.Value, "`")) ||
		tok.Type == V12_EMPTY_STR_S || tok.Type == V12_EMPTY_STR_D
	if !isQuoted {
		return nil, p.errAt(fmt.Sprintf("expected single- or double-quoted string, got %s %q", tok.Type, tok.Value))
	}

	inner, err := p.ParseString()
	if err != nil {
		return nil, err
	}
	if inner.Kind == V12StringTemplate {
		return nil, &V12ParseError{Line: line, Col: col, Message: "string_quoted: template string not allowed here"}
	}
	return &V12StringQuotedNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: inner}, nil
}

// v12splitTemplateParts splits the raw template string (including backtick delimiters)
// into literal-text and $(...) interpolation segments.
func v12splitTemplateParts(raw string) []V12TmplPart {
	// Strip surrounding backticks.
	inner := raw
	if len(inner) >= 2 && inner[0] == '`' {
		inner = inner[1:]
	}
	if len(inner) > 0 && inner[len(inner)-1] == '`' {
		inner = inner[:len(inner)-1]
	}

	var parts []V12TmplPart
	for len(inner) > 0 {
		idx := strings.Index(inner, "$(")
		if idx == -1 {
			parts = append(parts, V12TmplPart{IsExpr: false, Text: inner})
			break
		}
		if idx > 0 {
			parts = append(parts, V12TmplPart{IsExpr: false, Text: inner[:idx]})
		}
		// Find matching close paren.
		depth := 1
		i := idx + 2
		for i < len(inner) && depth > 0 {
			if inner[i] == '(' {
				depth++
			} else if inner[i] == ')' {
				depth--
			}
			i++
		}
		exprInner := inner[idx+2 : i-1]
		parts = append(parts, V12TmplPart{IsExpr: true, Text: exprInner})
		inner = inner[i:]
	}
	return parts
}

// ParseChar parses:  char = "'" /(?<value>[\s\S])/ "'"
// The lexer emits single-quoted strings as V12_STRING.  We validate that the
// content is exactly one Unicode code point.
func (p *V12Parser) ParseChar() (*V12CharNode, error) {
	tok := p.cur()
	if tok.Type != V12_STRING || !strings.HasPrefix(tok.Value, "'") {
		return nil, p.errAt(fmt.Sprintf("expected single-quoted char, got %s %q", tok.Type, tok.Value))
	}
	line, col := tok.Line, tok.Col
	p.advance()

	// Strip surrounding single quotes.
	inner := tok.Value[1 : len(tok.Value)-1]
	// Handle escape: \'  →  '
	if inner == "\\'" {
		return &V12CharNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: '\''}, nil
	}
	runes := []rune(inner)
	if len(runes) != 1 {
		return nil, &V12ParseError{Line: line, Col: col,
			Message: fmt.Sprintf("char literal must be exactly one code point, got %q", inner)}
	}
	return &V12CharNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: runes[0]}, nil
}

// --------------------------------------------------------------------------
// Date / Time parsers
// --------------------------------------------------------------------------

// V12isDigitSeq returns true if the string consists entirely of ASCII digits.
func V12isDigitSeq(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ParseDate parses:  date = date_year [ ["-"] date_month [ ["-"] date_day ] ]
// date_year  = /[0-9]{4}/
// date_month = RANGE 1..12<digits2>
// date_day   = RANGE 1..31<digits2>
// The lexer emits digit sequences as V12_INTEGER tokens.
func (p *V12Parser) ParseDate() (*V12DateNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Year must be exactly 4 digits.
	if tok.Type != V12_INTEGER || len(tok.Value) != 4 || !V12isDigitSeq(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected 4-digit year, got %s %q", tok.Type, tok.Value))
	}
	year := tok.Value
	p.advance()

	node := &V12DateNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Year: year}

	// Optional month (preceded by optional "-")
	if p.cur().Type == V12_MINUS {
		p.advance()
	}
	nextTok := p.cur()
	if !V12isDateMonthOrDay(nextTok) {
		return node, nil
	}
	node.Month = nextTok.Value
	p.advance()

	// Optional day
	if p.cur().Type == V12_MINUS {
		p.advance()
	}
	nextTok = p.cur()
	if !V12isDateMonthOrDay(nextTok) {
		return node, nil
	}
	node.Day = nextTok.Value
	p.advance()

	return node, nil
}

// V12isDateMonthOrDay returns true if tok looks like a 1-2 digit number in range 1..31.
func V12isDateMonthOrDay(tok V12Token) bool {
	if tok.Type != V12_INTEGER {
		return false
	}
	if len(tok.Value) > 2 {
		return false
	}
	return V12isDigitSeq(tok.Value)
}

// ParseTime parses:  time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
func (p *V12Parser) ParseTime() (*V12TimeNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V12_INTEGER || len(tok.Value) > 2 || !V12isDigitSeq(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected hour digits, got %s %q", tok.Type, tok.Value))
	}
	hour := tok.Value
	p.advance()

	node := &V12TimeNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Hour: hour}

	// Optional minute
	if p.cur().Type == V12_COLON {
		p.advance()
	}
	if p.cur().Type != V12_INTEGER || len(p.cur().Value) > 2 {
		return node, nil
	}
	node.Minute = p.cur().Value
	p.advance()

	// Optional second
	if p.cur().Type == V12_COLON {
		p.advance()
	}
	if p.cur().Type != V12_INTEGER || len(p.cur().Value) > 2 {
		return node, nil
	}
	node.Second = p.cur().Value
	p.advance()

	// Optional millis
	if p.cur().Type == V12_DOT {
		p.advance()
	}
	if p.cur().Type != V12_INTEGER || len(p.cur().Value) > 3 {
		return node, nil
	}
	node.Millis = p.cur().Value
	p.advance()

	return node, nil
}

// ParseDateTime parses:  date_time = ( date [ [" "] time ] ) | time
// We try date first; if a time follows (or the whole thing looks time-like), emit accordingly.
func (p *V12Parser) ParseDateTime() (*V12DateTimeNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	node := &V12DateTimeNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}

	// A 4-digit integer is unambiguously a year; otherwise assume time.
	if tok.Type == V12_INTEGER && len(tok.Value) == 4 {
		date, err := p.ParseDate()
		if err != nil {
			return nil, err
		}
		node.Date = date
		// Optional time component following the date.
		if p.cur().Type == V12_INTEGER {
			time, err := p.ParseTime()
			if err != nil {
				return nil, err
			}
			node.Time = time
		}
		return node, nil
	}

	// Time-only path.
	t, err := p.ParseTime()
	if err != nil {
		return nil, err
	}
	node.Time = t
	return node, nil
}

// ParseTimeStamp parses:  time_stamp = date [" "] time
// Both date and time are mandatory.
func (p *V12Parser) ParseTimeStamp() (*V12TimeStampNode, error) {
	line, col := p.cur().Line, p.cur().Col

	date, err := p.ParseDate()
	if err != nil {
		return nil, err
	}
	time, err := p.ParseTime()
	if err != nil {
		return nil, err
	}
	return &V12TimeStampNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Date:        date,
		Time:        time,
	}, nil
}

// ParseDuration parses:  duration = digits duration_unit { digits duration_unit }
// e.g. "1h30m", "500ms"
func (p *V12Parser) ParseDuration() (*V12DurationNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V12_INTEGER {
		return nil, p.errAt(fmt.Sprintf("expected digits for duration, got %s %q", tok.Type, tok.Value))
	}

	node := &V12DurationNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}

	for p.cur().Type == V12_INTEGER {
		digits := p.cur().Value
		p.advance()
		unit, err := p.parseDurationUnit()
		if err != nil {
			return nil, err
		}
		node.Segments = append(node.Segments, V12DurationSegment{Digits: digits, Unit: unit})
	}
	return node, nil
}

// parseDurationUnit consumes one duration_unit token.
func (p *V12Parser) parseDurationUnit() (string, error) {
	tok := p.cur()
	switch tok.Type {
	case V12_MS, V12_SEC, V12_MIN, V12_HR, V12_DAY, V12_WK:
		p.advance()
		return tok.Value, nil
	}
	return "", p.errAt(fmt.Sprintf("expected duration unit (ms s m h d w), got %s %q", tok.Type, tok.Value))
}

// --------------------------------------------------------------------------
// Regexp
// --------------------------------------------------------------------------

// ParseRegexp parses:  regexp = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags { regexp_flags } ]
// The lexer has already done the heavy lifting — the token value is the full
// /pattern/flags string.
func (p *V12Parser) ParseRegexp() (*V12RegexpNode, error) {
	tok := p.cur()
	if tok.Type != V12_REGEXP && tok.Type != V12_REGEXP_DECL {
		return nil, p.errAt(fmt.Sprintf("expected regexp, got %s %q", tok.Type, tok.Value))
	}
	line, col := tok.Line, tok.Col
	p.advance()

	raw := tok.Value
	// Trim surrounding slashes and extract flags.
	// Format: /pattern/flags  or  //  (empty)
	pattern := ""
	flags := ""
	if raw == "//" {
		// empty regexp
	} else {
		// raw starts and ends with / (with optional flags after last /)
		end := strings.LastIndex(raw[1:], "/") + 1
		if end > 0 {
			pattern = raw[1:end]
			flags = raw[end+1:]
		}
	}
	return &V12RegexpNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Pattern:     pattern,
		Flags:       flags,
	}, nil
}

// --------------------------------------------------------------------------
// Boolean / null
// --------------------------------------------------------------------------

// ParseBoolean parses:  boolean = "true" | "false"
func (p *V12Parser) ParseBoolean() (*V12BoolNode, error) {
	tok := p.cur()
	switch tok.Type {
	case V12_TRUE:
		p.advance()
		return &V12BoolNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}, Value: true}, nil
	case V12_FALSE:
		p.advance()
		return &V12BoolNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}, Value: false}, nil
	}
	return nil, p.errAt(fmt.Sprintf("expected boolean (true | false), got %s %q", tok.Type, tok.Value))
}

// ParseNull parses:  null = "null"
func (p *V12Parser) ParseNull() (*V12NullNode, error) {
	tok := p.cur()
	if tok.Type != V12_NULL {
		return nil, p.errAt(fmt.Sprintf("expected null, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V12NullNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// ParseAnyType parses:  any_type = "@?"
func (p *V12Parser) ParseAnyType() (*V12AnyTypeNode, error) {
	tok := p.cur()
	if tok.Type != V12_ANY_TYPE {
		return nil, p.errAt(fmt.Sprintf("expected @?, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V12AnyTypeNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// --------------------------------------------------------------------------
// Cardinality / range
// --------------------------------------------------------------------------

// ParseCardinality parses:  cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
func (p *V12Parser) ParseCardinality() (*V12CardinalityNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	lo, err := p.ParseDigits()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_DOTDOT); err != nil {
		return nil, err
	}

	hiTok := p.cur()
	var hi string
	switch hiTok.Type {
	case V12_INTEGER:
		hi = hiTok.Value
		p.advance()
	case V12_MANY:
		hi = hiTok.Value
		p.advance()
	case V12_MIN:
		// "m" is lexed as V12_MIN (minutes unit) but also serves as short-form "many"
		hi = "m"
		p.advance()
	case V12_IDENT:
		v := strings.ToLower(hiTok.Value)
		if v == "m" || v == "many" {
			hi = hiTok.Value
			p.advance()
		} else {
			return nil, p.errAt(fmt.Sprintf("expected cardinality upper bound, got %q", hiTok.Value))
		}
	default:
		return nil, p.errAt(fmt.Sprintf("expected cardinality upper bound, got %s %q", hiTok.Type, hiTok.Value))
	}

	return &V12CardinalityNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// ParseRange parses:  range = integer ".." integer
func (p *V12Parser) ParseRange() (*V12RangeNode, error) {
	line, col := p.cur().Line, p.cur().Col

	lo, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_DOTDOT); err != nil {
		return nil, err
	}
	hi, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	return &V12RangeNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// --------------------------------------------------------------------------
// UUID parsers
// --------------------------------------------------------------------------

// V12hexSeg returns true if s is a hex string of exactly n characters.
func V12hexSeg(s string, n int) bool {
	if len(s) != n {
		return false
	}
	re := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	return re.MatchString(s)
}

// ParseUUID parses a standard 8-4-4-4-12 UUID from sequential tokens.
// Grammar: hex_seg8 "-" hex_seg4 "-" hex_seg4 "-" hex_seg4 "-" hex_seg12
// The lexer does not have a dedicated UUID token; the parser assembles it
// from identifier/integer tokens and dash separators.
func (p *V12Parser) ParseUUID() (*V12UUIDNode, error) {
	line, col := p.cur().Line, p.cur().Col

	seg8, err := p.parseHexSeg(8)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_MINUS); err != nil {
		return nil, err
	}
	seg4a, err := p.parseHexSeg(4)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_MINUS); err != nil {
		return nil, err
	}
	seg4b, err := p.parseHexSeg(4)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_MINUS); err != nil {
		return nil, err
	}
	seg4c, err := p.parseHexSeg(4)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_MINUS); err != nil {
		return nil, err
	}
	seg12, err := p.parseHexSeg(12)
	if err != nil {
		return nil, err
	}

	value := seg8 + "-" + seg4a + "-" + seg4b + "-" + seg4c + "-" + seg12
	return &V12UUIDNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: value}, nil
}

// parseHexSeg reads the next token as a hex segment of exactly n characters.
// The token may be V12_INTEGER (all-digit hex) or V12_IDENT (contains a-f).
func (p *V12Parser) parseHexSeg(n int) (string, error) {
	tok := p.cur()
	if tok.Type != V12_INTEGER && tok.Type != V12_IDENT {
		return "", p.errAt(fmt.Sprintf("expected %d-char hex segment, got %s %q", n, tok.Type, tok.Value))
	}
	if !V12hexSeg(tok.Value, n) {
		return "", p.errAt(fmt.Sprintf("expected %d-char hex segment, got %q", n, tok.Value))
	}
	p.advance()
	return tok.Value, nil
}

// --------------------------------------------------------------------------
// URL / file path parsers
// --------------------------------------------------------------------------

// V12urlRe matches http/https URLs.
var V12urlRe = regexp.MustCompile(`^https?://`)

// ParseHTTPURL parses an http_url.  In source text URLs appear as plain STRING
// tokens (quoted) or as unquoted identifier sequences.  We accept either quoting.
func (p *V12Parser) ParseHTTPURL() (*V12URLNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var raw string
	switch tok.Type {
	case V12_STRING:
		raw = tok.Value
		// Strip surrounding quotes.
		if len(raw) >= 2 {
			raw = raw[1 : len(raw)-1]
		}
		p.advance()
	default:
		return nil, p.errAt(fmt.Sprintf("expected quoted http_url, got %s %q", tok.Type, tok.Value))
	}

	if !V12urlRe.MatchString(raw) {
		return nil, &V12ParseError{Line: line, Col: col,
			Message: fmt.Sprintf("invalid http_url: %q", raw)}
	}
	return &V12URLNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: raw}, nil
}

// ParseFilePath parses a file_path (quoted string beginning with / or C:/ etc.).
func (p *V12Parser) ParseFilePath() (*V12FilePathNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V12_STRING {
		return nil, p.errAt(fmt.Sprintf("expected quoted file_path, got %s %q", tok.Type, tok.Value))
	}
	raw := tok.Value
	if len(raw) >= 2 {
		raw = raw[1 : len(raw)-1]
	}
	p.advance()
	return &V12FilePathNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: raw}, nil
}

// --------------------------------------------------------------------------
// Constant — top-level value node
// --------------------------------------------------------------------------

// ParseConstant parses:
//
//	constant = numeric_const | string | char | regexp | boolean | null
//	           | date | time | date_time | time_stamp | uuid | http_url | file_path
//
// The grammar is ambiguous without context (e.g. a 4-digit integer could be a
// year or just a number).  This parser uses the following priority order:
//  1. regexp
//  2. boolean
//  3. null
//  4. NaN / Infinity   (float values)
//  5. string (any quoted)
//  6. numeric_const  (integer or decimal)
//
// Date/Time/UUID/URL are not attempted here — they require caller context.
func (p *V12Parser) ParseConstant() (*V12ConstantNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var inner V12Node
	var err error

	switch tok.Type {
	case V12_REGEXP, V12_REGEXP_DECL:
		inner, err = p.ParseRegexp()
	case V12_TRUE, V12_FALSE:
		inner, err = p.ParseBoolean()
	case V12_NULL:
		inner, err = p.ParseNull()
	case V12_NAN:
		inner, err = p.ParseNan()
	case V12_INFINITY:
		inner, err = p.ParseInfinity()
	case V12_PLUS:
		// Could be +Infinity, +decimal, +integer
		next := p.peek(1)
		if next.Type == V12_INFINITY {
			inner, err = p.ParseInfinity()
		} else if next.Type == V12_DECIMAL {
			inner, err = p.ParseDecimal()
		} else {
			inner, err = p.ParseInteger()
		}
	case V12_MINUS:
		next := p.peek(1)
		if next.Type == V12_INFINITY {
			inner, err = p.ParseInfinity()
		} else if next.Type == V12_DECIMAL {
			inner, err = p.ParseDecimal()
		} else {
			inner, err = p.ParseInteger()
		}
	case V12_STRING, V12_EMPTY_STR_D, V12_EMPTY_STR_S, V12_EMPTY_STR_T:
		inner, err = p.ParseString()
	case V12_DECIMAL:
		inner, err = p.ParseDecimal()
	case V12_INTEGER:
		inner, err = p.ParseInteger()
	default:
		return nil, p.errAt(fmt.Sprintf("unexpected token %s %q in constant", tok.Type, tok.Value))
	}

	if err != nil {
		return nil, err
	}
	return &V12ConstantNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: inner}, nil
}
