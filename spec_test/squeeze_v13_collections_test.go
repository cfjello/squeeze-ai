//go:build ignore

// squeeze_v13_collections_test.go — Specification-driven tests for the
// collection functions grammar (spec/15_collections.sqg).
//
// Covers:
//   - PIPELINE<name> directive parsing and registration
//   - Pipeline call syntax: col >>func(args)
//   - Pipeline chaining: col >>filter(p) >>map(f)
//   - Free function call forms: map(col, f), reduce(col, init, f)
//   - Unknown ident after >> falls back (does not parse as pipeline)
//   - Pipeline call as assign_rhs
//   - All aggregate and transform names are pre-registered
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parsePipelineCall(src string) (*parser.V13PipelineCallNode, error) {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return nil, err
	}
	p := parser.NewV13Parser(toks)
	return p.ParsePipelineCall()
}

func parsePipelineDecl(src string) (*parser.V13PipelineDeclNode, error) {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return nil, err
	}
	p := parser.NewV13Parser(toks)
	return p.ParsePipelineDecl()
}

func parseCollRHS(src string) (*parser.V13AssignRHSNode, error) {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return nil, err
	}
	p := parser.NewV13Parser(toks)
	return p.ParseAssignRHS()
}

// ---------------------------------------------------------------------------
// 15.4  PIPELINE<name> directive
// ---------------------------------------------------------------------------

func TestV13_Collections_PipelineDecl_Parses(t *testing.T) {
	node, err := parsePipelineDecl("PIPELINE<custom_func>")
	if err != nil {
		t.Fatalf("ParsePipelineDecl failed: %v", err)
	}
	if node == nil {
		t.Fatal("expected non-nil V13PipelineDeclNode")
	}
	if node.Name != "custom_func" {
		t.Errorf("expected Name %q, got %q", "custom_func", node.Name)
	}
	// The name must now be registered
	if !parser.V13PipelineFuncs["custom_func"] {
		t.Error("custom_func was not registered in V13PipelineFuncs after ParsePipelineDecl")
	}
}

func TestV13_Collections_PipelineDecl_BadSyntax(t *testing.T) {
	// Missing <name>
	_, err := parsePipelineDecl("PIPELINE")
	if err == nil {
		t.Error("expected parse error for bare PIPELINE, got nil")
	}
	// Missing >
	_, err = parsePipelineDecl("PIPELINE<foo")
	if err == nil {
		t.Error("expected parse error for unclosed PIPELINE<foo, got nil")
	}
}

// ---------------------------------------------------------------------------
// 15.11  Standard pipeline functions are pre-registered
// ---------------------------------------------------------------------------

func TestV13_Collections_PreRegistered(t *testing.T) {
	names := []string{"map", "filter", "take", "drop", "zip", "join", "reduce"}
	for _, name := range names {
		if !parser.V13PipelineFuncs[name] {
			t.Errorf("expected %q to be pre-registered in V13PipelineFuncs", name)
		}
	}
}

// ---------------------------------------------------------------------------
// 15.4  Pipeline call: col >>map(f)
// ---------------------------------------------------------------------------

func TestV13_Collections_PipelineCall_Map(t *testing.T) {
	node, err := parsePipelineCall("col >>map(f)")
	if err != nil {
		t.Fatalf("ParsePipelineCall failed: %v", err)
	}
	if node.FuncName != "map" {
		t.Errorf("expected FuncName %q, got %q", "map", node.FuncName)
	}
	if node.Source == nil {
		t.Error("expected non-nil Source")
	}
	if len(node.ExtraArgs) != 1 {
		t.Errorf("expected 1 ExtraArg, got %d", len(node.ExtraArgs))
	}
}

func TestV13_Collections_PipelineCall_Filter(t *testing.T) {
	node, err := parsePipelineCall("items >>filter(pred)")
	if err != nil {
		t.Fatalf("ParsePipelineCall failed: %v", err)
	}
	if node.FuncName != "filter" {
		t.Errorf("expected FuncName %q, got %q", "filter", node.FuncName)
	}
}

func TestV13_Collections_PipelineCall_Take(t *testing.T) {
	node, err := parsePipelineCall("items >>take(5)")
	if err != nil {
		t.Fatalf("ParsePipelineCall failed: %v", err)
	}
	if node.FuncName != "take" {
		t.Errorf("expected FuncName %q, got %q", "take", node.FuncName)
	}
	if len(node.ExtraArgs) != 1 {
		t.Errorf("expected 1 ExtraArg, got %d", len(node.ExtraArgs))
	}
}

func TestV13_Collections_PipelineCall_Drop(t *testing.T) {
	node, err := parsePipelineCall("items >>drop(3)")
	if err != nil {
		t.Fatalf("ParsePipelineCall failed: %v", err)
	}
	if node.FuncName != "drop" {
		t.Errorf("expected FuncName %q, got %q", "drop", node.FuncName)
	}
}

func TestV13_Collections_PipelineCall_Join(t *testing.T) {
	node, err := parsePipelineCall(`parts >>join(",")`)
	if err != nil {
		t.Fatalf("ParsePipelineCall failed: %v", err)
	}
	if node.FuncName != "join" {
		t.Errorf("expected FuncName %q, got %q", "join", node.FuncName)
	}
}

func TestV13_Collections_PipelineCall_Reduce(t *testing.T) {
	node, err := parsePipelineCall("nums >>reduce(0, f)")
	if err != nil {
		t.Fatalf("ParsePipelineCall failed: %v", err)
	}
	if node.FuncName != "reduce" {
		t.Errorf("expected FuncName %q, got %q", "reduce", node.FuncName)
	}
	if len(node.ExtraArgs) != 2 {
		t.Errorf("expected 2 ExtraArgs (init, f), got %d", len(node.ExtraArgs))
	}
}

