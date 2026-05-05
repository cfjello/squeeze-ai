// Package squeeze_v1_test — functional tests for spec/04_objects.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: empty_array_decl, func_stream_decl, func_regexp_decl,
//	          empty_scope_decl, func_string_decl, spread_oper
//	Level 1: empty_decl, spread_array, values_list, lookup_idx_expr,
//	          lookup_txt_expr, object_init
//	Level 2: array_uniform, empty_array_typed, array_append_tail,
//	          array_omit_tail, object_merge_tail, object_omit_tail
//	Level 3: array_final, array_lookup, object_final, object_lookup
//	Level 4: collection
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// empty_array_decl / func_stream_decl / func_regexp_decl
// empty_scope_decl / func_string_decl  (spec/04_objects.sqg)
// ===========================================================================

func TestV17_EmptyArrayDecl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"[]", true},
		{"[1]", false},
		{"{}", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseEmptyArrayDecl()
		if tc.valid && err != nil {
			t.Errorf("empty_array_decl %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("empty_array_decl %q: expected failure", tc.src)
		}
	}
}

func TestV17_FuncStreamDecl(t *testing.T) {
	p := newV17(t, ">>")
	if _, err := p.ParseFuncStreamDecl(); err != nil {
		t.Errorf("func_stream_decl '>>': unexpected error: %v", err)
	}
	p2 := newV17(t, ">")
	if _, err := p2.ParseFuncStreamDecl(); err == nil {
		t.Error("func_stream_decl '>': expected failure for single '>'")
	}
}

func TestV17_FuncRegexpDecl(t *testing.T) {
	// "//" in non-value context lexes as V17_REGEXP with empty value
	p := newV17(t, "//")
	if _, err := p.ParseFuncRegexpDecl(); err != nil {
		t.Errorf("func_regexp_decl '//': unexpected error: %v", err)
	}
	p2 := newV17(t, "/abc/")
	if _, err := p2.ParseFuncRegexpDecl(); err == nil {
		t.Error("func_regexp_decl '/abc/': expected failure for non-empty regexp")
	}
}

func TestV17_EmptyScopeDecl(t *testing.T) {
	p := newV17(t, "{}")
	if _, err := p.ParseEmptyScopeDecl(); err != nil {
		t.Errorf("empty_scope_decl '{}': unexpected error: %v", err)
	}
	p2 := newV17(t, "{x}")
	if _, err := p2.ParseEmptyScopeDecl(); err == nil {
		t.Error("empty_scope_decl '{x}': expected failure")
	}
}

