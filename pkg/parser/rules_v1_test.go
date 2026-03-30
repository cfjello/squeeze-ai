// rules_v1_test.go — Unit tests for the V1 grammar rules (rules_v1.go).
package parser

import (
	"testing"
)

// helper: parse src, skip BOF, apply fn, return node.
func parseWith(src string, fn func(*Parser) *Node) *Node {
	p := parserFor(src)
	p.advance() // skip BOF
	return fn(p)
}

// helper: assert node is non-nil, not an error, and has expected kind.
func assertNode(t *testing.T, n *Node, expected NodeKind) {
	t.Helper()
	if n == nil {
		t.Fatalf("expected %s node, got nil", expected)
	}
	if n.IsError() {
		t.Fatalf("expected %s node, got error node", expected)
	}
	if n.Kind != expected {
		t.Fatalf("expected kind %s, got %s", expected, n.Kind)
	}
}

// =============================================================================
// NL / EOL
// =============================================================================

func TestParseNL(t *testing.T) {
	n := parseWith("\n", func(p *Parser) *Node {
		return p.parseNL()
	})
	assertNode(t, n, NodeToken)
}

func TestParseEOL_Semicolon(t *testing.T) {
	n := parseWith(";", func(p *Parser) *Node {
		return p.parseEOL()
	})
	assertNode(t, n, NodeToken)
}

// =============================================================================
// Numeric primitives
// =============================================================================

func TestParseDigits(t *testing.T) {
	n := parseWith("42", func(p *Parser) *Node {
		return p.parseDigits()
	})
	assertNode(t, n, NodeDigits)
	if n.Tok != nil {
		t.Error("digits node should not have a direct Tok field set (uses children)")
	}
	if len(n.Children) == 0 || n.Children[0].Tok == nil || n.Children[0].Tok.Value != "42" {
		t.Fatalf("digits: unexpected children: %v", n.Children)
	}
}

func TestParseInteger_Unsigned(t *testing.T) {
	n := parseWith("99", func(p *Parser) *Node {
		return p.parseInteger()
	})
	assertNode(t, n, NodeInteger)
}

func TestParseInteger_Positive(t *testing.T) {
	n := parseWith("+7", func(p *Parser) *Node {
		return p.parseInteger()
	})
	assertNode(t, n, NodeInteger)
	// first child should be the "+" token, second the digits
	if len(n.Children) != 2 {
		t.Fatalf("expected 2 children for +7, got %d", len(n.Children))
	}
}

func TestParseInteger_Negative(t *testing.T) {
	n := parseWith("-3", func(p *Parser) *Node {
		return p.parseInteger()
	})
	assertNode(t, n, NodeInteger)
}

func TestParseInteger_MismatchReturnsNil(t *testing.T) {
	n := parseWith("hello", func(p *Parser) *Node {
		return p.parseInteger()
	})
	if n != nil {
		t.Fatal("expected nil for non-integer input")
	}
}

func TestParseDecimal(t *testing.T) {
	n := parseWith("3.14", func(p *Parser) *Node {
		return p.parseDecimal()
	})
	assertNode(t, n, NodeDecimal)
}

func TestParseDecimal_Negative(t *testing.T) {
	n := parseWith("-0.5", func(p *Parser) *Node {
		return p.parseDecimal()
	})
	assertNode(t, n, NodeDecimal)
}

func TestParseNumericConst_Integer(t *testing.T) {
	n := parseWith("100", func(p *Parser) *Node {
		return p.parseNumericConst()
	})
	assertNode(t, n, NodeNumericConst)
}

func TestParseNumericConst_Decimal(t *testing.T) {
	n := parseWith("1.5", func(p *Parser) *Node {
		return p.parseNumericConst()
	})
	assertNode(t, n, NodeNumericConst)
}

