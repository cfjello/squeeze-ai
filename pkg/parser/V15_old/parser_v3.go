// Package parser implements a hand-written recursive-descent parser for the
// Squeeze language (V3), following the plan in specifications/parser_v3.md.
//
// This file is structured in phases:
//
//	Phase 1 — Token types & Lexer          (this file, complete)
//	Phase 2 — AST node types               (TODO)
//	Phase 3 — Recursive-descent parser     (TODO)
//	Phase 4 — Directive processor          (TODO)
package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// =============================================================================
// PHASE 1 — TOKEN TYPES
// =============================================================================

// TokenType identifies the category of a lexed token.
type TokenType int

const (
	// ---- Synthetic / control tokens ----
	TOK_BOF     TokenType = iota // Beginning of file (first token always emitted)
	TOK_EOF                      // End of file  (last token always emitted)
	TOK_NL                       // Newline sequence  /([ \t]*[\r\n]+)+/
	TOK_ILLEGAL                  // Unrecognised character

	// ---- Literal value tokens ----
	TOK_INTEGER // e.g.  42  -1  +7
	TOK_DECIMAL // e.g.  3.14  -0.5
	TOK_STRING  // single/double/template quoted  '…' "…" `…`
	TOK_REGEXP  // regexp literal  /pattern/flags

	// ---- Identifier tokens ----
	TOK_IDENT    // plain identifier  (Unicode letters, digits, _, embedded spaces)
	TOK_AT_IDENT // @-prefixed identifier  @name  @type  @storeName  …

	// ---- Boolean keywords ----
	TOK_TRUE  // "true"
	TOK_FALSE // "false"

	// ---- Range keyword ----
	TOK_MANY // "many" | "Many"  (used in range upper bound)

	// ---- Parser directive keywords (grammar-level, not in user source) ----
	TOK_UNIQUE     // UNIQUE
	TOK_RANGE_KW   // RANGE  (suffixed _KW to avoid collision with range rule names)
	TOK_TYPE_OF    // TYPE_OF
	TOK_VALUE_OF   // VALUE_OF
	TOK_ADDRESS_OF // ADDRESS_OF
	TOK_RETURN_DIR // RETURN  (directive — distinct from the <- return-statement token)
	TOK_UNIFORM    // UNIFORM
	TOK_INFER      // INFER

	// ---- Multi-character operators (max-munch priority) ----
	TOK_DOTDOT      // ..    range separator
	TOK_ARROW       // ->    function arg separator / data-chain connector
	TOK_STREAM      // >>    stream connector / func_stream_decl
	TOK_STORE       // =>    store / deps operator
	TOK_RETURN_STMT // <-    return statement marker
	TOK_INC         // ++    inline increment
	TOK_DEC         // --    inline decrement
	TOK_POW         // **    exponentiation
	TOK_IADD_IMM    // +:    incremental immutable assign  +
	TOK_ISUB_IMM    // -:    incremental immutable assign  -
	TOK_IMUL_IMM    // *:    incremental immutable assign  *
	TOK_IDIV_IMM    // /:    incremental immutable assign  /
	TOK_IADD_MUT    // +~    incremental mutable assign  +
	TOK_ISUB_MUT    // -~    incremental mutable assign  -
	TOK_IMUL_MUT    // *~    incremental mutable assign  *
	TOK_IDIV_MUT    // /~    incremental mutable assign  /
	TOK_READONLY    // :~    read-only reference assign
	TOK_GEQ         // >=    compare
	TOK_LEQ         // <=    compare
	TOK_NEQ         // !=    compare (not-equal)
	TOK_MATCH_OP    // =~    string-regexp compare
	TOK_EMPTY_ARR   // []    empty array initialiser
	TOK_EMPTY_OBJ   // {}    empty object initialiser
	TOK_EMPTY_STR_D // ""    empty double-quoted string / func_string_decl
	TOK_EMPTY_STR_S // ''    empty single-quoted string / func_string_decl
	TOK_EMPTY_STR_T // ``    empty template-quoted string / func_string_decl
	TOK_REGEXP_DECL // //    empty regexp initialiser / func_regexp_decl

	// ---- Single-character operators and punctuation ----
	TOK_PLUS      // +
	TOK_MINUS     // -
	TOK_STAR      // *
	TOK_SLASH     // /   (division; / beginning a regexp is emitted as TOK_REGEXP)
	TOK_PERCENT   // %
	TOK_TILDE     // ~   mutable assign
	TOK_COLON     // :   immutable assign
	TOK_EQ        // =
	TOK_GT        // >
	TOK_LT        // <
	TOK_BANG      // !   logical not
	TOK_AMP       // &   logical and
	TOK_PIPE      // |   logical or
	TOK_CARET     // ^   logical xor
	TOK_DOT       // .   member access
	TOK_COMMA     // ,
	TOK_SEMICOLON // ;
	TOK_LPAREN    // (
	TOK_RPAREN    // )
	TOK_LBRACKET  // [
	TOK_RBRACKET  // ]
	TOK_LBRACE    // {
	TOK_RBRACE    // }
)

