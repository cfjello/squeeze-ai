// parser_v13_operators.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/02_operators.sqg.
//
// V13 changes vs V12:
//   - string_compare accepts regexp as the RHS when the operator is "=~"
//     (promoting string_regexp_comp as an alternative inside string_compare)
//
// Covered rules:
//
//	ident_name, ident_dotted, ident_prefix, ident_ref
//	numeric_oper, inline_incr, single_num_expr, num_expr_list, num_grouping, numeric_expr
//	string_oper, string_expr_list, string_grouping, string_expr
//	compare_oper, num_compare, string_compare, string_regexp_comp, compare_expr
//	not_oper, logic_oper, single_logic_expr, logic_expr_list, logic_grouping, logic_expr
//	calc_unit
package parser

import (
	"fmt"
	"strings"
)

// =============================================================================
// PHASE 2 — AST NODE TYPES  (02_operators.sqg)
// =============================================================================

// ---------- Identifiers ----------

// V13IdentDottedNode  ident_dotted = ident_name { "." ident_name }
type V13IdentDottedNode struct {
	V13BaseNode
	Parts []string
}

// V13IdentPrefixNode  ident_prefix = ( "../" { "../" } ) | "./"
type V13IdentPrefixNode struct {
	V13BaseNode
	Value string // "./" or "../" or "../../" …
}

// V13IdentRefNode  ident_ref = [ ident_prefix ] ident_dotted
type V13IdentRefNode struct {
	V13BaseNode
	Prefix *V13IdentPrefixNode
	Dotted *V13IdentDottedNode
}

// V13TypeRefNode  type_ref = ident_name "." "@type"  (V13 new)
// Allows referring to the type annotation on a scope binding.
type V13TypeRefNode struct {
	V13BaseNode
	Name string // the ident_name before the dot
}

// ---------- Numeric expressions ----------

// V13SingleNumExprNode  single_num_expr = numeric_const | [ inline_incr ] ident_ref
type V13SingleNumExprNode struct {
	V13BaseNode
	Literal    *V13NumericConstNode
	InlineIncr string // "", "++" or "--"
	IdentRef   *V13IdentRefNode
}

// V13NumExprTerm is one element of a num_expr_list chain.
type V13NumExprTerm struct {
	Oper string // "" for the first element
	Expr *V13SingleNumExprNode
}

// V13NumExprListNode  num_expr_list = single_num_expr { numeric_oper single_num_expr }
type V13NumExprListNode struct {
	V13BaseNode
	Terms []V13NumExprTerm
}

// V13NumGroupTerm is one element in a num_grouping.
type V13NumGroupTerm struct {
	Oper string  // "" for first, else numeric_oper
	Expr V13Node // *V13NumExprListNode | *V13NumGroupingNode
}

// V13NumGroupingNode  num_grouping = "(" ( num_expr_list | num_grouping ) { numeric_oper … } ")"
type V13NumGroupingNode struct {
	V13BaseNode
	Terms []V13NumGroupTerm
}

// V13NumericExprNode  numeric_expr = num_expr_list | num_grouping
type V13NumericExprNode struct {
	V13BaseNode
	Value V13Node // *V13NumExprListNode | *V13NumGroupingNode
}

// ---------- String expressions ----------

// V13StringExprTerm is one element of a string_expr_list chain.
type V13StringExprTerm struct {
	Oper string // "" for first; "+" for rest
	Str  *V13StringNode
}

// V13StringExprListNode  string_expr_list = string { "+" string }
type V13StringExprListNode struct {
	V13BaseNode
	Terms []V13StringExprTerm
}

// V13StringGroupTerm is one element in a string_grouping.
type V13StringGroupTerm struct {
	Oper string  // "" for first; "+"
	Expr V13Node // *V13StringExprListNode | *V13StringGroupingNode
}

// V13StringGroupingNode  string_grouping = "(" ( string_expr_list | string_grouping ) { "+" … } ")"
type V13StringGroupingNode struct {
	V13BaseNode
	Terms []V13StringGroupTerm
}

// V13StringExprNode  string_expr = string_expr_list | string_grouping
type V13StringExprNode struct {
	V13BaseNode
	Value V13Node // *V13StringExprListNode | *V13StringGroupingNode
}

// ---------- Compare expressions ----------

// V13NumCompareNode  TYPE_OF boolean<numeric_expr compare_oper numeric_expr>
type V13NumCompareNode struct {
	V13BaseNode
	Left  *V13NumericExprNode
	Oper  string
	Right *V13NumericExprNode
}

