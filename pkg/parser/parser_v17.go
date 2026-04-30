// Package parser — V17 lexer + parser infrastructure for spec/01_definitions.sqg.
//
// Design rules (from implement-parse-method SKILL):
//
//	SIR-1: ParseXxx name == grammar rule name (camelCase).
//	SIR-2: Every ParseXxx calls debugEnter / defer done.
//	SIR-3: Parser follows grammar exactly; no patches inside parser.
//	SIR-4: Never pre-lex domain tokens; all classification at parse time.
package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// =============================================================================
// TOKEN TYPES
// =============================================================================

// V17TokenType identifies the category of a single V17 lexeme.
type V17TokenType int

const (
	// Synthetic / control
	V17_BOF     V17TokenType = iota // beginning of file (synthetic, pos 0)
	V17_EOF                         // end of file
	V17_NL                          // newline sequence — /([ \t]*[\r\n]+)+/
	V17_ILLEGAL                     // unrecognised character

	// Digit run — all digit sequences; width checked at parse time (SIR-4)
	V17_DIGITS // /[0-9]+/

	// String literals — inner content already captured; delimiter chars included
	V17_STRING_SQ // '…'
	V17_STRING_DQ // "…"
	V17_STRING_TQ // `…`

	// Regexp literal — /pattern/ including delimiter slashes; flags are V17_IDENT (SIR-4)
	V17_REGEXP

	// Keywords (lexer keyword map)
	V17_TRUE     // true
	V17_FALSE    // false
	V17_NULL     // null
	V17_NAN      // NaN
	V17_INFINITY // Infinity
	V17_MANY     // many | Many

	// General identifier — includes duration units, regexp flags, hex content (SIR-4)
	V17_IDENT

	// Comment delimiters — emitted as distinct tokens so ParseComment can handle nesting
	V17_COMMENT_BEGIN // (*
	V17_COMMENT_END   // *)

	// Two-character tokens (max-munch)
	V17_DOTDOT   // ..
	V17_ANY_TYPE // @?

	// Two-character operator tokens — max-munch (must precede single-char siblings)
	V17_STARSTAR   // **
	V17_PLUSPLUS   // ++
	V17_MINUSMINUS // --
	V17_EQEQ       // ==
	V17_NEQ        // !=
	V17_GTE        // >=
	V17_LTE        // <=

	// Single-character operators / punctuation
	V17_PLUS      // +
	V17_MINUS     // -
	V17_COLON     // :
	V17_DOT       // .
	V17_SLASH     // /
	V17_SEMICOLON // ;
	V17_COMMA     // ,
	V17_LPAREN    // (
	V17_RPAREN    // )
	V17_LBRACKET  // [
	V17_RBRACKET  // ]
	V17_LBRACE    // {
	V17_RBRACE    // }
	V17_AT        // @
	V17_QUESTION  // ?
	V17_STAR      // *
	V17_BACKTICK  // ` (lone backtick; template strings are V17_STRING_TQ)
	V17_PERCENT   // %
	V17_GT        // >
	V17_LT        // <
	V17_BANG      // !
	V17_AMP       // &
	V17_PIPE      // |
	V17_CARET     // ^
	V17_EQ        // =

	// Two-character compound assignment tokens (03_assignment.sqg)
	V17_PLUS_EQ     // +=
	V17_MINUS_EQ    // -=
	V17_STAR_EQ     // *=
	V17_SLASH_EQ    // /=
	V17_COLON_TILDE // :~

	// Self-reference token (04_functions.sqg — forward ref used in 03_assignment EXTEND)
	V17_DOLLAR // $

	// Two-character operator added for 04_objects.sqg
	V17_GTGT // >>
)

