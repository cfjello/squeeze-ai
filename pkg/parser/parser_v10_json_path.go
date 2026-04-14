// parser_v10_json_path.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V10 grammar rule set defined in spec/05_json_path.sqg.
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

// V10JPNameNode  jp_name = ident_name | string
type V10JPNameNode struct {
	v10BaseNode
	Value string // plain name or unquoted string value
}

// V10JPWildcardNode  jp_wildcard = "*"
type V10JPWildcardNode struct{ v10BaseNode }

// V10JPIndexNode  jp_index = integer   (can be negative, represented as signed string)
type V10JPIndexNode struct {
	v10BaseNode
	Value string // e.g. "0", "-1"
}

// V10JPSliceNode  jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ]
type V10JPSliceNode struct {
	v10BaseNode
	Start *string // nil = omitted
	End   *string // nil = omitted
	Step  *string // nil = omitted
}

// V10JPFilterOperKind identifies the filter comparison operator.
type V10JPFilterOperKind int

const (
	JPFilterEqEq  V10JPFilterOperKind = iota // ==
	JPFilterNeq                              // !=
	JPFilterGEq                              // >=
	JPFilterLEq                              // <=
	JPFilterGt                               // >
	JPFilterLt                               // <
	JPFilterMatch                            // =~
)

// V10JPCurrentPathNode  jp_current_path = "@" { jp_segment }
type V10JPCurrentPathNode struct {
	v10BaseNode
	Segments []V10JPSegmentNode
}

// V10JPFilterValueNode  jp_filter_value = constant | jp_current_path | ident_ref
type V10JPFilterValueNode struct {
	v10BaseNode
	Value V10Node // *V10ConstantNode | *V10JPCurrentPathNode | *V10IdentRefNode
}

// V10JPFilterCmpNode  jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value
type V10JPFilterCmpNode struct {
	v10BaseNode
	Left  *V10JPFilterValueNode
	Oper  V10JPFilterOperKind
	Right *V10JPFilterValueNode
}

// V10JPFilterAtomNode  jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")"
type V10JPFilterAtomNode struct {
	v10BaseNode
	Value V10Node // *V10JPFilterCmpNode | *V10JPCurrentPathNode | *V10JPFilterExprNode
}

// V10JPFilterUnaryNode  jp_filter_unary = [ jp_filter_not ] jp_filter_atom
type V10JPFilterUnaryNode struct {
	v10BaseNode
	Not  bool
	Atom *V10JPFilterAtomNode
}

// V10JPFilterExprNode  jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
type V10JPFilterExprNode struct {
	v10BaseNode
	Head  *V10JPFilterUnaryNode
	Pairs []V10JPFilterExprPair
}

// V10JPFilterExprPair is one (op, operand) step in a logic chain.
type V10JPFilterExprPair struct {
	IsAnd bool // true=&&  false=||
	RHS   *V10JPFilterUnaryNode
}

// V10JPFilterNode  jp_filter = "?" "(" jp_filter_expr ")"
type V10JPFilterNode struct {
	v10BaseNode
	Expr *V10JPFilterExprNode
}

// V10JPSelectorNode holds one selector inside a bracket or union.
// Exactly one field is non-nil.
type V10JPSelectorNode struct {
	v10BaseNode
	Filter   *V10JPFilterNode   // ?(...)
	Slice    *V10JPSliceNode    // a:b:c
	Index    *V10JPIndexNode    // integer
	Name     *V10JPNameNode     // ident_name or string
	Wildcard *V10JPWildcardNode // *
}

// V10JPSelectorListNode holds one or more selectors from a union bracket.
type V10JPSelectorListNode struct {
	v10BaseNode
	Items []*V10JPSelectorNode
}

// V10JPBracketSegNode  jp_bracket_seg = "[" jp_selector_list "]"
type V10JPBracketSegNode struct {
	v10BaseNode
	Selectors *V10JPSelectorListNode
}

// V10JPDotSegNode  jp_dot_seg = "." ( jp_name | jp_wildcard )
type V10JPDotSegNode struct {
	v10BaseNode
	Name     *V10JPNameNode     // non-nil when name segment
	Wildcard *V10JPWildcardNode // non-nil when wildcard
}