func TestV17_FuncStringDecl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`""`, true},
		{"''", true},
		{"``", true},
		{`"hello"`, false},
		{"42", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFuncStringDecl()
		if tc.valid && err != nil {
			t.Errorf("func_string_decl %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("func_string_decl %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// empty_decl  (spec/04_objects.sqg — union)
// ===========================================================================

func TestV17_EmptyDecl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"[]", true},
		{">>", true},
		{"//", true},
		{"{}", true},
		{`""`, true},
		{"''", true},
		{"42", false},
		{"foo", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseEmptyDecl()
		if tc.valid && err != nil {
			t.Errorf("empty_decl %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("empty_decl %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// spread_oper / spread_array  (spec/04_objects.sqg)
// ===========================================================================

func TestV17_SpreadOper(t *testing.T) {
	p := newV17(t, "...")
	if _, err := p.ParseSpreadOper(); err != nil {
		t.Errorf("spread_oper '...': unexpected error: %v", err)
	}
	p2 := newV17(t, "..")
	if _, err := p2.ParseSpreadOper(); err == nil {
		t.Error("spread_oper '..': expected failure (only two dots)")
	}
}

func TestV17_SpreadArray(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"...42", true},
		{`..."hello"`, true},
		{"...123", true},
		{"..", false},
		{"42", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseSpreadArray()
		if tc.valid && err != nil {
			t.Errorf("spread_array %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("spread_array %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// values_list  (spec/04_objects.sqg)
// ===========================================================================

func TestV17_ValuesList(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"42", true},
		{"1, 2, 3", true},
		{`"a", "b"`, true},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseValuesList()
		if tc.valid && err != nil {
			t.Errorf("values_list %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("values_list %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// array_uniform / empty_array_typed  (spec/04_objects.sqg)
// ===========================================================================

func TestV17_ArrayUniform(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"[]", true},
		{"[42]", true},
		{"[1, 2, 3]", true},
		{"[1, 2, 3,]", true},
		{`["a", "b"]`, true},
		{"[[1, 2], [3, 4]]", true},
		{"[...42]", true},
		{"42", false},
		{"[", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseArrayUniform()
		if tc.valid && err != nil {
			t.Errorf("array_uniform %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("array_uniform %q: expected failure", tc.src)
		}
	}
}

func TestV17_EmptyArrayTyped(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"myType[]", true},
		{"foo[]", true},
		{"42[]", false},
		{"foo", false},
		{"foo[1]", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseEmptyArrayTyped()
		if tc.valid && err != nil {
			t.Errorf("empty_array_typed %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("empty_array_typed %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// array_append_tail / array_omit_tail  (spec/04_objects.sqg)
// ===========================================================================

func TestV17_ArrayAppendTail(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"+ [4, 5]", true},
		{"+ [] + [1]", true},
		{"[4, 5]", false},
		{"- 0", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseArrayAppendTail()
		if tc.valid && err != nil {
			t.Errorf("array_append_tail %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("array_append_tail %q: expected failure", tc.src)
		}
	}
}

func TestV17_ArrayOmitTail(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"- 0", true},
		{"- 0, 1, 2", true},
		{"+ [1]", false},
		{"0", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseArrayOmitTail()
		if tc.valid && err != nil {
			t.Errorf("array_omit_tail %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("array_omit_tail %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// array_final / lookup_idx_expr / array_lookup  (spec/04_objects.sqg)
// ===========================================================================

func TestV17_ArrayFinal(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"[1, 2, 3]", true},
		{"myArr[]", true},
		{"myArr", true},
		{"[1] + [2]", true},
		{"[1] - 0", true},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseArrayFinal()
		if tc.valid && err != nil {
			t.Errorf("array_final %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("array_final %q: expected failure", tc.src)
		}
	}
}

func TestV17_LookupIdxExpr(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"0", true},
		{"42", true},
		{"idx", true},
		{"i + 1", true},
		{`"key"`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseLookupIdxExpr()
		if tc.valid && err != nil {
			t.Errorf("lookup_idx_expr %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("lookup_idx_expr %q: expected failure", tc.src)
		}
	}
}

func TestV17_ArrayLookup(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"myArr[0]", true},
		{"myArr[1][2]", true},
		{"myArr[i]", true},
		{"myArr[]", false},
		{"myArr", false},
		{"42", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseArrayLookup()
		if tc.valid && err != nil {
			t.Errorf("array_lookup %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("array_lookup %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// object_init / object_merge_tail / object_omit_tail / object_final
// (spec/04_objects.sqg)
// ===========================================================================

func TestV17_ObjectInit(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"[x : 1]", true},
		{"[x : 1, y : 2]", true},
		{"[x : 1, y : 2,]", true},
		{"[]", true},
		{"[42]", false},
		{"42", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseObjectInit()
		if tc.valid && err != nil {
			t.Errorf("object_init %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("object_init %q: expected failure", tc.src)
		}
	}
}

func TestV17_ObjectMergeTail(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"+ [a : 3]", true},
		{"+ myObj", true},
		{"+ [a : 1] + [b : 2]", true},
		{"[a : 1]", false},
		{"- x", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseObjectMergeTail()
		if tc.valid && err != nil {
			t.Errorf("object_merge_tail %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("object_merge_tail %q: expected failure", tc.src)
		}
	}
}

func TestV17_ObjectOmitTail(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"- x", true},
		{"- x, y", true},
		{"- 0", true},
		{"x", false},
		{"+ [a : 1]", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseObjectOmitTail()
		if tc.valid && err != nil {
			t.Errorf("object_omit_tail %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("object_omit_tail %q: expected failure", tc.src)
		}
	}
}

func TestV17_ObjectFinal(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"[x : 1]", true},
		{"myObj[]", true},
		{"[x : 1] + [y : 2]", true},
		{"[x : 1] - z", true},
		{"42", false},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseObjectFinal()
		if tc.valid && err != nil {
			t.Errorf("object_final %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("object_final %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// lookup_txt_expr / object_lookup / collection  (spec/04_objects.sqg)
// ===========================================================================

func TestV17_LookupTxtExpr(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"key"`, true},
		{"myKey", true},
		{`"a" ++ "b"`, true},
		{"42", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseLookupTxtExpr()
		if tc.valid && err != nil {
			t.Errorf("lookup_txt_expr %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("lookup_txt_expr %q: expected failure", tc.src)
		}
	}
}

func TestV17_ObjectLookup(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`myObj["key"]`, true},
		{`myObj["a"]["b"]`, true},
		{"myObj[myKey]", true},
		{"myObj[]", false},
		{"myObj[0]", false},
		{"myObj", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseObjectLookup()
		if tc.valid && err != nil {
			t.Errorf("object_lookup %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("object_lookup %q: expected failure", tc.src)
		}
	}
}

func TestV17_Collection(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"1..10", true},
		{"[1, 2, 3]", true},
		{"myArr", true},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseCollection()
		if tc.valid && err != nil {
			t.Errorf("collection %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("collection %q: expected failure", tc.src)
		}
	}
}