// v17tokenNames maps each V17TokenType to its display name.
var v17tokenNames = map[V17TokenType]string{
	V17_BOF: "BOF", V17_EOF: "EOF", V17_NL: "NL", V17_ILLEGAL: "ILLEGAL",
	V17_DIGITS:    "DIGITS",
	V17_STRING_SQ: "STRING_SQ", V17_STRING_DQ: "STRING_DQ", V17_STRING_TQ: "STRING_TQ",
	V17_REGEXP: "REGEXP",
	V17_TRUE:   "true", V17_FALSE: "false", V17_NULL: "null",
	V17_NAN: "NaN", V17_INFINITY: "Infinity", V17_MANY: "many",
	V17_IDENT:         "IDENT",
	V17_COMMENT_BEGIN: "(*", V17_COMMENT_END: "*)",
	V17_DOTDOT: "..", V17_ANY_TYPE: "@?",
	V17_STARSTAR: "**", V17_PLUSPLUS: "++", V17_MINUSMINUS: "--",
	V17_EQEQ: "==", V17_NEQ: "!=", V17_GTE: ">=", V17_LTE: "<=",
	V17_PLUS: "+", V17_MINUS: "-", V17_COLON: ":", V17_DOT: ".", V17_SLASH: "/",
	V17_SEMICOLON: ";", V17_COMMA: ",",
	V17_LPAREN: "(", V17_RPAREN: ")",
	V17_LBRACKET: "[", V17_RBRACKET: "]",
	V17_LBRACE: "{", V17_RBRACE: "}",
	V17_AT: "@", V17_QUESTION: "?", V17_STAR: "*", V17_BACKTICK: "`",
	V17_PERCENT: "%", V17_GT: ">", V17_LT: "<",
	V17_BANG: "!", V17_AMP: "&", V17_PIPE: "|", V17_CARET: "^", V17_EQ: "=",
	V17_PLUS_EQ: "+=", V17_MINUS_EQ: "-=", V17_STAR_EQ: "*=", V17_SLASH_EQ: "/=",
	V17_COLON_TILDE: ":~",
	V17_DOLLAR:      "$",
	V17_GTGT:        ">>",
}

// String returns the display name of a V17TokenType.
func (t V17TokenType) String() string {
	if s, ok := v17tokenNames[t]; ok {
		return s
	}
	return fmt.Sprintf("V17TOKEN(%d)", int(t))
}

// =============================================================================
// TOKEN
// =============================================================================

// V17Token is a single lexeme produced by the V17 lexer.
type V17Token struct {
	Type  V17TokenType
	Value string
	Line  int
	Col   int
}

// String returns a compact diagnostic representation.
func (t V17Token) String() string {
	return fmt.Sprintf("V17Token{%s %q L%d:C%d}", t.Type, t.Value, t.Line, t.Col)
}

// =============================================================================
// LEXER
// =============================================================================

// v17keywords maps source strings to keyword token types.
var v17keywords = map[string]V17TokenType{
	"true":     V17_TRUE,
	"false":    V17_FALSE,
	"null":     V17_NULL,
	"NaN":      V17_NAN,
	"Infinity": V17_INFINITY,
	"many":     V17_MANY,
	"Many":     V17_MANY,
}

// V17Lexer holds the mutable scan state.
type V17Lexer struct {
	input []rune
	pos   int
	line  int
	col   int
	last  V17TokenType // type of most-recently emitted non-NL token
}

// NewV17Lexer constructs a fresh V17Lexer for the given source string.
func NewV17Lexer(src string) *V17Lexer {
	return &V17Lexer{
		input: []rune(src),
		pos:   0,
		line:  1,
		col:   0,
		last:  V17_BOF,
	}
}

// V17Tokenize scans the entire input and returns the complete token slice.
// The first token is always BOF; the last is always EOF.
func (l *V17Lexer) V17Tokenize() ([]V17Token, error) {
	tokens := []V17Token{l.makeTok(V17_BOF, "", 1, 0)}
	for {
		tok, err := l.scan()
		if err != nil {
			return tokens, err
		}
		if tok.Type != V17_NL {
			l.last = tok.Type
		}
		tokens = append(tokens, tok)
		if tok.Type == V17_EOF {
			break
		}
	}
	return tokens, nil
}

// --------------------------------------------------------------------------
// internal helpers
// --------------------------------------------------------------------------

func (l *V17Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *V17Lexer) peek2() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *V17Lexer) advance() rune {
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

func (l *V17Lexer) skipHorizWS() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *V17Lexer) makeTok(typ V17TokenType, val string, line, col int) V17Token {
	return V17Token{Type: typ, Value: val, Line: line, Col: col}
}

