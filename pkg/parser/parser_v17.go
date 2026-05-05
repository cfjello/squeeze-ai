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

	// =~ operator for jp_filter_oper (05_json_path.sqg).
	// Note: ~ alone is ILLEGAL so this must remain a compound lexer token.
	// It is ONLY consumed by ParseJpFilterOper — never by any other rule.
	V17_EQ_TILDE // =~

	// ~> operator — push_oper (06_functions.sqg / 13_push_pull.sqg).
	// Note: ~ alone is ILLEGAL so this must be a compound lexer token.
	V17_TILDE_GT // ~>
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
	V17_EQ_TILDE:    "=~",
	V17_TILDE_GT:    "~>",
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

// =============================================================================
// PARSER STRUCT
// =============================================================================

// V17Pos captures the full parser position for backtracking.
// It holds both the legacy token-slice index (during migration) and the
// rune-stream cursor used by the new match-primitive API.
// All savePos/restorePos call sites use := with type inference, so the
// change from plain int is transparent to every caller.
type V17Pos struct {
	tokenPos int // index into p.tokens (legacy token-slice API)
	runePos  int // byte-index into p.input (rune-stream API)
	runeLine int
	runeCol  int
	atBOF    bool // mirrors p.atBOF so backtracking restores BOF state
}

// V17Parser is a hand-written recursive-descent parser for spec/01_definitions.sqg.
type V17Parser struct {
	// ── legacy token-slice API (kept while terminal methods are migrated) ──
	tokens []V17Token
	pos    int

	// ── rune-stream API (new match-primitive layer) ──
	// The parser owns the source as a []rune so that every character is
	// classified for the first time inside the grammar rule function that
	// needs it (SIR-4).  No pre-tokenization step exists for this API.
	input      []rune
	runePos    int
	runeLine   int   // 1-based
	runeCol    int   // 0-based
	lineStarts []int // lineStarts[i] = rune index of start of line i+1 (0-indexed array, 1-based lines)

	// ── shared ──
	DebugFlag      bool
	debugDepth     int
	atBOF          bool     // true until the first ParseNl consumes the BOF position
	src            string   // raw source text
	callStack      []string // live rule-name stack; always maintained
	lastErrorStack []string // snapshot of callStack at the time of the last errAt call
	SourceLines    []string // src split on "\n" (line n → SourceLines[n-1])
}