// V13StringCompareNode  TYPE_OF boolean<string_expr compare_oper string_expr>
type V13StringCompareNode struct {
	V13BaseNode
	Left  *V13StringExprNode
	Oper  string
	Right *V13StringExprNode
}

// V13StringRegexpCompNode  TYPE_OF boolean<string_expr "=~" regexp>
// In V13 this is also the RHS of string_compare when oper is "=~".
type V13StringRegexpCompNode struct {
	V13BaseNode
	Left   *V13StringExprNode
	Regexp *V13RegexpNode
}

// V13CompareExprNode  compare_expr = num_compare | string_compare | string_regexp_comp
type V13CompareExprNode struct {
	V13BaseNode
	Value V13Node // *V13NumCompareNode | *V13StringCompareNode | *V13StringRegexpCompNode
}

// ---------- Logic expressions ----------

// V13SingleLogicExprNode  single_logic_expr
type V13SingleLogicExprNode struct {
	V13BaseNode
	Negated   bool
	IdentRef  *V13IdentRefNode
	Compare   *V13CompareExprNode
	Numeric   *V13NumericExprNode
	StringVal *V13StringExprNode
}

// V13LogicExprTerm is one element of a logic_expr_list.
type V13LogicExprTerm struct {
	Oper string // "" for first; "&", "|", "^"
	Expr *V13SingleLogicExprNode
}

// V13LogicExprListNode  logic_expr_list = single_logic_expr { logic_oper single_logic_expr }
type V13LogicExprListNode struct {
	V13BaseNode
	Terms []V13LogicExprTerm
}

// V13LogicGroupTerm is one element of a logic_grouping.
type V13LogicGroupTerm struct {
	Oper string  // "" for first; "&", "|", "^"
	Expr V13Node // *V13LogicExprListNode | *V13LogicGroupingNode
}

// V13LogicGroupingNode  logic_grouping = "(" ( logic_expr_list | logic_grouping ) { logic_oper … } ")"
type V13LogicGroupingNode struct {
	V13BaseNode
	Terms []V13LogicGroupTerm
}

// V13LogicExprNode  logic_expr = logic_expr_list | logic_grouping
type V13LogicExprNode struct {
	V13BaseNode
	Value V13Node // *V13LogicExprListNode | *V13LogicGroupingNode
}

// ---------- Top-level expression ----------

// V13SelfRefNode  "$" — self-reference inside object/scope body
type V13SelfRefNode struct{ V13BaseNode }

// V13CalcUnitNode  calc_unit = numeric_expr | string_expr | logic_expr
type V13CalcUnitNode struct {
	V13BaseNode
	Value V13Node // *V13NumericExprNode | *V13StringExprNode | *V13LogicExprNode | *V13SelfRefNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (02_operators.sqg)
// =============================================================================

// ---------- Operator classification helpers ----------

func V13numericOper(t V13TokenType) string {
	switch t {
	case V13_PLUS:
		return "+"
	case V13_MINUS:
		return "-"
	case V13_STAR:
		return "*"
	case V13_POW:
		return "**"
	case V13_SLASH:
		return "/"
	case V13_PERCENT:
		return "%"
	}
	return ""
}

func V13inlineIncr(t V13TokenType) string {
	switch t {
	case V13_INC:
		return "++"
	case V13_DEC:
		return "--"
	}
	return ""
}

func V13compareOper(t V13TokenType) string {
	switch t {
	case V13_NEQ:
		return "!="
	case V13_EQEQ:
		return "=="
	case V13_GT:
		return ">"
	case V13_GEQ:
		return ">="
	case V13_LT:
		return "<"
	case V13_LEQ:
		return "<="
	}
	return ""
}

func V13logicOper(t V13TokenType) string {
	switch t {
	case V13_AMP:
		return "&"
	case V13_PIPE:
		return "|"
	case V13_CARET:
		return "^"
	}
	return ""
}

func V13isStringTok(t V13TokenType) bool {
	switch t {
	case V13_STRING, V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T:
		return true
	}
	return false
}

// ---------- Identifier parsers ----------

// ParseIdentDotted parses:  ident_dotted = ident_name { "." ident_name }
func (p *V13Parser) ParseIdentDotted() (*V13IdentDottedNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected identifier, got %s %q", tok.Type, tok.Value))
	}
	parts := []string{tok.Value}
	p.advance()

	for p.cur().Type == V13_DOT {
		saved := p.savePos()
		p.advance()
		if p.cur().Type != V13_IDENT {
			p.restorePos(saved)
			break
		}
		parts = append(parts, p.cur().Value)
		p.advance()
	}
	return &V13IdentDottedNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Parts:       parts,
	}, nil
}