// V10JPDescSegNode  jp_desc_seg = ".." ( jp_name | jp_wildcard | jp_bracket_seg )
type V10JPDescSegNode struct {
	v10BaseNode
	Name     *V10JPNameNode       // non-nil for ..name
	Wildcard *V10JPWildcardNode   // non-nil for ..*
	Bracket  *V10JPBracketSegNode // non-nil for ..[selectors]
}

// V10JPSegmentNode  jp_segment = jp_desc_seg | jp_child_seg
// Exactly one child is non-nil.
type V10JPSegmentNode struct {
	v10BaseNode
	Desc    *V10JPDescSegNode    // non-nil for recursive-descent
	Dot     *V10JPDotSegNode     // non-nil for .name / .*
	Bracket *V10JPBracketSegNode // non-nil for [selectors]
}

// V10JSONPathNode  json_path = ".$" { jp_segment }
type V10JSONPathNode struct {
	v10BaseNode
	Segments []V10JPSegmentNode
}

// V10IdentRefWithPathNode  EXTEND<ident_ref> — ident_ref with optional json_path suffix
type V10IdentRefWithPathNode struct {
	v10BaseNode
	Base *V10IdentRefNode
	Path *V10JSONPathNode // nil when no JSONPath suffix
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (05_json_path.sqg)
// =============================================================================

// ---------- jp_name / jp_wildcard ----------

// parseJPName parses:  jp_name = ident_name | string
func (p *V10Parser) parseJPName() (*V10JPNameNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	switch tok.Type {
	case V10_IDENT:
		p.advance()
		return &V10JPNameNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: tok.Value}, nil
	case V10_STRING:
		p.advance()
		return &V10JPNameNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: tok.Value}, nil
	}
	return nil, p.errAt(fmt.Sprintf("expected JSONPath name (ident or string), got %s %q", tok.Type, tok.Value))
}

// ---------- jp_index / jp_slice ----------

// parseJPSliceOrIndex tries jp_slice first, then jp_index.
// jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ]
// jp_index = integer
// Returns (*V10JPSliceNode, nil) or (*V10JPIndexNode, nil), or error.
func (p *V10Parser) parseJPSliceOrIndex() (V10Node, error) {
	line, col := p.cur().Line, p.cur().Col

	// Read optional leading integer (possibly preceded by '-').
	var startStr *string
	if p.cur().Type == V10_MINUS || p.cur().Type == V10_INTEGER {
		s := p.parseJPSignedInt()
		startStr = &s
	}

	// If next token is ':', this is a slice.
	if p.cur().Type == V10_COLON {
		p.advance() // consume ':'
		var endStr *string
		if p.cur().Type == V10_MINUS || p.cur().Type == V10_INTEGER {
			s := p.parseJPSignedInt()
			endStr = &s
		}
		var stepStr *string
		if p.cur().Type == V10_COLON {
			p.advance() // consume second ':'
			if p.cur().Type == V10_MINUS || p.cur().Type == V10_INTEGER {
				s := p.parseJPSignedInt()
				stepStr = &s
			}
		}
		return &V10JPSliceNode{
			v10BaseNode: v10BaseNode{Line: line, Col: col},
			Start:       startStr,
			End:         endStr,
			Step:        stepStr,
		}, nil
	}

	// No ':' — this is jp_index, startStr must have been set.
	if startStr == nil {
		return nil, p.errAt("expected integer index or slice in JSONPath bracket")
	}
	return &V10JPIndexNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: *startStr}, nil
}

// parseJPSignedInt reads an optional '-' followed by INTEGER and returns the combined string.
func (p *V10Parser) parseJPSignedInt() string {
	neg := ""
	if p.cur().Type == V10_MINUS {
		neg = "-"
		p.advance()
	}
	if p.cur().Type == V10_INTEGER {
		v := neg + p.cur().Value
		p.advance()
		return v
	}
	return neg
}

// ---------- jp_filter_value ----------