func TestParseNumericConst_NegativeDecimal(t *testing.T) {
	n := parseWith("-2.7", func(p *Parser) *Node {
		return p.parseNumericConst()
	})
	assertNode(t, n, NodeNumericConst)
}

// =============================================================================
// Strings
// =============================================================================

func TestParseStringSingleQuoted(t *testing.T) {
	n := parseWith(`'hello'`, func(p *Parser) *Node {
		return p.parseString()
	})
	assertNode(t, n, NodeString)
	if n.Children[0].Kind != NodeSingleQuoted {
		t.Fatalf("expected NodeSingleQuoted child, got %s", n.Children[0].Kind)
	}
}

func TestParseStringDoubleQuoted(t *testing.T) {
	n := parseWith(`"world"`, func(p *Parser) *Node {
		return p.parseString()
	})
	assertNode(t, n, NodeString)
	if n.Children[0].Kind != NodeDoubleQuoted {
		t.Fatalf("expected NodeDoubleQuoted child, got %s", n.Children[0].Kind)
	}
}

func TestParseStringTemplateQuoted(t *testing.T) {
	n := parseWith("`template`", func(p *Parser) *Node {
		return p.parseString()
	})
	assertNode(t, n, NodeString)
	if n.Children[0].Kind != NodeTmplQuoted {
		t.Fatalf("expected NodeTmplQuoted child, got %s", n.Children[0].Kind)
	}
}

func TestParseStringMismatch(t *testing.T) {
	n := parseWith("42", func(p *Parser) *Node { return p.parseString() })
	if n != nil {
		t.Fatal("expected nil for non-string input")
	}
}

// =============================================================================
// Regexp
// =============================================================================

func TestParseRegexp(t *testing.T) {
	n := parseWith("/hello/gi", func(p *Parser) *Node {
		return p.parseRegexp()
	})
	assertNode(t, n, NodeRegexp)
	if n.Children[0].Tok == nil || n.Children[0].Tok.Value != "/hello/gi" {
		t.Fatalf("regexp token mismatch: %+v", n.Children[0].Tok)
	}
}

func TestParseRegexpMismatch(t *testing.T) {
	n := parseWith("42", func(p *Parser) *Node { return p.parseRegexp() })
	if n != nil {
		t.Fatal("expected nil for non-regexp input")
	}
}

// =============================================================================
// Boolean
// =============================================================================

func TestParseBooleanTrue(t *testing.T) {
	n := parseWith("true", func(p *Parser) *Node { return p.parseBoolean() })
	assertNode(t, n, NodeBoolean)
	if n.Children[0].Kind != NodeBooleanTrue {
		t.Fatalf("expected NodeBooleanTrue child, got %s", n.Children[0].Kind)
	}
}

func TestParseBooleanFalse(t *testing.T) {
	n := parseWith("false", func(p *Parser) *Node { return p.parseBoolean() })
	assertNode(t, n, NodeBoolean)
	if n.Children[0].Kind != NodeBooleanFalse {
		t.Fatalf("expected NodeBooleanFalse child, got %s", n.Children[0].Kind)
	}
}

// =============================================================================
// Constant
// =============================================================================

func TestParseConstantInteger(t *testing.T) {
	n := parseWith("7", func(p *Parser) *Node { return p.parseConstant() })
	assertNode(t, n, NodeConstant)
}

func TestParseConstantString(t *testing.T) {
	n := parseWith(`"hi"`, func(p *Parser) *Node { return p.parseConstant() })
	assertNode(t, n, NodeConstant)
}

func TestParseConstantBool(t *testing.T) {
	n := parseWith("true", func(p *Parser) *Node { return p.parseConstant() })
	assertNode(t, n, NodeConstant)
}

func TestParseConstantRegexp(t *testing.T) {
	// Regexp literals are only emitted when not preceded by a value token;
	// since we skip BOF manually, the first real token after skipBOF will
	// be a regexp (not division).
	n := parseWith("/abc/", func(p *Parser) *Node { return p.parseConstant() })
	assertNode(t, n, NodeConstant)
}

