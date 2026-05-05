// parser_v17_json_path.go — ParseXxx methods for spec/05_json_path.sqg.
//
// Rules implemented:
//
//	jp_name, jp_wildcard, jp_index, jp_slice,
//	jp_current_path, jp_filter_value, jp_filter_oper, jp_filter_cmp,
//	jp_filter_not, jp_filter_logic, jp_filter_atom, jp_filter_unary,
//	jp_filter_expr, jp_filter, jp_selector, jp_selector_list,
//	jp_bracket_seg, jp_dot_seg, jp_child_seg, jp_desc_seg, jp_segment,
//	json_path
//
// EXTEND<ident_ref> = [ json_path ]
//
//	(added to ParseIdentRef in parser_v17_operators.go)
//
// SIR-4 note:
//
//	"&&" and "||" (jp_filter_logic) are NEVER pre-lexed as compound tokens.
//	They are matched at parse time from two consecutive V17_AMP / V17_PIPE
//	tokens inside ParseJpFilterLogic only.
//	"=~" (jp_filter_oper) uses V17_EQ_TILDE (the only compound exception,
//	because '~' alone is ILLEGAL in the lexer and is not a standalone token).
//	Both "&&" / "||" matching and "=~" matching happen exclusively inside
//	json_path parse methods — they are never matched by any other rule.
//
// !WS! directive:
//
//	Adjacency between tokens is checked by comparing column numbers.
//	Token A immediately followed by token B (no whitespace, same line):
//	  B.Col == A.Col + len(A.Value)
//	The lexer skips whitespace silently, so this is the only way to detect
//	the absence of whitespace between two tokens.
package parser

import "fmt"

// =============================================================================
// jp_name = ident_name | string
// =============================================================================

// V17JpNameNode  jp_name = ident_name | string
type V17JpNameNode struct {
	V17BaseNode
	// Value is *V17IdentNameNode | *V17StringNode
	Value interface{}
}