// parseJPFilterValue parses:  jp_filter_value = constant | jp_current_path | ident_ref
func (p *V10Parser) parseJPFilterValue() (*V10JPFilterValueNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// jp_current_path begins with '@'.
	if p.cur().Type == V10_AT {
		cp, err := p.parseJPCurrentPath()
		if err != nil {
			return nil, err
		}
		return &V10JPFilterValueNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: cp}, nil
	}

	// constant — try first for unambiguous literal tokens.
	if v10isConstantStart(p.cur().Type) {
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil {
			return &V10JPFilterValueNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: c}, nil
		}
		p.restorePos(saved)
	}

	// ident_ref
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	return &V10JPFilterValueNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: ref}, nil
}

// v10isConstantStart returns true for tokens that can only begin a constant literal.
func v10isConstantStart(t V10TokenType) bool {
	switch t {
	case V10_INTEGER, V10_DECIMAL, V10_STRING,
		V10_EMPTY_STR_D, V10_EMPTY_STR_S, V10_EMPTY_STR_T,
		V10_REGEXP, V10_REGEXP_DECL,
		V10_TRUE, V10_FALSE, V10_NULL,
		V10_NAN, V10_INFINITY,
		V10_PLUS, V10_MINUS:
		return true
	}
	return false
}

// ---------- jp_filter_oper ----------

type jpFilterOperResult struct {
	kind V10JPFilterOperKind
}

func (p *V10Parser) parseJPFilterOper() (V10JPFilterOperKind, error) {
	switch p.cur().Type {
	case V10_EQEQ:
		p.advance()
		return JPFilterEqEq, nil
	case V10_NEQ:
		p.advance()
		return JPFilterNeq, nil
	case V10_GEQ:
		p.advance()
		return JPFilterGEq, nil
	case V10_LEQ:
		p.advance()
		return JPFilterLEq, nil
	case V10_GT:
		p.advance()
		return JPFilterGt, nil
	case V10_LT:
		p.advance()
		return JPFilterLt, nil
	case V10_MATCH_OP:
		p.advance()
		return JPFilterMatch, nil
	}
	return 0, p.errAt(fmt.Sprintf("expected filter comparison operator, got %s %q", p.cur().Type, p.cur().Value))
}

// ---------- jp_filter_cmp ----------

// parseJPFilterCmp parses:  jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value
func (p *V10Parser) parseJPFilterCmp() (*V10JPFilterCmpNode, error) {
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
	return &V10JPFilterCmpNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        op,
		Right:       right,
	}, nil
}

// ---------- jp_filter_atom ----------

// parseJPFilterAtom parses:
//
//	jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")"
func (p *V10Parser) parseJPFilterAtom() (*V10JPFilterAtomNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Grouped expression "(" jp_filter_expr ")".
	if p.cur().Type == V10_LPAREN {
		p.advance()
		expr, err := p.ParseJPFilterExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V10_RPAREN); err != nil {
			return nil, err
		}
		return &V10JPFilterAtomNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: expr}, nil
	}

	// '@' → could be jp_current_path used as existence check OR the LHS of jp_filter_cmp.
	// Try comparison first (backtrack if no operator follows); fall back to bare current_path.
	if p.cur().Type == V10_AT {
		saved := p.savePos()
		if cmp, err := p.parseJPFilterCmp(); err == nil {
			return &V10JPFilterAtomNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: cmp}, nil
		}
		p.restorePos(saved)
		cp, err := p.parseJPCurrentPath()
		if err != nil {
			return nil, err
		}
		return &V10JPFilterAtomNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: cp}, nil
	}

	// Otherwise must be jp_filter_cmp.
	cmp, err := p.parseJPFilterCmp()
	if err != nil {
		return nil, err
	}
	return &V10JPFilterAtomNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: cmp}, nil
}

// ---------- jp_filter_unary ----------

