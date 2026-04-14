// Package parser implements a hand-written recursive-descent parser for the
// Squeeze language (V10), following the plan in specifications/parser_v3.md.
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
// PHASE 1 — TOKEN TYPES (V10)
// =============================================================================

// V10TokenType identifies the category of a lexed token.
type V10TokenType int

const (
	// ---- Synthetic / control tokens ----
	V10_BOF     V10TokenType = iota // Beginning of file (first token always emitted)
	V10_EOF                         // End of file (last token always emitted)
	V10_NL                          // Newline sequence  /([ \t]*[\r\n]+)+/ or BOF
	V10_ILLEGAL                     // Unrecognised character

	// ---- Literal value tokens ----
	V10_INTEGER // unsigned digit sequence  e.g. 42  (sign is a separate token)
	V10_DECIMAL // unsigned decimal  e.g. 3.14
	V10_STRING  // single/double/template quoted  '…' "…" `…` (delimiters included)
	V10_REGEXP  // regexp literal  /pattern/flags

	// ---- Identifier tokens ----
	V10_IDENT    // plain identifier  (Unicode letters, digits, _, embedded spaces)
	V10_AT_IDENT // @-prefixed identifier  @name

	// ---- Boolean / null keywords ----
	V10_TRUE  // true
	V10_FALSE // false
	V10_NULL  // null

	// ---- Cardinality upper-bound keyword ----
	V10_MANY // many | Many

	// ---- Built-in type keywords ----
	V10_NAN      // NaN
	V10_INFINITY // Infinity | +Infinity | -Infinity  (sign emitted as separate tok)

	// ---- Duration-unit keywords ----
	V10_MS  // ms
	V10_SEC // s
	V10_MIN // m
	V10_HR  // h
	V10_DAY // d
	V10_WK  // w

	// ---- Parser directive keywords ----
	V10_UNIQUE     // UNIQUE
	V10_RANGE_KW   // RANGE
	V10_TYPE_OF    // TYPE_OF
	V10_VALUE_OF   // VALUE_OF
	V10_ADDRESS_OF // ADDRESS_OF
	V10_RETURN_DIR // RETURN  (directive form — distinct from <- return-statement)
	V10_UNIFORM    // UNIFORM
	V10_INFER      // INFER
	V10_CAST       // CAST
	V10_EXTEND     // EXTEND
	V10_MERGE      // MERGE
	V10_SQZ        // SQZ
	V10_CODE       // CODE

	// ---- Multi-character operators (max-munch) ----
	V10_DOTDOT        // ..    range separator
	V10_DOTDOLLAR     // .$    JSONPath entry shorthand
	V10_ARROW         // ->    data-chain connector
	V10_STREAM        // >>    stream connector
	V10_STORE         // =>    store/deps operator
	V10_RETURN_STMT   // <-    return statement marker
	V10_INC           // ++    inline increment
	V10_DEC           // --    inline decrement
	V10_POW           // **    exponentiation
	V10_IADD_IMM      // +:    incremental immutable assign +
	V10_ISUB_IMM      // -:    incremental immutable assign -
	V10_IMUL_IMM      // *:    incremental immutable assign *
	V10_IDIV_IMM      // /:    incremental immutable assign /
	V10_IADD_MUT      // +~    incremental mutable assign +
	V10_ISUB_MUT      // -~    incremental mutable assign -
	V10_IMUL_MUT      // *~    incremental mutable assign *
	V10_IDIV_MUT      // /~    incremental mutable assign /
	V10_READONLY      // :~    read-only reference assign
	V10_EXTEND_ASSIGN // +=    incremental mutable assign + / extend-scope operator
	V10_ISUB_ASSIGN   // -=    incremental mutable assign -
	V10_IMUL_ASSIGN   // *=    incremental mutable assign *
	V10_IDIV_ASSIGN   // /=    incremental mutable assign /
	V10_GEQ           // >=    compare
	V10_LEQ           // <=    compare
	V10_NEQ           // !=    compare (not-equal)
	V10_EQEQ          // ==    equality compare
	V10_MATCH_OP      // =~    string-regexp compare
	V10_AMP_AMP       // &&    logical and
	V10_PIPE_PIPE     // ||    logical or
	V10_EMPTY_ARR     // []    empty array initialiser
	V10_EMPTY_OBJ     // {}    empty object initialiser
	V10_EMPTY_STR_D   // ""    empty double-quoted string
	V10_EMPTY_STR_S   // ''    empty single-quoted string
	V10_EMPTY_STR_T   // ``    empty template-quoted string
	V10_REGEXP_DECL   // //    empty regexp initialiser

	// ---- Single-character operators and punctuation ----
	V10_PLUS      // +
	V10_MINUS     // -
	V10_STAR      // *
	V10_SLASH     // /   (division)
	V10_PERCENT   // %
	V10_TILDE     // ~   mutable assign base
	V10_COLON     // :   immutable assign base
	V10_EQ        // =
	V10_GT        // >
	V10_LT        // <
	V10_BANG      // !   logical not
	V10_AMP       // &
	V10_PIPE      // |
	V10_CARET     // ^
	V10_DOT       // .
	V10_COMMA     // ,
	V10_SEMICOLON // ;
	V10_LPAREN    // (
	V10_RPAREN    // )
	V10_LBRACKET  // [
	V10_RBRACKET  // ]
	V10_LBRACE    // {
	V10_RBRACE    // }
	V10_DOLLAR    // $   self-reference / JSONPath root marker
	V10_SECTION   // §   section marker
	V10_AT        // @   (standalone before @-ident is consumed)
	V10_QUESTION  // ?   JSONPath filter shorthand
)

