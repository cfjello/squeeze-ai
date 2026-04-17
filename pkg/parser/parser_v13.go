// Package parser implements a hand-written recursive-descent parser for the
// Squeeze language (V13), following the spec files in spec/*.sqg.
//
// V13 changes relative to V12:
//   - New directive tokens: HAS_ONE, SUBSET_OF, UNQUOTE, ENUM, BITFIELD
//   - New structure keywords: columns, rows, key_columns, type, root, children,
//     nodes, edges, from, to, label, key, value
//   - New key types: ulid, nano_id, snowflake_id, seq_id*, hash_*, composite_key
//   - Set literals use { } braces (already tokenised in V12 as LBRACE/RBRACE)
//   - string_unquoted / tmpl_unquoted via UNQUOTE directive
//   - sortable / hashable type unions (runtime contracts)
//   - New parser file layout follows spec file structure
package parser

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// =============================================================================
// PHASE 1 — TOKEN TYPES (V13)
// =============================================================================

// V13TokenType identifies the category of a lexed token.
type V13TokenType int

const (
	// ---- Synthetic / control tokens ----
	V13_BOF     V13TokenType = iota // Beginning of file
	V13_EOF                         // End of file
	V13_NL                          // Newline sequence /([ \t]*[\r\n]+)+/ or BOF
	V13_ILLEGAL                     // Unrecognised character

	// ---- Literal value tokens ----
	V13_INTEGER // unsigned digit sequence e.g. 42
	V13_DECIMAL // unsigned decimal e.g. 3.14
	V13_STRING  // single/double/template quoted '…' "…" `…`
	V13_REGEXP  // regexp literal /pattern/flags

	// ---- Identifier tokens ----
	V13_IDENT    // plain identifier
	V13_AT_IDENT // @-prefixed identifier @name

	// ---- Boolean / null keywords ----
	V13_TRUE  // true
	V13_FALSE // false
	V13_NULL  // null

	// ---- Cardinality upper-bound keyword ----
	V13_MANY // many | Many

	// ---- Built-in type keywords ----
	V13_NAN      // NaN
	V13_INFINITY // Infinity | +Infinity | -Infinity

	// ---- Duration-unit keywords ----
	V13_MS  // ms
	V13_SEC // s
	V13_MIN // m
	V13_HR  // h
	V13_DAY // d
	V13_WK  // w

	// ---- Parser directive keywords ----
	V13_UNIQUE     // UNIQUE
	V13_RANGE_KW   // RANGE
	V13_TYPE_OF    // TYPE_OF
	V13_VALUE_OF   // VALUE_OF
	V13_ADDRESS_OF // ADDRESS_OF
	V13_RETURN_DIR // RETURN
	V13_UNIFORM    // UNIFORM
	V13_INFER      // INFER
	V13_CAST       // CAST
	V13_EXTEND     // EXTEND
	V13_MERGE      // MERGE
	V13_SQZ        // SQZ
	V13_CODE       // CODE
	V13_HAS_ONE    // HAS_ONE  (V13 new)
	V13_SUBSET_OF  // SUBSET_OF  (V13 new)
	V13_UNQUOTE    // UNQUOTE  (V13 new)
	V13_PIPELINE   // PIPELINE  (V13 new — library pipeline directive)

	// ---- Structure / type declaration keywords (V13 new) ----
	V13_ENUM     // ENUM
	V13_BITFIELD // BITFIELD

	// ---- Structure field keywords (V13 new) ----
	// These are lexed as V13_IDENT; the parser recognises them by value.
	// Listed here for documentation only — no distinct token type.

	// ---- Multi-character operators (max-munch) ----
	V13_DOTDOT        // ..
	V13_ELLIPSIS      // ...
	V13_DOTDOLLAR     // .$
	V13_ARROW         // ->
	V13_STREAM        // >>
	V13_PUSH          // ~>
	V13_STORE         // =>
	V13_RETURN_STMT   // <-
	V13_INC           // ++
	V13_DEC           // --
	V13_POW           // **
	V13_IADD_IMM      // +:
	V13_ISUB_IMM      // -:
	V13_IMUL_IMM      // *:
	V13_IDIV_IMM      // /:
	V13_IADD_MUT      // +~
	V13_ISUB_MUT      // -~
	V13_IMUL_MUT      // *~
	V13_IDIV_MUT      // /~
	V13_READONLY      // :~
	V13_EXTEND_ASSIGN // +=
	V13_ISUB_ASSIGN   // -=
	V13_IMUL_ASSIGN   // *=
	V13_IDIV_ASSIGN   // /=
	V13_GEQ           // >=
	V13_LEQ           // <=
	V13_NEQ           // !=
	V13_EQEQ          // ==
	V13_MATCH_OP      // =~
	V13_VALIDATE_OP   // ><
	V13_AMP_AMP       // &&
	V13_PIPE_PIPE     // ||
	V13_EMPTY_ARR     // []
	V13_EMPTY_OBJ     // {}
	V13_EMPTY_STR_D   // ""
	V13_EMPTY_STR_S   // ''
	V13_EMPTY_STR_T   // ``
	V13_REGEXP_DECL   // //

	// ---- Single-character operators and punctuation ----
	V13_PLUS      // +
	V13_MINUS     // -
	V13_STAR      // *
	V13_SLASH     // /
	V13_PERCENT   // %
	V13_TILDE     // ~
	V13_COLON     // :
	V13_EQ        // =
	V13_GT        // >
	V13_LT        // <
	V13_BANG      // !
	V13_AMP       // &
	V13_PIPE      // |
	V13_CARET     // ^
	V13_DOT       // .
	V13_COMMA     // ,
	V13_SEMICOLON // ;
	V13_LPAREN    // (
	V13_RPAREN    // )
	V13_LBRACKET  // [
	V13_RBRACKET  // ]
	V13_LBRACE    // {
	V13_RBRACE    // }
	V13_DOLLAR    // $
	V13_SECTION   // §
	V13_AT        // @
	V13_QUESTION  // ?
	V13_ANY_TYPE  // @?
)

