// directive_processor_test.go — Tests for the DirectiveProcessor (Phase 3).
package parser

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// dpProcess builds a parser for src, skips BOF, applies fn to get a node,
// then runs the directive processor over that node and returns the result.
func dpProcess(src string, fn func(*Parser) *Node) (*Node, *DirectiveProcessor) {
	p := NewParser(mustLex(src))
	p.advance() // skip BOF
	n := fn(p)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	return n, dp
}

// ---------------------------------------------------------------------------
// NewDirectiveProcessor
// ---------------------------------------------------------------------------

func TestNewDirectiveProcessor(t *testing.T) {
	dp := NewDirectiveProcessor()
	if dp == nil {
		t.Fatal("expected non-nil DirectiveProcessor")
	}
	if dp.HasErrors() {
		t.Fatal("fresh processor should have no errors")
	}
}

// ---------------------------------------------------------------------------
// UNIQUE
// ---------------------------------------------------------------------------

func TestDirectiveUnique_NoViolation(t *testing.T) {
	// assign_lhs "x, y" → UNIQUE<ident_name, ident_name> — both unique
	_, dp := dpProcess("x, y", func(p *Parser) *Node {
		return p.parseAssignLHS()
	})
	if dp.HasErrors() {
		t.Fatalf("expected no UNIQUE errors, got: %v", dp.Errors())
	}
}

func TestDirectiveUnique_Violation(t *testing.T) {
	// assign_lhs "x, x" → UNIQUE should fire
	_, dp := dpProcess("x, x", func(p *Parser) *Node {
		return p.parseAssignLHS()
	})
	if !dp.HasErrors() {
		t.Fatal("expected UNIQUE violation error, got none")
	}
	found := false
	for _, e := range dp.Errors() {
		if contains(e.Msg, "UNIQUE") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected UNIQUE error message, got: %v", dp.Errors())
	}
}

// ---------------------------------------------------------------------------
// RANGE
// ---------------------------------------------------------------------------

