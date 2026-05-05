// Package squeeze_v1_test — functional tests for spec/13_push_pull.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: push_oper
//	Level 1: push_recv_decl, push_forward_stmt
//	Level 2: push_stream_bind, assign_push
//	Level 3: func_body_stmt EXTEND (push), assign_rhs EXTEND (push)
//	Level 4: func_unit EXTEND (push_stream)
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ===========================================================================
// push_oper  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_PushOper tests push_oper = "~>".
func TestV17_PushOper(t *testing.T) {
	p := newV17(t, "~>")
	if _, err := p.ParsePushOper(); err != nil {
		t.Errorf("push_oper '~>': unexpected error: %v", err)
	}
	p2 := newV17(t, ">>")
	if _, err := p2.ParsePushOper(); err == nil {
		t.Error("push_oper '>>': expected failure (that is iterator_oper)")
	}
}

// TestV17_PushOper_Token verifies that the lexer produces a "~>" token.
func TestV17_PushOper_Token(t *testing.T) {
	_, err := parser.NewV17ParserFromSource("~>")
	if err != nil {
		t.Fatalf("lex error for '~>': %v", err)
	}
}

// ===========================================================================
// push_recv_decl  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_PushRecvDecl tests push_recv_decl = push_oper ident_name assign_oper type_ref.
func TestV17_PushRecvDecl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"~> stream: my_type.@type", true},
		{"~> the_stream: @?", true},
		{"~> stream my_type.@type", false}, // missing assign_oper
		{"> stream: my_type.@type", false}, // '>' alone is not push_oper
		{"stream: my_type.@type", false},   // missing push_oper
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParsePushRecvDecl()
		if tc.valid && err != nil {
			t.Errorf("push_recv_decl %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("push_recv_decl %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// push_forward_stmt  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_PushForwardStmt tests push_forward_stmt = statement push_oper (terminal — no handler).
func TestV17_PushForwardStmt(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"my_value ~>", true},
		{"42 ~>", true},
		{"result ~>", true},
		{"my_value ~> handler", false}, // push_oper must be terminal here
		{"my_value ~> my_fn", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParsePushForwardStmt()
		if tc.valid && err != nil {
			t.Errorf("push_forward_stmt %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("push_forward_stmt %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// push_stream_bind  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_PushStreamBind tests push_stream_bind chain segment count.
func TestV17_PushStreamBind(t *testing.T) {
	cases := []struct {
		src      string
		valid    bool
		segments int // expected number of chain segments (handlers after "~>")
	}{
		{"my_list ~> my_handler", true, 1},
		{"my_list ~> (<- 42)", true, 1},
		{"my_list ~> my_handler ~> second_handler", true, 2},
		{"my_list ~> my_handler ~> (<- x) ~> third", true, 3},
		{"1..10 ~> my_handler", true, 1},
		// Invalid — no handler provided
		{"my_list ~>", false, 0},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		node, err := p.ParsePushStreamBind()
		if tc.valid && err != nil {
			t.Errorf("push_stream_bind %q: unexpected error: %v", tc.src, err)
			continue
		}
		if !tc.valid && err == nil {
			t.Errorf("push_stream_bind %q: expected failure", tc.src)
			continue
		}
		if tc.valid && len(node.Segments) != tc.segments {
			t.Errorf("push_stream_bind %q: expected %d segments, got %d", tc.src, tc.segments, len(node.Segments))
		}
	}
}

// ===========================================================================
// assign_push  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_AssignPush tests assign_push = ( collection | range ) push_oper (terminal).
func TestV17_AssignPush(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"my_feed ~>", true},
		{"1..100 ~>", true},
		{"my_feed ~> handler", false}, // push_oper must be terminal
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseAssignPush()
		if tc.valid && err != nil {
			t.Errorf("assign_push %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assign_push %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// func_body_stmt EXTEND: push variants  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_FuncBodyStmt_Push tests push-related alternatives in func_body_stmt.
func TestV17_FuncBodyStmt_Push(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"my_value ~>", true},
		{"my_list ~> my_handler", true},
		{"my_list ~> stage1 ~> stage2", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncBodyStmt()
		if tc.valid && err != nil {
			t.Errorf("func_body_stmt[push] %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_body_stmt[push] %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// assign_rhs EXTEND: push node type assertion  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_AssignRhs_Push tests that assign_rhs correctly produces a V17AssignPushNode
// when the rhs is assign_push and a plain node otherwise.
func TestV17_AssignRhs_Push(t *testing.T) {
	cases := []struct {
		src          string
		valid        bool
		wantPushNode bool
	}{
		{"my_feed ~>", true, true},
		{"1..50 ~>", true, true},
		{"my_feed ~> handler", true, false}, // push_stream_bind, not assign_push
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		node, err := p.ParseAssignRhs()
		if tc.valid && err != nil {
			t.Errorf("assign_rhs[push] %q: unexpected error: %v", tc.src, err)
			continue
		}
		if !tc.valid && err == nil {
			t.Errorf("assign_rhs[push] %q: expected failure", tc.src)
			continue
		}
		if tc.valid {
			_, isAssignPush := node.Value.(*parser.V17AssignPushNode)
			if tc.wantPushNode && !isAssignPush {
				t.Errorf("assign_rhs[push] %q: expected *V17AssignPushNode, got %T", tc.src, node.Value)
			}
			if !tc.wantPushNode && isAssignPush {
				t.Errorf("assign_rhs[push] %q: expected non-push node, got *V17AssignPushNode", tc.src)
			}
		}
	}
}

// ===========================================================================
// func_unit EXTEND: push_stream variant  (spec/13_push_pull.sqg)
// ===========================================================================

// TestV17_FuncUnit_PushStream tests func_unit bodies that involve push_recv_decl.
func TestV17_FuncUnit_PushStream(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		// push_recv_decl + push_stream_bind inside func_unit
		{"(~> stream: my_type.@type stream ~> my_sub_handler)", true},
		// Regular value func_unit still works
		{"(-> value: my_type.@type <- value)", true},
		// Iterator variant
		{"(>> iter: my_type.@type <- iter)", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncUnit()
		if tc.valid && err != nil {
			t.Errorf("func_unit[push_stream] %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_unit[push_stream] %q: expected failure", tc.src)
		}
	}
}