// V13tokenNames maps each V13TokenType to its display name.
var V13tokenNames = map[V13TokenType]string{
	V13_BOF: "BOF", V13_EOF: "EOF", V13_NL: "NL", V13_ILLEGAL: "ILLEGAL",
	V13_INTEGER: "INTEGER", V13_DECIMAL: "DECIMAL", V13_STRING: "STRING", V13_REGEXP: "REGEXP",
	V13_IDENT: "IDENT", V13_AT_IDENT: "AT_IDENT",
	V13_TRUE: "true", V13_FALSE: "false", V13_NULL: "null", V13_MANY: "many",
	V13_NAN: "NaN", V13_INFINITY: "Infinity",
	V13_MS: "ms", V13_SEC: "s", V13_MIN: "m", V13_HR: "h", V13_DAY: "d", V13_WK: "w",
	V13_UNIQUE: "UNIQUE", V13_RANGE_KW: "RANGE", V13_TYPE_OF: "TYPE_OF",
	V13_VALUE_OF: "VALUE_OF", V13_ADDRESS_OF: "ADDRESS_OF",
	V13_RETURN_DIR: "RETURN", V13_UNIFORM: "UNIFORM", V13_INFER: "INFER",
	V13_CAST: "CAST", V13_EXTEND: "EXTEND", V13_MERGE: "MERGE",
	V13_SQZ: "SQZ", V13_CODE: "CODE",
	V13_HAS_ONE: "HAS_ONE", V13_SUBSET_OF: "SUBSET_OF", V13_UNQUOTE: "UNQUOTE", V13_PIPELINE: "PIPELINE",
	V13_ENUM: "ENUM", V13_BITFIELD: "BITFIELD",
	V13_DOTDOT: "..", V13_ELLIPSIS: "...", V13_DOTDOLLAR: ".$", V13_ARROW: "->", V13_STREAM: ">>", V13_PUSH: "~>",
	V13_STORE: "=>", V13_RETURN_STMT: "<-",
	V13_INC: "++", V13_DEC: "--", V13_POW: "**",
	V13_IADD_IMM: "+:", V13_ISUB_IMM: "-:", V13_IMUL_IMM: "*:", V13_IDIV_IMM: "/:",
	V13_IADD_MUT: "+~", V13_ISUB_MUT: "-~", V13_IMUL_MUT: "*~", V13_IDIV_MUT: "/~",
	V13_READONLY: ":~", V13_EXTEND_ASSIGN: "+=", V13_ISUB_ASSIGN: "-=",
	V13_IMUL_ASSIGN: "*=", V13_IDIV_ASSIGN: "/=",
	V13_GEQ: ">=", V13_LEQ: "<=", V13_NEQ: "!=", V13_EQEQ: "==", V13_MATCH_OP: "=~", V13_VALIDATE_OP: "><",
	V13_AMP_AMP: "&&", V13_PIPE_PIPE: "||",
	V13_EMPTY_ARR: "[]", V13_EMPTY_OBJ: "{}", V13_EMPTY_STR_D: `""`,
	V13_EMPTY_STR_S: "''", V13_EMPTY_STR_T: "``", V13_REGEXP_DECL: "//",
	V13_PLUS: "+", V13_MINUS: "-", V13_STAR: "*", V13_SLASH: "/", V13_PERCENT: "%",
	V13_TILDE: "~", V13_COLON: ":", V13_EQ: "=", V13_GT: ">", V13_LT: "<",
	V13_BANG: "!", V13_AMP: "&", V13_PIPE: "|", V13_CARET: "^",
	V13_DOT: ".", V13_COMMA: ",", V13_SEMICOLON: ";",
	V13_LPAREN: "(", V13_RPAREN: ")", V13_LBRACKET: "[",
	V13_RBRACKET: "]", V13_LBRACE: "{", V13_RBRACE: "}",
	V13_DOLLAR: "$", V13_SECTION: "§", V13_AT: "@", V13_QUESTION: "?",
	V13_ANY_TYPE: "@?",
}

// String returns the display name of a V13TokenType.
func (t V13TokenType) String() string {
	if s, ok := V13tokenNames[t]; ok {
		return s
	}
	return fmt.Sprintf("V13TOKEN(%d)", int(t))
}

// =============================================================================
// PHASE 1 — TOKEN (V13)
// =============================================================================

// V13Token represents a single lexeme produced by the V13 lexer.
type V13Token struct {
	Type  V13TokenType
	Value string
	Line  int
	Col   int
}

// String returns a compact, readable representation for diagnostics.
func (t V13Token) String() string {
	return fmt.Sprintf("V13Token{%s %q L%d:C%d}", t.Type, t.Value, t.Line, t.Col)
}

// =============================================================================
// PHASE 1 — LEXER (V13)
// =============================================================================

// V13keywords maps literal source strings to their corresponding V13TokenType.
var V13keywords = map[string]V13TokenType{
	"true":       V13_TRUE,
	"false":      V13_FALSE,
	"null":       V13_NULL,
	"many":       V13_MANY,
	"Many":       V13_MANY,
	"NaN":        V13_NAN,
	"Infinity":   V13_INFINITY,
	"ms":         V13_MS,
	"UNIQUE":     V13_UNIQUE,
	"RANGE":      V13_RANGE_KW,
	"TYPE_OF":    V13_TYPE_OF,
	"VALUE_OF":   V13_VALUE_OF,
	"ADDRESS_OF": V13_ADDRESS_OF,
	"RETURN":     V13_RETURN_DIR,
	"UNIFORM":    V13_UNIFORM,
	"INFER":      V13_INFER,
	"CAST":       V13_CAST,
	"EXTEND":     V13_EXTEND,
	"MERGE":      V13_MERGE,
	"SQZ":        V13_SQZ,
	"CODE":       V13_CODE,
	"HAS_ONE":    V13_HAS_ONE,
	"SUBSET_OF":  V13_SUBSET_OF,
	"UNQUOTE":    V13_UNQUOTE,
	"PIPELINE":   V13_PIPELINE,
	"ENUM":       V13_ENUM,
	"BITFIELD":   V13_BITFIELD,
}

// V13durationUnits are single-character duration unit keywords.
var V13durationUnits = map[string]V13TokenType{
	"s": V13_SEC,
	"m": V13_MIN,
	"h": V13_HR,
	"d": V13_DAY,
	"w": V13_WK,
}

// V13prevIsValue returns true when tok is a "value-ending" token —
// i.e. the previous token closes a value expression, meaning a following '/'
// is division rather than the start of a regexp literal.
func V13prevIsValue(t V13TokenType) bool {
	switch t {
	case V13_INTEGER, V13_DECIMAL, V13_STRING, V13_REGEXP,
		V13_IDENT, V13_AT_IDENT, V13_TRUE, V13_FALSE, V13_NULL,
		V13_NAN, V13_INFINITY,
		V13_RPAREN, V13_RBRACKET, V13_RBRACE,
		V13_INC, V13_DEC,
		V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T,
		V13_EMPTY_ARR, V13_EMPTY_OBJ:
		return true
	}
	return false
}

// V13Lexer holds the mutable state of the V13 scanner.
type V13Lexer struct {
	input []rune
	pos   int
	line  int
	col   int
	last  V13TokenType
}

// NewV13Lexer constructs a fresh V13Lexer for the given source string.
func NewV13Lexer(src string) *V13Lexer {
	return &V13Lexer{
		input: []rune(src),
		pos:   0,
		line:  1,
		col:   0,
		last:  V13_BOF,
	}
}

// V13Tokenize scans the entire input and returns the full token slice.
func (l *V13Lexer) V13Tokenize() ([]V13Token, error) {
	tokens := []V13Token{l.V13makeTok(V13_BOF, "", 1, 0)}

	for {
		tok, err := l.V13scan()
		if err != nil {
			return tokens, err
		}
		if tok.Type != V13_NL {
			l.last = tok.Type
		}
		tokens = append(tokens, tok)
		if tok.Type == V13_EOF {
			break
		}
	}
	return tokens, nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

func (l *V13Lexer) V13peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *V13Lexer) V13peek2() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *V13Lexer) V13peek3() rune {
	if l.pos+2 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+2]
}

func (l *V13Lexer) V13advance() rune {
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

func (l *V13Lexer) V13skipHorizWS() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' {
			l.V13advance()
		} else {
			break
		}
	}
}

func (l *V13Lexer) V13makeTok(typ V13TokenType, val string, line, col int) V13Token {
	return V13Token{Type: typ, Value: val, Line: line, Col: col}
}

// --------------------------------------------------------------------------
// scan — top-level dispatch
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scan() (V13Token, error) {
	l.V13skipHorizWS()

	if l.pos >= len(l.input) {
		return l.V13makeTok(V13_EOF, "", l.line, l.col), nil
	}

	line, col := l.line, l.col
	ch := l.V13peek()

	if ch == '\r' || ch == '\n' {
		return l.V13scanNL(line, col)
	}
	if ch == '(' && l.V13peek2() == '*' {
		return l.V13scanComment(line, col)
	}
	if unicode.IsLetter(ch) || ch == '_' {
		return l.V13scanIdentOrKeyword(line, col), nil
	}
	if ch == '@' {
		return l.V13scanAtIdent(line, col)
	}
	if unicode.IsDigit(ch) {
		return l.V13scanNumber(line, col), nil
	}
	switch ch {
	case '"':
		return l.V13scanDoubleQuoted(line, col)
	case '\'':
		return l.V13scanSingleQuoted(line, col)
	case '`':
		return l.V13scanTemplateQuoted(line, col)
	}
	return l.V13scanOperator(line, col)
}