// v10tokenNames maps each V10TokenType to its display name.
var v10tokenNames = map[V10TokenType]string{
	V10_BOF: "BOF", V10_EOF: "EOF", V10_NL: "NL", V10_ILLEGAL: "ILLEGAL",
	V10_INTEGER: "INTEGER", V10_DECIMAL: "DECIMAL", V10_STRING: "STRING", V10_REGEXP: "REGEXP",
	V10_IDENT: "IDENT", V10_AT_IDENT: "AT_IDENT",
	V10_TRUE: "true", V10_FALSE: "false", V10_NULL: "null", V10_MANY: "many",
	V10_NAN: "NaN", V10_INFINITY: "Infinity",
	V10_MS: "ms", V10_SEC: "s", V10_MIN: "m", V10_HR: "h", V10_DAY: "d", V10_WK: "w",
	V10_UNIQUE: "UNIQUE", V10_RANGE_KW: "RANGE", V10_TYPE_OF: "TYPE_OF",
	V10_VALUE_OF: "VALUE_OF", V10_ADDRESS_OF: "ADDRESS_OF",
	V10_RETURN_DIR: "RETURN", V10_UNIFORM: "UNIFORM", V10_INFER: "INFER",
	V10_CAST: "CAST", V10_EXTEND: "EXTEND", V10_MERGE: "MERGE",
	V10_SQZ: "SQZ", V10_CODE: "CODE",
	V10_DOTDOT: "..", V10_DOTDOLLAR: ".$", V10_ARROW: "->", V10_STREAM: ">>",
	V10_STORE: "=>", V10_RETURN_STMT: "<-",
	V10_INC: "++", V10_DEC: "--", V10_POW: "**",
	V10_IADD_IMM: "+:", V10_ISUB_IMM: "-:", V10_IMUL_IMM: "*:", V10_IDIV_IMM: "/:",
	V10_IADD_MUT: "+~", V10_ISUB_MUT: "-~", V10_IMUL_MUT: "*~", V10_IDIV_MUT: "/~",
	V10_READONLY: ":~", V10_EXTEND_ASSIGN: "+=", V10_ISUB_ASSIGN: "-=", V10_IMUL_ASSIGN: "*=", V10_IDIV_ASSIGN: "/=",
	V10_GEQ: ">=", V10_LEQ: "<=", V10_NEQ: "!=", V10_EQEQ: "==", V10_MATCH_OP: "=~",
	V10_AMP_AMP: "&&", V10_PIPE_PIPE: "||",
	V10_EMPTY_ARR: "[]", V10_EMPTY_OBJ: "{}", V10_EMPTY_STR_D: `""`,
	V10_EMPTY_STR_S: "''", V10_EMPTY_STR_T: "``", V10_REGEXP_DECL: "//",
	V10_PLUS: "+", V10_MINUS: "-", V10_STAR: "*", V10_SLASH: "/", V10_PERCENT: "%",
	V10_TILDE: "~", V10_COLON: ":", V10_EQ: "=", V10_GT: ">", V10_LT: "<",
	V10_BANG: "!", V10_AMP: "&", V10_PIPE: "|", V10_CARET: "^",
	V10_DOT: ".", V10_COMMA: ",", V10_SEMICOLON: ";",
	V10_LPAREN: "(", V10_RPAREN: ")", V10_LBRACKET: "[",
	V10_RBRACKET: "]", V10_LBRACE: "{", V10_RBRACE: "}",
	V10_DOLLAR: "$", V10_SECTION: "§", V10_AT: "@", V10_QUESTION: "?",
}

// String returns the display name of a V10TokenType.
func (t V10TokenType) String() string {
	if s, ok := v10tokenNames[t]; ok {
		return s
	}
	return fmt.Sprintf("V10TOKEN(%d)", int(t))
}

// =============================================================================
// PHASE 1 — TOKEN (V10)
// =============================================================================

// V10Token represents a single lexeme produced by the V10 lexer.
type V10Token struct {
	Type  V10TokenType
	Value string
	Line  int
	Col   int
}

// String returns a compact, readable representation for diagnostics.
func (t V10Token) String() string {
	return fmt.Sprintf("V10Token{%s %q L%d:C%d}", t.Type, t.Value, t.Line, t.Col)
}

// =============================================================================
// PHASE 1 — LEXER (V10)
// =============================================================================

// v10keywords maps literal source strings to their corresponding V10TokenType.
var v10keywords = map[string]V10TokenType{
	"true":       V10_TRUE,
	"false":      V10_FALSE,
	"null":       V10_NULL,
	"many":       V10_MANY,
	"Many":       V10_MANY,
	"NaN":        V10_NAN,
	"Infinity":   V10_INFINITY,
	"ms":         V10_MS,
	"UNIQUE":     V10_UNIQUE,
	"RANGE":      V10_RANGE_KW,
	"TYPE_OF":    V10_TYPE_OF,
	"VALUE_OF":   V10_VALUE_OF,
	"ADDRESS_OF": V10_ADDRESS_OF,
	"RETURN":     V10_RETURN_DIR,
	"UNIFORM":    V10_UNIFORM,
	"INFER":      V10_INFER,
	"CAST":       V10_CAST,
	"EXTEND":     V10_EXTEND,
	"MERGE":      V10_MERGE,
	"SQZ":        V10_SQZ,
	"CODE":       V10_CODE,
}

// v10durationKeywords are single-character duration unit keywords.
// They only become duration-unit tokens when parsing is inside a duration context;
// elsewhere they are plain TOK_IDENT.  The lexer always checks here and emits
// the specialised type — the parser uses context to disambiguate.
var v10durationUnits = map[string]V10TokenType{
	"s": V10_SEC,
	"m": V10_MIN,
	"h": V10_HR,
	"d": V10_DAY,
	"w": V10_WK,
}

// v10prevIsValue returns true when tok is a "value-ending" token —
// i.e. the previous token closes a value expression, meaning a following `/`
// is division rather than the start of a regexp literal.
func v10prevIsValue(t V10TokenType) bool {
	switch t {
	case V10_INTEGER, V10_DECIMAL, V10_STRING, V10_REGEXP,
		V10_IDENT, V10_AT_IDENT, V10_TRUE, V10_FALSE, V10_NULL,
		V10_NAN, V10_INFINITY,
		V10_RPAREN, V10_RBRACKET, V10_RBRACE,
		V10_INC, V10_DEC,
		V10_EMPTY_STR_D, V10_EMPTY_STR_S, V10_EMPTY_STR_T,
		V10_EMPTY_ARR, V10_EMPTY_OBJ:
		return true
	}
	return false
}

// V10Lexer holds the mutable state of the V10 scanner.
type V10Lexer struct {
	input []rune
	pos   int
	line  int
	col   int
	last  V10TokenType // last significant (non-NL) token type for disambiguation
}

// NewV10Lexer constructs a fresh V10Lexer for the given source string.
func NewV10Lexer(src string) *V10Lexer {
	return &V10Lexer{
		input: []rune(src),
		pos:   0,
		line:  1,
		col:   0,
		last:  V10_BOF,
	}
}

// V10Tokenize scans the entire input and returns the full token slice.
// The slice always begins with V10_BOF and ends with V10_EOF.
// An error is returned if an illegal character is encountered; partial results
// up to that point are also returned.
func (l *V10Lexer) V10Tokenize() ([]V10Token, error) {
	tokens := []V10Token{l.v10makeTok(V10_BOF, "", 1, 0)}

	for {
		tok, err := l.v10scan()
		if err != nil {
			return tokens, err
		}
		if tok.Type != V10_NL {
			l.last = tok.Type
		}
		tokens = append(tokens, tok)
		if tok.Type == V10_EOF {
			break
		}
	}
	return tokens, nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

func (l *V10Lexer) v10peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *V10Lexer) v10peek2() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *V10Lexer) v10advance() rune {
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

func (l *V10Lexer) v10skipHorizWS() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' {
			l.v10advance()
		} else {
			break
		}
	}
}

func (l *V10Lexer) v10makeTok(typ V10TokenType, val string, line, col int) V10Token {
	return V10Token{Type: typ, Value: val, Line: line, Col: col}
}

// --------------------------------------------------------------------------
// scan — top-level dispatch
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scan() (V10Token, error) {
	l.v10skipHorizWS()

	if l.pos >= len(l.input) {
		return l.v10makeTok(V10_EOF, "", l.line, l.col), nil
	}

	line, col := l.line, l.col
	ch := l.v10peek()

	// Newlines → NL token
	if ch == '\r' || ch == '\n' {
		return l.v10scanNL(line, col)
	}

	// (* …  *) nested comments — emitted as NL (whitespace-like)
	if ch == '(' && l.v10peek2() == '*' {
		return l.v10scanComment(line, col)
	}

	// Unicode letter or underscore → identifier or keyword
	if unicode.IsLetter(ch) || ch == '_' {
		return l.v10scanIdentOrKeyword(line, col), nil
	}

	// @identifier
	if ch == '@' {
		return l.v10scanAtIdent(line, col)
	}

	// Digit → number
	if unicode.IsDigit(ch) {
		return l.v10scanNumber(line, col), nil
	}

	// Quoted strings
	switch ch {
	case '"':
		return l.v10scanDoubleQuoted(line, col)
	case '\'':
		return l.v10scanSingleQuoted(line, col)
	case '`':
		return l.v10scanTemplateQuoted(line, col)
	}

	// Operators and punctuation
	return l.v10scanOperator(line, col)
}

