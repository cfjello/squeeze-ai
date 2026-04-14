// parser_v13_json_path.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/05_json_path.sqg.
//
// V13 changes vs V12:  none (all rules carry forward unchanged).
//
// Covered rules:
//
//	jp_name, jp_wildcard
//	jp_index, jp_slice
//	jp_filter_value, jp_filter_oper, jp_filter_cmp,
//	jp_filter_not, jp_filter_logic
//	jp_filter_atom, jp_filter_unary, jp_filter_expr
//	jp_filter
//	jp_selector, jp_selector_list, jp_bracket_seg
//	jp_dot_seg, jp_child_seg, jp_desc_seg, jp_segment
//	json_path
//	EXTEND<ident_ref>  (json_path suffix on ident_ref)
package parser

import "fmt"

// =============================================================================
// PHASE 2 — AST NODE TYPES  (05_json_path.sqg)
// =============================================================================

// V13JPNameNode  jp_name = ident_name | string
type V13JPNameNode struct {
	V13BaseNode
	Value string
}

// V13JPWildcardNode  jp_wildcard = "*"
type V13JPWildcardNode struct{ V13BaseNode }

// V13JPIndexNode  jp_index = integer  (can be negative)
type V13JPIndexNode struct {
	V13BaseNode
	Value string // e.g. "0", "-1"
}

// V13JPSliceNode  jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ]
type V13JPSliceNode struct {
	V13BaseNode
	Start *string
	End   *string
	Step  *string
}

// V13JPFilterOperKind identifies the filter comparison operator.
type V13JPFilterOperKind int

const (
	V13JPFilterEqEq  V13JPFilterOperKind = iota // ==
	V13JPFilterNeq                              // !=
	V13JPFilterGEq                              // >=
	V13JPFilterLEq                              // <=
	V13JPFilterGt                               // >
	V13JPFilterLt                               // <
	V13JPFilterMatch                            // =~
)

// V13JPCurrentPathNode  jp_current_path = "@" { jp_segment }
type V13JPCurrentPathNode struct {
	V13BaseNode
	Segments []V13JPSegmentNode
}

// V13JPFilterValueNode  jp_filter_value = constant | jp_current_path | ident_ref
type V13JPFilterValueNode struct {
	V13BaseNode
	Value V13Node
}

// V13JPFilterCmpNode  jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value
type V13JPFilterCmpNode struct {
	V13BaseNode
	Left  *V13JPFilterValueNode
	Oper  V13JPFilterOperKind
	Right *V13JPFilterValueNode
}

// V13JPFilterAtomNode  jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")"
type V13JPFilterAtomNode struct {
	V13BaseNode
	Value V13Node
}

// V13JPFilterUnaryNode  jp_filter_unary = [ "!" ] jp_filter_atom
type V13JPFilterUnaryNode struct {
	V13BaseNode
	Not  bool
	Atom *V13JPFilterAtomNode
}

// V13JPFilterExprNode  jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
type V13JPFilterExprNode struct {
	V13BaseNode
	Head  *V13JPFilterUnaryNode
	Pairs []V13JPFilterExprPair
}

// V13JPFilterExprPair is one (op, operand) step in a logic chain.
type V13JPFilterExprPair struct {
	IsAnd bool
	RHS   *V13JPFilterUnaryNode
}

// V13JPFilterNode  jp_filter = "?" "(" jp_filter_expr ")"
type V13JPFilterNode struct {
	V13BaseNode
	Expr *V13JPFilterExprNode
}

// V13JPSelectorNode holds one selector inside a bracket or union.
type V13JPSelectorNode struct {
	V13BaseNode
	Filter   *V13JPFilterNode
	Slice    *V13JPSliceNode
	Index    *V13JPIndexNode
	Name     *V13JPNameNode
	Wildcard *V13JPWildcardNode
}

// V13JPSelectorListNode holds one or more selectors from a union bracket.
type V13JPSelectorListNode struct {
	V13BaseNode
	Items []*V13JPSelectorNode
}

// V13JPBracketSegNode  jp_bracket_seg = "[" jp_selector_list "]"
type V13JPBracketSegNode struct {
	V13BaseNode
	Selectors *V13JPSelectorListNode
}

// V13JPDotSegNode  jp_dot_seg = "." ( jp_name | jp_wildcard )
type V13JPDotSegNode struct {
	V13BaseNode
	Name     *V13JPNameNode
	Wildcard *V13JPWildcardNode
}

