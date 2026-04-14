// parser_V12_json_path.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V12 grammar rule set defined in spec/05_json_path.sqg.
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

// V12JPNameNode  jp_name = ident_name | string
type V12JPNameNode struct {
	V12BaseNode
	Value string // plain name or unquoted string value
}

// V12JPWildcardNode  jp_wildcard = "*"
type V12JPWildcardNode struct{ V12BaseNode }

// V12JPIndexNode  jp_index = integer   (can be negative, represented as signed string)
type V12JPIndexNode struct {
	V12BaseNode
	Value string // e.g. "0", "-1"
}

// V12JPSliceNode  jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ]
type V12JPSliceNode struct {
	V12BaseNode
	Start *string // nil = omitted
	End   *string // nil = omitted
	Step  *string // nil = omitted
}

// V12JPFilterOperKind identifies the filter comparison operator.
type V12JPFilterOperKind int

const (
	V12JPFilterEqEq  V12JPFilterOperKind = iota // ==
	V12JPFilterNeq                              // !=
	V12JPFilterGEq                              // >=
	V12JPFilterLEq                              // <=
	V12JPFilterGt                               // >
	V12JPFilterLt                               // <
	V12JPFilterMatch                            // =~
)

// V12JPCurrentPathNode  jp_current_path = "@" { jp_segment }
type V12JPCurrentPathNode struct {
	V12BaseNode
	Segments []V12JPSegmentNode
}

// V12JPFilterValueNode  jp_filter_value = constant | jp_current_path | ident_ref
type V12JPFilterValueNode struct {
	V12BaseNode
	Value V12Node // *V12ConstantNode | *V12JPCurrentPathNode | *V12IdentRefNode
}

// V12JPFilterCmpNode  jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value
type V12JPFilterCmpNode struct {
	V12BaseNode
	Left  *V12JPFilterValueNode
	Oper  V12JPFilterOperKind
	Right *V12JPFilterValueNode
}

// V12JPFilterAtomNode  jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")"
type V12JPFilterAtomNode struct {
	V12BaseNode
	Value V12Node // *V12JPFilterCmpNode | *V12JPCurrentPathNode | *V12JPFilterExprNode
}

// V12JPFilterUnaryNode  jp_filter_unary = [ jp_filter_not ] jp_filter_atom
type V12JPFilterUnaryNode struct {
	V12BaseNode
	Not  bool
	Atom *V12JPFilterAtomNode
}

// V12JPFilterExprNode  jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
type V12JPFilterExprNode struct {
	V12BaseNode
	Head  *V12JPFilterUnaryNode
	Pairs []V12JPFilterExprPair
}

// V12JPFilterExprPair is one (op, operand) step in a logic chain.
type V12JPFilterExprPair struct {
	IsAnd bool // true=&&  false=||
	RHS   *V12JPFilterUnaryNode
}

// V12JPFilterNode  jp_filter = "?" "(" jp_filter_expr ")"
type V12JPFilterNode struct {
	V12BaseNode
	Expr *V12JPFilterExprNode
}

// V12JPSelectorNode holds one selector inside a bracket or union.
// Exactly one field is non-nil.
type V12JPSelectorNode struct {
	V12BaseNode
	Filter   *V12JPFilterNode   // ?(...)
	Slice    *V12JPSliceNode    // a:b:c
	Index    *V12JPIndexNode    // integer
	Name     *V12JPNameNode     // ident_name or string
	Wildcard *V12JPWildcardNode // *
}

// V12JPSelectorListNode holds one or more selectors from a union bracket.
type V12JPSelectorListNode struct {
	V12BaseNode
	Items []*V12JPSelectorNode
}

// V12JPBracketSegNode  jp_bracket_seg = "[" jp_selector_list "]"
type V12JPBracketSegNode struct {
	V12BaseNode
	Selectors *V12JPSelectorListNode
}

// V12JPDotSegNode  jp_dot_seg = "." ( jp_name | jp_wildcard )
type V12JPDotSegNode struct {
	V12BaseNode
	Name     *V12JPNameNode     // non-nil when name segment
	Wildcard *V12JPWildcardNode // non-nil when wildcard
}