// --------------------------------------------------------------------------
// Newline scanner  NL = /([ \t]*[\r\n]+)+/
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanNL(line, col int) (V10Token, error) {
	var sb strings.Builder

	for l.pos < len(l.input) {
		wsStart := l.pos
		for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t') {
			l.v10advance()
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
			ch := l.v10advance()
			sb.WriteRune(ch)
			if ch == '\r' && l.pos < len(l.input) && l.input[l.pos] == '\n' {
				sb.WriteRune(l.v10advance())
			}
			consumed = true
		}
		if !consumed {
			break
		}
	}

	return l.v10makeTok(V10_NL, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Nested comment scanner  comment = "(*"  comment_txt { comment }  "*)"
// Comments are treated as whitespace (emitted as V10_NL).
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanComment(line, col int) (V10Token, error) {
	depth := 0
	var sb strings.Builder

	for l.pos < len(l.input) {
		ch := l.v10peek()
		ch2 := l.v10peek2()

		if ch == '(' && ch2 == '*' {
			depth++
			sb.WriteRune(l.v10advance())
			sb.WriteRune(l.v10advance())
			continue
		}
		if ch == '*' && ch2 == ')' {
			sb.WriteRune(l.v10advance())
			sb.WriteRune(l.v10advance())
			depth--
			if depth == 0 {
				return l.v10makeTok(V10_NL, sb.String(), line, col), nil
			}
			continue
		}
		sb.WriteRune(l.v10advance())
	}

	return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated comment", line, col)
}

// --------------------------------------------------------------------------
// Identifier / keyword scanner
// Grammar: ident_name = /(?<value>[\p{L}][\p{L}0-9_]*)/
// Spaces are NOT part of an identifier (V11 and later).
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanIdentOrKeyword(line, col int) V10Token {
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.v10advance())
		} else {
			break
		}
	}
	value := sb.String()

	// Check explicit keyword map first.
	if tt, ok := v10keywords[value]; ok {
		return l.v10makeTok(tt, value, line, col)
	}
	// Single-character duration unit keywords (s m h d w) — emitted as specialised types
	// so the parser can recognise them in a duration context.
	if tt, ok := v10durationUnits[value]; ok {
		return l.v10makeTok(tt, value, line, col)
	}
	return l.v10makeTok(V10_IDENT, value, line, col)
}

// --------------------------------------------------------------------------
// @-prefixed identifier scanner  @name  @type  …
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanAtIdent(line, col int) (V10Token, error) {
	l.v10advance() // consume '@'
	// Standalone '@' (e.g. JSONPath current-node '@') — emit V10_AT.
	if l.pos >= len(l.input) || !unicode.IsLetter(l.input[l.pos]) {
		return l.v10makeTok(V10_AT, "@", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('@')
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.v10advance())
		} else {
			break
		}
	}
	return l.v10makeTok(V10_AT_IDENT, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Number scanner  →  V10_INTEGER | V10_DECIMAL
// The lexer emits unsigned digit sequences; signs (+/-) are separate tokens.
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanNumber(line, col int) V10Token {
	var sb strings.Builder
	for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
		sb.WriteRune(l.v10advance())
	}
	// Decimal: digits "." digits — but NOT the ".." range operator.
	if l.pos < len(l.input) && l.input[l.pos] == '.' &&
		l.pos+1 < len(l.input) && l.input[l.pos+1] != '.' {
		sb.WriteRune(l.v10advance()) // '.'
		for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
			sb.WriteRune(l.v10advance())
		}
		return l.v10makeTok(V10_DECIMAL, sb.String(), line, col)
	}
	return l.v10makeTok(V10_INTEGER, sb.String(), line, col)
}