// V13JPDescSegNode  jp_desc_seg = ".." ( jp_name | jp_wildcard | jp_bracket_seg )
type V13JPDescSegNode struct {
	V13BaseNode
	Name     *V13JPNameNode
	Wildcard *V13JPWildcardNode
	Bracket  *V13JPBracketSegNode
}

// V13JPSegmentNode  jp_segment = jp_desc_seg | jp_child_seg
type V13JPSegmentNode struct {
	V13BaseNode
	Desc    *V13JPDescSegNode
	Dot     *V13JPDotSegNode
	Bracket *V13JPBracketSegNode
}

// V13JSONPathNode  json_path = ".$" { jp_segment }
type V13JSONPathNode struct {
	V13BaseNode
	Segments []V13JPSegmentNode
}

// V13IdentRefWithPathNode  EXTEND<ident_ref> — ident_ref with optional json_path suffix
type V13IdentRefWithPathNode struct {
	V13BaseNode
	Base *V13IdentRefNode
	Path *V13JSONPathNode // nil when no JSONPath suffix
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (05_json_path.sqg)
// =============================================================================

// ---------- jp_name / jp_wildcard ----------

func (p *V13Parser) parseJPName() (*V13JPNameNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	switch tok.Type {
	case V13_IDENT:
		p.advance()
		return &V13JPNameNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: tok.Value}, nil
	case V13_STRING:
		p.advance()
		return &V13JPNameNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: tok.Value}, nil
	}
	return nil, p.errAt(fmt.Sprintf("expected JSONPath name (ident or string), got %s %q", tok.Type, tok.Value))
}

// ---------- jp_index / jp_slice ----------

func (p *V13Parser) parseJPSliceOrIndex() (V13Node, error) {
	line, col := p.cur().Line, p.cur().Col

	var startStr *string
	if p.cur().Type == V13_MINUS || p.cur().Type == V13_INTEGER {
		s := p.parseJPSignedInt()
		startStr = &s
	}

	if p.cur().Type == V13_COLON {
		p.advance()
		var endStr *string
		if p.cur().Type == V13_MINUS || p.cur().Type == V13_INTEGER {
			s := p.parseJPSignedInt()
			endStr = &s
		}
		var stepStr *string
		if p.cur().Type == V13_COLON {
			p.advance()
			if p.cur().Type == V13_MINUS || p.cur().Type == V13_INTEGER {
				s := p.parseJPSignedInt()
				stepStr = &s
			}
		}
		return &V13JPSliceNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			Start:       startStr,
			End:         endStr,
			Step:        stepStr,
		}, nil
	}

	if startStr == nil {
		return nil, p.errAt("expected integer index or slice in JSONPath bracket")
	}
	return &V13JPIndexNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: *startStr}, nil
}

func (p *V13Parser) parseJPSignedInt() string {
	neg := ""
	if p.cur().Type == V13_MINUS {
		neg = "-"
		p.advance()
	}
	if p.cur().Type == V13_INTEGER {
		v := neg + p.cur().Value
		p.advance()
		return v
	}
	return neg
}

// ---------- jp_filter_value ----------

// V13isConstantStart returns true for tokens that can only begin a constant literal.
func V13isConstantStart(t V13TokenType) bool {
	switch t {
	case V13_INTEGER, V13_DECIMAL, V13_STRING,
		V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T,
		V13_REGEXP, V13_REGEXP_DECL,
		V13_TRUE, V13_FALSE, V13_NULL,
		V13_NAN, V13_INFINITY,
		V13_PLUS, V13_MINUS:
		return true
	}
	return false
}

func (p *V13Parser) parseJPFilterValue() (*V13JPFilterValueNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V13_AT {
		cp, err := p.parseJPCurrentPath()
		if err != nil {
			return nil, err
		}
		return &V13JPFilterValueNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: cp}, nil
	}

	if V13isConstantStart(p.cur().Type) {
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil {
			return &V13JPFilterValueNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: c}, nil
		}
		p.restorePos(saved)
	}

	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	return &V13JPFilterValueNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: ref}, nil
}

// ---------- jp_filter_oper ----------