// tokenNames maps each TokenType to its human-readable display name.
var tokenNames = map[TokenType]string{
	TOK_BOF: "BOF", TOK_EOF: "EOF", TOK_NL: "NL", TOK_ILLEGAL: "ILLEGAL",
	TOK_INTEGER: "INTEGER", TOK_DECIMAL: "DECIMAL", TOK_STRING: "STRING", TOK_REGEXP: "REGEXP",
	TOK_IDENT: "IDENT", TOK_AT_IDENT: "AT_IDENT",
	TOK_TRUE: "true", TOK_FALSE: "false", TOK_MANY: "many",
	TOK_UNIQUE: "UNIQUE", TOK_RANGE_KW: "RANGE", TOK_TYPE_OF: "TYPE_OF",
	TOK_VALUE_OF: "VALUE_OF", TOK_ADDRESS_OF: "ADDRESS_OF",
	TOK_RETURN_DIR: "RETURN", TOK_UNIFORM: "UNIFORM", TOK_INFER: "INFER",
	TOK_DOTDOT: "..", TOK_ARROW: "->", TOK_STREAM: ">>", TOK_STORE: "=>",
	TOK_RETURN_STMT: "<-", TOK_INC: "++", TOK_DEC: "--", TOK_POW: "**",
	TOK_IADD_IMM: "+:", TOK_ISUB_IMM: "-:", TOK_IMUL_IMM: "*:", TOK_IDIV_IMM: "/:",
	TOK_IADD_MUT: "+~", TOK_ISUB_MUT: "-~", TOK_IMUL_MUT: "*~", TOK_IDIV_MUT: "/~",
	TOK_READONLY: ":~", TOK_GEQ: ">=", TOK_LEQ: "<=", TOK_NEQ: "!=", TOK_MATCH_OP: "=~",
	TOK_EMPTY_ARR: "[]", TOK_EMPTY_OBJ: "{}", TOK_EMPTY_STR_D: `""`,
	TOK_EMPTY_STR_S: "''", TOK_EMPTY_STR_T: "``", TOK_REGEXP_DECL: "//",
	TOK_PLUS: "+", TOK_MINUS: "-", TOK_STAR: "*", TOK_SLASH: "/", TOK_PERCENT: "%",
	TOK_TILDE: "~", TOK_COLON: ":", TOK_EQ: "=", TOK_GT: ">", TOK_LT: "<",
	TOK_BANG: "!", TOK_AMP: "&", TOK_PIPE: "|", TOK_CARET: "^",
	TOK_DOT: ".", TOK_COMMA: ",", TOK_SEMICOLON: ";",
	TOK_LPAREN: "(", TOK_RPAREN: ")", TOK_LBRACKET: "[",
	TOK_RBRACKET: "]", TOK_LBRACE: "{", TOK_RBRACE: "}",
}