// --------------------------------------------------------------------------
// String scanners
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanDoubleQuoted(line, col int) (V10Token, error) {
	l.v10advance() // consume opening "
	if l.v10peek() == '"' {
		l.v10advance()
		return l.v10makeTok(V10_EMPTY_STR_D, `""`, line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('"')
	for l.pos < len(l.input) {
		ch := l.v10advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.v10advance())
		case ch == '"':
			return l.v10makeTok(V10_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
		}
	}
	return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
}

func (l *V10Lexer) v10scanSingleQuoted(line, col int) (V10Token, error) {
	l.v10advance() // consume opening '
	if l.v10peek() == '\'' {
		l.v10advance()
		return l.v10makeTok(V10_EMPTY_STR_S, "''", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('\'')
	for l.pos < len(l.input) {
		ch := l.v10advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.v10advance())
		case ch == '\'':
			return l.v10makeTok(V10_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
		}
	}
	return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
}

// v10scanTemplateQuoted handles `…` strings with optional $(...) interpolation.
// Grammar: tmpl_quoted = "`" ( tmpl_text | tmpl_expr ) { ( tmpl_text | tmpl_expr ) } "`"
// The lexer emits the entire template literal as a single V10_STRING token,
// preserving all interpolation syntax verbatim.  The parser reconstructs the
// tmpl_expr segments from the raw value when building AST nodes.
func (l *V10Lexer) v10scanTemplateQuoted(line, col int) (V10Token, error) {
	l.v10advance() // consume opening `
	if l.v10peek() == '`' {
		l.v10advance()
		return l.v10makeTok(V10_EMPTY_STR_T, "``", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('`')
	depth := 0 // nesting depth inside $( ... )
	for l.pos < len(l.input) {
		ch := l.v10peek()
		ch2 := l.v10peek2()

		if ch == '$' && ch2 == '(' && depth == 0 {
			sb.WriteRune(l.v10advance()) // $
			sb.WriteRune(l.v10advance()) // (
			depth++
			continue
		}
		if ch == '(' && depth > 0 {
			depth++
			sb.WriteRune(l.v10advance())
			continue
		}
		if ch == ')' && depth > 0 {
			depth--
			sb.WriteRune(l.v10advance())
			continue
		}
		if ch == '`' && depth == 0 {
			sb.WriteRune(l.v10advance())
			return l.v10makeTok(V10_STRING, sb.String(), line, col), nil
		}
		if ch == 0 {
			return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated template string", line, col)
		}
		sb.WriteRune(l.v10advance())
	}
	return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated template string", line, col)
}

// --------------------------------------------------------------------------
// Regexp scanner
// grammar: regexp = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags { regexp_flags } ]
// Called when '/' is encountered and context indicates a regexp, not division.
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanRegexp(line, col int) (V10Token, error) {
	l.v10advance() // consume opening /
	var sb strings.Builder
	sb.WriteRune('/')

	for l.pos < len(l.input) {
		ch := l.v10advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.v10advance())
		case ch == '/':
			// Consume optional flags: g i m s u y x n A
			for l.pos < len(l.input) {
				f := l.input[l.pos]
				if f == 'g' || f == 'i' || f == 'm' || f == 's' ||
					f == 'u' || f == 'y' || f == 'x' || f == 'n' || f == 'A' {
					sb.WriteRune(l.v10advance())
				} else {
					break
				}
			}
			return l.v10makeTok(V10_REGEXP, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
		}
	}
	return l.v10makeTok(V10_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
}

// --------------------------------------------------------------------------
// Operator scanner — max-munch, all single and multi-character operators.
// --------------------------------------------------------------------------

func (l *V10Lexer) v10scanOperator(line, col int) (V10Token, error) {
	ch := l.v10peek()
	ch2 := l.v10peek2()

	switch ch {

	// . .. .$
	case '.':
		if ch2 == '.' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_DOTDOT, "..", line, col), nil
		}
		if ch2 == '$' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_DOTDOLLAR, ".$", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_DOT, ".", line, col), nil

	// - -> -- -: -~
	case '-':
		switch ch2 {
		case '>':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_ARROW, "->", line, col), nil
		case '-':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_DEC, "--", line, col), nil
		case ':':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_ISUB_IMM, "-:", line, col), nil
		case '~':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_ISUB_MUT, "-~", line, col), nil
		case '=':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_ISUB_ASSIGN, "-=", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_MINUS, "-", line, col), nil

	// > >> >=
	case '>':
		switch ch2 {
		case '>':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_STREAM, ">>", line, col), nil
		case '=':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_GEQ, ">=", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_GT, ">", line, col), nil

	// = => =~
	case '=':
		switch ch2 {
		case '>':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_STORE, "=>", line, col), nil
		case '~':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_MATCH_OP, "=~", line, col), nil
		case '=':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_EQEQ, "==", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_EQ, "=", line, col), nil

	// < <- <=
	case '<':
		switch ch2 {
		case '-':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_RETURN_STMT, "<-", line, col), nil
		case '=':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_LEQ, "<=", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_LT, "<", line, col), nil

	// + ++ +: +~ +=
	case '+':
		switch ch2 {
		case '+':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_INC, "++", line, col), nil
		case ':':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IADD_IMM, "+:", line, col), nil
		case '~':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IADD_MUT, "+~", line, col), nil
		case '=':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_EXTEND_ASSIGN, "+=", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_PLUS, "+", line, col), nil

	// * ** *: *~
	case '*':
		switch ch2 {
		case '*':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_POW, "**", line, col), nil
		case ':':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IMUL_IMM, "*:", line, col), nil
		case '~':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IMUL_MUT, "*~", line, col), nil
		case '=':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IMUL_ASSIGN, "*=", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_STAR, "*", line, col), nil

	// / // /: /~  or regexp /…/flags
	case '/':
		switch ch2 {
		case '/':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_REGEXP_DECL, "//", line, col), nil
		case ':':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IDIV_IMM, "/:", line, col), nil
		case '~':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IDIV_MUT, "/~", line, col), nil
		case '=':
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_IDIV_ASSIGN, "/=", line, col), nil
		}
		if v10prevIsValue(l.last) {
			l.v10advance()
			return l.v10makeTok(V10_SLASH, "/", line, col), nil
		}
		return l.v10scanRegexp(line, col)

	// : :~
	case ':':
		if ch2 == '~' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_READONLY, ":~", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_COLON, ":", line, col), nil

	// ! !=
	case '!':
		if ch2 == '=' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_NEQ, "!=", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_BANG, "!", line, col), nil

	// & &&
	case '&':
		if ch2 == '&' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_AMP_AMP, "&&", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_AMP, "&", line, col), nil

	// | ||
	case '|':
		if ch2 == '|' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_PIPE_PIPE, "||", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_PIPE, "|", line, col), nil

	// [ []
	case '[':
		if ch2 == ']' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_EMPTY_ARR, "[]", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_LBRACKET, "[", line, col), nil

	// { {}
	case '{':
		if ch2 == '}' {
			l.v10advance()
			l.v10advance()
			return l.v10makeTok(V10_EMPTY_OBJ, "{}", line, col), nil
		}
		l.v10advance()
		return l.v10makeTok(V10_LBRACE, "{", line, col), nil

	// $ dollar — self-reference / JSONPath root
	case '$':
		l.v10advance()
		return l.v10makeTok(V10_DOLLAR, "$", line, col), nil

	// § section marker (U+00A7)
	case '§':
		l.v10advance()
		return l.v10makeTok(V10_SECTION, "§", line, col), nil

	// ? question mark
	case '?':
		l.v10advance()
		return l.v10makeTok(V10_QUESTION, "?", line, col), nil

	// Single-character punctuation
	case '~':
		l.v10advance()
		return l.v10makeTok(V10_TILDE, "~", line, col), nil
	case '%':
		l.v10advance()
		return l.v10makeTok(V10_PERCENT, "%", line, col), nil
	case '^':
		l.v10advance()
		return l.v10makeTok(V10_CARET, "^", line, col), nil
	case ',':
		l.v10advance()
		return l.v10makeTok(V10_COMMA, ",", line, col), nil
	case ';':
		l.v10advance()
		return l.v10makeTok(V10_SEMICOLON, ";", line, col), nil
	case '(':
		l.v10advance()
		return l.v10makeTok(V10_LPAREN, "(", line, col), nil
	case ')':
		l.v10advance()
		return l.v10makeTok(V10_RPAREN, ")", line, col), nil
	case ']':
		l.v10advance()
		return l.v10makeTok(V10_RBRACKET, "]", line, col), nil
	case '}':
		l.v10advance()
		return l.v10makeTok(V10_RBRACE, "}", line, col), nil
	}

	bad := l.v10advance()
	return l.v10makeTok(V10_ILLEGAL, string(bad), line, col),
		fmt.Errorf("L%d:C%d: unexpected character %q", line, col, bad)
}

// =============================================================================
// PHASE 2 — AST NODE TYPES  (01_definitions.sqg)
// =============================================================================

// V10Node is the common interface for all AST nodes.
type V10Node interface {
	v10NodePos() (line, col int)
}

// v10BaseNode carries source position, embedded in every AST node.
type v10BaseNode struct {
	Line int
	Col  int
}

func (n v10BaseNode) v10NodePos() (int, int) { return n.Line, n.Col }

// ---------- Numeric primitives ----------

// V10IntegerNode  integer = [ "+" | "-" ] digits
type V10IntegerNode struct {
	v10BaseNode
	Sign   string // "", "+", or "-"
	Digits string // raw digit string
}

// V10DecimalNode  decimal = [ "+" | "-" ] digits "." digits
type V10DecimalNode struct {
	v10BaseNode
	Sign     string
	Integral string // digits before "."
	Frac     string // digits after "."
}

// V10NumericConstNode  numeric_const = integer | decimal
type V10NumericConstNode struct {
	v10BaseNode
	Value V10Node // *V10IntegerNode | *V10DecimalNode
}

// ---------- Typed integer / unsigned integer nodes ----------

// V10IntTypeNode represents a range-constrained signed integer (int8..int128)
// or unsigned integer (byte, uint8..uint128).
// The Kind field carries the type name ("int8", "uint32", etc.).
type V10IntTypeNode struct {
	v10BaseNode
	Kind  string // "int8", "byte", "uint64", …
	Value *V10IntegerNode
}

