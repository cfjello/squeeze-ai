// Package squeeze_v1_test — functional tests for spec/07_types_scope.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: assign_oper, extend_scope_oper, scope_assign_inline
//	Level 1: scope_inject, import_assign, other_inline_assign
//	Level 2: scope_body_item, private_block, scope_body
//	Level 3: scope_assign, scope_merge_tail
//	Level 4: scope_final, parser_root
//	(EXTEND) func_args_decl += func_inject
//	(EXTEND) func_scope_assign
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// assign_oper / extend_scope_oper / scope_assign_inline  (spec/07_types_scope.sqg)
// ===========================================================================

// TestV17_AssignOper tests assign_oper = ":" | ":~" | "=".
func TestV17_AssignOper(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{":", true},
		{":~", true},
		{"=", true},
		{"+", false},
		{"==", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseAssignOper()
		if tc.valid && err != nil {
			t.Errorf("assign_oper %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assign_oper %q: expected failure", tc.src)
		}
	}
}

// TestV17_ExtendScopeOper tests extend_scope_oper = "+=".
func TestV17_ExtendScopeOper(t *testing.T) {
	p := newV17(t, "+=")
	if _, err := p.ParseExtendScopeOper(); err != nil {
		t.Errorf("extend_scope_oper '+=': unexpected error: %v", err)
	}
	p2 := newV17(t, "=")
	if _, err := p2.ParseExtendScopeOper(); err == nil {
		t.Error("extend_scope_oper '=': expected failure")
	}
}

// TestV17_ScopeAssignInline tests scope_assign_inline = "_".
func TestV17_ScopeAssignInline(t *testing.T) {
	p := newV17(t, "_")
	if _, err := p.ParseScopeAssignInline(); err != nil {
		t.Errorf("scope_assign_inline '_': unexpected error: %v", err)
	}
	p2 := newV17(t, "x")
	if _, err := p2.ParseScopeAssignInline(); err == nil {
		t.Error("scope_assign_inline 'x': expected failure")
	}
}

// ===========================================================================
// scope_inject / import_assign / other_inline_assign  (spec/07_types_scope.sqg)
// ===========================================================================

// TestV17_ScopeInject tests scope_inject = "(" UNIQUE< ident_name assign_oper type_ref
// { "," ident_name assign_oper type_ref } > ")".
func TestV17_ScopeInject(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"(x: myType.@type)", true},
		{"(x: pkg.myType.@type)", true},
		{"(x: @?)", true},
		{"(x: myType.@type, y: other.@type)", true},
		{"(x myType)", false}, // missing assign_oper
		{"()", false},         // empty — must have at least one
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseScopeInject()
		if tc.valid && err != nil {
			t.Errorf("scope_inject %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("scope_inject %q: expected failure", tc.src)
		}
	}
}

// TestV17_ImportAssign tests import_assign (file / http import).
func TestV17_ImportAssign(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"\ndata: \"file://./data.sqz\"", true},
		{"\nhttp_data: \"http://example.com/api\"", true},
		{"\nhttp_data: \"https://example.com/api\"", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseImportAssign()
		if tc.valid && err != nil {
			t.Errorf("import_assign %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("import_assign %q: expected failure", tc.src)
		}
	}
}