// NewV17Parser constructs a V17Parser from an already-lexed token slice.
// src is the original source string (used for debug preview and the rune-stream API).
func NewV17Parser(tokens []V17Token, src string) *V17Parser {
	runes := []rune(src)
	// Precompute rune index of the start of each line for O(1) line/col → rune-offset mapping.
	lineStarts := []int{0}
	for i, r := range runes {
		if r == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	p := &V17Parser{
		tokens:      tokens,
		src:         src,
		input:       runes,
		runeLine:    1,
		runePos:     0,
		runeCol:     0,
		lineStarts:  lineStarts,
		atBOF:       true,
		SourceLines: strings.Split(src, "\n"),
	}
	// Skip the synthetic BOF token so cur() returns the first real token.
	if len(tokens) > 0 && tokens[0].Type == V17_BOF {
		p.pos = 1
	}
	// Sync rune cursor to the first real token so mixed-mode code sees a
	// consistent position before any advance() is called.
	// Only sync if there are tokens; with no tokens the rune cursor stays at 0.
	if len(tokens) > 0 {
		p.syncRuneToToken()
	}
	return p
}

// NewV17ParserFromSource constructs a V17Parser directly from source (no pre-lexing).
func NewV17ParserFromSource(src string) (*V17Parser, error) {
	return NewV17Parser(nil, src), nil
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
// It also keeps the rune cursor in sync so that mixed-mode callers (methods
// that still use the token API alongside rune-stream methods) see a
// consistent position after every token advance.
func (p *V17Parser) advance() V17Token {
	tok := p.cur()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	p.syncRuneToToken() // keep rune cursor in sync
	return tok
}

// skipNL advances past any V17_NL tokens.
func (p *V17Parser) skipNL() {
	for {
		// Skip horizontal whitespace
		for p.runePos < len(p.input) && (p.input[p.runePos] == ' ' || p.input[p.runePos] == '\t') {
			p.runeAdvanceBy(1)
		}
		if p.runePos < len(p.input) && (p.input[p.runePos] == '\n' || p.input[p.runePos] == '\r') {
			p.runeAdvanceBy(1)
			p.syncTokenToRune()
		} else {
			break
		}
	}
}

// skipNLAndComments extends skipNL to also consume (possibly nested) (* … *)
// block comments, which may appear inline after a value on any line.
// It alternates between skipNL and comment-skipping until no further progress.
func (p *V17Parser) skipNLAndComments() {
	for {
		p.skipNL()
		// Peek for horizontal whitespace then "(*"
		i := p.runePos
		for i < len(p.input) && (p.input[i] == ' ' || p.input[i] == '\t') {
			i++
		}
		if i+1 < len(p.input) && p.input[i] == '(' && p.input[i+1] == '*' {
			// Advance past the whitespace so matchLit("(*") can find it
			p.runePos = i
			p.syncTokenToRune()
			if _, err := p.matchLit("(*"); err != nil {
				break
			}
			// Skip comment body (nesting aware)
			depth := 1
			for depth > 0 && p.runePos < len(p.input) {
				if p.runePos+1 < len(p.input) && p.input[p.runePos] == '(' && p.input[p.runePos+1] == '*' {
					depth++
					p.runeAdvanceBy(2)
				} else if p.runePos+1 < len(p.input) && p.input[p.runePos] == '*' && p.input[p.runePos+1] == ')' {
					depth--
					p.runeAdvanceBy(2)
				} else {
					p.runeAdvanceBy(1)
				}
			}
			p.syncTokenToRune()
		} else {
			break
		}
	}
}

// savePos captures the complete parser position for backtracking.
// Returns V17Pos which holds both the token-slice index and the rune-stream
// cursor.  All call sites use := so the type change from plain int is
// transparent — no call site needs editing.
func (p *V17Parser) savePos() V17Pos {
	return V17Pos{p.pos, p.runePos, p.runeLine, p.runeCol, p.atBOF}
}

// restorePos resets the parser to a previously saved position.
func (p *V17Parser) restorePos(s V17Pos) {
	p.pos = s.tokenPos
	p.runePos = s.runePos
	p.runeLine = s.runeLine
	p.runeCol = s.runeCol
	p.atBOF = s.atBOF
}

// =============================================================================
// BIDIRECTIONAL SYNC — keeps the legacy token cursor and the rune cursor
// consistent so that mixed-mode code (some methods converted, others not)
// works correctly throughout Phase 2 migration.
//
// Invariant:
//   After any advance() call  : runePos == start-of-new-current-token
//   After any matchLit/matchRe: p.pos   == first unconsumed token
// =============================================================================

// syncRuneToToken sets the rune cursor to the start of the current token.
// Called by advance() so that callers who then invoke rune-stream primitives
// find the rune cursor already positioned correctly.
func (p *V17Parser) syncRuneToToken() {
	tok := p.cur()
	if tok.Type == V17_EOF {
		p.runePos = len(p.input)
		// leave runeLine/runeCol as-is; they are only meaningful when runePos < len(input)
		return
	}
	line := tok.Line // 1-based
	col := tok.Col   // 0-based
	if line >= 1 && line <= len(p.lineStarts) {
		p.runePos = p.lineStarts[line-1] + col
		p.runeLine = line
		p.runeCol = col
	}
}

// syncTokenToRune advances the token cursor forward until p.tokens[p.pos]
// starts at or after the current rune position.  Called by matchLit/matchRe
// so that subsequent token-API code sees the correct next token.
func (p *V17Parser) syncTokenToRune() {
	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		if tok.Type == V17_EOF {
			break
		}
		var tokRuneOffset int
		line := tok.Line
		if line >= 1 && line <= len(p.lineStarts) {
			tokRuneOffset = p.lineStarts[line-1] + tok.Col
		}
		if tokRuneOffset >= p.runePos {
			break
		}
		p.pos++
	}
}

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
	return p.matchLit(lit)
}

// errAt creates a position-stamped error at the current token and snapshots
// the call stack for use by FormatParseError.
func (p *V17Parser) errAt(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	// Snapshot the live call stack; the defers will unwind it before the caller
	// of FormatParseError can inspect it, so we must capture it here.
	p.lastErrorStack = append([]string{}, p.callStack...)
	return fmt.Errorf("%s at L%d:C%d", msg, p.runeLine, p.runeCol)
}

