//go:build ignore

// squeeze_v13_iterators_test.go — Specification-driven tests for iterator syntax.
// Covers spec/12_iterators.sqg: iterator_source, iterator_yield_stmt,
// assign_iterator, and func_stream_loop (updated to use iterator_source).
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseFuncStreamLoop(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParseFuncStreamLoop()
	return err
}

func parseIteratorSource(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParseIteratorSource()
	return err
}

func parseIteratorYieldStmt(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParseIteratorYieldStmt()
	return err
}

// ---------------------------------------------------------------------------
// Section 12.1 — ">>" token disambiguation
// ---------------------------------------------------------------------------

func TestV13_Iterators_LexStream(t *testing.T) {
	// ">>" lexes as V13_STREAM; "<-", "<=", "<" are unchanged
	cases := []struct {
		src      string
		wantType parser.V13TokenType
	}{
		{">>", parser.V13_STREAM},
		{"<-", parser.V13_RETURN_STMT},
		{"<=", parser.V13_LEQ},
		{"<", parser.V13_LT},
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, err := lex.V13Tokenize()
		if err != nil {
			t.Errorf("lex %q: unexpected error: %v", tc.src, err)
			continue
		}
		// toks[0] is always V13_BOF; the operator token is at index 1.
		if len(toks) < 2 {
			t.Errorf("lex %q: expected at least 2 tokens, got %d", tc.src, len(toks))
			continue
		}
		if toks[1].Type != tc.wantType {
			t.Errorf("lex %q: want token type %v, got %v", tc.src, tc.wantType, toks[1].Type)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 12.2 — iterator_source
// ---------------------------------------------------------------------------

func TestV13_Iterators_IteratorSource_BooleanTrue(t *testing.T) {
	// boolean_true as source — infinite loop provider
	if err := parseIteratorSource("true"); err != nil {
		t.Errorf("iterator_source(true): unexpected error: %v", err)
	}
}

func TestV13_Iterators_IteratorSource_IdentRef(t *testing.T) {
	cases := []string{
		"my_collection",
		"db_result",
		"rows",
	}
	for _, src := range cases {
		if err := parseIteratorSource(src); err != nil {
			t.Errorf("iterator_source(%q): unexpected error: %v", src, err)
		}
	}
}

func TestV13_Iterators_IteratorSource_FuncRangeArgs(t *testing.T) {
	cases := []string{
		"1..10",
		"0..99",
	}
	for _, src := range cases {
		if err := parseIteratorSource(src); err != nil {
			t.Errorf("iterator_source(%q): unexpected error: %v", src, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 12.5 — iterator_yield_stmt  ("result >>" postfix)
// ---------------------------------------------------------------------------

func TestV13_Iterators_YieldStmt_SimpleIdent(t *testing.T) {
	// mapped >> \n  — postfix yield
	if err := parseIteratorYieldStmt("mapped >>\n"); err != nil {
		t.Errorf("iterator_yield_stmt(mapped >>): unexpected error: %v", err)
	}
}

func TestV13_Iterators_YieldStmt_DottedRef(t *testing.T) {
	// rec.data >> \n
	if err := parseIteratorYieldStmt("rec.data >>\n"); err != nil {
		t.Errorf("iterator_yield_stmt(rec.data >>): unexpected error: %v", err)
	}
}

func TestV13_Iterators_YieldStmt_RejectsArrow(t *testing.T) {
	// "<-" is func_return_stmt, not iterator_yield_stmt
	if err := parseIteratorYieldStmt("<- value"); err == nil {
		t.Errorf("iterator_yield_stmt: expected parse failure for '<- value' but succeeded")
	}
}

func TestV13_Iterators_YieldStmt_RejectsMissingNL(t *testing.T) {
	// "result >> expr" — no NL after >> means this is a stream loop, not a yield
	if err := parseIteratorYieldStmt("result >> handler"); err == nil {
		t.Errorf("iterator_yield_stmt: expected parse failure when '>>' not followed by NL")
	}
}

// ---------------------------------------------------------------------------
// Section 12.7.1 — LOOP CONTEXT  (func_stream_loop with iterator_source)
// ---------------------------------------------------------------------------

func TestV13_Iterators_LoopContext_IdentRefSource(t *testing.T) {
	// any_collection >> ( >> rec: @my_type \n <- rec.data )
	src := "any_collection >> (\n>> rec: @my_type\n<- rec.data\n)"
	if err := parseFuncStreamLoop(src); err != nil {
		t.Errorf("loop/ident_ref: unexpected error: %v", err)
	}
}

func TestV13_Iterators_LoopContext_BooleanTrueSource(t *testing.T) {
	// infinite loop: true >> ( body )
	src := "true >> (\n<- done\n)"
	if err := parseFuncStreamLoop(src); err != nil {
		t.Errorf("loop/boolean_true: unexpected error: %v", err)
	}
}

func TestV13_Iterators_LoopContext_RangeSource(t *testing.T) {
	// range as source: 1..10 >> ( body )
	src := "1..10 >> (\n<- i\n)"
	if err := parseFuncStreamLoop(src); err != nil {
		t.Errorf("loop/range: unexpected error: %v", err)
	}
}

func TestV13_Iterators_LoopContext_YieldInsideBody(t *testing.T) {
	// Loop body uses postfix >> to yield downstream
	src := "rows >> (\n>> rec: @row_type\nrec.data >>\n)"
	if err := parseFuncStreamLoop(src); err != nil {
		t.Errorf("loop/yield: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Section 12.7.3 — ASSIGNMENT CONTEXT  (assign_iterator / lazy binding)
// ---------------------------------------------------------------------------

func TestV13_Iterators_AssignIterator_IdentRef(t *testing.T) {
	// my_iter = db_query >>
	// The full assignment goes through ParseAssignRHS; test the iterator
	// sub-production directly via ParseAssignIterator.
	cases := []string{
		"db_query >>\n",
		"rows >>\n",
	}
	for _, src := range cases {
		lex := parser.NewV13Lexer(src)
		toks, err := lex.V13Tokenize()
		if err != nil {
			t.Errorf("lex %q: %v", src, err)
			continue
		}
		p := parser.NewV13Parser(toks)
		_, err = p.ParseAssignIterator()
		if err != nil {
			t.Errorf("assign_iterator %q: unexpected error: %v", src, err)
		}
	}
}

func TestV13_Iterators_AssignIterator_RangeSource(t *testing.T) {
	src := "1..100 >>\n"
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		t.Errorf("lex: %v", err)
		return
	}
	p := parser.NewV13Parser(toks)
	if _, err := p.ParseAssignIterator(); err != nil {
		t.Errorf("assign_iterator range: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AST node type assertions
// ---------------------------------------------------------------------------

func TestV13_Iterators_NodeTypes_IteratorSource(t *testing.T) {
	// ParseIteratorSource must return a *V13IteratorSourceNode
	lex := parser.NewV13Lexer("my_col")
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParseIteratorSource()
	if err != nil {
		t.Fatalf("ParseIteratorSource: %v", err)
	}
	if node == nil {
		t.Fatal("ParseIteratorSource returned nil node")
	}
	if node.Value == nil {
		t.Fatal("V13IteratorSourceNode.Value is nil")
	}
}

func TestV13_Iterators_NodeTypes_IteratorYieldStmt(t *testing.T) {
	// ParseIteratorYieldStmt must return a *V13IteratorYieldStmtNode with non-nil Stmt
	lex := parser.NewV13Lexer("result >>\n")
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParseIteratorYieldStmt()
	if err != nil {
		t.Fatalf("ParseIteratorYieldStmt: %v", err)
	}
	if node == nil {
		t.Fatal("ParseIteratorYieldStmt returned nil node")
	}
	if node.Stmt == nil {
		t.Fatal("V13IteratorYieldStmtNode.Stmt is nil")
	}
}