// ParseJpName parses jp_name = ident_name | string.
func (p *V17Parser) ParseJpName() (node *V17JpNameNode, err error) {
	done := p.debugEnter("jp_name")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// ident_name: letter start
	if ch := p.peekAfterWS(); ch != 0 && (ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch > 127) {
		saved := p.savePos()
		if in, inerr := p.ParseIdentName(); inerr == nil {
			return &V17JpNameNode{V17BaseNode{in.Line, in.Col}, in}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if s, serr := p.ParseString(); serr == nil {
			return &V17JpNameNode{V17BaseNode{line, col}, s}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("jp_name: expected identifier or string")
}

// =============================================================================
// jp_wildcard = "*"
// =============================================================================

// V17JpWildcardNode  jp_wildcard = "*"
type V17JpWildcardNode struct{ V17BaseNode }

// ParseJpWildcard parses jp_wildcard = "*".
func (p *V17Parser) ParseJpWildcard() (node *V17JpWildcardNode, err error) {
	done := p.debugEnter("jp_wildcard")
	defer func() { done(err == nil) }()
	tok, terr := p.matchLit("*")
	if terr != nil {
		return nil, fmt.Errorf("jp_wildcard: %w", terr)
	}
	return &V17JpWildcardNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// =============================================================================
// jp_index = integer
// =============================================================================

// V17JpIndexNode  jp_index = integer
type V17JpIndexNode struct {
	V17BaseNode
	Index *V17IntegerNode
}

// ParseJpIndex parses jp_index = integer.
func (p *V17Parser) ParseJpIndex() (node *V17JpIndexNode, err error) {
	done := p.debugEnter("jp_index")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	idx, idxerr := p.ParseInteger()
	if idxerr != nil {
		return nil, fmt.Errorf("jp_index: %w", idxerr)
	}
	return &V17JpIndexNode{V17BaseNode{line, col}, idx}, nil
}

// =============================================================================
// jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ]
// =============================================================================

// V17JpSliceNode  jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ]
type V17JpSliceNode struct {
	V17BaseNode
	Start *V17IntegerNode // nil when absent
	End   *V17IntegerNode // nil when absent
	Step  *V17IntegerNode // nil when second ":" absent or integer after it absent
}

// ParseJpSlice parses jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ].
func (p *V17Parser) ParseJpSlice() (node *V17JpSliceNode, err error) {
	done := p.debugEnter("jp_slice")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// [ integer ]
	var start *V17IntegerNode
	if p.peekAfterWS() != ':' {
		saved := p.savePos()
		if i, ierr := p.ParseInteger(); ierr == nil {
			start = i
		} else {
			p.restorePos(saved)
			return nil, p.errAt("jp_slice: expected integer or ':'")
		}
	}

	// mandatory ":"
	if _, cerr := p.matchLit(":"); cerr != nil {
		return nil, fmt.Errorf("jp_slice: %w", cerr)
	}

	// [ integer ]
	var end *V17IntegerNode
	if saved := p.savePos(); true {
		if i, ierr := p.ParseInteger(); ierr == nil {
			end = i
		} else {
			p.restorePos(saved)
		}
	}

	// [ ":" [ integer ] ]
	var step *V17IntegerNode
	if p.peekAfterWS() == ':' {
		inner := p.savePos()
		if _, cerr := p.matchLit(":"); cerr != nil {
			p.restorePos(inner)
		} else {
			if saved := p.savePos(); true {
				if i, ierr := p.ParseInteger(); ierr == nil {
					step = i
				} else {
					p.restorePos(saved)
				}
			}
		}
	}

	return &V17JpSliceNode{V17BaseNode{line, col}, start, end, step}, nil
}

// =============================================================================
// jp_current_path = "@" { jp_segment }
// =============================================================================

// V17JpCurrentPathNode  jp_current_path = "@" { jp_segment }
type V17JpCurrentPathNode struct {
	V17BaseNode
	Segments []*V17JpSegmentNode // zero or more
}

// ParseJpCurrentPath parses jp_current_path = "@" { jp_segment }.
func (p *V17Parser) ParseJpCurrentPath() (node *V17JpCurrentPathNode, err error) {
	done := p.debugEnter("jp_current_path")
	defer func() { done(err == nil) }()
	tok, terr := p.matchLit("@")
	if terr != nil {
		return nil, fmt.Errorf("jp_current_path: %w", terr)
	}

	var segments []*V17JpSegmentNode
	for {
		saved := p.savePos()
		seg, segerr := p.ParseJpSegment()
		if segerr != nil {
			p.restorePos(saved)
			break
		}
		segments = append(segments, seg)
	}

	return &V17JpCurrentPathNode{V17BaseNode{tok.Line, tok.Col}, segments}, nil
}

// =============================================================================
// jp_filter_value = constant | jp_current_path | ident_ref
// =============================================================================

// V17JpFilterValueNode  jp_filter_value = constant | jp_current_path | ident_ref
type V17JpFilterValueNode struct {
	V17BaseNode
	// Value is *V17ConstantNode | *V17JpCurrentPathNode | *V17IdentRefNode
	Value interface{}
}

// ParseJpFilterValue parses jp_filter_value = constant | jp_current_path | ident_ref.
func (p *V17Parser) ParseJpFilterValue() (node *V17JpFilterValueNode, err error) {
	done := p.debugEnter("jp_filter_value")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// jp_current_path — starts with '@'
	if p.peekAfterWS() == '@' {
		cp, cperr := p.ParseJpCurrentPath()
		if cperr != nil {
			return nil, fmt.Errorf("jp_filter_value: %w", cperr)
		}
		return &V17JpFilterValueNode{V17BaseNode{line, col}, cp}, nil
	}
	// constant
	if saved := p.savePos(); true {
		if cn, cnerr := p.ParseConstant(); cnerr == nil {
			return &V17JpFilterValueNode{V17BaseNode{line, col}, cn}, nil
		}
		p.restorePos(saved)
	}
	// ident_ref
	if saved := p.savePos(); true {
		if ref, referr := p.ParseIdentRef(); referr == nil {
			return &V17JpFilterValueNode{V17BaseNode{line, col}, ref}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("jp_filter_value: expected constant, '@' path, or identifier")
}

// =============================================================================
// jp_filter_oper = "==" | "!=" | ">=" | "<=" | ">" | "<" | "=~"
// =============================================================================

// V17JpFilterOperNode  jp_filter_oper = "==" | "!=" | ">=" | "<=" | ">" | "<" | "=~"
type V17JpFilterOperNode struct {
	V17BaseNode
	Op string
}

// ParseJpFilterOper parses jp_filter_oper.
func (p *V17Parser) ParseJpFilterOper() (node *V17JpFilterOperNode, err error) {
	done := p.debugEnter("jp_filter_oper")
	defer func() { done(err == nil) }()
	switch {
	case p.peekLit("=~"):
		tok, _ := p.matchLit("=~")
		return &V17JpFilterOperNode{V17BaseNode{tok.Line, tok.Col}, "=~"}, nil
	case p.peekLit("=="):
		tok, _ := p.matchLit("==")
		return &V17JpFilterOperNode{V17BaseNode{tok.Line, tok.Col}, "=="}, nil
	case p.peekLit("!="):
		tok, _ := p.matchLit("!=")
		return &V17JpFilterOperNode{V17BaseNode{tok.Line, tok.Col}, "!="}, nil
	case p.peekLit(">="):
		tok, _ := p.matchLit(">=")
		return &V17JpFilterOperNode{V17BaseNode{tok.Line, tok.Col}, ">="}, nil
	case p.peekLit("<="):
		tok, _ := p.matchLit("<=")
		return &V17JpFilterOperNode{V17BaseNode{tok.Line, tok.Col}, "<="}, nil
	case p.peekAfterWS() == '>':
		tok, _ := p.matchLit(">")
		return &V17JpFilterOperNode{V17BaseNode{tok.Line, tok.Col}, ">"}, nil
	case p.peekAfterWS() == '<':
		tok, _ := p.matchLit("<")
		return &V17JpFilterOperNode{V17BaseNode{tok.Line, tok.Col}, "<"}, nil
	}
	return nil, p.errAt("jp_filter_oper: expected one of ==, !=, >=, <=, >, <, =~")
}

// =============================================================================
// jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value
// =============================================================================

// V17JpFilterCmpNode  jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value
type V17JpFilterCmpNode struct {
	V17BaseNode
	LHS  *V17JpFilterValueNode
	Oper *V17JpFilterOperNode
	RHS  *V17JpFilterValueNode
}

// ParseJpFilterCmp parses jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value.
func (p *V17Parser) ParseJpFilterCmp() (node *V17JpFilterCmpNode, err error) {
	done := p.debugEnter("jp_filter_cmp")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	lhs, lhserr := p.ParseJpFilterValue()
	if lhserr != nil {
		return nil, fmt.Errorf("jp_filter_cmp: %w", lhserr)
	}
	oper, opererr := p.ParseJpFilterOper()
	if opererr != nil {
		return nil, fmt.Errorf("jp_filter_cmp: %w", opererr)
	}
	rhs, rhserr := p.ParseJpFilterValue()
	if rhserr != nil {
		return nil, fmt.Errorf("jp_filter_cmp: %w", rhserr)
	}
	return &V17JpFilterCmpNode{V17BaseNode{line, col}, lhs, oper, rhs}, nil
}

// =============================================================================
// jp_filter_not = "!"
// =============================================================================

// V17JpFilterNotNode  jp_filter_not = "!"
type V17JpFilterNotNode struct{ V17BaseNode }

// ParseJpFilterNot parses jp_filter_not = "!".
func (p *V17Parser) ParseJpFilterNot() (node *V17JpFilterNotNode, err error) {
	done := p.debugEnter("jp_filter_not")
	defer func() { done(err == nil) }()
	tok, terr := p.matchLit("!")
	if terr != nil {
		return nil, fmt.Errorf("jp_filter_not: %w", terr)
	}
	return &V17JpFilterNotNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// =============================================================================
// jp_filter_logic = "&&" | "||"
//
// SIR-4: "&&" and "||" are NEVER pre-lexed.  They are matched at parse time
// as two consecutive V17_AMP tokens ("&&") or two consecutive V17_PIPE tokens
// ("||").  This function is the ONLY place in the entire parser that consumes
// these two-token combinations as a logical unit.
// =============================================================================

// V17JpFilterLogicNode  jp_filter_logic = "&&" | "||"
type V17JpFilterLogicNode struct {
	V17BaseNode
	Op string // "&&" or "||"
}

// ParseJpFilterLogic parses jp_filter_logic = "&&" | "||".
// Uses peekLit to detect the two-char combinations without pre-lexing (SIR-4).
func (p *V17Parser) ParseJpFilterLogic() (node *V17JpFilterLogicNode, err error) {
	done := p.debugEnter("jp_filter_logic")
	defer func() { done(err == nil) }()
	switch {
	case p.peekLit("&&"):
		tok, _ := p.matchLit("&&")
		return &V17JpFilterLogicNode{V17BaseNode{tok.Line, tok.Col}, "&&"}, nil
	case p.peekLit("||"):
		tok, _ := p.matchLit("||")
		return &V17JpFilterLogicNode{V17BaseNode{tok.Line, tok.Col}, "||"}, nil
	}
	return nil, p.errAt("jp_filter_logic: expected '&&' or '||'")
}

// =============================================================================
// jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")"
// =============================================================================

// V17JpFilterAtomNode  jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")"
type V17JpFilterAtomNode struct {
	V17BaseNode
	// Value is *V17JpFilterCmpNode | *V17JpCurrentPathNode | *V17JpFilterExprNode
	Value   interface{}
	Grouped bool // true when "(" jp_filter_expr ")" form
}

// ParseJpFilterAtom parses jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")".
func (p *V17Parser) ParseJpFilterAtom() (node *V17JpFilterAtomNode, err error) {
	done := p.debugEnter("jp_filter_atom")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// "(" jp_filter_expr ")" — unambiguous via LPAREN
	if p.peekAfterWS() == '(' {
		if _, lperr := p.matchLit("("); lperr != nil {
			return nil, fmt.Errorf("jp_filter_atom: %w", lperr)
		}
		expr, exprerr := p.ParseJpFilterExpr()
		if exprerr != nil {
			return nil, fmt.Errorf("jp_filter_atom: %w", exprerr)
		}
		if _, rperr := p.matchLit(")"); rperr != nil {
			return nil, fmt.Errorf("jp_filter_atom: %w", rperr)
		}
		return &V17JpFilterAtomNode{V17BaseNode{line, col}, expr, true}, nil
	}

	// jp_filter_cmp — try first (requires oper after filter_value)
	if saved := p.savePos(); true {
		if cmp, cmperr := p.ParseJpFilterCmp(); cmperr == nil {
			return &V17JpFilterAtomNode{V17BaseNode{line, col}, cmp, false}, nil
		}
		p.restorePos(saved)
	}

	// jp_current_path — existence test; starts with '@'
	if p.peekAfterWS() == '@' {
		cp, cperr := p.ParseJpCurrentPath()
		if cperr != nil {
			return nil, fmt.Errorf("jp_filter_atom: %w", cperr)
		}
		return &V17JpFilterAtomNode{V17BaseNode{line, col}, cp, false}, nil
	}

	return nil, p.errAt("jp_filter_atom: expected filter comparison, '@' path, or '(' expr ')'")
}

// =============================================================================
// jp_filter_unary = [ jp_filter_not ] jp_filter_atom
// =============================================================================

// V17JpFilterUnaryNode  jp_filter_unary = [ jp_filter_not ] jp_filter_atom
type V17JpFilterUnaryNode struct {
	V17BaseNode
	Not  *V17JpFilterNotNode // nil when absent
	Atom *V17JpFilterAtomNode
}

// ParseJpFilterUnary parses jp_filter_unary = [ jp_filter_not ] jp_filter_atom.
func (p *V17Parser) ParseJpFilterUnary() (node *V17JpFilterUnaryNode, err error) {
	done := p.debugEnter("jp_filter_unary")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	var not *V17JpFilterNotNode
	if p.peekAfterWS() == '!' {
		saved := p.savePos()
		if n, nerr := p.ParseJpFilterNot(); nerr == nil {
			not = n
		} else {
			p.restorePos(saved)
		}
	}

	atom, atomerr := p.ParseJpFilterAtom()
	if atomerr != nil {
		return nil, fmt.Errorf("jp_filter_unary: %w", atomerr)
	}
	return &V17JpFilterUnaryNode{V17BaseNode{line, col}, not, atom}, nil
}

// =============================================================================
// jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
// =============================================================================

// V17JpFilterExprNode  jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
type V17JpFilterExprNode struct {
	V17BaseNode
	First    *V17JpFilterUnaryNode
	LogicOps []*V17JpFilterLogicNode
	Operands []*V17JpFilterUnaryNode
}

// ParseJpFilterExpr parses jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }.
func (p *V17Parser) ParseJpFilterExpr() (node *V17JpFilterExprNode, err error) {
	done := p.debugEnter("jp_filter_expr")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseJpFilterUnary()
	if ferr != nil {
		return nil, fmt.Errorf("jp_filter_expr: %w", ferr)
	}

	var logicOps []*V17JpFilterLogicNode
	var operands []*V17JpFilterUnaryNode

	for {
		// jp_filter_logic = "&&" | "||" (two consecutive single-char tokens)
		saved := p.savePos()
		op, operr := p.ParseJpFilterLogic()
		if operr != nil {
			p.restorePos(saved)
			break
		}
		rhs, rhserr := p.ParseJpFilterUnary()
		if rhserr != nil {
			p.restorePos(saved)
			break
		}
		logicOps = append(logicOps, op)
		operands = append(operands, rhs)
	}

	return &V17JpFilterExprNode{V17BaseNode{line, col}, first, logicOps, operands}, nil
}

// =============================================================================
// jp_filter = "?" "(" jp_filter_expr ")"
// =============================================================================

// V17JpFilterNode  jp_filter = "?" "(" jp_filter_expr ")"
type V17JpFilterNode struct {
	V17BaseNode
	Expr *V17JpFilterExprNode
}

// ParseJpFilter parses jp_filter = "?" "(" jp_filter_expr ")".
func (p *V17Parser) ParseJpFilter() (node *V17JpFilterNode, err error) {
	done := p.debugEnter("jp_filter")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, qerr := p.matchLit("?"); qerr != nil {
		return nil, fmt.Errorf("jp_filter: %w", qerr)
	}
	if _, lperr := p.matchLit("("); lperr != nil {
		return nil, fmt.Errorf("jp_filter: %w", lperr)
	}
	expr, exprerr := p.ParseJpFilterExpr()
	if exprerr != nil {
		return nil, fmt.Errorf("jp_filter: %w", exprerr)
	}
	if _, rperr := p.matchLit(")"); rperr != nil {
		return nil, fmt.Errorf("jp_filter: %w", rperr)
	}
	return &V17JpFilterNode{V17BaseNode{line, col}, expr}, nil
}

// =============================================================================
// jp_selector = jp_filter | jp_slice | jp_index | jp_name | jp_wildcard
// =============================================================================

// V17JpSelectorNode  jp_selector = jp_filter | jp_slice | jp_index | jp_name | jp_wildcard
type V17JpSelectorNode struct {
	V17BaseNode
	// Value is *V17JpFilterNode | *V17JpSliceNode | *V17JpIndexNode |
	//           *V17JpNameNode | *V17JpWildcardNode
	Value interface{}
}

// ParseJpSelector parses jp_selector = jp_filter | jp_slice | jp_index | jp_name | jp_wildcard.
func (p *V17Parser) ParseJpSelector() (node *V17JpSelectorNode, err error) {
	done := p.debugEnter("jp_selector")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// jp_filter — "?" is unambiguous
	if p.peekAfterWS() == '?' {
		f, ferr := p.ParseJpFilter()
		if ferr != nil {
			return nil, fmt.Errorf("jp_selector: %w", ferr)
		}
		return &V17JpSelectorNode{V17BaseNode{line, col}, f}, nil
	}

	// jp_wildcard — "*" is unambiguous
	if p.peekAfterWS() == '*' {
		wc, wcerr := p.ParseJpWildcard()
		if wcerr != nil {
			return nil, fmt.Errorf("jp_selector: %w", wcerr)
		}
		return &V17JpSelectorNode{V17BaseNode{line, col}, wc}, nil
	}

	// jp_slice — try before jp_index (both can start with integer or ":")
	if saved := p.savePos(); true {
		if sl, slerr := p.ParseJpSlice(); slerr == nil {
			return &V17JpSelectorNode{V17BaseNode{line, col}, sl}, nil
		}
		p.restorePos(saved)
	}

	// jp_index — integer only (no ":" after it)
	if saved := p.savePos(); true {
		if idx, idxerr := p.ParseJpIndex(); idxerr == nil {
			return &V17JpSelectorNode{V17BaseNode{line, col}, idx}, nil
		}
		p.restorePos(saved)
	}

	// jp_name — ident_name or string
	if saved := p.savePos(); true {
		if jn, jnerr := p.ParseJpName(); jnerr == nil {
			return &V17JpSelectorNode{V17BaseNode{line, col}, jn}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("jp_selector: expected filter, slice, index, name, or wildcard")
}

// =============================================================================
// jp_selector_list = jp_selector { "," jp_selector }
// =============================================================================

// V17JpSelectorListNode  jp_selector_list = jp_selector { "," jp_selector }
type V17JpSelectorListNode struct {
	V17BaseNode
	Items []*V17JpSelectorNode
}

// ParseJpSelectorList parses jp_selector_list = jp_selector { "," jp_selector }.
func (p *V17Parser) ParseJpSelectorList() (node *V17JpSelectorListNode, err error) {
	done := p.debugEnter("jp_selector_list")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseJpSelector()
	if ferr != nil {
		return nil, fmt.Errorf("jp_selector_list: %w", ferr)
	}
	items := []*V17JpSelectorNode{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, merr := p.matchLit(","); merr != nil {
			p.restorePos(saved)
			break
		}
		next, nexterr := p.ParseJpSelector()
		if nexterr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	return &V17JpSelectorListNode{V17BaseNode{line, col}, items}, nil
}

// =============================================================================
// jp_bracket_seg = "[" jp_selector_list "]"
// =============================================================================

// V17JpBracketSegNode  jp_bracket_seg = "[" jp_selector_list "]"
type V17JpBracketSegNode struct {
	V17BaseNode
	List *V17JpSelectorListNode
}

// ParseJpBracketSeg parses jp_bracket_seg = "[" jp_selector_list "]".
func (p *V17Parser) ParseJpBracketSeg() (node *V17JpBracketSegNode, err error) {
	done := p.debugEnter("jp_bracket_seg")
	defer func() { done(err == nil) }()
	tok, lberr := p.matchLit("[")
	if lberr != nil {
		return nil, fmt.Errorf("jp_bracket_seg: %w", lberr)
	}
	list, listerr := p.ParseJpSelectorList()
	if listerr != nil {
		return nil, fmt.Errorf("jp_bracket_seg: %w", listerr)
	}
	if _, rberr := p.matchLit("]"); rberr != nil {
		return nil, fmt.Errorf("jp_bracket_seg: %w", rberr)
	}
	return &V17JpBracketSegNode{V17BaseNode{tok.Line, tok.Col}, list}, nil
}

// =============================================================================
// jp_dot_seg = "." !WS! ( jp_name | jp_wildcard )
// =============================================================================

// V17JpDotSegNode  jp_dot_seg = "." !WS! ( jp_name | jp_wildcard )
type V17JpDotSegNode struct {
	V17BaseNode
	// Value is *V17JpNameNode | *V17JpWildcardNode
	Value interface{}
}

// ParseJpDotSeg parses jp_dot_seg = "." !WS! ( jp_name | jp_wildcard ).
func (p *V17Parser) ParseJpDotSeg() (node *V17JpDotSegNode, err error) {
	done := p.debugEnter("jp_dot_seg")
	defer func() { done(err == nil) }()
	// Match "." (with WS skip) then immediately (no WS) check name/wildcard
	dotTok, doterr := p.matchLit(".")
	if doterr != nil {
		return nil, p.errAt("jp_dot_seg: expected '.'")
	}
	line, col := dotTok.Line, dotTok.Col

	// !WS! — jp_name or jp_wildcard must follow immediately (no whitespace)
	// jp_wildcard: "*"
	if tok, terr := p.matchLitNoWS("*"); terr == nil {
		wc := &V17JpWildcardNode{V17BaseNode{tok.Line, tok.Col}}
		return &V17JpDotSegNode{V17BaseNode{line, col}, wc}, nil
	}

	// jp_name: ident_name (no WS allowed)
	if tok, terr := p.matchReNoWS(reIdentNameScanNoWS); terr == nil {
		in := &V17IdentNameNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}
		jn := &V17JpNameNode{V17BaseNode{tok.Line, tok.Col}, in}
		return &V17JpDotSegNode{V17BaseNode{line, col}, jn}, nil
	}

	return nil, p.errAt("jp_dot_seg: expected name or '*' immediately after '.'")
}

// =============================================================================
// jp_child_seg = jp_bracket_seg | jp_dot_seg
// =============================================================================

// V17JpChildSegNode  jp_child_seg = jp_bracket_seg | jp_dot_seg
type V17JpChildSegNode struct {
	V17BaseNode
	// Value is *V17JpBracketSegNode | *V17JpDotSegNode
	Value interface{}
}

// ParseJpChildSeg parses jp_child_seg = jp_bracket_seg | jp_dot_seg.
func (p *V17Parser) ParseJpChildSeg() (node *V17JpChildSegNode, err error) {
	done := p.debugEnter("jp_child_seg")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// jp_bracket_seg starts with "["
	if p.peekAfterWS() == '[' {
		bs, bserr := p.ParseJpBracketSeg()
		if bserr != nil {
			return nil, fmt.Errorf("jp_child_seg: %w", bserr)
		}
		return &V17JpChildSegNode{V17BaseNode{line, col}, bs}, nil
	}

	// jp_dot_seg starts with "."
	if p.peekAfterWS() == '.' {
		ds, dserr := p.ParseJpDotSeg()
		if dserr != nil {
			return nil, fmt.Errorf("jp_child_seg: %w", dserr)
		}
		return &V17JpChildSegNode{V17BaseNode{line, col}, ds}, nil
	}

	return nil, p.errAt("jp_child_seg: expected '[' or '.'")
}

// =============================================================================
// jp_desc_seg = ".." !WS! ( jp_name | jp_wildcard | jp_bracket_seg )
// =============================================================================

// V17JpDescSegNode  jp_desc_seg = ".." !WS! ( jp_name | jp_wildcard | jp_bracket_seg )
type V17JpDescSegNode struct {
	V17BaseNode
	// Value is *V17JpNameNode | *V17JpWildcardNode | *V17JpBracketSegNode
	Value interface{}
}

// ParseJpDescSeg parses jp_desc_seg = ".." !WS! ( jp_name | jp_wildcard | jp_bracket_seg ).
func (p *V17Parser) ParseJpDescSeg() (node *V17JpDescSegNode, err error) {
	done := p.debugEnter("jp_desc_seg")
	defer func() { done(err == nil) }()
	// Match ".." with WS skip, then no-WS for selector
	ddTok, dderr := p.matchLit("..")
	if dderr != nil {
		return nil, p.errAt("jp_desc_seg: expected '..'")
	}
	line, col := ddTok.Line, ddTok.Col

	// !WS! — selector must follow immediately
	switch {
	case p.runePos < len(p.input) && p.input[p.runePos] == '[':
		bs, bserr := p.ParseJpBracketSeg()
		if bserr != nil {
			return nil, fmt.Errorf("jp_desc_seg: %w", bserr)
		}
		return &V17JpDescSegNode{V17BaseNode{line, col}, bs}, nil
	case p.runePos < len(p.input) && p.input[p.runePos] == '*':
		wcTok, _ := p.matchLitNoWS("*")
		wc := &V17JpWildcardNode{V17BaseNode{wcTok.Line, wcTok.Col}}
		return &V17JpDescSegNode{V17BaseNode{line, col}, wc}, nil
	default:
		// jp_name (no-WS ident)
		tok, terr := p.matchReNoWS(reIdentNameScanNoWS)
		if terr != nil {
			return nil, fmt.Errorf("jp_desc_seg: expected name, '*', or '[' after '..' (no whitespace)")
		}
		in := &V17IdentNameNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}
		jn := &V17JpNameNode{V17BaseNode{tok.Line, tok.Col}, in}
		return &V17JpDescSegNode{V17BaseNode{line, col}, jn}, nil
	}
}

// =============================================================================
// jp_segment = jp_desc_seg | jp_child_seg
// =============================================================================

// V17JpSegmentNode  jp_segment = jp_desc_seg | jp_child_seg
type V17JpSegmentNode struct {
	V17BaseNode
	// Value is *V17JpDescSegNode | *V17JpChildSegNode
	Value interface{}
}

// ParseJpSegment parses jp_segment = jp_desc_seg | jp_child_seg.
func (p *V17Parser) ParseJpSegment() (node *V17JpSegmentNode, err error) {
	done := p.debugEnter("jp_segment")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// jp_desc_seg starts with ".." (must check before jp_dot_seg which starts with ".")
	if p.peekLit("..") {
		ds, dserr := p.ParseJpDescSeg()
		if dserr != nil {
			return nil, fmt.Errorf("jp_segment: %w", dserr)
		}
		return &V17JpSegmentNode{V17BaseNode{line, col}, ds}, nil
	}
	// jp_child_seg starts with "." or "["
	if ch := p.peekAfterWS(); ch == '.' || ch == '[' {
		cs, cserr := p.ParseJpChildSeg()
		if cserr != nil {
			return nil, fmt.Errorf("jp_segment: %w", cserr)
		}
		return &V17JpSegmentNode{V17BaseNode{line, col}, cs}, nil
	}
	return nil, p.errAt("jp_segment: expected '.', '..', or '['")
}

// =============================================================================
// json_path = "." !WS! "$" { jp_segment }
// =============================================================================

// V17JsonPathNode  json_path = "." !WS! "$" { jp_segment }
type V17JsonPathNode struct {
	V17BaseNode
	Segments []*V17JpSegmentNode // zero or more
}

// ParseJsonPath parses json_path = "." !WS! "$" { jp_segment }.
func (p *V17Parser) ParseJsonPath() (node *V17JsonPathNode, err error) {
	done := p.debugEnter("json_path")
	defer func() { done(err == nil) }()

	// Match "." (with WS skip) then immediately "$" (no WS)
	dotTok, doterr := p.matchLit(".")
	if doterr != nil {
		return nil, p.errAt("json_path: expected '.'")
	}
	_, dollerr := p.matchLitNoWS("$")
	if dollerr != nil {
		return nil, p.errAt("json_path: expected '$' immediately after '.' (no whitespace)")
	}
	line, col := dotTok.Line, dotTok.Col

	var segments []*V17JpSegmentNode
	for {
		saved := p.savePos()
		seg, segerr := p.ParseJpSegment()
		if segerr != nil {
			p.restorePos(saved)
			break
		}
		segments = append(segments, seg)
	}

	return &V17JsonPathNode{V17BaseNode{line, col}, segments}, nil
}

// Ensure fmt is used (referenced via error-wrapping in every Parse method).
var _ = fmt.Sprintf
