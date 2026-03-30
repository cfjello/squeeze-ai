// parser.go — Parser struct and all helper methods for the Squeeze V3 parser.
//
// This is Phase 2 infrastructure from parser_v3.md:
//   - Parser struct: token stream, position, lookahead, context flags
//   - Token navigation: peek, peek2, advance, expect, match, skipNL, skipEOL
//   - Backtracking: save/restore cursor (try-parse pattern)
//   - Error accumulation: non-fatal errors collected, parse continues
//   - Directive bracket helpers: parseDirectiveArg, parseDirectiveBrackets
//   - Context flags: inExpression / inDirective / inScope (disambiguate "<")
//
// Grammar rules (V1 and V3) are implemented in rules_v1.go and rules_v3.go.
package parser

import (
	"fmt"
	"strings"
)

// =============================================================================
// ParseError
// =============================================================================

// ParseError records a single non-fatal parse error with its location.
type ParseError struct {
	Pos Pos
	Msg string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("L%d:C%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

// =============================================================================
// Parser
// =============================================================================

const maxDepth = 512 // recursion depth guard

// Parser holds the full token stream produced by the Lexer and all state
// needed during the recursive-descent parse.
type Parser struct {
	tokens []Token // full flat token stream (including BOF and EOF)
	pos    int     // index of the current (not-yet-consumed) token

	errors []ParseError // accumulated non-fatal errors

	// Context flags — drive disambiguation of "<" and "/" (see parser_v3.md §4).
	inExpression bool // inside a calc / compare expression — '<' is a compare op
	inDirective  bool // parsing a directive argument list — '<' is a bracket
	inScope      bool // parsing a scope_assign body — '<' is a scope delimiter

	depth int // current recursion depth (guards against infinite loops)
}

// NewParser creates a Parser from a pre-lexed token slice.
// The slice must start with TOK_BOF and end with TOK_EOF (as produced by
// Lexer.Tokenize).
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// =============================================================================
// Token navigation primitives
// =============================================================================

// cur returns the token at the current position without consuming it.
// Always safe: returns a synthetic EOF if pos is out of range.
func (p *Parser) cur() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TOK_EOF}
	}
	return p.tokens[p.pos]
}

// peek returns the current token (alias for cur, clearer at call sites).
func (p *Parser) peek() Token { return p.cur() }

// peek2 returns the token one position ahead of the current without consuming.
func (p *Parser) peek2() Token {
	if p.pos+1 >= len(p.tokens) {
		return Token{Type: TOK_EOF}
	}
	return p.tokens[p.pos+1]
}

// peekAt returns the token n positions ahead of the current without consuming.
func (p *Parser) peekAt(n int) Token {
	idx := p.pos + n
	if idx >= len(p.tokens) {
		return Token{Type: TOK_EOF}
	}
	return p.tokens[idx]
}