// ParseIdentPrefix parses:  ident_prefix = ( "../" { "../" } ) | "./"
func (p *V13Parser) ParseIdentPrefix() (*V13IdentPrefixNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var sb strings.Builder

	switch tok.Type {
	case V13_DOT:
		p.advance()
		if _, err := p.expect(V13_SLASH); err != nil {
			return nil, err
		}
		sb.WriteString("./")
	case V13_DOTDOT:
		for p.cur().Type == V13_DOTDOT {
			p.advance()
			if _, err := p.expect(V13_SLASH); err != nil {
				return nil, err
			}
			sb.WriteString("../")
		}
	default:
		return nil, p.errAt(fmt.Sprintf("expected './' or '../', got %s %q", tok.Type, tok.Value))
	}
	return &V13IdentPrefixNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Value:       sb.String(),
	}, nil
}

// ParseIdentRef parses:  ident_ref = [ ident_prefix ] ident_dotted
func (p *V13Parser) ParseIdentRef() (*V13IdentRefNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V13IdentRefNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}

	tok := p.cur()
	if tok.Type == V13_DOT || tok.Type == V13_DOTDOT {
		saved := p.savePos()
		prefix, err := p.ParseIdentPrefix()
		if err != nil || p.cur().Type != V13_IDENT {
			p.restorePos(saved)
		} else {
			node.Prefix = prefix
		}
	}

	dotted, err := p.ParseIdentDotted()
	if err != nil {
		return nil, err
	}
	node.Dotted = dotted
	return node, nil
}

// ParseTypeRef parses:  type_ref = ident_name "." "@type"  (V13 new)
func (p *V13Parser) ParseTypeRef() (*V13TypeRefNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected ident_name for type_ref, got %s %q", tok.Type, tok.Value))
	}
	name := tok.Value
	p.advance()
	if _, err := p.expect(V13_DOT); err != nil {
		return nil, err
	}
	// Expect the literal "@type" (an AT_IDENT with value "@type")
	next := p.cur()
	if next.Type != V13_AT_IDENT || next.Value != "@type" {
		return nil, p.errAt(fmt.Sprintf("expected @type, got %s %q", next.Type, next.Value))
	}
	p.advance()
	return &V13TypeRefNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: name}, nil
}

// ---------- Numeric expression parsers ----------

// ParseSingleNumExpr parses:  single_num_expr = numeric_const | [ inline_incr ] ident_ref
func (p *V13Parser) ParseSingleNumExpr() (*V13SingleNumExprNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	switch tok.Type {
	case V13_INTEGER, V13_DECIMAL:
		lit, err := p.ParseNumericConst()
		if err != nil {
			return nil, err
		}
		return &V13SingleNumExprNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			Literal:     lit,
		}, nil
	case V13_PLUS, V13_MINUS:
		next := p.peek(1)
		if next.Type == V13_INTEGER || next.Type == V13_DECIMAL {
			lit, err := p.ParseNumericConst()
			if err != nil {
				return nil, err
			}
			return &V13SingleNumExprNode{
				V13BaseNode: V13BaseNode{Line: line, Col: col},
				Literal:     lit,
			}, nil
		}
	}

	incr := V13inlineIncr(tok.Type)
	if incr != "" {
		p.advance()
	}
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	return &V13SingleNumExprNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		InlineIncr:  incr,
		IdentRef:    ref,
	}, nil
}

// ParseNumExprList parses:  num_expr_list = single_num_expr { numeric_oper single_num_expr }
func (p *V13Parser) ParseNumExprList() (*V13NumExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.ParseSingleNumExpr()
	if err != nil {
		return nil, err
	}
	terms := []V13NumExprTerm{{Oper: "", Expr: first}}
	for {
		oper := V13numericOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		next, err := p.ParseSingleNumExpr()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V13NumExprTerm{Oper: oper, Expr: next})
	}
	return &V13NumExprListNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseNumGrouping parses:
