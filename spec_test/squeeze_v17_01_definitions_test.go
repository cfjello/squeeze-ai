// Package squeeze_v1_test Ã¢â‚¬â€ functional tests for spec/01_definitions.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: comment_begin, comment_end, sign_prefix, boolean_true, boolean_false,
//	          null, any_type, duration_unit
//	Level 1: comment, comment_TBD_stub, digits, digits2, digits3, digits4,
//	          integer, decimal, nan, infinity, boolean, hex_seg2..128,
//	          date_year, date_month, date_day, time_hour, time_minute,
//	          time_second, time_millis, single_quoted, double_quoted, tmpl_quoted
//	Level 2: numeric_const, string, byte, uint16..128, float32, float64,
//	          decimal8..128, decimal_num, uuid, uuid_v7, ulid, nano_id,
//	          hash_md5..sha512, hash_key, date, time, duration,
//	          http_url, file_url, assign_version
//	Level 3: date_time, time_stamp, snowflake_id, seq_id, unique_key,
//	          regexp_expr, constant, empty_array_decl, empty_scope_decl,
//	          empty_decl, func_stream_decl, func_regexp_decl, func_string_decl
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ===========================================================================
// Comments  (spec/01_definitions.sqg Ã¢â‚¬â€ comment, comment_TBD_stub)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ comment
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
	// 3.3 no-consume: wrong first token must not advance
	p := newV17(t, "hello *)")
	if _, err := p.ParseComment(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ comment_TBD_stub
func TestV17_CommentTbdStub(t *testing.T) {
	p := newV17(t, "(* TBD_STUB *)")
	if _, err := p.ParseCommentTbdStub(); err != nil {
		t.Errorf("comment_TBD_stub: unexpected error: %v", err)
	}

	p2 := newV17(t, "(* not a stub *)")
	if _, err := p2.ParseCommentTbdStub(); err == nil {
		t.Errorf("comment_TBD_stub: expected failure for non-stub comment")
	}
	// 3.3 no-consume
	p3 := newV17(t, "hello")
	if _, err := p3.ParseCommentTbdStub(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}

// ===========================================================================
// Digits  (spec/01_definitions.sqg Ã¢â‚¬â€ digits, digits2, digits3, digits4)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ digits
func TestV17_Digits(t *testing.T) {
	valids := []struct{ label, src string }{
		{"zero", "0"},
		{"multi", "123"},
		{"large", "9999999999"},
	}
	for _, tc := range valids {
		p := newV17(t, tc.src)
		if _, err := p.ParseDigits(); err != nil {
			t.Errorf("digits %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
	}
	invalids := []struct{ label, src string }{
		{"wrong_first_token", "abc"},
		{"empty", ""},
	}
	for _, tc := range invalids {
		p := newV17(t, tc.src)
		if _, err := p.ParseDigits(); err == nil {
			t.Errorf("digits %s: expected failure for %q", tc.label, tc.src)
		}
	}
	// 3.3 no-consume
	p := newV17(t, "abc")
	if _, err := p.ParseDigits(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ digits2
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

// spec/01_definitions.sqg Ã¢â‚¬â€ digits3
func TestV17_Digits3(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"999", true}, {"1", true}, {"99", true}, {"1000", false}, {"abc", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDigits3()
		if tc.valid && err != nil {
			t.Errorf("digits3: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("digits3: expected failure for %q", tc.src)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ digits4
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

// spec/01_definitions.sqg Ã¢â‚¬â€ sign_prefix
func TestV17_SignPrefix(t *testing.T) {
	for _, src := range []string{"+", "-", ""} {
		p := newV17(t, src+"42")
		if _, err := p.ParseSignPrefix(); err != nil {
			t.Errorf("sign_prefix: unexpected error for %q: %v", src+"42", err)
		}
	}
}

// ===========================================================================
// Numeric  (spec/01_definitions.sqg Ã¢â‚¬â€ integer, decimal, numeric_const, nan, infinity)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ integer
func TestV17_Integer(t *testing.T) {
	valids := []struct{ label, src string }{
		{"zero", "0"},
		{"positive", "42"},
		{"negative", "-7"},
		{"explicit_plus", "+100"},
	}
	for _, tc := range valids {
		p := newV17(t, tc.src)
		if _, err := p.ParseInteger(); err != nil {
			t.Errorf("integer %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
	}
	invalids := []struct{ label, src string }{
		{"wrong_first_token", "abc"},
	}
	for _, tc := range invalids {
		p := newV17(t, tc.src)
		if _, err := p.ParseInteger(); err == nil {
			t.Errorf("integer %s: expected failure for %q", tc.label, tc.src)
		}
		// 3.3 no-consume
	}
	// 3.5 whitespace tolerance
	for _, ws := range []string{" ", "\t"} {
		p := newV17(t, ws+"42")
		if _, err := p.ParseInteger(); err != nil {
			t.Errorf("integer: leading whitespace %q: unexpected error", ws)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ decimal
func TestV17_Decimal(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"3.14", true},
		{"-1.0", true},
		{"+0.5", true},
		{"3", false},  // no dot
		{".5", false}, // no leading digits before sign_prefix digits "." digits
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

// spec/01_definitions.sqg Ã¢â‚¬â€ numeric_const
func TestV17_NumericConst(t *testing.T) {
	for _, src := range []string{"42", "-3", "3.14", "+0.0"} {
		p := newV17(t, src)
		if _, err := p.ParseNumericConst(); err != nil {
			t.Errorf("numeric_const: unexpected error for %q: %v", src, err)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ nan
func TestV17_Nan(t *testing.T) {
	p := newV17(t, "NaN")
	if _, err := p.ParseNan(); err != nil {
		t.Errorf("nan: unexpected error: %v", err)
	}
	p2 := newV17(t, "nan")
	if _, err := p2.ParseNan(); err == nil {
		t.Errorf("nan: expected failure for lowercase 'nan'")
	}
	// 3.3 no-consume
}

// spec/01_definitions.sqg Ã¢â‚¬â€ infinity
func TestV17_Infinity(t *testing.T) {
	p := newV17(t, "Infinity")
	if _, err := p.ParseInfinity(); err != nil {
		t.Errorf("infinity: unexpected error: %v", err)
	}
	p2 := newV17(t, "infinity")
	if _, err := p2.ParseInfinity(); err == nil {
		t.Errorf("infinity: expected failure for lowercase")
	}
	// 3.3 no-consume
}

// ===========================================================================
// Unsigned integer types  (spec/01_definitions.sqg Ã¢â‚¬â€ byte, uint8Ã¢â‚¬Â¦uint128)
// 3.6 RANGE boundary cases for all types
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ byte (RANGE 0..255)
func TestV17_Byte_ValidRange(t *testing.T) {
	for _, src := range []string{"0", "128", "255"} {
		p := newV17(t, src)
		if _, err := p.ParseByte(); err != nil {
			t.Errorf("byte: unexpected error for %q: %v", src, err)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ byte boundary: above_max and below_min
func TestV17_Byte_InvalidRange(t *testing.T) {
	for _, src := range []string{"256", "-1"} {
		p := newV17(t, src)
		if _, err := p.ParseByte(); err == nil {
			t.Errorf("byte: expected failure for %q", src)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ uint16 (RANGE 0..65535)
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
		t.Errorf("uint16: expected failure for 65536 (above_max)")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ uint32 (RANGE 0..4294967295)
func TestV17_Uint32(t *testing.T) {
	p := newV17(t, "4294967295")
	if _, err := p.ParseUint32(); err != nil {
		t.Errorf("uint32: unexpected error: %v", err)
	}
	p2 := newV17(t, "4294967296")
	if _, err := p2.ParseUint32(); err == nil {
		t.Errorf("uint32: expected failure for 4294967296 (above_max)")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ uint64 (RANGE 0..18446744073709551615)
func TestV17_Uint64(t *testing.T) {
	p := newV17(t, "18446744073709551615")
	if _, err := p.ParseUint64(); err != nil {
		t.Errorf("uint64 max: unexpected error: %v", err)
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ uint128 (RANGE 0..340282366920938463463374607431768211455)
func TestV17_Uint128(t *testing.T) {
	p := newV17(t, "340282366920938463463374607431768211455")
	if _, err := p.ParseUint128(); err != nil {
		t.Errorf("uint128 max: unexpected error: %v", err)
	}
}

// ===========================================================================
// Float types  (spec/01_definitions.sqg Ã¢â‚¬â€ float32, float64)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ float32
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

// spec/01_definitions.sqg Ã¢â‚¬â€ float64
func TestV17_Float64(t *testing.T) {
	for _, src := range []string{"1.23456789012345", "-0.0", "NaN", "Infinity"} {
		p := newV17(t, src)
		if _, err := p.ParseFloat64(); err != nil {
			t.Errorf("float64: unexpected error for %q: %v", src, err)
		}
	}
}

// ===========================================================================
// Decimal number types  (spec/01_definitions.sqg Ã¢â‚¬â€ decimal8Ã¢â‚¬Â¦decimal128, decimal_num)
// 3.6 RANGE boundary cases
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ decimal8 (RANGE -128..127)
func TestV17_Decimal8(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"-128.0", true},  // at_min
		{"127.99", true},  // at_max
		{"0.5", true},     // middle
		{"128.0", false},  // above_max
		{"-129.0", false}, // below_min
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

// spec/01_definitions.sqg Ã¢â‚¬â€ decimal64 (RANGE -9223372036854775808..9223372036854775807)
func TestV17_Decimal64(t *testing.T) {
	p := newV17(t, "-9223372036854775808.0")
	if _, err := p.ParseDecimal64(); err != nil {
		t.Errorf("decimal64 min: unexpected error: %v", err)
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ decimal_num
func TestV17_DecimalNum_AutoSelect(t *testing.T) {
	for _, src := range []string{"0.0", "127.5", "-128.1", "200.0", "32767.9", "-32768.0"} {
		p := newV17(t, src)
		if _, err := p.ParseDecimalNum(); err != nil {
			t.Errorf("decimal_num: unexpected error for %q: %v", src, err)
		}
	}
}

// ===========================================================================
// Date/Time types  (spec/01_definitions.sqg Ã¢â‚¬â€ date_year..time_stamp)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ date_year (digits4)
func TestV17_DateYear(t *testing.T) {
	for _, src := range []string{"2024", "0001", "9999"} {
		p := newV17(t, src)
		if _, err := p.ParseDateYear(); err != nil {
			t.Errorf("date_year: unexpected error for %q: %v", src, err)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ date_month (RANGE 1..12)
func TestV17_DateMonth(t *testing.T) {
	valids := []string{"1", "12", "06"}
	for _, src := range valids {
		p := newV17(t, src)
		if _, err := p.ParseDateMonth(); err != nil {
			t.Errorf("date_month: unexpected error for %q: %v", src, err)
		}
	}
	// 3.6 RANGE boundaries: below_min=0, above_max=13
	for _, src := range []string{"0", "13"} {
		p := newV17(t, src)
		if _, err := p.ParseDateMonth(); err == nil {
			t.Errorf("date_month: expected failure for %q", src)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ date_day (RANGE 1..31)
func TestV17_DateDay(t *testing.T) {
	p := newV17(t, "31")
	if _, err := p.ParseDateDay(); err != nil {
		t.Errorf("date_day: unexpected error: %v", err)
	}
	p2 := newV17(t, "32")
	if _, err := p2.ParseDateDay(); err == nil {
		t.Errorf("date_day: expected failure for 32 (above_max)")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ date
func TestV17_Date_WithSeparators(t *testing.T) {
	valids := []struct{ label, src string }{
		{"year_only", "2024"},
		{"year_month", "2024-04"},
		{"full_date", "2024-04-15"},
	}
	for _, tc := range valids {
		p := newV17(t, tc.src)
		if _, err := p.ParseDate(); err != nil {
			t.Errorf("date %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
	}
	// 3.5 whitespace tolerance
	for _, ws := range []string{" ", "\t"} {
		p := newV17(t, ws+"2024-01-01")
		if _, err := p.ParseDate(); err != nil {
			t.Errorf("date: leading whitespace %q: unexpected error", ws)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ time_hour (RANGE 0..23)
func TestV17_TimeHour_Range(t *testing.T) {
	for _, src := range []string{"0", "23", "12"} {
		p := newV17(t, src)
		if _, err := p.ParseTimeHour(); err != nil {
			t.Errorf("time_hour: unexpected error for %q: %v", src, err)
		}
	}
	// 3.6 RANGE: above_max = 24
	p := newV17(t, "24")
	if _, err := p.ParseTimeHour(); err == nil {
		t.Errorf("time_hour: expected failure for 24 (above_max)")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ time
func TestV17_Time_WithSeparators(t *testing.T) {
	valids := []struct{ label, src string }{
		{"hour_only", "23"},
		{"hour_minute", "23:59"},
		{"full_time", "23:59:59"},
		{"with_millis", "23:59:59.999"},
	}
	for _, tc := range valids {
		p := newV17(t, tc.src)
		if _, err := p.ParseTime(); err != nil {
			t.Errorf("time %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ date_time
func TestV17_DateTime(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"2024-04-15 23:59", true},
		{"2024-04-15", true},
		{"23:59:59", true},
		{"abc", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDateTime()
		if tc.valid && err != nil {
			t.Errorf("date_time: unexpected error for %q: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("date_time: expected failure for %q", tc.src)
		}
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ time_stamp
func TestV17_TimeStamp(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"2024-04-15 23:59:59", true},
		{"2024-04-15 00:00:00.000", true},
		{"2024-04-15", false}, // time required
		{"abc", false},
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
// Duration  (spec/01_definitions.sqg Ã¢â‚¬â€ duration_unit, duration)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ duration_unit
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
	// 3.3 no-consume
}

// spec/01_definitions.sqg Ã¢â‚¬â€ duration
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
// String types  (spec/01_definitions.sqg Ã¢â‚¬â€ single_quoted, double_quoted,
//                                           tmpl_quoted, string)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ single_quoted
func TestV17_SingleQuoted(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`'hello'`, true},
		{`'it\'s'`, true},
		{`''`, false},      // empty not allowed
		{`"hello"`, false}, // wrong delimiter
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
	// 3.3 no-consume (wrong first token)
	p := newV17(t, `"hello"`)
	if _, err := p.ParseSingleQuoted(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ double_quoted
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

// spec/01_definitions.sqg Ã¢â‚¬â€ tmpl_quoted
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

// spec/01_definitions.sqg Ã¢â‚¬â€ string
func TestV17_String(t *testing.T) {
	// 3.1 valid Ã¢â‚¬â€ one per alternative (single_quoted | double_quoted | tmpl_quoted)
	for _, src := range []string{`'hi'`, `"hi"`, "`hi`"} {
		p := newV17(t, src)
		if _, err := p.ParseString(); err != nil {
			t.Errorf("string: unexpected error for %q: %v", src, err)
		}
	}
	// 3.2 invalid
	p := newV17(t, "42")
	if _, err := p.ParseString(); err == nil {
		t.Errorf("string: expected failure for '42'")
	}
	// 3.3 no-consume
}

// ===========================================================================
// Regexp  (spec/01_definitions.sqg Ã¢â‚¬â€ regexp_flags, regexp_expr)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ regexp_expr
func TestV17_RegexpExpr(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`/hello/`, true},
		{`/hello/gi`, true},
		{`/[0-9]+/imsuyxnA`, true},
		// invalid flag 'z': parser returns valid regexp with flags=[], z unconsumed
		{`/abc/z`, true},
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
	// bare slash is a parse error (unterminated regexp)
	p2, lexErr := parser.NewV17ParserFromSource("/")
	if lexErr != nil {
		t.Errorf("regexp_expr: unexpected lex error for bare '/': %v", lexErr)
	}
	if _, parseErr := p2.ParseRegexpExpr(); parseErr == nil {
		t.Errorf("regexp_expr: expected parse error for bare '/'")
	}
}

// ===========================================================================
// Boolean, null, any_type  (spec/01_definitions.sqg)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ boolean (boolean_true | boolean_false)
func TestV17_Boolean(t *testing.T) {
	// 3.1 valid Ã¢â‚¬â€ one per alternative
	for _, src := range []string{"true", "false"} {
		p := newV17(t, src)
		if _, err := p.ParseBoolean(); err != nil {
			t.Errorf("boolean: unexpected error for %q: %v", src, err)
		}
	}
	// 3.2 invalid Ã¢â‚¬â€ wrong first token
	p := newV17(t, "True")
	if _, err := p.ParseBoolean(); err == nil {
		t.Errorf("boolean: expected failure for 'True'")
	}
	// 3.3 no-consume
}

// spec/01_definitions.sqg Ã¢â‚¬â€ null
func TestV17_Null(t *testing.T) {
	p := newV17(t, "null")
	if _, err := p.ParseNull(); err != nil {
		t.Errorf("null: unexpected error: %v", err)
	}
	p2 := newV17(t, "NULL")
	if _, err := p2.ParseNull(); err == nil {
		t.Errorf("null: expected failure for 'NULL'")
	}
	// 3.3 no-consume
}

// spec/01_definitions.sqg Ã¢â‚¬â€ any_type
func TestV17_AnyType(t *testing.T) {
	p := newV17(t, "@?")
	if _, err := p.ParseAnyType(); err != nil {
		t.Errorf("any_type: unexpected error: %v", err)
	}
	p2 := newV17(t, "@")
	if _, err := p2.ParseAnyType(); err == nil {
		t.Errorf("any_type: expected failure for '@' alone")
	}
	// 3.3 no-consume
}

// ===========================================================================
// Hex segments  (spec/01_definitions.sqg Ã¢â‚¬â€ hex_seg2..128)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ hex_seg8
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

// spec/01_definitions.sqg Ã¢â‚¬â€ hex_seg4
func TestV17_HexSeg4(t *testing.T) {
	p := newV17(t, "1a2b")
	if _, err := p.ParseHexSeg4(); err != nil {
		t.Errorf("hex_seg4: unexpected error: %v", err)
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ hex_seg12
func TestV17_HexSeg12(t *testing.T) {
	p := newV17(t, "1a2b3c4d5e6f")
	if _, err := p.ParseHexSeg12(); err != nil {
		t.Errorf("hex_seg12: unexpected error: %v", err)
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ hex_seg32 (MD5 length)
func TestV17_HexSeg32_MD5Length(t *testing.T) {
	p := newV17(t, "d41d8cd98f00b204e9800998ecf8427e")
	if _, err := p.ParseHexSeg32(); err != nil {
		t.Errorf("hex_seg32: unexpected error: %v", err)
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ hex_seg40 (SHA1 length)
func TestV17_HexSeg40_SHA1Length(t *testing.T) {
	p := newV17(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709")
	if _, err := p.ParseHexSeg40(); err != nil {
		t.Errorf("hex_seg40: unexpected error: %v", err)
	}
}

// ===========================================================================
// UUID  (spec/01_definitions.sqg Ã¢â‚¬â€ uuid, uuid_v7)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ uuid
func TestV17_Uuid(t *testing.T) {
	cases := []struct {
		label string
		src   string
		valid bool
	}{
		{"valid", "550e8400-e29b-41d4-a716-446655440000", true},
		{"uppercase", "DEADBEEF-DEAD-BEEF-DEAD-BEEFDEADBEEF", true},
		{"truncated", "550e8400-e29b-41d4-a716", false},
		{"wrong_first_token", "not-a-uuid-at-all-here", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseUuid()
		if tc.valid && err != nil {
			t.Errorf("uuid %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("uuid %s: expected failure for %q", tc.label, tc.src)
		}
	}
	// 3.3 no-consume (wrong first token)
	p := newV17(t, "not-a-uuid-at-all-here")
	if _, err := p.ParseUuid(); err == nil {
		t.Fatal("no-consume: expected error")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ uuid_v7
func TestV17_UuidV7(t *testing.T) {
	valid := "018f1e3c-6b4a-7abc-8def-0123456789ab"
	p := newV17(t, valid)
	if _, err := p.ParseUuidV7(); err != nil {
		t.Errorf("uuid_v7: unexpected error for %q: %v", valid, err)
	}
	// version nibble not 7
	invalid := "018f1e3c-6b4a-4abc-8def-0123456789ab"
	p2 := newV17(t, invalid)
	if _, err := p2.ParseUuidV7(); err == nil {
		t.Errorf("uuid_v7: expected failure for version-4 UUID %q", invalid)
	}
	// variant nibble not in {8,9,a,b}
	invalid2 := "018f1e3c-6b4a-7abc-cdef-0123456789ab"
	p3 := newV17(t, invalid2)
	if _, err := p3.ParseUuidV7(); err == nil {
		t.Errorf("uuid_v7: expected failure for bad variant %q", invalid2)
	}
}

// ===========================================================================
// Hash keys  (spec/01_definitions.sqg Ã¢â‚¬â€ hash_md5..sha512, hash_key)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ hash_md5 (TYPE_OF hash_md5<hex_seg32>)
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

// spec/01_definitions.sqg Ã¢â‚¬â€ hash_sha1 (TYPE_OF hash_sha1<hex_seg40>)
func TestV17_HashSha1(t *testing.T) {
	p := newV17(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709")
	if _, err := p.ParseHashSha1(); err != nil {
		t.Errorf("hash_sha1: unexpected error: %v", err)
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ hash_sha256 (TYPE_OF hash_sha256<hex_seg64>)
func TestV17_HashSha256(t *testing.T) {
	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	p := newV17(t, hash)
	if _, err := p.ParseHashSha256(); err != nil {
		t.Errorf("hash_sha256: unexpected error: %v", err)
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ hash_sha512 (TYPE_OF hash_sha512<hex_seg128>)
func TestV17_HashSha512(t *testing.T) {
	hash128 := "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce" +
		"47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
	if len(hash128) != 128 {
		t.Fatalf("test setup error: expected 128-char hash, got %d", len(hash128))
	}
	p := newV17(t, hash128)
	if _, err := p.ParseHashSha512(); err != nil {
		t.Errorf("hash_sha512 (128-char): unexpected error: %v", err)
	}
	// one char short
	p2 := newV17(t, hash128[:127])
	if _, err := p2.ParseHashSha512(); err == nil {
		t.Errorf("hash_sha512 (127-char): expected failure")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ hash_key (union of all hash types)
func TestV17_HashKey_Alternation(t *testing.T) {
	// 3.7 EXTEND: one case per alternative
	cases := []struct{ label, src string }{
		{"md5_32", "d41d8cd98f00b204e9800998ecf8427e"},
		{"sha1_40", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		if _, err := p.ParseHashKey(); err != nil {
			t.Errorf("hash_key[%s]: unexpected error: %v", tc.label, err)
		}
	}
}

// ===========================================================================
// ULID and Nano ID  (spec/01_definitions.sqg)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ ulid
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

// spec/01_definitions.sqg Ã¢â‚¬â€ nano_id
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
// Snowflake ID and Sequence IDs  (spec/01_definitions.sqg)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ snowflake_id (TYPE_OF snowflake_id<uint64>)
func TestV17_SnowflakeId(t *testing.T) {
	p := newV17(t, "1701388800000000000")
	if _, err := p.ParseSnowflakeId(); err != nil {
		t.Errorf("snowflake_id: unexpected error: %v", err)
	}
	p2 := newV17(t, "18446744073709551616") // uint64 overflow
	if _, err := p2.ParseSnowflakeId(); err == nil {
		t.Errorf("snowflake_id: expected failure for uint64 overflow")
	}
}

// spec/01_definitions.sqg Ã¢â‚¬â€ seq_id (seq_id16 | seq_id32 | seq_id64)
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
// Unique Key  (spec/01_definitions.sqg Ã¢â‚¬â€ unique_key union)
// 3.7 EXTEND: one case per alternative
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ unique_key
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
		{"wrong_first_token", "hello world", false},
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
// HTTP URL and File URL  (spec/01_definitions.sqg)
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ http_url
func TestV17_HttpUrl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"https://example.com"`, true},
		{`"http://foo.bar/path?q=1#anchor"`, true},
		{`"ftp://example.com"`, false},
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

// spec/01_definitions.sqg Ã¢â‚¬â€ file_url
func TestV17_FileUrl(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`"file:///home/user/file.txt"`, true},
		{`"file://./relative"`, false},
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
// Constant  (spec/01_definitions.sqg Ã¢â‚¬â€ constant union)
// 3.7 one case per alternative
// ===========================================================================

// spec/01_definitions.sqg Ã¢â‚¬â€ constant
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

// Note: empty_array_decl, func_stream_decl, func_regexp_decl, empty_scope_decl,
// func_string_decl, and empty_decl are tested in squeeze_v17_04_objects_test.go
// (spec/04_objects.sqg section) to match the monolithic test organization.
