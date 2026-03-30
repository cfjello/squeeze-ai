// parser_test.go — Unit tests for the Parser struct and helpers.
package parser

import (
	"testing"
)

// helper: lex a string, panic on error.
func mustLex(src string) []Token {
	toks, err := NewLexer(src).Tokenize()
	if err != nil {
		panic(err)
	}
	return toks
}

// helper: build a parser from a source string.
func parserFor(src string) *Parser {
	return NewParser(mustLex(src))
}

// =============================================================================
// Basic navigation
// =============================================================================

func TestParserCurAndAdvance(t *testing.T) {
	p := parserFor("42")
	// tokens: BOF  INTEGER("42")  EOF
	if p.cur().Type != TOK_BOF {
		t.Fatalf("expected BOF, got %s", p.cur().Type)
	}
	p.advance()
	if p.cur().Type != TOK_INTEGER {
		t.Fatalf("expected INTEGER, got %s", p.cur().Type)
	}
	p.advance()
	if p.cur().Type != TOK_EOF {
		t.Fatalf("expected EOF, got %s", p.cur().Type)
	}
	// advancing past EOF is safe
	tok := p.advance()
	if tok.Type != TOK_EOF {
		t.Fatalf("expected EOF on extra advance, got %s", tok.Type)
	}
}

func TestParserPeekAndPeek2(t *testing.T) {
	p := parserFor("1 + 2")
	p.advance() // consume BOF
	// cur = INTEGER(1)
	if p.peek().Type != TOK_INTEGER {
		t.Fatalf("peek: expected INTEGER, got %s", p.peek().Type)
	}
	if p.peek().Value != "1" {
		t.Fatalf("peek: expected value 1, got %q", p.peek().Value)
	}
	if p.peek2().Type != TOK_PLUS {
		t.Fatalf("peek2: expected PLUS, got %s", p.peek2().Type)
	}
}

func TestParserSkipNL(t *testing.T) {
	p := parserFor("\n\n42")
	p.advance() // skip BOF
	p.skipNL()
	if got := p.cur(); got.Type != TOK_INTEGER {
		t.Fatalf("after skipNL expected INTEGER, got %s", got.Type)
	}
}

func TestParserMatch(t *testing.T) {
	p := parserFor("true")
	p.advance() // skip BOF
	if !p.match(TOK_TRUE) {
		t.Fatal("match(TOK_TRUE) should be true")
	}
	if p.match(TOK_FALSE) {
		t.Fatal("match(TOK_FALSE) should be false for 'true'")
	}
}

func TestParserMatchAny(t *testing.T) {
	p := parserFor("false")
	p.advance() // skip BOF
	if !p.matchAny(TOK_TRUE, TOK_FALSE) {
		t.Fatal("matchAny should match TOK_FALSE")
	}
	if p.matchAny(TOK_INTEGER, TOK_STRING) {
		t.Fatal("matchAny should not match integer/string for 'false'")
	}
}

func TestParserMatchValue(t *testing.T) {
	p := parserFor("hello world")
	p.advance() // skip BOF
	if !p.matchValue(TOK_IDENT, "hello world") {
		t.Fatalf("matchValue: expected true for 'hello world', got false; cur=%s %q",
			p.peek().Type, p.peek().Value)
	}
}

// =============================================================================
// consume helpers
// =============================================================================

func TestParserConsume(t *testing.T) {
	p := parserFor("99")
	p.advance() // skip BOF

	node := p.consume(TOK_INTEGER)
	if node.IsError() {
		t.Fatalf("consume(TOK_INTEGER): unexpected error: %v", p.errors)
	}
	if node.Tok == nil || node.Tok.Value != "99" {
		t.Fatalf("consume(TOK_INTEGER): wrong token, got %+v", node.Tok)
	}
	if p.HasErrors() {
		t.Fatalf("no error expected but got: %v", p.errors)
	}
}

func TestParserConsumeWrongType(t *testing.T) {
	p := parserFor("hello")
	p.advance() // skip BOF

	node := p.consume(TOK_INTEGER) // wrong
	if !node.IsError() {
		t.Fatal("expected error node on type mismatch")
	}
	if !p.HasErrors() {
		t.Fatal("expected error recorded")
	}
	// position should NOT have advanced
	if p.cur().Type != TOK_IDENT {
		t.Fatalf("expected parser to stay on IDENT after failed consume, got %s", p.cur().Type)
	}
}

