// Package squeeze_v1_test contains a specification-driven test suite for the
// Squeeze V1 grammar as defined in spec/squeeze_v1.ebnf.txt.
//
// Each test group corresponds to a grammar rule (or group of related rules).
// Tests validate that terminal regex patterns and structural examples match
// the expected valid and invalid inputs documented in the EBNF.
package squeeze_v1_test

import (
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper types and utilities
// ---------------------------------------------------------------------------

type grammarCase struct {
	input   string
	valid   bool
	comment string
}

// matchFull returns true if the pattern matches the entire input string.
func matchFull(re *regexp.Regexp, s string) bool {
	loc := re.FindStringIndex(s)
	return loc != nil && loc[0] == 0 && loc[1] == len(s)
}

// runCases runs each grammarCase against re using full-string matching.
func runCases(t *testing.T, ruleName string, re *regexp.Regexp, cases []grammarCase) {
	t.Helper()
	for _, tc := range cases {
		name := tc.comment
		if name == "" {
			name = tc.input
		}
		t.Run(ruleName+"/"+name, func(t *testing.T) {
			got := matchFull(re, tc.input)
			if got != tc.valid {
				if tc.valid {
					t.Errorf("rule %s: expected %q to match but it did not", ruleName, tc.input)
				} else {
					t.Errorf("rule %s: expected %q NOT to match but it did", ruleName, tc.input)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Compiled terminal patterns from the V1 grammar
// ---------------------------------------------------------------------------

var (
	// NL = /([ \t]*[\r\n]+)+/
	reNL = regexp.MustCompile(`^([ \t]*[\r\n]+)+$`)

	// EOL = NL | ";"   — for testing, just the semicolon literal; NL covered separately
	reEOLSemi = regexp.MustCompile(`^;$`)

	// digits = /[0-9]+/
	reDigits = regexp.MustCompile(`^[0-9]+$`)

	// integer = [ "+" | "-" ] digits
	reInteger = regexp.MustCompile(`^[+\-]?[0-9]+$`)

	// decimal = [ "+" | "-" ] digits "." digits
	reDecimal = regexp.MustCompile(`^[+\-]?[0-9]+\.[0-9]+$`)

	// numeric_const = integer | decimal
	// (decimal must be tried first to avoid integer consuming digits before the ".")
	reNumericConst = regexp.MustCompile(`^[+\-]?[0-9]+(\.[0-9]+)?$`)

	// single_quoted = "'" /(?<value>(\\'|[^'])+)/ "'"
	reSingleQuoted = regexp.MustCompile(`^'((\\'|[^'])+)'$`)

	// double_quoted = "\"" /(?<value>(\\"|[^"])+)/ "\""
	reDoubleQuoted = regexp.MustCompile(`^"(\\"|[^"])*"$`)

	// tmpl_quoted = "`" /[^`]*/ "`"
	reTmplQuoted = regexp.MustCompile("^`[^`]*`$")

	// regexp_flags = "g" | "i"
	reRegexpFlags = regexp.MustCompile(`^[gi]$`)

	// boolean = "true" | "false"
	reBoolean = regexp.MustCompile(`^(true|false)$`)

	// ident_name = /(?<value>[\p{L}][\p{L}0-9_][\p{L}][\p{L}0-9_ ]*)/
	// Minimum 3 chars: letter, letter/digit/underscore, letter; then optional letter/digit/underscore/space
	reIdentName = regexp.MustCompile(`^[\p{L}][\p{L}0-9_][\p{L}][\p{L}0-9_ ]*$`)

	// ident_prefix = ( "../" { "../" } ) | "./"
	reIdentPrefix = regexp.MustCompile(`^(\.\./)+|^\.\/$`)

	// numeric_oper = "+" | "-" | "*" | "**" | "/" | "%"
	reNumericOper = regexp.MustCompile(`^(\*\*|[+\-*/%])$`)

	// inline_incr = "++" | "--"
	reInlineIncr = regexp.MustCompile(`^(\+\+|--)$`)

	// compare_oper = "!=" | "=" | ">" | ">=" | "<" | "<="
	reCompareOper = regexp.MustCompile(`^(!=|>=|<=|=|>|<)$`)

	// not_oper = "!"
	reNotOper = regexp.MustCompile(`^!$`)

	// logic_oper = "&" | "|" | "^"
	reLogicOper = regexp.MustCompile(`^[&|^]$`)

	// incr_assign_immutable = "+:" | "-:" | "*:" | "/:"
	reIncrAssignImmutable = regexp.MustCompile(`^[+\-*/]:$`)

	// incr_assign_mutable = "+~" | "-~" | "*~" | "/~"
	reIncrAssignMutable = regexp.MustCompile(`^[+\-*/]~$`)

	// equal_assign = "~" | ":"
	reEqualAssign = regexp.MustCompile(`^[~:]$`)

	// assign_oper = incr_assign_immutable | incr_assign_mutable | equal_assign
	reAssignOper = regexp.MustCompile(`^([+\-*/][~:]|[~:])$`)

	// range = digits ".." ( digits | "m" | "M" | "many" | "Many" )
	reRange = regexp.MustCompile(`^[0-9]+\.\.(([0-9]+)|m|M|many|Many)$`)

	// string_oper = "+"  (V2)
	reStringOper = regexp.MustCompile(`^\+$`)
)

// ---------------------------------------------------------------------------
// Tests: NL — newline token
// ---------------------------------------------------------------------------

func TestNL(t *testing.T) {
	cases := []grammarCase{
		{"\n", true, "bare LF"},
		{"\r\n", true, "CRLF"},
		{"\t  \n", true, "tabs and spaces before LF"},
		{"\n\n", true, "consecutive newlines"},
		{"   \n\t\n", true, "mixed whitespace then two LFs"},
		{"", false, "empty string"},
		{" ", false, "space only, no newline"},
		{"abc\n", false, "leading text"},
	}
	runCases(t, "NL", reNL, cases)
}

// ---------------------------------------------------------------------------
// Tests: EOL — end of line (semicolon variant)
// ---------------------------------------------------------------------------

func TestEOL_Semicolon(t *testing.T) {
	cases := []grammarCase{
		{";", true, "semicolon"},
		{";;", false, "double semicolon"},
		{"", false, "empty"},
		{" ;", false, "leading space"},
	}
	runCases(t, "EOL_semi", reEOLSemi, cases)
}

// ---------------------------------------------------------------------------
// Tests: digits
// ---------------------------------------------------------------------------

func TestDigits(t *testing.T) {
	cases := []grammarCase{
		{"0", true, "single zero"},
		{"42", true, "two digits"},
		{"1000000", true, "large number"},
		{"007", true, "leading zeros"},
		{"", false, "empty string"},
		{"a1", false, "leading letter"},
		{"1a", false, "trailing letter"},
		{"-1", false, "negative sign not part of digits rule"},
	}
	runCases(t, "digits", reDigits, cases)
}

// ---------------------------------------------------------------------------
// Tests: integer
// ---------------------------------------------------------------------------

func TestInteger(t *testing.T) {
	cases := []grammarCase{
		{"0", true, "zero"},
		{"42", true, "positive no sign"},
		{"+42", true, "explicit positive"},
		{"-42", true, "negative"},
		{"-0", true, "negative zero"},
		{"", false, "empty"},
		{"+-1", false, "double sign"},
		{"1.0", false, "decimal not integer"},
		{"abc", false, "letters"},
	}
	runCases(t, "integer", reInteger, cases)
}

// ---------------------------------------------------------------------------
// Tests: decimal
// ---------------------------------------------------------------------------

func TestDecimal(t *testing.T) {
	cases := []grammarCase{
		{"3.14", true, "pi"},
		{"+1.0", true, "explicit positive"},
		{"-0.5", true, "negative decimal"},
		{"0.0", true, "zero decimal"},
		{"100.001", true, "multi-digit both sides"},
		{"3", false, "integer, no dot"},
		{".5", false, "missing integer part"},
		{"3.", false, "missing fractional part"},
		{"3.1.4", false, "two dots"},
		{"", false, "empty"},
	}
	runCases(t, "decimal", reDecimal, cases)
}

// ---------------------------------------------------------------------------
// Tests: numeric_const (integer | decimal)
// ---------------------------------------------------------------------------

func TestNumericConst(t *testing.T) {
	cases := []grammarCase{
		{"0", true, "zero integer"},
		{"42", true, "positive integer"},
		{"-7", true, "negative integer"},
		{"3.14", true, "decimal"},
		{"-3.14", true, "negative decimal"},
		{"+0.001", true, "small positive decimal"},
		{"", false, "empty"},
		{"abc", false, "letters"},
		{"3.1.4", false, "multiple dots"},
	}
	runCases(t, "numeric_const", reNumericConst, cases)
}

// ---------------------------------------------------------------------------
// Tests: single_quoted string
// ---------------------------------------------------------------------------

func TestSingleQuoted(t *testing.T) {
	cases := []grammarCase{
		{"'hello'", true, "simple word"},
		{"'it\\'s fine'", true, "escaped single quote inside"},
		{"'multi word string'", true, "spaces inside"},
		{"''", false, "empty — requires at least one char"},
		{`"hello"`, false, "double quotes not single"},
		{"'unterminated", false, "no closing quote"},
		{"hello", false, "no quotes at all"},
	}
	runCases(t, "single_quoted", reSingleQuoted, cases)
}

// ---------------------------------------------------------------------------
// Tests: double_quoted string
// ---------------------------------------------------------------------------

func TestDoubleQuoted(t *testing.T) {
	cases := []grammarCase{
		{`"hello"`, true, "simple word"},
		{`"say \"hi\""`, true, "escaped double quote inside"},
		{`"multi word"`, true, "spaces inside"},
		{`""`, true, "empty double-quoted string"},
		{"'hello'", false, "single quotes"},
		{`"unterminated`, false, "no closing quote"},
	}
	runCases(t, "double_quoted", reDoubleQuoted, cases)
}

// ---------------------------------------------------------------------------
// Tests: tmpl_quoted (template literal)
// ---------------------------------------------------------------------------

func TestTmplQuoted(t *testing.T) {
	cases := []grammarCase{
		{"``", true, "empty template"},
		{"`hello world`", true, "simple content"},
		{"`line1\nline2`", true, "newline inside template"},
		{"`contains 'single' and \"double\"`", true, "mixed quotes inside"},
		{"'not a template'", false, "single quotes"},
		{"`unclosed", false, "no closing backtick"},
	}
	runCases(t, "tmpl_quoted", reTmplQuoted, cases)
}

// ---------------------------------------------------------------------------
// Tests: regexp_flags
// ---------------------------------------------------------------------------

func TestRegexpFlags(t *testing.T) {
	cases := []grammarCase{
		{"g", true, "global flag"},
		{"i", true, "case-insensitive flag"},
		{"gi", false, "combined — not a single flag token"},
		{"m", false, "unsupported flag"},
		{"", false, "empty"},
		{"G", false, "uppercase not valid"},
	}
	runCases(t, "regexp_flags", reRegexpFlags, cases)
}

// ---------------------------------------------------------------------------
// Tests: boolean
// ---------------------------------------------------------------------------

func TestBoolean(t *testing.T) {
	cases := []grammarCase{
		{"true", true, "true"},
		{"false", true, "false"},
		{"True", false, "uppercase T"},
		{"FALSE", false, "all caps"},
		{"1", false, "numeric"},
		{"", false, "empty"},
		{"truee", false, "extra char"},
	}
	runCases(t, "boolean", reBoolean, cases)
}

// ---------------------------------------------------------------------------
// Tests: ident_name
// ---------------------------------------------------------------------------

func TestIdentName(t *testing.T) {
	cases := []grammarCase{
		{"abc", true, "three letters minimum"},
		{"a1b", true, "letter digit letter"},
		{"a_b", true, "letter underscore letter"},
		{"myVar", true, "camel-case style"},
		{"myVar name", true, "space allowed after third char"},
		{"résumé", true, "unicode letters"},
		{"ab", false, "too short — only 2 chars"},
		{"1ab", false, "starts with digit"},
		{"_ab", false, "starts with underscore"},
		{"a b", false, "space at second position not allowed"},
		{"a1 ", false, "third char must be letter, but space is not"},
		{"", false, "empty"},
	}
	runCases(t, "ident_name", reIdentName, cases)
}

// ---------------------------------------------------------------------------
// Tests: ident_prefix
// ---------------------------------------------------------------------------

func TestIdentPrefix(t *testing.T) {
	cases := []grammarCase{
		{"../", true, "one level up"},
		{"../../", true, "two levels up"},
		{"../../../", true, "three levels up"},
		{"./", true, "current directory"},
		{"/", false, "absolute path not valid prefix"},
		{"", false, "empty"},
		{"..", false, "missing trailing slash"},
		{"./.", false, "dot after current dir"},
	}
	runCases(t, "ident_prefix", reIdentPrefix, cases)
}

// ---------------------------------------------------------------------------
// Tests: numeric_oper
// ---------------------------------------------------------------------------

func TestNumericOper(t *testing.T) {
	cases := []grammarCase{
		{"+", true, "addition"},
		{"-", true, "subtraction"},
		{"*", true, "multiplication"},
		{"**", true, "power"},
		{"/", true, "division"},
		{"%", true, "modulo"},
		{"//", false, "double slash not valid"},
		{"++", false, "increment not a numeric_oper"},
		{"", false, "empty"},
		{"^", false, "XOR is logic_oper not numeric"},
	}
	runCases(t, "numeric_oper", reNumericOper, cases)
}

// ---------------------------------------------------------------------------
// Tests: inline_incr
// ---------------------------------------------------------------------------

func TestInlineIncr(t *testing.T) {
	cases := []grammarCase{
		{"++", true, "increment"},
		{"--", true, "decrement"},
		{"+", false, "single plus"},
		{"-", false, "single minus"},
		{"+-", false, "mixed"},
		{"", false, "empty"},
	}
	runCases(t, "inline_incr", reInlineIncr, cases)
}

// ---------------------------------------------------------------------------
// Tests: compare_oper
// ---------------------------------------------------------------------------

func TestCompareOper(t *testing.T) {
	cases := []grammarCase{
		{"=", true, "equals"},
		{"!=", true, "not equals"},
		{">", true, "greater than"},
		{">=", true, "greater or equal"},
		{"<", true, "less than"},
		{"<=", true, "less or equal"},
		{"==", false, "double equals not defined"},
		{"<>", false, "not defined"},
		{"=~", false, "regexp match is separate"},
		{"", false, "empty"},
	}
	runCases(t, "compare_oper", reCompareOper, cases)
}

// ---------------------------------------------------------------------------
// Tests: not_oper
// ---------------------------------------------------------------------------

func TestNotOper(t *testing.T) {
	cases := []grammarCase{
		{"!", true, "negation operator"},
		{"!!", false, "double not"},
		{"!=", false, "not-equal is compare_oper"},
		{"", false, "empty"},
	}
	runCases(t, "not_oper", reNotOper, cases)
}

// ---------------------------------------------------------------------------
// Tests: logic_oper
// ---------------------------------------------------------------------------

func TestLogicOper(t *testing.T) {
	cases := []grammarCase{
		{"&", true, "AND"},
		{"|", true, "OR"},
		{"^", true, "XOR"},
		{"&&", false, "double AND not defined"},
		{"||", false, "double OR not defined"},
		{"!", false, "not is not_oper"},
		{"", false, "empty"},
	}
	runCases(t, "logic_oper", reLogicOper, cases)
}

// ---------------------------------------------------------------------------
// Tests: assign_oper (all assignment operators)
// ---------------------------------------------------------------------------

func TestAssignOper(t *testing.T) {
	immutable := []string{"+:", "-:", "*:", "/:"}
	mutable := []string{"+~", "-~", "*~", "/~"}
	equal := []string{"~", ":"}

	for _, op := range append(append(immutable, mutable...), equal...) {
		op := op
		t.Run("assign_oper/valid/"+op, func(t *testing.T) {
			if !matchFull(reAssignOper, op) {
				t.Errorf("expected %q to match assign_oper but it did not", op)
			}
		})
	}

	invalid := []string{"=", "::", "~~", "+=", "", "->", "<-"}
	for _, op := range invalid {
		op := op
		t.Run("assign_oper/invalid/"+op, func(t *testing.T) {
			if matchFull(reAssignOper, op) {
				t.Errorf("expected %q NOT to match assign_oper but it did", op)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: range
// ---------------------------------------------------------------------------

func TestRange(t *testing.T) {
	cases := []grammarCase{
		{"0..10", true, "zero to ten"},
		{"1..128", true, "1 to 128"},
		{"5..m", true, "5 to many (m)"},
		{"5..M", true, "5 to many (M)"},
		{"5..many", true, "5 to many (many)"},
		{"5..Many", true, "5 to many (Many)"},
		{"0..0", true, "zero range"},
		{"10..1", true, "upper bound less than lower — grammar allows, semantic check separate"},
		{"..10", false, "missing lower bound"},
		{"5.", false, "single dot"},
		{"5...10", false, "triple dot"},
		{"5..x", false, "invalid upper bound token"},
		{"", false, "empty"},
		{"-1..10", false, "negative lower bound not in grammar"},
	}
	runCases(t, "range", reRange, cases)
}

// ---------------------------------------------------------------------------
// Tests: incr_assign_immutable
// ---------------------------------------------------------------------------

func TestIncrAssignImmutable(t *testing.T) {
	cases := []grammarCase{
		{"+:", true, "add assign immutable"},
		{"-:", true, "sub assign immutable"},
		{"*:", true, "mul assign immutable"},
		{"/:", true, "div assign immutable"},
		{"+~", false, "mutable variant"},
		{":", false, "plain colon"},
		{"", false, "empty"},
	}
	runCases(t, "incr_assign_immutable", reIncrAssignImmutable, cases)
}

// ---------------------------------------------------------------------------
// Tests: incr_assign_mutable
// ---------------------------------------------------------------------------

func TestIncrAssignMutable(t *testing.T) {
	cases := []grammarCase{
		{"+~", true, "add assign mutable"},
		{"-~", true, "sub assign mutable"},
		{"*~", true, "mul assign mutable"},
		{"/~", true, "div assign mutable"},
		{"+:", false, "immutable variant"},
		{"~", false, "plain tilde"},
		{"", false, "empty"},
	}
	runCases(t, "incr_assign_mutable", reIncrAssignMutable, cases)
}

// ---------------------------------------------------------------------------
// Tests: string_oper  (V2)
//   string_oper = "+"
// ---------------------------------------------------------------------------

func TestStringOper(t *testing.T) {
	cases := []grammarCase{
		{"+", true, "concat operator"},
		{"-", false, "minus is numeric_oper not string_oper"},
		{"++", false, "double plus not valid"},
		{"*", false, "multiplication not string_oper"},
		{"", false, "empty"},
	}
	runCases(t, "string_oper", reStringOper, cases)
}

// ---------------------------------------------------------------------------
// Tests: string_concat and string_expr structural check  (V2)
//   string_oper   = "+"
//   string_concat = string string_oper string { string_oper string }
//   string_expr   = string_concat
// ---------------------------------------------------------------------------

func TestStringConcat_Structural(t *testing.T) {
	// Pattern: one or more quoted strings joined by "+"
	// We use a lightweight check: split by " + " and verify each part is a quoted string
	isQuoted := func(s string) bool {
		return reSingleQuoted.MatchString(s) || reDoubleQuoted.MatchString(s) || reTmplQuoted.MatchString(s)
	}

	isStringConcat := func(s string) bool {
		parts := strings.Split(s, " + ")
		if len(parts) < 2 {
			return false
		}
		for _, p := range parts {
			if !isQuoted(strings.TrimSpace(p)) {
				return false
			}
		}
		return true
	}

	cases := []struct {
		input   string
		valid   bool
		comment string
	}{
		{`'hello' + 'world'`, true, "two single-quoted"},
		{`"foo" + "bar"`, true, "two double-quoted"},
		{"'a' + `b`", true, "single and template"},
		{"`x` + `y` + `z`", true, "three template strings"},
		{`'alone'`, false, "single string — need at least two for concat"},
		{`'a' + 42`, false, "number is not a string"},
		{`'a' + `, false, "trailing plus"},
		{"", false, "empty"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("string_concat/"+tc.comment, func(t *testing.T) {
			got := isStringConcat(tc.input)
			if got != tc.valid {
				if tc.valid {
					t.Errorf("expected %q to be valid string_concat but check failed", tc.input)
				} else {
					t.Errorf("expected %q to be INVALID string_concat but check passed", tc.input)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: string_grouping structural check  (V2)
//   string_grouping = "(" ( string_expr | string_grouping ) { string_oper ( string_expr | string_grouping ) } ")"
// ---------------------------------------------------------------------------

// splitByStringOper splits s on " + " tokens at depth 0 (respects parentheses nesting).
func splitByStringOper(s string) []string {
	var parts []string
	depth := 0
	start := 0
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '+':
			if depth == 0 && i >= 1 && runes[i-1] == ' ' && i+1 < len(runes) && runes[i+1] == ' ' {
				parts = append(parts, strings.TrimSpace(string(runes[start:i-1])))
				start = i + 2
			}
		}
	}
	parts = append(parts, strings.TrimSpace(string(runes[start:])))
	return parts
}

// isStringGrouping returns true if s is a valid string_grouping.
func isStringGrouping(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return false
	}
	return isStringContent(inner)
}

// isStringContent returns true if s is valid string_expr or string_grouping content.
func isStringContent(s string) bool {
	isQuotedAtom := func(p string) bool {
		p = strings.TrimSpace(p)
		return reSingleQuoted.MatchString(p) || reDoubleQuoted.MatchString(p) || reTmplQuoted.MatchString(p)
	}
	parts := splitByStringOper(s)
	if len(parts) == 1 {
		// single term must be a nested grouping
		return isStringGrouping(strings.TrimSpace(parts[0]))
	}
	// multiple terms: each must be a quoted string or a nested grouping
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !isQuotedAtom(p) && !isStringGrouping(p) {
			return false
		}
	}
	return true
}

func TestStringGrouping_Structural(t *testing.T) {
	cases := []struct {
		input   string
		valid   bool
		comment string
	}{
		{"('hello' + 'world')", true, "simple grouping with two strings"},
		{"('a' + 'b' + 'c')", true, "grouping with three strings"},
		{`("foo" + "bar")`, true, "double-quoted strings in grouping"},
		{"(('a' + 'b') + 'c')", true, "nested grouping on left"},
		{"('a' + ('b' + 'c'))", true, "nested grouping on right"},
		{"(('x' + 'y'))", true, "grouping containing only a sub-grouping"},
		{"('hello')", false, "single string — not a valid string_expr"},
		{"()", false, "empty grouping"},
		{"'a' + 'b'", false, "no outer parens — not a grouping"},
		{"('a' + 42)", false, "number is not a string atom"},
		{"(+)", false, "operator alone"},
		{"", false, "empty"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("string_grouping/"+tc.comment, func(t *testing.T) {
			got := isStringGrouping(tc.input)
			if got != tc.valid {
				if tc.valid {
					t.Errorf("expected %q to be valid string_grouping but check failed", tc.input)
				} else {
					t.Errorf("expected %q to be INVALID string_grouping but check passed", tc.input)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: ident_dotted structural check
//   ident_dotted = ident_name { "." ident_name }
// ---------------------------------------------------------------------------

func TestIdentDotted_Structural(t *testing.T) {
	isIdentDotted := func(s string) bool {
		parts := strings.Split(s, ".")
		for _, p := range parts {
			if !reIdentName.MatchString(p) {
				return false
			}
		}
		return len(parts) >= 1
	}

	cases := []struct {
		input   string
		valid   bool
		comment string
	}{
		{"abc", true, "single ident_name"},
		{"myObj.myProp", true, "two-level dotted"},
		{"foo.bar.baz", true, "three-level dotted"},
		{"résumé.prop", true, "unicode ident dotted"},
		{"ab.cd", false, "both parts too short (need ≥3 chars)"},
		{"abc.", false, "trailing dot, empty second part"},
		{".abc", false, "leading dot"},
		{"", false, "empty"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("ident_dotted/"+tc.comment, func(t *testing.T) {
			got := isIdentDotted(tc.input)
			if got != tc.valid {
				if tc.valid {
					t.Errorf("expected %q to be valid ident_dotted but check failed", tc.input)
				} else {
					t.Errorf("expected %q to be INVALID ident_dotted but check passed", tc.input)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: ident_ref structural check
//   ident_ref = [ ident_prefix ] ident_dotted
// ---------------------------------------------------------------------------

func TestIdentRef_Structural(t *testing.T) {
	isIdentRef := func(s string) bool {
		// Strip optional prefix
		stripped := s
		for _, prefix := range []string{"../../", "../", "./"} {
			if strings.HasPrefix(s, prefix) {
				// consume all leading "../" chains
				tmp := s
				for strings.HasPrefix(tmp, "../") {
					tmp = strings.TrimPrefix(tmp, "../")
				}
				if strings.HasPrefix(s, "./") {
					tmp = strings.TrimPrefix(s, "./")
				}
				stripped = tmp
				break
			}
		}
		// What remains must be a valid ident_dotted
		parts := strings.Split(stripped, ".")
		for _, p := range parts {
			if !reIdentName.MatchString(p) {
				return false
			}
		}
		return len(parts) >= 1 && stripped != ""
	}

	cases := []struct {
		input   string
		valid   bool
		comment string
	}{
		{"abc", true, "plain ident"},
		{"myObj.myProp", true, "dotted no prefix"},
		{"../abc", true, "one level up"},
		{"../../abc", true, "two levels up"},
		{"./abc", true, "current dir"},
		{"../myObj.prop", true, "prefix with dotted"},
		{".abc", false, "leading dot not a prefix"},
		{"../", false, "prefix only, no ident"},
		{"", false, "empty"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("ident_ref/"+tc.comment, func(t *testing.T) {
			got := isIdentRef(tc.input)
			if got != tc.valid {
				if tc.valid {
					t.Errorf("expected %q to be valid ident_ref but check failed", tc.input)
				} else {
					t.Errorf("expected %q to be INVALID ident_ref but check passed", tc.input)
				}
			}
		})
	}
}