// =============================================================================
// MATCH PRIMITIVES  (rune-stream API — SIR-4 compliant)
//
// These are the only methods that ever touch p.input directly.  Every
// terminal grammar rule function calls exactly one of these; no ParseXxx
// method touches p.input any other way.
//
// Conventions:
//   - All primitives skip horizontal whitespace (space, tab) first, per
//     directive 1.1 in spec/00_directives.sqg, UNLESS the caller has
//     annotated the adjacency "!WS!" — handled by calling matchLitNoWS.
//   - On success  the rune cursor is advanced past the matched text.
//   - On failure  the rune cursor is NOT advanced (caller can backtrack).
//   - Line/col in the returned V17Token reflect the start of the match
//     (after WS skip), not the pre-skip position.
// =============================================================================

// runeAdvanceBy advances the rune cursor by n runes, tracking line/col.
func (p *V17Parser) runeAdvanceBy(n int) {
	for i := 0; i < n && p.runePos < len(p.input); i++ {
		if p.input[p.runePos] == '\n' {
			p.runeLine++
			p.runeCol = 0
		} else {
			p.runeCol++
		}
		p.runePos++
	}
}

// skipRuneHorizWS consumes horizontal whitespace (space, tab) at the rune
// cursor.  It does NOT consume newlines (those are grammar-level NL tokens).
func (p *V17Parser) skipRuneHorizWS() {
	for p.runePos < len(p.input) {
		ch := p.input[p.runePos]
		if ch == ' ' || ch == '\t' {
			p.runeAdvanceBy(1)
		} else {
			break
		}
	}
}

// peekAfterWS returns the rune that would be seen after skipping horizontal
// whitespace, without consuming anything.  Returns 0 at end of input.
func (p *V17Parser) peekAfterWS() rune {
	i := p.runePos
	for i < len(p.input) && (p.input[i] == ' ' || p.input[i] == '\t') {
		i++
	}
	if i >= len(p.input) {
		return 0
	}
	return p.input[i]
}

// peekLit returns true if the literal string s appears at the current rune
// position (after skipping horizontal whitespace), without consuming anything.
func (p *V17Parser) peekLit(s string) bool {
	runes := []rune(s)
	i := p.runePos
	for i < len(p.input) && (p.input[i] == ' ' || p.input[i] == '\t') {
		i++
	}
	if i+len(runes) > len(p.input) {
		return false
	}
	for j, r := range runes {
		if p.input[i+j] != r {
			return false
		}
	}
	return true
}

// matchLit skips horizontal whitespace and then attempts to match the literal
// string s at the rune cursor.  On success the cursor advances past s and the
// token cursor is synced forward (bidirectional sync for Phase 2 migration).
// On failure the cursor is unchanged and an error is returned.
func (p *V17Parser) matchLit(s string) (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	runes := []rune(s)
	if p.runePos+len(runes) > len(p.input) {
		return V17Token{}, fmt.Errorf("expected %q, got EOF at L%d:C%d", s, line, col)
	}
	for i, r := range runes {
		if p.input[p.runePos+i] != r {
			got := string(p.input[p.runePos:min(p.runePos+len(runes), len(p.input))])
			return V17Token{}, fmt.Errorf("expected %q, got %q at L%d:C%d", s, got, line, col)
		}
	}
	p.runeAdvanceBy(len(runes))
	p.syncTokenToRune()
	return V17Token{Value: s, Line: line, Col: col}, nil
}

// matchKeyword is like matchLit but also enforces a word-boundary after the
// matched literal — the next rune must not be alphanumeric or '_'. This
// prevents "true" from matching the first four runes of "truthy".
// Internally saves and restores position on any failure, so it is safe for
// both required and optional (try-style) call sites.
func (p *V17Parser) matchKeyword(word string) (V17Token, error) {
	saved := p.savePos()
	tok, err := p.matchLit(word)
	if err != nil {
		p.restorePos(saved)
		return V17Token{}, err
	}
	// Word-boundary: next rune must not be alphanumeric or underscore.
	if p.runePos < len(p.input) {
		ch := p.input[p.runePos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' {
			p.restorePos(saved)
			return V17Token{}, fmt.Errorf("expected keyword %q but got longer identifier at L%d:C%d",
				word, saved.runeLine, saved.runeCol)
		}
	}
	return tok, nil
}

