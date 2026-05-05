// Package squeeze_v1_test — functional tests for spec/06_functions.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: into_arrow, return_arrow, deps_oper, type_ref
//	Level 1: inspect_type, args_single_decl, func_return_stmt
//	Level 2: func_args, func_deps, func_args_decl,
//	          func_unit, return_func_unit, func_call_chain
//	Level 3+: func_body_stmt, func_scope_assign, func_inject
//	(EXTEND) statement += func_call_final, return_func_unit
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// into_arrow / return_arrow / deps_oper  (spec/06_functions.sqg)
// ===========================================================================

// TestV17_IntoArrow tests "->".
func TestV17_IntoArrow(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"->", true},
		{"<-", false},
		{">>", false},
		{"-", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIntoArrow()
		if tc.valid && err != nil {
			t.Errorf("into_arrow %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("into_arrow %q: expected failure", tc.src)
		}
	}
}

// TestV17_ReturnArrow tests "<-".
func TestV17_ReturnArrow(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"<-", true},
		{"->", false},
		{">>", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseReturnArrow()
		if tc.valid && err != nil {
			t.Errorf("return_arrow %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("return_arrow %q: expected failure", tc.src)
		}
	}
}

// TestV17_DepsOper tests "=>".
func TestV17_DepsOper(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"=>", true},
		{"->", false},
		{"==", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDepsOper()
		if tc.valid && err != nil {
			t.Errorf("deps_oper %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("deps_oper %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// type_ref / inspect_type  (spec/06_functions.sqg)
// ===========================================================================

// TestV17_TypeRef tests type_ref = ( ident_ref !WS! ".@type" ) | "@?".
func TestV17_TypeRef(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"MyType.@type", true},
		{"@?", true},
		{"my_type.sub.@type", true},
		{"MyType. @type", false}, // space before '@' violates !WS!
		{"@", false},             // lone @ is not ident_ref.@type
		{"MyType", false},        // missing .@type
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseTypeRef()
		if tc.valid && err != nil {
			t.Errorf("type_ref %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("type_ref %q: expected failure", tc.src)
		}
	}
}

// TestV17_InspectType tests inspect_type = type_ref.
func TestV17_InspectType(t *testing.T) {
	p := newV17(t, "integer.@type")
	if _, err := p.ParseInspectType(); err != nil {
		t.Errorf("inspect_type: unexpected error: %v", err)
	}
	p2 := newV17(t, "@?")
	if _, err := p2.ParseInspectType(); err != nil {
		t.Errorf("inspect_type @?: unexpected error: %v", err)
	}
}

// ===========================================================================
// args_single_decl / func_args / func_deps  (spec/06_functions.sqg)
// ===========================================================================

// TestV17_ArgsSingleDecl tests args_single_decl = assign_lhs [ modifier ] decl_types.
func TestV17_ArgsSingleDecl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"value: integer.@type", true}, // immutable with type
		{"value integer.@type", true},  // no modifier, just type
		{"value:~ string.@type", true}, // read-only-ref with type
		{"value: @?", true},            // any_type
		{"value", false},               // missing type
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseArgsSingleDecl()
		if tc.valid && err != nil {
			t.Errorf("args_single_decl %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("args_single_decl %q: expected failure", tc.src)
		}
	}
}

// TestV17_FuncArgs tests func_args = into_arrow args_decl.
func TestV17_FuncArgs(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"-> value: integer.@type", true},
		{"-> x: string.@type, y: integer.@type", true},
		{"-> @integer", false},          // missing arg name
		{"value: integer.@type", false}, // missing arrow
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncArgs()
		if tc.valid && err != nil {
			t.Errorf("func_args %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_args %q: expected failure", tc.src)
		}
	}
}