// ---------- Float nodes ----------

// V10NanNode  nan = "NaN"
type V10NanNode struct{ v10BaseNode }

// V10InfinityNode  infinity = "+Infinity" | "-Infinity" | "Infinity"
type V10InfinityNode struct {
	v10BaseNode
	Sign string // "", "+", or "-"
}

// V10FloatNode  float32 | float64  (TYPE_OF self-referential)
// At parse time this is just a wrapper around decimal | nan | infinity.
type V10FloatNode struct {
	v10BaseNode
	Kind  string  // "float32" or "float64"
	Value V10Node // *V10DecimalNode | *V10NanNode | *V10InfinityNode
}

// ---------- Decimal number types ----------

// V10DecimalTypeNode  decimalN = TYPE_OF intN<integer> "." digits
type V10DecimalTypeNode struct {
	v10BaseNode
	Kind     string // "decimal8", "decimal16", …
	Integral *V10IntegerNode
	Frac     string // raw fractional digits
}

// V10DecimalNumNode  decimal_num = decimal8 | … | decimal128
type V10DecimalNumNode struct {
	v10BaseNode
	Value *V10DecimalTypeNode
}

// ---------- CAST directive record ----------

// V10CastDirective records a CAST<T> = A | B | … widening chain.
// Not an AST value node — stored in V10Parser.CastChains.
type V10CastDirective struct {
	Source  string   // e.g. "int8"
	Targets []string // e.g. ["int16","int32","int64","int128"]
}

// ---------- String nodes ----------

// StringKind distinguishes the three quoted string varieties.
type V10StringKind int

const (
	V10StringSingle   V10StringKind = iota // '…'
	V10StringDouble                        // "…"
	V10StringTemplate                      // `…` (may contain $(...) interpolations)
)

// V10StringNode  single_quoted | double_quoted | tmpl_quoted
type V10StringNode struct {
	v10BaseNode
	Kind  V10StringKind
	Raw   string        // verbatim source text including delimiters
	Parts []V10TmplPart // non-nil only for template strings
}

// V10TmplPart is one segment of a template string:
// either literal text or a $(expr) interpolation.
type V10TmplPart struct {
	IsExpr bool   // false → literal text, true → $(...) expression
	Text   string // raw text (when !IsExpr) or inner expression source (when IsExpr)
}

// V10CharNode  char = "'" /(?<value>[\s\S])/ "'"
type V10CharNode struct {
	v10BaseNode
	Value rune
}

// ---------- Date / Time nodes ----------

// V10DateNode  date = date_year [ ["-"] date_month [ ["-"] date_day ] ]
type V10DateNode struct {
	v10BaseNode
	Year  string
	Month string // empty if omitted
	Day   string // empty if omitted
}

// V10TimeNode  time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
type V10TimeNode struct {
	v10BaseNode
	Hour   string
	Minute string
	Second string
	Millis string
}

// V10DateTimeNode  date_time = ( date [ [" "] time ] ) | time
type V10DateTimeNode struct {
	v10BaseNode
	Date *V10DateNode // nil if time-only
	Time *V10TimeNode // nil if date-only
}

// V10TimeStampNode  time_stamp = date [" "] time
type V10TimeStampNode struct {
	v10BaseNode
	Date *V10DateNode
	Time *V10TimeNode
}

// ---------- Duration ----------

// V10DurationSegment is one "quantity unit" pair, e.g. "30" "ms"
type V10DurationSegment struct {
	Digits string
	Unit   string
}

// V10DurationNode  duration = digits duration_unit { digits duration_unit }
type V10DurationNode struct {
	v10BaseNode
	Segments []V10DurationSegment
}

// ---------- Regexp ----------

// V10RegexpNode  regexp = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags ]
type V10RegexpNode struct {
	v10BaseNode
	Pattern string // everything between the slashes
	Flags   string // zero or more flag characters
}

// ---------- Boolean / null ----------

// V10BoolNode  boolean = "true" | "false"
type V10BoolNode struct {
	v10BaseNode
	Value bool
}

// V10NullNode  null = "null"
type V10NullNode struct{ v10BaseNode }

// ---------- Cardinality / range ----------

// V10CardinalityNode  cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
type V10CardinalityNode struct {
	v10BaseNode
	Lo string
	Hi string // raw upper bound token value
}

// V10RangeNode  range = integer ".." integer
type V10RangeNode struct {
	v10BaseNode
	Lo *V10IntegerNode
	Hi *V10IntegerNode
}

// ---------- UUID ----------

// V10UUIDNode  uuid  (standard 8-4-4-4-12)
type V10UUIDNode struct {
	v10BaseNode
	Value string // e.g. "550e8400-e29b-41d4-a716-446655440000"
}

// V10UUIDV7Node  uuid_v7
type V10UUIDV7Node struct {
	v10BaseNode
	Value string
}

// ---------- URL / file path ----------

// V10URLNode  http_url
type V10URLNode struct {
	v10BaseNode
	Value string
}

// V10FilePathNode  file_path
type V10FilePathNode struct {
	v10BaseNode
	Value string
}

// ---------- Top-level constant ----------

// V10ConstantNode  constant = numeric_const | string | char | regexp | boolean | null |
//
//	date | time | date_time | time_stamp | uuid | http_url | file_path
type V10ConstantNode struct {
	v10BaseNode
	Value V10Node
}

// =============================================================================
// PHASE 3 — RECURSIVE-DESCENT PARSER  (01_definitions.sqg)
// =============================================================================

// V10ParseError records a parse failure with location information.
type V10ParseError struct {
	Line    int
	Col     int
	Message string
}

func (e *V10ParseError) Error() string {
	return fmt.Sprintf("L%d:C%d: %s", e.Line, e.Col, e.Message)
}

// V10Parser holds the parser state: the token stream and the current position.
type V10Parser struct {
	tokens []V10Token
	pos    int
	// CastChains stores every CAST<T> = … directive encountered during parsing.
	CastChains []V10CastDirective
}

// NewV10Parser creates a V10Parser from an already-tokenised slice.
// The caller should have run V10Lexer.V10Tokenize() first.
func NewV10Parser(tokens []V10Token) *V10Parser {
	return &V10Parser{tokens: tokens, pos: 0}
}

// ParseV10FromSource is a convenience wrapper: lex + parse in one call.
func ParseV10FromSource(src string) (*V10Parser, error) {
	lex := NewV10Lexer(src)
	toks, err := lex.V10Tokenize()
	if err != nil {
		return nil, err
	}
	return NewV10Parser(toks), nil
}

// --------------------------------------------------------------------------
// Parser helpers
// --------------------------------------------------------------------------

// cur returns the current token (skipping NL tokens).
func (p *V10Parser) cur() V10Token {
	i := p.pos
	for i < len(p.tokens) && p.tokens[i].Type == V10_NL {
		i++
	}
	if i >= len(p.tokens) {
		return V10Token{Type: V10_EOF}
	}
	return p.tokens[i]
}

// curRaw returns the current token WITHOUT skipping NL.
func (p *V10Parser) curRaw() V10Token {
	if p.pos >= len(p.tokens) {
		return V10Token{Type: V10_EOF}
	}
	return p.tokens[p.pos]
}