// advance consumes the current token and moves pos forward.
// Returns the consumed token.  Safe to call at EOF (returns EOF token).
func (p *Parser) advance() Token {
	tok := p.cur()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

// skipNL skips zero or more consecutive TOK_NL tokens.
func (p *Parser) skipNL() {
	for p.peek().Type == TOK_NL {
		p.advance()
	}
}

// skipEOL skips zero or more consecutive EOL tokens (NL, EOF, or ";").
// Used at statement boundaries where the grammar says { EOL }.
func (p *Parser) skipEOL() {
	for {
		t := p.peek().Type
		if t == TOK_NL || t == TOK_SEMICOLON || t == TOK_EOF {
			p.advance()
		} else {
			break
		}
	}
}

// isEOL reports whether the current token is an end-of-line marker
// (NL, ";", EOF, or BOF — BOF counts as a virtual NL per the grammar).
func (p *Parser) isEOL() bool {
	switch p.peek().Type {
	case TOK_NL, TOK_SEMICOLON, TOK_EOF, TOK_BOF:
		return true
	}
	return false
}

// curPos returns the source position of the current token.
func (p *Parser) curPos() Pos {
	tok := p.cur()
	return Pos{Line: tok.Line, Col: tok.Col}
}

// =============================================================================
// Match / expect helpers
// =============================================================================

// match reports whether the current token has the given type.
func (p *Parser) match(t TokenType) bool {
	return p.peek().Type == t
}

// matchAny reports whether the current token type is any of the supplied types.
func (p *Parser) matchAny(types ...TokenType) bool {
	cur := p.peek().Type
	for _, t := range types {
		if cur == t {
			return true
		}
	}
	return false
}

// matchValue reports whether the current token has the given type AND value.
func (p *Parser) matchValue(t TokenType, val string) bool {
	tok := p.peek()
	return tok.Type == t && tok.Value == val
}

// consume advances past the current token if it matches t, returning a
// NodeToken leaf.  On mismatch it records an error and returns an error node
// WITHOUT advancing (so the caller can attempt recovery).
func (p *Parser) consume(t TokenType) *Node {
	tok := p.peek()
	if tok.Type != t {
		p.errorf("expected %s, got %s %q", t, tok.Type, tok.Value)
		return NewErrorNode(fmt.Sprintf("expected %s", t), p.curPos())
	}
	p.advance()
	return NewTokenNode(tok)
}

// consumeValue advances past and returns the current token as a NodeToken leaf
// if it matches both type and value exactly.  Otherwise records an error.
func (p *Parser) consumeValue(t TokenType, val string) *Node {
	tok := p.peek()
	if tok.Type != t || tok.Value != val {
		p.errorf("expected %s %q, got %s %q", t, val, tok.Type, tok.Value)
		return NewErrorNode(fmt.Sprintf("expected %s %q", t, val), p.curPos())
	}
	p.advance()
	return NewTokenNode(tok)
}

// consumeAny advances past and returns the current token as a NodeToken leaf
// if its type is one of the given types.  Otherwise records an error and
// returns an error node without advancing.
func (p *Parser) consumeAny(types ...TokenType) *Node {
	tok := p.peek()
	for _, t := range types {
		if tok.Type == t {
			p.advance()
			return NewTokenNode(tok)
		}
	}
	var names []string
	for _, t := range types {
		names = append(names, t.String())
	}
	p.errorf("expected one of [%s], got %s %q", strings.Join(names, ", "), tok.Type, tok.Value)
	return NewErrorNode("unexpected token", p.curPos())
}

// =============================================================================
// Backtracking — save / restore
// =============================================================================

// cursor is a saved parser position used for backtracking.
type cursor struct {
	pos          int
	errCount     int
	inExpression bool
	inDirective  bool
	inScope      bool
}

// save captures the current parser position so it can be restored on failure.
func (p *Parser) save() cursor {
	return cursor{
		pos:          p.pos,
		errCount:     len(p.errors),
		inExpression: p.inExpression,
		inDirective:  p.inDirective,
		inScope:      p.inScope,
	}
}

// restore rewinds the parser to a previously saved position.
// Errors recorded after the save point are discarded (backtracking semantics).
func (p *Parser) restore(c cursor) {
	p.pos = c.pos
	p.errors = p.errors[:c.errCount]
	p.inExpression = c.inExpression
	p.inDirective = c.inDirective
	p.inScope = c.inScope
}

// try runs fn with backtracking: if fn returns nil the parser position is
// restored and nil is returned to the caller; otherwise the result is kept.
// This implements optional / alternative production rules cleanly.
func (p *Parser) try(fn func() *Node) *Node {
	c := p.save()
	n := fn()
	if n == nil || n.IsError() {
		p.restore(c)
		return nil
	}
	return n
}

// =============================================================================
// Depth guard
// =============================================================================

// enter increments the recursion depth counter.  Returns false (and records an
// error) when the limit is exceeded, protecting against infinite loops in
// mutually recursive rules.
func (p *Parser) enter(rule string) bool {
	p.depth++
	if p.depth > maxDepth {
		p.errorf("maximum recursion depth exceeded in rule %q", rule)
		p.depth--
		return false
	}
	return true
}

// leave decrements the recursion depth counter.  Always call as defer p.leave()
// after a successful p.enter().
func (p *Parser) leave() { p.depth-- }

// =============================================================================
// Error helpers
// =============================================================================

// errorf records a non-fatal parse error at the current position.
func (p *Parser) errorf(format string, args ...any) {
	pos := p.curPos()
	p.errors = append(p.errors, ParseError{
		Pos: pos,
		Msg: fmt.Sprintf(format, args...),
	})
}

// Errors returns all accumulated parse errors.
func (p *Parser) Errors() []ParseError { return p.errors }

// HasErrors reports whether any parse errors have been recorded.
func (p *Parser) HasErrors() bool { return len(p.errors) > 0 }

// =============================================================================
// Directive bracket helpers
// =============================================================================

// DirectiveBracketContent holds the result of parseDirectiveBrackets.
type DirectiveBracketContent struct {
	Inner *Node // the sub-tree parsed inside < … >
}

// parseDirectiveBrackets parses the < innerFn() > wrapper used by all
// directive productions (UNIQUE<…>, TYPE_OF token<…>, etc.).
// It sets inDirective=true to prevent '<' from being misread as a compare op.
func (p *Parser) parseDirectiveBrackets(innerFn func() *Node) *Node {
	prev := p.inDirective
	p.inDirective = true
	defer func() { p.inDirective = prev }()

	if !p.match(TOK_LT) {
		p.errorf("expected '<' to open directive bracket, got %s", p.peek().Type)
		return nil
	}
	p.advance() // consume '<'

	inner := innerFn()
	if inner == nil {
		p.errorf("empty directive bracket content")
		inner = NewErrorNode("empty directive", p.curPos())
	}

	if !p.match(TOK_GT) {
		p.errorf("expected '>' to close directive bracket, got %s", p.peek().Type)
	} else {
		p.advance() // consume '>'
	}
	return inner
}

// parseDirectiveUnique parses   UNIQUE < innerFn() >
// and wraps the result in a NodeDirectiveUnique node.
func (p *Parser) parseDirectiveUnique(pos Pos, innerFn func() *Node) *Node {
	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveUnique, "", pos, inner)
}

