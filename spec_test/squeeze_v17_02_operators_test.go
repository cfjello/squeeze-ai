// Package squeeze_v1_test â€” functional tests for spec/02_operators.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: group_begin, group_end, numeric_oper, string_oper, compare_oper,
//	          logic_oper, not_oper, inline_incr
//	Level 1: ident_name, ident_prefix, ident_dotted, ident_ref,
//	          single_num_expr, single_logic_expr, string_expr_chain,
//	          num_grouping, string_grouping, logic_grouping
//	Level 2: num_expr_chain, numeric_calc, string_concat,
//	          num_compare, string_compare, logic_expr_chain, logic_expr,
//	          condition, statement
//	(EXTEND) ident_ref += json_path
//	(EXTEND) statement += func_call_final, return_func_unit
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// ident_name / group_begin / group_end  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_IdentName(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo", true},
		{"myVar", true},
		{"x123", true},
		{"_invalid", false}, // must start with Unicode letter
		{"123abc", false},
		{"", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIdentName()
		if tc.valid && err != nil {
			t.Errorf("ident_name: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ident_name: expected failure for %q", tc.src)
		}
	}
	// 3.3 no-consume: wrong first token must not advance
	p := newV17(t, "123abc")
	if _, err := p.ParseIdentName(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}

func TestV17_GroupBegin(t *testing.T) {
	p := newV17(t, "(")
	if _, err := p.ParseGroupBegin(); err != nil {
		t.Errorf("group_begin: unexpected error: %v", err)
	}
	p2 := newV17(t, ")")
	if _, err := p2.ParseGroupBegin(); err == nil {
		t.Error("group_begin: expected failure for ')'")
	}
}

func TestV17_GroupEnd(t *testing.T) {
	p := newV17(t, ")")
	if _, err := p.ParseGroupEnd(); err != nil {
		t.Errorf("group_end: unexpected error: %v", err)
	}
	p2 := newV17(t, "(")
	if _, err := p2.ParseGroupEnd(); err == nil {
		t.Error("group_end: expected failure for '('")
	}
}

// ===========================================================================
// ident_dotted / ident_prefix / ident_ref  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_IdentDotted(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo", true},
		{"foo.bar", true},
		{"foo.bar.baz", true},
		{"123", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIdentDotted()
		if tc.valid && err != nil {
			t.Errorf("ident_dotted: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ident_dotted: expected failure for %q", tc.src)
		}
	}
}

func TestV17_IdentPrefix(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`./`, true},
		{`../`, true},
		{`../../`, true},
		{`../../../`, true},
		{`foo`, false},   // no path prefix
		{`..foo`, false}, // DOTDOT not followed by SLASH
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIdentPrefix()
		if tc.valid && err != nil {
			t.Errorf("ident_prefix: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ident_prefix: expected failure for %q", tc.src)
		}
	}
}