// advance moves past the current token (NL-aware: skips leading NLs first).
func (p *V10Parser) advance() {
	// Skip any leading NL tokens.
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == V10_NL {
		p.pos++
	}
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// advanceRaw moves past exactly one token regardless of type.
func (p *V10Parser) advanceRaw() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// skipNL advances past all NL tokens at the current position.
func (p *V10Parser) skipNL() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == V10_NL {
		p.pos++
	}
}

// peek returns the token n positions ahead (NL-transparent).
func (p *V10Parser) peek(n int) V10Token {
	count := 0
	i := p.pos
	for i < len(p.tokens) {
		if p.tokens[i].Type != V10_NL {
			if count == n {
				return p.tokens[i]
			}
			count++
		}
		i++
	}
	return V10Token{Type: V10_EOF}
}

// expect consumes the current token if it matches typ; otherwise returns an error.
func (p *V10Parser) expect(typ V10TokenType) (V10Token, error) {
	tok := p.cur()
	if tok.Type != typ {
		return tok, &V10ParseError{
			Line:    tok.Line,
			Col:     tok.Col,
			Message: fmt.Sprintf("expected %s, got %s %q", typ, tok.Type, tok.Value),
		}
	}
	p.advance()
	return tok, nil
}

// expectValue consumes a V10_STRING or V10_INTEGER or V10_DECIMAL token if the
// current token's Value field matches the literal str.
func (p *V10Parser) expectLit(str string) (V10Token, error) {
	tok := p.cur()
	if tok.Value != str {
		return tok, &V10ParseError{
			Line:    tok.Line,
			Col:     tok.Col,
			Message: fmt.Sprintf("expected %q, got %s %q", str, tok.Type, tok.Value),
		}
	}
	p.advance()
	return tok, nil
}

// errAt builds a V10ParseError at the current token's location.
func (p *V10Parser) errAt(msg string) *V10ParseError {
	tok := p.cur()
	return &V10ParseError{Line: tok.Line, Col: tok.Col, Message: msg}
}

// --------------------------------------------------------------------------
// Numeric primitive parsers
// --------------------------------------------------------------------------

// ParseDigits parses the next token as an unsigned digit sequence.
// The lexer always emits unsigned values; this returns the raw string.
func (p *V10Parser) ParseDigits() (string, error) {
	tok := p.cur()
	if tok.Type != V10_INTEGER {
		return "", p.errAt(fmt.Sprintf("expected digits (INTEGER), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return tok.Value, nil
}

// ParseInteger parses:  integer = [ "+" | "-" ] digits
func (p *V10Parser) ParseInteger() (*V10IntegerNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	sign := ""
	if tok.Type == V10_PLUS {
		sign = "+"
		p.advance()
	} else if tok.Type == V10_MINUS {
		sign = "-"
		p.advance()
	}

	digitTok, err := p.expect(V10_INTEGER)
	if err != nil {
		return nil, err
	}
	return &V10IntegerNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Sign:        sign,
		Digits:      digitTok.Value,
	}, nil
}

// ParseDecimal parses:  decimal = [ "+" | "-" ] digits "." digits
// The lexer emits the entire "3.14" as a single V10_DECIMAL token, so we
// split it here for the AST.
func (p *V10Parser) ParseDecimal() (*V10DecimalNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	sign := ""
	if tok.Type == V10_PLUS {
		sign = "+"
		p.advance()
		tok = p.cur()
	} else if tok.Type == V10_MINUS {
		sign = "-"
		p.advance()
		tok = p.cur()
	}

	if tok.Type != V10_DECIMAL {
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
	return &V10DecimalNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Sign:        sign,
		Integral:    integral,
		Frac:        frac,
	}, nil
}

// ParseNumericConst parses:  numeric_const = integer | decimal
// Looks ahead: if a decimal token is present (with optional sign) use ParseDecimal,
// otherwise use ParseInteger.
func (p *V10Parser) ParseNumericConst() (*V10NumericConstNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Determine whether the upcoming value is a decimal.
	isDecSign := tok.Type == V10_PLUS || tok.Type == V10_MINUS
	nextTok := p.peek(1)
	if isDecSign {
		if nextTok.Type == V10_DECIMAL {
			d, err := p.ParseDecimal()
			if err != nil {
				return nil, err
			}
			return &V10NumericConstNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: d}, nil
		}
		// Fall through to integer
		i, err := p.ParseInteger()
		if err != nil {
			return nil, err
		}
		return &V10NumericConstNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: i}, nil
	}

	if tok.Type == V10_DECIMAL {
		d, err := p.ParseDecimal()
		if err != nil {
			return nil, err
		}
		return &V10NumericConstNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: d}, nil
	}

	i, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	return &V10NumericConstNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: i}, nil
}

// --------------------------------------------------------------------------
// NaN / Infinity parsers
// --------------------------------------------------------------------------