// --------------------------------------------------------------------------
// Newline scanner
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanNL(line, col int) (V13Token, error) {
	var sb strings.Builder
	for l.pos < len(l.input) {
		wsStart := l.pos
		for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t') {
			l.V13advance()
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
			ch := l.V13advance()
			sb.WriteRune(ch)
			if ch == '\r' && l.pos < len(l.input) && l.input[l.pos] == '\n' {
				sb.WriteRune(l.V13advance())
			}
			consumed = true
		}
		if !consumed {
			break
		}
	}
	return l.V13makeTok(V13_NL, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Nested comment scanner
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanComment(line, col int) (V13Token, error) {
	depth := 0
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.V13peek()
		ch2 := l.V13peek2()
		if ch == '(' && ch2 == '*' {
			depth++
			sb.WriteRune(l.V13advance())
			sb.WriteRune(l.V13advance())
			continue
		}
		if ch == '*' && ch2 == ')' {
			sb.WriteRune(l.V13advance())
			sb.WriteRune(l.V13advance())
			depth--
			if depth == 0 {
				return l.V13makeTok(V13_NL, sb.String(), line, col), nil
			}
			continue
		}
		sb.WriteRune(l.V13advance())
	}
	return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated comment", line, col)
}

// --------------------------------------------------------------------------
// Identifier / keyword scanner
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanIdentOrKeyword(line, col int) V13Token {
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.V13advance())
		} else {
			break
		}
	}
	value := sb.String()
	if tt, ok := V13keywords[value]; ok {
		return l.V13makeTok(tt, value, line, col)
	}
	if tt, ok := V13durationUnits[value]; ok {
		return l.V13makeTok(tt, value, line, col)
	}
	return l.V13makeTok(V13_IDENT, value, line, col)
}

// --------------------------------------------------------------------------
// @-prefixed identifier scanner
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanAtIdent(line, col int) (V13Token, error) {
	l.V13advance() // consume '@'
	if l.pos < len(l.input) && l.input[l.pos] == '?' {
		l.V13advance()
		return l.V13makeTok(V13_ANY_TYPE, "@?", line, col), nil
	}
	if l.pos >= len(l.input) || !unicode.IsLetter(l.input[l.pos]) {
		return l.V13makeTok(V13_AT, "@", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('@')
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.V13advance())
		} else {
			break
		}
	}
	return l.V13makeTok(V13_AT_IDENT, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Number scanner
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanNumber(line, col int) V13Token {
	var sb strings.Builder
	for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
		sb.WriteRune(l.V13advance())
	}
	if l.pos < len(l.input) && l.input[l.pos] == '.' &&
		l.pos+1 < len(l.input) && l.input[l.pos+1] != '.' {
		sb.WriteRune(l.V13advance())
		for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
			sb.WriteRune(l.V13advance())
		}
		return l.V13makeTok(V13_DECIMAL, sb.String(), line, col)
	}
	return l.V13makeTok(V13_INTEGER, sb.String(), line, col)
}

// --------------------------------------------------------------------------
// String scanners
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanDoubleQuoted(line, col int) (V13Token, error) {
	l.V13advance()
	if l.V13peek() == '"' {
		l.V13advance()
		return l.V13makeTok(V13_EMPTY_STR_D, `""`, line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('"')
	for l.pos < len(l.input) {
		ch := l.V13advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.V13advance())
		case ch == '"':
			return l.V13makeTok(V13_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
		}
	}
	return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
}

func (l *V13Lexer) V13scanSingleQuoted(line, col int) (V13Token, error) {
	l.V13advance()
	if l.V13peek() == '\'' {
		l.V13advance()
		return l.V13makeTok(V13_EMPTY_STR_S, "''", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('\'')
	for l.pos < len(l.input) {
		ch := l.V13advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.V13advance())
		case ch == '\'':
			return l.V13makeTok(V13_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
		}
	}
	return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
}

func (l *V13Lexer) V13scanTemplateQuoted(line, col int) (V13Token, error) {
	l.V13advance()
	if l.V13peek() == '`' {
		l.V13advance()
		return l.V13makeTok(V13_EMPTY_STR_T, "``", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('`')
	depth := 0
	for l.pos < len(l.input) {
		ch := l.V13peek()
		ch2 := l.V13peek2()
		if ch == '§' && ch2 == '(' && depth == 0 {
			sb.WriteRune(l.V13advance())
			sb.WriteRune(l.V13advance())
			depth++
			continue
		}
		if ch == '(' && depth > 0 {
			depth++
			sb.WriteRune(l.V13advance())
			continue
		}
		if ch == ')' && depth > 0 {
			depth--
			sb.WriteRune(l.V13advance())
			continue
		}
		if ch == '`' && depth == 0 {
			sb.WriteRune(l.V13advance())
			return l.V13makeTok(V13_STRING, sb.String(), line, col), nil
		}
		if ch == 0 {
			return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated template string", line, col)
		}
		sb.WriteRune(l.V13advance())
	}
	return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated template string", line, col)
}

// --------------------------------------------------------------------------
// Regexp scanner
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanRegexp(line, col int) (V13Token, error) {
	l.V13advance()
	var sb strings.Builder
	sb.WriteRune('/')
	for l.pos < len(l.input) {
		ch := l.V13advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.V13advance())
		case ch == '/':
			for l.pos < len(l.input) {
				f := l.input[l.pos]
				if f == 'g' || f == 'i' || f == 'm' || f == 's' ||
					f == 'u' || f == 'y' || f == 'x' || f == 'n' || f == 'A' {
					sb.WriteRune(l.V13advance())
				} else {
					break
				}
			}
			return l.V13makeTok(V13_REGEXP, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
		}
	}
	return l.V13makeTok(V13_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
}

// --------------------------------------------------------------------------
// Operator scanner — max-munch
// --------------------------------------------------------------------------

func (l *V13Lexer) V13scanOperator(line, col int) (V13Token, error) {
	ch := l.V13peek()
	ch2 := l.V13peek2()

	switch ch {
	case '.':
		if ch2 == '.' && l.V13peek3() == '.' {
			l.V13advance()
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_ELLIPSIS, "...", line, col), nil
		}
		if ch2 == '.' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_DOTDOT, "..", line, col), nil
		}
		if ch2 == '$' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_DOTDOLLAR, ".$", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_DOT, ".", line, col), nil

	case '-':
		switch ch2 {
		case '>':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_ARROW, "->", line, col), nil
		case '-':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_DEC, "--", line, col), nil
		case ':':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_ISUB_IMM, "-:", line, col), nil
		case '~':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_ISUB_MUT, "-~", line, col), nil
		case '=':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_ISUB_ASSIGN, "-=", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_MINUS, "-", line, col), nil

	case '>':
		switch ch2 {
		case '>':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_STREAM, ">>", line, col), nil
		case '=':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_GEQ, ">=", line, col), nil
		case '<':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_VALIDATE_OP, "><", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_GT, ">", line, col), nil

	case '=':
		switch ch2 {
		case '>':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_STORE, "=>", line, col), nil
		case '~':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_MATCH_OP, "=~", line, col), nil
		case '=':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_EQEQ, "==", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_EQ, "=", line, col), nil

	case '<':
		switch ch2 {
		case '-':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_RETURN_STMT, "<-", line, col), nil
		case '=':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_LEQ, "<=", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_LT, "<", line, col), nil

	case '+':
		switch ch2 {
		case '+':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_INC, "++", line, col), nil
		case ':':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IADD_IMM, "+:", line, col), nil
		case '~':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IADD_MUT, "+~", line, col), nil
		case '=':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_EXTEND_ASSIGN, "+=", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_PLUS, "+", line, col), nil

	case '*':
		switch ch2 {
		case '*':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_POW, "**", line, col), nil
		case ':':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IMUL_IMM, "*:", line, col), nil
		case '~':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IMUL_MUT, "*~", line, col), nil
		case '=':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IMUL_ASSIGN, "*=", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_STAR, "*", line, col), nil

	case '/':
		switch ch2 {
		case '/':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_REGEXP_DECL, "//", line, col), nil
		case ':':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IDIV_IMM, "/:", line, col), nil
		case '~':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IDIV_MUT, "/~", line, col), nil
		case '=':
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_IDIV_ASSIGN, "/=", line, col), nil
		}
		if V13prevIsValue(l.last) {
			l.V13advance()
			return l.V13makeTok(V13_SLASH, "/", line, col), nil
		}
		return l.V13scanRegexp(line, col)

	case ':':
		if ch2 == '~' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_READONLY, ":~", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_COLON, ":", line, col), nil

	case '!':
		if ch2 == '=' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_NEQ, "!=", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_BANG, "!", line, col), nil

	case '&':
		if ch2 == '&' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_AMP_AMP, "&&", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_AMP, "&", line, col), nil

	case '|':
		if ch2 == '|' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_PIPE_PIPE, "||", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_PIPE, "|", line, col), nil

	case '[':
		if ch2 == ']' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_EMPTY_ARR, "[]", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_LBRACKET, "[", line, col), nil

	case '{':
		if ch2 == '}' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_EMPTY_OBJ, "{}", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_LBRACE, "{", line, col), nil

	case '$':
		l.V13advance()
		return l.V13makeTok(V13_DOLLAR, "$", line, col), nil

	case '§':
		l.V13advance()
		return l.V13makeTok(V13_SECTION, "§", line, col), nil

	case '?':
		l.V13advance()
		return l.V13makeTok(V13_QUESTION, "?", line, col), nil

	case '~':
		if ch2 == '>' {
			l.V13advance()
			l.V13advance()
			return l.V13makeTok(V13_PUSH, "~>", line, col), nil
		}
		l.V13advance()
		return l.V13makeTok(V13_TILDE, "~", line, col), nil
	case '%':
		l.V13advance()
		return l.V13makeTok(V13_PERCENT, "%", line, col), nil
	case '^':
		l.V13advance()
		return l.V13makeTok(V13_CARET, "^", line, col), nil
	case ',':
		l.V13advance()
		return l.V13makeTok(V13_COMMA, ",", line, col), nil
	case ';':
		l.V13advance()
		return l.V13makeTok(V13_SEMICOLON, ";", line, col), nil
	case '(':
		l.V13advance()
		return l.V13makeTok(V13_LPAREN, "(", line, col), nil
	case ')':
		l.V13advance()
		return l.V13makeTok(V13_RPAREN, ")", line, col), nil
	case ']':
		l.V13advance()
		return l.V13makeTok(V13_RBRACKET, "]", line, col), nil
	case '}':
		l.V13advance()
		return l.V13makeTok(V13_RBRACE, "}", line, col), nil
	}

	bad := l.V13advance()
	return l.V13makeTok(V13_ILLEGAL, string(bad), line, col),
		fmt.Errorf("L%d:C%d: unexpected character %q", line, col, bad)
}

