// squeeze_v13_template_test.go — Specification-driven tests for template syntax.
// Covers spec/14_templates.sqg: modes 1-3, scope checker, call-site validator,
// nesting patterns and error cases.
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseTmplDeferred(src string) (*parser.V13TmplDeferredNode, error) {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return nil, err
	}
	p := parser.NewV13Parser(toks)
	return p.ParseTmplDeferred()
}

func parseTmplAssignRHS(src string) (*parser.V13AssignRHSNode, error) {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return nil, err
	}
	p := parser.NewV13Parser(toks)
	return p.ParseAssignRHS()
}

// ---------------------------------------------------------------------------
// Section 14.1 / 14.3 — lexer: § inside template string
// ---------------------------------------------------------------------------

func TestV13_Template_LexSectionParen(t *testing.T) {
	// § followed by ( inside a backtick string must not split into two tokens;
	// the whole backtick string is one V13_STRING token.
	src := "`Hello §(name)`"
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}
	// Expect: BOF  V13_STRING  EOF
	if len(toks) != 3 {
		t.Fatalf("expected 3 tokens (BOF, STRING, EOF), got %d: %v", len(toks), toks)
	}
	if toks[1].Type != parser.V13_STRING {
		t.Errorf("expected V13_STRING, got %s", toks[1].Type)
	}
	if toks[1].Value != "`Hello §(name)`" {
		t.Errorf("unexpected token value: %q", toks[1].Value)
	}
}

func TestV13_Template_LexEmpty(t *testing.T) {
	src := "``"
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}
	if toks[1].Type != parser.V13_EMPTY_STR_T {
		t.Errorf("expected V13_EMPTY_STR_T, got %s", toks[1].Type)
	}
}

// ---------------------------------------------------------------------------
// Section 14.3 — v13splitTemplateParts SlotIdx = -1 for non-deferred
// ---------------------------------------------------------------------------