// TestV17_OtherInlineAssign tests other_inline_assign = scope_assign_inline assign_oper statement.
func TestV17_OtherInlineAssign(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"\n_ : 42", true},
		{"\n_ = 42", true},
		{"\n-_ : 42", true},
		{"\nx : 42", false}, // 'x' is not scope_assign_inline
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseOtherInlineAssign()
		if tc.valid && err != nil {
			t.Errorf("other_inline_assign %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("other_inline_assign %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// scope_body_item / private_block / scope_body  (spec/07_types_scope.sqg)
// ===========================================================================

// TestV17_ScopeBodyItem tests scope_body_item = various assignment / func forms.
func TestV17_ScopeBodyItem(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"x: 42", true},
		{"s = {}", true},
		{"f: (-> x: string.@type <- x)", true},
		{"data: \"file://./data.sqz\"", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseScopeBodyItem()
		if tc.valid && err != nil {
			t.Errorf("scope_body_item %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("scope_body_item %q: expected failure", tc.src)
		}
	}
}

// TestV17_PrivateBlock tests private_block = ( "-(" | "-{" ) scope_body_item { "," scope_body_item } ( ")" | "}" ).
func TestV17_PrivateBlock(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"-(x: 42)", true},
		{"-{x: 42}", true},
		{"-(x: 1, y: 2)", true},
		{"(x: 42)", false},  // missing '-' prefix
		{"-[x: 42]", false}, // wrong bracket type
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParsePrivateBlock()
		if tc.valid && err != nil {
			t.Errorf("private_block %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("private_block %q: expected failure", tc.src)
		}
	}
}

// TestV17_ScopeBody tests scope_body = "{" [ scope_body_item { "," scope_body_item } ] "}".
func TestV17_ScopeBody(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"{}", true},
		{"{x: 42}", true},
		{"{x: 1, y: 2}", true},
		{"{inner = {}}", true},
		{"{-(x: 1)}", true},
		{"x: 42", false}, // not wrapped in {}
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseScopeBody()
		if tc.valid && err != nil {
			t.Errorf("scope_body %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("scope_body %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// scope_assign / scope_merge_tail / scope_final / parser_root
// (spec/07_types_scope.sqg)
// ===========================================================================

// TestV17_ScopeAssign tests scope_assign = [ scope_inject ] ident_name ( assign_oper | extend_scope_oper ) scope_body.
func TestV17_ScopeAssign(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"s = {}", true},
		{"s = {x: 42}", true},
		{"s += {x: 42}", true},
		{"(db: dbType.@type) s = {}", true},
		{"s: {}", true},
		{"s:~ {}", true},
		{"s = ", false}, // missing scope_body
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseScopeAssign()
		if tc.valid && err != nil {
			t.Errorf("scope_assign %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("scope_assign %q: expected failure", tc.src)
		}
	}
}

// TestV17_ScopeMergeTail tests scope_merge_tail = ident_ref "+" scope_body { "+" scope_body }.
func TestV17_ScopeMergeTail(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"a = {} + {}", true},
		{"myRef + {}", true},
		{"a = {} + {} + {}", true},
		{"a = {}", false},   // no merge tail
		{"a = {} +", false}, // trailing +
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseScopeMergeTail()
		if tc.valid && err != nil {
			t.Errorf("scope_merge_tail %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("scope_merge_tail %q: expected failure", tc.src)
		}
	}
}

// TestV17_ScopeFinal tests scope_final = scope_assign | scope_merge_tail.
func TestV17_ScopeFinal(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"s = {x: 1}", true},
		{"a = {} + {}", true},
		{"s = {}", true},
		{"s {}", false}, // missing assign_oper
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseScopeFinal()
		if tc.valid && err != nil {
			t.Errorf("scope_final %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("scope_final %q: expected failure", tc.src)
		}
	}
}

// TestV17_ParserRoot tests parser_root = scope_final { scope_final }.
func TestV17_ParserRoot(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"myScope = {x: 1}", true},
		{"a = {} + {}", true},
		{"s = {}", true},
		{"42", false},
		{"foo", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseParserRoot()
		if tc.valid && err != nil {
			t.Errorf("parser_root %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("parser_root %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// func_inject / func_scope_assign  (spec/07_types_scope.sqg — EXTEND)
// ===========================================================================

// TestV17_FuncInject tests the EXTEND<func_args_decl> func_inject form:
// "(" ident_name assign_oper ( type_ref | ident_ref ) { "," … } ")".
func TestV17_FuncInject(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"(x: string.@type)", true},
		{"(x: myModule.Type)", true},
		{"(x: string.@type, y: int32.@type)", true},
		{"(x: string.@type[])", true},
		{"(x string)", false}, // missing assign_oper
		{"()", false},         // empty — must have at least one
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncInject()
		if tc.valid && err != nil {
			t.Errorf("func_inject %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_inject %q: expected failure", tc.src)
		}
	}
}

// TestV17_FuncScopeAssign tests func_scope_assign = [ func_inject ] ident_name assign_oper func_unit.
func TestV17_FuncScopeAssign(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"myFunc: (-> x: string.@type <- x)", true},
		{"(ctx: myCtx.@type) myFunc: (-> x: string.@type <- x)", true},
		{"myFunc (-> x: string.@type <- x)", false}, // missing assign_oper
		{"myFunc: 42", false},                       // rhs is not func_unit
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncScopeAssign()
		if tc.valid && err != nil {
			t.Errorf("func_scope_assign %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_scope_assign %q: expected failure", tc.src)
		}
	}
}