// =============================================================================
// PHASE 2 — AST NODE TYPES  (01_definitions.sqg)
// =============================================================================

// V13Node is the common interface for all AST nodes.
type V13Node interface {
	V13NodePos() (line, col int)
}

// V13BaseNode carries source position, embedded in every AST node.
type V13BaseNode struct {
	Line int
	Col  int
}

func (n V13BaseNode) V13NodePos() (int, int) { return n.Line, n.Col }

// ---------- Numeric primitives ----------

// V13SignPrefixNode  sign_prefix = [ "+" | "-" ]
type V13SignPrefixNode struct {
	V13BaseNode
	Sign string // "", "+", or "-"
}

// V13IntegerNode  integer = sign_prefix digits
type V13IntegerNode struct {
	V13BaseNode
	SignPrefix *V13SignPrefixNode
	Digits     string
}

// V13DecimalNode  decimal = sign_prefix digits "." digits
type V13DecimalNode struct {
	V13BaseNode
	SignPrefix *V13SignPrefixNode
	Integral   string
	Frac       string
}

// V13NumericConstNode  numeric_const = integer | decimal
type V13NumericConstNode struct {
	V13BaseNode
	Value V13Node // *V13IntegerNode | *V13DecimalNode
}

// ---------- Float nodes ----------

// V13NanNode  nan = "NaN"
type V13NanNode struct{ V13BaseNode }

// V13InfinityNode  infinity = "+Infinity" | "-Infinity" | "Infinity"
type V13InfinityNode struct {
	V13BaseNode
	Sign string
}

// V13FloatNode  float32 | float64
type V13FloatNode struct {
	V13BaseNode
	Kind  string  // "float32" or "float64"
	Value V13Node // *V13DecimalNode | *V13NanNode | *V13InfinityNode
}

// ---------- Decimal number types ----------

// V13DecimalTypeNode  decimalN
type V13DecimalTypeNode struct {
	V13BaseNode
	Kind     string
	Integral *V13IntegerNode
	Frac     string
}

// V13DecimalNumNode  decimal_num = decimal8 | … | decimal128
type V13DecimalNumNode struct {
	V13BaseNode
	Value *V13DecimalTypeNode
}

// ---------- CAST directive record ----------

// V13CastDirective records a CAST<T> = A | B | … widening chain.
type V13CastDirective struct {
	Source  string
	Targets []string
}

// ---------- String nodes ----------

// V13StringKind distinguishes the three quoted string varieties.
type V13StringKind int

const (
	V13StringSingle   V13StringKind = iota // '…'
	V13StringDouble                        // "…"
	V13StringTemplate                      // `…`
)

// V13TmplPart is one segment of a template string.
// SlotIdx is -1 for literal segments and for mode-1/2 expression segments.
// In mode-3 (deferred) templates, SlotIdx holds the zero-based positional
// parameter index for each IsExpr segment.
type V13TmplPart struct {
	IsExpr  bool
	Text    string
	SlotIdx int
}

// V13TmplParam is one positional parameter of a deferred template.
// Name is the raw expression text of the §(…) slot (e.g. "first_name").
// SlotIdx matches the corresponding V13TmplPart.SlotIdx.
type V13TmplParam struct {
	Name    string
	SlotIdx int
}

// V13TmplDeferredNode  my_var : <- tmpl_quoted
// Wraps a template string as a callable func_unit whose arguments are bound
// left-to-right to each §(expr) slot in source order.
type V13TmplDeferredNode struct {
	V13BaseNode
	Tmpl   *V13StringNode
	Params []V13TmplParam
}

// V13StringNode  single_quoted | double_quoted | tmpl_quoted
type V13StringNode struct {
	V13BaseNode
	Kind  V13StringKind
	Raw   string
	Parts []V13TmplPart
}

// V13StringQuotedNode  string_quoted = single_quoted | double_quoted
type V13StringQuotedNode struct {
	V13BaseNode
	Value *V13StringNode
}

// V13StringUnquotedNode  string_unquoted = UNQUOTE<string_quoted>
// Inner holds the string value with outer quotes stripped.
type V13StringUnquotedNode struct {
	V13BaseNode
	Inner string
}

// V13TmplUnquotedNode  tmpl_unquoted = UNQUOTE<tmpl_quoted>
type V13TmplUnquotedNode struct {
	V13BaseNode
	Inner string
	Parts []V13TmplPart
}

// V13CharNode  char = "'" /(?<value>[\s\S])/ "'"
type V13CharNode struct {
	V13BaseNode
	Value rune
}

// ---------- Date / Time nodes ----------

// V13DateNode  date = date_year [ ["-"] date_month [ ["-"] date_day ] ]
type V13DateNode struct {
	V13BaseNode
	Year  string
	Month string
	Day   string
}