// v17prevIsValue returns true when the previous token closes a value —
// meaning a following '/' is division, not a regexp delimiter.
func v17prevIsValue(t V17TokenType) bool {
	switch t {
	case V17_DIGITS, V17_STRING_SQ, V17_STRING_DQ, V17_STRING_TQ, V17_REGEXP,
		V17_IDENT, V17_TRUE, V17_FALSE, V17_NULL, V17_NAN, V17_INFINITY,
		V17_RPAREN, V17_RBRACKET, V17_RBRACE,
		V17_DOT, V17_DOTDOT: // after "." or ".." the '/' is always a path separator, never regexp
		return true
	}
	return false
}

// --------------------------------------------------------------------------
// scan — top-level dispatch
// --------------------------------------------------------------------------

func (l *V17Lexer) scan() (V17Token, error) {
	l.skipHorizWS()

	if l.pos >= len(l.input) {
		return l.makeTok(V17_EOF, "", l.line, l.col), nil
	}

	line, col := l.line, l.col
	ch := l.peek()

	// Newline
	if ch == '\r' || ch == '\n' {
		return l.scanNL(line, col)
	}
	// Comment  (*
	if ch == '(' && l.peek2() == '*' {
		return l.scanComment(line, col)
	}
	// Identifier / keyword
	if unicode.IsLetter(ch) || ch == '_' {
		return l.scanIdentOrKeyword(line, col), nil
	}
	// Comment end — *) can appear mid-stream after an opening (*
	if ch == '*' && l.peek2() == ')' {
		return l.scanCommentEnd(line, col), nil
	}
	// Digits
	if unicode.IsDigit(ch) {
		return l.scanDigits(line, col), nil
	}
	// @ or @?
	if ch == '@' {
		return l.scanAt(line, col)
	}
	// String literals
	switch ch {
	case '\'':
		return l.scanSingleQuoted(line, col)
	case '"':
		return l.scanDoubleQuoted(line, col)
	case '`':
		return l.scanTemplateQuoted(line, col)
	case '/':
		return l.scanSlashOrRegexp(line, col)
	}
	return l.scanOperator(line, col)
}

// --------------------------------------------------------------------------
// Newline scanner
// --------------------------------------------------------------------------

func (l *V17Lexer) scanNL(line, col int) (V17Token, error) {
	var sb strings.Builder
	for l.pos < len(l.input) {
		wsStart := l.pos
		for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t') {
			l.advance()
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
			ch := l.advance()
			sb.WriteRune(ch)
			if ch == '\r' && l.pos < len(l.input) && l.input[l.pos] == '\n' {
				sb.WriteRune(l.advance())
			}
			consumed = true
		}
		if !consumed {
			break
		}
	}
	return l.makeTok(V17_NL, sb.String(), line, col), nil
}

// --------------------------------------------------------------------------
// Comment scanner  (* ... *)  — nested comments handled at parse level.
// The lexer emits V17_COMMENT_BEGIN for "(*" and then tokenises the interior
// as normal tokens until it emits V17_COMMENT_END for "*)".  Nesting is the
// parser's responsibility (ParseComment recurses on nested "(*").
// --------------------------------------------------------------------------

func (l *V17Lexer) scanComment(line, col int) (V17Token, error) {
	// Consume "(*" and emit V17_COMMENT_BEGIN.
	l.advance()
	l.advance()
	return l.makeTok(V17_COMMENT_BEGIN, "(*", line, col), nil
}

// scanCommentEnd is called by scan() when "*)" is encountered mid-stream.
func (l *V17Lexer) scanCommentEnd(line, col int) V17Token {
	l.advance() // *
	l.advance() // )
	return l.makeTok(V17_COMMENT_END, "*)", line, col)
}

// --------------------------------------------------------------------------
// Identifier / keyword scanner
// --------------------------------------------------------------------------

func (l *V17Lexer) scanIdentOrKeyword(line, col int) V17Token {
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(ch)
			l.advance()
		} else {
			break
		}
	}
	val := sb.String()
	if kw, ok := v17keywords[val]; ok {
		return l.makeTok(kw, val, line, col)
	}
	return l.makeTok(V17_IDENT, val, line, col)
}