// V12JPDescSegNode  jp_desc_seg = ".." ( jp_name | jp_wildcard | jp_bracket_seg )
type V12JPDescSegNode struct {
	V12BaseNode
	Name     *V12JPNameNode       // non-nil for ..name
	Wildcard *V12JPWildcardNode   // non-nil for ..*
	Bracket  *V12JPBracketSegNode // non-nil for ..[selectors]
}

// V12JPSegmentNode  jp_segment = jp_desc_seg | jp_child_seg
// Exactly one child is non-nil.
type V12JPSegmentNode struct {
	V12BaseNode
	Desc    *V12JPDescSegNode    // non-nil for recursive-descent
	Dot     *V12JPDotSegNode     // non-nil for .name / .*
	Bracket *V12JPBracketSegNode // non-nil for [selectors]
}

// V12JSONPathNode  json_path = ".$" { jp_segment }
type V12JSONPathNode struct {
	V12BaseNode
	Segments []V12JPSegmentNode
}

// V12IdentRefWithPathNode  EXTEND<ident_ref> — ident_ref with optional json_path suffix
type V12IdentRefWithPathNode struct {
	V12BaseNode
	Base *V12IdentRefNode
	Path *V12JSONPathNode // nil when no JSONPath suffix
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (05_json_path.sqg)
// =============================================================================

// ---------- jp_name / jp_wildcard ----------

// parseJPName parses:  jp_name = ident_name | string
func (p *V12Parser) parseJPName() (*V12JPNameNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	switch tok.Type {
	case V12_IDENT:
		p.advance()
		return &V12JPNameNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: tok.Value}, nil
	case V12_STRING:
		p.advance()
		return &V12JPNameNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: tok.Value}, nil
	}
	return nil, p.errAt(fmt.Sprintf("expected JSONPath name (ident or string), got %s %q", tok.Type, tok.Value))
}

// ---------- jp_index / jp_slice ----------

// parseJPSliceOrIndex tries jp_slice first, then jp_index.
// jp_slice = [ integer ] ":" [ integer ] [ ":" [ integer ] ]
// jp_index = integer
// Returns (*V12JPSliceNode, nil) or (*V12JPIndexNode, nil), or error.
func (p *V12Parser) parseJPSliceOrIndex() (V12Node, error) {
	line, col := p.cur().Line, p.cur().Col

	// Read optional leading integer (possibly preceded by '-').
	var startStr *string
	if p.cur().Type == V12_MINUS || p.cur().Type == V12_INTEGER {
		s := p.parseJPSignedInt()
		startStr = &s
	}

	// If next token is ':', this is a slice.
	if p.cur().Type == V12_COLON {
		p.advance() // consume ':'
		var endStr *string
		if p.cur().Type == V12_MINUS || p.cur().Type == V12_INTEGER {
			s := p.parseJPSignedInt()
			endStr = &s
		}
		var stepStr *string
		if p.cur().Type == V12_COLON {
			p.advance() // consume second ':'
			if p.cur().Type == V12_MINUS || p.cur().Type == V12_INTEGER {
				s := p.parseJPSignedInt()
				stepStr = &s
			}
		}
		return &V12JPSliceNode{
			V12BaseNode: V12BaseNode{Line: line, Col: col},
			Start:       startStr,
			End:         endStr,
			Step:        stepStr,
		}, nil
	}

	// No ':' — this is jp_index, startStr must have been set.
	if startStr == nil {
		return nil, p.errAt("expected integer index or slice in JSONPath bracket")
	}
	return &V12JPIndexNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: *startStr}, nil
}

// parseJPSignedInt reads an optional '-' followed by INTEGER and returns the combined string.
func (p *V12Parser) parseJPSignedInt() string {
	neg := ""
	if p.cur().Type == V12_MINUS {
		neg = "-"
		p.advance()
	}
	if p.cur().Type == V12_INTEGER {
		v := neg + p.cur().Value
		p.advance()
		return v
	}
	return neg
}

// ---------- jp_filter_value ----------