// V13TimeNode  time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
type V13TimeNode struct {
	V13BaseNode
	Hour   string
	Minute string
	Second string
	Millis string
}

// V13DateTimeNode  date_time = ( date [ [" "] time ] ) | time
type V13DateTimeNode struct {
	V13BaseNode
	Date *V13DateNode
	Time *V13TimeNode
}

// V13TimeStampNode  time_stamp = date [" "] time
type V13TimeStampNode struct {
	V13BaseNode
	Date *V13DateNode
	Time *V13TimeNode
}

// ---------- Duration ----------

// V13DurationSegment  digits duration_unit
type V13DurationSegment struct {
	Digits string
	Unit   string
}

// V13DurationNode  duration = digits duration_unit { digits duration_unit }
type V13DurationNode struct {
	V13BaseNode
	Segments []V13DurationSegment
}

// ---------- Regexp ----------

// V13RegexpNode  regexp = "/" XRegExp "/" [ flags ]
type V13RegexpNode struct {
	V13BaseNode
	Pattern string
	Flags   string
}

// ---------- Boolean / null / any_type ----------

// V13BoolNode  boolean = "true" | "false"
type V13BoolNode struct {
	V13BaseNode
	Value bool
}

// V13NullNode  null = "null"
type V13NullNode struct{ V13BaseNode }

// V13AnyTypeNode  any_type = "@?"
type V13AnyTypeNode struct{ V13BaseNode }

// ---------- Cardinality / range ----------

// V13CardinalityNode  cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
type V13CardinalityNode struct {
	V13BaseNode
	Lo string
	Hi string
}

// V13RangeNode  range = integer ".." integer
type V13RangeNode struct {
	V13BaseNode
	Lo *V13IntegerNode
	Hi *V13IntegerNode
}

// ---------- UUID / key types (V13 new) ----------

// V13UUIDNode  uuid (8-4-4-4-12)
type V13UUIDNode struct {
	V13BaseNode
	Value string
}

// V13UUIDV7Node  uuid_v7
type V13UUIDV7Node struct {
	V13BaseNode
	Value string
}

// V13ULIDNode  ulid  (26-char Crockford base32, time-sortable)
type V13ULIDNode struct {
	V13BaseNode
	Value string
}

// V13NanoIDNode  nano_id  (21-char URL-safe random)
type V13NanoIDNode struct {
	V13BaseNode
	Value string
}

// V13SnowflakeIDNode  snowflake_id  (uint64 distributed sortable)
type V13SnowflakeIDNode struct {
	V13BaseNode
	Value string // raw uint64 digits
}

// V13SeqIDNode  seq_id16 | seq_id32 | seq_id64
type V13SeqIDNode struct {
	V13BaseNode
	Kind  string // "seq_id16" | "seq_id32" | "seq_id64"
	Value string
}

// V13HashKeyNode  hash_md5 | hash_sha1 | hash_sha256 | hash_sha512
type V13HashKeyNode struct {
	V13BaseNode
	Kind  string // "hash_md5" | "hash_sha1" | "hash_sha256" | "hash_sha512"
	Value string
}

// V13CompositeKeyNode  composite_key = "(" array_value "," array_value { "," array_value } ")"
type V13CompositeKeyNode struct {
	V13BaseNode
	Parts []V13Node // ≥2 elements
}

// V13UniqueKeyNode  unique_key = uuid | uuid_v7 | ulid | snowflake_id | nano_id | hash_key | seq_id | composite_key
type V13UniqueKeyNode struct {
	V13BaseNode
	Value V13Node
}

// ---------- URL / file path ----------

// V13URLNode  http_url
type V13URLNode struct {
	V13BaseNode
	Value string
}

// V13FilePathNode  file_path
type V13FilePathNode struct {
	V13BaseNode
	Value string
}

// ---------- Top-level constant ----------

// V13ConstantNode  constant = numeric_const | string | char | regexp | boolean | null |
//
//	date | time | date_time | time_stamp | uuid | http_url | file_path
type V13ConstantNode struct {
	V13BaseNode
	Value V13Node
}

// =============================================================================
// PHASE 3 — RECURSIVE-DESCENT PARSER  (01_definitions.sqg)
// =============================================================================

// V13ParseError records a parse failure with location.
type V13ParseError struct {
	Line    int
	Col     int
	Message string
}

func (e *V13ParseError) Error() string {
	return fmt.Sprintf("L%d:C%d: %s", e.Line, e.Col, e.Message)
}

// V13Parser holds the parser state.
type V13Parser struct {
	tokens     []V13Token
	pos        int
	CastChains []V13CastDirective
}

// NewV13Parser creates a V13Parser from an already-tokenised slice.
func NewV13Parser(tokens []V13Token) *V13Parser {
	return &V13Parser{tokens: tokens, pos: 0}
}

// ParseV13FromSource is a convenience wrapper: lex + parse in one call.
func ParseV13FromSource(src string) (*V13Parser, error) {
	lex := NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return nil, err
	}
	return NewV13Parser(toks), nil
}

// --------------------------------------------------------------------------
// Parser helpers
// --------------------------------------------------------------------------

// cur returns the current token, skipping NL and BOF.
func (p *V13Parser) cur() V13Token {
	i := p.pos
	for i < len(p.tokens) && (p.tokens[i].Type == V13_NL || p.tokens[i].Type == V13_BOF) {
		i++
	}
	if i >= len(p.tokens) {
		return V13Token{Type: V13_EOF}
	}
	return p.tokens[i]
}

// curRaw returns the current token without skipping NL.
func (p *V13Parser) curRaw() V13Token {
	if p.pos >= len(p.tokens) {
		return V13Token{Type: V13_EOF}
	}
	return p.tokens[p.pos]
}

// advance moves past the current token (NL/BOF-aware).
func (p *V13Parser) advance() {
	for p.pos < len(p.tokens) && (p.tokens[p.pos].Type == V13_NL || p.tokens[p.pos].Type == V13_BOF) {
		p.pos++
	}
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// advanceRaw moves past exactly one token.
func (p *V13Parser) advanceRaw() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// skipNL advances past all NL tokens at the current position.
func (p *V13Parser) skipNL() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == V13_NL {
		p.pos++
	}
}

// peek returns the token n positions ahead (NL-transparent).
func (p *V13Parser) peek(n int) V13Token {
	count := 0
	i := p.pos
	for i < len(p.tokens) {
		if p.tokens[i].Type != V13_NL {
			if count == n {
				return p.tokens[i]
			}
			count++
		}
		i++
	}
	return V13Token{Type: V13_EOF}
}

// savePos returns a snapshot of the current position for backtracking.
func (p *V13Parser) savePos() int { return p.pos }

// restorePos restores a previously saved position.
func (p *V13Parser) restorePos(saved int) { p.pos = saved }

// expect consumes the current token if it matches typ; otherwise returns error.
func (p *V13Parser) expect(typ V13TokenType) (V13Token, error) {
	tok := p.cur()
	if tok.Type != typ {
		return tok, &V13ParseError{
			Line:    tok.Line,
			Col:     tok.Col,
			Message: fmt.Sprintf("expected %s, got %s %q", typ, tok.Type, tok.Value),
		}
	}
	p.advance()
	return tok, nil
}

// expectLit consumes the current token if its Value matches str.
func (p *V13Parser) expectLit(str string) (V13Token, error) {
	tok := p.cur()
	if tok.Value != str {
		return tok, &V13ParseError{
			Line:    tok.Line,
			Col:     tok.Col,
			Message: fmt.Sprintf("expected %q, got %s %q", str, tok.Type, tok.Value),
		}
	}
	p.advance()
	return tok, nil
}

// errAt builds a V13ParseError at the current token's location.
func (p *V13Parser) errAt(msg string) *V13ParseError {
	tok := p.cur()
	return &V13ParseError{Line: tok.Line, Col: tok.Col, Message: msg}
}