// matchLitNoWS is like matchLit but does NOT skip horizontal whitespace first.
// Use this when the grammar spec has annotated the adjacency with !WS!.
func (p *V17Parser) matchLitNoWS(s string) (V17Token, error) {
	line, col := p.runeLine, p.runeCol
	runes := []rune(s)
	if p.runePos+len(runes) > len(p.input) {
		return V17Token{}, fmt.Errorf("expected %q (no-WS), got EOF at L%d:C%d", s, line, col)
	}
	for i, r := range runes {
		if p.input[p.runePos+i] != r {
			got := string(p.input[p.runePos:min(p.runePos+len(runes), len(p.input))])
			return V17Token{}, fmt.Errorf("expected %q (no-WS), got %q at L%d:C%d", s, got, line, col)
		}
	}
	p.runeAdvanceBy(len(runes))
	p.syncTokenToRune()
	return V17Token{Value: s, Line: line, Col: col}, nil
}

// matchReNoWS is like matchRe but does NOT skip horizontal whitespace first.
// Use this when the grammar spec has annotated the adjacency with !WS!.
func (p *V17Parser) matchReNoWS(re *regexp.Regexp) (V17Token, error) {
	line, col := p.runeLine, p.runeCol
	remaining := string(p.input[p.runePos:])
	m := re.FindString(remaining)
	if m == "" {
		return V17Token{}, fmt.Errorf("no match (no-WS) for %s at L%d:C%d", re.String(), line, col)
	}
	p.runeAdvanceBy(len([]rune(m)))
	p.syncTokenToRune()
	return V17Token{Value: m, Line: line, Col: col}, nil
}

// matchRe skips horizontal whitespace and then attempts to match the compiled
// regular expression re at the rune cursor.  re MUST be anchored at the start
// with "^" — callers compile patterns at package init time.
// On success the cursor advances past the full match and the token cursor is
// synced forward (bidirectional sync for Phase 2 migration).
// On failure the cursor is unchanged and an error is returned.
func (p *V17Parser) matchRe(re *regexp.Regexp) (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	remaining := string(p.input[p.runePos:])
	m := re.FindString(remaining)
	if m == "" {
		preview := remaining
		if len(preview) > 20 {
			preview = preview[:20]
		}
		return V17Token{}, fmt.Errorf("no match for %s at L%d:C%d (got %q)", re.String(), line, col, preview)
	}
	p.runeAdvanceBy(len([]rune(m)))
	p.syncTokenToRune()
	return V17Token{Value: m, Line: line, Col: col}, nil
}

// min is a local helper (Go 1.20 has a built-in; keep this for older toolchains).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// SPECIALISED MATCH HELPERS  (rune-stream, for use by converted ParseXxx methods)
// =============================================================================

// matchDigits matches one or more ASCII digits ([0-9]+) at the current rune
// position after skipping horizontal whitespace.
func (p *V17Parser) matchDigits() (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	start := p.runePos
	for p.runePos < len(p.input) && p.input[p.runePos] >= '0' && p.input[p.runePos] <= '9' {
		p.runeAdvanceBy(1)
	}
	if p.runePos == start {
		return V17Token{}, fmt.Errorf("digits: expected [0-9]+ at L%d:C%d", line, col)
	}
	p.syncTokenToRune()
	return V17Token{Value: string(p.input[start:p.runePos]), Line: line, Col: col}, nil
}

// matchDigitsN matches exactly min..max consecutive ASCII digits.
// Fails if fewer than min digits are available, or if more than max consecutive
// digits immediately follow (to avoid silently partial-matching "123" as "12").
func (p *V17Parser) matchDigitsN(minD, maxD int) (V17Token, error) {
	saved := p.savePos()
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	start := p.runePos
	n := 0
	for p.runePos < len(p.input) && n < maxD && p.input[p.runePos] >= '0' && p.input[p.runePos] <= '9' {
		p.runeAdvanceBy(1)
		n++
	}
	// Too few digits?
	if n < minD {
		p.restorePos(saved)
		return V17Token{}, fmt.Errorf("digits: expected %d-%d digits at L%d:C%d, got %d", minD, maxD, line, col, n)
	}
	// Too many (more digits follow immediately)?
	if p.runePos < len(p.input) && p.input[p.runePos] >= '0' && p.input[p.runePos] <= '9' {
		p.restorePos(saved)
		return V17Token{}, fmt.Errorf("digits: too many consecutive digits (expected %d-%d) at L%d:C%d", minD, maxD, line, col)
	}
	p.syncTokenToRune()
	return V17Token{Value: string(p.input[start:p.runePos]), Line: line, Col: col}, nil
}

