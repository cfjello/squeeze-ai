// parser_v17_operators.go — ParseXxx methods for spec/02_operators.sqg.
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
	"regexp"
)

// =============================================================================
// IDENTIFIERS
// ident_name   = /(?<value>[\p{L}][\p{L}0-9_]*)/
// group_begin  = "("
// group_end    = ")"
// ident_dotted = ( ident_name ) { "." ident_name }
// ident_prefix = ( "../" { "../" } ) | ( "./" )
// ident_ref    = [ ident_prefix ] ident_dotted
// =============================================================================

var reIdentName = regexp.MustCompile(`^[\p{L}][\p{L}0-9_]*$`)

// V17IdentNameNode  ident_name = /(?<value>[\p{L}][\p{L}0-9_]*)/
type V17IdentNameNode struct {
	V17BaseNode
	Value string
}

// ParseIdentName parses ident_name = /(?<value>[\p{L}][\p{L}0-9_]*)/
func (p *V17Parser) ParseIdentName() (node *V17IdentNameNode, err error) {
	done := p.debugEnter("ident_name")
	defer func() { done(err == nil) }()
	tok, terr := p.matchRe(reIdentNameScan)
	if terr != nil {
		return nil, p.errAt("ident_name: expected identifier")
	}
	return &V17IdentNameNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17GroupBeginNode  group_begin = "("
type V17GroupBeginNode struct{ V17BaseNode }

// ParseGroupBegin parses group_begin = "("
func (p *V17Parser) ParseGroupBegin() (node *V17GroupBeginNode, err error) {
	done := p.debugEnter("group_begin")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("(")
	if err != nil {
		return nil, fmt.Errorf("group_begin: %w", err)
	}
	return &V17GroupBeginNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17GroupEndNode  group_end = ")"
type V17GroupEndNode struct{ V17BaseNode }

// ParseGroupEnd parses group_end = ")"
func (p *V17Parser) ParseGroupEnd() (node *V17GroupEndNode, err error) {
	done := p.debugEnter("group_end")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit(")")
	if err != nil {
		return nil, fmt.Errorf("group_end: %w", err)
	}
	return &V17GroupEndNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17IdentDottedNode  ident_dotted = ( ident_name ) { "." ident_name }
type V17IdentDottedNode struct {
	V17BaseNode
	Parts []string // one or more name segments
}

// ParseIdentDotted parses ident_dotted = ( ident_name ) { "." ident_name }
func (p *V17Parser) ParseIdentDotted() (node *V17IdentDottedNode, err error) {
	done := p.debugEnter("ident_dotted")
	defer func() { done(err == nil) }()
	first, err := p.ParseIdentName()
	if err != nil {
		return nil, fmt.Errorf("ident_dotted: %w", err)
	}
	line, col := first.Line, first.Col
	parts := []string{first.Value}
	for p.peekAfterWS() == '.' {
		saved := p.savePos()
		if _, merr := p.matchLit("."); merr != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.ParseIdentName()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		parts = append(parts, next.Value)
	}
	return &V17IdentDottedNode{V17BaseNode{line, col}, parts}, nil
}

// V17IdentPrefixNode  ident_prefix = ( "../" { "../" } ) | ( "./" )
type V17IdentPrefixNode struct {
	V17BaseNode
	Value string // the full prefix string, e.g. "../../" or "./"
}

// ParseIdentPrefix parses ident_prefix = ( "../" { "../" } ) | ( "./" )
func (p *V17Parser) ParseIdentPrefix() (node *V17IdentPrefixNode, err error) {
	done := p.debugEnter("ident_prefix")
	defer func() { done(err == nil) }()
	saved := p.savePos()
	// Try "../"
	if tok, terr := p.matchLit("../"); terr == nil {
		line, col := tok.Line, tok.Col
		prefix := "../"
		for p.peekLit("../") {
			if _, err2 := p.matchLit("../"); err2 != nil {
				break
			}
			prefix += "../"
		}
		return &V17IdentPrefixNode{V17BaseNode{line, col}, prefix}, nil
	}
	p.restorePos(saved)
	// Try "./"
	tok2, terr2 := p.matchLit("./")
	if terr2 != nil {
		p.restorePos(saved)
		return nil, p.errAt("ident_prefix: expected '../' or './'")
	}
	return &V17IdentPrefixNode{V17BaseNode{tok2.Line, tok2.Col}, "./"}, nil
}

// V17IdentRefNode  ident_ref = [ ident_prefix ] ident_dotted
// EXTEND<ident_ref> = [ json_path ]     (05_json_path.sqg)
// PREFIX<ident_ref> = __ps_token_ref |  (30_bootstrap.sqg)
// ident_ref = __ps_token_ref | ( [ ident_prefix ] ident_dotted [ json_path ] )
type V17IdentRefNode struct {
	V17BaseNode
	Prefix     *V17IdentPrefixNode // nil when PsTokenRef is set
	Dotted     *V17IdentDottedNode // nil when PsTokenRef is set
	JsonPath   *V17JsonPathNode    // nil when no json_path follows
	PsTokenRef *V17PsTokenRefNode  // non-nil when ident_ref is a §token reference
}

// ParseIdentRef parses ident_ref = __ps_token_ref | ( [ ident_prefix ] ident_dotted [ json_path ] )
// PREFIX<ident_ref> = __ps_token_ref |  (30_bootstrap.sqg)
// EXTEND<ident_ref> = [ json_path ]     (05_json_path.sqg)
func (p *V17Parser) ParseIdentRef() (node *V17IdentRefNode, err error) {
	done := p.debugEnter("ident_ref")
	defer func() { done(err == nil) }()

	// PREFIX<ident_ref> = __ps_token_ref |  — "§" is unambiguous; tried first.
	if p.peekLit("§") {
		line, col := p.runeLine, p.runeCol
		tr, trerr := p.ParsePsTokenRef()
		if trerr != nil {
			return nil, fmt.Errorf("ident_ref: %w", trerr)
		}
		return &V17IdentRefNode{V17BaseNode{line, col}, nil, nil, nil, tr}, nil
	}

	var prefix *V17IdentPrefixNode
	if p.peekAfterWS() == '.' {
		saved := p.savePos()
		if pn, perr := p.ParseIdentPrefix(); perr == nil {
			prefix = pn
		} else {
			p.restorePos(saved)
		}
	}

	dotted, derr := p.ParseIdentDotted()
	if derr != nil {
		return nil, fmt.Errorf("ident_ref: %w", derr)
	}
	line, col := dotted.Line, dotted.Col
	if prefix != nil {
		line, col = prefix.Line, prefix.Col
	}
	node = &V17IdentRefNode{V17BaseNode{line, col}, prefix, dotted, nil, nil}
	// EXTEND<ident_ref> = [ json_path ]  (05_json_path.sqg)
	if saved := p.savePos(); true {
		if jp, jperr := p.ParseJsonPath(); jperr == nil {
			node.JsonPath = jp
		} else {
			p.restorePos(saved)
		}
	}
	return node, nil
}

// =============================================================================
// NUMERIC OPERATORS
// numeric_oper = "+" | "-" | "*"  | "**" | "/" | "%"
// inline_incr  = "++" | "--"
// =============================================================================

// V17NumericOperNode  numeric_oper = "+" | "-" | "*" | "**" | "/" | "%"
type V17NumericOperNode struct {
	V17BaseNode
	Op string
}

// ParseNumericOper parses numeric_oper
func (p *V17Parser) ParseNumericOper() (node *V17NumericOperNode, err error) {
	done := p.debugEnter("numeric_oper")
	defer func() { done(err == nil) }()
	switch {
	case p.peekLit("**"):
		tok, _ := p.matchLit("**")
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "**"}, nil
	case p.peekAfterWS() == '+' && !p.peekLit("++"):
		tok, _ := p.matchLit("+")
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "+"}, nil
	case p.peekAfterWS() == '-' && !p.peekLit("--"):
		tok, _ := p.matchLit("-")
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "-"}, nil
	case p.peekAfterWS() == '*':
		tok, _ := p.matchLit("*")
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "*"}, nil
	case p.peekAfterWS() == '/':
		tok, _ := p.matchLit("/")
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "/"}, nil
	case p.peekAfterWS() == '%':
		tok, _ := p.matchLit("%")
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "%"}, nil
	}
	return nil, p.errAt("numeric_oper: expected +|-|*|**|/|%%")
}