// --------------------------------------------------------------------------
// Numeric primitive parsers
// --------------------------------------------------------------------------

// ParseDigits parses an unsigned digit sequence.
func (p *V13Parser) ParseDigits() (string, error) {
	tok := p.cur()
	if tok.Type != V13_INTEGER {
		return "", p.errAt(fmt.Sprintf("expected digits (INTEGER), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return tok.Value, nil
}

// ParseSignPrefix parses:  sign_prefix = [ "+" | "-" ]
// Always succeeds — returns empty Sign when no sign token is present.
func (p *V13Parser) ParseSignPrefix() *V13SignPrefixNode {
	tok := p.cur()
	node := &V13SignPrefixNode{V13BaseNode: V13BaseNode{Line: tok.Line, Col: tok.Col}}
	if tok.Type == V13_PLUS {
		node.Sign = "+"
		p.advance()
	} else if tok.Type == V13_MINUS {
		node.Sign = "-"
		p.advance()
	}
	return node
}

// ParseInteger parses:  integer = sign_prefix digits
func (p *V13Parser) ParseInteger() (*V13IntegerNode, error) {
	line, col := p.cur().Line, p.cur().Col
	sp := p.ParseSignPrefix()
	digitTok, err := p.expect(V13_INTEGER)
	if err != nil {
		return nil, err
	}
	return &V13IntegerNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		SignPrefix:  sp,
		Digits:      digitTok.Value,
	}, nil
}

// ParseDecimal parses:  decimal = sign_prefix digits "." digits
func (p *V13Parser) ParseDecimal() (*V13DecimalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	sp := p.ParseSignPrefix()
	tok := p.cur()
	if tok.Type != V13_DECIMAL {
		return nil, p.errAt(fmt.Sprintf("expected decimal literal, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	parts := strings.SplitN(tok.Value, ".", 2)
	integral := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	return &V13DecimalNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		SignPrefix:  sp,
		Integral:    integral,
		Frac:        frac,
	}, nil
}

// ParseNumericConst parses:  numeric_const = integer | decimal
func (p *V13Parser) ParseNumericConst() (*V13NumericConstNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	isSign := tok.Type == V13_PLUS || tok.Type == V13_MINUS
	if isSign {
		next := p.peek(1)
		if next.Type == V13_DECIMAL {
			d, err := p.ParseDecimal()
			if err != nil {
				return nil, err
			}
			return &V13NumericConstNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: d}, nil
		}
		i, err := p.ParseInteger()
		if err != nil {
			return nil, err
		}
		return &V13NumericConstNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: i}, nil
	}

	if tok.Type == V13_DECIMAL {
		d, err := p.ParseDecimal()
		if err != nil {
			return nil, err
		}
		return &V13NumericConstNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: d}, nil
	}

	i, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	return &V13NumericConstNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: i}, nil
}

// --------------------------------------------------------------------------
// NaN / Infinity parsers
// --------------------------------------------------------------------------