//
//	num_grouping = "(" ( num_expr_list | num_grouping ) { numeric_oper … } ")"
func (p *V13Parser) ParseNumGrouping() (*V13NumGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseNumGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V13NumGroupTerm{{Oper: "", Expr: first}}
	for {
		oper := V13numericOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		item, err := p.parseNumGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V13NumGroupTerm{Oper: oper, Expr: item})
	}
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return &V13NumGroupingNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V13Parser) parseNumGroupItem() (V13Node, error) {
	if p.cur().Type == V13_LPAREN {
		return p.ParseNumGrouping()
	}
	return p.ParseNumExprList()
}

// ParseNumericExpr parses:  numeric_expr = num_expr_list | num_grouping
func (p *V13Parser) ParseNumericExpr() (*V13NumericExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V13Node
	var err error
	if p.cur().Type == V13_LPAREN {
		val, err = p.ParseNumGrouping()
	} else {
		val, err = p.ParseNumExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V13NumericExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- String expression parsers ----------

// ParseStringExprList parses:  string_expr_list = string { "+" string }
func (p *V13Parser) ParseStringExprList() (*V13StringExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.ParseString()
	if err != nil {
		return nil, err
	}
	terms := []V13StringExprTerm{{Oper: "", Str: first}}
	for p.cur().Type == V13_PLUS {
		p.advance()
		str, err := p.ParseString()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V13StringExprTerm{Oper: "+", Str: str})
	}
	return &V13StringExprListNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseStringGrouping parses:
//
//	string_grouping = "(" ( string_expr_list | string_grouping ) { "+" … } ")"
func (p *V13Parser) ParseStringGrouping() (*V13StringGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseStringGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V13StringGroupTerm{{Oper: "", Expr: first}}
	for p.cur().Type == V13_PLUS {
		p.advance()
		item, err := p.parseStringGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V13StringGroupTerm{Oper: "+", Expr: item})
	}
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return &V13StringGroupingNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V13Parser) parseStringGroupItem() (V13Node, error) {
	if p.cur().Type == V13_LPAREN {
		return p.ParseStringGrouping()
	}
	return p.ParseStringExprList()
}

// ParseStringExpr parses:  string_expr = string_expr_list | string_grouping
func (p *V13Parser) ParseStringExpr() (*V13StringExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V13Node
	var err error
	if p.cur().Type == V13_LPAREN {
		val, err = p.ParseStringGrouping()
	} else {
		val, err = p.ParseStringExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V13StringExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- Compare expression parsers ----------

// ParseCompareExpr parses:
//
//	compare_expr = num_compare | string_compare | string_regexp_comp
//
// V13 change: when the LHS is a string and the operator is "=~", the RHS can
// be a regexp literal (string_regexp_comp) rather than another string_expr.
func (p *V13Parser) ParseCompareExpr() (*V13CompareExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V13Node
	var err error

	if V13isStringTok(p.cur().Type) {
		saved := p.savePos()
		val, err = p.parseStringRegexpComp()
		if err != nil {
			p.restorePos(saved)
			val, err = p.parseStringCompare()
			if err != nil {
				return nil, err
			}
		}
	} else {
		val, err = p.parseNumCompare()
		if err != nil {
			return nil, err
		}
	}
	return &V13CompareExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

func (p *V13Parser) parseNumCompare() (*V13NumCompareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseNumericExpr()
	if err != nil {
		return nil, err
	}
	oper := V13compareOper(p.cur().Type)
	if oper == "" {
		return nil, p.errAt(fmt.Sprintf("expected compare operator, got %s %q", p.cur().Type, p.cur().Value))
	}
	p.advance()
	right, err := p.ParseNumericExpr()
	if err != nil {
		return nil, err
	}
	return &V13NumCompareNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        oper,
		Right:       right,
	}, nil
}

func (p *V13Parser) parseStringCompare() (*V13StringCompareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	oper := V13compareOper(p.cur().Type)
	if oper == "" {
		return nil, p.errAt(fmt.Sprintf("expected compare operator, got %s %q", p.cur().Type, p.cur().Value))
	}
	p.advance()
	right, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	return &V13StringCompareNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        oper,
		Right:       right,
	}, nil
}

func (p *V13Parser) parseStringRegexpComp() (*V13StringRegexpCompNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_MATCH_OP); err != nil {
		return nil, err
	}
	re, err := p.ParseRegexp()
	if err != nil {
		return nil, err
	}
	return &V13StringRegexpCompNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Left:        left,
		Regexp:      re,
	}, nil
}

// ---------- Logic expression parsers ----------