// --------------------------------------------------------------------------
// Digit scanner
// --------------------------------------------------------------------------

func (l *V17Lexer) scanDigits(line, col int) V17Token {
	var sb strings.Builder
	for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
		sb.WriteRune(l.advance())
	}
	return l.makeTok(V17_DIGITS, sb.String(), line, col)
}

// --------------------------------------------------------------------------
// @ and @? scanner
// --------------------------------------------------------------------------

func (l *V17Lexer) scanAt(line, col int) (V17Token, error) {
	l.advance() // consume @
	if l.pos < len(l.input) && l.input[l.pos] == '?' {
		l.advance()
		return l.makeTok(V17_ANY_TYPE, "@?", line, col), nil
	}
	return l.makeTok(V17_AT, "@", line, col), nil
}

// --------------------------------------------------------------------------
// String literal scanners
// --------------------------------------------------------------------------

func (l *V17Lexer) scanSingleQuoted(line, col int) (V17Token, error) {
	l.advance() // consume opening '
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '\\' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '\'' {
			sb.WriteRune(l.advance())
			sb.WriteRune(l.advance())
			continue
		}
		if ch == '\'' {
			l.advance()
			return l.makeTok(V17_STRING_SQ, sb.String(), line, col), nil
		}
		sb.WriteRune(l.advance())
	}
	return l.makeTok(V17_ILLEGAL, sb.String(), line, col), fmt.Errorf("unclosed single-quoted string at L%d:C%d", line, col)
}

func (l *V17Lexer) scanDoubleQuoted(line, col int) (V17Token, error) {
	l.advance() // consume opening "
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '\\' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '"' {
			sb.WriteRune(l.advance())
			sb.WriteRune(l.advance())
			continue
		}
		if ch == '"' {
			l.advance()
			return l.makeTok(V17_STRING_DQ, sb.String(), line, col), nil
		}
		sb.WriteRune(l.advance())
	}
	return l.makeTok(V17_ILLEGAL, sb.String(), line, col), fmt.Errorf("unclosed double-quoted string at L%d:C%d", line, col)
}

func (l *V17Lexer) scanTemplateQuoted(line, col int) (V17Token, error) {
	l.advance() // consume opening `
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '`' {
			l.advance()
			return l.makeTok(V17_STRING_TQ, sb.String(), line, col), nil
		}
		sb.WriteRune(l.advance())
	}
	return l.makeTok(V17_ILLEGAL, sb.String(), line, col), fmt.Errorf("unclosed template string at L%d:C%d", line, col)
}

// --------------------------------------------------------------------------
// Slash / regexp scanner
// --------------------------------------------------------------------------

func (l *V17Lexer) scanSlashOrRegexp(line, col int) (V17Token, error) {
	// If the previous token was a value-ending token, '/' is division (or /=).
	if v17prevIsValue(l.last) {
		l.advance()
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_SLASH_EQ, "/=", line, col), nil
		}
		return l.makeTok(V17_SLASH, "/", line, col), nil
	}
	// Regexp literal: /pattern/[flags]
	l.advance() // consume opening /
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '\\' && l.pos+1 < len(l.input) {
			sb.WriteRune(l.advance())
			sb.WriteRune(l.advance())
			continue
		}
		if ch == '/' {
			l.advance() // consume closing /
			// Consume flag letters (a-zA-Z) — stored in the token value after the closing /
			// SIR-4: flags are V17_IDENT tokens; we do NOT pre-scan them into the regexp token.
			return l.makeTok(V17_REGEXP, sb.String(), line, col), nil
		}
		if ch == '\n' || ch == '\r' {
			return l.makeTok(V17_ILLEGAL, sb.String(), line, col),
				fmt.Errorf("unterminated regexp at L%d:C%d", line, col)
		}
		sb.WriteRune(l.advance())
	}
	return l.makeTok(V17_ILLEGAL, sb.String(), line, col),
		fmt.Errorf("unterminated regexp at L%d:C%d", line, col)
}