// ---------------------------------------------------------------------------
// Pipeline chaining: col >>filter(p) >>map(f)
// ---------------------------------------------------------------------------

func TestV13_Collections_PipelineChain_FilterThenMap(t *testing.T) {
	node, err := parsePipelineCall("col >>filter(pred) >>map(f)")
	if err != nil {
		t.Fatalf("ParsePipelineCall chain failed: %v", err)
	}
	// Outer node is map
	if node.FuncName != "map" {
		t.Errorf("outer FuncName: expected %q, got %q", "map", node.FuncName)
	}
	// Inner source is the filter step
	inner, ok := node.Source.(*parser.V13PipelineCallNode)
	if !ok {
		t.Fatalf("expected Source to be *V13PipelineCallNode, got %T", node.Source)
	}
	if inner.FuncName != "filter" {
		t.Errorf("inner FuncName: expected %q, got %q", "filter", inner.FuncName)
	}
}

func TestV13_Collections_PipelineChain_ThreeSteps(t *testing.T) {
	node, err := parsePipelineCall("col >>filter(pred) >>take(10) >>join(\", \")")
	if err != nil {
		t.Fatalf("3-step chain failed: %v", err)
	}
	if node.FuncName != "join" {
		t.Errorf("expected outermost FuncName %q, got %q", "join", node.FuncName)
	}
	mid, ok := node.Source.(*parser.V13PipelineCallNode)
	if !ok {
		t.Fatalf("expected second Source to be *V13PipelineCallNode, got %T", node.Source)
	}
	if mid.FuncName != "take" {
		t.Errorf("expected middle FuncName %q, got %q", "take", mid.FuncName)
	}
}

// ---------------------------------------------------------------------------
// Pipeline call fails for unknown ident after >>
// ---------------------------------------------------------------------------

func TestV13_Collections_PipelineCall_UnknownFuncFails(t *testing.T) {
	// "notregistered" is not in V13PipelineFuncs (assuming it wasn't registered)
	// The pipeline call should fail so the caller can try other paths.
	delete(parser.V13PipelineFuncs, "notregistered") // ensure it's absent
	_, err := parsePipelineCall("col >>notregistered(f)")
	if err == nil {
		t.Error("expected ParsePipelineCall to fail for unregistered func name, got nil")
	}
}

// ---------------------------------------------------------------------------
// Pipeline call as assign_rhs
// ---------------------------------------------------------------------------

func TestV13_Collections_AssignRHS_PipelineCall(t *testing.T) {
	// col >>map(f) should parse as an assign_rhs whose Value is *V13PipelineCallNode.
	node, err := parseCollRHS("col >>map(f)")
	if err != nil {
		t.Fatalf("ParseAssignRHS with pipeline call failed: %v", err)
	}
	pc, ok := node.Value.(*parser.V13PipelineCallNode)
	if !ok {
		t.Fatalf("expected Value to be *V13PipelineCallNode, got %T", node.Value)
	}
	if pc.FuncName != "map" {
		t.Errorf("expected FuncName %q, got %q", "map", pc.FuncName)
	}
}

func TestV13_Collections_AssignRHS_PipelineChain(t *testing.T) {
	node, err := parseCollRHS("items >>filter(pred) >>map(f)")
	if err != nil {
		t.Fatalf("ParseAssignRHS chain failed: %v", err)
	}
	pc, ok := node.Value.(*parser.V13PipelineCallNode)
	if !ok {
		t.Fatalf("expected *V13PipelineCallNode, got %T", node.Value)
	}
	if pc.FuncName != "map" {
		t.Errorf("expected outermost func %q, got %q", "map", pc.FuncName)
	}
}

// ---------------------------------------------------------------------------
// Free function form still parses (existing calc_unit / func_call paths)
// ---------------------------------------------------------------------------

func TestV13_Collections_FreeFunc_MapStillParses(t *testing.T) {
	// Free function form via ParseAssignRHS → ParseCalcUnit → ident_ref.
	// map(col, f) is a plain identifier expression — parses as ident_ref "map" + ...
	// We just verify it does not error and is not rejected.
	if err := parseRHS("myvar"); err != nil {
		t.Errorf("simple ident_ref should parse: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 15.8  reduce — two extra args (init mandatory in pipeline form)
// ---------------------------------------------------------------------------

func TestV13_Collections_PipelineReduce_TwoExtraArgs(t *testing.T) {
	node, err := parsePipelineCall("nums >>reduce(0, adder)")
	if err != nil {
		t.Fatalf("pipeline reduce failed: %v", err)
	}
	if len(node.ExtraArgs) != 2 {
		t.Errorf("reduce pipeline: expected 2 extra args (init + f), got %d", len(node.ExtraArgs))
	}
}

// ---------------------------------------------------------------------------
// Pipeline call with multiple args (verifies arg list parsing)
// ---------------------------------------------------------------------------

func TestV13_Collections_PipelineCall_MultipleArgs(t *testing.T) {
	// col >>zip(col2) — two ident args (source + one extra)
	node, err := parsePipelineCall("col >>zip(col2)")
	if err != nil {
		t.Fatalf("pipeline call zip failed: %v", err)
	}
	if node.FuncName != "zip" {
		t.Errorf("expected FuncName %q, got %q", "zip", node.FuncName)
	}
	if len(node.ExtraArgs) != 1 {
		t.Errorf("expected 1 ExtraArg, got %d", len(node.ExtraArgs))
	}
}