// String returns the display name of a TokenType.
func (t TokenType) String() string {
	if s, ok := tokenNames[t]; ok {
		return s
	}
	return fmt.Sprintf("TOKEN(%d)", int(t))
}

// =============================================================================
// PHASE 1 — TOKEN
// =============================================================================

// Token represents a single lexeme produced by the Lexer.
type Token struct {
	Type  TokenType
	Value string // verbatim source text of the token
	Line  int    // 1-based source line of the first character
	Col   int    // 0-based column of the first character
}

// String returns a compact, readable representation useful for diagnostics.
func (t Token) String() string {
	return fmt.Sprintf("Token{%s %q L%d:C%d}", t.Type, t.Value, t.Line, t.Col)
}

// =============================================================================
// PHASE 1 — LEXER
// =============================================================================

// keywords maps literal source strings to their corresponding reserved TokenType.
// Only exact case-matches are keywords; identifiers that don't appear here stay
// as TOK_IDENT.
var keywords = map[string]TokenType{
	"true":       TOK_TRUE,
	"false":      TOK_FALSE,
	"many":       TOK_MANY,
	"Many":       TOK_MANY,
	"UNIQUE":     TOK_UNIQUE,
	"RANGE":      TOK_RANGE_KW,
	"TYPE_OF":    TOK_TYPE_OF,
	"VALUE_OF":   TOK_VALUE_OF,
	"ADDRESS_OF": TOK_ADDRESS_OF,
	"RETURN":     TOK_RETURN_DIR,
	"UNIFORM":    TOK_UNIFORM,
	"INFER":      TOK_INFER,
}

// prevIsValue reports whether a token type represents a "value" on the right — i.e.
// something that can be the dividend in a numeric expression.  This drives the
// regexp vs. division disambiguation when '/' is encountered.
func prevIsValue(t TokenType) bool {
	switch t {
	case TOK_INTEGER, TOK_DECIMAL, TOK_STRING, TOK_REGEXP,
		TOK_IDENT, TOK_AT_IDENT, TOK_TRUE, TOK_FALSE,
		TOK_RPAREN, TOK_RBRACKET, TOK_RBRACE:
		return true
	}
	return false
}

// Lexer scans a Squeeze source string into a flat slice of Tokens.
//
// Key behaviours (matching the grammar spec):
//   - Horizontal whitespace (space, tab) is silently skipped between tokens.
//   - Newlines are emitted as TOK_NL tokens (EOL in the grammar).
//   - A synthetic TOK_BOF is always the first token; TOK_EOF is always last.
//   - Identifiers may contain embedded spaces: /[\p{L}][\p{L}0-9_ ]*/
//     Trailing spaces are trimmed so they are not included in the token value.
//   - '/' is emitted as TOK_REGEXP when preceded by a non-value token, and as
//     TOK_SLASH (division) when preceded by a value token, integer, or ')'/']'/'}'.
//   - '//' is always emitted as TOK_REGEXP_DECL (never a line comment).
type Lexer struct {
	input []rune    // full source as rune slice for O(1) indexed access
	pos   int       // current rune index into input
	line  int       // current 1-based source line
	col   int       // current 0-based column
	last  TokenType // type of the last emitted non-NL token (for regexp disambiguation)
}

// NewLexer creates a Lexer ready to scan src.
func NewLexer(src string) *Lexer {
	return &Lexer{
		input: []rune(src),
		pos:   0,
		line:  1,
		col:   0,
		last:  TOK_BOF,
	}
}

