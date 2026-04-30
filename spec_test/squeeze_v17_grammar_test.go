// Package squeeze_v17_test contains a specification-driven test suite for the
// Squeeze V17 definitions grammar as defined in spec/01_definitions.sqg.
//
// Each test group corresponds to a grammar rule or group of related rules.
// Tests validate that the V17 parser correctly accepts valid inputs and
// rejects invalid ones for every rule in 01_definitions.sqg.
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ===========================================================================
// Helpers
// ===========================================================================

// newV17 creates a V17Parser from src, fataling the test on lex error.
func newV17(t *testing.T, src string) *parser.V17Parser {
	t.Helper()
	p, err := parser.NewV17ParserFromSource(src)
	if err != nil {
		t.Fatalf("lex error for %q: %v", src, err)
	}
	return p
}

// ===========================================================================
// Comments (comment, comment_TBD_stub)
// ===========================================================================

func TestV17_Comment_Nested(t *testing.T) {
	cases := []struct {
		label string
		src   string
		valid bool
	}{
		{"simple", "(* hello *)", true},
		{"nested", "(* outer (* inner *) outer *)", true},
		{"tbd stub", "(* TBD_STUB *)", true},
		{"unclosed", "(* hello", false},
		{"not a comment", "hello *)", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseComment()
		if tc.valid && err != nil {
			t.Errorf("comment %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("comment %s: expected failure for %q", tc.label, tc.src)
		}
	}
}

func TestV17_CommentTbdStub(t *testing.T) {
	p := newV17(t, "(* TBD_STUB *)")
	if _, err := p.ParseCommentTbdStub(); err != nil {
		t.Errorf("comment_TBD_stub: unexpected error: %v", err)
	}

	p2 := newV17(t, "(* not a stub *)")
	if _, err := p2.ParseCommentTbdStub(); err == nil {
		t.Errorf("comment_TBD_stub: expected failure for non-stub comment")
	}
}

// ===========================================================================
// Digits (digits, digits2, digits3, digits4, sign_prefix)
// ===========================================================================

func TestV17_Digits(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"0", true},
		{"123", true},
		{"9999999999", true},
		{"abc", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDigits()
		if tc.valid && err != nil {
			t.Errorf("digits: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("digits: expected failure for %q", tc.src)
		}
	}
}

func TestV17_Digits2(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"01", true}, {"9", true}, {"99", true}, {"100", false}, {"abc", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDigits2()
		if tc.valid && err != nil {
			t.Errorf("digits2: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("digits2: expected failure for %q", tc.src)
		}
	}
}

func TestV17_Digits4(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"2024", true}, {"0001", true}, {"99", false}, {"12345", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDigits4()
		if tc.valid && err != nil {
			t.Errorf("digits4: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("digits4: expected failure for %q", tc.src)
		}
	}
}

func TestV17_SignPrefix(t *testing.T) {
	for _, src := range []string{"+", "-", ""} {
		p := newV17(t, src+"42")
		if _, err := p.ParseSignPrefix(); err != nil {
			t.Errorf("sign_prefix: unexpected error for %q: %v", src+"42", err)
		}
	}
}

// ===========================================================================
// Numeric (integer, decimal, numeric_const, nan, infinity)
// ===========================================================================

func TestV17_Integer(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"42", true},
		{"-7", true},
		{"+100", true},
		{"0", true},
		{"abc", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseInteger()
		if tc.valid && err != nil {
			t.Errorf("integer: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("integer: expected failure for %q", tc.src)
		}
	}
}

func TestV17_Decimal(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"3.14", true},
		{"-1.0", true},
		{"+0.5", true},
		{"3", false},  // no dot
		{".5", false}, // no leading digits before the sign_prefix digits "." digits pattern
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDecimal()
		if tc.valid && err != nil {
			t.Errorf("decimal: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("decimal: expected failure for %q", tc.src)
		}
	}
}

func TestV17_NumericConst(t *testing.T) {
	for _, src := range []string{"42", "-3", "3.14", "+0.0"} {
		p := newV17(t, src)
		if _, err := p.ParseNumericConst(); err != nil {
			t.Errorf("numeric_const: unexpected error for %q: %v", src, err)
		}
	}
}

func TestV17_Nan(t *testing.T) {
	p := newV17(t, "NaN")
	if _, err := p.ParseNan(); err != nil {
		t.Errorf("nan: unexpected error: %v", err)
	}
	p2 := newV17(t, "nan")
	if _, err := p2.ParseNan(); err == nil {
		t.Errorf("nan: expected failure for lowercase 'nan'")
	}
}

func TestV17_Infinity(t *testing.T) {
	p := newV17(t, "Infinity")
	if _, err := p.ParseInfinity(); err != nil {
		t.Errorf("infinity: unexpected error: %v", err)
	}
	p2 := newV17(t, "infinity")
	if _, err := p2.ParseInfinity(); err == nil {
		t.Errorf("infinity: expected failure for lowercase")
	}
}

// ===========================================================================
// Unsigned integer types (byte, uint8…uint128)
// ===========================================================================

func TestV17_Byte_ValidRange(t *testing.T) {
	for _, src := range []string{"0", "128", "255"} {
		p := newV17(t, src)
		if _, err := p.ParseByte(); err != nil {
			t.Errorf("byte: unexpected error for %q: %v", src, err)
		}
	}
}

func TestV17_Byte_InvalidRange(t *testing.T) {
	for _, src := range []string{"256", "-1"} {
		p := newV17(t, src)
		if _, err := p.ParseByte(); err == nil {
			t.Errorf("byte: expected failure for %q", src)
		}
	}
}

func TestV17_Uint16(t *testing.T) {
	valids := []string{"0", "65535", "1000"}
	for _, src := range valids {
		p := newV17(t, src)
		if _, err := p.ParseUint16(); err != nil {
			t.Errorf("uint16: unexpected error for %q: %v", src, err)
		}
	}
	p := newV17(t, "65536")
	if _, err := p.ParseUint16(); err == nil {
		t.Errorf("uint16: expected failure for 65536")
	}
}

func TestV17_Uint32(t *testing.T) {
	p := newV17(t, "4294967295")
	if _, err := p.ParseUint32(); err != nil {
		t.Errorf("uint32: unexpected error: %v", err)
	}
	p2 := newV17(t, "4294967296")
	if _, err := p2.ParseUint32(); err == nil {
		t.Errorf("uint32: expected failure for 4294967296")
	}
}

func TestV17_Uint64(t *testing.T) {
	p := newV17(t, "18446744073709551615")
	if _, err := p.ParseUint64(); err != nil {
		t.Errorf("uint64 max: unexpected error: %v", err)
	}
}

func TestV17_Uint128(t *testing.T) {
	p := newV17(t, "340282366920938463463374607431768211455")
	if _, err := p.ParseUint128(); err != nil {
		t.Errorf("uint128 max: unexpected error: %v", err)
	}
}

// ===========================================================================
// Float types (float32, float64)
// ===========================================================================

func TestV17_Float32(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"3.14", true},
		{"-1.0", true},
		{"NaN", true},
		{"Infinity", true},
		{"abc", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFloat32()
		if tc.valid && err != nil {
			t.Errorf("float32: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("float32: expected failure for %q", tc.src)
		}
	}
}

func TestV17_Float64(t *testing.T) {
	for _, src := range []string{"1.23456789012345", "-0.0", "NaN", "Infinity"} {
		p := newV17(t, src)
		if _, err := p.ParseFloat64(); err != nil {
			t.Errorf("float64: unexpected error for %q: %v", src, err)
		}
	}
}

// ===========================================================================
// Decimal number types (decimal8…decimal128, decimal_num)
// ===========================================================================

func TestV17_Decimal8(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"-128.0", true},
		{"127.99", true},
		{"0.5", true},
		{"128.0", false},
		{"-129.0", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDecimal8()
		if tc.valid && err != nil {
			t.Errorf("decimal8: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("decimal8: expected failure for %q", tc.src)
		}
	}
}

func TestV17_Decimal64(t *testing.T) {
	p := newV17(t, "-9223372036854775808.0")
	if _, err := p.ParseDecimal64(); err != nil {
		t.Errorf("decimal64 min: unexpected error: %v", err)
	}
}

func TestV17_DecimalNum_AutoSelect(t *testing.T) {
	// decimal_num selects narrowest type
	for _, src := range []string{"0.0", "127.5", "-128.1", "200.0", "32767.9", "-32768.0"} {
		p := newV17(t, src)
		if _, err := p.ParseDecimalNum(); err != nil {
			t.Errorf("decimal_num: unexpected error for %q: %v", src, err)
		}
	}
}

// ===========================================================================
// Date/Time types
// ===========================================================================

func TestV17_DateYear(t *testing.T) {
	for _, src := range []string{"2024", "0001", "9999"} {
		p := newV17(t, src)
		if _, err := p.ParseDateYear(); err != nil {
			t.Errorf("date_year: unexpected error for %q: %v", src, err)
		}
	}
}

func TestV17_DateMonth(t *testing.T) {
	valids := []string{"1", "12", "06"}
	for _, src := range valids {
		p := newV17(t, src)
		if _, err := p.ParseDateMonth(); err != nil {
			t.Errorf("date_month: unexpected error for %q: %v", src, err)
		}
	}
	for _, src := range []string{"0", "13"} {
		p := newV17(t, src)
		if _, err := p.ParseDateMonth(); err == nil {
			t.Errorf("date_month: expected failure for %q", src)
		}
	}
}

func TestV17_DateDay(t *testing.T) {
	p := newV17(t, "31")
	if _, err := p.ParseDateDay(); err != nil {
		t.Errorf("date_day: unexpected error: %v", err)
	}
	p2 := newV17(t, "32")
	if _, err := p2.ParseDateDay(); err == nil {
		t.Errorf("date_day: expected failure for 32")
	}
}

func TestV17_Date_WithSeparators(t *testing.T) {
	cases := []string{
		"2024-04-15",
		"2024-04",
		"2024",
		// compact forms like "20240415" are NOT valid: the DIGITS token
		// has 8 chars which doesn't match the exact-4-digit digits4 rule.
	}
	for _, src := range cases {
		p := newV17(t, src)
		if _, err := p.ParseDate(); err != nil {
			t.Errorf("date: unexpected error for %q: %v", src, err)
		}
	}
}

func TestV17_Time_WithSeparators(t *testing.T) {
	cases := []string{
		"23:59:59.999",
		"23:59:59",
		"23:59",
		"23",
		// compact forms like "235959999" are NOT valid per grammar:
		// the single DIGITS token doesn't match the 1-2 digit digits2 rule.
	}
	for _, src := range cases {
		p := newV17(t, src)
		if _, err := p.ParseTime(); err != nil {
			t.Errorf("time: unexpected error for %q: %v", src, err)
		}
	}
}

func TestV17_TimeHour_Range(t *testing.T) {
	for _, src := range []string{"0", "23", "12"} {
		p := newV17(t, src)
		if _, err := p.ParseTimeHour(); err != nil {
			t.Errorf("time_hour: unexpected error for %q: %v", src, err)
		}
	}
	p := newV17(t, "24")
	if _, err := p.ParseTimeHour(); err == nil {
		t.Errorf("time_hour: expected failure for 24")
	}
}

func TestV17_DateTime(t *testing.T) {
	cases := []string{
		"2024-04-15 23:59",
		"2024-04-15",
		"23:59:59",
	}
	for _, src := range cases {
		p := newV17(t, src)
		if _, err := p.ParseDateTime(); err != nil {
			t.Errorf("date_time: unexpected error for %q: %v", src, err)
		}
	}
}

func TestV17_TimeStamp(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"2024-04-15 23:59:59", true},
		{"2024-04-15 00:00:00.000", true},
		{"2024-04-15", false}, // time required
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseTimeStamp()
		if tc.valid && err != nil {
			t.Errorf("time_stamp: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("time_stamp: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// Duration
// ===========================================================================

func TestV17_DurationUnit(t *testing.T) {
	for _, src := range []string{"ms", "s", "m", "h", "d", "w"} {
		p := newV17(t, src)
		if _, err := p.ParseDurationUnit(); err != nil {
			t.Errorf("duration_unit: unexpected error for %q: %v", src, err)
		}
	}
	p := newV17(t, "x")
	if _, err := p.ParseDurationUnit(); err == nil {
		t.Errorf("duration_unit: expected failure for 'x'")
	}
}

func TestV17_Duration(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"1h", true},
		{"1h30m", true},
		{"500ms", true},
		{"3d", true},
		{"2w", true},
		{"90s", true},
		{"h", false}, // no digits
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDuration()
		if tc.valid && err != nil {
			t.Errorf("duration: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("duration: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// String types (single_quoted, double_quoted, string_quoted, tmpl_quoted, string)
// ===========================================================================

func TestV17_SingleQuoted(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`'hello'`, true},
		{`'it\'s'`, true},
		{`''`, false}, // empty not allowed by spec pattern (\\'|[^'])+
		{`"hello"`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseSingleQuoted()
		if tc.valid && err != nil {
			t.Errorf("single_quoted: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("single_quoted: expected failure for %q", tc.src)
		}
	}
}

func TestV17_DoubleQuoted(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"hello"`, true},
		{`"say \"hi\""`, true},
		{`""`, false},
		{`'hello'`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDoubleQuoted()
		if tc.valid && err != nil {
			t.Errorf("double_quoted: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("double_quoted: expected failure for %q", tc.src)
		}
	}
}

func TestV17_TmplQuoted(t *testing.T) {
	cases := []string{
		"`hello world`",
		"`no interpolation here`",
	}
	for _, src := range cases {
		p := newV17(t, src)
		if _, err := p.ParseTmplQuoted(); err != nil {
			t.Errorf("tmpl_quoted: unexpected error for %q: %v", src, err)
		}
	}
}

func TestV17_String(t *testing.T) {
	for _, src := range []string{`'hi'`, `"hi"`, "`hi`"} {
		p := newV17(t, src)
		if _, err := p.ParseString(); err != nil {
			t.Errorf("string: unexpected error for %q: %v", src, err)
		}
	}
}

// ===========================================================================
// Regexp
// ===========================================================================

func TestV17_RegexpExpr(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`/hello/`, true},
		{`/hello/gi`, true},
		{`/[0-9]+/imsuyxnA`, true},
		// "/abc/z" — 'z' is not a valid flag, but since flags are optional in the
		// grammar the parser just stops after the pattern. z remains unconsumed.
		// A valid regexp is returned (flags=[]) and z is the next token.
		{`/abc/z`, true},
		// bare "/" causes a lex error (unterminated regexp) — tested separately
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseRegexpExpr()
		if tc.valid && err != nil {
			t.Errorf("regexp_expr: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("regexp_expr: expected failure for %q", tc.src)
		}
	}
	// bare slash is a lex error
	_, lexErr := parser.NewV17ParserFromSource("/")
	if lexErr == nil {
		t.Errorf("regexp_expr: expected lex error for bare '/'")
	}
}

// ===========================================================================
// Boolean, null, any_type
// ===========================================================================

func TestV17_Boolean(t *testing.T) {
	for _, src := range []string{"true", "false"} {
		p := newV17(t, src)
		if _, err := p.ParseBoolean(); err != nil {
			t.Errorf("boolean: unexpected error for %q: %v", src, err)
		}
	}
	p := newV17(t, "True")
	if _, err := p.ParseBoolean(); err == nil {
		t.Errorf("boolean: expected failure for 'True'")
	}
}

func TestV17_Null(t *testing.T) {
	p := newV17(t, "null")
	if _, err := p.ParseNull(); err != nil {
		t.Errorf("null: unexpected error: %v", err)
	}
	p2 := newV17(t, "NULL")
	if _, err := p2.ParseNull(); err == nil {
		t.Errorf("null: expected failure for 'NULL'")
	}
}

func TestV17_AnyType(t *testing.T) {
	p := newV17(t, "@?")
	if _, err := p.ParseAnyType(); err != nil {
		t.Errorf("any_type: unexpected error: %v", err)
	}
	p2 := newV17(t, "@")
	if _, err := p2.ParseAnyType(); err == nil {
		t.Errorf("any_type: expected failure for '@' alone")
	}
}

// ===========================================================================
// Cardinality and Range
// ===========================================================================

func TestV17_Cardinality(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"0..1", true},
		{"1..10", true},
		{"0..m", true},
		{"0..M", true},
		{"0..many", true},
		{"0..Many", true},
		{"1", false},    // no ..
		{"a..b", false}, // not digits
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseCardinality()
		if tc.valid && err != nil {
			t.Errorf("cardinality: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("cardinality: expected failure for %q", tc.src)
		}
	}
}

func TestV17_Range(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"-10..10", true},
		{"0..255", true},
		{"1..1", true},
		{"a..b", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseRange()
		if tc.valid && err != nil {
			t.Errorf("range: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("range: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// Hex segments
// ===========================================================================

func TestV17_HexSeg8(t *testing.T) {
	p := newV17(t, "deadbeef")
	if _, err := p.ParseHexSeg8(); err != nil {
		t.Errorf("hex_seg8: unexpected error: %v", err)
	}
	p2 := newV17(t, "dead")
	if _, err := p2.ParseHexSeg8(); err == nil {
		t.Errorf("hex_seg8: expected failure for 4-char hex")
	}
}

func TestV17_HexSeg4(t *testing.T) {
	p := newV17(t, "1a2b")
	if _, err := p.ParseHexSeg4(); err != nil {
		t.Errorf("hex_seg4: unexpected error: %v", err)
	}
}

func TestV17_HexSeg12(t *testing.T) {
	p := newV17(t, "1a2b3c4d5e6f")
	if _, err := p.ParseHexSeg12(); err != nil {
		t.Errorf("hex_seg12: unexpected error: %v", err)
	}
}

func TestV17_HexSeg32_MD5Length(t *testing.T) {
	p := newV17(t, "d41d8cd98f00b204e9800998ecf8427e")
	if _, err := p.ParseHexSeg32(); err != nil {
		t.Errorf("hex_seg32: unexpected error: %v", err)
	}
}

func TestV17_HexSeg40_SHA1Length(t *testing.T) {
	p := newV17(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709")
	if _, err := p.ParseHexSeg40(); err != nil {
		t.Errorf("hex_seg40: unexpected error: %v", err)
	}
}

// ===========================================================================
// UUID
// ===========================================================================

func TestV17_Uuid(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"DEADBEEF-DEAD-BEEF-DEAD-BEEFDEADBEEF", true},
		{"550e8400-e29b-41d4-a716", false}, // truncated
		{"not-a-uuid-at-all-here", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseUuid()
		if tc.valid && err != nil {
			t.Errorf("uuid: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("uuid: expected failure for %q", tc.src)
		}
	}
}

func TestV17_UuidV7(t *testing.T) {
	// Valid UUID v7: format tttttttt-tttt-7xxx-yxxx-xxxxxxxxxxxx where y ∈ {8,9,a,b}
	valid := "018f1e3c-6b4a-7abc-8def-0123456789ab"
	p := newV17(t, valid)
	if _, err := p.ParseUuidV7(); err != nil {
		t.Errorf("uuid_v7: unexpected error for %q: %v", valid, err)
	}

	// Version nibble is not 7
	invalid := "018f1e3c-6b4a-4abc-8def-0123456789ab"
	p2 := newV17(t, invalid)
	if _, err := p2.ParseUuidV7(); err == nil {
		t.Errorf("uuid_v7: expected failure for version-4 UUID %q", invalid)
	}

	// Variant nibble not in {8,9,a,b}
	invalid2 := "018f1e3c-6b4a-7abc-cdef-0123456789ab"
	p3 := newV17(t, invalid2)
	if _, err := p3.ParseUuidV7(); err == nil {
		t.Errorf("uuid_v7: expected failure for bad variant %q", invalid2)
	}
}

// ===========================================================================
// Hash keys
// ===========================================================================

func TestV17_HashMd5(t *testing.T) {
	p := newV17(t, "d41d8cd98f00b204e9800998ecf8427e")
	if _, err := p.ParseHashMd5(); err != nil {
		t.Errorf("hash_md5: unexpected error: %v", err)
	}
	p2 := newV17(t, "d41d8cd98f00b204e9800998ecf8427") // 31 chars
	if _, err := p2.ParseHashMd5(); err == nil {
		t.Errorf("hash_md5: expected failure for 31-char hex")
	}
}

func TestV17_HashSha1(t *testing.T) {
	p := newV17(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709")
	if _, err := p.ParseHashSha1(); err != nil {
		t.Errorf("hash_sha1: unexpected error: %v", err)
	}
}

func TestV17_HashSha256(t *testing.T) {
	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	p := newV17(t, hash)
	if _, err := p.ParseHashSha256(); err != nil {
		t.Errorf("hash_sha256: unexpected error: %v", err)
	}
}

func TestV17_HashSha512(t *testing.T) {
	// SHA-512 of empty string split into two halves for easy counting (64+64=128 chars).
	hash128 := "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce" +
		"47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
	if len(hash128) != 128 {
		t.Fatalf("test setup error: expected 128-char hash, got %d", len(hash128))
	}
	p2 := newV17(t, hash128)
	if _, err := p2.ParseHashSha512(); err != nil {
		t.Errorf("hash_sha512 (128-char): unexpected error: %v", err)
	}

	// One char short — should fail.
	hash127 := hash128[:127]
	p := newV17(t, hash127)
	if _, err := p.ParseHashSha512(); err == nil {
		t.Errorf("hash_sha512 (127-char): expected failure")
	}
}

func TestV17_HashKey_Alternation(t *testing.T) {
	// hash_key = hash_md5 | hash_sha1 | hash_sha256 | hash_sha512
	// MD5 (32)
	p := newV17(t, "d41d8cd98f00b204e9800998ecf8427e")
	if _, err := p.ParseHashKey(); err != nil {
		t.Errorf("hash_key(md5): unexpected error: %v", err)
	}
	// SHA1 (40)
	p2 := newV17(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709")
	if _, err := p2.ParseHashKey(); err != nil {
		t.Errorf("hash_key(sha1): unexpected error: %v", err)
	}
}

// ===========================================================================
// ULID and Nano ID
// ===========================================================================

func TestV17_Ulid(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"01ARZ3NDEKTSV4RRFFQ69G5FAV", true},
		{"7ZZZZZZZZZZZZZZZZZZZZZZZZZ", true},
		{"01ARZ3NDEKTSV4RRFFQ69G5FA", false},  // 25 chars
		{"IXXXXXXXXXXXXXXXXXXXXXXXX0", false}, // 'I' not in Crockford base32
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseUlid()
		if tc.valid && err != nil {
			t.Errorf("ulid: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ulid: expected failure for %q", tc.src)
		}
	}
}

func TestV17_NanoId(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"V1StGXR8_Z5jdHi6B-myT", true},
		{"abc123_-ABC123_-abcde", true},
		{"abc123_-ABC123_-abcd", false},  // 20 chars
		{"abc 23_-ABC123_-abcde", false}, // space
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseNanoId()
		if tc.valid && err != nil {
			t.Errorf("nano_id: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("nano_id: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// Snowflake ID and Sequence IDs
// ===========================================================================

func TestV17_SnowflakeId(t *testing.T) {
	// snowflake_id = uint64 (grammar treats it as uint64)
	p := newV17(t, "1701388800000000000")
	if _, err := p.ParseSnowflakeId(); err != nil {
		t.Errorf("snowflake_id: unexpected error: %v", err)
	}
	p2 := newV17(t, "18446744073709551616") // uint64 overflow
	if _, err := p2.ParseSnowflakeId(); err == nil {
		t.Errorf("snowflake_id: expected failure for uint64 overflow")
	}
}

func TestV17_SeqId(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"42", true},         // fits uint16
		{"65535", true},      // max uint16
		{"65536", true},      // fits uint32
		{"4294967295", true}, // max uint32
		{"4294967296", true}, // fits uint64
		{"abc", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseSeqId()
		if tc.valid && err != nil {
			t.Errorf("seq_id: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("seq_id: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// Unique Key (union of all key types)
// ===========================================================================

func TestV17_UniqueKey(t *testing.T) {
	cases := []struct {
		label string
		src   string
		valid bool
	}{
		{"uuid", "550e8400-e29b-41d4-a716-446655440000", true},
		{"uuid_v7", "018f1e3c-6b4a-7abc-8def-0123456789ab", true},
		{"ulid", "01ARZ3NDEKTSV4RRFFQ69G5FAV", true},
		{"snowflake", "1701388800000000000", true},
		{"nano_id", "V1StGXR8_Z5jdHi6B-myT", true},
		{"hash_md5", "d41d8cd98f00b204e9800998ecf8427e", true},
		{"seq_id", "42", true},
		{"invalid", "hello world", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseUniqueKey()
		if tc.valid && err != nil {
			t.Errorf("unique_key %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("unique_key %s: expected failure for %q", tc.label, tc.src)
		}
	}
}

// ===========================================================================
// HTTP URL and File URL
// ===========================================================================

func TestV17_HttpUrl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"https://example.com"`, true},
		{`"http://foo.bar/path?q=1#anchor"`, true},
		{`"ftp://example.com"`, false}, // ftp not http/https
		{`"not a url"`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseHttpUrl()
		if tc.valid && err != nil {
			t.Errorf("http_url: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("http_url: expected failure for %q", tc.src)
		}
	}
}

func TestV17_FileUrl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"file:///home/user/file.txt"`, true},
		{`"file://./relative"`, false}, // not matching pattern
		{`"https://foo.com"`, false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseFileUrl()
		if tc.valid && err != nil {
			t.Errorf("file_url: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("file_url: expected failure for %q", tc.src)
		}
	}
}

// ===========================================================================
// Constant (top-level union)
// ===========================================================================

func TestV17_Constant(t *testing.T) {
	cases := []struct {
		label string
		src   string
		valid bool
	}{
		{"integer", "42", true},
		{"decimal", "3.14", true},
		{"single_quoted", `'hello'`, true},
		{"double_quoted", `"world"`, true},
		{"regexp", `/\d+/gi`, true},
		{"boolean_true", "true", true},
		{"boolean_false", "false", true},
		{"null", "null", true},
		{"date", "2024-04-15", true},
		{"time", "23:59:59", true},
		{"datetime", "2024-04-15 23:59", true},
		{"uuid", "550e8400-e29b-41d4-a716-446655440000", true},
		{"http_url", `"https://example.com"`, true},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseConstant()
		if tc.valid && err != nil {
			t.Errorf("constant %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("constant %s: expected failure for %q", tc.label, tc.src)
		}
	}
}

// ===========================================================================
// 02_operators.sqg — ident_name / group_begin / group_end
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
// 02_operators.sqg — ident_dotted / ident_prefix / ident_ref
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
// 02_operators.sqg — numeric_oper / inline_incr
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
// 02_operators.sqg — single_num_expr / num_expr_chain / numeric_calc
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
// 02_operators.sqg — string_oper / string_expr_chain / string_concat
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
// 02_operators.sqg — compare_oper / num_compare / string_compare / condition
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
// 02_operators.sqg — logic operators / logic_expr
// ===========================================================================

func TestV17_LogicOpers(t *testing.T) {
	for _, op := range []string{"&", "|", "^"} {
		p := newV17(t, op)
		if _, err := p.ParseLogicOper(); err != nil {
			t.Errorf("logic_oper: unexpected error for %q: %v", op, err)
		}
	}
	// "!" is genuinely invalid for logic_oper. "&&" and "||" each lex to two
	// single-char tokens — ParseLogicOper consumes the first valid one, so those
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
// 02_operators.sqg — statement
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

// ===========================================================================
// 03_assignment.sqg
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

func TestV17_AssignVersion(t *testing.T) {
	cases := []struct {
		src   string
		hasV  bool
		parts []string
	}{
		{"1", false, []string{"1"}},
		{"1.2", false, []string{"1", "2"}},
		{"1.2.3", false, []string{"1", "2", "3"}},
		{"v1", true, []string{"1"}},
		{"v1.2", true, []string{"1", "2"}},
		{"v2.0.1", true, []string{"2", "0", "1"}},
	}
	for _, tc := range cases {
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
}

func TestV17_AssignLhs(t *testing.T) {
	cases := []struct {
		src         string
		name        string
		annotations int
	}{
		{"foo", "foo", 0},
		{"foo, bar", "foo", 1},
		{"foo, 1*", "foo", 1},   // cardinality annotation
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

func TestV17_AssignSingle(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"foo = 42", true},      // assign_lhs assign_mutable statement (numeric)
		{`foo : "hello"`, true}, // assign_lhs assign_immutable statement (string)
		{"foo :~ true", true},   // assign_lhs assign_read_only_ref statement (bool constant)
		{"1 == 2 & 42", true},   // assign_cond_rhs
		{"foo 42", false},       // no operator — no longer valid
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

func TestV17_SelfRef(t *testing.T) {
	p := newV17(t, "$")
	if _, err := p.ParseSelfRef(); err != nil {
		t.Errorf("self_ref: unexpected error: %v", err)
	}
	p2 := newV17(t, "foo")
	if _, err := p2.ParseSelfRef(); err == nil {
		t.Error("self_ref: expected failure for 'foo'")
	}
}