// V17InlineIncrNode  inline_incr = "++" | "--"
type V17InlineIncrNode struct {
	V17BaseNode
	Op string
}

// ParseInlineIncr parses inline_incr = "++" | "--"
func (p *V17Parser) ParseInlineIncr() (node *V17InlineIncrNode, err error) {
	done := p.debugEnter("inline_incr")
	defer func() { done(err == nil) }()
	switch {
	case p.peekLit("++"):
		tok, _ := p.matchLit("++")
		return &V17InlineIncrNode{V17BaseNode{tok.Line, tok.Col}, "++"}, nil
	case p.peekLit("--"):
		tok, _ := p.matchLit("--")
		return &V17InlineIncrNode{V17BaseNode{tok.Line, tok.Col}, "--"}, nil
	}
	return nil, p.errAt("inline_incr: expected ++ or --")
}

// =============================================================================
// NUMERIC EXPRESSIONS
// single_num_expr = numeric_const | [ inline_incr ] TYPE_OF numeric_const<ident_ref>
// num_expr_chain  = single_num_expr { numeric_oper single_num_expr }
// num_grouping    = group_begin ( num_expr_chain | num_grouping )
//                   { numeric_oper ( num_expr_chain | num_grouping ) } group_end
// numeric_calc    = num_expr_chain | num_grouping
// =============================================================================

// V17SingleNumExprNode  single_num_expr
// single_num_expr = [ inline_incr ] ( numeric_const | TYPE_OF numeric_const<ident_ref> ) [ inline_incr ]
type V17SingleNumExprNode struct {
	V17BaseNode
	// Value is one of:
	//   *V17NumericConstNode           — literal numeric constant
	//   *V17IdentRefNode               — ident_ref with TYPE_OF numeric_const check
	Value    interface{}
	PrefixOp *V17InlineIncrNode // nil when no prefix ++ or --
	SuffixOp *V17InlineIncrNode // nil when no suffix ++ or --
}