// Tokenize scans the entire input and returns the full token slice.
// The slice always begins with TOK_BOF and ends with TOK_EOF.
// An error is returned if an illegal character is encountered; partial results
// up to that point are also returned so diagnostics can report context.
func (l *Lexer) Tokenize() ([]Token, error) {
	tokens := []Token{l.makeTok(TOK_BOF, "", 1, 0)}

	for {
		tok, err := l.scan()
		if err != nil {
			return tokens, err
		}
		// Update regexp disambiguation state: only non-NL tokens are "significant".
		if tok.Type != TOK_NL {
			l.last = tok.Type
		}
		tokens = append(tokens, tok)
		if tok.Type == TOK_EOF {
			break
		}
	}
	return tokens, nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// peek returns the rune at the current position without consuming it.
// Returns 0 if at end of input.
func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

// peek2 returns the rune one position ahead of the current position.
// Returns 0 if out of range.
func (l *Lexer) peek2() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

// advance consumes and returns the current rune, updating line/col tracking.
func (l *Lexer) advance() rune {
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

// skipHorizWS consumes spaces and tabs (horizontal whitespace only).
// Newlines are NOT consumed here; they become TOK_NL tokens.
func (l *Lexer) skipHorizWS() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' {
			l.advance()
		} else {
			break
		}
	}
}

// makeTok constructs a Token with the given fields.
func (l *Lexer) makeTok(typ TokenType, val string, line, col int) Token {
	return Token{Type: typ, Value: val, Line: line, Col: col}
}

// --------------------------------------------------------------------------
// scan — top-level dispatch
// --------------------------------------------------------------------------

// scan returns the next token from the current input position.
func (l *Lexer) scan() (Token, error) {
	l.skipHorizWS()

	if l.pos >= len(l.input) {
		return l.makeTok(TOK_EOF, "", l.line, l.col), nil
	}

	line, col := l.line, l.col
	ch := l.peek()

	// Newlines → NL token
	if ch == '\r' || ch == '\n' {
		return l.scanNL(line, col)
	}

	// Unicode letter or underscore → identifier or keyword
	if unicode.IsLetter(ch) || ch == '_' {
		return l.scanIdentOrKeyword(line, col), nil
	}

	// @identifier
	if ch == '@' {
		return l.scanAtIdent(line, col)
	}

	// Digit → number
	if unicode.IsDigit(ch) {
		return l.scanNumber(line, col), nil
	}

	// Quoted strings
	switch ch {
	case '"':
		return l.scanDoubleQuoted(line, col)
	case '\'':
		return l.scanSingleQuoted(line, col)
	case '`':
		return l.scanTemplateQuoted(line, col)
	}

	// Operators and punctuation
	return l.scanOperator(line, col)
}

// --------------------------------------------------------------------------
// Newline scanner
// Implements NL = /([ \t]*[\r\n]+)+/
// --------------------------------------------------------------------------

// scanNL scans one or more newline sequences (with optional interleaved
// horizontal whitespace on blank lines) into a single TOK_NL token.
func (l *Lexer) scanNL(line, col int) (Token, error) {
	var sb strings.Builder

	for l.pos < len(l.input) {
		// Consume optional horizontal whitespace preceding the next newline.
		// We peek ahead first to ensure whitespace is actually followed by \r/\n
		// (otherwise it belongs to the next real token).
		wsStart := l.pos
		for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t') {
			l.advance()
		}
		if l.pos >= len(l.input) || (l.input[l.pos] != '\r' && l.input[l.pos] != '\n') {
			// Whitespace was NOT followed by a newline — put position back and stop.
			l.pos = wsStart
			// (line/col tracking: we didn't advance past any newlines, so col reset
			// is not needed; advance() only changes l.line on '\n'.)
			break
		}
		// Include the consumed whitespace in the token value.
		for i := wsStart; i < l.pos; i++ {
			sb.WriteRune(l.input[i])
		}

		// Consume one or more consecutive newline characters.
		consumed := false
		for l.pos < len(l.input) && (l.input[l.pos] == '\r' || l.input[l.pos] == '\n') {
			ch := l.advance()
			sb.WriteRune(ch)
			if ch == '\r' && l.pos < len(l.input) && l.input[l.pos] == '\n' {
				sb.WriteRune(l.advance()) // consume \n of \r\n pair
			}
			consumed = true
		}
		if !consumed {
			break
		}
	}

	return l.makeTok(TOK_NL, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Identifier / keyword scanner
// Grammar: ident_name = /(?<value>[\p{L}][\p{L}0-9_ ]*)/
// Embedded spaces are valid within identifier names; trailing spaces are trimmed.
// --------------------------------------------------------------------------

func (l *Lexer) scanIdentOrKeyword(line, col int) Token {
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		// Accept Unicode letters, digits, underscores, and SPACE (not tab).
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == ' ' {
			sb.WriteRune(l.advance())
		} else {
			break
		}
	}
	// Trim trailing spaces — they are inter-token whitespace, not part of the name.
	value := strings.TrimRight(sb.String(), " ")

	if tt, ok := keywords[value]; ok {
		return l.makeTok(tt, value, line, col)
	}
	return l.makeTok(TOK_IDENT, value, line, col)
}