// =============================================================================
// Identifiers
// =============================================================================

func TestParseIdentName(t *testing.T) {
	n := parseWith("myVar", func(p *Parser) *Node { return p.parseIdentName() })
	assertNode(t, n, NodeIdentName)
}

func TestParseIdentNameWithSpaces(t *testing.T) {
	n := parseWith("my var", func(p *Parser) *Node { return p.parseIdentName() })
	assertNode(t, n, NodeIdentName)
	if n.Children[0].Tok.Value != "my var" {
		t.Fatalf("expected 'my var', got %q", n.Children[0].Tok.Value)
	}
}

func TestParseIdentDotted_Single(t *testing.T) {
	n := parseWith("foo", func(p *Parser) *Node { return p.parseIdentDotted() })
	assertNode(t, n, NodeIdentDotted)
}

func TestParseIdentDotted_Multi(t *testing.T) {
	n := parseWith("a.b.c", func(p *Parser) *Node { return p.parseIdentDotted() })
	assertNode(t, n, NodeIdentDotted)
	// children: ident, dot, ident, dot, ident
	if len(n.Children) != 5 {
		t.Fatalf("expected 5 children for a.b.c, got %d", len(n.Children))
	}
}

func TestParseIdentRef(t *testing.T) {
	n := parseWith("pkg.field", func(p *Parser) *Node { return p.parseIdentRef() })
	assertNode(t, n, NodeIdentRef)
}

// =============================================================================
// Numeric expressions
// =============================================================================

func TestParseNumericOper(t *testing.T) {
	// Most operators can be scanned at the start of input without issues.
	for _, op := range []string{"+", "-", "*", "**", "%"} {
		n := parseWith(op+" 1", func(p *Parser) *Node { return p.parseNumericOper() })
		if n == nil || n.Kind != NodeNumericOper {
			t.Errorf("parseNumericOper returned nil/wrong kind for %q", op)
		}
	}
	// '/' requires a preceding value token so the lexer emits TOK_SLASH (not a
	// regexp). Provide full context "1 / 2" and skip past the leading integer.
	{
		toks, err := NewLexer("1 / 2").Tokenize()
		if err != nil {
			t.Fatalf("lex error for '/': %v", err)
		}
		p := NewParser(toks)
		p.advance() // BOF
		p.advance() // integer "1"
		n := p.parseNumericOper()
		if n == nil || n.Kind != NodeNumericOper {
			t.Errorf("parseNumericOper returned nil/wrong for '/'")
		}
	}
}

func TestParseInlineIncr(t *testing.T) {
	incr := parseWith("++", func(p *Parser) *Node { return p.parseInlineIncr() })
	assertNode(t, incr, NodeInlineIncr)
	decr := parseWith("--", func(p *Parser) *Node { return p.parseInlineIncr() })
	assertNode(t, decr, NodeInlineIncr)
}

func TestParseSingleNumExpr_Literal(t *testing.T) {
	n := parseWith("42", func(p *Parser) *Node { return p.parseSingleNumExpr() })
	assertNode(t, n, NodeSingleNumExpr)
}

func TestParseSingleNumExpr_WithIncr(t *testing.T) {
	n := parseWith("++3", func(p *Parser) *Node { return p.parseSingleNumExpr() })
	assertNode(t, n, NodeSingleNumExpr)
}

func TestParseSingleNumExpr_IdentRef(t *testing.T) {
	// TYPE_OF numeric_const<ident_ref> — user writes just an ident
	n := parseWith("myNum", func(p *Parser) *Node { return p.parseSingleNumExpr() })
	// directive node wrapping NodeSingleNumExpr
	if n == nil {
		t.Fatal("expected non-nil node for ident in numeric expr")
	}
	if n.Kind != NodeDirectiveTypeOf {
		t.Fatalf("expected NodeDirectiveTypeOf, got %s", n.Kind)
	}
	if n.Meta.DirectiveArg != "numeric_const" {
		t.Fatalf("expected directive arg 'numeric_const', got %q", n.Meta.DirectiveArg)
	}
}