// parseDirectiveTypeOf parses   TYPE_OF token < innerFn() >
// where token is the grammar token name supplied as the directive argument.
// If the next token is the INFER keyword the arg is set to "INFER".
func (p *Parser) parseDirectiveTypeOf(pos Pos, innerFn func() *Node) *Node {
	arg := ""
	// Optional argument: grammar rule name or INFER
	if p.match(TOK_INFER) {
		arg = "INFER"
		p.advance()
	} else if p.peek().Type == TOK_IDENT {
		arg = p.advance().Value
	}
	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveTypeOf, arg, pos, inner)
}

// parseDirectiveRange parses   RANGE n..m < innerFn() >
// and stores the range string "n..m" as the directive argument.
func (p *Parser) parseDirectiveRange(pos Pos, innerFn func() *Node) *Node {
	// Expect: integer ".." integer
	if !p.match(TOK_INTEGER) {
		p.errorf("RANGE directive: expected integer lower bound, got %s", p.peek().Type)
		return nil
	}
	lo := p.advance().Value
	if !p.match(TOK_DOTDOT) {
		p.errorf("RANGE directive: expected '..', got %s", p.peek().Type)
		return nil
	}
	p.advance()
	if !p.match(TOK_INTEGER) {
		p.errorf("RANGE directive: expected integer upper bound, got %s", p.peek().Type)
		return nil
	}
	hi := p.advance().Value
	arg := lo + ".." + hi

	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveRange, arg, pos, inner)
}

// parseDirectiveUniform parses   UNIFORM (token|INFER) < innerFn() >
func (p *Parser) parseDirectiveUniform(pos Pos, innerFn func() *Node) *Node {
	arg := ""
	if p.match(TOK_INFER) {
		arg = "INFER"
		p.advance()
	} else if p.peek().Type == TOK_IDENT {
		arg = p.advance().Value
	}
	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveUniform, arg, pos, inner)
}

// parseDirectiveInfer parses   INFER < innerFn() >
func (p *Parser) parseDirectiveInfer(pos Pos, innerFn func() *Node) *Node {
	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveInfer, "", pos, inner)
}

// parseDirectiveValueOf parses   VALUE_OF < innerFn() >
func (p *Parser) parseDirectiveValueOf(pos Pos, innerFn func() *Node) *Node {
	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveValueOf, "", pos, inner)
}

// parseDirectiveAddressOf parses   ADDRESS_OF < innerFn() >
func (p *Parser) parseDirectiveAddressOf(pos Pos, innerFn func() *Node) *Node {
	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveAddressOf, "", pos, inner)
}