// matchHexExact matches exactly n consecutive hex digits [0-9a-fA-F]{n} after
// skipping horizontal whitespace.
func (p *V17Parser) matchHexExact(n int) (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	start := p.runePos
	for i := 0; i < n; i++ {
		if p.runePos >= len(p.input) {
			return V17Token{}, fmt.Errorf("hex: expected %d hex digits, got EOF at L%d:C%d", n, line, col)
		}
		ch := p.input[p.runePos]
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return V17Token{}, fmt.Errorf("hex: expected hex digit at L%d:C%d, got %q", line, col, ch)
		}
		p.runeAdvanceBy(1)
	}
	p.syncTokenToRune()
	return V17Token{Value: string(p.input[start:p.runePos]), Line: line, Col: col}, nil
}

// matchSingleQuoted matches a single-quoted string at the current rune position.
// Returns a token whose Value is the raw content between the quotes
// (identical to what V17_STRING_SQ used to carry).
func (p *V17Parser) matchSingleQuoted() (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	if p.runePos >= len(p.input) || p.input[p.runePos] != '\'' {
		return V17Token{}, fmt.Errorf("single_quoted: expected \"'\", got EOF at L%d:C%d", line, col)
	}
	p.runeAdvanceBy(1) // consume opening '
	var sb strings.Builder
	for p.runePos < len(p.input) {
		ch := p.input[p.runePos]
		if ch == '\\' && p.runePos+1 < len(p.input) && p.input[p.runePos+1] == '\'' {
			sb.WriteRune('\\')
			sb.WriteRune('\'')
			p.runeAdvanceBy(2)
			continue
		}
		if ch == '\'' {
			p.runeAdvanceBy(1) // consume closing '
			p.syncTokenToRune()
			return V17Token{Value: sb.String(), Line: line, Col: col}, nil
		}
		sb.WriteRune(ch)
		p.runeAdvanceBy(1)
	}
	return V17Token{}, fmt.Errorf("single_quoted: unterminated string at L%d:C%d", line, col)
}

// matchDoubleQuoted matches a double-quoted string at the current rune position.
// Returns a token whose Value is the raw content between the quotes.
func (p *V17Parser) matchDoubleQuoted() (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	if p.runePos >= len(p.input) || p.input[p.runePos] != '"' {
		return V17Token{}, fmt.Errorf("double_quoted: expected '\"', got EOF at L%d:C%d", line, col)
	}
	p.runeAdvanceBy(1) // consume opening "
	var sb strings.Builder
	for p.runePos < len(p.input) {
		ch := p.input[p.runePos]
		if ch == '\\' && p.runePos+1 < len(p.input) && p.input[p.runePos+1] == '"' {
			sb.WriteRune('\\')
			sb.WriteRune('"')
			p.runeAdvanceBy(2)
			continue
		}
		if ch == '"' {
			p.runeAdvanceBy(1) // consume closing "
			p.syncTokenToRune()
			return V17Token{Value: sb.String(), Line: line, Col: col}, nil
		}
		sb.WriteRune(ch)
		p.runeAdvanceBy(1)
	}
	return V17Token{}, fmt.Errorf("double_quoted: unterminated string at L%d:C%d", line, col)
}

// matchTemplateQuoted matches a backtick-quoted template string.
// Returns a token whose Value is the raw content between the backticks.
func (p *V17Parser) matchTemplateQuoted() (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	if p.runePos >= len(p.input) || p.input[p.runePos] != '`' {
		return V17Token{}, fmt.Errorf("tmpl_quoted: expected backtick, got EOF at L%d:C%d", line, col)
	}
	p.runeAdvanceBy(1) // consume opening `
	var sb strings.Builder
	for p.runePos < len(p.input) {
		ch := p.input[p.runePos]
		if ch == '`' {
			p.runeAdvanceBy(1) // consume closing `
			p.syncTokenToRune()
			return V17Token{Value: sb.String(), Line: line, Col: col}, nil
		}
		sb.WriteRune(ch)
		p.runeAdvanceBy(1)
	}
	return V17Token{}, fmt.Errorf("tmpl_quoted: unterminated template string at L%d:C%d", line, col)
}

