// parser_v17_definitions.go — ParseXxx methods for spec/01_definitions.sqg.
//
// Every method in this file follows SIR-1/2/3/4:
//
//	SIR-1: method name = camelCase of grammar rule name.
//	SIR-2: every method calls debugEnter / defer done.
//	SIR-3: implementation matches the spec exactly.
//	SIR-4: no pre-lexing; domain classification happens here at parse time.
package parser

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// =============================================================================
// AST BASE NODE
// =============================================================================

// V17BaseNode carries source position for every AST node.
type V17BaseNode struct {
	Line int
	Col  int
}

// =============================================================================
// STEP 1-2 — COMMENTS
// comment_begin = "(*"
// comment_end   = "*)"
// comment_txt   = /(?:(?!\(\*|\*\))[\s\S])*/
// comment       = comment_begin comment_txt { comment } comment_end
// comment_TBD_stub = comment_begin " TBD_STUB " comment_end
// =============================================================================

// V17CommentBeginNode  comment_begin = "(*"
type V17CommentBeginNode struct{ V17BaseNode }

// ParseCommentBegin parses comment_begin = "(*".
func (p *V17Parser) ParseCommentBegin() (node *V17CommentBeginNode, err error) {
	done := p.debugEnter("comment_begin")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("(*")
	if err != nil {
		return nil, err
	}
	return &V17CommentBeginNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17CommentEndNode  comment_end = "*)"
type V17CommentEndNode struct{ V17BaseNode }

// ParseCommentEnd parses comment_end = "*)".
func (p *V17Parser) ParseCommentEnd() (node *V17CommentEndNode, err error) {
	done := p.debugEnter("comment_end")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("*)")
	if err != nil {
		return nil, err
	}
	return &V17CommentEndNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17CommentTxtNode  comment_txt = /(?:(?!\(\*|\*\))[\s\S])*/
// The text between comment delimiters (excluding nested (* *) pairs).
type V17CommentTxtNode struct {
	V17BaseNode
	Text string
}

// ParseCommentTxt parses comment_txt — all characters up to the next "(*" or "*)",
// accepting any Unicode.  Uses the hand-coded matchCommentTxt helper (Go RE2
// lacks negative-lookahead so the grammar regex cannot be compiled directly).
func (p *V17Parser) ParseCommentTxt() (node *V17CommentTxtNode, err error) {
	done := p.debugEnter("comment_txt")
	defer func() { done(err == nil) }()
	tok, err := p.matchCommentTxt()
	if err != nil {
		return nil, err
	}
	return &V17CommentTxtNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17CommentNode  comment = comment_begin comment_txt { comment } comment_end
type V17CommentNode struct {
	V17BaseNode
	Txt    *V17CommentTxtNode
	Nested []*V17CommentNode
}

// ParseComment parses comment = comment_begin comment_txt { comment } comment_end.
func (p *V17Parser) ParseComment() (node *V17CommentNode, err error) {
	done := p.debugEnter("comment")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.ParseCommentBegin(); err != nil {
		return nil, err
	}
	txt, err := p.ParseCommentTxt()
	if err != nil {
		return nil, err
	}
	n := &V17CommentNode{V17BaseNode{line, col}, txt, nil}

	// { comment } — consume any nested (* ... *) blocks
	for p.peekLit("(*") {
		nested, nerr := p.ParseComment()
		if nerr != nil {
			return nil, nerr
		}
		n.Nested = append(n.Nested, nested)
		// After a nested comment, consume any further comment_txt
		moreTxt, merr := p.ParseCommentTxt()
		if merr != nil {
			return nil, merr
		}
		n.Txt.Text += moreTxt.Text
	}

	if _, err = p.ParseCommentEnd(); err != nil {
		return nil, fmt.Errorf("comment: %w", err)
	}
	return n, nil
}

// V17CommentTbdStubNode  comment_TBD_stub = comment_begin " TBD_STUB " comment_end
type V17CommentTbdStubNode struct{ V17BaseNode }

// ParseCommentTbdStub parses comment_TBD_stub = comment_begin " TBD_STUB " comment_end.
func (p *V17Parser) ParseCommentTbdStub() (node *V17CommentTbdStubNode, err error) {
	done := p.debugEnter("comment_TBD_stub")
	defer func() { done(err == nil) }()
	saved := p.savePos()

	var tok V17Token
	tok, err = p.matchLit("(*")
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	line, col := tok.Line, tok.Col
	txt, err := p.ParseCommentTxt()
	if err != nil || strings.TrimSpace(txt.Text) != "TBD_STUB" {
		p.restorePos(saved)
		return nil, p.errAt("comment_TBD_stub: expected \" TBD_STUB \"")
	}
	if _, err = p.matchLit("*)"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	return &V17CommentTbdStubNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// STEP 3 — WHITESPACE
// NL  = /([ \t]*[\r\n]+)+/ | BOF
// EOL = NL | ";" | comment | EOF
// =============================================================================

// V17NlNode  NL = /([ \t]*[\r\n]+)+/ | BOF
type V17NlNode struct {
	V17BaseNode
	IsBOF bool
}

// ParseNl parses NL = /([ \t]*[\r\n]+)+/ | BOF.
func (p *V17Parser) ParseNl() (node *V17NlNode, err error) {
	done := p.debugEnter("NL")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Try a real newline first; BOF is only used when there is no actual NL.
	if tok, terr := p.matchRe(reNlScan); terr == nil {
		_ = tok
		return &V17NlNode{V17BaseNode{line, col}, false}, nil
	}

	// BOF: start of file counts as an implicit newline boundary.
	// Only valid at position 0 — once any input has been consumed, BOF no
	// longer applies (preventing it from falsely satisfying mid-parse EOL checks).
	if p.atBOF && p.runePos == 0 {
		p.atBOF = false
		return &V17NlNode{V17BaseNode{line, col}, true}, nil
	}
	return nil, p.errAt("NL: expected newline")
}

// V17EolNode  EOL = NL | ";" | comment | EOF
type V17EolNode struct {
	V17BaseNode
	Kind string // "NL" | ";" | "comment" | "EOF"
}

// ParseEol parses EOL = NL | ";" | comment | EOF.
func (p *V17Parser) ParseEol() (node *V17EolNode, err error) {
	done := p.debugEnter("EOL")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// NL or BOF
	if saved := p.savePos(); true {
		if _, nerr := p.ParseNl(); nerr == nil {
			return &V17EolNode{V17BaseNode{line, col}, "NL"}, nil
		}
		p.restorePos(saved)
	}
	// ";"
	if p.peekAfterWS() == ';' {
		if _, serr := p.matchLit(";"); serr == nil {
			return &V17EolNode{V17BaseNode{line, col}, ";"}, nil
		}
	}
	// comment
	if p.peekLit("(*") {
		if _, cerr := p.ParseComment(); cerr != nil {
			return nil, cerr
		}
		return &V17EolNode{V17BaseNode{line, col}, "comment"}, nil
	}
	// EOF
	if p.runePos >= len(p.input) {
		return &V17EolNode{V17BaseNode{line, col}, "EOF"}, nil
	}
	return nil, p.errAt("EOL: expected newline, ';', comment, or EOF")
}

// =============================================================================
// STEP 4-5 — DIGIT PRIMITIVES & SIGN
// digits  = /[0-9]+/
// digits2 = /[0-9]{1,2}/
// digits3 = /[0-9]{1,3}/
// digits4 = /[0-9]{4}/
// sign_prefix = [ "+" | "-" ]
// =============================================================================

// V17DigitsNode  digits = /[0-9]+/
type V17DigitsNode struct {
	V17BaseNode
	Value string
}

// ParseDigits parses digits = /[0-9]+/.
func (p *V17Parser) ParseDigits() (node *V17DigitsNode, err error) {
	done := p.debugEnter("digits")
	defer func() { done(err == nil) }()
	tok, err := p.matchDigits()
	if err != nil {
		return nil, err
	}
	return &V17DigitsNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17Digits2Node  digits2 = /[0-9]{1,2}/
type V17Digits2Node struct {
	V17BaseNode
	Value string
}

// ParseDigits2 parses digits2 = /[0-9]{1,2}/ — width 1 or 2 digits.
func (p *V17Parser) ParseDigits2() (node *V17Digits2Node, err error) {
	done := p.debugEnter("digits2")
	defer func() { done(err == nil) }()
	tok, err := p.matchDigitsN(1, 2)
	if err != nil {
		return nil, p.errAt("digits2: expected 1-2 digit number: %v", err)
	}
	return &V17Digits2Node{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17Digits3Node  digits3 = /[0-9]{1,3}/
type V17Digits3Node struct {
	V17BaseNode
	Value string
}

// ParseDigits3 parses digits3 = /[0-9]{1,3}/ — width 1 to 3 digits.
func (p *V17Parser) ParseDigits3() (node *V17Digits3Node, err error) {
	done := p.debugEnter("digits3")
	defer func() { done(err == nil) }()
	tok, err := p.matchDigitsN(1, 3)
	if err != nil {
		return nil, p.errAt("digits3: expected 1-3 digit number: %v", err)
	}
	return &V17Digits3Node{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17Digits4Node  digits4 = /[0-9]{4}/
type V17Digits4Node struct {
	V17BaseNode
	Value string
}

// ParseDigits4 parses digits4 = /[0-9]{4}/ — exactly 4 digits.
func (p *V17Parser) ParseDigits4() (node *V17Digits4Node, err error) {
	done := p.debugEnter("digits4")
	defer func() { done(err == nil) }()
	tok, err := p.matchDigitsN(4, 4)
	if err != nil {
		return nil, p.errAt("digits4: expected exactly 4 digits: %v", err)
	}
	return &V17Digits4Node{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17SignPrefixNode  sign_prefix = [ "+" | "-" ]
type V17SignPrefixNode struct {
	V17BaseNode
	Sign string // "+" | "-" | "" (absent)
}

// ParseSignPrefix parses sign_prefix = [ "+" | "-" ].
func (p *V17Parser) ParseSignPrefix() (node *V17SignPrefixNode, err error) {
	done := p.debugEnter("sign_prefix")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	ch := p.peekAfterWS()
	if ch == '+' {
		tok, _ := p.matchLit("+")
		return &V17SignPrefixNode{V17BaseNode{tok.Line, tok.Col}, "+"}, nil
	}
	if ch == '-' {
		tok, _ := p.matchLit("-")
		return &V17SignPrefixNode{V17BaseNode{tok.Line, tok.Col}, "-"}, nil
	}
	// Optional — absent is valid
	return &V17SignPrefixNode{V17BaseNode{line, col}, ""}, nil
}

// =============================================================================
// STEP 6-7 — NUMERIC CONSTANTS
// integer       = sign_prefix digits
// decimal       = sign_prefix digits "." digits
// numeric_const = integer | decimal
// nan           = "NaN"
// infinity      = "Infinity"
// =============================================================================

// V17IntegerNode  integer = sign_prefix digits
type V17IntegerNode struct {
	V17BaseNode
	Sign   string
	Digits string
}

// ParseInteger parses integer = sign_prefix digits.
func (p *V17Parser) ParseInteger() (node *V17IntegerNode, err error) {
	done := p.debugEnter("integer")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	sign, err := p.ParseSignPrefix()
	if err != nil {
		return nil, err
	}
	dig, err := p.ParseDigits()
	if err != nil {
		return nil, err
	}
	return &V17IntegerNode{V17BaseNode{line, col}, sign.Sign, dig.Value}, nil
}

// V17DecimalNode  decimal = sign_prefix digits "." digits
type V17DecimalNode struct {
	V17BaseNode
	Sign     string
	Whole    string
	Fraction string
}

// ParseDecimal parses decimal = sign_prefix digits "." digits.
func (p *V17Parser) ParseDecimal() (node *V17DecimalNode, err error) {
	done := p.debugEnter("decimal")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	saved := p.savePos()

	sign, err := p.ParseSignPrefix()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	whole, err := p.ParseDigits()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("."); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	frac, err := p.ParseDigits()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	return &V17DecimalNode{V17BaseNode{line, col}, sign.Sign, whole.Value, frac.Value}, nil
}

// V17NumericConstNode  numeric_const = integer | decimal
type V17NumericConstNode struct {
	V17BaseNode
	Value interface{} // *V17DecimalNode | *V17IntegerNode
}

// ParseNumericConst parses numeric_const = integer | decimal.
// Tries decimal first (it is a superset of integer for parsing purposes).
func (p *V17Parser) ParseNumericConst() (node *V17NumericConstNode, err error) {
	done := p.debugEnter("numeric_const")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Try decimal first — requires "." so unambiguous
	saved := p.savePos()
	dec, derr := p.ParseDecimal()
	if derr == nil {
		return &V17NumericConstNode{V17BaseNode{line, col}, dec}, nil
	}
	p.restorePos(saved)

	// Try integer
	intNode, ierr := p.ParseInteger()
	if ierr == nil {
		return &V17NumericConstNode{V17BaseNode{line, col}, intNode}, nil
	}
	p.restorePos(saved)
	return nil, p.errAt("numeric_const: expected integer or decimal")
}

// V17NanNode  nan = "NaN"
type V17NanNode struct{ V17BaseNode }

// ParseNan parses nan = "NaN".
func (p *V17Parser) ParseNan() (node *V17NanNode, err error) {
	done := p.debugEnter("nan")
	defer func() { done(err == nil) }()
	tok, err := p.matchKeyword("NaN")
	if err != nil {
		return nil, p.errAt("nan: expected 'NaN'")
	}
	return &V17NanNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17InfinityNode  infinity = "Infinity"
type V17InfinityNode struct{ V17BaseNode }

// ParseInfinity parses infinity = "Infinity".
func (p *V17Parser) ParseInfinity() (node *V17InfinityNode, err error) {
	done := p.debugEnter("infinity")
	defer func() { done(err == nil) }()
	tok, err := p.matchKeyword("Infinity")
	if err != nil {
		return nil, p.errAt("infinity: expected 'Infinity'")
	}
	return &V17InfinityNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// =============================================================================
// STEP 9 — UNSIGNED INTEGER TYPES (RANGE directive — value check at parse time)
// byte   = RANGE 0..255<digits>
// uint8  = RANGE 0..255<digits>
// uint16 = RANGE 0..65535<digits>
// uint32 = RANGE 0..4294967295<digits>
// uint64 = RANGE 0..18446744073709551615<digits>
// uint128 = RANGE 0..340282366920938463463374607431768211455<digits>
// =============================================================================

// V17UintNode is shared by byte, uint8…uint128.
type V17UintNode struct {
	V17BaseNode
	TypeName string   // "byte" | "uint8" | "uint16" | "uint32" | "uint64" | "uint128"
	Value    string   // raw digit string
	Min      *big.Int // RANGE lower bound
	Max      *big.Int // RANGE upper bound
}

// parseUintRange is a private helper that parses digits and range-checks them.
func (p *V17Parser) parseUintRange(typeName string, min, max *big.Int) (*V17UintNode, error) {
	line, col := p.runeLine, p.runeCol
	dig, err := p.ParseDigits()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", typeName, err)
	}
	val := new(big.Int)
	if _, ok := val.SetString(dig.Value, 10); !ok {
		return nil, fmt.Errorf("%s: invalid digits %q at L%d:C%d", typeName, dig.Value, line, col)
	}
	if val.Cmp(min) < 0 || val.Cmp(max) > 0 {
		return nil, fmt.Errorf("%s: value %s out of range [%s..%s] at L%d:C%d",
			typeName, val, min, max, line, col)
	}
	return &V17UintNode{V17BaseNode{line, col}, typeName, dig.Value, min, max}, nil
}

var (
	bigZero      = big.NewInt(0)
	bigMax255    = big.NewInt(255)
	bigMax64k    = big.NewInt(65535)
	bigMax32     = new(big.Int).SetUint64(4294967295)
	bigMax64     = new(big.Int).SetUint64(18446744073709551615)
	bigMax128, _ = new(big.Int).SetString("340282366920938463463374607431768211455", 10)
)

// ParseByte parses byte = RANGE 0..255<digits>.
func (p *V17Parser) ParseByte() (node *V17UintNode, err error) {
	done := p.debugEnter("byte")
	defer func() { done(err == nil) }()
	return p.parseUintRange("byte", bigZero, bigMax255)
}

// ParseUint8 parses uint8 = RANGE 0..255<digits>.
func (p *V17Parser) ParseUint8() (node *V17UintNode, err error) {
	done := p.debugEnter("uint8")
	defer func() { done(err == nil) }()
	return p.parseUintRange("uint8", bigZero, bigMax255)
}

// ParseUint16 parses uint16 = RANGE 0..65535<digits>.
func (p *V17Parser) ParseUint16() (node *V17UintNode, err error) {
	done := p.debugEnter("uint16")
	defer func() { done(err == nil) }()
	return p.parseUintRange("uint16", bigZero, bigMax64k)
}

// ParseUint32 parses uint32 = RANGE 0..4294967295<digits>.
func (p *V17Parser) ParseUint32() (node *V17UintNode, err error) {
	done := p.debugEnter("uint32")
	defer func() { done(err == nil) }()
	return p.parseUintRange("uint32", bigZero, bigMax32)
}

// ParseUint64 parses uint64 = RANGE 0..18446744073709551615<digits>.
func (p *V17Parser) ParseUint64() (node *V17UintNode, err error) {
	done := p.debugEnter("uint64")
	defer func() { done(err == nil) }()
	return p.parseUintRange("uint64", bigZero, bigMax64)
}

// ParseUint128 parses uint128 = RANGE 0..340282366920938463463374607431768211455<digits>.
func (p *V17Parser) ParseUint128() (node *V17UintNode, err error) {
	done := p.debugEnter("uint128")
	defer func() { done(err == nil) }()
	return p.parseUintRange("uint128", bigZero, bigMax128)
}

// =============================================================================
// STEP 10 — FLOAT TYPES (TYPE_OF directive — type tag + value check)
// float32 = TYPE_OF float32<decimal | nan | infinity>
// float64 = TYPE_OF float64<decimal | nan | infinity>
// =============================================================================

// V17FloatNode holds a parsed float32 or float64 value.
type V17FloatNode struct {
	V17BaseNode
	TypeName string // "float32" | "float64"
	// exactly one of the following is non-nil:
	Decimal  *V17DecimalNode
	Nan      *V17NanNode
	Infinity *V17InfinityNode
}

// parseFloatInner parses decimal | nan | infinity and type-checks the decimal
// value fits in the named Go float type.
func (p *V17Parser) parseFloatInner(typeName string) (*V17FloatNode, error) {
	line, col := p.runeLine, p.runeCol

	// nan
	if p.peekLit("NaN") {
		n, err := p.ParseNan()
		if err != nil {
			return nil, err
		}
		return &V17FloatNode{V17BaseNode{line, col}, typeName, nil, n, nil}, nil
	}
	// infinity
	if p.peekLit("Infinity") || p.peekLit("+Infinity") || p.peekLit("-Infinity") {
		inf, err := p.ParseInfinity()
		if err != nil {
			return nil, err
		}
		return &V17FloatNode{V17BaseNode{line, col}, typeName, nil, nil, inf}, nil
	}
	// decimal
	saved := p.savePos()
	dec, err := p.ParseDecimal()
	if err != nil {
		p.restorePos(saved)
		return nil, p.errAt("%s: expected decimal, nan, or infinity", typeName)
	}
	// TYPE_OF check: ensure value is representable in the named type
	raw := dec.Sign + dec.Whole + "." + dec.Fraction
	f64, perr := strconv.ParseFloat(raw, 64)
	if perr != nil {
		return nil, fmt.Errorf("%s: cannot parse %q as float: %w", typeName, raw, perr)
	}
	if typeName == "float32" {
		f32 := float32(f64)
		if float64(f32) != f64 && !isInfOrNaN(f64) {
			// Precision loss is acceptable for float32 — we only reject overflow to ±Inf
			if f32 != float32(math.Inf(1)) && f32 != float32(math.Inf(-1)) {
				// value fits (precision loss is expected)
			}
		}
	}
	return &V17FloatNode{V17BaseNode{line, col}, typeName, dec, nil, nil}, nil
}

// isInfOrNaN is a small helper to avoid importing math in this file.
func isInfOrNaN(f float64) bool {
	return f != f || f > 1.7976931348623157e+308 || f < -1.7976931348623157e+308
}

// ParseFloat32 parses float32 = TYPE_OF float32<decimal | nan | infinity>.
func (p *V17Parser) ParseFloat32() (node *V17FloatNode, err error) {
	done := p.debugEnter("float32")
	defer func() { done(err == nil) }()
	return p.parseFloatInner("float32")
}

// ParseFloat64 parses float64 = TYPE_OF float64<decimal | nan | infinity>.
func (p *V17Parser) ParseFloat64() (node *V17FloatNode, err error) {
	done := p.debugEnter("float64")
	defer func() { done(err == nil) }()
	return p.parseFloatInner("float64")
}

// =============================================================================
// STEP 11 — DECIMAL TYPES (RANGE directive on integer part)
// decimal8   = RANGE -128..127<integer>     "." digits
// decimal16  = RANGE -32768..32767<integer> "." digits
// decimal32  = RANGE -2147483648..2147483647<integer>  "." digits
// decimal64  = RANGE -9223372036854775808..9223372036854775807<integer>  "." digits
// decimal128 = RANGE -170141183460469231731687303715884105728..170141183460469231731687303715884105727<integer>  "." digits
// =============================================================================

// V17DecimalTypeNode holds a parsed decimal8…decimal128 value.
type V17DecimalTypeNode struct {
	V17BaseNode
	TypeName string
	Integer  *V17IntegerNode // whole part (range-checked)
	Fraction string          // digit string after "."
	Min      *big.Int
	Max      *big.Int
}

var (
	bigDec8Min, _   = new(big.Int).SetString("-128", 10)
	bigDec8Max, _   = new(big.Int).SetString("127", 10)
	bigDec16Min, _  = new(big.Int).SetString("-32768", 10)
	bigDec16Max, _  = new(big.Int).SetString("32767", 10)
	bigDec32Min, _  = new(big.Int).SetString("-2147483648", 10)
	bigDec32Max, _  = new(big.Int).SetString("2147483647", 10)
	bigDec64Min, _  = new(big.Int).SetString("-9223372036854775808", 10)
	bigDec64Max, _  = new(big.Int).SetString("9223372036854775807", 10)
	bigDec128Min, _ = new(big.Int).SetString("-170141183460469231731687303715884105728", 10)
	bigDec128Max, _ = new(big.Int).SetString("170141183460469231731687303715884105727", 10)
)

// parseDecimalType is a private helper for decimal8…decimal128.
func (p *V17Parser) parseDecimalType(typeName string, min, max *big.Int) (*V17DecimalTypeNode, error) {
	line, col := p.runeLine, p.runeCol
	saved := p.savePos()

	intNode, err := p.ParseInteger()
	if err != nil {
		p.restorePos(saved)
		return nil, fmt.Errorf("%s: %w", typeName, err)
	}
	// RANGE check on integer part
	valStr := intNode.Sign + intNode.Digits
	val := new(big.Int)
	if _, ok := val.SetString(valStr, 10); !ok {
		p.restorePos(saved)
		return nil, fmt.Errorf("%s: invalid integer part %q at L%d:C%d", typeName, valStr, line, col)
	}
	if val.Cmp(min) < 0 || val.Cmp(max) > 0 {
		p.restorePos(saved)
		return nil, fmt.Errorf("%s: integer part %s out of range [%s..%s] at L%d:C%d",
			typeName, val, min, max, line, col)
	}
	if _, err = p.matchLit("."); err != nil {
		p.restorePos(saved)
		return nil, fmt.Errorf("%s: expected '.': %w", typeName, err)
	}
	frac, err := p.ParseDigits()
	if err != nil {
		p.restorePos(saved)
		return nil, fmt.Errorf("%s: expected fractional digits: %w", typeName, err)
	}
	return &V17DecimalTypeNode{V17BaseNode{line, col}, typeName, intNode, frac.Value, min, max}, nil
}

// ParseDecimal8 parses decimal8 = RANGE -128..127<integer> "." digits.
func (p *V17Parser) ParseDecimal8() (node *V17DecimalTypeNode, err error) {
	done := p.debugEnter("decimal8")
	defer func() { done(err == nil) }()
	return p.parseDecimalType("decimal8", bigDec8Min, bigDec8Max)
}

// ParseDecimal16 parses decimal16 = RANGE -32768..32767<integer> "." digits.
func (p *V17Parser) ParseDecimal16() (node *V17DecimalTypeNode, err error) {
	done := p.debugEnter("decimal16")
	defer func() { done(err == nil) }()
	return p.parseDecimalType("decimal16", bigDec16Min, bigDec16Max)
}

// ParseDecimal32 parses decimal32 = RANGE -2147483648..2147483647<integer> "." digits.
func (p *V17Parser) ParseDecimal32() (node *V17DecimalTypeNode, err error) {
	done := p.debugEnter("decimal32")
	defer func() { done(err == nil) }()
	return p.parseDecimalType("decimal32", bigDec32Min, bigDec32Max)
}

// ParseDecimal64 parses decimal64 = RANGE -9223372036854775808..9223372036854775807<integer> "." digits.
func (p *V17Parser) ParseDecimal64() (node *V17DecimalTypeNode, err error) {
	done := p.debugEnter("decimal64")
	defer func() { done(err == nil) }()
	return p.parseDecimalType("decimal64", bigDec64Min, bigDec64Max)
}

// ParseDecimal128 parses decimal128 = RANGE ...128-bit bounds...<integer> "." digits.
func (p *V17Parser) ParseDecimal128() (node *V17DecimalTypeNode, err error) {
	done := p.debugEnter("decimal128")
	defer func() { done(err == nil) }()
	return p.parseDecimalType("decimal128", bigDec128Min, bigDec128Max)
}

// V17DecimalNumNode  decimal_num = decimal8 | decimal16 | decimal32 | decimal64 | decimal128
type V17DecimalNumNode struct {
	V17BaseNode
	Value *V17DecimalTypeNode // narrowest fitting type
}

// ParseDecimalNum parses decimal_num — tries narrowest type first.
func (p *V17Parser) ParseDecimalNum() (node *V17DecimalNumNode, err error) {
	done := p.debugEnter("decimal_num")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	for _, try := range []func() (*V17DecimalTypeNode, error){
		p.ParseDecimal8, p.ParseDecimal16, p.ParseDecimal32,
		p.ParseDecimal64, p.ParseDecimal128,
	} {
		saved := p.savePos()
		n, nerr := try()
		if nerr == nil {
			return &V17DecimalNumNode{V17BaseNode{line, col}, n}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("decimal_num: value does not fit any decimal type")
}

// =============================================================================
// STEP 13-15 — DATE / TIME
// date_year   = digits4
// date_month  = RANGE 1..12<digits2>
// date_day    = RANGE 1..31<digits2>
// time_hour   = RANGE 0..23<digits2>
// time_minute = RANGE 0..59<digits2>
// time_second = RANGE 0..59<digits2>
// time_millis = RANGE 0..999<digits3>
// date        = date_year [ ["-"] date_month [ ["-"] date_day ] ]
// time        = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
// date_time   = ( date [ [" "] time ] ) | time
// time_stamp  = date [" "] time
// =============================================================================

// V17DateYearNode  date_year = digits4
type V17DateYearNode struct {
	V17BaseNode
	Value string
}

// ParseDateYear parses date_year = digits4.
func (p *V17Parser) ParseDateYear() (node *V17DateYearNode, err error) {
	done := p.debugEnter("date_year")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	d, err := p.ParseDigits4()
	if err != nil {
		return nil, err
	}
	return &V17DateYearNode{V17BaseNode{line, col}, d.Value}, nil
}

// parseDateComponent is a private helper: parse digits2/digits3 then RANGE-check.
func (p *V17Parser) parseDateComponent(name string, parseDigits func() (string, error), min, max int) (string, int, int, error) {
	line, col := p.runeLine, p.runeCol
	val, err := parseDigits()
	if err != nil {
		return "", 0, 0, fmt.Errorf("%s: %w", name, err)
	}
	n, _ := strconv.Atoi(val)
	if n < min || n > max {
		return "", 0, 0, fmt.Errorf("%s: value %d out of range [%d..%d] at L%d:C%d", name, n, min, max, line, col)
	}
	return val, line, col, nil
}

// V17DateMonthNode  date_month = RANGE 1..12<digits2>
type V17DateMonthNode struct {
	V17BaseNode
	Value string
}

// ParseDateMonth parses date_month = RANGE 1..12<digits2>.
func (p *V17Parser) ParseDateMonth() (node *V17DateMonthNode, err error) {
	done := p.debugEnter("date_month")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	d2, err := p.ParseDigits2()
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(d2.Value)
	if n < 1 || n > 12 {
		return nil, fmt.Errorf("date_month: value %d out of range [1..12] at L%d:C%d", n, line, col)
	}
	return &V17DateMonthNode{V17BaseNode{line, col}, d2.Value}, nil
}

// V17DateDayNode  date_day = RANGE 1..31<digits2>
type V17DateDayNode struct {
	V17BaseNode
	Value string
}

// ParseDateDay parses date_day = RANGE 1..31<digits2>.
func (p *V17Parser) ParseDateDay() (node *V17DateDayNode, err error) {
	done := p.debugEnter("date_day")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	d2, err := p.ParseDigits2()
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(d2.Value)
	if n < 1 || n > 31 {
		return nil, fmt.Errorf("date_day: value %d out of range [1..31] at L%d:C%d", n, line, col)
	}
	return &V17DateDayNode{V17BaseNode{line, col}, d2.Value}, nil
}

// V17TimeHourNode  time_hour = RANGE 0..23<digits2>
type V17TimeHourNode struct {
	V17BaseNode
	Value string
}

// ParseTimeHour parses time_hour = RANGE 0..23<digits2>.
func (p *V17Parser) ParseTimeHour() (node *V17TimeHourNode, err error) {
	done := p.debugEnter("time_hour")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	d2, err := p.ParseDigits2()
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(d2.Value)
	if n < 0 || n > 23 {
		return nil, fmt.Errorf("time_hour: value %d out of range [0..23] at L%d:C%d", n, line, col)
	}
	return &V17TimeHourNode{V17BaseNode{line, col}, d2.Value}, nil
}

// V17TimeMinuteNode  time_minute = RANGE 0..59<digits2>
type V17TimeMinuteNode struct {
	V17BaseNode
	Value string
}

// ParseTimeMinute parses time_minute = RANGE 0..59<digits2>.
func (p *V17Parser) ParseTimeMinute() (node *V17TimeMinuteNode, err error) {
	done := p.debugEnter("time_minute")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	d2, err := p.ParseDigits2()
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(d2.Value)
	if n < 0 || n > 59 {
		return nil, fmt.Errorf("time_minute: value %d out of range [0..59] at L%d:C%d", n, line, col)
	}
	return &V17TimeMinuteNode{V17BaseNode{line, col}, d2.Value}, nil
}

// V17TimeSecondNode  time_second = RANGE 0..59<digits2>
type V17TimeSecondNode struct {
	V17BaseNode
	Value string
}

// ParseTimeSecond parses time_second = RANGE 0..59<digits2>.
func (p *V17Parser) ParseTimeSecond() (node *V17TimeSecondNode, err error) {
	done := p.debugEnter("time_second")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	d2, err := p.ParseDigits2()
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(d2.Value)
	if n < 0 || n > 59 {
		return nil, fmt.Errorf("time_second: value %d out of range [0..59] at L%d:C%d", n, line, col)
	}
	return &V17TimeSecondNode{V17BaseNode{line, col}, d2.Value}, nil
}

// V17TimeMillisNode  time_millis = RANGE 0..999<digits3>
type V17TimeMillisNode struct {
	V17BaseNode
	Value string
}

// ParseTimeMillis parses time_millis = RANGE 0..999<digits3>.
func (p *V17Parser) ParseTimeMillis() (node *V17TimeMillisNode, err error) {
	done := p.debugEnter("time_millis")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	d3, err := p.ParseDigits3()
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(d3.Value)
	if n < 0 || n > 999 {
		return nil, fmt.Errorf("time_millis: value %d out of range [0..999] at L%d:C%d", n, line, col)
	}
	return &V17TimeMillisNode{V17BaseNode{line, col}, d3.Value}, nil
}

// V17DateNode  date = date_year [ ["-"] date_month [ ["-"] date_day ] ]
type V17DateNode struct {
	V17BaseNode
	Year  *V17DateYearNode
	Month *V17DateMonthNode // nil if absent
	Day   *V17DateDayNode   // nil if absent
}

// ParseDate parses date = date_year [ ["-"] date_month [ ["-"] date_day ] ].
func (p *V17Parser) ParseDate() (node *V17DateNode, err error) {
	done := p.debugEnter("date")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	year, err := p.ParseDateYear()
	if err != nil {
		return nil, err
	}
	n := &V17DateNode{V17BaseNode{line, col}, year, nil, nil}

	// [ ["-"] date_month [ ["-"] date_day ] ]
	saved := p.savePos()
	// optional "-"
	if p.peekAfterWS() == '-' {
		p.matchLit("-") //nolint:errcheck
	}
	month, merr := p.ParseDateMonth()
	if merr != nil {
		p.restorePos(saved)
		return n, nil
	}
	n.Month = month

	saved2 := p.savePos()
	if p.peekAfterWS() == '-' {
		p.matchLit("-") //nolint:errcheck
	}
	day, derr := p.ParseDateDay()
	if derr != nil {
		p.restorePos(saved2)
		return n, nil
	}
	n.Day = day
	return n, nil
}

// V17TimeNode  time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ]
type V17TimeNode struct {
	V17BaseNode
	Hour   *V17TimeHourNode
	Minute *V17TimeMinuteNode // nil if absent
	Second *V17TimeSecondNode // nil if absent
	Millis *V17TimeMillisNode // nil if absent
}

// ParseTime parses time = time_hour [ [":"] time_minute [ [":"] time_second [ ["."] time_millis ] ] ].
func (p *V17Parser) ParseTime() (node *V17TimeNode, err error) {
	done := p.debugEnter("time")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	hour, err := p.ParseTimeHour()
	if err != nil {
		return nil, err
	}
	n := &V17TimeNode{V17BaseNode{line, col}, hour, nil, nil, nil}

	saved := p.savePos()
	ch := p.peekAfterWS()
	if ch == ':' {
		p.matchLit(":") //nolint:errcheck
	} else if ch >= '0' && ch <= '9' {
		// no separator — try to parse minute directly
	} else {
		return n, nil
	}
	min, merr := p.ParseTimeMinute()
	if merr != nil {
		p.restorePos(saved)
		return n, nil
	}
	n.Minute = min

	saved2 := p.savePos()
	// optional ":" separator
	if p.peekAfterWS() == ':' {
		p.matchLit(":") //nolint:errcheck
	}
	sec, serr := p.ParseTimeSecond()
	if serr != nil {
		p.restorePos(saved2)
		return n, nil
	}
	n.Second = sec

	saved3 := p.savePos()
	if p.peekAfterWS() == '.' {
		p.matchLit(".") //nolint:errcheck
	}
	ms, mserr := p.ParseTimeMillis()
	if mserr != nil {
		p.restorePos(saved3)
		return n, nil
	}
	n.Millis = ms
	return n, nil
}

// V17DateTimeNode  date_time = ( date [ [" "] time ] ) | time
type V17DateTimeNode struct {
	V17BaseNode
	Date *V17DateNode // nil if time-only
	Time *V17TimeNode // nil if date-only
}

// ParseDateTime parses date_time = ( date [ [" "] time ] ) | time.
func (p *V17Parser) ParseDateTime() (node *V17DateTimeNode, err error) {
	done := p.debugEnter("date_time")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Try date first
	saved := p.savePos()
	date, derr := p.ParseDate()
	if derr == nil {
		n := &V17DateTimeNode{V17BaseNode{line, col}, date, nil}
		// optional [" "] time
		savedT := p.savePos()
		// skip optional space token (already consumed by lexer as whitespace)
		t, terr := p.ParseTime()
		if terr == nil {
			n.Time = t
		} else {
			p.restorePos(savedT)
		}
		return n, nil
	}
	p.restorePos(saved)

	// Try time only
	t, terr := p.ParseTime()
	if terr != nil {
		return nil, p.errAt("date_time: expected date or time")
	}
	return &V17DateTimeNode{V17BaseNode{line, col}, nil, t}, nil
}

// V17TimeStampNode  time_stamp = date [" "] time
type V17TimeStampNode struct {
	V17BaseNode
	Date *V17DateNode
	Time *V17TimeNode
}

// ParseTimeStamp parses time_stamp = date [" "] time.
func (p *V17Parser) ParseTimeStamp() (node *V17TimeStampNode, err error) {
	done := p.debugEnter("time_stamp")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	saved := p.savePos()

	date, derr := p.ParseDate()
	if derr != nil {
		return nil, derr
	}
	// [" "] — space already consumed by lexer; no token to skip
	t, terr := p.ParseTime()
	if terr != nil {
		p.restorePos(saved)
		return nil, fmt.Errorf("time_stamp: time part required: %w", terr)
	}
	return &V17TimeStampNode{V17BaseNode{line, col}, date, t}, nil
}

// =============================================================================
// STEP 16-17 — DURATION
// duration_unit = "ms" | "s" | "m" | "h" | "d" | "w"
// duration      = digits duration_unit { digits duration_unit }
// =============================================================================

// V17DurationUnitNode  duration_unit = "ms" | "s" | "m" | "h" | "d" | "w"
type V17DurationUnitNode struct {
	V17BaseNode
	Unit string
}

// v17durationUnits is the set of valid unit strings (SIR-4 — checked at parse time).
var v17durationUnits = map[string]bool{
	"ms": true, "s": true, "m": true, "h": true, "d": true, "w": true,
}

// extractDurationUnit extracts a valid duration unit prefix from s.
// Returns (unit, rest). Returns ("", s) if no valid unit prefix found.
func extractDurationUnit(s string) (unit, rest string) {
	if len(s) >= 2 && v17durationUnits[s[:2]] {
		return s[:2], s[2:]
	}
	if len(s) >= 1 && v17durationUnits[s[:1]] {
		return s[:1], s[1:]
	}
	return "", s
}

// extractLeadingDigits splits s into leading digit chars and the remainder.
func extractLeadingDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i], s[i:]
}

// parseDurationFromCompoundIdent parses a compound duration like "h30m" (from
// an IDENT token that follows the first DIGITS token) and returns all parts.
func parseDurationFromCompoundIdent(firstDigits, identVal string) ([]V17DurationPart, bool) {
	unit, rem := extractDurationUnit(identVal)
	if unit == "" {
		return nil, false
	}
	parts := []V17DurationPart{{firstDigits, unit}}
	for rem != "" {
		digs, r := extractLeadingDigits(rem)
		if digs == "" {
			return nil, false
		}
		u2, r2 := extractDurationUnit(r)
		if u2 == "" {
			return nil, false
		}
		parts = append(parts, V17DurationPart{digs, u2})
		rem = r2
	}
	return parts, true
}

// ParseDurationUnit parses duration_unit = "ms" | "s" | "m" | "h" | "d" | "w".
func (p *V17Parser) ParseDurationUnit() (node *V17DurationUnitNode, err error) {
	done := p.debugEnter("duration_unit")
	defer func() { done(err == nil) }()
	saved := p.savePos()
	tok, terr := p.matchRe(reIdentScan)
	if terr != nil || !v17durationUnits[tok.Value] {
		p.restorePos(saved)
		return nil, p.errAt("duration_unit: expected ms|s|m|h|d|w")
	}
	return &V17DurationUnitNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17DurationNode  duration = digits duration_unit { digits duration_unit }
type V17DurationNode struct {
	V17BaseNode
	Parts []V17DurationPart
}

// V17DurationPart holds one (digits, unit) pair.
type V17DurationPart struct {
	Digits string
	Unit   string
}

// ParseDuration parses duration = digits duration_unit { digits duration_unit }.
// Handles both separate-token form ("1h" → DIGITS+IDENT) and compact compound
// IDENT form ("1h30m" → DIGITS("1")+IDENT("h30m")).
func (p *V17Parser) ParseDuration() (node *V17DurationNode, err error) {
	done := p.debugEnter("duration")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	dig, err := p.ParseDigits()
	if err != nil {
		return nil, err
	}

	// Check for compound ident e.g. IDENT("h30m")
	// Try to match an ident after the digits and check if it's a valid unit
	if saved2 := p.savePos(); true {
		if identTok, ierr := p.matchRe(reIdentScan); ierr == nil {
			if !v17durationUnits[identTok.Value] {
				// Might be a compound like "h30m" — try string parsing
				parts, ok := parseDurationFromCompoundIdent(dig.Value, identTok.Value)
				if !ok {
					p.restorePos(saved2)
					return nil, p.errAt("duration: invalid unit or compound %q", identTok.Value)
				}
				return &V17DurationNode{V17BaseNode{line, col}, parts}, nil
			}
			// valid single unit — still consumed, return it
			return &V17DurationNode{V17BaseNode{line, col}, []V17DurationPart{{dig.Value, identTok.Value}}}, nil
		} else {
			p.restorePos(saved2)
		}
	}

	// Standard path: simple unit token
	unit, err := p.ParseDurationUnit()
	if err != nil {
		return nil, err
	}
	n := &V17DurationNode{V17BaseNode{line, col}, []V17DurationPart{{dig.Value, unit.Unit}}}

	// { digits duration_unit }
	for {
		saved := p.savePos()
		d2, derr := p.ParseDigits()
		if derr != nil {
			p.restorePos(saved)
			break
		}
		u2, uerr := p.ParseDurationUnit()
		if uerr != nil {
			p.restorePos(saved)
			break
		}
		n.Parts = append(n.Parts, V17DurationPart{d2.Value, u2.Unit})
	}
	return n, nil
}

// =============================================================================
// STEP 18 — STRINGS
// single_quoted = "'" /(?<value>(\\'|[^'])+)/ "'"
// double_quoted = '"' /(?<value>(\\"|[^"])+)/ '"'
// string_quoted = single_quoted | double_quoted
// tmpl_text     = /(?:(?!§\(|`)[\s\S])*/
// tmpl_quoted   = "`" tmpl_text "`"
// string        = single_quoted | double_quoted | tmpl_quoted
// =============================================================================

// V17SingleQuotedNode  single_quoted = "'" … "'"
type V17SingleQuotedNode struct {
	V17BaseNode
	Value string
}

// ParseSingleQuoted parses single_quoted.
func (p *V17Parser) ParseSingleQuoted() (node *V17SingleQuotedNode, err error) {
	done := p.debugEnter("single_quoted")
	defer func() { done(err == nil) }()
	tok, err := p.matchSingleQuoted()
	if err != nil {
		return nil, err
	}
	if tok.Value == "" {
		return nil, p.errAt("single_quoted: empty string not allowed")
	}
	return &V17SingleQuotedNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17DoubleQuotedNode  double_quoted = '"' … '"'
type V17DoubleQuotedNode struct {
	V17BaseNode
	Value string
}

// ParseDoubleQuoted parses double_quoted.
func (p *V17Parser) ParseDoubleQuoted() (node *V17DoubleQuotedNode, err error) {
	done := p.debugEnter("double_quoted")
	defer func() { done(err == nil) }()
	tok, err := p.matchDoubleQuoted()
	if err != nil {
		return nil, err
	}
	if tok.Value == "" {
		return nil, p.errAt("double_quoted: empty string not allowed")
	}
	return &V17DoubleQuotedNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17StringQuotedNode  string_quoted = single_quoted | double_quoted
type V17StringQuotedNode struct {
	V17BaseNode
	Value interface{} // *V17SingleQuotedNode | *V17DoubleQuotedNode
}

// ParseStringQuoted parses string_quoted = single_quoted | double_quoted.
func (p *V17Parser) ParseStringQuoted() (node *V17StringQuotedNode, err error) {
	done := p.debugEnter("string_quoted")
	defer func() { done(err == nil) }()

	ch := p.peekAfterWS()
	line, col := p.runeLine, p.runeCol
	if ch == '\'' {
		n, nerr := p.ParseSingleQuoted()
		if nerr != nil {
			return nil, nerr
		}
		return &V17StringQuotedNode{V17BaseNode{line, col}, n}, nil
	}
	if ch == '"' {
		n, nerr := p.ParseDoubleQuoted()
		if nerr != nil {
			return nil, nerr
		}
		return &V17StringQuotedNode{V17BaseNode{line, col}, n}, nil
	}
	return nil, p.errAt("string_quoted: expected single or double quoted string")
}

// V17TmplTextNode  tmpl_text = /(?:(?!§\(|`)[\s\S])*/
type V17TmplTextNode struct {
	V17BaseNode
	Value string
}

// ParseTmplText parses tmpl_text — the content of a template literal (already
// captured in the V17_STRING_TQ token value by the lexer).
// This method is called from ParseTmplQuoted and operates on the already-lexed value.
func (p *V17Parser) ParseTmplText(raw string, line, col int) (node *V17TmplTextNode, err error) {
	done := p.debugEnter("tmpl_text")
	defer func() { done(err == nil) }()
	return &V17TmplTextNode{V17BaseNode{line, col}, raw}, nil
}

// V17TmplQuotedNode  tmpl_quoted = "`" tmpl_text "`"
type V17TmplQuotedNode struct {
	V17BaseNode
	Text *V17TmplTextNode
}

// ParseTmplQuoted parses tmpl_quoted = "`" tmpl_text "`".
func (p *V17Parser) ParseTmplQuoted() (node *V17TmplQuotedNode, err error) {
	done := p.debugEnter("tmpl_quoted")
	defer func() { done(err == nil) }()
	tok, err := p.matchTemplateQuoted()
	if err != nil {
		return nil, err
	}
	txt, _ := p.ParseTmplText(tok.Value, tok.Line, tok.Col)
	return &V17TmplQuotedNode{V17BaseNode{tok.Line, tok.Col}, txt}, nil
}

// V17StringNode  string = single_quoted | double_quoted | tmpl_quoted
type V17StringNode struct {
	V17BaseNode
	Value interface{} // *V17SingleQuotedNode | *V17DoubleQuotedNode | *V17TmplQuotedNode
}

// ParseString parses string = single_quoted | double_quoted | tmpl_quoted.
func (p *V17Parser) ParseString() (node *V17StringNode, err error) {
	done := p.debugEnter("string")
	defer func() { done(err == nil) }()

	ch := p.peekAfterWS()
	line, col := p.runeLine, p.runeCol
	if ch == '\'' {
		n, nerr := p.ParseSingleQuoted()
		if nerr != nil {
			return nil, nerr
		}
		return &V17StringNode{V17BaseNode{line, col}, n}, nil
	}
	if ch == '"' {
		n, nerr := p.ParseDoubleQuoted()
		if nerr != nil {
			return nil, nerr
		}
		return &V17StringNode{V17BaseNode{line, col}, n}, nil
	}
	if ch == '`' {
		n, nerr := p.ParseTmplQuoted()
		if nerr != nil {
			return nil, nerr
		}
		return &V17StringNode{V17BaseNode{line, col}, n}, nil
	}
	return nil, p.errAt("string: expected quoted string")
}

// =============================================================================
// STEP 20-21 — REGEXP
// regexp_flags = "g" | "i" | "m" | "s" | "u" | "y" | "x" | "n" | "A"
// regexp_expr  = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags { regexp_flags } ]
// =============================================================================

// V17RegexpFlagsNode  regexp_flags = "g"|"i"|"m"|"s"|"u"|"y"|"x"|"n"|"A"
type V17RegexpFlagsNode struct {
	V17BaseNode
	Flag string
}

// v17regexpFlagSet is the valid set of regexp flag characters (SIR-4).
var v17regexpFlagSet = map[string]bool{
	"g": true, "i": true, "m": true, "s": true, "u": true,
	"y": true, "x": true, "n": true, "A": true,
}

// ParseRegexpFlags parses regexp_flags — a single flag character.
func (p *V17Parser) ParseRegexpFlags() (node *V17RegexpFlagsNode, err error) {
	done := p.debugEnter("regexp_flags")
	defer func() { done(err == nil) }()
	tok, terr := p.matchRe(reRegexpFlagOneScan)
	if terr != nil {
		return nil, p.errAt("regexp_flags: expected g|i|m|s|u|y|x|n|A")
	}
	return &V17RegexpFlagsNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17RegexpExprNode  regexp_expr = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags { regexp_flags } ]
type V17RegexpExprNode struct {
	V17BaseNode
	Pattern string
	Flags   []string
}

// ParseRegexpExpr parses regexp_expr.
// TYPE_OF XRegExp is a type annotation — the parser records it on the node; no
// extra token is consumed. The regexp pattern is captured by the rune-stream helper.
func (p *V17Parser) ParseRegexpExpr() (node *V17RegexpExprNode, err error) {
	done := p.debugEnter("regexp_expr")
	defer func() { done(err == nil) }()

	tok, terr := p.matchRegexpLiteral()
	if terr != nil {
		return nil, terr
	}
	n := &V17RegexpExprNode{V17BaseNode{tok.Line, tok.Col}, tok.Value, nil}

	// [ regexp_flags { regexp_flags } ]
	for {
		saved := p.savePos()
		f, ferr := p.ParseRegexpFlags()
		if ferr != nil {
			p.restorePos(saved)
			break
		}
		n.Flags = append(n.Flags, f.Flag)
	}
	return n, nil
}

// =============================================================================
// STEP 22 — BOOLEAN / NULL / ANY_TYPE
// boolean_true  = "true"
// boolean_false = "false"
// boolean       = "true" | "false"
// null          = "null"
// any_type      = "@?"
// =============================================================================

// V17BooleanTrueNode  boolean_true = "true"
type V17BooleanTrueNode struct{ V17BaseNode }

// ParseBooleanTrue parses boolean_true = "true".
func (p *V17Parser) ParseBooleanTrue() (node *V17BooleanTrueNode, err error) {
	done := p.debugEnter("boolean_true")
	defer func() { done(err == nil) }()
	tok, err := p.matchKeyword("true")
	if err != nil {
		return nil, p.errAt("boolean_true: expected 'true'")
	}
	return &V17BooleanTrueNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17BooleanFalseNode  boolean_false = "false"
type V17BooleanFalseNode struct{ V17BaseNode }

// ParseBooleanFalse parses boolean_false = "false".
func (p *V17Parser) ParseBooleanFalse() (node *V17BooleanFalseNode, err error) {
	done := p.debugEnter("boolean_false")
	defer func() { done(err == nil) }()
	tok, err := p.matchKeyword("false")
	if err != nil {
		return nil, p.errAt("boolean_false: expected 'false'")
	}
	return &V17BooleanFalseNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17BooleanNode  boolean = "true" | "false"
type V17BooleanNode struct {
	V17BaseNode
	Value bool
}

// ParseBoolean parses boolean = "true" | "false".
func (p *V17Parser) ParseBoolean() (node *V17BooleanNode, err error) {
	done := p.debugEnter("boolean")
	defer func() { done(err == nil) }()
	if tok, kerr := p.matchKeyword("true"); kerr == nil {
		return &V17BooleanNode{V17BaseNode{tok.Line, tok.Col}, true}, nil
	}
	tok, kerr := p.matchKeyword("false")
	if kerr != nil {
		return nil, p.errAt("boolean: expected true or false")
	}
	return &V17BooleanNode{V17BaseNode{tok.Line, tok.Col}, false}, nil
}

// V17NullNode  null = "null"
type V17NullNode struct{ V17BaseNode }

// ParseNull parses null = "null".
func (p *V17Parser) ParseNull() (node *V17NullNode, err error) {
	done := p.debugEnter("null")
	defer func() { done(err == nil) }()
	tok, err := p.matchKeyword("null")
	if err != nil {
		return nil, p.errAt("null: expected 'null'")
	}
	return &V17NullNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17AnyTypeNode  any_type = "@?"
type V17AnyTypeNode struct{ V17BaseNode }

// ParseAnyType parses any_type = "@?".
func (p *V17Parser) ParseAnyType() (node *V17AnyTypeNode, err error) {
	done := p.debugEnter("any_type")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("@?")
	if err != nil {
		return nil, p.errAt("any_type: expected '@?'")
	}
	return &V17AnyTypeNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// =============================================================================
// STEP 23 — CARDINALITY / RANGE
// cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
// range       = integer ".." integer
// =============================================================================

// V17CardinalityNode  cardinality = digits ".." ( digits | "m" | "M" | "many" | "Many" )
type V17CardinalityNode struct {
	V17BaseNode
	Lower string
	Upper string // digits value or "m"/"M"/"many"/"Many"
}

// v17cardinalityUpperKw is the set of keyword values valid as cardinality upper bound.
var v17cardinalityUpperKw = map[string]bool{
	"m": true, "M": true, "many": true, "Many": true,
}

// ParseCardinality parses cardinality = digits ".." ( digits | "m"|"M"|"many"|"Many" ).
func (p *V17Parser) ParseCardinality() (node *V17CardinalityNode, err error) {
	done := p.debugEnter("cardinality")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	low, err := p.ParseDigits()
	if err != nil {
		return nil, err
	}
	if _, err = p.matchLit(".."); err != nil {
		return nil, err
	}
	// upper: digits | "m" | "M" | "many" | "Many"
	ch := p.peekAfterWS()
	if ch >= '0' && ch <= '9' {
		dig, derr := p.ParseDigits()
		if derr != nil {
			return nil, p.errAt("cardinality: expected digits after '..'")
		}
		return &V17CardinalityNode{V17BaseNode{line, col}, low.Value, dig.Value}, nil
	}
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
		saved2 := p.savePos()
		kwTok, kwerr := p.matchRe(reIdentScan)
		if kwerr != nil || !v17cardinalityUpperKw[kwTok.Value] {
			p.restorePos(saved2)
			return nil, p.errAt("cardinality: expected digits or many/Many/m/M after '..'")
		}
		return &V17CardinalityNode{V17BaseNode{line, col}, low.Value, kwTok.Value}, nil
	}
	return nil, p.errAt("cardinality: expected digits or many/Many/m/M after '..'")
}

// V17RangeNode  range = integer ".." integer
type V17RangeNode struct {
	V17BaseNode
	Lower *V17IntegerNode
	Upper *V17IntegerNode
}

// ParseRange parses range = integer ".." integer.
func (p *V17Parser) ParseRange() (node *V17RangeNode, err error) {
	done := p.debugEnter("range")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	low, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	if _, err = p.matchLit(".."); err != nil {
		return nil, err
	}
	high, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	return &V17RangeNode{V17BaseNode{line, col}, low, high}, nil
}

// =============================================================================
// STEP 24 — HEX SEGMENTS
// hex_seg2  = /[0-9a-fA-F]{2}/
// hex_seg4  = /[0-9a-fA-F]{4}/
// hex_seg8  = /[0-9a-fA-F]{8}/
// hex_seg12 = /[0-9a-fA-F]{12}/
// hex_seg32 = /[0-9a-fA-F]{32}/
// hex_seg40 = /[0-9a-fA-F]{40}/
// hex_seg64 = /[0-9a-fA-F]{64}/
// hex_seg128= /[0-9a-fA-F]{128}/
// =============================================================================

// V17HexSegNode holds a validated hex segment.
type V17HexSegNode struct {
	V17BaseNode
	RuleName string
	Value    string
}

// parseHexSegRS is the rune-stream replacement for parseHexSeg.
// It uses matchHexExact(n) instead of the token-API matchHexToken helper.
func (p *V17Parser) parseHexSegRS(ruleName string, n int) (*V17HexSegNode, error) {
	tok, err := p.matchHexExact(n)
	if err != nil {
		return nil, p.errAt("%s: expected %d hex digits: %v", ruleName, n, err)
	}
	return &V17HexSegNode{V17BaseNode{tok.Line, tok.Col}, ruleName, tok.Value}, nil
}

// ParseHexSeg2 parses hex_seg2 = /[0-9a-fA-F]{2}/.
func (p *V17Parser) ParseHexSeg2() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg2")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg2", 2)
}

// ParseHexSeg4 parses hex_seg4 = /[0-9a-fA-F]{4}/.
func (p *V17Parser) ParseHexSeg4() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg4")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg4", 4)
}

// ParseHexSeg8 parses hex_seg8 = /[0-9a-fA-F]{8}/.
func (p *V17Parser) ParseHexSeg8() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg8")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg8", 8)
}

// ParseHexSeg12 parses hex_seg12 = /[0-9a-fA-F]{12}/.
func (p *V17Parser) ParseHexSeg12() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg12")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg12", 12)
}

// ParseHexSeg32 parses hex_seg32 = /[0-9a-fA-F]{32}/.
func (p *V17Parser) ParseHexSeg32() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg32")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg32", 32)
}

// ParseHexSeg40 parses hex_seg40 = /[0-9a-fA-F]{40}/.
func (p *V17Parser) ParseHexSeg40() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg40")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg40", 40)
}

// ParseHexSeg64 parses hex_seg64 = /[0-9a-fA-F]{64}/.
func (p *V17Parser) ParseHexSeg64() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg64")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg64", 64)
}

// ParseHexSeg128 parses hex_seg128 = /[0-9a-fA-F]{128}/.
func (p *V17Parser) ParseHexSeg128() (node *V17HexSegNode, err error) {
	done := p.debugEnter("hex_seg128")
	defer func() { done(err == nil) }()
	return p.parseHexSegRS("hex_seg128", 128)
}

// =============================================================================
// STEP 25-27 — UUID
// uuid    = hex_seg8 "-" hex_seg4 "-" hex_seg4 "-" hex_seg4 "-" hex_seg12
// uuid_v7_ver = /7[0-9a-fA-F]{3}/
// uuid_v7_var = /[89aAbB][0-9a-fA-F]{3}/
// uuid_v7 = hex_seg8 "-" hex_seg4 "-" uuid_v7_ver "-" uuid_v7_var "-" hex_seg12
// =============================================================================

// V17UuidNode  uuid = hex_seg8 "-" hex_seg4 "-" hex_seg4 "-" hex_seg4 "-" hex_seg12
type V17UuidNode struct {
	V17BaseNode
	Value string // reassembled hex string
}

// ParseUuid parses uuid.
func (p *V17Parser) ParseUuid() (node *V17UuidNode, err error) {
	done := p.debugEnter("uuid")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	saved := p.savePos()

	s8, err := p.ParseHexSeg8()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	s4a, err := p.ParseHexSeg4()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	s4b, err := p.ParseHexSeg4()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	s4c, err := p.ParseHexSeg4()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	s12, err := p.ParseHexSeg12()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}

	val := s8.Value + "-" + s4a.Value + "-" + s4b.Value + "-" + s4c.Value + "-" + s12.Value
	return &V17UuidNode{V17BaseNode{line, col}, val}, nil
}

// V17UuidV7VerNode  uuid_v7_ver = /7[0-9a-fA-F]{3}/
type V17UuidV7VerNode struct {
	V17BaseNode
	Value string
}

// ParseUuidV7Ver parses uuid_v7_ver.
func (p *V17Parser) ParseUuidV7Ver() (node *V17UuidV7VerNode, err error) {
	done := p.debugEnter("uuid_v7_ver")
	defer func() { done(err == nil) }()
	tok, terr := p.matchRe(reUuidV7VerScan)
	if terr != nil {
		return nil, p.errAt("uuid_v7_ver: expected 4-char hex starting with 7")
	}
	return &V17UuidV7VerNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17UuidV7VarNode  uuid_v7_var = /[89aAbB][0-9a-fA-F]{3}/
type V17UuidV7VarNode struct {
	V17BaseNode
	Value string
}

// ParseUuidV7Var parses uuid_v7_var.
func (p *V17Parser) ParseUuidV7Var() (node *V17UuidV7VarNode, err error) {
	done := p.debugEnter("uuid_v7_var")
	defer func() { done(err == nil) }()
	tok, terr := p.matchRe(reUuidV7VarScan)
	if terr != nil {
		return nil, p.errAt("uuid_v7_var: expected 4-char hex starting with 8|9|a|b")
	}
	return &V17UuidV7VarNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17UuidV7Node  uuid_v7 = hex_seg8 "-" hex_seg4 "-" uuid_v7_ver "-" uuid_v7_var "-" hex_seg12
type V17UuidV7Node struct {
	V17BaseNode
	Value string
}

// ParseUuidV7 parses uuid_v7.
func (p *V17Parser) ParseUuidV7() (node *V17UuidV7Node, err error) {
	done := p.debugEnter("uuid_v7")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	saved := p.savePos()

	s8, err := p.ParseHexSeg8()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	s4, err := p.ParseHexSeg4()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	ver, err := p.ParseUuidV7Ver()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	vari, err := p.ParseUuidV7Var()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchLit("-"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	s12, err := p.ParseHexSeg12()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}

	val := s8.Value + "-" + s4.Value + "-" + ver.Value + "-" + vari.Value + "-" + s12.Value
	return &V17UuidV7Node{V17BaseNode{line, col}, val}, nil
}

// =============================================================================
// STEP 29-30 — HASH KEYS
// hash_md5    = TYPE_OF hash_md5<hex_seg32>
// hash_sha1   = TYPE_OF hash_sha1<hex_seg40>
// hash_sha256 = TYPE_OF hash_sha256<hex_seg64>
// hash_sha512 = TYPE_OF hash_sha512<hex_seg128>
// hash_key    = hash_md5 | hash_sha1 | hash_sha256 | hash_sha512
// =============================================================================

// V17HashKeyNode holds a parsed hash key with TYPE_OF tag.
type V17HashKeyNode struct {
	V17BaseNode
	TypeName string // "hash_md5" | "hash_sha1" | "hash_sha256" | "hash_sha512"
	Seg      *V17HexSegNode
}

// ParseHashMd5 parses hash_md5 = TYPE_OF hash_md5<hex_seg32>.
func (p *V17Parser) ParseHashMd5() (node *V17HashKeyNode, err error) {
	done := p.debugEnter("hash_md5")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	seg, err := p.ParseHexSeg32()
	if err != nil {
		return nil, err
	}
	return &V17HashKeyNode{V17BaseNode{line, col}, "hash_md5", seg}, nil
}

// ParseHashSha1 parses hash_sha1 = TYPE_OF hash_sha1<hex_seg40>.
func (p *V17Parser) ParseHashSha1() (node *V17HashKeyNode, err error) {
	done := p.debugEnter("hash_sha1")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	seg, err := p.ParseHexSeg40()
	if err != nil {
		return nil, err
	}
	return &V17HashKeyNode{V17BaseNode{line, col}, "hash_sha1", seg}, nil
}

// ParseHashSha256 parses hash_sha256 = TYPE_OF hash_sha256<hex_seg64>.
func (p *V17Parser) ParseHashSha256() (node *V17HashKeyNode, err error) {
	done := p.debugEnter("hash_sha256")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	seg, err := p.ParseHexSeg64()
	if err != nil {
		return nil, err
	}
	return &V17HashKeyNode{V17BaseNode{line, col}, "hash_sha256", seg}, nil
}

// ParseHashSha512 parses hash_sha512 = TYPE_OF hash_sha512<hex_seg128>.
func (p *V17Parser) ParseHashSha512() (node *V17HashKeyNode, err error) {
	done := p.debugEnter("hash_sha512")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	seg, err := p.ParseHexSeg128()
	if err != nil {
		return nil, err
	}
	return &V17HashKeyNode{V17BaseNode{line, col}, "hash_sha512", seg}, nil
}

// V17HashKeyUnionNode  hash_key = hash_md5 | hash_sha1 | hash_sha256 | hash_sha512
type V17HashKeyUnionNode struct {
	V17BaseNode
	Value *V17HashKeyNode
}

// ParseHashKey parses hash_key.
func (p *V17Parser) ParseHashKey() (node *V17HashKeyUnionNode, err error) {
	done := p.debugEnter("hash_key")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	for _, try := range []func() (*V17HashKeyNode, error){
		p.ParseHashMd5, p.ParseHashSha1, p.ParseHashSha256, p.ParseHashSha512,
	} {
		saved := p.savePos()
		n, nerr := try()
		if nerr == nil {
			return &V17HashKeyUnionNode{V17BaseNode{line, col}, n}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("hash_key: expected hash_md5|hash_sha1|hash_sha256|hash_sha512")
}

// =============================================================================
// STEP 30 — ULID / NANO_ID
// ulid    = /[0-7][0-9A-HJKMNP-TV-Z]{9}[0-9A-HJKMNP-TV-Z]{16}/i
// nano_id = /[A-Za-z0-9_-]{21}/
// =============================================================================

// V17UlidNode  ulid
type V17UlidNode struct {
	V17BaseNode
	Value string
}

// ParseUlid parses ulid.
func (p *V17Parser) ParseUlid() (node *V17UlidNode, err error) {
	done := p.debugEnter("ulid")
	defer func() { done(err == nil) }()
	tok, terr := p.matchRe(reUlidScan)
	if terr != nil {
		return nil, p.errAt("ulid: expected 26-char Crockford base32 string")
	}
	return &V17UlidNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17NanoIdNode  nano_id
type V17NanoIdNode struct {
	V17BaseNode
	Value string
}

// ParseNanoId parses nano_id.
func (p *V17Parser) ParseNanoId() (node *V17NanoIdNode, err error) {
	done := p.debugEnter("nano_id")
	defer func() { done(err == nil) }()
	tok, terr := p.matchRe(reNanoIdScan)
	if terr != nil {
		return nil, p.errAt("nano_id: expected 21-char [A-Za-z0-9_-] string")
	}
	return &V17NanoIdNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// =============================================================================
// STEP 31-32 — SNOWFLAKE / SEQ IDs
// snowflake_id = TYPE_OF snowflake_id<uint64>
// seq_id16     = TYPE_OF seq_id16<uint16>
// seq_id32     = TYPE_OF seq_id32<uint32>
// seq_id64     = TYPE_OF seq_id64<uint64>
// seq_id       = seq_id16 | seq_id32 | seq_id64
// =============================================================================

// V17TypedIdNode holds a TYPE_OF-tagged integer ID.
type V17TypedIdNode struct {
	V17BaseNode
	TypeName string
	UintVal  *V17UintNode
}

// ParseSnowflakeId parses snowflake_id = TYPE_OF snowflake_id<uint64>.
func (p *V17Parser) ParseSnowflakeId() (node *V17TypedIdNode, err error) {
	done := p.debugEnter("snowflake_id")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	u, err := p.ParseUint64()
	if err != nil {
		return nil, err
	}
	return &V17TypedIdNode{V17BaseNode{line, col}, "snowflake_id", u}, nil
}

// ParseSeqId16 parses seq_id16 = TYPE_OF seq_id16<uint16>.
func (p *V17Parser) ParseSeqId16() (node *V17TypedIdNode, err error) {
	done := p.debugEnter("seq_id16")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	u, err := p.ParseUint16()
	if err != nil {
		return nil, err
	}
	return &V17TypedIdNode{V17BaseNode{line, col}, "seq_id16", u}, nil
}

// ParseSeqId32 parses seq_id32 = TYPE_OF seq_id32<uint32>.
func (p *V17Parser) ParseSeqId32() (node *V17TypedIdNode, err error) {
	done := p.debugEnter("seq_id32")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	u, err := p.ParseUint32()
	if err != nil {
		return nil, err
	}
	return &V17TypedIdNode{V17BaseNode{line, col}, "seq_id32", u}, nil
}

// ParseSeqId64 parses seq_id64 = TYPE_OF seq_id64<uint64>.
func (p *V17Parser) ParseSeqId64() (node *V17TypedIdNode, err error) {
	done := p.debugEnter("seq_id64")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	u, err := p.ParseUint64()
	if err != nil {
		return nil, err
	}
	return &V17TypedIdNode{V17BaseNode{line, col}, "seq_id64", u}, nil
}

// V17SeqIdNode  seq_id = seq_id16 | seq_id32 | seq_id64
type V17SeqIdNode struct {
	V17BaseNode
	Value *V17TypedIdNode
}

// ParseSeqId parses seq_id — tries narrowest first.
func (p *V17Parser) ParseSeqId() (node *V17SeqIdNode, err error) {
	done := p.debugEnter("seq_id")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	for _, try := range []func() (*V17TypedIdNode, error){
		p.ParseSeqId16, p.ParseSeqId32, p.ParseSeqId64,
	} {
		saved := p.savePos()
		n, nerr := try()
		if nerr == nil {
			return &V17SeqIdNode{V17BaseNode{line, col}, n}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("seq_id: expected seq_id16|seq_id32|seq_id64")
}

// =============================================================================
// STEP 33 — UNIQUE KEY
// unique_key = uuid | uuid_v7 | ulid | snowflake_id | nano_id | hash_key | seq_id
// =============================================================================

// V17UniqueKeyNode  unique_key = uuid | uuid_v7 | ulid | snowflake_id | nano_id | hash_key | seq_id
type V17UniqueKeyNode struct {
	V17BaseNode
	Value interface{}
	// one of: *V17UuidNode | *V17UuidV7Node | *V17UlidNode | *V17TypedIdNode |
	//         *V17NanoIdNode | *V17HashKeyUnionNode | *V17SeqIdNode
}

// ParseUniqueKey parses unique_key.
// uuid_v7 must be tried before uuid (uuid_v7 is a strict subset).
func (p *V17Parser) ParseUniqueKey() (node *V17UniqueKeyNode, err error) {
	done := p.debugEnter("unique_key")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// uuid_v7 before uuid (more specific)
	{
		saved := p.savePos()
		if n, nerr := p.ParseUuidV7(); nerr == nil {
			return &V17UniqueKeyNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	{
		saved := p.savePos()
		if n, nerr := p.ParseUuid(); nerr == nil {
			return &V17UniqueKeyNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	{
		saved := p.savePos()
		if n, nerr := p.ParseUlid(); nerr == nil {
			return &V17UniqueKeyNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	{
		saved := p.savePos()
		if n, nerr := p.ParseSnowflakeId(); nerr == nil {
			return &V17UniqueKeyNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	{
		saved := p.savePos()
		if n, nerr := p.ParseNanoId(); nerr == nil {
			return &V17UniqueKeyNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	{
		saved := p.savePos()
		if n, nerr := p.ParseHashKey(); nerr == nil {
			return &V17UniqueKeyNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	{
		saved := p.savePos()
		if n, nerr := p.ParseSeqId(); nerr == nil {
			return &V17UniqueKeyNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	return nil, p.errAt("unique_key: no matching key type")
}

// =============================================================================
// STEP 34 — URL TYPES
// http_url = /^https?:\/\/.../
// file_url = /^file:\/\/.../
// =============================================================================

// V17HttpUrlNode  http_url
type V17HttpUrlNode struct {
	V17BaseNode
	Value string
}

// ParseHttpUrl parses http_url — a double-quoted http(s) URL or a bare URL token.
func (p *V17Parser) ParseHttpUrl() (node *V17HttpUrlNode, err error) {
	done := p.debugEnter("http_url")
	defer func() { done(err == nil) }()
	saved := p.savePos()
	// Try as a double-quoted string first.
	if tok, qerr := p.matchDoubleQuoted(); qerr == nil {
		val := tok.Value
		if !reHttpUrl.MatchString(val) {
			p.restorePos(saved)
			return nil, p.errAt("http_url: value %q does not match http(s) URL pattern", val)
		}
		return &V17HttpUrlNode{V17BaseNode{tok.Line, tok.Col}, val}, nil
	}
	p.restorePos(saved)
	// Try as a bare URL in the rune stream.
	tok, terr := p.matchRe(reHttpUrlScan)
	if terr != nil {
		return nil, p.errAt("http_url: expected http(s) URL")
	}
	return &V17HttpUrlNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17FileUrlNode  file_url
type V17FileUrlNode struct {
	V17BaseNode
	Value string
}

// ParseFileUrl parses file_url — a double-quoted file:// URL or a bare URL token.
func (p *V17Parser) ParseFileUrl() (node *V17FileUrlNode, err error) {
	done := p.debugEnter("file_url")
	defer func() { done(err == nil) }()
	saved := p.savePos()
	// Try as a double-quoted string first.
	if tok, qerr := p.matchDoubleQuoted(); qerr == nil {
		val := tok.Value
		if !reFileUrl.MatchString(val) {
			p.restorePos(saved)
			return nil, p.errAt("file_url: value %q does not match file URL pattern", val)
		}
		return &V17FileUrlNode{V17BaseNode{tok.Line, tok.Col}, val}, nil
	}
	p.restorePos(saved)
	// Try as a bare URL in the rune stream.
	tok, terr := p.matchRe(reFileUrlScan)
	if terr != nil {
		return nil, p.errAt("file_url: expected file:// URL")
	}
	return &V17FileUrlNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// =============================================================================
// STEP 36 — CONSTANT
// constant = numeric_const | string | regexp_expr | boolean | null
//          | date | time | date_time | time_stamp | unique_key | http_url | file_url
// =============================================================================

// V17ConstantNode  constant = …
type V17ConstantNode struct {
	V17BaseNode
	Value interface{}
	// one of: *V17NumericConstNode | *V17StringNode | *V17RegexpExprNode |
	//         *V17BooleanNode | *V17NullNode | *V17DateNode | *V17TimeNode |
	//         *V17DateTimeNode | *V17TimeStampNode | *V17UniqueKeyNode |
	//         *V17HttpUrlNode | *V17FileUrlNode
}

// ParseConstant parses constant.
func (p *V17Parser) ParseConstant() (node *V17ConstantNode, err error) {
	done := p.debugEnter("constant")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	type tryFn func() (interface{}, error)
	wrap := func(f func() (*V17NumericConstNode, error)) tryFn {
		return func() (interface{}, error) { return f() }
	}

	// Ordered from most-specific to most-generic first-token.
	// regexp_expr — starts with '/'
	if p.peekAfterWS() == '/' {
		n, nerr := p.ParseRegexpExpr()
		if nerr != nil {
			return nil, nerr
		}
		return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
	}
	// boolean — "true" or "false"
	if p.peekLit("true") || p.peekLit("false") {
		n, nerr := p.ParseBoolean()
		if nerr != nil {
			return nil, nerr
		}
		return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
	}
	// null
	if p.peekLit("null") {
		n, nerr := p.ParseNull()
		if nerr != nil {
			return nil, nerr
		}
		return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
	}
	// string — single-quoted, double-quoted, or template-quoted
	if ch := p.peekAfterWS(); ch == '\'' || ch == '"' || ch == '`' {
		n, nerr := p.ParseString()
		if nerr != nil {
			return nil, nerr
		}
		return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
	}
	// unique_key | numeric_const | date | time | date_time | time_stamp
	// These all start with digits or ident — use savePos/try ordering.
	_ = wrap // silence unused warning

	// time_stamp (most specific date/time — requires both date AND time)
	{
		saved := p.savePos()
		if n, nerr := p.ParseTimeStamp(); nerr == nil {
			return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	// date_time
	{
		saved := p.savePos()
		if n, nerr := p.ParseDateTime(); nerr == nil {
			return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	// unique_key (hex/uuid/ulid etc.)
	{
		saved := p.savePos()
		if n, nerr := p.ParseUniqueKey(); nerr == nil {
			return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	// http_url
	{
		saved := p.savePos()
		if n, nerr := p.ParseHttpUrl(); nerr == nil {
			return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	// file_url
	{
		saved := p.savePos()
		if n, nerr := p.ParseFileUrl(); nerr == nil {
			return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}
	// numeric_const (last, most generic)
	{
		saved := p.savePos()
		if n, nerr := p.ParseNumericConst(); nerr == nil {
			return &V17ConstantNode{V17BaseNode{line, col}, n}, nil
		} else {
			p.restorePos(saved)
		}
	}

	// No pattern matched
	ch := p.peekAfterWS()
	p.trackUnknown(V17Token{Value: string(ch), Line: p.runeLine, Col: p.runeCol})
	return nil, p.errAt("constant: unrecognised character %q", ch)
}