// parseJPFilterValue parses:  jp_filter_value = constant | jp_current_path | ident_ref
func (p *V12Parser) parseJPFilterValue() (*V12JPFilterValueNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// jp_current_path begins with '@'.
	if p.cur().Type == V12_AT {
		cp, err := p.parseJPCurrentPath()
		if err != nil {
			return nil, err
		}
		return &V12JPFilterValueNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: cp}, nil
	}

	// constant — try first for unambiguous literal tokens.
	if V12isConstantStart(p.cur().Type) {
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil {
			return &V12JPFilterValueNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: c}, nil
		}
		p.restorePos(saved)
	}

	// ident_ref
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	return &V12JPFilterValueNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: ref}, nil
}

// V12isConstantStart returns true for tokens that can only begin a constant literal.
func V12isConstantStart(t V12TokenType) bool {
	switch t {
	case V12_INTEGER, V12_DECIMAL, V12_STRING,
		V12_EMPTY_STR_D, V12_EMPTY_STR_S, V12_EMPTY_STR_T,
		V12_REGEXP, V12_REGEXP_DECL,
		V12_TRUE, V12_FALSE, V12_NULL,
		V12_NAN, V12_INFINITY,
		V12_PLUS, V12_MINUS:
		return true
	}
	return false
}

// ---------- jp_filter_oper ----------

type v12jpFilterOperResult struct {
	kind V12JPFilterOperKind
}

func (p *V12Parser) parseJPFilterOper() (V12JPFilterOperKind, error) {
	switch p.cur().Type {
	case V12_EQEQ:
		p.advance()
		return V12JPFilterEqEq, nil
	case V12_NEQ:
		p.advance()
		return V12JPFilterNeq, nil
	case V12_GEQ:
		p.advance()
		return V12JPFilterGEq, nil
	case V12_LEQ:
		p.advance()
		return V12JPFilterLEq, nil
	case V12_GT:
		p.advance()
		return V12JPFilterGt, nil
	case V12_LT:
		p.advance()
		return V12JPFilterLt, nil
	case V12_MATCH_OP:
		p.advance()
		return V12JPFilterMatch, nil
	}
	return 0, p.errAt(fmt.Sprintf("expected filter comparison operator, got %s %q", p.cur().Type, p.cur().Value))
}

// ---------- jp_filter_cmp ----------

// parseJPFilterCmp parses:  jp_filter_cmp = jp_filter_value jp_filter_oper jp_filter_value
func (p *V12Parser) parseJPFilterCmp() (*V12JPFilterCmpNode, error) {
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
	return &V12JPFilterCmpNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        op,
		Right:       right,
	}, nil
}

// ---------- jp_filter_atom ----------

// parseJPFilterAtom parses:
//
//	jp_filter_atom = jp_filter_cmp | jp_current_path | "(" jp_filter_expr ")"
func (p *V12Parser) parseJPFilterAtom() (*V12JPFilterAtomNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Grouped expression "(" jp_filter_expr ")".
	if p.cur().Type == V12_LPAREN {
		p.advance()
		expr, err := p.ParseJPFilterExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V12_RPAREN); err != nil {
			return nil, err
		}
		return &V12JPFilterAtomNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: expr}, nil
	}

	// '@' → could be jp_current_path used as existence check OR the LHS of jp_filter_cmp.
	// Try comparison first (backtrack if no operator follows); fall back to bare current_path.
	if p.cur().Type == V12_AT {
		saved := p.savePos()
		if cmp, err := p.parseJPFilterCmp(); err == nil {
			return &V12JPFilterAtomNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: cmp}, nil
		}
		p.restorePos(saved)
		cp, err := p.parseJPCurrentPath()
		if err != nil {
			return nil, err
		}
		return &V12JPFilterAtomNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: cp}, nil
	}

	// Otherwise must be jp_filter_cmp.
	cmp, err := p.parseJPFilterCmp()
	if err != nil {
		return nil, err
	}
	return &V12JPFilterAtomNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: cmp}, nil
}

// ---------- jp_filter_unary ----------