// matchRegexpLiteral matches a /pattern/ regexp literal at the current rune
// position. Returns a token whose Value is the pattern between the slashes
// (identical to what V17_REGEXP used to carry).
func (p *V17Parser) matchRegexpLiteral() (V17Token, error) {
	p.skipRuneHorizWS()
	line, col := p.runeLine, p.runeCol
	if p.runePos >= len(p.input) || p.input[p.runePos] != '/' {
		return V17Token{}, fmt.Errorf("regexp_expr: expected '/', got EOF at L%d:C%d", line, col)
	}
	p.runeAdvanceBy(1) // consume opening /
	var sb strings.Builder
	for p.runePos < len(p.input) {
		ch := p.input[p.runePos]
		if ch == '\\' && p.runePos+1 < len(p.input) {
			sb.WriteRune(ch)
			sb.WriteRune(p.input[p.runePos+1])
			p.runeAdvanceBy(2)
			continue
		}
		if ch == '/' {
			p.runeAdvanceBy(1) // consume closing /
			p.syncTokenToRune()
			return V17Token{Value: sb.String(), Line: line, Col: col}, nil
		}
		if ch == '\n' || ch == '\r' {
			return V17Token{}, fmt.Errorf("regexp_expr: unterminated regexp at L%d:C%d", line, col)
		}
		sb.WriteRune(ch)
		p.runeAdvanceBy(1)
	}
	return V17Token{}, fmt.Errorf("regexp_expr: unterminated regexp at L%d:C%d", line, col)
}

// matchCommentTxt matches the body of a comment — all characters up to (but
// not including) the next "(*" or "*)" delimiter.  Accepts any Unicode rune.
// Go's RE2 engine lacks negative-lookahead so this is hand-coded instead of
// using matchRe.
func (p *V17Parser) matchCommentTxt() (V17Token, error) {
	// No WS skip: comment body starts immediately after "(*" including any space.
	line, col := p.runeLine, p.runeCol
	start := p.runePos
	for p.runePos < len(p.input) {
		ch := p.input[p.runePos]
		// Stop at "(*" — start of nested comment
		if ch == '(' && p.runePos+1 < len(p.input) && p.input[p.runePos+1] == '*' {
			break
		}
		// Stop at "*)" — end of comment
		if ch == '*' && p.runePos+1 < len(p.input) && p.input[p.runePos+1] == ')' {
			break
		}
		p.runeAdvanceBy(1)
	}
	p.syncTokenToRune()
	return V17Token{Value: string(p.input[start:p.runePos]), Line: line, Col: col}, nil
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

	// Scan-oriented patterns (no $ anchor) — used by rune-stream match helpers.
	// NL = /([ \t]*[\r\n]+)+/
	reNlScan = regexp.MustCompile(`^([ \t]*[\r\n]+)+`)
	// identifier = /[a-zA-Z_][a-zA-Z0-9_]*/
	reIdentScan         = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`)
	reIdentNameScan     = regexp.MustCompile(`^[\p{L}][\p{L}0-9_]*`) // Unicode ident_name
	reIdentNameScanNoWS = reIdentNameScan                            // alias — used with matchReNoWS
	// regexp flags = /[gimsuyxnA]+/
	reRegexpFlagsScan = regexp.MustCompile(`^[gimsuyxnA]+`)
	// ULID = /[0-7][0-9A-HJKMNP-TV-Z]{25}/
	reUlidScan = regexp.MustCompile(`(?i)^[0-7][0-9A-HJKMNP-TV-Z]{25}`)
	// NanoID = /[A-Za-z0-9_\-]{21}/
	reNanoIdScan = regexp.MustCompile(`^[A-Za-z0-9_-]{21}`)
	// HTTP URL scan
	reHttpUrlScan = regexp.MustCompile(`(?i)^https?://(?:[\w-]+\.)+[\w-]+(?::\d+)?(?:/[^\s]*)?(?:\?[^\s]*)?(?:#[^\s]*)?`)
	// File URL scan
	reFileUrlScan = regexp.MustCompile(`^file:///[^\s\x00]+`)
	// Single regexp flag character
	reRegexpFlagOneScan = regexp.MustCompile(`^[gimsuyxnA]`)
	// UUID v7 version/variant fields (rune-stream scan, no $ anchor)
	reUuidV7VerScan = regexp.MustCompile(`(?i)^7[0-9a-fA-F]{3}`)
	reUuidV7VarScan = regexp.MustCompile(`(?i)^[89abAB][0-9a-fA-F]{3}`)
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