func TestDirectiveRange_WithinBounds(t *testing.T) {
	// Build a RANGE 1..10 directive wrapping integer 5.
	inner := NewNode(NodeInteger, Pos{}, NewTokenNode(Token{
		Type: TOK_INTEGER, Value: "5", Line: 1, Col: 1,
	}))
	n := NewDirectiveNode(NodeDirectiveRange, "1..10", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if dp.HasErrors() {
		t.Fatalf("expected no RANGE error for 5 in [1,10], got: %v", dp.Errors())
	}
}

func TestDirectiveRange_OutOfBounds(t *testing.T) {
	inner := NewNode(NodeInteger, Pos{}, NewTokenNode(Token{
		Type: TOK_INTEGER, Value: "99", Line: 1, Col: 1,
	}))
	n := NewDirectiveNode(NodeDirectiveRange, "1..10", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if !dp.HasErrors() {
		t.Fatal("expected RANGE violation, got no errors")
	}
}

func TestDirectiveRange_MalformedArg(t *testing.T) {
	inner := NewNode(NodeInteger, Pos{}, NewTokenNode(Token{
		Type: TOK_INTEGER, Value: "5", Line: 1, Col: 1,
	}))
	n := NewDirectiveNode(NodeDirectiveRange, "bad-arg", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if !dp.HasErrors() {
		t.Fatal("expected error for malformed RANGE arg")
	}
}

// ---------------------------------------------------------------------------
// TYPE_OF
// ---------------------------------------------------------------------------

func TestDirectiveTypeOf_AnnotatesChild(t *testing.T) {
	inner := NewNode(NodeIdentRef, Pos{}, NewTokenNode(Token{
		Type: TOK_IDENT, Value: "myVar", Line: 1, Col: 1,
	}))
	n := NewDirectiveNode(NodeDirectiveTypeOf, "string", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if dp.HasErrors() {
		t.Fatalf("unexpected errors: %v", dp.Errors())
	}
	// Child should have TypeRef set to "string"
	if inner.Meta.TypeRef == nil {
		t.Fatal("expected TypeRef to be set on child node")
	}
	if *inner.Meta.TypeRef != "string" {
		t.Fatalf("expected TypeRef=string, got %q", *inner.Meta.TypeRef)
	}
}

// ---------------------------------------------------------------------------
// VALUE_OF / ADDRESS_OF
// ---------------------------------------------------------------------------

func TestDirectiveValueOf(t *testing.T) {
	inner := NewNode(NodeIdentRef, Pos{})
	n := NewDirectiveNode(NodeDirectiveValueOf, "", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if !inner.Meta.IsValueOf {
		t.Fatal("expected IsValueOf=true on child after VALUE_OF directive")
	}
}

func TestDirectiveAddressOf(t *testing.T) {
	inner := NewNode(NodeIdentRef, Pos{})
	n := NewDirectiveNode(NodeDirectiveAddressOf, "", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if !inner.Meta.IsAddressOf {
		t.Fatal("expected IsAddressOf=true on child after ADDRESS_OF directive")
	}
}

// ---------------------------------------------------------------------------
// RETURN
// ---------------------------------------------------------------------------

func TestDirectiveReturn_Colon_MeansValueOf(t *testing.T) {
	inner := NewNode(NodeIdentRef, Pos{})
	n := NewDirectiveNode(NodeDirectiveReturn, ":", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if !inner.Meta.IsValueOf {
		t.Fatal("expected IsValueOf=true for RETURN ':'")
	}
}

func TestDirectiveReturn_Tilde_MeansAddressOf(t *testing.T) {
	inner := NewNode(NodeIdentRef, Pos{})
	n := NewDirectiveNode(NodeDirectiveReturn, "~", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if !inner.Meta.IsAddressOf {
		t.Fatal("expected IsAddressOf=true for RETURN '~'")
	}
}

// ---------------------------------------------------------------------------
// UNIFORM
// ---------------------------------------------------------------------------

func TestDirectiveUniform_HomogeneousLeaves(t *testing.T) {
	// Build [1, 2, 3] — all INTEGER tokens.
	leaves := []*Node{
		NewTokenNode(Token{Type: TOK_INTEGER, Value: "1"}),
		NewTokenNode(Token{Type: TOK_INTEGER, Value: "2"}),
		NewTokenNode(Token{Type: TOK_INTEGER, Value: "3"}),
	}
	inner := NewNode(NodeArrayInit, Pos{}, leaves...)
	n := NewDirectiveNode(NodeDirectiveUniform, "INTEGER", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if dp.HasErrors() {
		t.Fatalf("expected no UNIFORM errors for homogeneous [1,2,3]: %v", dp.Errors())
	}
}

func TestDirectiveUniform_HeterogeneousLeaves(t *testing.T) {
	// Mix INTEGER and STRING — should produce UNIFORM violation.
	inner := NewNode(NodeArrayInit, Pos{},
		NewTokenNode(Token{Type: TOK_INTEGER, Value: "1"}),
		NewTokenNode(Token{Type: TOK_STRING, Value: "hello"}),
	)
	n := NewDirectiveNode(NodeDirectiveUniform, "INTEGER", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if !dp.HasErrors() {
		t.Fatal("expected UNIFORM violation for mixed [1, 'hello'], got none")
	}
}

// ---------------------------------------------------------------------------
// INFER
// ---------------------------------------------------------------------------

func TestDirectiveInfer_FromLeaf(t *testing.T) {
	inner := NewNode(NodeInteger, Pos{}, NewTokenNode(Token{
		Type: TOK_INTEGER, Value: "42", Line: 1, Col: 1,
	}))
	n := NewDirectiveNode(NodeDirectiveInfer, "", Pos{}, inner)
	dp := NewDirectiveProcessor()
	dp.Process(n)
	if n.Meta.InferredType == "" {
		t.Fatal("expected InferredType to be set after INFER processing")
	}
}

func TestDirectiveInfer_FromSymbolTable(t *testing.T) {
	// Pre-populate symbol table with x → "string".
	dp := NewDirectiveProcessor()
	dp.symbols["x"] = "string"

	identTok := NewTokenNode(Token{Type: TOK_IDENT, Value: "x"})
	inner := NewNode(NodeIdentName, Pos{}, identTok)
	inner.IsLeaf() // hint
	inner.Tok = &Token{Type: TOK_IDENT, Value: "x"}
	inner.Kind = NodeIdentName

	n := NewDirectiveNode(NodeDirectiveInfer, "", Pos{}, inner)
	dp.Process(n)
	if n.Meta.InferredType != "string" {
		t.Fatalf("expected InferredType=string, got %q", n.Meta.InferredType)
	}
}

// ---------------------------------------------------------------------------
// Symbol table population via NodeAssignment
// ---------------------------------------------------------------------------

func TestDirectiveProcessor_RecordsAssignment(t *testing.T) {
	// Parse "x : 42" as an assignment, then run processor.
	_, dp := dpProcess("x : 42", func(p *Parser) *Node {
		return p.parseAssignment()
	})
	// After processing, "x" should be in the symbol table with some type.
	if _, ok := dp.symbols["x"]; !ok {
		t.Fatal("expected 'x' to be recorded in symbol table after assignment")
	}
}

// ---------------------------------------------------------------------------
// Integration: Process on nil root
// ---------------------------------------------------------------------------

func TestDirectiveProcess_NilRoot(t *testing.T) {
	dp := NewDirectiveProcessor()
	// Should not panic.
	dp.Process(nil)
	if dp.HasErrors() {
		t.Fatal("processing nil root should produce no errors")
	}
}

// ---------------------------------------------------------------------------
// helper: contains
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}()))
}