func (p *V13Parser) parseJPFilterOper() (V13JPFilterOperKind, error) {
	switch p.cur().Type {
	case V13_EQEQ:
		p.advance()
		return V13JPFilterEqEq, nil
	case V13_NEQ:
		p.advance()
		return V13JPFilterNeq, nil
	case V13_GEQ:
		p.advance()
		return V13JPFilterGEq, nil
	case V13_LEQ:
		p.advance()
		return V13JPFilterLEq, nil
	case V13_GT:
		p.advance()
		return V13JPFilterGt, nil
	case V13_LT:
		p.advance()
		return V13JPFilterLt, nil
	case V13_MATCH_OP:
		p.advance()
		return V13JPFilterMatch, nil
	}
	return 0, p.errAt(fmt.Sprintf("expected filter comparison operator, got %s %q", p.cur().Type, p.cur().Value))
}

// ---------- jp_filter_cmp ----------

func (p *V13Parser) parseJPFilterCmp() (*V13JPFilterCmpNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.parseJPFilterValue()
	if err != nil {
		return nil, err
	}
	op, err := p.parseJPFilterOper()
	if err != nil {
		return nil, err
	}
	right, err := p.parseJPFilterValue()
	if err != nil {
		return nil, err
	}
	return &V13JPFilterCmpNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        op,
		Right:       right,
	}, nil
}

// ---------- jp_filter_atom ----------

func (p *V13Parser) parseJPFilterAtom() (*V13JPFilterAtomNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V13_LPAREN {
		p.advance()
		expr, err := p.ParseJPFilterExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V13_RPAREN); err != nil {
			return nil, err
		}
		return &V13JPFilterAtomNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: expr}, nil
	}

	if p.cur().Type == V13_AT {
		saved := p.savePos()
		if cmp, err := p.parseJPFilterCmp(); err == nil {
			return &V13JPFilterAtomNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: cmp}, nil
		}
		p.restorePos(saved)
		cp, err := p.parseJPCurrentPath()
		if err != nil {
			return nil, err
		}
		return &V13JPFilterAtomNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: cp}, nil
	}

	cmp, err := p.parseJPFilterCmp()
	if err != nil {
		return nil, err
	}
	return &V13JPFilterAtomNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: cmp}, nil
}

// ---------- jp_filter_unary ----------

func (p *V13Parser) parseJPFilterUnary() (*V13JPFilterUnaryNode, error) {
	line, col := p.cur().Line, p.cur().Col
	neg := false
	if p.cur().Type == V13_BANG {
		neg = true
		p.advance()
	}
	atom, err := p.parseJPFilterAtom()
	if err != nil {
		return nil, err
	}
	return &V13JPFilterUnaryNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Not: neg, Atom: atom}, nil
}

// ---------- jp_filter_expr ----------

// ParseJPFilterExpr parses:  jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
func (p *V13Parser) ParseJPFilterExpr() (*V13JPFilterExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	head, err := p.parseJPFilterUnary()
	if err != nil {
		return nil, err
	}
	node := &V13JPFilterExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Head: head}
	for p.cur().Type == V13_AMP_AMP || p.cur().Type == V13_PIPE_PIPE {
		isAnd := p.cur().Type == V13_AMP_AMP
		p.advance()
		rhs, err := p.parseJPFilterUnary()
		if err != nil {
			return nil, err
		}
		node.Pairs = append(node.Pairs, V13JPFilterExprPair{IsAnd: isAnd, RHS: rhs})
	}
	return node, nil
}

// ---------- jp_filter ----------

func (p *V13Parser) parseJPFilter() (*V13JPFilterNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_QUESTION); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	expr, err := p.ParseJPFilterExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return &V13JPFilterNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Expr: expr}, nil
}

// ---------- jp_selector ----------

func (p *V13Parser) parseJPSelector() (*V13JPSelectorNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V13_QUESTION {
		f, err := p.parseJPFilter()
		if err != nil {
			return nil, err
		}
		return &V13JPSelectorNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Filter: f}, nil
	}

	if p.cur().Type == V13_STAR {
		p.advance()
		return &V13JPSelectorNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			Wildcard:    &V13JPWildcardNode{V13BaseNode: V13BaseNode{Line: line, Col: col}},
		}, nil
	}

	if p.cur().Type == V13_INTEGER || p.cur().Type == V13_MINUS || p.cur().Type == V13_COLON {
		saved := p.savePos()
		val, err := p.parseJPSliceOrIndex()
		if err == nil {
			switch v := val.(type) {
			case *V13JPSliceNode:
				return &V13JPSelectorNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Slice: v}, nil
			case *V13JPIndexNode:
				return &V13JPSelectorNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Index: v}, nil
			}
		}
		p.restorePos(saved)
	}

	n, err := p.parseJPName()
	if err != nil {
		return nil, err
	}
	return &V13JPSelectorNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: n}, nil
}