// TestV17_FuncDeps tests func_deps = deps_oper UNIQUE< ident_ref { "," ident_ref } >.
func TestV17_FuncDeps(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"=> db", true},
		{"=> db, logger, cache", true},
		{"=> db.connection", true},
		{"=> ", false}, // missing ident after =>
		{"db", false},  // missing deps_oper
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncDeps()
		if tc.valid && err != nil {
			t.Errorf("func_deps %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_deps %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// func_args_decl  (spec/06_functions.sqg — includes MERGE)
// ===========================================================================

// TestV17_FuncArgsDecl tests func_args_decl (includes MERGE for iterator_recv_decl
// and push_recv_decl).
func TestV17_FuncArgsDecl(t *testing.T) {
	cases := []struct {
		src          string
		valid        bool
		wantIterRecv bool
		wantPushRecv bool
		wantFuncArgs bool
		wantFuncDeps bool
	}{
		// Role A: value form
		{"-> value: T.@type", true, false, false, true, false},
		// Role A with deps
		{"-> value: T.@type => db", true, false, false, true, true},
		// Role A': iterator form (MERGE)
		{">> iter: T.@type", true, true, false, false, false},
		// Role A: push stream form (MERGE)
		{"~> stream: T.@type", true, false, true, false, false},
		// Deps only
		{"=> db, cache", true, false, false, false, true},
		// Empty (both optional parts absent)
		{"", true, false, false, false, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		node, err := p.ParseFuncArgsDecl()
		if tc.valid && err != nil {
			t.Errorf("func_args_decl %q: unexpected error: %v", tc.src, err)
			continue
		}
		if !tc.valid && err == nil {
			t.Errorf("func_args_decl %q: expected failure", tc.src)
			continue
		}
		if tc.valid {
			if tc.wantIterRecv && node.IterRecvDecl == nil {
				t.Errorf("func_args_decl %q: expected IterRecvDecl to be set", tc.src)
			}
			if !tc.wantIterRecv && node.IterRecvDecl != nil {
				t.Errorf("func_args_decl %q: expected IterRecvDecl to be nil", tc.src)
			}
			if tc.wantPushRecv && node.PushRecvDecl == nil {
				t.Errorf("func_args_decl %q: expected PushRecvDecl to be set", tc.src)
			}
			if !tc.wantPushRecv && node.PushRecvDecl != nil {
				t.Errorf("func_args_decl %q: expected PushRecvDecl to be nil", tc.src)
			}
			if tc.wantFuncArgs && node.FuncArgs == nil {
				t.Errorf("func_args_decl %q: expected FuncArgs to be set", tc.src)
			}
			if tc.wantFuncDeps && node.FuncDeps == nil {
				t.Errorf("func_args_decl %q: expected FuncDeps to be set", tc.src)
			}
		}
	}
}

// ===========================================================================
// func_return_stmt / func_unit / return_func_unit  (spec/06_functions.sqg)
// ===========================================================================

// TestV17_FuncReturnStmt tests func_return_stmt = return_arrow statement.
func TestV17_FuncReturnStmt(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"<- 42", true},
		{"<- my_var", true},
		{"<- \"hello\"", true},
		{"-> 42", false},
		{"42", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncReturnStmt()
		if tc.valid && err != nil {
			t.Errorf("func_return_stmt %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_return_stmt %q: expected failure", tc.src)
		}
	}
}

// TestV17_FuncUnit tests func_unit = "(" func_args_decl func_body_stmt { ... } ")".
func TestV17_FuncUnit(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		// Simple func_unit with return
		{"(<- 42)", true},
		// With value arg
		{"(-> x: integer.@type <- x)", true},
		// With multiple statements
		{"(-> x: integer.@type\n  y = x\n  <- y)", true},
		// With iterator recv decl (Role A')
		{"(>> iter: integer.@type\n  <- 1)", true},
		// Empty body should fail (no func_body_stmt)
		{"()", false},
		// Missing close paren
		{"(<- 42", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncUnit()
		if tc.valid && err != nil {
			t.Errorf("func_unit %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_unit %q: expected failure", tc.src)
		}
	}
}

// TestV17_ReturnFuncUnit tests return_func_unit = return_arrow func_unit.
func TestV17_ReturnFuncUnit(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"<- (<- 42)", true},
		{"<- (-> x: integer.@type <- x)", true},
		{"<- ()", false},   // empty func_unit body
		{"(<- 42)", false}, // missing return_arrow
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseReturnFuncUnit()
		if tc.valid && err != nil {
			t.Errorf("return_func_unit %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("return_func_unit %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// func_call_chain / func_body_stmt (iterator)  (spec/06_functions.sqg)
// ===========================================================================

// TestV17_FuncCallChain tests func_call_chain = func_call { arrow func_ref }.
func TestV17_FuncCallChain(t *testing.T) {
	cases := []struct {
		src      string
		valid    bool
		segments int // expected number of chain segments (0 = just func_call)
	}{
		{"my_func", true, 0},
		{"my_func -> next_func", true, 1},
		{"a -> b -> c", true, 2},
		{"a >> b", true, 1},
		{"a ~> b", true, 1},
		{"a -> b >> c ~> d", true, 3},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		node, err := p.ParseFuncCallChain()
		if tc.valid && err != nil {
			t.Errorf("func_call_chain %q: unexpected error: %v", tc.src, err)
			continue
		}
		if !tc.valid && err == nil {
			t.Errorf("func_call_chain %q: expected failure", tc.src)
			continue
		}
		if tc.valid && len(node.Segments) != tc.segments {
			t.Errorf("func_call_chain %q: expected %d segments, got %d", tc.src, tc.segments, len(node.Segments))
		}
	}
}

// TestV17_FuncBodyStmt_Iterator tests iterator_yield_stmt and iterator_loop inside func_body_stmt.
func TestV17_FuncBodyStmt_Iterator(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		// func_return_stmt
		{"<- 42", true},
		// func_store_stmt
		{"=> my_obj", true},
		// assignment
		{"x = 5", true},
		// plain statement
		{"my_var", true},
		// iterator_loop
		{"my_list >> my_handler", true},
		// iterator_yield_stmt (Role B)
		{"my_value >>", true},
		// push_loop
		{"my_list ~> my_handler", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncBodyStmt()
		if tc.valid && err != nil {
			t.Errorf("func_body_stmt %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_body_stmt %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// statement EXTEND: func_call_final, return_func_unit  (spec/06_functions.sqg)
// ===========================================================================

// TestV17_Statement_FuncCallFinal tests that func_call_final chains
// are reachable via ParseStatement EXTEND.
func TestV17_Statement_FuncCallFinal(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"a -> b", true},
		{"a -> b -> c", true},
		{"a >> b", true},
		{"a ~> b", true},
		// Bare ident (no chain, no args) is handled by other alternatives — should still parse OK
		{"my_var", true},
		// Numeric expression — should NOT be stolen by func_call_final
		{"1 + 2", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseStatement()
		if tc.valid && err != nil {
			t.Errorf("statement[func_call_final] %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("statement[func_call_final] %q: expected failure", tc.src)
		}
	}
}

// TestV17_Statement_ReturnFuncUnit tests that return_func_unit is
// reachable via ParseStatement EXTEND.
func TestV17_Statement_ReturnFuncUnit(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"<- (<- 42)", true},
		{"<- (-> x: integer.@type <- x)", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseStatement()
		if tc.valid && err != nil {
			t.Errorf("statement[return_func_unit] %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("statement[return_func_unit] %q: expected failure", tc.src)
		}
	}
}