// parseJPFilterUnary parses:  jp_filter_unary = [ "!" ] jp_filter_atom
func (p *V10Parser) parseJPFilterUnary() (*V10JPFilterUnaryNode, error) {
	line, col := p.cur().Line, p.cur().Col
	neg := false
	if p.cur().Type == V10_BANG {
		neg = true
		p.advance()
	}
	atom, err := p.parseJPFilterAtom()
	if err != nil {
		return nil, err
	}
	return &V10JPFilterUnaryNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Not: neg, Atom: atom}, nil
}

// ---------- jp_filter_expr ----------

// ParseJPFilterExpr parses:  jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
func (p *V10Parser) ParseJPFilterExpr() (*V10JPFilterExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	head, err := p.parseJPFilterUnary()
	if err != nil {
		return nil, err
	}
	node := &V10JPFilterExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Head: head}
	for p.cur().Type == V10_AMP_AMP || p.cur().Type == V10_PIPE_PIPE {
		isAnd := p.cur().Type == V10_AMP_AMP
		p.advance()
		rhs, err := p.parseJPFilterUnary()
		if err != nil {
			return nil, err
		}
		node.Pairs = append(node.Pairs, V10JPFilterExprPair{IsAnd: isAnd, RHS: rhs})
	}
	return node, nil
}

// ---------- jp_filter ----------

// parseJPFilter parses:  jp_filter = "?" "(" jp_filter_expr ")"
func (p *V10Parser) parseJPFilter() (*V10JPFilterNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_QUESTION); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_LPAREN); err != nil {
		return nil, err
	}
	expr, err := p.ParseJPFilterExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_RPAREN); err != nil {
		return nil, err
	}
	return &V10JPFilterNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Expr: expr}, nil
}

// ---------- jp_selector ----------

// parseJPSelector parses one selector:
//
//	jp_selector = jp_filter | jp_slice | jp_index | jp_name | jp_wildcard
//
// Order matters — filter and slice are tried before index and name.
func (p *V10Parser) parseJPSelector() (*V10JPSelectorNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// jp_filter: "?" "(" ...
	if p.cur().Type == V10_QUESTION {
		f, err := p.parseJPFilter()
		if err != nil {
			return nil, err
		}
		return &V10JPSelectorNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Filter: f}, nil
	}

	// jp_wildcard: "*"
	if p.cur().Type == V10_STAR {
		p.advance()
		return &V10JPSelectorNode{
			v10BaseNode: v10BaseNode{Line: line, Col: col},
			Wildcard:    &V10JPWildcardNode{v10BaseNode: v10BaseNode{Line: line, Col: col}},
		}, nil
	}

	// jp_slice or jp_index: both begin with optional integer then ':'
	if p.cur().Type == V10_INTEGER || p.cur().Type == V10_MINUS || p.cur().Type == V10_COLON {
		saved := p.savePos()
		val, err := p.parseJPSliceOrIndex()
		if err != nil {
			p.restorePos(saved)
		} else {
			switch v := val.(type) {
			case *V10JPSliceNode:
				return &V10JPSelectorNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Slice: v}, nil
			case *V10JPIndexNode:
				return &V10JPSelectorNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Index: v}, nil
			}
		}
		p.restorePos(saved)
	}

	// jp_name: ident_name | string
	n, err := p.parseJPName()
	if err != nil {
		return nil, err
	}
	return &V10JPSelectorNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Name: n}, nil
}

// ---------- jp_selector_list / jp_bracket_seg ----------

// parseJPSelectorList parses:  jp_selector_list = jp_selector { "," jp_selector }
func (p *V10Parser) parseJPSelectorList() (*V10JPSelectorListNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.parseJPSelector()
	if err != nil {
		return nil, err
	}
	items := []*V10JPSelectorNode{first}
	for p.cur().Type == V10_COMMA {
		p.advance()
		sel, err := p.parseJPSelector()
		if err != nil {
			return nil, err
		}
		items = append(items, sel)
	}
	return &V10JPSelectorListNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Items: items}, nil
}

// parseJPBracketSeg parses:  jp_bracket_seg = "[" jp_selector_list "]"
func (p *V10Parser) parseJPBracketSeg() (*V10JPBracketSegNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_LBRACKET); err != nil {
		return nil, err
	}
	sels, err := p.parseJPSelectorList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_RBRACKET); err != nil {
		return nil, err
	}
	return &V10JPBracketSegNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Selectors: sels}, nil
}