// parseJPFilterUnary parses:  jp_filter_unary = [ "!" ] jp_filter_atom
func (p *V12Parser) parseJPFilterUnary() (*V12JPFilterUnaryNode, error) {
	line, col := p.cur().Line, p.cur().Col
	neg := false
	if p.cur().Type == V12_BANG {
		neg = true
		p.advance()
	}
	atom, err := p.parseJPFilterAtom()
	if err != nil {
		return nil, err
	}
	return &V12JPFilterUnaryNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Not: neg, Atom: atom}, nil
}

// ---------- jp_filter_expr ----------

// ParseJPFilterExpr parses:  jp_filter_expr = jp_filter_unary { jp_filter_logic jp_filter_unary }
func (p *V12Parser) ParseJPFilterExpr() (*V12JPFilterExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	head, err := p.parseJPFilterUnary()
	if err != nil {
		return nil, err
	}
	node := &V12JPFilterExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Head: head}
	for p.cur().Type == V12_AMP_AMP || p.cur().Type == V12_PIPE_PIPE {
		isAnd := p.cur().Type == V12_AMP_AMP
		p.advance()
		rhs, err := p.parseJPFilterUnary()
		if err != nil {
			return nil, err
		}
		node.Pairs = append(node.Pairs, V12JPFilterExprPair{IsAnd: isAnd, RHS: rhs})
	}
	return node, nil
}

// ---------- jp_filter ----------

// parseJPFilter parses:  jp_filter = "?" "(" jp_filter_expr ")"
func (p *V12Parser) parseJPFilter() (*V12JPFilterNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_QUESTION); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_LPAREN); err != nil {
		return nil, err
	}
	expr, err := p.ParseJPFilterExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_RPAREN); err != nil {
		return nil, err
	}
	return &V12JPFilterNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Expr: expr}, nil
}

// ---------- jp_selector ----------

// parseJPSelector parses one selector:
//
//	jp_selector = jp_filter | jp_slice | jp_index | jp_name | jp_wildcard
//
// Order matters — filter and slice are tried before index and name.
func (p *V12Parser) parseJPSelector() (*V12JPSelectorNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// jp_filter: "?" "(" ...
	if p.cur().Type == V12_QUESTION {
		f, err := p.parseJPFilter()
		if err != nil {
			return nil, err
		}
		return &V12JPSelectorNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Filter: f}, nil
	}

	// jp_wildcard: "*"
	if p.cur().Type == V12_STAR {
		p.advance()
		return &V12JPSelectorNode{
			V12BaseNode: V12BaseNode{Line: line, Col: col},
			Wildcard:    &V12JPWildcardNode{V12BaseNode: V12BaseNode{Line: line, Col: col}},
		}, nil
	}

	// jp_slice or jp_index: both begin with optional integer then ':'
	if p.cur().Type == V12_INTEGER || p.cur().Type == V12_MINUS || p.cur().Type == V12_COLON {
		saved := p.savePos()
		val, err := p.parseJPSliceOrIndex()
		if err != nil {
			p.restorePos(saved)
		} else {
			switch v := val.(type) {
			case *V12JPSliceNode:
				return &V12JPSelectorNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Slice: v}, nil
			case *V12JPIndexNode:
				return &V12JPSelectorNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Index: v}, nil
			}
		}
		p.restorePos(saved)
	}

	// jp_name: ident_name | string
	n, err := p.parseJPName()
	if err != nil {
		return nil, err
	}
	return &V12JPSelectorNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Name: n}, nil
}

// ---------- jp_selector_list / jp_bracket_seg ----------

// parseJPSelectorList parses:  jp_selector_list = jp_selector { "," jp_selector }
func (p *V12Parser) parseJPSelectorList() (*V12JPSelectorListNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.parseJPSelector()
	if err != nil {
		return nil, err
	}
	items := []*V12JPSelectorNode{first}
	for p.cur().Type == V12_COMMA {
		p.advance()
		sel, err := p.parseJPSelector()
		if err != nil {
			return nil, err
		}
		items = append(items, sel)
	}
	return &V12JPSelectorListNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Items: items}, nil
}

