// parse_test.go — Tests for the top-level Parse entry point and parseProgram.
package parser

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Parse (top-level API)
// ---------------------------------------------------------------------------

func TestParse_EmptySource(t *testing.T) {
	result := Parse("")
	if result.Root == nil {
		t.Fatal("expected non-nil Root for empty source")
	}
	if result.Root.Kind != NodeProgram {
		t.Fatalf("expected NodeProgram root, got %s", result.Root.Kind)
	}
}

func TestParse_SingleAssignment(t *testing.T) {
	result := Parse("x : 42")
	if result.Root == nil || result.Root.Kind != NodeProgram {
		t.Fatalf("expected NodeProgram root, got %v", result.Root)
	}
	if len(result.Root.Children) == 0 {
		t.Fatal("expected at least one child in program for 'x : 42'")
	}
}

func TestParse_MultipleAssignments(t *testing.T) {
	result := Parse("x : 1\ny : 2\nz : 3")
	if result.Root == nil {
		t.Fatal("expected non-nil root")
	}
	if len(result.Root.Children) < 3 {
		t.Fatalf("expected 3 top-level statements, got %d", len(result.Root.Children))
	}
}

func TestParse_ScopeAssign(t *testing.T) {
	result := Parse("myScope : < x : 1 >\n")
	if result.Root == nil {
		t.Fatal("expected non-nil root")
	}
	if len(result.Root.Children) == 0 {
		t.Fatal("expected scope_assign child in program")
	}
	stmt := result.Root.Children[0]
	if stmt.Kind != NodeScopeAssign {
		t.Fatalf("expected NodeScopeAssign, got %s", stmt.Kind)
	}
}

func TestParse_ErrorRecovery(t *testing.T) {
	// A bare integer literal is valid lexically but cannot be a top-level
	// statement (needs an assignment LHS).  The parser should record an error
	// for that line and still parse the following valid assignment.
	result := Parse("42\nx : 42")
	if result.Root == nil {
		t.Fatal("expected non-nil root even on error input")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected errors for unrecognised top-level token")
	}
	// Should still have parsed the valid assignment after recovery.
	found := false
	for _, child := range result.Root.Children {
		if child.Kind == NodeAssignment {
			found = true
		}
	}
	if !found {
		t.Fatal("expected recovered NodeAssignment after unrecognised line")
	}
}

func TestParse_DirectiveErrorsIncluded(t *testing.T) {
	// assign_lhs with duplicate identifiers — UNIQUE directive should fire.
	result := Parse("x, x : 42")
	if result.Root == nil {
		t.Fatal("expected non-nil root")
	}
	found := false
	for _, e := range result.Errors {
		if contains(e.Msg, "UNIQUE") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected UNIQUE directive error from duplicate assign_lhs")
	}
}

func TestParse_ReturnsNodeProgram(t *testing.T) {
	result := Parse("a : 1\nb : 2")
	if result.Root.Kind != NodeProgram {
		t.Fatalf("expected NodeProgram, got %s", result.Root.Kind)
	}
}

// ---------------------------------------------------------------------------
// parseProgram (internal)
// ---------------------------------------------------------------------------

func TestParseProgram_BlankLines(t *testing.T) {
	p := parserV3("\n\n\nx : 1\n\n")
	n := p.parseProgram()
	if n == nil || n.Kind != NodeProgram {
		t.Fatalf("expected NodeProgram, got %v", n)
	}
	if len(n.Children) == 0 {
		t.Fatal("expected at least one child in program after skipping blanks")
	}
}

func TestParseProgram_Empty(t *testing.T) {
	p := parserV3("")
	n := p.parseProgram()
	if n == nil || n.Kind != NodeProgram {
		t.Fatalf("expected NodeProgram, got %v", n)
	}
}

func TestParseProgram_MultipleStatements(t *testing.T) {
	p := parserV3("x : 1\ny : 2")
	n := p.parseProgram()
	if n == nil {
		t.Fatal("expected non-nil node")
	}
	if len(n.Children) < 2 {
		t.Fatalf("expected 2 children, got %d", len(n.Children))
	}
}