// --------------------------------------------------------------------------
// Operator scanner
// --------------------------------------------------------------------------

func (l *V17Lexer) scanOperator(line, col int) (V17Token, error) {
	ch := l.advance()
	switch ch {
	case '.':
		if l.pos < len(l.input) && l.input[l.pos] == '.' {
			l.advance()
			return l.makeTok(V17_DOTDOT, "..", line, col), nil
		}
		return l.makeTok(V17_DOT, ".", line, col), nil
	case '+':
		if l.pos < len(l.input) && l.input[l.pos] == '+' {
			l.advance()
			return l.makeTok(V17_PLUSPLUS, "++", line, col), nil
		}
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_PLUS_EQ, "+=", line, col), nil
		}
		return l.makeTok(V17_PLUS, "+", line, col), nil
	case '-':
		if l.pos < len(l.input) && l.input[l.pos] == '-' {
			l.advance()
			return l.makeTok(V17_MINUSMINUS, "--", line, col), nil
		}
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_MINUS_EQ, "-=", line, col), nil
		}
		return l.makeTok(V17_MINUS, "-", line, col), nil
	case '*':
		if l.pos < len(l.input) && l.input[l.pos] == '*' {
			l.advance()
			return l.makeTok(V17_STARSTAR, "**", line, col), nil
		}
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_STAR_EQ, "*=", line, col), nil
		}
		return l.makeTok(V17_STAR, "*", line, col), nil
	case '=':
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_EQEQ, "==", line, col), nil
		}
		return l.makeTok(V17_EQ, "=", line, col), nil
	case '!':
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_NEQ, "!=", line, col), nil
		}
		return l.makeTok(V17_BANG, "!", line, col), nil
	case '>':
		if l.pos < len(l.input) && l.input[l.pos] == '>' {
			l.advance()
			return l.makeTok(V17_GTGT, ">>", line, col), nil
		}
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_GTE, ">=", line, col), nil
		}
		return l.makeTok(V17_GT, ">", line, col), nil
	case '<':
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return l.makeTok(V17_LTE, "<=", line, col), nil
		}
		return l.makeTok(V17_LT, "<", line, col), nil
	case '%':
		return l.makeTok(V17_PERCENT, "%", line, col), nil
	case '&':
		return l.makeTok(V17_AMP, "&", line, col), nil
	case '|':
		return l.makeTok(V17_PIPE, "|", line, col), nil
	case '^':
		return l.makeTok(V17_CARET, "^", line, col), nil
	case ':':
		if l.pos < len(l.input) && l.input[l.pos] == '~' {
			l.advance()
			return l.makeTok(V17_COLON_TILDE, ":~", line, col), nil
		}
		return l.makeTok(V17_COLON, ":", line, col), nil
	case ';':
		return l.makeTok(V17_SEMICOLON, ";", line, col), nil
	case ',':
		return l.makeTok(V17_COMMA, ",", line, col), nil
	case '(':
		return l.makeTok(V17_LPAREN, "(", line, col), nil
	case ')':
		return l.makeTok(V17_RPAREN, ")", line, col), nil
	case '[':
		return l.makeTok(V17_LBRACKET, "[", line, col), nil
	case ']':
		return l.makeTok(V17_RBRACKET, "]", line, col), nil
	case '{':
		return l.makeTok(V17_LBRACE, "{", line, col), nil
	case '}':
		return l.makeTok(V17_RBRACE, "}", line, col), nil
	case '?':
		return l.makeTok(V17_QUESTION, "?", line, col), nil
	case '$':
		return l.makeTok(V17_DOLLAR, "$", line, col), nil
	case '§':
		return l.makeTok(V17_IDENT, "§", line, col), nil
	}
	return l.makeTok(V17_ILLEGAL, string(ch), line, col),
		fmt.Errorf("unexpected character %q at L%d:C%d", ch, line, col)
}

// =============================================================================
// PARSER STRUCT
// =============================================================================