// parseJPBracketSeg parses:  jp_bracket_seg = "[" jp_selector_list "]"
func (p *V12Parser) parseJPBracketSeg() (*V12JPBracketSegNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_LBRACKET); err != nil {
		return nil, err
	}
	sels, err := p.parseJPSelectorList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_RBRACKET); err != nil {
		return nil, err
	}
	return &V12JPBracketSegNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Selectors: sels}, nil
}

// ---------- jp_dot_seg / jp_desc_seg / jp_segment ----------

// parseJPDotSeg parses:  jp_dot_seg = "." ( jp_name | jp_wildcard )
// The "." has already been consumed by the caller.
func (p *V12Parser) parseJPDotSegAfterDot(line, col int) (*V12JPDotSegNode, error) {
	if p.cur().Type == V12_STAR {
		p.advance()
		return &V12JPDotSegNode{
			V12BaseNode: V12BaseNode{Line: line, Col: col},
			Wildcard:    &V12JPWildcardNode{V12BaseNode: V12BaseNode{Line: line, Col: col}},
		}, nil
	}
	n, err := p.parseJPName()
	if err != nil {
		return nil, err
	}
	return &V12JPDotSegNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Name: n}, nil
}

// parseJPSegment parses:  jp_segment = jp_desc_seg | jp_child_seg
//
// jp_desc_seg  = ".." ( jp_name | jp_wildcard | jp_bracket_seg )
// jp_child_seg = jp_bracket_seg | jp_dot_seg
func (p *V12Parser) parseJPSegment() (*V12JPSegmentNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Bracket child segment: "[" ...
	if p.cur().Type == V12_LBRACKET {
		bs, err := p.parseJPBracketSeg()
		if err != nil {
			return nil, err
		}
		return &V12JPSegmentNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Bracket: bs}, nil
	}

	// ".." recursive-descent segment.
	if p.cur().Type == V12_DOTDOT {
		p.advance() // consume ".."
		var desc V12JPDescSegNode
		desc.V12BaseNode = V12BaseNode{Line: line, Col: col}
		switch p.cur().Type {
		case V12_LBRACKET:
			bs, err := p.parseJPBracketSeg()
			if err != nil {
				return nil, err
			}
			desc.Bracket = bs
		case V12_STAR:
			p.advance()
			desc.Wildcard = &V12JPWildcardNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}
		default:
			n, err := p.parseJPName()
			if err != nil {
				return nil, err
			}
			desc.Name = n
		}
		return &V12JPSegmentNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Desc: &desc}, nil
	}

	// "." child dot segment.
	if p.cur().Type == V12_DOT {
		p.advance() // consume "."
		ds, err := p.parseJPDotSegAfterDot(line, col)
		if err != nil {
			return nil, err
		}
		return &V12JPSegmentNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Dot: ds}, nil
	}

	return nil, p.errAt(fmt.Sprintf("expected JSONPath segment (., .., or [), got %s %q", p.cur().Type, p.cur().Value))
}

// ---------- jp_current_path ----------

// parseJPCurrentPath parses:  jp_current_path = "@" { jp_segment }
func (p *V12Parser) parseJPCurrentPath() (*V12JPCurrentPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_AT); err != nil {
		return nil, err
	}
	node := &V12JPCurrentPathNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}
	for p.cur().Type == V12_DOT || p.cur().Type == V12_DOTDOT || p.cur().Type == V12_LBRACKET {
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
// The ".$" compound token is V12_DOTDOLLAR.
func (p *V12Parser) ParseJSONPath() (*V12JSONPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_DOTDOLLAR); err != nil {
		return nil, err
	}
	node := &V12JSONPathNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}
	for p.cur().Type == V12_DOT || p.cur().Type == V12_DOTDOT || p.cur().Type == V12_LBRACKET {
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
func (p *V12Parser) ParseIdentRefWithPath() (*V12IdentRefWithPathNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	node := &V12IdentRefWithPathNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Base: ref}
	if p.cur().Type == V12_DOTDOLLAR {
		jp, err := p.ParseJSONPath()
		if err != nil {
			return nil, err
		}
		node.Path = jp
	}
	return node, nil
}