// parseDirectiveReturn parses   RETURN (":"| "~") < innerFn() >
// The argument is ":" (immutable → VALUE_OF) or "~" (mutable → ADDRESS_OF).
func (p *Parser) parseDirectiveReturn(pos Pos, innerFn func() *Node) *Node {
	if !p.matchAny(TOK_COLON, TOK_TILDE) {
		p.errorf("RETURN directive: expected ':' or '~', got %s", p.peek().Type)
		return nil
	}
	arg := p.advance().Value
	inner := p.parseDirectiveBrackets(innerFn)
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveReturn, arg, pos, inner)
}

// =============================================================================
// Context helpers
// =============================================================================

// withExpression runs fn with inExpression=true (e.g. inside a calc expression
// so that '<' is treated as a comparison operator, not a scope delimiter or
// directive bracket).
func (p *Parser) withExpression(fn func() *Node) *Node {
	prev := p.inExpression
	p.inExpression = true
	defer func() { p.inExpression = prev }()
	return fn()
}

// withScope runs fn with inScope=true (inside a scope_assign body).
func (p *Parser) withScope(fn func() *Node) *Node {
	prev := p.inScope
	p.inScope = true
	defer func() { p.inScope = prev }()
	return fn()
}

// ltIsCompare reports whether '<' at the current position should be treated
// as a comparison operator rather than a directive bracket or scope delimiter.
func (p *Parser) ltIsCompare() bool { return p.inExpression && !p.inDirective }

// ltIsScope reports whether '<' at the current position opens a scope block.
func (p *Parser) ltIsScope() bool { return p.inScope && !p.inDirective && !p.inExpression }

// =============================================================================
// Lookahead helpers for disambiguation
// =============================================================================

// lookAheadIsAssignOper scans forward (from current pos) skipping
// ident_name tokens and dots, then checks whether an assignment operator
// follows.  Used to tell scope_unit content apart.
func (p *Parser) lookAheadIsAssignOper() bool {
	i := 0
	// Skip over ident-dotted prefix: ident { "." ident }
	for {
		tok := p.peekAt(i)
		if tok.Type == TOK_IDENT {
			i++
			if p.peekAt(i).Type == TOK_DOT {
				i++ // skip the dot, continue
			} else {
				break
			}
		} else {
			break
		}
	}
	// Now check if what follows is an assignment operator.
	return isAssignOperToken(p.peekAt(i).Type)
}

// isAssignOperToken reports whether a TokenType is any assignment operator.
func isAssignOperToken(t TokenType) bool {
	switch t {
	case TOK_COLON, TOK_TILDE, TOK_READONLY,
		TOK_IADD_IMM, TOK_ISUB_IMM, TOK_IMUL_IMM, TOK_IDIV_IMM,
		TOK_IADD_MUT, TOK_ISUB_MUT, TOK_IMUL_MUT, TOK_IDIV_MUT:
		return true
	}
	return false
}

// isNumericOperToken reports whether a TokenType is a numeric binary operator.
func isNumericOperToken(t TokenType) bool {
	switch t {
	case TOK_PLUS, TOK_MINUS, TOK_STAR, TOK_POW, TOK_SLASH, TOK_PERCENT:
		return true
	}
	return false
}

// isLogicOperToken reports whether a token type is a logic binary operator.
func isLogicOperToken(t TokenType) bool {
	switch t {
	case TOK_AMP, TOK_PIPE, TOK_CARET:
		return true
	}
	return false
}

// isCompareOperToken reports whether a token type is a compare operator.
func isCompareOperToken(t TokenType) bool {
	switch t {
	case TOK_NEQ, TOK_EQ, TOK_GT, TOK_GEQ, TOK_LT, TOK_LEQ:
		return true
	}
	return false
}

// isEmptyDeclToken reports whether the current token starts an empty_decl.
func isEmptyDeclToken(t TokenType) bool {
	switch t {
	case TOK_EMPTY_ARR, TOK_STREAM, TOK_REGEXP_DECL,
		TOK_EMPTY_OBJ, TOK_EMPTY_STR_D, TOK_EMPTY_STR_S, TOK_EMPTY_STR_T:
		return true
	}
	return false
}

// isConstantFirst reports whether the current token can begin a constant.
func isConstantFirst(t TokenType) bool {
	switch t {
	case TOK_INTEGER, TOK_DECIMAL, TOK_PLUS, TOK_MINUS, // numeric_const
		TOK_STRING,          // string
		TOK_REGEXP,          // regexp
		TOK_TRUE, TOK_FALSE: // boolean
		return true
	}
	return false
}