func TestV13_Template_SplitPartsSlotIdxMinusOne(t *testing.T) {
	// ParseString on a plain template must leave SlotIdx == -1 on every part.
	src := "`Hello §(name), count §(n)`"
	lex := parser.NewV13Lexer(src)
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParseString()
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	for _, part := range node.Parts {
		if part.SlotIdx != -1 {
			t.Errorf("non-deferred part %q: expected SlotIdx -1, got %d", part.Text, part.SlotIdx)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 14.2 mode 1 — inline template via ParseString
// ---------------------------------------------------------------------------

func TestV13_Template_Mode1_ParseString(t *testing.T) {
	cases := []string{
		"`Hello §(name)`",
		"`§(a) + §(b) = §(c)`",
		"`plain text no expression`",
		"`§(user.first_name)`",
	}
	for _, src := range cases {
		lex := parser.NewV13Lexer(src)
		toks, err := lex.V13Tokenize()
		if err != nil {
			t.Errorf("lex %q: %v", src, err)
			continue
		}
		p := parser.NewV13Parser(toks)
		if _, err := p.ParseString(); err != nil {
			t.Errorf("ParseString(%q): unexpected error: %v", src, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 14.2 mode 2 — assigned template via ParseAssignRHS
// ---------------------------------------------------------------------------

func TestV13_Template_Mode2_AssignRHS_String(t *testing.T) {
	// A backtick string as the RHS of an assignment must parse as V13StringNode.
	cases := []string{
		"`Hello §(name)`",
		"`no interpolation`",
		"`§(first) §(last)`",
	}
	for _, src := range cases {
		if _, err := parseTmplAssignRHS(src); err != nil {
			t.Errorf("ParseAssignRHS(%q): unexpected error: %v", src, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 14.2 mode 3 — deferred template
// ---------------------------------------------------------------------------

func TestV13_Template_Mode3_ParseTmplDeferred_Basic(t *testing.T) {
	node, err := parseTmplDeferred("<- `Hello §(name)`")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node == nil {
		t.Fatal("got nil node")
	}
	if len(node.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(node.Params))
	}
	if node.Params[0].Name != "name" {
		t.Errorf("param 0 name: expected %q, got %q", "name", node.Params[0].Name)
	}
	if node.Params[0].SlotIdx != 0 {
		t.Errorf("param 0 slot: expected 0, got %d", node.Params[0].SlotIdx)
	}
}

func TestV13_Template_Mode3_MultipleParams(t *testing.T) {
	node, err := parseTmplDeferred("<- `Dear §(first_name), you have §(count) messages`")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(node.Params) != 2 {
		t.Fatalf("expected 2 params, got %d: %+v", len(node.Params), node.Params)
	}
	if node.Params[0].Name != "first_name" || node.Params[0].SlotIdx != 0 {
		t.Errorf("param 0: %+v", node.Params[0])
	}
	if node.Params[1].Name != "count" || node.Params[1].SlotIdx != 1 {
		t.Errorf("param 1: %+v", node.Params[1])
	}
}

func TestV13_Template_Mode3_SlotIdxAnnotated(t *testing.T) {
	// The Parts inside the embedded StringNode must have SlotIdx set.
	node, err := parseTmplDeferred("<- `§(a) middle §(b)`")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	exprParts := []parser.V13TmplPart{}
	for _, p := range node.Tmpl.Parts {
		if p.IsExpr {
			exprParts = append(exprParts, p)
		}
	}
	if len(exprParts) != 2 {
		t.Fatalf("expected 2 expr parts, got %d", len(exprParts))
	}
	if exprParts[0].SlotIdx != 0 {
		t.Errorf("first expr part SlotIdx: expected 0, got %d", exprParts[0].SlotIdx)
	}
	if exprParts[1].SlotIdx != 1 {
		t.Errorf("second expr part SlotIdx: expected 1, got %d", exprParts[1].SlotIdx)
	}
}

func TestV13_Template_Mode3_NoInterpolations(t *testing.T) {
	// A template with no §() slots parses as deferred with zero params.
	node, err := parseTmplDeferred("<- `static string`")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(node.Params) != 0 {
		t.Errorf("expected 0 params, got %d", len(node.Params))
	}
}

func TestV13_Template_Mode3_FuncCallChainSlot(t *testing.T) {
	// An expr slot may contain a func_call_chain text (parsed as raw text here).
	node, err := parseTmplDeferred("<- `§(records >>first() >>name)`")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(node.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(node.Params))
	}
	if node.Params[0].Name != "records >>first() >>name" {
		t.Errorf("param 0 name: %q", node.Params[0].Name)
	}
}

func TestV13_Template_Mode3_ViaAssignRHS(t *testing.T) {
	// <- tmpl_quoted must be parsed by ParseAssignRHS as V13TmplDeferredNode.
	rhs, err := parseTmplAssignRHS("<- `Hello §(name)`")
	if err != nil {
		t.Fatalf("ParseAssignRHS failed: %v", err)
	}
	if _, ok := rhs.Value.(*parser.V13TmplDeferredNode); !ok {
		t.Errorf("expected *V13TmplDeferredNode inside AssignRHS, got %T", rhs.Value)
	}
}

func TestV13_Template_Mode3_MustNotMatchFuncUnit(t *testing.T) {
	// "<-" followed by a backtick string must NOT parse as ReturnFuncUnit.
	rhs, err := parseTmplAssignRHS("<- `§(x)`")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := rhs.Value.(*parser.V13ReturnFuncUnitNode); ok {
		t.Error("parsed as ReturnFuncUnitNode — should have been TmplDeferredNode")
	}
}

// ---------------------------------------------------------------------------
// Section 14.6 — mode 3 with func_unit RHS still works (regression)
// ---------------------------------------------------------------------------

func TestV13_Template_ReturnFuncUnit_StillParses(t *testing.T) {
	// When "<-" is followed by "{" (not a backtick string), ParseAssignRHS must
	// route to ParseReturnFuncUnit, NOT ParseTmplDeferred.
	// A full func_unit with a push-recv body is the simplest valid form.
	src := "<- {\n~> item: @my_type\n<- item\n}"
	rhs, err := parseTmplAssignRHS(src)
	if err != nil {
		t.Fatalf("ReturnFuncUnit regression: %v", err)
	}
	if _, ok := rhs.Value.(*parser.V13TmplDeferredNode); ok {
		t.Error("regressed: parsed as TmplDeferredNode instead of ReturnFuncUnitNode")
	}
}

// ---------------------------------------------------------------------------
// Section 14.6 — scope checker (V13CheckTmplScope)
// ---------------------------------------------------------------------------

func TestV13_Template_ScopeChecker_AllInScope(t *testing.T) {
	src := "`Hello §(name), age §(age)`"
	lex := parser.NewV13Lexer(src)
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, _ := p.ParseString()
	scope := map[string]bool{"name": true, "age": true}
	if err := parser.V13CheckTmplScope(node, scope); err != nil {
		t.Errorf("unexpected scope error: %v", err)
	}
}

func TestV13_Template_ScopeChecker_Missing(t *testing.T) {
	src := "`Hello §(name), §(missing_var)`"
	lex := parser.NewV13Lexer(src)
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, _ := p.ParseString()
	scope := map[string]bool{"name": true}
	err := parser.V13CheckTmplScope(node, scope)
	if err == nil {
		t.Fatal("expected scope error, got nil")
	}
	se, ok := err.(*parser.V13TmplScopeError)
	if !ok {
		t.Fatalf("expected *V13TmplScopeError, got %T", err)
	}
	if len(se.MissingNames) != 1 || se.MissingNames[0] != "missing_var" {
		t.Errorf("unexpected missing names: %v", se.MissingNames)
	}
}

func TestV13_Template_ScopeChecker_DotPath(t *testing.T) {
	// "user.first_name" — only "user" needs to be in scope.
	src := "`§(user.first_name)`"
	lex := parser.NewV13Lexer(src)
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, _ := p.ParseString()
	scope := map[string]bool{"user": true}
	if err := parser.V13CheckTmplScope(node, scope); err != nil {
		t.Errorf("unexpected scope error: %v", err)
	}
}

func TestV13_Template_ScopeChecker_NoInterpolations(t *testing.T) {
	src := "`plain text`"
	lex := parser.NewV13Lexer(src)
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, _ := p.ParseString()
	if err := parser.V13CheckTmplScope(node, map[string]bool{}); err != nil {
		t.Errorf("unexpected error for plain text template: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Section 14.7 — call-site validator (V13ValidateTmplCall)
// ---------------------------------------------------------------------------

func mustDeferredNode(t *testing.T, src string) *parser.V13TmplDeferredNode {
	t.Helper()
	node, err := parseTmplDeferred(src)
	if err != nil {
		t.Fatalf("parseTmplDeferred(%q): %v", src, err)
	}
	return node
}

func TestV13_Template_CallValidator_OK(t *testing.T) {
	node := mustDeferredNode(t, "<- `§(a) §(b)`")
	if err := parser.V13ValidateTmplCall(node, []string{"string", "string"}, []string{"string", "string"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestV13_Template_CallValidator_TooFewArgs(t *testing.T) {
	node := mustDeferredNode(t, "<- `§(a) §(b) §(c)`")
	if err := parser.V13ValidateTmplCall(node, []string{"string"}, nil); err == nil {
		t.Error("expected error for too few args, got nil")
	}
}

func TestV13_Template_CallValidator_TooManyArgs(t *testing.T) {
	node := mustDeferredNode(t, "<- `§(a)`")
	if err := parser.V13ValidateTmplCall(node, []string{"string", "string"}, nil); err == nil {
		t.Error("expected error for too many args, got nil")
	}
}

func TestV13_Template_CallValidator_TypeMismatch(t *testing.T) {
	// paramType is "int" but caller passes "array" — not coercible, must error.
	node := mustDeferredNode(t, "<- `§(a)`")
	err := parser.V13ValidateTmplCall(node,
		[]string{"array"}, // argType supplied by caller
		[]string{"int"})   // paramType expected by the slot
	if err == nil {
		t.Error("expected type mismatch error, got nil")
	}
}

func TestV13_Template_CallValidator_NumericCoercibleToString(t *testing.T) {
	// int, float, decimal, bool are coercible to string for template output.
	node := mustDeferredNode(t, "<- `§(count)`")
	for _, typ := range []string{"int", "integer", "float", "decimal", "bool", "boolean"} {
		if err := parser.V13ValidateTmplCall(node, []string{typ}, []string{"string"}); err != nil {
			t.Errorf("type %q should be coercible to string but got error: %v", typ, err)
		}
	}
}

func TestV13_Template_CallValidator_EmptyParamType_Unconstrained(t *testing.T) {
	// Empty paramType means unconstrained — any argType is accepted.
	node := mustDeferredNode(t, "<- `§(x)`")
	if err := parser.V13ValidateTmplCall(node, []string{"anything"}, []string{""}); err != nil {
		t.Errorf("unexpected error for unconstrained slot: %v", err)
	}
}
