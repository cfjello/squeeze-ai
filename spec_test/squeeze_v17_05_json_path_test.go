// Package squeeze_v1_test — functional tests for spec/05_json_path.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: jp_wildcard, jp_filter_not, jp_filter_oper, jp_filter_logic
//	Level 1: jp_name, jp_index, jp_slice, jp_current_path,
//	          jp_filter_cmp, jp_filter_unary
//	Level 2: jp_filter_expr, jp_filter, jp_selector
//	Level 3: jp_selector_list, jp_bracket_seg, jp_dot_seg, jp_desc_seg
//	Level 4: jp_segment
//	Level 5: json_path
//	(EXTEND) ident_ref += json_path
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// jp_name / jp_wildcard / jp_index / jp_slice  (spec/05_json_path.sqg)
// ===========================================================================

func TestV17_JpName(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"title", true},
		{`"my-key"`, true},
		{"42", false},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpName()
		if tc.valid && err != nil {
			t.Errorf("jp_name %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_name %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpWildcard(t *testing.T) {
	p := newV17(t, "*")
	if _, err := p.ParseJpWildcard(); err != nil {
		t.Errorf("jp_wildcard '*': unexpected error: %v", err)
	}
	p2 := newV17(t, "foo")
	if _, err := p2.ParseJpWildcard(); err == nil {
		t.Error("jp_wildcard 'foo': expected failure")
	}
}

func TestV17_JpIndex(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"0", true},
		{"42", true},
		{"-1", true},
		{`"key"`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpIndex()
		if tc.valid && err != nil {
			t.Errorf("jp_index %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_index %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpSlice(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"1:5", true},
		{":3", true},
		{"1:", true},
		{":", true},
		{"::2", true},
		{"1:10:2", true},
		{"-3:", true},
		{"0", false}, // no colon → jp_index, not jp_slice
		{"foo", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpSlice()
		if tc.valid && err != nil {
			t.Errorf("jp_slice %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_slice %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// jp_filter_oper / jp_filter_not / jp_filter_logic  (spec/05_json_path.sqg)
// ===========================================================================

func TestV17_JpFilterOper(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"==", true},
		{"!=", true},
		{">=", true},
		{"<=", true},
		{">", true},
		{"<", true},
		{"=~", true},
		{"+", false},
		{"=", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpFilterOper()
		if tc.valid && err != nil {
			t.Errorf("jp_filter_oper %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_filter_oper %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpFilterNot(t *testing.T) {
	p := newV17(t, "!")
	if _, err := p.ParseJpFilterNot(); err != nil {
		t.Errorf("jp_filter_not '!': unexpected error: %v", err)
	}
	p2 := newV17(t, "x")
	if _, err := p2.ParseJpFilterNot(); err == nil {
		t.Error("jp_filter_not 'x': expected failure")
	}
}

func TestV17_JpFilterLogic(t *testing.T) {
	// "&&" and "||" are detected as two consecutive single-char tokens (SIR-4)
	cases := []struct {
		src   string
		valid bool
	}{
		{"&&", true},
		{"||", true},
		{"&", false}, // single & is not jp_filter_logic
		{"|", false}, // single | is not jp_filter_logic
		{"+", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpFilterLogic()
		if tc.valid && err != nil {
			t.Errorf("jp_filter_logic %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_filter_logic %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// jp_current_path / jp_filter_cmp / jp_filter_unary  (spec/05_json_path.sqg)
// ===========================================================================

func TestV17_JpCurrentPath(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"@", true},
		{"@.price", true},
		{"@.store.book", true},
		{"@[0]", true},
		{"foo", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpCurrentPath()
		if tc.valid && err != nil {
			t.Errorf("jp_current_path %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_current_path %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpFilterCmp(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"@.price < 10", true},
		{"@.price == maxPrice", true},
		{`@.category == "fiction"`, true},
		{"@.rating >= 4", true},
		{"@", false}, // no oper → not a comparison
		{"10", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpFilterCmp()
		if tc.valid && err != nil {
			t.Errorf("jp_filter_cmp %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_filter_cmp %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpFilterUnary(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"@.price < 10", true},
		{"!@.price < 10", true},
		{"(@.price < 10)", true},
		{"@.category", true}, // existence test
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpFilterUnary()
		if tc.valid && err != nil {
			t.Errorf("jp_filter_unary %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_filter_unary %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// jp_filter_expr / jp_filter / jp_selector  (spec/05_json_path.sqg)
// ===========================================================================

func TestV17_JpFilterExpr(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"@.price < 10", true},
		{"@.price < 10 && @.rating >= 4", true},
		{"@.a == 1 || @.b == 2", true},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpFilterExpr()
		if tc.valid && err != nil {
			t.Errorf("jp_filter_expr %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_filter_expr %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpFilter(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"?(@.price < 10)", true},
		{"?(@.category)", true},
		{"?(@.price < 10 && @.rating >= 4)", true},
		{"@.price < 10", false}, // missing ?( )
		{"?()", false},          // empty expression
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpFilter()
		if tc.valid && err != nil {
			t.Errorf("jp_filter %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_filter %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpSelector(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"?(@.price < 10)", true},
		{"1:5", true},
		{"0", true},
		{"title", true},
		{"*", true},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpSelector()
		if tc.valid && err != nil {
			t.Errorf("jp_selector %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_selector %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// jp_selector_list / jp_bracket_seg / jp_dot_seg / jp_desc_seg / jp_segment
// (spec/05_json_path.sqg)
// ===========================================================================

func TestV17_JpSelectorList(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"0", true},
		{"0, 1, 2", true},
		{"*", true},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpSelectorList()
		if tc.valid && err != nil {
			t.Errorf("jp_selector_list %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_selector_list %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpBracketSeg(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"[0]", true},
		{"[*]", true},
		{"[0, 1, 2]", true},
		{"[?(@.price < 10)]", true},
		{"[1:5]", true},
		{"0", false},
		{"[]", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpBracketSeg()
		if tc.valid && err != nil {
			t.Errorf("jp_bracket_seg %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_bracket_seg %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpDotSeg(t *testing.T) {
	// Note: the input is written directly adjacent - .title (no spaces around dot)
	cases := []struct {
		src   string
		valid bool
	}{
		{".title", true},
		{".*", true},
		{". title", false}, // space after dot violates !WS!
		{"title", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpDotSeg()
		if tc.valid && err != nil {
			t.Errorf("jp_dot_seg %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_dot_seg %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpDescSeg(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"..title", true},
		{"..*", true},
		{"..[0]", true},
		{".. title", false}, // space after .. violates !WS!
		{".title", false},   // single dot is not desc
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpDescSeg()
		if tc.valid && err != nil {
			t.Errorf("jp_desc_seg %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_desc_seg %q: expected failure", tc.src)
		}
	}
}

func TestV17_JpSegment(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"..title", true},
		{".title", true},
		{"[0]", true},
		{"[*]", true},
		{"foo", false},
		{"!", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJpSegment()
		if tc.valid && err != nil {
			t.Errorf("jp_segment %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("jp_segment %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// json_path / ident_ref+json_path  (spec/05_json_path.sqg)
// ===========================================================================

func TestV17_JsonPath(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{".$", true},
		{".$.*", true},
		{".$[0]", true},
		{".$[*].author", true},
		{".$[?(@.price < 10)]", true},
		{".$..title", true},
		{". $", false}, // space between . and $ violates !WS!
		{"$", false},   // missing leading dot
		{"foo", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseJsonPath()
		if tc.valid && err != nil {
			t.Errorf("json_path %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("json_path %q: expected failure", tc.src)
		}
	}
}

// TestV17_IdentRefWithJsonPath verifies EXTEND<ident_ref> = [ json_path ]
func TestV17_IdentRefWithJsonPath(t *testing.T) {
	cases := []struct {
		src      string
		valid    bool
		hasJPath bool // whether json_path extension is expected
	}{
		{"store", true, false},
		{"store.$", true, true},
		{"store.$.book", true, true},
		{"store.$[0]", true, true},
		{"store.$[?(@.price < 10)]", true, true},
		{"42", false, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		node, err := p.ParseIdentRef()
		if tc.valid && err != nil {
			t.Errorf("ident_ref+json_path %q: unexpected error: %v", tc.src, err)
			continue
		}
		if !tc.valid && err == nil {
			t.Errorf("ident_ref+json_path %q: expected failure", tc.src)
			continue
		}
		if tc.valid && tc.hasJPath && node.JsonPath == nil {
			t.Errorf("ident_ref+json_path %q: expected JsonPath to be set", tc.src)
		}
		if tc.valid && !tc.hasJPath && node.JsonPath != nil {
			t.Errorf("ident_ref+json_path %q: expected JsonPath to be nil", tc.src)
		}
	}
}