func TestParseNumExprList_Simple(t *testing.T) {
	n := parseWith("1 + 2", func(p *Parser) *Node { return p.parseNumExprList() })
	assertNode(t, n, NodeNumExprList)
}

func TestParseNumExprList_Multi(t *testing.T) {
	n := parseWith("3 * 4 - 1", func(p *Parser) *Node { return p.parseNumExprList() })
	assertNode(t, n, NodeNumExprList)
	// children: singleExpr, oper, singleExpr, oper, singleExpr
	if len(n.Children) != 5 {
		t.Fatalf("expected 5 children for '3 * 4 - 1', got %d: %v",
			len(n.Children), n.Children)
	}
}

func TestParseNumGrouping(t *testing.T) {
	n := parseWith("(2 + 3)", func(p *Parser) *Node { return p.parseNumGrouping() })
	assertNode(t, n, NodeNumGrouping)
}

func TestParseNumericExpr_List(t *testing.T) {
	n := parseWith("5 ** 2", func(p *Parser) *Node { return p.parseNumericExpr() })
	assertNode(t, n, NodeNumericExpr)
}

func TestParseNumericExpr_Grouping(t *testing.T) {
	n := parseWith("(10 / 2)", func(p *Parser) *Node { return p.parseNumericExpr() })
	assertNode(t, n, NodeNumericExpr)
}

// =============================================================================
// String expressions
// =============================================================================

func TestParseStringExprList(t *testing.T) {
	n := parseWith(`"a" + "b"`, func(p *Parser) *Node { return p.parseStringExprList() })
	assertNode(t, n, NodeStringExprList)
}

func TestParseStringExprList_ThreeItems(t *testing.T) {
	n := parseWith(`"a" + "b" + "c"`, func(p *Parser) *Node { return p.parseStringExprList() })
	assertNode(t, n, NodeStringExprList)
	// children: str, +, str, +, str
	if len(n.Children) != 5 {
		t.Fatalf("expected 5 children, got %d", len(n.Children))
	}
}

func TestParseStringExprList_SingleStringReturnsNil(t *testing.T) {
	// A single string is NOT a string_expr_list (requires at least "+")
	n := parseWith(`"hello"`, func(p *Parser) *Node { return p.parseStringExprList() })
	if n != nil {
		t.Fatal("single string should return nil from parseStringExprList")
	}
}

func TestParseStringExpr(t *testing.T) {
	n := parseWith(`"x" + "y"`, func(p *Parser) *Node { return p.parseStringExpr() })
	assertNode(t, n, NodeStringExpr)
}

// =============================================================================
// Compare expressions
// =============================================================================

func TestParseCompareOper(t *testing.T) {
	opers := []string{"!=", "=", ">", ">=", "<="}
	for _, op := range opers {
		src := op + " 1"
		p := parserFor(src)
		p.advance()           // skip BOF
		p.inExpression = true // needed for '<' but not for these
		n := p.parseCompareOper()
		if n == nil {
			t.Errorf("parseCompareOper returned nil for %q", op)
		}
	}
}

func TestParseCompareOperLtInExpression(t *testing.T) {
	p := parserFor("< 5")
	p.advance()
	p.inExpression = true
	n := p.parseCompareOper()
	if n == nil || n.Children[0].Tok.Value != "<" {
		t.Fatalf("parseCompareOper failed for '<' in expression context")
	}
}