func TestParserConsumeValue(t *testing.T) {
	p := parserFor(": 1")
	p.advance() // skip BOF

	node := p.consumeValue(TOK_COLON, ":")
	if node.IsError() || p.HasErrors() {
		t.Fatalf("consumeValue failed: %v", p.errors)
	}
	if p.cur().Type != TOK_INTEGER {
		t.Fatalf("expected INTEGER after consumeValue, got %s", p.cur().Type)
	}
}

func TestParserConsumeAny(t *testing.T) {
	p := parserFor("~")
	p.advance() // skip BOF

	node := p.consumeAny(TOK_COLON, TOK_TILDE)
	if node.IsError() {
		t.Fatalf("consumeAny failed: %v", p.errors)
	}
	if node.Tok.Type != TOK_TILDE {
		t.Fatalf("expected TILDE, got %s", node.Tok.Type)
	}
}

// =============================================================================
// Save / restore / try
// =============================================================================

func TestParserSaveRestore(t *testing.T) {
	p := parserFor("1 + 2")
	p.advance() // skip BOF

	c := p.save()
	p.advance() // consume 1
	p.advance() // consume +
	if p.cur().Type != TOK_INTEGER {
		t.Fatalf("mid-parse: expected INTEGER, got %s", p.cur().Type)
	}

	p.restore(c)
	if p.cur().Type != TOK_INTEGER || p.cur().Value != "1" {
		t.Fatalf("after restore: expected INTEGER 1, got %s %q", p.cur().Type, p.cur().Value)
	}
}

func TestParserSaveRestoreDiscardsErrors(t *testing.T) {
	p := parserFor("hello")
	p.advance() // skip BOF

	c := p.save()
	p.consume(TOK_INTEGER) // produces an error
	if !p.HasErrors() {
		t.Fatal("expected error after bad consume")
	}
	p.restore(c)
	if p.HasErrors() {
		t.Fatal("errors should be discarded after restore")
	}
}

func TestParserTrySuccess(t *testing.T) {
	p := parserFor("true")
	p.advance() // skip BOF

	result := p.try(func() *Node {
		return p.consume(TOK_TRUE)
	})
	if result == nil {
		t.Fatal("try: expected success")
	}
}

func TestParserTryFailure(t *testing.T) {
	p := parserFor("hello")
	p.advance() // skip BOF

	origPos := p.pos
	result := p.try(func() *Node {
		return p.consume(TOK_INTEGER) // will fail
	})
	if result != nil {
		t.Fatal("try: expected nil on failure")
	}
	if p.pos != origPos {
		t.Fatal("try: pos should be restored after failure")
	}
	if p.HasErrors() {
		t.Fatal("try: errors should be discarded on failure")
	}
}

// =============================================================================
// Depth guard
// =============================================================================

func TestParserDepthGuard(t *testing.T) {
	p := parserFor("")
	p.depth = maxDepth
	if p.enter("test") {
		t.Fatal("enter should fail when depth == maxDepth")
	}
	if !p.HasErrors() {
		t.Fatal("enter should record an error when depth is exceeded")
	}
}

func TestParserEnterLeave(t *testing.T) {
	p := parserFor("")
	if !p.enter("rule") {
		t.Fatal("enter should succeed on fresh parser")
	}
	if p.depth != 1 {
		t.Fatalf("depth should be 1 after enter, got %d", p.depth)
	}
	p.leave()
	if p.depth != 0 {
		t.Fatalf("depth should be 0 after leave, got %d", p.depth)
	}
}

// =============================================================================
// Context helpers
// =============================================================================

