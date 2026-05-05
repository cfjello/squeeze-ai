// Package squeeze_v1_test â€” functional tests for spec/03_assignment.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: assign_mutable, assign_immutable, assign_read_only_ref,
//	          private_modifier, update_mutable_oper
//	Level 1: assign_version, assign_lhs, assign_single, assign_rhs,
//	          assign_new_var, update_mutable, self_ref
//	Level 2: assignment
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// update_mutable_oper / assign_operators  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_UpdateMutableOper(t *testing.T) {
	// Note: "/=" requires a preceding value-token for the lexer to emit V17_SLASH_EQ
	// (otherwise "/" is treated as a regexp delimiter). Test "/=" via ParseUpdateMutable instead.
	for _, op := range []string{"+=", "-=", "*="} {
		p := newV17(t, op)
		node, err := p.ParseUpdateMutableOper()
		if err != nil {
			t.Errorf("update_mutable_oper: unexpected error for %q: %v", op, err)
			continue
		}
		if node.Value != op {
			t.Errorf("update_mutable_oper: expected %q, got %q", op, node.Value)
		}
	}
	for _, bad := range []string{"=", ":", "+", "-"} {
		p := newV17(t, bad)
		if _, err := p.ParseUpdateMutableOper(); err == nil {
			t.Errorf("update_mutable_oper: expected failure for %q", bad)
		}
	}
	// 3.3 no-consume
	p := newV17(t, "=")
	if _, err := p.ParseUpdateMutableOper(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}

func TestV17_AssignOperators(t *testing.T) {
	// assign_mutable = "="
	p := newV17(t, "=")
	if _, err := p.ParseAssignMutable(); err != nil {
		t.Errorf("assign_mutable: unexpected error: %v", err)
	}
	p2 := newV17(t, ":")
	if _, err := p2.ParseAssignMutable(); err == nil {
		t.Error("assign_mutable: expected failure for ':'")
	}

	// assign_immutable = ":"
	p3 := newV17(t, ":")
	if _, err := p3.ParseAssignImmutable(); err != nil {
		t.Errorf("assign_immutable: unexpected error: %v", err)
	}
	p4 := newV17(t, ":~")
	if _, err := p4.ParseAssignImmutable(); err == nil {
		t.Error("assign_immutable: expected failure for ':~'")
	}

	// assign_read_only_ref = ":~"
	p5 := newV17(t, ":~")
	if _, err := p5.ParseAssignReadOnlyRef(); err != nil {
		t.Errorf("assign_read_only_ref: unexpected error: %v", err)
	}
	p6 := newV17(t, ":")
	if _, err := p6.ParseAssignReadOnlyRef(); err == nil {
		t.Error("assign_read_only_ref: expected failure for ':'")
	}

	// private_modifier = "-"
	p7 := newV17(t, "-")
	if _, err := p7.ParsePrivateModifier(); err != nil {
		t.Errorf("private_modifier: unexpected error: %v", err)
	}
}

// ===========================================================================
// assign_version  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_AssignVersion(t *testing.T) {
	validCases := []struct {
		src   string
		hasV  bool
		parts []string
	}{
		{"v1", true, []string{"1"}},
		{"v1.2", true, []string{"1", "2"}},
		{"v2.0.1", true, []string{"2", "0", "1"}},
	}
	for _, tc := range validCases {
		p := newV17(t, tc.src)
		node, err := p.ParseAssignVersion()
		if err != nil {
			t.Errorf("assign_version %q: unexpected error: %v", tc.src, err)
			continue
		}
		if node.HasV != tc.hasV {
			t.Errorf("assign_version %q: HasV = %v, want %v", tc.src, node.HasV, tc.hasV)
		}
		if len(node.Parts) != len(tc.parts) {
			t.Errorf("assign_version %q: len(Parts) = %d, want %d", tc.src, len(node.Parts), len(tc.parts))
			continue
		}
		for i, want := range tc.parts {
			if node.Parts[i] != want {
				t.Errorf("assign_version %q: Parts[%d] = %q, want %q", tc.src, i, node.Parts[i], want)
			}
		}
	}
	// bare digits without 'v' prefix are not valid assign_version (v prefix is required)
	for _, src := range []string{"1", "1.2", "1.2.3"} {
		p := newV17(t, src)
		_, err := p.ParseAssignVersion()
		if err == nil {
			t.Errorf("assign_version %q: expected error (no 'v' prefix), got nil", src)
		}
	}
}