func TestParseNumCompare(t *testing.T) {
	n := parseWith("3 > 2", func(p *Parser) *Node { return p.parseNumCompare() })
	if n == nil {
		t.Fatal("parseNumCompare returned nil for '3 > 2'")
	}
	if n.Kind != NodeDirectiveTypeOf {
		t.Fatalf("expected NodeDirectiveTypeOf, got %s", n.Kind)
	}
	if n.Meta.DirectiveArg != "boolean" {
		t.Fatalf("expected directive arg 'boolean', got %q", n.Meta.DirectiveArg)
	}
	if len(n.Children) != 1 || n.Children[0].Kind != NodeNumCompare {
		t.Fatalf("expected NodeNumCompare child")
	}
}

func TestParseStringCompare(t *testing.T) {
	// string_expr requires at least two strings joined by '+' on each side.
	n := parseWith(`"a" + "b" != "c" + "d"`, func(p *Parser) *Node {
		return p.parseStringCompare()
	})
	if n == nil {
		t.Fatal("parseStringCompare returned nil")
	}
	if n.Kind != NodeDirectiveTypeOf {
		t.Fatalf("expected NodeDirectiveTypeOf, got %s", n.Kind)
	}
}

func TestParseStringRegexpComp(t *testing.T) {
	n := parseWith(`"hello" =~ /ell/`, func(p *Parser) *Node {
		return p.parseStringRegexpComp()
	})
	if n == nil {
		t.Fatal("parseStringRegexpComp returned nil")
	}
	if n.Kind != NodeDirectiveTypeOf {
		t.Fatalf("expected NodeDirectiveTypeOf, got %s", n.Kind)
	}
}

func TestParseCompareExpr_Dispatch(t *testing.T) {
	cases := []struct {
		src  string
		desc string
	}{
		{"3 > 2", "num compare"},
		// string_compare requires string_expr (2+ strings) on each side:
		{`"a" + "b" != "c" + "d"`, "string compare"},
		{`"x" =~ /y/`, "string regexp compare"},
	}
	for _, tc := range cases {
		n := parseWith(tc.src, func(p *Parser) *Node { return p.parseCompareExpr() })
		if n == nil {
			t.Errorf("%s (%q): expected compare_expr node, got nil", tc.desc, tc.src)
			continue
		}
		assertNode(t, n, NodeCompareExpr)
	}
}

// =============================================================================
// Logic expressions
// =============================================================================

func TestParseNotOper(t *testing.T) {
	n := parseWith("!", func(p *Parser) *Node { return p.parseNotOper() })
	assertNode(t, n, NodeNotOper)
}

func TestParseLogicOper(t *testing.T) {
	for _, op := range []string{"&", "|", "^"} {
		n := parseWith(op, func(p *Parser) *Node { return p.parseLogicOper() })
		if n == nil || n.Kind != NodeLogicOper {
			t.Errorf("parseLogicOper failed for %q", op)
		}
	}
}

func TestParseSingleLogicExpr_BoolIdent(t *testing.T) {
	n := parseWith("myFlag", func(p *Parser) *Node { return p.parseSingleLogicExpr() })
	assertNode(t, n, NodeSingleLogicExpr)
}

func TestParseSingleLogicExpr_Negated(t *testing.T) {
	n := parseWith("!myFlag", func(p *Parser) *Node { return p.parseSingleLogicExpr() })
	assertNode(t, n, NodeSingleLogicExpr)
	if len(n.Children) != 2 {
		t.Fatalf("negated: expected 2 children (not_oper + ident), got %d", len(n.Children))
	}
	if n.Children[0].Kind != NodeNotOper {
		t.Fatalf("expected NodeNotOper as first child, got %s", n.Children[0].Kind)
	}
}

func TestParseSingleLogicExpr_NumericLiteral(t *testing.T) {
	n := parseWith("42", func(p *Parser) *Node { return p.parseSingleLogicExpr() })
	assertNode(t, n, NodeSingleLogicExpr)
}

func TestParseLogicExprList(t *testing.T) {
	n := parseWith("a & b", func(p *Parser) *Node { return p.parseLogicExprList() })
	assertNode(t, n, NodeLogicExprList)
}