// ParseNan parses:  nan = "NaN"
func (p *V13Parser) ParseNan() (*V13NanNode, error) {
	tok := p.cur()
	if tok.Type != V13_NAN {
		return nil, p.errAt(fmt.Sprintf("expected NaN, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13NanNode{V13BaseNode: V13BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// ParseInfinity parses:  infinity = "+Infinity" | "-Infinity" | "Infinity"
func (p *V13Parser) ParseInfinity() (*V13InfinityNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	sign := ""
	if tok.Type == V13_PLUS {
		sign = "+"
		p.advance()
		tok = p.cur()
	} else if tok.Type == V13_MINUS {
		sign = "-"
		p.advance()
		tok = p.cur()
	}
	if tok.Type != V13_INFINITY {
		return nil, p.errAt(fmt.Sprintf("expected Infinity, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13InfinityNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Sign: sign}, nil
}

// --------------------------------------------------------------------------
// CAST directive parser
// --------------------------------------------------------------------------

// ParseCastDirective parses:  CAST "<" ident ">" "=" ident { "|" ident }
func (p *V13Parser) ParseCastDirective() (*V13CastDirective, error) {
	if _, err := p.expect(V13_CAST); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	srcTok, err := p.expect(V13_IDENT)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_EQ); err != nil {
		return nil, err
	}
	var targets []string
	firstTok, err := p.expect(V13_IDENT)
	if err != nil {
		return nil, err
	}
	targets = append(targets, firstTok.Value)
	for p.cur().Type == V13_PIPE {
		p.advance()
		t, err := p.expect(V13_IDENT)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t.Value)
	}
	if p.cur().Type == V13_SEMICOLON {
		p.advance()
	}
	cd := &V13CastDirective{Source: srcTok.Value, Targets: targets}
	p.CastChains = append(p.CastChains, *cd)
	return cd, nil
}

// --------------------------------------------------------------------------
// String / char parsers
// --------------------------------------------------------------------------

// v13splitTemplateParts splits raw template string into literal and $() segments.
func v13splitTemplateParts(raw string) []V13TmplPart {
	inner := raw
	if len(inner) >= 2 && inner[0] == '`' {
		inner = inner[1:]
	}
	if len(inner) > 0 && inner[len(inner)-1] == '`' {
		inner = inner[:len(inner)-1]
	}
	var parts []V13TmplPart
	for len(inner) > 0 {
		idx := strings.Index(inner, "§(")
		if idx == -1 {
			parts = append(parts, V13TmplPart{IsExpr: false, Text: inner, SlotIdx: -1})
			break
		}
		if idx > 0 {
			parts = append(parts, V13TmplPart{IsExpr: false, Text: inner[:idx], SlotIdx: -1})
		}
		const sectParenLen = 3 // §( is 2 UTF-8 bytes for § plus 1 byte for (
		depth := 1
		i := idx + sectParenLen
		for i < len(inner) && depth > 0 {
			if inner[i] == '(' {
				depth++
			} else if inner[i] == ')' {
				depth--
			}
			i++
		}
		parts = append(parts, V13TmplPart{IsExpr: true, Text: inner[idx+sectParenLen : i-1], SlotIdx: -1})
		inner = inner[i:]
	}
	return parts
}

// ParseString parses:  string = single_quoted | double_quoted | tmpl_quoted
func (p *V13Parser) ParseString() (*V13StringNode, error) {
	tok := p.cur()
	if tok.Type != V13_STRING && tok.Type != V13_EMPTY_STR_D &&
		tok.Type != V13_EMPTY_STR_S && tok.Type != V13_EMPTY_STR_T {
		return nil, p.errAt(fmt.Sprintf("expected string literal, got %s %q", tok.Type, tok.Value))
	}
	line, col := tok.Line, tok.Col
	p.advance()

	raw := tok.Value
	var kind V13StringKind
	var parts []V13TmplPart
	switch {
	case strings.HasPrefix(raw, "'"):
		kind = V13StringSingle
	case strings.HasPrefix(raw, "`"):
		kind = V13StringTemplate
		parts = v13splitTemplateParts(raw)
	default:
		kind = V13StringDouble
	}
	return &V13StringNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Kind:        kind,
		Raw:         raw,
		Parts:       parts,
	}, nil
}

// ParseStringQuoted parses:  string_quoted = single_quoted | double_quoted
func (p *V13Parser) ParseStringQuoted() (*V13StringQuotedNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	isQuoted := (tok.Type == V13_STRING && !strings.HasPrefix(tok.Value, "`")) ||
		tok.Type == V13_EMPTY_STR_S || tok.Type == V13_EMPTY_STR_D
	if !isQuoted {
		return nil, p.errAt(fmt.Sprintf("expected single- or double-quoted string, got %s %q", tok.Type, tok.Value))
	}
	inner, err := p.ParseString()
	if err != nil {
		return nil, err
	}
	if inner.Kind == V13StringTemplate {
		return nil, &V13ParseError{Line: line, Col: col, Message: "string_quoted: template string not allowed here"}
	}
	return &V13StringQuotedNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: inner}, nil
}

// ParseStringUnquoted parses:  string_unquoted = UNQUOTE<string_quoted>
// Returns the inner string value with outer quotes stripped.
func (p *V13Parser) ParseStringUnquoted() (*V13StringUnquotedNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	sq, err := p.ParseStringQuoted()
	if err != nil {
		return nil, err
	}
	raw := sq.Value.Raw
	// Strip one layer of surrounding quotes.
	inner := raw
	if len(inner) >= 2 {
		inner = inner[1 : len(inner)-1]
	}
	return &V13StringUnquotedNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Inner:       inner,
	}, nil
}

// ParseTmplUnquoted parses:  tmpl_unquoted = UNQUOTE<tmpl_quoted>
func (p *V13Parser) ParseTmplUnquoted() (*V13TmplUnquotedNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_STRING || !strings.HasPrefix(tok.Value, "`") {
		return nil, p.errAt(fmt.Sprintf("expected template-quoted string, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	raw := tok.Value
	// Strip backticks.
	inner := raw
	if len(inner) >= 2 {
		inner = inner[1 : len(inner)-1]
	}
	parts := v13splitTemplateParts(raw)
	return &V13TmplUnquotedNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Inner:       inner,
		Parts:       parts,
	}, nil
}

// ParseChar parses:  char = "'" /(?<value>[\s\S])/ "'"
func (p *V13Parser) ParseChar() (*V13CharNode, error) {
	tok := p.cur()
	if tok.Type != V13_STRING || !strings.HasPrefix(tok.Value, "'") {
		return nil, p.errAt(fmt.Sprintf("expected single-quoted char, got %s %q", tok.Type, tok.Value))
	}
	line, col := tok.Line, tok.Col
	p.advance()
	inner := tok.Value[1 : len(tok.Value)-1]
	if inner == "\\'" {
		return &V13CharNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: '\''}, nil
	}
	runes := []rune(inner)
	if len(runes) != 1 {
		return nil, &V13ParseError{Line: line, Col: col,
			Message: fmt.Sprintf("char literal must be exactly one code point, got %q", inner)}
	}
	return &V13CharNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: runes[0]}, nil
}

// ParseAnyType parses:  any_type = "@?"
func (p *V13Parser) ParseAnyType() (*V13AnyTypeNode, error) {
	tok := p.cur()
	if tok.Type != V13_ANY_TYPE {
		return nil, p.errAt(fmt.Sprintf("expected @?, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13AnyTypeNode{V13BaseNode: V13BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// ParseTmplDeferred parses:  deferred_tmpl = "<-" tmpl_quoted
//
// This is mode-3 template assignment (spec 14.2).  Each §(expr) slot is
// annotated left-to-right with a zero-based SlotIdx and collected as a
// positional parameter in the returned V13TmplDeferredNode.Params slice.
func (p *V13Parser) ParseTmplDeferred() (*V13TmplDeferredNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_RETURN_STMT); err != nil {
		return nil, err
	}
	tok := p.cur()
	if (tok.Type != V13_STRING && tok.Type != V13_EMPTY_STR_T) ||
		!strings.HasPrefix(tok.Value, "`") {
		return nil, p.errAt(fmt.Sprintf("deferred_tmpl: expected template-quoted string after <-, got %s %q", tok.Type, tok.Value))
	}
	tmplNode, err := p.ParseString()
	if err != nil {
		return nil, err
	}
	// Annotate IsExpr parts with sequential SlotIdx values and collect Params.
	var params []V13TmplParam
	slot := 0
	for i := range tmplNode.Parts {
		if tmplNode.Parts[i].IsExpr {
			tmplNode.Parts[i].SlotIdx = slot
			params = append(params, V13TmplParam{Name: tmplNode.Parts[i].Text, SlotIdx: slot})
			slot++
		}
	}
	return &V13TmplDeferredNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Tmpl:        tmplNode,
		Params:      params,
	}, nil
}

// --------------------------------------------------------------------------
// Date / Time parsers
// --------------------------------------------------------------------------

// V13isDigitSeq returns true if the string consists entirely of ASCII digits.
func V13isDigitSeq(s string) bool {
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

// V13isDateMonthOrDay returns true if tok looks like a 1-2 digit number.
func V13isDateMonthOrDay(tok V13Token) bool {
	return tok.Type == V13_INTEGER && len(tok.Value) <= 2 && V13isDigitSeq(tok.Value)
}

// ParseDate parses:  date = date_year [ ["-"] date_month [ ["-"] date_day ] ]
func (p *V13Parser) ParseDate() (*V13DateNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_INTEGER || len(tok.Value) != 4 || !V13isDigitSeq(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected 4-digit year, got %s %q", tok.Type, tok.Value))
	}
	year := tok.Value
	p.advance()
	node := &V13DateNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Year: year}

	if p.cur().Type == V13_MINUS {
		p.advance()
	}
	if !V13isDateMonthOrDay(p.cur()) {
		return node, nil
	}
	node.Month = p.cur().Value
	p.advance()

	if p.cur().Type == V13_MINUS {
		p.advance()
	}
	if !V13isDateMonthOrDay(p.cur()) {
		return node, nil
	}
	node.Day = p.cur().Value
	p.advance()
	return node, nil
}

// ParseTime parses:  time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
func (p *V13Parser) ParseTime() (*V13TimeNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_INTEGER || len(tok.Value) > 2 || !V13isDigitSeq(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected hour digits, got %s %q", tok.Type, tok.Value))
	}
	hour := tok.Value
	p.advance()
	node := &V13TimeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Hour: hour}

	if p.cur().Type == V13_COLON {
		p.advance()
	}
	if tok2 := p.cur(); tok2.Type != V13_INTEGER || len(tok2.Value) > 2 {
		return node, nil
	}
	node.Minute = p.cur().Value
	p.advance()

	if p.cur().Type == V13_COLON {
		p.advance()
	}
	if tok2 := p.cur(); tok2.Type != V13_INTEGER || len(tok2.Value) > 2 {
		return node, nil
	}
	node.Second = p.cur().Value
	p.advance()

	if p.cur().Type == V13_DOT {
		p.advance()
	}
	if tok2 := p.cur(); tok2.Type != V13_INTEGER || len(tok2.Value) > 3 {
		return node, nil
	}
	node.Millis = p.cur().Value
	p.advance()
	return node, nil
}

// ParseDuration parses:  duration = digits duration_unit { digits duration_unit }
func (p *V13Parser) ParseDuration() (*V13DurationNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_INTEGER {
		return nil, p.errAt(fmt.Sprintf("expected digits for duration, got %s %q", tok.Type, tok.Value))
	}
	var segs []V13DurationSegment
	for p.cur().Type == V13_INTEGER {
		digits := p.cur().Value
		p.advance()
		unitTok := p.cur()
		switch unitTok.Type {
		case V13_MS, V13_SEC, V13_MIN, V13_HR, V13_DAY, V13_WK:
			segs = append(segs, V13DurationSegment{Digits: digits, Unit: unitTok.Value})
			p.advance()
		default:
			return nil, p.errAt(fmt.Sprintf("expected duration unit, got %s %q", unitTok.Type, unitTok.Value))
		}
	}
	return &V13DurationNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Segments: segs}, nil
}

// --------------------------------------------------------------------------
// Regexp parser
// --------------------------------------------------------------------------

// ParseRegexp parses:  regexp = "/" XRegExp "/" [ flags ]
func (p *V13Parser) ParseRegexp() (*V13RegexpNode, error) {
	tok := p.cur()
	if tok.Type != V13_REGEXP {
		return nil, p.errAt(fmt.Sprintf("expected regexp literal, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	raw := tok.Value
	// Split /pattern/flags
	if len(raw) < 2 || raw[0] != '/' {
		return nil, &V13ParseError{Line: tok.Line, Col: tok.Col, Message: "malformed regexp token"}
	}
	lastSlash := strings.LastIndex(raw, "/")
	pattern := raw[1:lastSlash]
	flags := raw[lastSlash+1:]
	return &V13RegexpNode{
		V13BaseNode: V13BaseNode{Line: tok.Line, Col: tok.Col},
		Pattern:     pattern,
		Flags:       flags,
	}, nil
}

// --------------------------------------------------------------------------
// UUID parsers
// --------------------------------------------------------------------------

// uuidHexRe matches the standard 8-4-4-4-12 UUID format.
var uuidHexRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// uuidV7VerRe matches the version nibble (must start with '7').
var uuidV7VerRe = regexp.MustCompile(`(?i)^7[0-9a-f]{3}$`)

// uuidV7VarRe matches the variant nibble (must start with 8,9,a,b).
var uuidV7VarRe = regexp.MustCompile(`(?i)^[89ab][0-9a-f]{3}$`)

// ParseUUID parses a UUID string token (validated structurally).
// Grammar: uuid = hex_seg8 "-" hex_seg4 "-" hex_seg4 "-" hex_seg4 "-" hex_seg12
// The lexer emits UUIDs as V13_STRING tokens since they contain hyphens.
func (p *V13Parser) ParseUUID() (*V13UUIDNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	// A UUID will arrive as a STRING token value with quotes, or as IDENT pieces.
	// In practice the lexer does not recognise UUID specially — attempt to match
	// from an unquoted string (the caller strips quotes from string_unquoted context).
	val := tok.Value
	if !uuidHexRe.MatchString(val) {
		return nil, p.errAt(fmt.Sprintf("expected UUID, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13UUIDNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ParseUUIDV7 parses a v7 time-sortable UUID.
func (p *V13Parser) ParseUUIDV7() (*V13UUIDV7Node, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	val := tok.Value
	if !uuidHexRe.MatchString(val) {
		return nil, p.errAt(fmt.Sprintf("expected UUID v7, got %s %q", tok.Type, tok.Value))
	}
	// Version nibble: 3rd segment must start with '7'
	parts := strings.Split(val, "-")
	if len(parts) != 5 || !uuidV7VerRe.MatchString(parts[2]) || !uuidV7VarRe.MatchString(parts[3]) {
		return nil, p.errAt(fmt.Sprintf("expected UUID v7, got %q", val))
	}
	p.advance()
	return &V13UUIDV7Node{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ulidRe matches a 26-character Crockford base32 ULID.
var ulidRe = regexp.MustCompile(`(?i)^[0-7][0-9A-HJKMNP-TV-Z]{9}[0-9A-HJKMNP-TV-Z]{16}$`)

// ParseULID parses a ULID value.
func (p *V13Parser) ParseULID() (*V13ULIDNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	val := tok.Value
	if !ulidRe.MatchString(val) {
		return nil, p.errAt(fmt.Sprintf("expected ULID, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13ULIDNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// nanoIDRe matches a 21-character nano ID.
var nanoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{21}$`)

// ParseNanoID parses a nano_id value.
func (p *V13Parser) ParseNanoID() (*V13NanoIDNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	val := tok.Value
	if !nanoIDRe.MatchString(val) {
		return nil, p.errAt(fmt.Sprintf("expected nano_id, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13NanoIDNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ParseSnowflakeID parses a snowflake_id (uint64 >= 0).
func (p *V13Parser) ParseSnowflakeID() (*V13SnowflakeIDNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_INTEGER {
		return nil, p.errAt(fmt.Sprintf("expected snowflake_id (uint64), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13SnowflakeIDNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: tok.Value}, nil
}

// --------------------------------------------------------------------------
// Boolean / null parsers
// --------------------------------------------------------------------------

// ParseBoolean parses:  boolean = "true" | "false"
func (p *V13Parser) ParseBoolean() (*V13BoolNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	switch tok.Type {
	case V13_TRUE:
		p.advance()
		return &V13BoolNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: true}, nil
	case V13_FALSE:
		p.advance()
		return &V13BoolNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: false}, nil
	}
	return nil, p.errAt(fmt.Sprintf("expected boolean, got %s %q", tok.Type, tok.Value))
}

// ParseNull parses:  null = "null"
func (p *V13Parser) ParseNull() (*V13NullNode, error) {
	tok := p.cur()
	if tok.Type != V13_NULL {
		return nil, p.errAt(fmt.Sprintf("expected null, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13NullNode{V13BaseNode: V13BaseNode{Line: tok.Line, Col: tok.Col}}, nil
}

// --------------------------------------------------------------------------
// Cardinality / range parsers
// --------------------------------------------------------------------------

// ParseCardinality parses:  cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
func (p *V13Parser) ParseCardinality() (*V13CardinalityNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_INTEGER {
		return nil, p.errAt(fmt.Sprintf("expected digits for cardinality, got %s %q", tok.Type, tok.Value))
	}
	lo := tok.Value
	p.advance()
	if _, err := p.expect(V13_DOTDOT); err != nil {
		return nil, err
	}
	hi := ""
	hiTok := p.cur()
	switch hiTok.Type {
	case V13_INTEGER:
		hi = hiTok.Value
		p.advance()
	case V13_MANY:
		hi = hiTok.Value
		p.advance()
	case V13_IDENT:
		if hiTok.Value == "m" || hiTok.Value == "M" || hiTok.Value == "many" || hiTok.Value == "Many" {
			hi = hiTok.Value
			p.advance()
		} else {
			return nil, p.errAt(fmt.Sprintf("expected upper bound for cardinality, got %s %q", hiTok.Type, hiTok.Value))
		}
	default:
		return nil, p.errAt(fmt.Sprintf("expected upper bound for cardinality, got %s %q", hiTok.Type, hiTok.Value))
	}
	return &V13CardinalityNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// ParseRange parses:  range = integer ".." integer
func (p *V13Parser) ParseRange() (*V13RangeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	lo, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_DOTDOT); err != nil {
		return nil, err
	}
	hi, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	return &V13RangeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// --------------------------------------------------------------------------
// Constant parser
// --------------------------------------------------------------------------

// ParseConstant parses:
//
//	constant = numeric_const | string | char | regexp | boolean | null |
//	           date | time | date_time | time_stamp | uuid | http_url | file_path
func (p *V13Parser) ParseConstant() (*V13ConstantNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	switch tok.Type {
	case V13_NAN:
		n, err := p.ParseNan()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: n}, nil

	case V13_TRUE, V13_FALSE:
		b, err := p.ParseBoolean()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: b}, nil

	case V13_NULL:
		n, err := p.ParseNull()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: n}, nil

	case V13_REGEXP:
		r, err := p.ParseRegexp()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: r}, nil

	case V13_STRING, V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T:
		s, err := p.ParseString()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: s}, nil

	case V13_INTEGER:
		// Could be: date_year, time_hour, numeric_const, duration
		saved := p.savePos()
		if node, err := p.ParseNumericConst(); err == nil {
			return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: node}, nil
		}
		p.restorePos(saved)
		return nil, p.errAt(fmt.Sprintf("expected constant, got %s %q", tok.Type, tok.Value))

	case V13_DECIMAL:
		d, err := p.ParseNumericConst()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: d}, nil

	case V13_PLUS, V13_MINUS:
		// Signed numeric or Infinity
		saved := p.savePos()
		if node, err := p.ParseInfinity(); err == nil {
			return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: node}, nil
		}
		p.restorePos(saved)
		nc, err := p.ParseNumericConst()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: nc}, nil

	case V13_INFINITY:
		n, err := p.ParseInfinity()
		if err != nil {
			return nil, err
		}
		return &V13ConstantNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: n}, nil
	}

	return nil, p.errAt(fmt.Sprintf("expected constant, got %s %q", tok.Type, tok.Value))
}