// ParseSingleNumExpr parses single_num_expr
// single_num_expr = [ inline_incr ] ( numeric_const | TYPE_OF numeric_const<ident_ref> ) [ inline_incr ]
func (p *V17Parser) ParseSingleNumExpr() (node *V17SingleNumExprNode, err error) {
	done := p.debugEnter("single_num_expr")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Try numeric_const first (no prefix or suffix)
	if saved := p.savePos(); true {
		if nc, nerr := p.ParseNumericConst(); nerr == nil {
			// trailing [ inline_incr ] after a bare numeric literal is unusual but allowed
			var suffix *V17InlineIncrNode
			if p.peekLit("++") || p.peekLit("--") {
				if sx, sxerr := p.ParseInlineIncr(); sxerr == nil {
					suffix = sx
				}
			}
			return &V17SingleNumExprNode{V17BaseNode{line, col}, nc, nil, suffix}, nil
		}
		p.restorePos(saved)
	}

	// Try [ inline_incr ] TYPE_OF numeric_const<ident_ref> [ inline_incr ]
	var prefix *V17InlineIncrNode
	if p.peekLit("++") || p.peekLit("--") {
		saved := p.savePos()
		in, ierr := p.ParseInlineIncr()
		if ierr != nil {
			p.restorePos(saved)
		} else {
			prefix = in
		}
	}
	ref, rerr := p.ParseIdentRef()
	if rerr != nil {
		return nil, fmt.Errorf("single_num_expr: expected numeric_const or ident_ref, got %w", rerr)
	}
	// trailing [ inline_incr ]
	var suffix *V17InlineIncrNode
	if p.peekLit("++") || p.peekLit("--") {
		if sx, sxerr := p.ParseInlineIncr(); sxerr == nil {
			suffix = sx
		}
	}
	return &V17SingleNumExprNode{V17BaseNode{line, col}, ref, prefix, suffix}, nil
}

// V17NumExprChainNode  num_expr_chain = single_num_expr { numeric_oper single_num_expr }
type V17NumExprChainNode struct {
	V17BaseNode
	Items []interface{} // alternating: *V17SingleNumExprNode, *V17NumericOperNode, …
}

// ParseNumExprChain parses num_expr_chain
func (p *V17Parser) ParseNumExprChain() (node *V17NumExprChainNode, err error) {
	done := p.debugEnter("num_expr_chain")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseSingleNumExpr()
	if ferr != nil {
		return nil, fmt.Errorf("num_expr_chain: %w", ferr)
	}
	items := []interface{}{first}
	for {
		saved := p.savePos()
		op, oerr := p.ParseNumericOper()
		if oerr != nil {
			p.restorePos(saved)
			break
		}
		rhs, rerr := p.ParseSingleNumExpr()
		if rerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, op, rhs)
	}
	return &V17NumExprChainNode{V17BaseNode{line, col}, items}, nil
}

// V17NumGroupingNode  num_grouping = group_begin ( num_expr_chain | num_grouping )
//
//	{ numeric_oper ( num_expr_chain | num_grouping ) } group_end
type V17NumGroupingNode struct {
	V17BaseNode
	Items []interface{} // alternating: expr/grouping, *V17NumericOperNode, …
}

// ParseNumGrouping parses num_grouping (recursive)
func (p *V17Parser) ParseNumGrouping() (node *V17NumGroupingNode, err error) {
	done := p.debugEnter("num_grouping")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.ParseGroupBegin(); err != nil {
		return nil, fmt.Errorf("num_grouping: %w", err)
	}

	// first inner expr
	first, ferr := p.parseNumGroupingItem()
	if ferr != nil {
		return nil, fmt.Errorf("num_grouping: %w", ferr)
	}
	items := []interface{}{first}
	for {
		saved := p.savePos()
		op, oerr := p.ParseNumericOper()
		if oerr != nil {
			p.restorePos(saved)
			break
		}
		rhs, rerr := p.parseNumGroupingItem()
		if rerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, op, rhs)
	}
	if _, err = p.ParseGroupEnd(); err != nil {
		return nil, fmt.Errorf("num_grouping: %w", err)
	}
	return &V17NumGroupingNode{V17BaseNode{line, col}, items}, nil
}

// parseNumGroupingItem tries num_expr_chain then num_grouping.
func (p *V17Parser) parseNumGroupingItem() (interface{}, error) {
	if p.peekAfterWS() == '(' {
		return p.ParseNumGrouping()
	}
	return p.ParseNumExprChain()
}

// V17NumericCalcNode  numeric_calc = num_expr_chain | num_grouping
type V17NumericCalcNode struct {
	V17BaseNode
	// Value is one of: *V17NumExprChainNode | *V17NumGroupingNode
	Value interface{}
}

// ParseNumericCalc parses numeric_calc
func (p *V17Parser) ParseNumericCalc() (node *V17NumericCalcNode, err error) {
	done := p.debugEnter("numeric_calc")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() == '(' {
		g, gerr := p.ParseNumGrouping()
		if gerr != nil {
			return nil, fmt.Errorf("numeric_calc: %w", gerr)
		}
		return &V17NumericCalcNode{V17BaseNode{line, col}, g}, nil
	}
	c, cerr := p.ParseNumExprChain()
	if cerr != nil {
		return nil, fmt.Errorf("numeric_calc: %w", cerr)
	}
	return &V17NumericCalcNode{V17BaseNode{line, col}, c}, nil
}

// =============================================================================
// STRING OPERATORS
// string_oper       = "+"
// string_expr_chain = string { string_oper string }
// string_grouping   = group_begin ( string_expr_chain | string_grouping )
//                     { string_oper ( string_expr_chain | string_grouping ) } group_end
// string_concat     = string_expr_chain | string_grouping
// =============================================================================

// V17StringOperNode  string_oper = "+"
type V17StringOperNode struct{ V17BaseNode }