// ---------- jp_dot_seg / jp_desc_seg / jp_segment ----------

// parseJPDotSeg parses:  jp_dot_seg = "." ( jp_name | jp_wildcard )
// The "." has already been consumed by the caller.
func (p *V10Parser) parseJPDotSegAfterDot(line, col int) (*V10JPDotSegNode, error) {
	if p.cur().Type == V10_STAR {
		p.advance()
		return &V10JPDotSegNode{
			v10BaseNode: v10BaseNode{Line: line, Col: col},
			Wildcard:    &V10JPWildcardNode{v10BaseNode: v10BaseNode{Line: line, Col: col}},
		}, nil
	}
	n, err := p.parseJPName()
	if err != nil {
		return nil, err
	}
	return &V10JPDotSegNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Name: n}, nil
}

// parseJPSegment parses:  jp_segment = jp_desc_seg | jp_child_seg
//
// jp_desc_seg  = ".." ( jp_name | jp_wildcard | jp_bracket_seg )
// jp_child_seg = jp_bracket_seg | jp_dot_seg
func (p *V10Parser) parseJPSegment() (*V10JPSegmentNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Bracket child segment: "[" ...
	if p.cur().Type == V10_LBRACKET {
		bs, err := p.parseJPBracketSeg()
		if err != nil {
			return nil, err
		}
		return &V10JPSegmentNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Bracket: bs}, nil
	}

	// ".." recursive-descent segment.
	if p.cur().Type == V10_DOTDOT {
		p.advance() // consume ".."
		var desc V10JPDescSegNode
		desc.v10BaseNode = v10BaseNode{Line: line, Col: col}
		switch p.cur().Type {
		case V10_LBRACKET:
			bs, err := p.parseJPBracketSeg()
			if err != nil {
				return nil, err
			}
			desc.Bracket = bs
		case V10_STAR:
			p.advance()
			desc.Wildcard = &V10JPWildcardNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}
		default:
			n, err := p.parseJPName()
			if err != nil {
				return nil, err
			}
			desc.Name = n
		}
		return &V10JPSegmentNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Desc: &desc}, nil
	}

	// "." child dot segment.
	if p.cur().Type == V10_DOT {
		p.advance() // consume "."
		ds, err := p.parseJPDotSegAfterDot(line, col)
		if err != nil {
			return nil, err
		}
		return &V10JPSegmentNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Dot: ds}, nil
	}

	return nil, p.errAt(fmt.Sprintf("expected JSONPath segment (., .., or [), got %s %q", p.cur().Type, p.cur().Value))
}

// ---------- jp_current_path ----------

// parseJPCurrentPath parses:  jp_current_path = "@" { jp_segment }
func (p *V10Parser) parseJPCurrentPath() (*V10JPCurrentPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_AT); err != nil {
		return nil, err
	}
	node := &V10JPCurrentPathNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}
	for p.cur().Type == V10_DOT || p.cur().Type == V10_DOTDOT || p.cur().Type == V10_LBRACKET {
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
//
// The ".$" compound token is V10_DOTDOLLAR.
func (p *V10Parser) ParseJSONPath() (*V10JSONPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_DOTDOLLAR); err != nil {
		return nil, err
	}
	node := &V10JSONPathNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}
	for p.cur().Type == V10_DOT || p.cur().Type == V10_DOTDOT || p.cur().Type == V10_LBRACKET {
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
//
// This extends ident_ref so that any reference may be followed by a JSONPath
// query beginning with ".$".  Without ".$" the rule reduces to a plain ident_ref.
func (p *V10Parser) ParseIdentRefWithPath() (*V10IdentRefWithPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	node := &V10IdentRefWithPathNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Base: ref}
	if p.cur().Type == V10_DOTDOLLAR {
		jp, err := p.ParseJSONPath()
		if err != nil {
			return nil, err
		}
		node.Path = jp
	}
	return node, nil
}