// ParseSingleLogicExpr parses:
//
//	single_logic_expr = [ not_oper ] ( ident_dotted | TYPE_OF boolean<ident_ref> | compare_expr )
//	                  | numeric_expr | string_expr
func (p *V13Parser) ParseSingleLogicExpr() (*V13SingleLogicExprNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	node := &V13SingleLogicExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}

	if tok.Type == V13_BANG {
		node.Negated = true
		p.advance()
		tok = p.cur()
	}

	if !node.Negated && V13isStringTok(tok.Type) {
		str, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		node.StringVal = str
		return node, nil
	}

	if !node.Negated {
		switch tok.Type {
		case V13_INTEGER, V13_DECIMAL:
			num, err := p.ParseNumericExpr()
			if err != nil {
				return nil, err
			}
			node.Numeric = num
			return node, nil
		}
	}

	if tok.Type == V13_LPAREN {
		saved := p.savePos()
		if num, err := p.ParseNumericExpr(); err == nil {
			node.Numeric = num
			return node, nil
		}
		p.restorePos(saved)
		if str, err := p.ParseStringExpr(); err == nil {
			node.StringVal = str
			return node, nil
		}
		return nil, p.errAt("expected numeric or string grouping in single_logic_expr")
	}

	saved := p.savePos()
	if cmp, err := p.ParseCompareExpr(); err == nil {
		node.Compare = cmp
		return node, nil
	}
	p.restorePos(saved)

	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, p.errAt(fmt.Sprintf("unexpected token %s %q in single_logic_expr", tok.Type, tok.Value))
	}
	node.IdentRef = ref
	return node, nil
}

// ParseLogicExprList parses:  logic_expr_list = single_logic_expr { logic_oper single_logic_expr }
func (p *V13Parser) ParseLogicExprList() (*V13LogicExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.ParseSingleLogicExpr()
	if err != nil {
		return nil, err
	}
	terms := []V13LogicExprTerm{{Oper: "", Expr: first}}
	for {
		oper := V13logicOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		next, err := p.ParseSingleLogicExpr()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V13LogicExprTerm{Oper: oper, Expr: next})
	}
	return &V13LogicExprListNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseLogicGrouping parses:
//
//	logic_grouping = "(" ( logic_expr_list | logic_grouping ) { logic_oper … } ")"
func (p *V13Parser) ParseLogicGrouping() (*V13LogicGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseLogicGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V13LogicGroupTerm{{Oper: "", Expr: first}}
	for {
		oper := V13logicOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		item, err := p.parseLogicGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V13LogicGroupTerm{Oper: oper, Expr: item})
	}
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return &V13LogicGroupingNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V13Parser) parseLogicGroupItem() (V13Node, error) {
	if p.cur().Type == V13_LPAREN {
		return p.ParseLogicGrouping()
	}
	return p.ParseLogicExprList()
}

// ParseLogicExpr parses:  logic_expr = logic_expr_list | logic_grouping
func (p *V13Parser) ParseLogicExpr() (*V13LogicExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V13Node
	var err error
	if p.cur().Type == V13_LPAREN {
		val, err = p.ParseLogicGrouping()
	} else {
		val, err = p.ParseLogicExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V13LogicExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- Calc unit ----------

// ParseCalcUnit parses:  calc_unit = numeric_expr | string_expr | logic_expr
func (p *V13Parser) ParseCalcUnit() (*V13CalcUnitNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type == V13_DOLLAR {
		p.advance()
		return &V13CalcUnitNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			Value:       &V13SelfRefNode{V13BaseNode: V13BaseNode{Line: line, Col: col}},
		}, nil
	}

	if V13isStringTok(tok.Type) {
		str, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		return &V13CalcUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: str}, nil
	}

	if tok.Type == V13_INTEGER || tok.Type == V13_DECIMAL {
		saved := p.savePos()
		if num, err := p.ParseNumericExpr(); err == nil {
			if V13compareOper(p.cur().Type) == "" && V13logicOper(p.cur().Type) == "" {
				return &V13CalcUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: num}, nil
			}
		}
		p.restorePos(saved)
	}

	saved := p.savePos()
	if logic, err := p.ParseLogicExpr(); err == nil {
		return &V13CalcUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: logic}, nil
	}
	p.restorePos(saved)

	num, err := p.ParseNumericExpr()
	if err != nil {
		return nil, p.errAt(fmt.Sprintf("could not parse calc_unit: unexpected token %s %q", tok.Type, tok.Value))
	}
	return &V13CalcUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: num}, nil
}