// ParseStringOper parses string_oper = "+"
func (p *V17Parser) ParseStringOper() (node *V17StringOperNode, err error) {
	done := p.debugEnter("string_oper")
	defer func() { done(err == nil) }()
	if p.peekLit("++") {
		return nil, p.errAt("string_oper: '++' is inline_incr, not string_oper")
	}
	tok, err := p.matchLit("+")
	if err != nil {
		return nil, fmt.Errorf("string_oper: %w", err)
	}
	return &V17StringOperNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17StringExprChainNode  string_expr_chain = string { string_oper string }
type V17StringExprChainNode struct {
	V17BaseNode
	Items []*V17StringNode // one or more strings (operators are implicit between items)
}

// ParseStringExprChain parses string_expr_chain
func (p *V17Parser) ParseStringExprChain() (node *V17StringExprChainNode, err error) {
	done := p.debugEnter("string_expr_chain")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseString()
	if ferr != nil {
		return nil, fmt.Errorf("string_expr_chain: %w", ferr)
	}
	items := []*V17StringNode{first}
	for {
		saved := p.savePos()
		if _, oerr := p.ParseStringOper(); oerr != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.ParseString()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}
	return &V17StringExprChainNode{V17BaseNode{line, col}, items}, nil
}

// V17StringGroupingNode  string_grouping
type V17StringGroupingNode struct {
	V17BaseNode
	Items []interface{} // alternating: expr/grouping, *V17StringOperNode, …
}

// ParseStringGrouping parses string_grouping (recursive)
func (p *V17Parser) ParseStringGrouping() (node *V17StringGroupingNode, err error) {
	done := p.debugEnter("string_grouping")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.ParseGroupBegin(); err != nil {
		return nil, fmt.Errorf("string_grouping: %w", err)
	}
	first, ferr := p.parseStringGroupingItem()
	if ferr != nil {
		return nil, fmt.Errorf("string_grouping: %w", ferr)
	}
	items := []interface{}{first}
	for {
		saved := p.savePos()
		op, oerr := p.ParseStringOper()
		if oerr != nil {
			p.restorePos(saved)
			break
		}
		rhs, rerr := p.parseStringGroupingItem()
		if rerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, op, rhs)
	}
	if _, err = p.ParseGroupEnd(); err != nil {
		return nil, fmt.Errorf("string_grouping: %w", err)
	}
	return &V17StringGroupingNode{V17BaseNode{line, col}, items}, nil
}

func (p *V17Parser) parseStringGroupingItem() (interface{}, error) {
	if p.peekAfterWS() == '(' {
		return p.ParseStringGrouping()
	}
	return p.ParseStringExprChain()
}

// V17StringConcatNode  string_concat = string_expr_chain | string_grouping
type V17StringConcatNode struct {
	V17BaseNode
	// Value is one of: *V17StringExprChainNode | *V17StringGroupingNode
	Value interface{}
}

// ParseStringConcat parses string_concat
func (p *V17Parser) ParseStringConcat() (node *V17StringConcatNode, err error) {
	done := p.debugEnter("string_concat")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() == '(' {
		g, gerr := p.ParseStringGrouping()
		if gerr != nil {
			return nil, fmt.Errorf("string_concat: %w", gerr)
		}
		return &V17StringConcatNode{V17BaseNode{line, col}, g}, nil
	}
	c, cerr := p.ParseStringExprChain()
	if cerr != nil {
		return nil, fmt.Errorf("string_concat: %w", cerr)
	}
	return &V17StringConcatNode{V17BaseNode{line, col}, c}, nil
}

// =============================================================================
// COMPARE OPERATORS & EXPRESSIONS
// compare_oper   = "!=" | "==" | ">" | ">=" | "<" | "<="
// num_compare    = TYPE_OF boolean<(num_expr_chain|num_grouping) compare_oper (single_num_expr|num_grouping)>
// string_compare = TYPE_OF boolean<string_expr_chain compare_oper (string|string_grouping|regexp_expr)>
// condition      = num_compare | string_compare
// EXTEND<condition> = | logic_expr   (handled in ParseCondition dispatcher)
// =============================================================================

// V17CompareOperNode  compare_oper = "!=" | "==" | ">" | ">=" | "<" | "<="
type V17CompareOperNode struct {
	V17BaseNode
	Op string
}

// ParseCompareOper parses compare_oper
func (p *V17Parser) ParseCompareOper() (node *V17CompareOperNode, err error) {
	done := p.debugEnter("compare_oper")
	defer func() { done(err == nil) }()
	switch {
	case p.peekLit("!="):
		tok, _ := p.matchLit("!=")
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "!="}, nil
	case p.peekLit("=="):
		tok, _ := p.matchLit("==")
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "=="}, nil
	case p.peekLit(">="):
		tok, _ := p.matchLit(">=")
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, ">="}, nil
	case p.peekLit("<="):
		tok, _ := p.matchLit("<=")
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "<="}, nil
	case p.peekAfterWS() == '>':
		tok, _ := p.matchLit(">")
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, ">"}, nil
	case p.peekAfterWS() == '<':
		tok, _ := p.matchLit("<")
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "<"}, nil
	}
	return nil, p.errAt("compare_oper: expected comparison operator")
}

// V17NumCompareNode  num_compare = TYPE_OF boolean< LHS compare_oper RHS >
// TYPE_OF boolean is a checker directive; the parser records both sides and defers type assertion.
type V17NumCompareNode struct {
	V17BaseNode
	LHS interface{} // *V17NumExprChainNode | *V17NumGroupingNode
	Op  *V17CompareOperNode
	RHS interface{} // *V17SingleNumExprNode | *V17NumGroupingNode
}