// --------------------------------------------------------------------------
// @-prefixed identifier scanner  @name  @type  @storeName  …
// --------------------------------------------------------------------------

func (l *Lexer) scanAtIdent(line, col int) (Token, error) {
	l.advance() // consume '@'
	if l.pos >= len(l.input) || !unicode.IsLetter(l.input[l.pos]) {
		return l.makeTok(TOK_ILLEGAL, "@", line, col),
			fmt.Errorf("L%d:C%d: '@' must be followed by a letter", line, col)
	}
	var sb strings.Builder
	sb.WriteRune('@')
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		// @-identifiers do not allow embedded spaces.
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.advance())
		} else {
			break
		}
	}
	return l.makeTok(TOK_AT_IDENT, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Number scanner  →  TOK_INTEGER | TOK_DECIMAL
// Grammar:
//   digits  = /[0-9]+/
//   integer = [ "+" | "-" ] digits
//   decimal = [ "+" | "-" ] digits "." digits
//
// Note: sign handling (+/-) is done at the parser level (as part of numeric_const).
// The lexer emits unsigned digit sequences; the sign is a separate TOK_PLUS/TOK_MINUS.
// --------------------------------------------------------------------------

func (l *Lexer) scanNumber(line, col int) Token {
	var sb strings.Builder
	for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
		sb.WriteRune(l.advance())
	}
	// Decimal: digits "." digits — but NOT the ".." range operator.
	if l.pos < len(l.input) && l.input[l.pos] == '.' &&
		l.pos+1 < len(l.input) && l.input[l.pos+1] != '.' {
		sb.WriteRune(l.advance()) // '.'
		for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
			sb.WriteRune(l.advance())
		}
		return l.makeTok(TOK_DECIMAL, sb.String(), line, col)
	}
	return l.makeTok(TOK_INTEGER, sb.String(), line, col)
}

// --------------------------------------------------------------------------
// String scanners
// --------------------------------------------------------------------------