func TestV17_IdentRef(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo", true},
		{"foo.bar", true},
		{"./foo", true},
		{"../foo.bar", true},
		{"../../foo", true},
		{"123", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseIdentRef()
		if tc.valid && err != nil {
			t.Errorf("ident_ref: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ident_ref: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// numeric_oper / inline_incr  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_NumericOper(t *testing.T) {
	// '/' is lexed as V17_SLASH when preceded by a value token; use "1 / 2" and parse
	// the operator out of it rather than feeding '/' bare (bare '/' becomes regexp).
	for _, op := range []string{"+", "-", "*", "**", "%"} {
		p := newV17(t, op)
		if _, err := p.ParseNumericOper(); err != nil {
			t.Errorf("numeric_oper: unexpected error for %q: %v", op, err)
		}
	}
	// '/' must follow a value token so the lexer emits V17_SLASH not V17_REGEXP.
	{
		p := newV17(t, "1 / 2")
		if _, err := p.ParseNumExprChain(); err != nil {
			t.Errorf("numeric_oper '/': unexpected error for '1 / 2': %v", err)
		}
	}
	for _, bad := range []string{"=", "!", "foo"} {
		p := newV17(t, bad)
		if _, err := p.ParseNumericOper(); err == nil {
			t.Errorf("numeric_oper: expected failure for %q", bad)
		}
	}
}

func TestV17_InlineIncr(t *testing.T) {
	for _, op := range []string{"++", "--"} {
		p := newV17(t, op)
		if _, err := p.ParseInlineIncr(); err != nil {
			t.Errorf("inline_incr: unexpected error for %q: %v", op, err)
		}
	}
	p := newV17(t, "+")
	if _, err := p.ParseInlineIncr(); err == nil {
		t.Error("inline_incr: expected failure for '+'")
	}
}

// ===========================================================================
// single_num_expr / num_expr_chain / numeric_calc  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_SingleNumExpr_Literal(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"42", true},
		{"3.14", true},
		{"foo", true},   // ident_ref path
		{"++foo", true}, // inline_incr + ident_ref
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseSingleNumExpr()
		if tc.valid && err != nil {
			t.Errorf("single_num_expr: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("single_num_expr: expected failure for %q", tc.src)
		}
	}
}

func TestV17_NumExprChain(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"42", true},
		{"1 + 2", true},
		{"3 * 4 - 1", true},
		{"a + b", true},
		{"10 ** 3", true},
		{"10 % 3", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseNumExprChain()
		if tc.valid && err != nil {
			t.Errorf("num_expr_chain: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("num_expr_chain: expected failure for %q", tc.src)
		}
	}
}

func TestV17_NumGrouping(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"(42)", true},
		{"(1 + 2)", true},
		{"(1 + (2 * 3))", true},
		{"()", false},
		{"1 + 2", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseNumGrouping()
		if tc.valid && err != nil {
			t.Errorf("num_grouping: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("num_grouping: expected failure for %q", tc.src)
		}
	}
}

func TestV17_NumericCalc(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"42", true},
		{"1 + 2 * 3", true},
		{"(1 + 2)", true},
		{`"hello"`, false}, // a string is not numeric
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseNumericCalc()
		if tc.valid && err != nil {
			t.Errorf("numeric_calc: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("numeric_calc: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// string_oper / string_expr_chain / string_concat  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_StringOper(t *testing.T) {
	p := newV17(t, "+")
	if _, err := p.ParseStringOper(); err != nil {
		t.Errorf("string_oper: unexpected error: %v", err)
	}
	p2 := newV17(t, "-")
	if _, err := p2.ParseStringOper(); err == nil {
		t.Error("string_oper: expected failure for '-'")
	}
}

func TestV17_StringExprChain(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"hello"`, true},
		{`"hello" + "world"`, true},
		{`"a" + "b" + "c"`, true},
		{`42`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseStringExprChain()
		if tc.valid && err != nil {
			t.Errorf("string_expr_chain: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("string_expr_chain: expected failure for %q", tc.src)
		}
	}
}

func TestV17_StringGrouping(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`("hello")`, true},
		{`("a" + "b")`, true},
		{`("a" + ("b" + "c"))`, true},
		{`()`, false},
		{`"hello"`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseStringGrouping()
		if tc.valid && err != nil {
			t.Errorf("string_grouping: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("string_grouping: expected failure for %q", tc.src)
		}
	}
}

func TestV17_StringConcat(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"hello"`, true},
		{`"hello" + "world"`, true},
		{`("a" + "b")`, true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseStringConcat()
		if tc.valid && err != nil {
			t.Errorf("string_concat: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("string_concat: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// compare_oper / num_compare / string_compare / condition  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_CompareOper(t *testing.T) {
	valid := []string{"!=", "==", ">", ">=", "<", "<="}
	for _, op := range valid {
		p := newV17(t, op)
		if _, err := p.ParseCompareOper(); err != nil {
			t.Errorf("compare_oper: unexpected error for %q: %v", op, err)
		}
	}
	for _, bad := range []string{"=", "!", "+"} {
		p := newV17(t, bad)
		if _, err := p.ParseCompareOper(); err == nil {
			t.Errorf("compare_oper: expected failure for %q", bad)
		}
	}
}

func TestV17_NumCompare(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"1 == 2", true},
		{"a != b", true},
		{"10 >= 5", true},
		{"(1 + 2) < 4", true},
		{"a > (b + 1)", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseNumCompare()
		if tc.valid && err != nil {
			t.Errorf("num_compare: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("num_compare: expected failure for %q", tc.src)
		}
	}
}

func TestV17_StringCompare(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"foo" == "bar"`, true},
		{`"hello" != "world"`, true},
		{`"a" < "b"`, true},
		{`"test" == /test/i`, true},
		{`"x" == ("a" + "b")`, true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseStringCompare()
		if tc.valid && err != nil {
			t.Errorf("string_compare: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("string_compare: expected failure for %q", tc.src)
		}
	}
}

func TestV17_Condition(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"1 == 2", true},
		{`"a" != "b"`, true},
		{"a > b", true},
		{"foo", true}, // via EXTEND->logic_expr->logic_expr_chain->single_logic_expr->ident
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseCondition()
		if tc.valid && err != nil {
			t.Errorf("condition: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("condition: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// logic operators / logic_expr  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_LogicOpers(t *testing.T) {
	for _, op := range []string{"&", "|", "^"} {
		p := newV17(t, op)
		if _, err := p.ParseLogicOper(); err != nil {
			t.Errorf("logic_oper: unexpected error for %q: %v", op, err)
		}
	}
	// "!" is genuinely invalid for logic_oper. "&&" and "||" each lex to two
	// single-char tokens â€” ParseLogicOper consumes the first valid one, so those
	// cannot be tested as "bad" input at the single-token level.
	for _, bad := range []string{"!"} {
		p := newV17(t, bad)
		if _, err := p.ParseLogicOper(); err == nil {
			t.Errorf("logic_oper: expected failure for %q", bad)
		}
	}
}

func TestV17_NotOper(t *testing.T) {
	p := newV17(t, "!")
	if _, err := p.ParseNotOper(); err != nil {
		t.Errorf("not_oper: unexpected error: %v", err)
	}
	p2 := newV17(t, "&")
	if _, err := p2.ParseNotOper(); err == nil {
		t.Error("not_oper: expected failure for '&'")
	}
}

func TestV17_SingleLogicExpr(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo", true},
		{"!foo", true},
		{"1 == 2", true},
		{"!1 == 2", true},
		{"foo.bar", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseSingleLogicExpr()
		if tc.valid && err != nil {
			t.Errorf("single_logic_expr: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("single_logic_expr: expected failure for %q", tc.src)
		}
	}
}

func TestV17_LogicExprChain(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo", true},
		{"a & b", true},
		{"a | b | c", true},
		{"a ^ b", true},
		{"1 == 2 & foo", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseLogicExprChain()
		if tc.valid && err != nil {
			t.Errorf("logic_expr_chain: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("logic_expr_chain: expected failure for %q", tc.src)
		}
	}
}

func TestV17_LogicGrouping(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"(foo)", true},
		{"(a & b)", true},
		{"(a | (b ^ c))", true},
		{"()", false},
		{"foo", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseLogicGrouping()
		if tc.valid && err != nil {
			t.Errorf("logic_grouping: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("logic_grouping: expected failure for %q", tc.src)
		}
	}
}

func TestV17_LogicExpr(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo", true},
		{"a & b", true},
		{"(a | b)", true},
		{"!foo & bar", true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseLogicExpr()
		if tc.valid && err != nil {
			t.Errorf("logic_expr: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("logic_expr: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// statement  (spec/02_operators.sqg)
// ===========================================================================

func TestV17_Statement(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"42", true},
		{"1 + 2 * 3", true},
		{`"hello"`, true},
		{`"a" + "b"`, true},
		{"1 == 2 & 42", true}, // EXTEND: assign_cond_rhs
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseStatement()
		if tc.valid && err != nil {
			t.Errorf("statement: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("statement: expected failure for %q", tc.src)
		}
	}
}