// ParseNumCompare parses num_compare
func (p *V17Parser) ParseNumCompare() (node *V17NumCompareNode, err error) {
	done := p.debugEnter("num_compare")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// LHS: num_expr_chain | num_grouping
	var lhs interface{}
	if p.peekAfterWS() == '(' {
		g, gerr := p.ParseNumGrouping()
		if gerr != nil {
			return nil, fmt.Errorf("num_compare: lhs: %w", gerr)
		}
		lhs = g
	} else {
		c, cerr := p.ParseNumExprChain()
		if cerr != nil {
			return nil, fmt.Errorf("num_compare: lhs: %w", cerr)
		}
		lhs = c
	}

	op, oerr := p.ParseCompareOper()
	if oerr != nil {
		return nil, fmt.Errorf("num_compare: %w", oerr)
	}

	// RHS: single_num_expr | num_grouping
	var rhs interface{}
	if p.peekAfterWS() == '(' {
		g, gerr := p.ParseNumGrouping()
		if gerr != nil {
			return nil, fmt.Errorf("num_compare: rhs: %w", gerr)
		}
		rhs = g
	} else {
		s, serr := p.ParseSingleNumExpr()
		if serr != nil {
			return nil, fmt.Errorf("num_compare: rhs: %w", serr)
		}
		rhs = s
	}
	return &V17NumCompareNode{V17BaseNode{line, col}, lhs, op, rhs}, nil
}

// V17StringCompareNode  string_compare = TYPE_OF boolean< string_expr_chain compare_oper RHS >
type V17StringCompareNode struct {
	V17BaseNode
	LHS *V17StringExprChainNode
	Op  *V17CompareOperNode
	RHS interface{} // *V17StringNode | *V17StringGroupingNode | *V17RegexpExprNode
}

// ParseStringCompare parses string_compare
func (p *V17Parser) ParseStringCompare() (node *V17StringCompareNode, err error) {
	done := p.debugEnter("string_compare")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	lhs, lerr := p.ParseStringExprChain()
	if lerr != nil {
		return nil, fmt.Errorf("string_compare: lhs: %w", lerr)
	}
	op, oerr := p.ParseCompareOper()
	if oerr != nil {
		return nil, fmt.Errorf("string_compare: %w", oerr)
	}
	// RHS: string | string_grouping | regexp_expr
	var rhs interface{}
	if p.peekAfterWS() == '/' {
		re, rerr := p.ParseRegexpExpr()
		if rerr != nil {
			return nil, fmt.Errorf("string_compare: rhs: %w", rerr)
		}
		rhs = re
	} else if p.peekAfterWS() == '(' {
		g, gerr := p.ParseStringGrouping()
		if gerr != nil {
			return nil, fmt.Errorf("string_compare: rhs: %w", gerr)
		}
		rhs = g
	} else {
		s, serr := p.ParseString()
		if serr != nil {
			return nil, fmt.Errorf("string_compare: rhs: %w", serr)
		}
		rhs = s
	}
	return &V17StringCompareNode{V17BaseNode{line, col}, lhs, op, rhs}, nil
}

// V17ConditionNode  condition = num_compare | string_compare
// EXTEND<condition> = | logic_expr  (added by ParseCondition dispatcher)
// EXTEND<condition> = | num_range_valid | date_range_valid | time_range_valid  (08_range.sqg)
type V17ConditionNode struct {
	V17BaseNode
	// Value is one of: *V17NumCompareNode | *V17StringCompareNode | *V17LogicExprNode |
	//                  *V17NumRangeValidNode | *V17DateRangeValidNode | *V17TimeRangeValidNode
	Value interface{}
}

// parseBaseCondition tries num_compare | string_compare only — no EXTEND<condition>.
// Used by ParseSingleLogicExpr to break the mutual-recursion cycle:
// ParseCondition → ParseLogicExpr → … → ParseSingleLogicExpr → ParseCondition.
func (p *V17Parser) parseBaseCondition() (*V17ConditionNode, error) {
	line, col := p.runeLine, p.runeCol
	if saved := p.savePos(); true {
		if nc, nerr := p.ParseNumCompare(); nerr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, nc}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if sc, serr := p.ParseStringCompare(); serr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, sc}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("base condition: expected num_compare or string_compare")
}

// ParseCondition parses condition (with EXTEND<condition> = | logic_expr)
func (p *V17Parser) ParseCondition() (node *V17ConditionNode, err error) {
	done := p.debugEnter("condition")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// num_compare — LHS starts with a numeric token or group
	if saved := p.savePos(); true {
		if nc, nerr := p.ParseNumCompare(); nerr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, nc}, nil
		}
		p.restorePos(saved)
	}
	// string_compare — LHS starts with a string token
	if saved := p.savePos(); true {
		if sc, serr := p.ParseStringCompare(); serr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, sc}, nil
		}
		p.restorePos(saved)
	}
	// EXTEND<condition> = | num_range_valid | date_range_valid | time_range_valid  (08_range.sqg)
	// These must come before logic_expr: all three start with TYPE_OF boolean<...
	// which logic_expr would partially consume as TYPE_OF boolean<ident_ref>.
	if saved := p.savePos(); true {
		if nrv, nrverr := p.ParseNumRangeValid(); nrverr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, nrv}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if drv, drverr := p.ParseDateRangeValid(); drverr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, drv}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if trv, trverr := p.ParseTimeRangeValid(); trverr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, trv}, nil
		}
		p.restorePos(saved)
	}
	// EXTEND<condition> = | logic_expr
	if saved := p.savePos(); true {
		if le, lerr := p.ParseLogicExpr(); lerr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, le}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("condition: expected num_compare, string_compare, num_range_valid, date_range_valid, time_range_valid or logic_expr")
}