// V17Parser is a hand-written recursive-descent parser for spec/01_definitions.sqg.
type V17Parser struct {
	tokens         []V17Token
	pos            int
	DebugFlag      bool
	debugDepth     int
	src            string   // raw source text
	callStack      []string // live rule-name stack; always maintained
	lastErrorStack []string // snapshot of callStack at the time of the last errAt call
	SourceLines    []string // src split on "\n" (line n → SourceLines[n-1])
}

// NewV17Parser constructs a V17Parser from an already-lexed token slice.
// src is the original source string (used for debug preview only).
func NewV17Parser(tokens []V17Token, src string) *V17Parser {
	p := &V17Parser{
		tokens:      tokens,
		src:         src,
		SourceLines: strings.Split(src, "\n"),
	}
	// Skip the synthetic BOF token so cur() returns the first real token.
	if len(tokens) > 0 && tokens[0].Type == V17_BOF {
		p.pos = 1
	}
	return p
}

// NewV17ParserFromSource lexes src and returns a ready-to-use V17Parser.
func NewV17ParserFromSource(src string) (*V17Parser, error) {
	l := NewV17Lexer(src)
	tokens, err := l.V17Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lex error: %w", err)
	}
	return NewV17Parser(tokens, src), nil
}

// EnableDebug turns on parse trace output to stderr.
func (p *V17Parser) EnableDebug() { p.DebugFlag = true }

// --------------------------------------------------------------------------
// Token navigation
// --------------------------------------------------------------------------

// cur returns the current token without advancing.
func (p *V17Parser) cur() V17Token {
	if p.pos >= len(p.tokens) {
		return V17Token{Type: V17_EOF}
	}
	return p.tokens[p.pos]
}

// peek1 returns the token one position ahead of cur without advancing.
func (p *V17Parser) peek1() V17Token {
	if p.pos+1 >= len(p.tokens) {
		return V17Token{Type: V17_EOF}
	}
	return p.tokens[p.pos+1]
}

