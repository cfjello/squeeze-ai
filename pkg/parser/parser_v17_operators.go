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
	tok := p.cur()
	if tok.Type != V17_IDENT || !reIdentName.MatchString(tok.Value) {
		return nil, p.errAt("ident_name: expected identifier, got %s %q", tok.Type, tok.Value)
	}
	p.advance()
	return &V17IdentNameNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// V17GroupBeginNode  group_begin = "("
type V17GroupBeginNode struct{ V17BaseNode }

// ParseGroupBegin parses group_begin = "("
func (p *V17Parser) ParseGroupBegin() (node *V17GroupBeginNode, err error) {
	done := p.debugEnter("group_begin")
	defer func() { done(err == nil) }()
	tok, err := p.expect(V17_LPAREN)
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
	tok, err := p.expect(V17_RPAREN)
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
	line, col := p.cur().Line, p.cur().Col
	first, err := p.ParseIdentName()
	if err != nil {
		return nil, fmt.Errorf("ident_dotted: %w", err)
	}
	parts := []string{first.Value}
	for p.cur().Type == V17_DOT {
		saved := p.savePos()
		p.advance() // consume "."
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
// Uses DOTDOT+SLASH for "../" and DOT+SLASH for "./" (SIR-4 — no new token types).
func (p *V17Parser) ParseIdentPrefix() (node *V17IdentPrefixNode, err error) {
	done := p.debugEnter("ident_prefix")
	defer func() { done(err == nil) }()
	line, col := p.cur().Line, p.cur().Col

	// Try "../" — V17_DOTDOT followed by V17_SLASH
	if p.cur().Type == V17_DOTDOT {
		saved := p.savePos()
		p.advance() // consume ".."
		if p.cur().Type == V17_SLASH {
			p.advance() // consume "/"
			prefix := "../"
			// { "../" } — zero or more additional "../"
			for p.cur().Type == V17_DOTDOT {
				inner := p.savePos()
				p.advance()
				if p.cur().Type == V17_SLASH {
					p.advance()
					prefix += "../"
					continue
				}
				p.restorePos(inner)
				break
			}
			return &V17IdentPrefixNode{V17BaseNode{line, col}, prefix}, nil
		}
		p.restorePos(saved)
	}

	// Try "./" — V17_DOT followed by V17_SLASH
	if p.cur().Type == V17_DOT {
		saved := p.savePos()
		p.advance() // consume "."
		if p.cur().Type == V17_SLASH {
			p.advance() // consume "/"
			return &V17IdentPrefixNode{V17BaseNode{line, col}, "./"}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("ident_prefix: expected '../' or './', got %s %q", p.cur().Type, p.cur().Value)
}

// V17IdentRefNode  ident_ref = [ ident_prefix ] ident_dotted
type V17IdentRefNode struct {
	V17BaseNode
	Prefix *V17IdentPrefixNode // nil when no prefix
	Dotted *V17IdentDottedNode
}

// ParseIdentRef parses ident_ref = [ ident_prefix ] ident_dotted
func (p *V17Parser) ParseIdentRef() (node *V17IdentRefNode, err error) {
	done := p.debugEnter("ident_ref")
	defer func() { done(err == nil) }()
	line, col := p.cur().Line, p.cur().Col

	var prefix *V17IdentPrefixNode
	if p.cur().Type == V17_DOT || p.cur().Type == V17_DOTDOT {
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
	return &V17IdentRefNode{V17BaseNode{line, col}, prefix, dotted}, nil
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
	tok := p.cur()
	switch tok.Type {
	case V17_STARSTAR:
		p.advance()
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "**"}, nil
	case V17_PLUS:
		p.advance()
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "+"}, nil
	case V17_MINUS:
		p.advance()
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "-"}, nil
	case V17_STAR:
		p.advance()
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "*"}, nil
	case V17_SLASH:
		p.advance()
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "/"}, nil
	case V17_PERCENT:
		p.advance()
		return &V17NumericOperNode{V17BaseNode{tok.Line, tok.Col}, "%"}, nil
	}
	return nil, p.errAt("numeric_oper: expected +|-|*|**|/|%%, got %s %q", tok.Type, tok.Value)
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
	tok := p.cur()
	switch tok.Type {
	case V17_PLUSPLUS:
		p.advance()
		return &V17InlineIncrNode{V17BaseNode{tok.Line, tok.Col}, "++"}, nil
	case V17_MINUSMINUS:
		p.advance()
		return &V17InlineIncrNode{V17BaseNode{tok.Line, tok.Col}, "--"}, nil
	}
	return nil, p.errAt("inline_incr: expected ++ or --, got %s %q", tok.Type, tok.Value)
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
type V17SingleNumExprNode struct {
	V17BaseNode
	// Value is one of:
	//   *V17NumericConstNode           — literal numeric constant
	//   *V17IdentRefNode               — ident_ref with TYPE_OF numeric_const check
	Value    interface{}
	InlineOp *V17InlineIncrNode // nil when not present
}

// ParseSingleNumExpr parses single_num_expr
func (p *V17Parser) ParseSingleNumExpr() (node *V17SingleNumExprNode, err error) {
	done := p.debugEnter("single_num_expr")
	defer func() { done(err == nil) }()
	line, col := p.cur().Line, p.cur().Col

	// Try numeric_const first (no prefix possible)
	if saved := p.savePos(); true {
		if nc, nerr := p.ParseNumericConst(); nerr == nil {
			return &V17SingleNumExprNode{V17BaseNode{line, col}, nc, nil}, nil
		}
		p.restorePos(saved)
	}

	// Try [ inline_incr ] TYPE_OF numeric_const<ident_ref>
	// TYPE_OF is a directive; at parse time we parse ident_ref and record it;
	// type assertion is deferred to the checker (SIR per directive table 2.3).
	var incr *V17InlineIncrNode
	if p.cur().Type == V17_PLUSPLUS || p.cur().Type == V17_MINUSMINUS {
		saved := p.savePos()
		in, ierr := p.ParseInlineIncr()
		if ierr != nil {
			p.restorePos(saved)
		} else {
			incr = in
		}
	}
	ref, rerr := p.ParseIdentRef()
	if rerr != nil {
		return nil, fmt.Errorf("single_num_expr: expected numeric_const or ident_ref, got %w", rerr)
	}
	return &V17SingleNumExprNode{V17BaseNode{line, col}, ref, incr}, nil
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
	line, col := p.cur().Line, p.cur().Col

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
	line, col := p.cur().Line, p.cur().Col

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
	if p.cur().Type == V17_LPAREN {
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
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V17_LPAREN {
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
	tok, err := p.expect(V17_PLUS)
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
	line, col := p.cur().Line, p.cur().Col

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
	line, col := p.cur().Line, p.cur().Col

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
	if p.cur().Type == V17_LPAREN {
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
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V17_LPAREN {
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
	tok := p.cur()
	switch tok.Type {
	case V17_NEQ:
		p.advance()
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "!="}, nil
	case V17_EQEQ:
		p.advance()
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "=="}, nil
	case V17_GTE:
		p.advance()
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, ">="}, nil
	case V17_GT:
		p.advance()
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, ">"}, nil
	case V17_LTE:
		p.advance()
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "<="}, nil
	case V17_LT:
		p.advance()
		return &V17CompareOperNode{V17BaseNode{tok.Line, tok.Col}, "<"}, nil
	}
	return nil, p.errAt("compare_oper: expected comparison operator, got %s %q", tok.Type, tok.Value)
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
	line, col := p.cur().Line, p.cur().Col

	// LHS: num_expr_chain | num_grouping
	var lhs interface{}
	if p.cur().Type == V17_LPAREN {
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
	if p.cur().Type == V17_LPAREN {
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
	line, col := p.cur().Line, p.cur().Col

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
	if p.cur().Type == V17_REGEXP {
		re, rerr := p.ParseRegexpExpr()
		if rerr != nil {
			return nil, fmt.Errorf("string_compare: rhs: %w", rerr)
		}
		rhs = re
	} else if p.cur().Type == V17_LPAREN {
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
type V17ConditionNode struct {
	V17BaseNode
	// Value is one of: *V17NumCompareNode | *V17StringCompareNode | *V17LogicExprNode
	Value interface{}
}

// parseBaseCondition tries num_compare | string_compare only — no EXTEND<condition>.
// Used by ParseSingleLogicExpr to break the mutual-recursion cycle:
// ParseCondition → ParseLogicExpr → … → ParseSingleLogicExpr → ParseCondition.
func (p *V17Parser) parseBaseCondition() (*V17ConditionNode, error) {
	line, col := p.cur().Line, p.cur().Col
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
	line, col := p.cur().Line, p.cur().Col

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
	// EXTEND<condition> = | logic_expr
	if saved := p.savePos(); true {
		if le, lerr := p.ParseLogicExpr(); lerr == nil {
			return &V17ConditionNode{V17BaseNode{line, col}, le}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("condition: expected num_compare, string_compare or logic_expr")
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
	tok, err := p.expect(V17_BANG)
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
	tok, err := p.expect(V17_AMP)
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
	tok, err := p.expect(V17_PIPE)
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
	tok, err := p.expect(V17_CARET)
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
	tok := p.cur()
	switch tok.Type {
	case V17_AMP:
		p.advance()
		return &V17LogicOperNode{V17BaseNode{tok.Line, tok.Col}, "&"}, nil
	case V17_PIPE:
		p.advance()
		return &V17LogicOperNode{V17BaseNode{tok.Line, tok.Col}, "|"}, nil
	case V17_CARET:
		p.advance()
		return &V17LogicOperNode{V17BaseNode{tok.Line, tok.Col}, "^"}, nil
	}
	return nil, p.errAt("logic_oper: expected &, | or ^, got %s %q", tok.Type, tok.Value)
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
	line, col := p.cur().Line, p.cur().Col

	// Optional not_oper
	var notOp *V17NotOperNode
	if p.cur().Type == V17_BANG {
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
	line, col := p.cur().Line, p.cur().Col

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
	line, col := p.cur().Line, p.cur().Col

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
	if p.cur().Type == V17_LPAREN {
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
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V17_LPAREN {
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
type V17StatementNode struct {
	V17BaseNode
	// Value is one of: *V17NumericCalcNode | *V17StringConcatNode | *V17ConstantNode |
	//                  *V17CardinalityNode | *V17AssignCondRhsNode | *V17SelfRefNode
	Value interface{}
}

// ParseStatement parses statement = numeric_calc | string_concat
// EXTEND<statement> = | constant | cardinality  (03_assignment.sqg line 14)
// EXTEND<statement> = | assign_cond_rhs | self_ref  (03_assignment.sqg line 32)
func (p *V17Parser) ParseStatement() (node *V17StatementNode, err error) {
	done := p.debugEnter("statement")
	defer func() { done(err == nil) }()
	line, col := p.cur().Line, p.cur().Col

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
	// EXTEND<statement> = | constant | cardinality  (03_assignment.sqg line 14)
	if saved := p.savePos(); true {
		if cn, cerr := p.ParseConstant(); cerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, cn}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if ca, caerr := p.ParseCardinality(); caerr == nil {
			return &V17StatementNode{V17BaseNode{line, col}, ca}, nil
		}
		p.restorePos(saved)
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
	return nil, p.errAt("statement: expected numeric_calc, string_concat, constant, cardinality, assign_cond_rhs or self_ref")
}