// =============================================================================
// LOGIC OPERATORS & EXPRESSIONS
// not_oper           = "!"
// logic_and          = "&"
// logic_or           = "|"
// logic_exclusive_or = "^"
// logic_oper         = logic_and | logic_or | logic_exclusive_or
// single_logic_expr  = [ not_oper ] ( ident_dotted | TYPE_OF boolean<ident_ref> | condition ) | condition
// logic_expr_chain   = single_logic_expr { logic_oper single_logic_expr }
// logic_grouping     = group_begin ( logic_expr_chain | logic_grouping )
//                      { logic_oper ( logic_expr_chain | logic_grouping ) } group_end
// logic_expr         = logic_expr_chain | logic_grouping
// =============================================================================

// V17NotOperNode  not_oper = "!"
type V17NotOperNode struct{ V17BaseNode }

// ParseNotOper parses not_oper = "!"
func (p *V17Parser) ParseNotOper() (node *V17NotOperNode, err error) {
	done := p.debugEnter("not_oper")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("!")
	if err != nil {
		return nil, fmt.Errorf("not_oper: %w", err)
	}
	return &V17NotOperNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17LogicAndNode  logic_and = "&"
type V17LogicAndNode struct{ V17BaseNode }

// ParseLogicAnd parses logic_and = "&"
func (p *V17Parser) ParseLogicAnd() (node *V17LogicAndNode, err error) {
	done := p.debugEnter("logic_and")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("&")
	if err != nil {
		return nil, fmt.Errorf("logic_and: %w", err)
	}
	return &V17LogicAndNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17LogicOrNode  logic_or = "|"
type V17LogicOrNode struct{ V17BaseNode }

// ParseLogicOr parses logic_or = "|"
func (p *V17Parser) ParseLogicOr() (node *V17LogicOrNode, err error) {
	done := p.debugEnter("logic_or")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("|")
	if err != nil {
		return nil, fmt.Errorf("logic_or: %w", err)
	}
	return &V17LogicOrNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17LogicExclusiveOrNode  logic_exclusive_or = "^"
type V17LogicExclusiveOrNode struct{ V17BaseNode }

// ParseLogicExclusiveOr parses logic_exclusive_or = "^"
func (p *V17Parser) ParseLogicExclusiveOr() (node *V17LogicExclusiveOrNode, err error) {
	done := p.debugEnter("logic_exclusive_or")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("^")
	if err != nil {
		return nil, fmt.Errorf("logic_exclusive_or: %w", err)
	}
	return &V17LogicExclusiveOrNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17LogicOperNode  logic_oper = logic_and | logic_or | logic_exclusive_or
type V17LogicOperNode struct {
	V17BaseNode
	Op string // "&" | "|" | "^"
}

// ParseLogicOper parses logic_oper
func (p *V17Parser) ParseLogicOper() (node *V17LogicOperNode, err error) {
	done := p.debugEnter("logic_oper")
	defer func() { done(err == nil) }()
	switch {
	case p.peekAfterWS() == '&':
		tok, _ := p.matchLit("&")
		return &V17LogicOperNode{V17BaseNode{tok.Line, tok.Col}, "&"}, nil
	case p.peekAfterWS() == '|':
		tok, _ := p.matchLit("|")
		return &V17LogicOperNode{V17BaseNode{tok.Line, tok.Col}, "|"}, nil
	case p.peekAfterWS() == '^':
		tok, _ := p.matchLit("^")
		return &V17LogicOperNode{V17BaseNode{tok.Line, tok.Col}, "^"}, nil
	}
	return nil, p.errAt("logic_oper: expected &, | or ^")
}

// V17SingleLogicExprNode  single_logic_expr
// = [ not_oper ] ( ident_dotted | TYPE_OF boolean<ident_ref> | condition ) | condition
type V17SingleLogicExprNode struct {
	V17BaseNode
	Not   *V17NotOperNode
	Value interface{} // *V17IdentDottedNode | *V17IdentRefNode | *V17ConditionNode
}

// ParseSingleLogicExpr parses single_logic_expr
func (p *V17Parser) ParseSingleLogicExpr() (node *V17SingleLogicExprNode, err error) {
	done := p.debugEnter("single_logic_expr")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Optional not_oper
	var notOp *V17NotOperNode
	if p.peekAfterWS() == '!' {
		if n, nerr := p.ParseNotOper(); nerr == nil {
			notOp = n
		}
	}

	// Try base condition (num_compare | string_compare only) to avoid mutual recursion
	// with ParseCondition → ParseLogicExpr → … → ParseSingleLogicExpr.
	if saved := p.savePos(); true {
		if cond, cerr := p.parseBaseCondition(); cerr == nil {
			return &V17SingleLogicExprNode{V17BaseNode{line, col}, notOp, cond}, nil
		}
		p.restorePos(saved)
	}

	// Try TYPE_OF boolean<ident_ref> — parser records ident_ref; checker enforces boolean type
	if saved := p.savePos(); true {
		if ref, rerr := p.ParseIdentRef(); rerr == nil {
			return &V17SingleLogicExprNode{V17BaseNode{line, col}, notOp, ref}, nil
		}
		p.restorePos(saved)
	}

	// Bare ident_dotted
	if saved := p.savePos(); true {
		if id, derr := p.ParseIdentDotted(); derr == nil {
			return &V17SingleLogicExprNode{V17BaseNode{line, col}, notOp, id}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("single_logic_expr: expected condition, ident_ref or ident_dotted")
}

// V17LogicExprChainNode  logic_expr_chain = single_logic_expr { logic_oper single_logic_expr }
type V17LogicExprChainNode struct {
	V17BaseNode
	Items []interface{} // alternating: *V17SingleLogicExprNode, *V17LogicOperNode, …
}

// ParseLogicExprChain parses logic_expr_chain
func (p *V17Parser) ParseLogicExprChain() (node *V17LogicExprChainNode, err error) {
	done := p.debugEnter("logic_expr_chain")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseSingleLogicExpr()
	if ferr != nil {
		return nil, fmt.Errorf("logic_expr_chain: %w", ferr)
	}
	items := []interface{}{first}
	for {
		saved := p.savePos()
		op, oerr := p.ParseLogicOper()
		if oerr != nil {
			p.restorePos(saved)
			break
		}
		rhs, rerr := p.ParseSingleLogicExpr()
		if rerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, op, rhs)
	}
	return &V17LogicExprChainNode{V17BaseNode{line, col}, items}, nil
}

// V17LogicGroupingNode  logic_grouping
type V17LogicGroupingNode struct {
	V17BaseNode
	Items []interface{} // alternating: expr/grouping, *V17LogicOperNode, …
}

// ParseLogicGrouping parses logic_grouping (recursive)
func (p *V17Parser) ParseLogicGrouping() (node *V17LogicGroupingNode, err error) {
	done := p.debugEnter("logic_grouping")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.ParseGroupBegin(); err != nil {
		return nil, fmt.Errorf("logic_grouping: %w", err)
	}
	first, ferr := p.parseLogicGroupingItem()
	if ferr != nil {
		return nil, fmt.Errorf("logic_grouping: %w", ferr)
	}
	items := []interface{}{first}
	for {
		saved := p.savePos()
		op, oerr := p.ParseLogicOper()
		if oerr != nil {
			p.restorePos(saved)
			break
		}
		rhs, rerr := p.parseLogicGroupingItem()
		if rerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, op, rhs)
	}
	if _, err = p.ParseGroupEnd(); err != nil {
		return nil, fmt.Errorf("logic_grouping: %w", err)
	}
	return &V17LogicGroupingNode{V17BaseNode{line, col}, items}, nil
}

func (p *V17Parser) parseLogicGroupingItem() (interface{}, error) {
	if p.peekAfterWS() == '(' {
		return p.ParseLogicGrouping()
	}
	return p.ParseLogicExprChain()
}

// V17LogicExprNode  logic_expr = logic_expr_chain | logic_grouping
type V17LogicExprNode struct {
	V17BaseNode
	// Value is one of: *V17LogicExprChainNode | *V17LogicGroupingNode
	Value interface{}
}

// ParseLogicExpr parses logic_expr
func (p *V17Parser) ParseLogicExpr() (node *V17LogicExprNode, err error) {
	done := p.debugEnter("logic_expr")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() == '(' {
		g, gerr := p.ParseLogicGrouping()
		if gerr != nil {
			return nil, fmt.Errorf("logic_expr: %w", gerr)
		}
		return &V17LogicExprNode{V17BaseNode{line, col}, g}, nil
	}
	c, cerr := p.ParseLogicExprChain()
	if cerr != nil {
		return nil, fmt.Errorf("logic_expr: %w", cerr)
	}
	return &V17LogicExprNode{V17BaseNode{line, col}, c}, nil
}

// =============================================================================
// STATEMENT (top-level)
// statement = numeric_calc | string_concat
// =============================================================================

// V17StatementNode  statement = numeric_calc | string_concat
// EXTEND<statement> = | constant | cardinality  (03_assignment.sqg line 14)
// EXTEND<statement> = | assign_cond_rhs | self_ref  (03_assignment.sqg line 32)
// EXTEND<statement> = | array_final | object_final | array_lookup | object_lookup  (04_objects.sqg)
// EXTEND<statement> = | date_range | time_range  (08_range.sqg)
// EXTEND<statement> = | return_func_unit | iterator_loop  (06_functions.sqg)
// EXTEND<statement> = | __ps_call | __ps_proxy  (30_bootstrap.sqg)
type V17StatementNode struct {
	V17BaseNode
	// Value is one of: *V17NumericCalcNode | *V17StringConcatNode | *V17ConstantNode |
	//                  *V17CardinalityNode | *V17AssignCondRhsNode | *V17SelfRefNode |
	//                  *V17ArrayFinalNode | *V17ObjectFinalNode |
	//                  *V17ArrayLookupNode | *V17ObjectLookupNode |
	//                  *V17DateRangeNode | *V17TimeRangeNode |
	//                  *V17IteratorLoopNode |
	//                  *V17PsCallNode | *V17PsProxyNode
	Value interface{}
}

// ParseStatement parses statement = numeric_calc | string_concat
// EXTEND<statement> = | constant | cardinality  (03_assignment.sqg line 14)
// EXTEND<statement> = | assign_cond_rhs | self_ref  (03_assignment.sqg line 32)
// EXTEND<statement> = | array_final | object_final | array_lookup | object_lookup  (04_objects.sqg)
// EXTEND<statement> = | inspect_type  (06_functions.sqg)
// EXTEND<statement> = | return_func_unit | iterator_loop | func_call_final  (06_functions.sqg)
// EXTEND<statement> = | date_range | time_range  (08_range.sqg)
// EXTEND<statement> = | __ps_call | __ps_proxy  (30_bootstrap.sqg)
func (p *V17Parser) ParseStatement() (node *V17StatementNode, err error) {
	done := p.debugEnter("statement")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// EXTEND<statement> = | return_func_unit  (06_functions.sqg)
	// return_func_unit starts with "<-" which never starts a numeric expression.
	if p.peekLit("<-") {
		if saved := p.savePos(); true {
			if rfu, rfuerr := p.ParseReturnFuncUnit(); rfuerr == nil {
				return &V17StatementNode{V17BaseNode{line, col}, rfu}, nil
			}
			p.restorePos(saved)
		}
	}

	// EXTEND<statement> = | __ps_call | __ps_proxy  (30_bootstrap.sqg)
	// Must be tried BEFORE parseFuncCallFinalStrict: "__ps_call" and "__ps_proxy" start with
	// "__" which is also a valid ident prefix, so parseFuncCallFinalStrict would consume
	// them as a bare func_ref, treating the following args as func_call args.
	if p.peekLit("__ps_call") {
		if saved := p.savePos(); true {
			if pc, pcerr := p.ParsePsCall(); pcerr == nil {
				return &V17StatementNode{V17BaseNode{line, col}, pc}, nil
			}
			p.restorePos(saved)
		}
	}
	if p.peekLit("__ps_proxy") {
		if saved := p.savePos(); true {
			if pp, pperr := p.ParsePsProxy(); pperr == nil {
				return &V17StatementNode{V17BaseNode{line, col}, pp}, nil
			}
			p.restorePos(saved)
		}
	}

	// EXTEND<statement> = | func_call_final (strict — chain or args required)  (06_functions.sqg)
	// Must be checked before numeric_calc so that chains like "a -> b -> c" are
	// captured as a single func_call_final rather than stopping at just "a".
	// parseFuncCallFinalStrict fails for bare ident_refs (no chain, no args),
	// allowing numeric_calc to handle them as usual.
	if saved := p.savePos(); true {
		if fcf, fcferr := p.parseFuncCallFinalStrict(); fcferr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, fcf}, nil
		}
		p.restorePos(saved)
	}

	// EXTEND<statement> = | iterator_loop  (06_functions.sqg)
	// Must be tried before numeric_calc: a collection ident_ref followed by ">>" would
	// otherwise be consumed as a bare numeric expression (the ident_ref), leaving ">>" unmatched.
	if saved := p.savePos(); true {
		if il, ilerr := p.ParseIteratorLoop(); ilerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, il}, nil
		}
		p.restorePos(saved)
	}

	// EXTEND<statement> = | cardinality  (03_assignment.sqg line 14)
	// cardinality (digits ".." digits/m/M/many) must be tried BEFORE date_range/time_range
	// so that "1..1" or "1..2" is matched as a cardinality rather than a time/date range with
	// bare numeric components (time allows a bare hour, e.g. time(1)).
	// Real dates/times have "-"/":" separators that cardinality won't consume.
	if saved := p.savePos(); true {
		if ca, caerr := p.ParseCardinality(); caerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, ca}, nil
		}
		p.restorePos(saved)
	}

	// EXTEND<statement> = | date_range | time_range  (08_range.sqg)
	// Must be checked before numeric_calc: a date like 2024-01-01 would otherwise
	// be consumed as arithmetic (2024 - 1 - 1) by ParseNumericCalc.
	if saved := p.savePos(); true {
		if dr, drerr := p.ParseDateRange(); drerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, dr}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if tr, trerr := p.ParseTimeRange(); trerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, tr}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if nc, nerr := p.ParseNumericCalc(); nerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, nc}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if sc, serr := p.ParseStringConcat(); serr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, sc}, nil
		}
		p.restorePos(saved)
	}
	// EXTEND<statement> = | constant  (03_assignment.sqg line 14)
	if saved := p.savePos(); true {
		if cn, cerr := p.ParseConstant(); cerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, cn}, nil
		}
		p.restorePos(saved)
	}
	// EXTEND<statement> = | inspect_type  (06_functions.sqg)
	// inspect_type (@TypeName or @?) starts with '@' — unambiguous, try before assign_cond_rhs.
	if p.peekAfterWS() == '@' {
		if saved := p.savePos(); true {
			if it, iterr := p.ParseInspectType(); iterr == nil {
				return &V17StatementNode{V17BaseNode{line, col}, it}, nil
			}
			p.restorePos(saved)
		}
	}
	// EXTEND<statement> = | assign_cond_rhs | self_ref  (03_assignment.sqg line 32)
	if saved := p.savePos(); true {
		if ac, acerr := p.ParseAssignCondRhs(); acerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, ac}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if sr, srerr := p.ParseSelfRef(); srerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, sr}, nil
		}
		p.restorePos(saved)
	}
	// EXTEND<statement> = | array_final | object_final | array_lookup | object_lookup  (04_objects.sqg)
	// array_lookup and object_lookup are tried before array_final/object_final (more specific)
	if saved := p.savePos(); true {
		if al, alerr := p.ParseArrayLookup(); alerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, al}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if ol, olerr := p.ParseObjectLookup(); olerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, ol}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if af, aferr := p.ParseArrayFinal(); aferr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, af}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if of, oferr := p.ParseObjectFinal(); oferr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, of}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("statement: expected numeric_calc, string_concat, constant, cardinality, assign_cond_rhs, self_ref, array_final, object_final, array_lookup, object_lookup, func_call_final, return_func_unit, iterator_loop, date_range, time_range, __ps_call or __ps_proxy")
}