func TestParseLogicGrouping(t *testing.T) {
	n := parseWith("(a | b)", func(p *Parser) *Node { return p.parseLogicGrouping() })
	assertNode(t, n, NodeLogicGrouping)
}

func TestParseLogicExpr_List(t *testing.T) {
	n := parseWith("x ^ y", func(p *Parser) *Node { return p.parseLogicExpr() })
	assertNode(t, n, NodeLogicExpr)
}

// =============================================================================
// Assignment operators
// =============================================================================

func TestParseAssignOperAll(t *testing.T) {
	cases := []struct {
		src  string
		want NodeKind
	}{
		{"+:", NodeIncrAssignImmutable},
		{"-:", NodeIncrAssignImmutable},
		{"*:", NodeIncrAssignImmutable},
		{"/:", NodeIncrAssignImmutable},
		{"+~", NodeIncrAssignMutable},
		{"-~", NodeIncrAssignMutable},
		{"*~", NodeIncrAssignMutable},
		{"/~", NodeIncrAssignMutable},
		{"~", NodeAssignMutable},
		{":", NodeAssignImmutable},
		{":~", NodeAssignReadOnlyRef},
	}
	for _, tc := range cases {
		n := parseWith(tc.src+" identifier", func(p *Parser) *Node {
			return p.parseAssignOper()
		})
		if n == nil {
			t.Errorf("parseAssignOper returned nil for %q", tc.src)
			continue
		}
		// parseAssignOper always returns NodeAssignOper as the top node.
		assertNode(t, n, NodeAssignOper)
		// The direct child is either the incremental node (for +: etc.) OR
		// NodeEqualAssign (which further wraps NodeAssignMutable/Immutable/ReadOnly).
		child := n.Children[0]
		switch tc.want {
		case NodeIncrAssignImmutable, NodeIncrAssignMutable:
			if child.Kind != tc.want {
				t.Errorf("%q: expected child %s, got %s", tc.src, tc.want, child.Kind)
			}
		default:
			// equal_assign wraps the specific operator one level deeper.
			if child.Kind != NodeEqualAssign {
				t.Errorf("%q: expected NodeEqualAssign child, got %s", tc.src, child.Kind)
				continue
			}
			if len(child.Children) == 0 || child.Children[0].Kind != tc.want {
				t.Errorf("%q: expected equal_assign inner child %s, got %v",
					tc.src, tc.want, child.Children)
			}
		}
	}
}

func TestParseEqualAssign(t *testing.T) {
	for _, src := range []string{"~", ":", ":~"} {
		n := parseWith(src, func(p *Parser) *Node { return p.parseEqualAssign() })
		assertNode(t, n, NodeEqualAssign)
	}
}

// =============================================================================
// Range
// =============================================================================

func TestParseRange_Digits(t *testing.T) {
	n := parseWith("1..128", func(p *Parser) *Node {
		return p.try(p.parseRange)
	})
	assertNode(t, n, NodeRange)
	// children: lo(digits), dotdot, hi(digits)
	if len(n.Children) != 3 {
		t.Fatalf("expected 3 children for range, got %d", len(n.Children))
	}
}

func TestParseRange_Many(t *testing.T) {
	n := parseWith("0..many", func(p *Parser) *Node {
		return p.try(p.parseRange)
	})
	assertNode(t, n, NodeRange)
}

func TestParseRange_LowercaseM(t *testing.T) {
	n := parseWith("5..m", func(p *Parser) *Node {
		return p.try(p.parseRange)
	})
	assertNode(t, n, NodeRange)
}

func TestParseRangeNotARange(t *testing.T) {
	// Just a plain integer — should return nil via backtracking.
	n := parseWith("42", func(p *Parser) *Node {
		return p.try(p.parseRange)
	})
	if n != nil {
		t.Fatal("expected nil for plain integer, not a range")
	}
}