func TestParserContextFlags(t *testing.T) {
	p := parserFor("<")
	p.advance() // skip BOF

	// Default: '<' is not compare, not scope
	if p.ltIsCompare() {
		t.Fatal("ltIsCompare should be false by default")
	}
	if p.ltIsScope() {
		t.Fatal("ltIsScope should be false by default")
	}

	// withExpression sets inExpression
	p.withExpression(func() *Node {
		if !p.ltIsCompare() {
			t.Error("ltIsCompare should be true inside withExpression")
		}
		return nil
	})
	// flag is restored outside
	if p.inExpression {
		t.Fatal("inExpression should be restored after withExpression")
	}

	// withScope sets inScope
	p.inScope = false
	p.withScope(func() *Node {
		if !p.ltIsScope() {
			t.Error("ltIsScope should be true inside withScope")
		}
		return nil
	})
	if p.inScope {
		t.Fatal("inScope should be restored after withScope")
	}
}

// =============================================================================
// Directive bracket helpers
// =============================================================================

func TestParserDirectiveBrackets(t *testing.T) {
	// Simulate: < 42 >
	p := parserFor("< 42 >")
	p.advance() // skip BOF

	inner := p.parseDirectiveBrackets(func() *Node {
		if !p.match(TOK_INTEGER) {
			t.Error("inside directive brackets: expected INTEGER")
			return nil
		}
		tok := p.advance()
		return NewTokenNode(tok)
	})
	if inner == nil || inner.IsError() {
		t.Fatalf("parseDirectiveBrackets: unexpected nil or error, errors=%v", p.errors)
	}
	if inner.Tok == nil || inner.Tok.Value != "42" {
		t.Fatalf("parseDirectiveBrackets: expected integer 42, got %+v", inner.Tok)
	}
	// should NOT be in directive mode after the call
	if p.inDirective {
		t.Fatal("inDirective should be restored after parseDirectiveBrackets")
	}
}

func TestParserDirectiveUnique(t *testing.T) {
	p := parserFor("< true >")
	p.advance() // skip BOF

	pos := p.curPos()
	node := p.parseDirectiveUnique(pos, func() *Node {
		tok := p.advance()
		return NewTokenNode(tok)
	})
	if node == nil || node.IsError() {
		t.Fatalf("parseDirectiveUnique failed: %v", p.errors)
	}
	if node.Kind != NodeDirectiveUnique {
		t.Fatalf("expected NodeDirectiveUnique, got %s", node.Kind)
	}
}

func TestParserDirectiveRange(t *testing.T) {
	p := parserFor("1..128 < true >")
	p.advance() // skip BOF

	pos := p.curPos()
	node := p.parseDirectiveRange(pos, func() *Node {
		tok := p.advance()
		return NewTokenNode(tok)
	})
	if node == nil || node.IsError() {
		t.Fatalf("parseDirectiveRange failed: %v", p.errors)
	}
	if node.Kind != NodeDirectiveRange {
		t.Fatalf("expected NodeDirectiveRange, got %s", node.Kind)
	}
	if node.Meta.DirectiveArg != "1..128" {
		t.Fatalf("expected arg '1..128', got %q", node.Meta.DirectiveArg)
	}
}

// =============================================================================
// Lookahead helpers
// =============================================================================

func TestIsAssignOperToken(t *testing.T) {
	cases := []struct {
		t    TokenType
		want bool
	}{
		{TOK_COLON, true},
		{TOK_TILDE, true},
		{TOK_READONLY, true},
		{TOK_IADD_IMM, true},
		{TOK_PLUS, false},
		{TOK_INTEGER, false},
	}
	for _, c := range cases {
		if got := isAssignOperToken(c.t); got != c.want {
			t.Errorf("isAssignOperToken(%s) = %v, want %v", c.t, got, c.want)
		}
	}
}

func TestIsConstantFirst(t *testing.T) {
	yes := []TokenType{TOK_INTEGER, TOK_DECIMAL, TOK_PLUS, TOK_MINUS, TOK_STRING, TOK_REGEXP, TOK_TRUE, TOK_FALSE}
	no := []TokenType{TOK_IDENT, TOK_NL, TOK_EOF}
	for _, tok := range yes {
		if !isConstantFirst(tok) {
			t.Errorf("isConstantFirst(%s) should be true", tok)
		}
	}
	for _, tok := range no {
		if isConstantFirst(tok) {
			t.Errorf("isConstantFirst(%s) should be false", tok)
		}
	}
}