// ParseNan parses:  nan = "NaN"
func (p *V10Parser) ParseNan() (*V10NanNode, error) {
	tok := p.cur()
	if tok.Type != V10_NAN {
		return nil, p.errAt(fmt.Sprintf("expected NaN, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V10NanNode{v10BaseNode: v10BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// ParseInfinity parses:  infinity = "+Infinity" | "-Infinity" | "Infinity"
func (p *V10Parser) ParseInfinity() (*V10InfinityNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	sign := ""

	if tok.Type == V10_PLUS {
		sign = "+"
		p.advance()
		tok = p.cur()
	} else if tok.Type == V10_MINUS {
		sign = "-"
		p.advance()
		tok = p.cur()
	}

	if tok.Type != V10_INFINITY {
		return nil, p.errAt(fmt.Sprintf("expected Infinity, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V10InfinityNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Sign: sign}, nil
}

// --------------------------------------------------------------------------
// CAST directive parser
// Grammar:  CAST<T> = A | B | … ;
// This is a grammar-level directive, not a source-level expression.
// --------------------------------------------------------------------------

// ParseCastDirective parses:  CAST "<" ident ">" "=" ident { "|" ident } ";"
// and appends the result to p.CastChains.
func (p *V10Parser) ParseCastDirective() (*V10CastDirective, error) {
	if _, err := p.expect(V10_CAST); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_LT); err != nil {
		return nil, err
	}
	srcTok, err := p.expect(V10_IDENT)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_EQ); err != nil {
		return nil, err
	}

	var targets []string
	firstTok, err := p.expect(V10_IDENT)
	if err != nil {
		return nil, err
	}
	targets = append(targets, firstTok.Value)

	for p.cur().Type == V10_PIPE {
		p.advance()
		t, err := p.expect(V10_IDENT)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t.Value)
	}

	// Optional EOL / semicolon
	if p.cur().Type == V10_SEMICOLON {
		p.advance()
	}

	cd := &V10CastDirective{Source: srcTok.Value, Targets: targets}
	p.CastChains = append(p.CastChains, *cd)
	return cd, nil
}

// --------------------------------------------------------------------------
// String / char parsers
// --------------------------------------------------------------------------

// ParseString parses:  string = single_quoted | double_quoted | tmpl_quoted
// The raw token value includes delimiters.  For template strings the Parts
// slice is populated by splitting on $( … ) segments.
func (p *V10Parser) ParseString() (*V10StringNode, error) {
	tok := p.cur()
	if tok.Type != V10_STRING && tok.Type != V10_EMPTY_STR_D &&
		tok.Type != V10_EMPTY_STR_S && tok.Type != V10_EMPTY_STR_T {
		return nil, p.errAt(fmt.Sprintf("expected string literal, got %s %q", tok.Type, tok.Value))
	}

	line, col := tok.Line, tok.Col
	p.advance()

	raw := tok.Value
	var kind V10StringKind
	var parts []V10TmplPart

	switch {
	case strings.HasPrefix(raw, "'"):
		kind = V10StringSingle
	case strings.HasPrefix(raw, "\""):
		kind = V10StringDouble
	case strings.HasPrefix(raw, "`"):
		kind = V10StringTemplate
		parts = splitTemplateParts(raw)
	default:
		kind = V10StringDouble
	}

	return &V10StringNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Kind:        kind,
		Raw:         raw,
		Parts:       parts,
	}, nil
}

// splitTemplateParts splits the raw template string (including backtick delimiters)
// into literal-text and $(...) interpolation segments.
func splitTemplateParts(raw string) []V10TmplPart {
	// Strip surrounding backticks.
	inner := raw
	if len(inner) >= 2 && inner[0] == '`' {
		inner = inner[1:]
	}
	if len(inner) > 0 && inner[len(inner)-1] == '`' {
		inner = inner[:len(inner)-1]
	}

	var parts []V10TmplPart
	for len(inner) > 0 {
		idx := strings.Index(inner, "$(")
		if idx == -1 {
			parts = append(parts, V10TmplPart{IsExpr: false, Text: inner})
			break
		}
		if idx > 0 {
			parts = append(parts, V10TmplPart{IsExpr: false, Text: inner[:idx]})
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
		parts = append(parts, V10TmplPart{IsExpr: true, Text: exprInner})
		inner = inner[i:]
	}
	return parts
}

// ParseChar parses:  char = "'" /(?<value>[\s\S])/ "'"
// The lexer emits single-quoted strings as V10_STRING.  We validate that the
// content is exactly one Unicode code point.
func (p *V10Parser) ParseChar() (*V10CharNode, error) {
	tok := p.cur()
	if tok.Type != V10_STRING || !strings.HasPrefix(tok.Value, "'") {
		return nil, p.errAt(fmt.Sprintf("expected single-quoted char, got %s %q", tok.Type, tok.Value))
	}
	line, col := tok.Line, tok.Col
	p.advance()

	// Strip surrounding single quotes.
	inner := tok.Value[1 : len(tok.Value)-1]
	// Handle escape: \'  →  '
	if inner == "\\'" {
		return &V10CharNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: '\''}, nil
	}
	runes := []rune(inner)
	if len(runes) != 1 {
		return nil, &V10ParseError{Line: line, Col: col,
			Message: fmt.Sprintf("char literal must be exactly one code point, got %q", inner)}
	}
	return &V10CharNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: runes[0]}, nil
}

// --------------------------------------------------------------------------
// Date / Time parsers
// --------------------------------------------------------------------------

// v10isDigitSeq returns true if the string consists entirely of ASCII digits.
func v10isDigitSeq(s string) bool {
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
// The lexer emits digit sequences as V10_INTEGER tokens.
func (p *V10Parser) ParseDate() (*V10DateNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Year must be exactly 4 digits.
	if tok.Type != V10_INTEGER || len(tok.Value) != 4 || !v10isDigitSeq(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected 4-digit year, got %s %q", tok.Type, tok.Value))
	}
	year := tok.Value
	p.advance()

	node := &V10DateNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Year: year}

	// Optional month (preceded by optional "-")
	if p.cur().Type == V10_MINUS {
		p.advance()
	}
	nextTok := p.cur()
	if !v10isDateMonthOrDay(nextTok) {
		return node, nil
	}
	node.Month = nextTok.Value
	p.advance()

	// Optional day
	if p.cur().Type == V10_MINUS {
		p.advance()
	}
	nextTok = p.cur()
	if !v10isDateMonthOrDay(nextTok) {
		return node, nil
	}
	node.Day = nextTok.Value
	p.advance()

	return node, nil
}

// v10isDateMonthOrDay returns true if tok looks like a 1-2 digit number in range 1..31.
func v10isDateMonthOrDay(tok V10Token) bool {
	if tok.Type != V10_INTEGER {
		return false
	}
	if len(tok.Value) > 2 {
		return false
	}
	return v10isDigitSeq(tok.Value)
}

// ParseTime parses:  time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
func (p *V10Parser) ParseTime() (*V10TimeNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V10_INTEGER || len(tok.Value) > 2 || !v10isDigitSeq(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected hour digits, got %s %q", tok.Type, tok.Value))
	}
	hour := tok.Value
	p.advance()

	node := &V10TimeNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Hour: hour}

	// Optional minute
	if p.cur().Type == V10_COLON {
		p.advance()
	}
	if p.cur().Type != V10_INTEGER || len(p.cur().Value) > 2 {
		return node, nil
	}
	node.Minute = p.cur().Value
	p.advance()

	// Optional second
	if p.cur().Type == V10_COLON {
		p.advance()
	}
	if p.cur().Type != V10_INTEGER || len(p.cur().Value) > 2 {
		return node, nil
	}
	node.Second = p.cur().Value
	p.advance()

	// Optional millis
	if p.cur().Type == V10_DOT {
		p.advance()
	}
	if p.cur().Type != V10_INTEGER || len(p.cur().Value) > 3 {
		return node, nil
	}
	node.Millis = p.cur().Value
	p.advance()

	return node, nil
}

// ParseDateTime parses:  date_time = ( date [ [" "] time ] ) | time
// We try date first; if a time follows (or the whole thing looks time-like), emit accordingly.
func (p *V10Parser) ParseDateTime() (*V10DateTimeNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	node := &V10DateTimeNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	// A 4-digit integer is unambiguously a year; otherwise assume time.
	if tok.Type == V10_INTEGER && len(tok.Value) == 4 {
		date, err := p.ParseDate()
		if err != nil {
			return nil, err
		}
		node.Date = date
		// Optional time component following the date.
		if p.cur().Type == V10_INTEGER {
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
func (p *V10Parser) ParseTimeStamp() (*V10TimeStampNode, error) {
	line, col := p.cur().Line, p.cur().Col

	date, err := p.ParseDate()
	if err != nil {
		return nil, err
	}
	time, err := p.ParseTime()
	if err != nil {
		return nil, err
	}
	return &V10TimeStampNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Date:        date,
		Time:        time,
	}, nil
}

// ParseDuration parses:  duration = digits duration_unit { digits duration_unit }
// e.g. "1h30m", "500ms"
func (p *V10Parser) ParseDuration() (*V10DurationNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V10_INTEGER {
		return nil, p.errAt(fmt.Sprintf("expected digits for duration, got %s %q", tok.Type, tok.Value))
	}

	node := &V10DurationNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	for p.cur().Type == V10_INTEGER {
		digits := p.cur().Value
		p.advance()
		unit, err := p.parseDurationUnit()
		if err != nil {
			return nil, err
		}
		node.Segments = append(node.Segments, V10DurationSegment{Digits: digits, Unit: unit})
	}
	return node, nil
}

// parseDurationUnit consumes one duration_unit token.
func (p *V10Parser) parseDurationUnit() (string, error) {
	tok := p.cur()
	switch tok.Type {
	case V10_MS, V10_SEC, V10_MIN, V10_HR, V10_DAY, V10_WK:
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
func (p *V10Parser) ParseRegexp() (*V10RegexpNode, error) {
	tok := p.cur()
	if tok.Type != V10_REGEXP && tok.Type != V10_REGEXP_DECL {
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
	return &V10RegexpNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Pattern:     pattern,
		Flags:       flags,
	}, nil
}

// --------------------------------------------------------------------------
// Boolean / null
// --------------------------------------------------------------------------

// ParseBoolean parses:  boolean = "true" | "false"
func (p *V10Parser) ParseBoolean() (*V10BoolNode, error) {
	tok := p.cur()
	switch tok.Type {
	case V10_TRUE:
		p.advance()
		return &V10BoolNode{v10BaseNode: v10BaseNode{Line: tok.Line, Col: tok.Col}, Value: true}, nil
	case V10_FALSE:
		p.advance()
		return &V10BoolNode{v10BaseNode: v10BaseNode{Line: tok.Line, Col: tok.Col}, Value: false}, nil
	}
	return nil, p.errAt(fmt.Sprintf("expected boolean (true | false), got %s %q", tok.Type, tok.Value))
}

// ParseNull parses:  null = "null"
func (p *V10Parser) ParseNull() (*V10NullNode, error) {
	tok := p.cur()
	if tok.Type != V10_NULL {
		return nil, p.errAt(fmt.Sprintf("expected null, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V10NullNode{v10BaseNode: v10BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// --------------------------------------------------------------------------
// Cardinality / range
// --------------------------------------------------------------------------

// ParseCardinality parses:  cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
func (p *V10Parser) ParseCardinality() (*V10CardinalityNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	lo, err := p.ParseDigits()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_DOTDOT); err != nil {
		return nil, err
	}

	hiTok := p.cur()
	var hi string
	switch hiTok.Type {
	case V10_INTEGER:
		hi = hiTok.Value
		p.advance()
	case V10_MANY:
		hi = hiTok.Value
		p.advance()
	case V10_IDENT:
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

	return &V10CardinalityNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// ParseRange parses:  range = integer ".." integer
func (p *V10Parser) ParseRange() (*V10RangeNode, error) {
	line, col := p.cur().Line, p.cur().Col

	lo, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_DOTDOT); err != nil {
		return nil, err
	}
	hi, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	return &V10RangeNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// --------------------------------------------------------------------------
// UUID parsers
// --------------------------------------------------------------------------

// v10hexSeg returns true if s is a hex string of exactly n characters.
func v10hexSeg(s string, n int) bool {
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
func (p *V10Parser) ParseUUID() (*V10UUIDNode, error) {
	line, col := p.cur().Line, p.cur().Col

	seg8, err := p.parseHexSeg(8)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_MINUS); err != nil {
		return nil, err
	}
	seg4a, err := p.parseHexSeg(4)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_MINUS); err != nil {
		return nil, err
	}
	seg4b, err := p.parseHexSeg(4)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_MINUS); err != nil {
		return nil, err
	}
	seg4c, err := p.parseHexSeg(4)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_MINUS); err != nil {
		return nil, err
	}
	seg12, err := p.parseHexSeg(12)
	if err != nil {
		return nil, err
	}

	value := seg8 + "-" + seg4a + "-" + seg4b + "-" + seg4c + "-" + seg12
	return &V10UUIDNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: value}, nil
}

// parseHexSeg reads the next token as a hex segment of exactly n characters.
// The token may be V10_INTEGER (all-digit hex) or V10_IDENT (contains a-f).
func (p *V10Parser) parseHexSeg(n int) (string, error) {
	tok := p.cur()
	if tok.Type != V10_INTEGER && tok.Type != V10_IDENT {
		return "", p.errAt(fmt.Sprintf("expected %d-char hex segment, got %s %q", n, tok.Type, tok.Value))
	}
	if !v10hexSeg(tok.Value, n) {
		return "", p.errAt(fmt.Sprintf("expected %d-char hex segment, got %q", n, tok.Value))
	}
	p.advance()
	return tok.Value, nil
}

// --------------------------------------------------------------------------
// URL / file path parsers
// --------------------------------------------------------------------------

// v10urlRe matches http/https URLs.
var v10urlRe = regexp.MustCompile(`^https?://`)

// ParseHTTPURL parses an http_url.  In source text URLs appear as plain STRING
// tokens (quoted) or as unquoted identifier sequences.  We accept either quoting.
func (p *V10Parser) ParseHTTPURL() (*V10URLNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var raw string
	switch tok.Type {
	case V10_STRING:
		raw = tok.Value
		// Strip surrounding quotes.
		if len(raw) >= 2 {
			raw = raw[1 : len(raw)-1]
		}
		p.advance()
	default:
		return nil, p.errAt(fmt.Sprintf("expected quoted http_url, got %s %q", tok.Type, tok.Value))
	}

	if !v10urlRe.MatchString(raw) {
		return nil, &V10ParseError{Line: line, Col: col,
			Message: fmt.Sprintf("invalid http_url: %q", raw)}
	}
	return &V10URLNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: raw}, nil
}

// ParseFilePath parses a file_path (quoted string beginning with / or C:/ etc.).
func (p *V10Parser) ParseFilePath() (*V10FilePathNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V10_STRING {
		return nil, p.errAt(fmt.Sprintf("expected quoted file_path, got %s %q", tok.Type, tok.Value))
	}
	raw := tok.Value
	if len(raw) >= 2 {
		raw = raw[1 : len(raw)-1]
	}
	p.advance()
	return &V10FilePathNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: raw}, nil
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
func (p *V10Parser) ParseConstant() (*V10ConstantNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var inner V10Node
	var err error

	switch tok.Type {
	case V10_REGEXP, V10_REGEXP_DECL:
		inner, err = p.ParseRegexp()
	case V10_TRUE, V10_FALSE:
		inner, err = p.ParseBoolean()
	case V10_NULL:
		inner, err = p.ParseNull()
	case V10_NAN:
		inner, err = p.ParseNan()
	case V10_INFINITY:
		inner, err = p.ParseInfinity()
	case V10_PLUS:
		// Could be +Infinity, +decimal, +integer
		next := p.peek(1)
		if next.Type == V10_INFINITY {
			inner, err = p.ParseInfinity()
		} else if next.Type == V10_DECIMAL {
			inner, err = p.ParseDecimal()
		} else {
			inner, err = p.ParseInteger()
		}
	case V10_MINUS:
		next := p.peek(1)
		if next.Type == V10_INFINITY {
			inner, err = p.ParseInfinity()
		} else if next.Type == V10_DECIMAL {
			inner, err = p.ParseDecimal()
		} else {
			inner, err = p.ParseInteger()
		}
	case V10_STRING, V10_EMPTY_STR_D, V10_EMPTY_STR_S, V10_EMPTY_STR_T:
		inner, err = p.ParseString()
	case V10_DECIMAL:
		inner, err = p.ParseDecimal()
	case V10_INTEGER:
		inner, err = p.ParseInteger()
	default:
		return nil, p.errAt(fmt.Sprintf("unexpected token %s %q in constant", tok.Type, tok.Value))
	}

	if err != nil {
		return nil, err
	}
	return &V10ConstantNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: inner}, nil
}