// ---------- jp_selector_list / jp_bracket_seg ----------

func (p *V13Parser) parseJPSelectorList() (*V13JPSelectorListNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.parseJPSelector()
	if err != nil {
		return nil, err
	}
	items := []*V13JPSelectorNode{first}
	for p.cur().Type == V13_COMMA {
		p.advance()
		sel, err := p.parseJPSelector()
		if err != nil {
			return nil, err
		}
		items = append(items, sel)
	}
	return &V13JPSelectorListNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Items: items}, nil
}

func (p *V13Parser) parseJPBracketSeg() (*V13JPBracketSegNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	sels, err := p.parseJPSelectorList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13JPBracketSegNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Selectors: sels}, nil
}

// ---------- jp_dot_seg / jp_desc_seg / jp_segment ----------

func (p *V13Parser) parseJPDotSegAfterDot(line, col int) (*V13JPDotSegNode, error) {
	if p.cur().Type == V13_STAR {
		p.advance()
		return &V13JPDotSegNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			Wildcard:    &V13JPWildcardNode{V13BaseNode: V13BaseNode{Line: line, Col: col}},
		}, nil
	}
	n, err := p.parseJPName()
	if err != nil {
		return nil, err
	}
	return &V13JPDotSegNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: n}, nil
}

func (p *V13Parser) parseJPSegment() (*V13JPSegmentNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V13_LBRACKET {
		bs, err := p.parseJPBracketSeg()
		if err != nil {
			return nil, err
		}
		return &V13JPSegmentNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Bracket: bs}, nil
	}

	if p.cur().Type == V13_DOTDOT {
		p.advance()
		var desc V13JPDescSegNode
		desc.V13BaseNode = V13BaseNode{Line: line, Col: col}
		switch p.cur().Type {
		case V13_LBRACKET:
			bs, err := p.parseJPBracketSeg()
			if err != nil {
				return nil, err
			}
			desc.Bracket = bs
		case V13_STAR:
			p.advance()
			desc.Wildcard = &V13JPWildcardNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}
		default:
			n, err := p.parseJPName()
			if err != nil {
				return nil, err
			}
			desc.Name = n
		}
		return &V13JPSegmentNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Desc: &desc}, nil
	}

	if p.cur().Type == V13_DOT {
		p.advance()
		ds, err := p.parseJPDotSegAfterDot(line, col)
		if err != nil {
			return nil, err
		}
		return &V13JPSegmentNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Dot: ds}, nil
	}

	return nil, p.errAt(fmt.Sprintf("expected JSONPath segment (., .., or [), got %s %q", p.cur().Type, p.cur().Value))
}

// ---------- jp_current_path ----------

func (p *V13Parser) parseJPCurrentPath() (*V13JPCurrentPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_AT); err != nil {
		return nil, err
	}
	node := &V13JPCurrentPathNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}
	for p.cur().Type == V13_DOT || p.cur().Type == V13_DOTDOT || p.cur().Type == V13_LBRACKET {
		seg, err := p.parseJPSegment()
		if err != nil {
			break
		}
		node.Segments = append(node.Segments, *seg)
	}
	return node, nil
}

// ---------- json_path ----------

// ParseJSONPath parses:  json_path = ".$" { jp_segment }
func (p *V13Parser) ParseJSONPath() (*V13JSONPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_DOTDOLLAR); err != nil {
		return nil, err
	}
	node := &V13JSONPathNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}
	for p.cur().Type == V13_DOT || p.cur().Type == V13_DOTDOT || p.cur().Type == V13_LBRACKET {
		seg, err := p.parseJPSegment()
		if err != nil {
			break
		}
		node.Segments = append(node.Segments, *seg)
	}
	return node, nil
}

// ---------- EXTEND<ident_ref> ----------

// ParseIdentRefWithPath parses:  ident_ref [ json_path ]
func (p *V13Parser) ParseIdentRefWithPath() (*V13IdentRefWithPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	node := &V13IdentRefWithPathNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: ref}
	if p.cur().Type == V13_DOTDOLLAR {
		jp, err := p.ParseJSONPath()
		if err != nil {
			return nil, err
		}
		node.Path = jp
	}
	return node, nil
}
