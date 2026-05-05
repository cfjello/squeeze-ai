// Package squeeze_v1_test — functional tests for spec/12_iterators.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: iterator_oper
//	Level 1: iterator_recv_decl, iterator_yield_stmt
//	Level 2: assign_iterator
//	Level 3: iterator_loop
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// iterator_oper  (spec/12_iterators.sqg)
// ===========================================================================

// TestV17_IteratorOper tests iterator_oper = ">>".
func TestV17_IteratorOper(t *testing.T) {
	p := newV17(t, ">>")
	if _, err := p.ParseIteratorOper(); err != nil {
		t.Errorf("iterator_oper '>>': unexpected error: %v", err)
	}
	p2 := newV17(t, ">")
	if _, err := p2.ParseIteratorOper(); err == nil {
		t.Error("iterator_oper '>': expected failure for single '>'")
	}
}

// ===========================================================================
// iterator_recv_decl  (spec/12_iterators.sqg)
// ===========================================================================

// TestV17_IteratorRecvDecl tests iterator_recv_decl = iterator_oper ident_name assign_oper type_ref.
func TestV17_IteratorRecvDecl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{">> iter: my_type.@type", true},
		{">> the_iter: @?", true},
		{">> iter my_type.@type", false}, // missing assign_oper
		{"> iter: my_type.@type", false}, // single '>' is not iterator_oper
		{"iter: my_type.@type", false},   // missing iterator_oper
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIteratorRecvDecl()
		if tc.valid && err != nil {
			t.Errorf("iterator_recv_decl %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("iterator_recv_decl %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// iterator_yield_stmt  (spec/12_iterators.sqg)
// ===========================================================================

// TestV17_IteratorYieldStmt tests iterator_yield_stmt = statement iterator_oper.
func TestV17_IteratorYieldStmt(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"my_value >>", true},
		{"42 >>", true},
		{"my_value >> next_func", false}, // ">>" must be terminal (no handler here)
		{"my_value", false},              // missing iterator_oper
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIteratorYieldStmt()
		if tc.valid && err != nil {
			t.Errorf("iterator_yield_stmt %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("iterator_yield_stmt %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// assign_iterator  (spec/12_iterators.sqg)
// ===========================================================================

// TestV17_AssignIterator tests assign_iterator = ( collection | range ) iterator_oper.
func TestV17_AssignIterator(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"my_collection >>", true},
		{"[1, 2, 3] >>", true},
		{"1..10 >>", true},
		{"my_collection >> handler", false}, // ">>" must be terminal
		{"my_collection", false},            // missing iterator_oper
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseAssignIterator()
		if tc.valid && err != nil {
			t.Errorf("assign_iterator %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assign_iterator %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// iterator_loop  (spec/12_iterators.sqg)
// ===========================================================================

// TestV17_IteratorLoop tests iterator_loop = ( collection | range ) ">>" ( func_unit | ident_ref ).
func TestV17_IteratorLoop(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"my_list >> (<- 42)", true},
		{"my_list >> my_handler", true},
		{"1..10 >> (<- 1)", true},
		{"[1, 2, 3] >> process_item", true},
		{"my_list >>", false},        // missing handler
		{"my_list ~> (<- 1)", false}, // wrong operator (~> is push, not iterator)
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIteratorLoop()
		if tc.valid && err != nil {
			t.Errorf("iterator_loop %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("iterator_loop %q: expected failure", tc.src)
		}
	}
}