// advance consumes and returns the current token.
func (p *V17Parser) advance() V17Token {
	tok := p.cur()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

// skipNL advances past any V17_NL tokens.
func (p *V17Parser) skipNL() {
	for p.cur().Type == V17_NL {
		p.advance()
	}
}

// savePos returns the current position for backtracking.
func (p *V17Parser) savePos() int { return p.pos }

// restorePos resets the parser to a saved position.
func (p *V17Parser) restorePos(pos int) { p.pos = pos }

// expect consumes the current token if it matches typ and returns it.
// Returns an error if the token does not match.
func (p *V17Parser) expect(typ V17TokenType) (V17Token, error) {
	tok := p.cur()
	if tok.Type != typ {
		return tok, fmt.Errorf("expected %s, got %s %q at L%d:C%d",
			typ, tok.Type, tok.Value, tok.Line, tok.Col)
	}
	p.advance()
	return tok, nil
}

// expectLit consumes the current token if its Value matches lit and returns it.
func (p *V17Parser) expectLit(lit string) (V17Token, error) {
	tok := p.cur()
	if tok.Value != lit {
		return tok, fmt.Errorf("expected %q, got %s %q at L%d:C%d",
			lit, tok.Type, tok.Value, tok.Line, tok.Col)
	}
	p.advance()
	return tok, nil
}

// errAt creates a position-stamped error at the current token and snapshots
// the call stack for use by FormatParseError.
func (p *V17Parser) errAt(format string, args ...any) error {
	tok := p.cur()
	msg := fmt.Sprintf(format, args...)
	// Snapshot the live call stack; the defers will unwind it before the caller
	// of FormatParseError can inspect it, so we must capture it here.
	p.lastErrorStack = append([]string{}, p.callStack...)
	return fmt.Errorf("%s at L%d:C%d", msg, tok.Line, tok.Col)
}

// --------------------------------------------------------------------------
// Debug helpers (SIR-2)
// --------------------------------------------------------------------------

// debugPreview returns the first ≤20 characters of the token stream from the
// current position, with whitespace normalised to a single space.
func (p *V17Parser) debugPreview() string {
	var sb strings.Builder
	for i := p.pos; i < len(p.tokens) && sb.Len() < 20; i++ {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		s := p.tokens[i].Value
		if sb.Len()+len(s) > 20 {
			s = s[:20-sb.Len()]
		}
		sb.WriteString(s)
	}
	preview := sb.String()
	if len(preview) > 20 {
		preview = preview[:20]
	}
	return preview
}

// debugEnter prints the entry trace line and increments depth.
// It always pushes the rule name onto callStack for error reporting.
// Returns a done function that must be deferred.
func (p *V17Parser) debugEnter(rule string) func(ok bool) {
	// Always maintain call stack (needed for FormatParseError even without DebugFlag).
	p.callStack = append(p.callStack, rule)
	pop := func() {
		if len(p.callStack) > 0 {
			p.callStack = p.callStack[:len(p.callStack)-1]
		}
	}

	if !p.DebugFlag {
		p.debugDepth++
		return func(bool) {
			p.debugDepth--
			pop()
		}
	}
	tok := p.cur()
	indent := strings.Repeat("  ", p.debugDepth)
	fmt.Fprintf(os.Stderr, "%s→ %s  [L%d:C%d  %s]\n",
		indent, rule, tok.Line, tok.Col, p.debugPreview())
	p.debugDepth++
	return func(ok bool) {
		p.debugDepth--
		status := "OK"
		if !ok {
			status = "FAIL"
		}
		ind := strings.Repeat("  ", p.debugDepth)
		fmt.Fprintf(os.Stderr, "%s← %s  %s\n", ind, rule, status)
		pop()
	}
}

// unknownTokens accumulates unresolvable tokens (debug mode only).
var v17unknownTokens []V17Token

// trackUnknown records a token that no parse method could consume.
func (p *V17Parser) trackUnknown(tok V17Token) {
	if !p.DebugFlag {
		return
	}
	v17unknownTokens = append(v17unknownTokens, tok)
	fmt.Fprintf(os.Stderr, "[V17 UNKNOWN TOKEN] L%d:C%d  %s %q\n",
		tok.Line, tok.Col, tok.Type, tok.Value)
}

// FormatParseError produces the Step-11 four-section parse error report.
//
// Section 1 — short error framing (the raw error string).
// Section 2 — source excerpt with a caret pointer to the failing token.
// Section 3 — last ≤10 rule names from the call stack, innermost last.
// Section 4 — reminder to use --debug for the full call tree.
func (p *V17Parser) FormatParseError(err error) string {
	if err == nil {
		return ""
	}
	var sb strings.Builder

	// ── Section 1: short framing ─────────────────────────────────────────
	sb.WriteString("── Parse error ─────────────────────────────────────────────────────\n")
	sb.WriteString(err.Error())
	sb.WriteString("\n\n")

	// ── Section 2: source excerpt with caret ─────────────────────────────
	tok := p.cur()
	// If we landed on EOF, step back to the last real token for the excerpt.
	if tok.Type == V17_EOF && p.pos > 0 {
		tok = p.tokens[p.pos-1]
	}
	lineNum := tok.Line
	col := tok.Col
	if lineNum >= 1 && lineNum <= len(p.SourceLines) {
		srcLine := p.SourceLines[lineNum-1]
		sb.WriteString(fmt.Sprintf("  L%d:C%d\n\n", lineNum, col))
		sb.WriteString("  " + srcLine + "\n")
		// Caret: col is 0-based.
		caretOff := col
		if caretOff > len(srcLine) {
			caretOff = len(srcLine)
		}
		sb.WriteString("  " + strings.Repeat(" ", caretOff) + "^\n")
		sb.WriteString(fmt.Sprintf("  col %d  ← %s %q\n\n", col, tok.Type, tok.Value))
	}

	// ── Section 3: call stack (last ≤10 entries, outermost first) ────────
	stack := p.lastErrorStack
	if len(stack) == 0 {
		stack = p.callStack // fallback: use whatever is left
	}
	if len(stack) > 0 {
		sb.WriteString("Call stack (outermost → failing rule):\n")
		start := 0
		if len(stack) > 10 {
			start = len(stack) - 10
		}
		for i, rule := range stack[start:] {
			num := start + i + 1
			marker := ""
			if i == len(stack[start:])-1 {
				marker = "  ← FAIL"
			}
			sb.WriteString(fmt.Sprintf("  %2d. %s%s\n", num, rule, marker))
		}
		sb.WriteString("\n")
	}

	// ── Section 4: root cause hint ────────────────────────────────────────
	sb.WriteString("── Root cause ──────────────────────────────────────────────────────\n")
	sb.WriteString("Run with --debug for the full → / ← call trace.\n")
	sb.WriteString("────────────────────────────────────────────────────────────────────\n")

	return sb.String()
}

// --------------------------------------------------------------------------
// Regex helpers used by parse methods (SIR-4 — matching at parse time)
// --------------------------------------------------------------------------

var (
	reHexN = func(n int) *regexp.Regexp {
		return regexp.MustCompile(fmt.Sprintf(`(?i)^[0-9a-fA-F]{%d}$`, n))
	}
	reHex2   = regexp.MustCompile(`(?i)^[0-9a-fA-F]{2}$`)
	reHex4   = regexp.MustCompile(`(?i)^[0-9a-fA-F]{4}$`)
	reHex8   = regexp.MustCompile(`(?i)^[0-9a-fA-F]{8}$`)
	reHex12  = regexp.MustCompile(`(?i)^[0-9a-fA-F]{12}$`)
	reHex32  = regexp.MustCompile(`(?i)^[0-9a-fA-F]{32}$`)
	reHex40  = regexp.MustCompile(`(?i)^[0-9a-fA-F]{40}$`)
	reHex64  = regexp.MustCompile(`(?i)^[0-9a-fA-F]{64}$`)
	reHex128 = regexp.MustCompile(`(?i)^[0-9a-fA-F]{128}$`)

	reUuidV7Ver = regexp.MustCompile(`(?i)^7[0-9a-fA-F]{3}$`)
	reUuidV7Var = regexp.MustCompile(`(?i)^[89aAbB][0-9a-fA-F]{3}$`)

	reUlid   = regexp.MustCompile(`(?i)^[0-7][0-9A-HJKMNP-TV-Z]{9}[0-9A-HJKMNP-TV-Z]{16}$`)
	reNanoId = regexp.MustCompile(`^[A-Za-z0-9_-]{21}$`)

	reHttpUrl = regexp.MustCompile(`(?i)^https?://(?:[\w-]+\.)+[\w-]+(?::\d+)?(?:/[^\s]*)?(?:\?[^\s]*)?(?:#[^\s]*)?$`)
	reFileUrl = regexp.MustCompile(`^file:///(?:[^\s\0]+|\.\.\/[^\s\0]+|\./[^\s\0]+)$`)

	reDigits2 = regexp.MustCompile(`^[0-9]{1,2}$`)
	reDigits3 = regexp.MustCompile(`^[0-9]{1,3}$`)
	reDigits4 = regexp.MustCompile(`^[0-9]{4}$`)
)

// matchHex validates the current token value against a hex pattern without
// consuming. Returns the token and true if matched.
// matchHexToken checks whether the current position holds a valid hex value
// matching re.  Because the lexer splits digit-leading hex (e.g. "1a2b" →
// DIGITS("1") + IDENT("a2b")), this helper also tries a two-token combine.
// Returns: (synthetic token with combined value, number of tokens to advance, ok).
func (p *V17Parser) matchHexToken(re *regexp.Regexp) (V17Token, int, bool) {
	tok := p.cur()
	// Single-token match: IDENT or DIGITS whose entire value matches the regex.
	if (tok.Type == V17_IDENT || tok.Type == V17_DIGITS) && re.MatchString(tok.Value) {
		return tok, 1, true
	}
	// Two-token match: DIGITS followed immediately by IDENT where the concatenated
	// value matches the regex (handles hex starting with a numeric digit).
	if tok.Type == V17_DIGITS {
		nxt := p.peek1()
		if nxt.Type == V17_IDENT {
			combined := tok.Value + nxt.Value
			if re.MatchString(combined) {
				synth := V17Token{Type: V17_IDENT, Value: combined, Line: tok.Line, Col: tok.Col}
				return synth, 2, true
			}
		}
	}
	return tok, 0, false
}