// scanDoubleQuoted handles "…" strings and the empty "" initialiser.
func (l *Lexer) scanDoubleQuoted(line, col int) (Token, error) {
	l.advance() // consume opening "
	if l.peek() == '"' {
		l.advance()
		return l.makeTok(TOK_EMPTY_STR_D, `""`, line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('"')
	for l.pos < len(l.input) {
		ch := l.advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.advance()) // consume escaped character verbatim
		case ch == '"':
			return l.makeTok(TOK_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
		}
	}
	return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated double-quoted string", line, col)
}

// scanSingleQuoted handles '…' strings and the empty ” initialiser.
func (l *Lexer) scanSingleQuoted(line, col int) (Token, error) {
	l.advance() // consume opening '
	if l.peek() == '\'' {
		l.advance()
		return l.makeTok(TOK_EMPTY_STR_S, "''", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('\'')
	for l.pos < len(l.input) {
		ch := l.advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.advance())
		case ch == '\'':
			return l.makeTok(TOK_STRING, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
		}
	}
	return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated single-quoted string", line, col)
}

// scanTemplateQuoted handles `…` strings and the empty “ initialiser.
func (l *Lexer) scanTemplateQuoted(line, col int) (Token, error) {
	l.advance() // consume opening `
	if l.peek() == '`' {
		l.advance()
		return l.makeTok(TOK_EMPTY_STR_T, "``", line, col), nil
	}
	var sb strings.Builder
	sb.WriteRune('`')
	for l.pos < len(l.input) {
		ch := l.advance()
		sb.WriteRune(ch)
		if ch == '`' {
			return l.makeTok(TOK_STRING, sb.String(), line, col), nil
		}
		if ch == 0 {
			return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated template-quoted string", line, col)
		}
	}
	return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated template-quoted string", line, col)
}

// --------------------------------------------------------------------------
// Regexp scanner
// Grammar: regexp = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags { regexp_flags } ]
// Called when '/' is encountered and context indicates a regexp value (not
// division).  Handles \/ escape sequences inside the pattern.
// --------------------------------------------------------------------------

func (l *Lexer) scanRegexp(line, col int) (Token, error) {
	l.advance() // consume opening /
	var sb strings.Builder
	sb.WriteRune('/')

	for l.pos < len(l.input) {
		ch := l.advance()
		sb.WriteRune(ch)
		switch {
		case ch == '\\' && l.pos < len(l.input):
			sb.WriteRune(l.advance()) // escaped char — keep verbatim
		case ch == '/':
			// Pattern closed — scan optional flags: g | i
			for l.pos < len(l.input) && (l.input[l.pos] == 'g' || l.input[l.pos] == 'i') {
				sb.WriteRune(l.advance())
			}
			return l.makeTok(TOK_REGEXP, sb.String(), line, col), nil
		case ch == '\n' || ch == 0:
			return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
		}
	}
	return l.makeTok(TOK_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("L%d:C%d: unterminated regexp literal", line, col)
}

// --------------------------------------------------------------------------
// Operator scanner — max-munch, all single and multi-character operators.
// --------------------------------------------------------------------------

// scanOperator dispatches on the current and next rune to emit the correct
// (possibly multi-character) operator token.
func (l *Lexer) scanOperator(line, col int) (Token, error) {
	ch := l.peek()
	ch2 := l.peek2()

	switch ch {

	// . and ..
	case '.':
		if ch2 == '.' {
			l.advance()
			l.advance()
			return l.makeTok(TOK_DOTDOT, "..", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_DOT, ".", line, col), nil

	// - -> -- -: -~
	case '-':
		switch ch2 {
		case '>':
			l.advance()
			l.advance()
			return l.makeTok(TOK_ARROW, "->", line, col), nil
		case '-':
			l.advance()
			l.advance()
			return l.makeTok(TOK_DEC, "--", line, col), nil
		case ':':
			l.advance()
			l.advance()
			return l.makeTok(TOK_ISUB_IMM, "-:", line, col), nil
		case '~':
			l.advance()
			l.advance()
			return l.makeTok(TOK_ISUB_MUT, "-~", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_MINUS, "-", line, col), nil

	// > >> >=
	case '>':
		switch ch2 {
		case '>':
			l.advance()
			l.advance()
			return l.makeTok(TOK_STREAM, ">>", line, col), nil
		case '=':
			l.advance()
			l.advance()
			return l.makeTok(TOK_GEQ, ">=", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_GT, ">", line, col), nil

	// = => =~
	case '=':
		switch ch2 {
		case '>':
			l.advance()
			l.advance()
			return l.makeTok(TOK_STORE, "=>", line, col), nil
		case '~':
			l.advance()
			l.advance()
			return l.makeTok(TOK_MATCH_OP, "=~", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_EQ, "=", line, col), nil

	// < <- <=
	case '<':
		switch ch2 {
		case '-':
			l.advance()
			l.advance()
			return l.makeTok(TOK_RETURN_STMT, "<-", line, col), nil
		case '=':
			l.advance()
			l.advance()
			return l.makeTok(TOK_LEQ, "<=", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_LT, "<", line, col), nil

	// + ++ +: +~
	case '+':
		switch ch2 {
		case '+':
			l.advance()
			l.advance()
			return l.makeTok(TOK_INC, "++", line, col), nil
		case ':':
			l.advance()
			l.advance()
			return l.makeTok(TOK_IADD_IMM, "+:", line, col), nil
		case '~':
			l.advance()
			l.advance()
			return l.makeTok(TOK_IADD_MUT, "+~", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_PLUS, "+", line, col), nil

	// * ** *: *~
	case '*':
		switch ch2 {
		case '*':
			l.advance()
			l.advance()
			return l.makeTok(TOK_POW, "**", line, col), nil
		case ':':
			l.advance()
			l.advance()
			return l.makeTok(TOK_IMUL_IMM, "*:", line, col), nil
		case '~':
			l.advance()
			l.advance()
			return l.makeTok(TOK_IMUL_MUT, "*~", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_STAR, "*", line, col), nil

	// / // /: /~ or regexp /…/flags
	case '/':
		switch ch2 {
		case '/':
			l.advance()
			l.advance()
			return l.makeTok(TOK_REGEXP_DECL, "//", line, col), nil
		case ':':
			l.advance()
			l.advance()
			return l.makeTok(TOK_IDIV_IMM, "/:", line, col), nil
		case '~':
			l.advance()
			l.advance()
			return l.makeTok(TOK_IDIV_MUT, "/~", line, col), nil
		}
		// Regexp vs division disambiguation:
		// If the previous significant token was a value-like token, emit SLASH (division).
		// Otherwise scan a regexp literal.
		if prevIsValue(l.last) {
			l.advance()
			return l.makeTok(TOK_SLASH, "/", line, col), nil
		}
		return l.scanRegexp(line, col)

	// : :~
	case ':':
		if ch2 == '~' {
			l.advance()
			l.advance()
			return l.makeTok(TOK_READONLY, ":~", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_COLON, ":", line, col), nil

	// ! !=
	case '!':
		if ch2 == '=' {
			l.advance()
			l.advance()
			return l.makeTok(TOK_NEQ, "!=", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_BANG, "!", line, col), nil

	// [ []
	case '[':
		if ch2 == ']' {
			l.advance()
			l.advance()
			return l.makeTok(TOK_EMPTY_ARR, "[]", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_LBRACKET, "[", line, col), nil

	// { {}
	case '{':
		if ch2 == '}' {
			l.advance()
			l.advance()
			return l.makeTok(TOK_EMPTY_OBJ, "{}", line, col), nil
		}
		l.advance()
		return l.makeTok(TOK_LBRACE, "{", line, col), nil

	// Single-character punctuation
	case '~':
		l.advance()
		return l.makeTok(TOK_TILDE, "~", line, col), nil
	case '%':
		l.advance()
		return l.makeTok(TOK_PERCENT, "%", line, col), nil
	case '&':
		l.advance()
		return l.makeTok(TOK_AMP, "&", line, col), nil
	case '|':
		l.advance()
		return l.makeTok(TOK_PIPE, "|", line, col), nil
	case '^':
		l.advance()
		return l.makeTok(TOK_CARET, "^", line, col), nil
	case ',':
		l.advance()
		return l.makeTok(TOK_COMMA, ",", line, col), nil
	case ';':
		l.advance()
		return l.makeTok(TOK_SEMICOLON, ";", line, col), nil
	case '(':
		l.advance()
		return l.makeTok(TOK_LPAREN, "(", line, col), nil
	case ')':
		l.advance()
		return l.makeTok(TOK_RPAREN, ")", line, col), nil
	case ']':
		l.advance()
		return l.makeTok(TOK_RBRACKET, "]", line, col), nil
	case '}':
		l.advance()
		return l.makeTok(TOK_RBRACE, "}", line, col), nil
	}

	// Unrecognised character → ILLEGAL (report but continue)
	bad := l.advance()
	return l.makeTok(TOK_ILLEGAL, string(bad), line, col),
		fmt.Errorf("L%d:C%d: unexpected character %q", line, col, bad)
}