// ===========================================================================
// assign_lhs  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_AssignLhs(t *testing.T) {
	cases := []struct {
		src         string
		name        string
		annotations int
	}{
		{"foo", "foo", 0},
		{"foo, bar", "foo", 1},
		{"foo, 1..2", "foo", 1}, // cardinality annotation
		{"foo, v1.2", "foo", 1}, // version annotation
		{"foo, bar, baz", "foo", 2},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		node, err := p.ParseAssignLhs()
		if err != nil {
			t.Errorf("assign_lhs %q: unexpected error: %v", tc.src, err)
			continue
		}
		if node.Name.Value != tc.name {
			t.Errorf("assign_lhs %q: Name = %q, want %q", tc.src, node.Name.Value, tc.name)
		}
		if len(node.Annotations) != tc.annotations {
			t.Errorf("assign_lhs %q: annotations = %d, want %d", tc.src, len(node.Annotations), tc.annotations)
		}
	}
}

// ===========================================================================
// assign_single  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_AssignSingle(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo = 42", true},      // assign_lhs assign_mutable statement (numeric)
		{`foo : "hello"`, true}, // assign_lhs assign_immutable statement (string)
		{"foo :~ true", true},   // assign_lhs assign_read_only_ref statement (bool constant)
		{"1 == 2 & 42", true},   // assign_cond_rhs
		{"foo 42", false},       // no operator â€” no longer valid
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseAssignSingle()
		if tc.valid && err != nil {
			t.Errorf("assign_single %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assign_single %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// assign_rhs  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_AssignRhs(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		// ( assign_cond_rhs | statement ) alternatives
		{"42", true},          // statement (numeric literal)
		{`"hi"`, true},        // statement (string literal)
		{"1 == 2 & 42", true}, // assign_cond_rhs
		// assign_private_single: private_modifier assign_single
		{"-foo = 42", true},
		// assign_private_grouping: private_modifier group_begin assign_single ... group_end
		{"-(foo = 42)", true},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseAssignRhs()
		if tc.valid && err != nil {
			t.Errorf("assign_rhs %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assign_rhs %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// assign_new_var  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_AssignNewVar(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"x : 42", true},          // assign_immutable; rhs = statement (numeric)
		{"x :~ true", true},       // assign_read_only_ref; rhs = statement (bool constant)
		{"x = 1 == 2 & 43", true}, // assign_mutable; rhs = assign_cond_rhs
		{"x : -foo = 42", true},   // assign_immutable; rhs = assign_private_single
		{"x : -(foo = 42)", true}, // assign_immutable; rhs = assign_private_grouping
		{"x += 42", false},        // wrong outer operator
		{"42 : 43", false},        // lhs must start with ident_name
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseAssignNewVar()
		if tc.valid && err != nil {
			t.Errorf("assign_new_var %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assign_new_var %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// update_mutable  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_UpdateMutable(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"x += 42", true},          // rhs = statement (numeric)
		{"x -= 1 == 2 & 43", true}, // rhs = assign_cond_rhs
		{"x *= -foo = 42", true},   // rhs = assign_private_single
		{"x : 42", false},          // wrong operator (assign_new_var territory)
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseUpdateMutable()
		if tc.valid && err != nil {
			t.Errorf("update_mutable %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("update_mutable %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// assignment  (spec/03_assignment.sqg â€” top-level rule)
// ===========================================================================

func TestV17_Assignment(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"x : 42", true},           // assign_new_var, immutable, numeric rhs
		{"x :~ true", true},        // assign_new_var, read-only-ref, bool rhs
		{"x = 1 == 2 & 43", true},  // assign_new_var, mutable, assign_cond_rhs
		{"x += 42", true},          // update_mutable, numeric rhs
		{"x -= 1 == 2 & 43", true}, // update_mutable, assign_cond_rhs
		{"!", false},
		{"42 : 43", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseAssignment()
		if tc.valid && err != nil {
			t.Errorf("assignment %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assignment %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// self_ref  (spec/03_assignment.sqg)
// ===========================================================================

func TestV17_SelfRef(t *testing.T) {
	p := newV17(t, "$")
	if _, err := p.ParseSelfRef(); err != nil {
		t.Errorf("self_ref: unexpected error: %v", err)
	}
	p2 := newV17(t, "foo")
	if _, err := p2.ParseSelfRef(); err == nil {
		t.Error("self_ref: expected failure for 'foo'")
	}
	// 3.3 no-consume
	p3 := newV17(t, "foo")
	if _, err := p3.ParseSelfRef(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}
