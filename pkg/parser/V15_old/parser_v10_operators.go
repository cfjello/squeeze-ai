// parser_v10_operators.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V10 grammar rule set defined in spec/02_operators.sqg.
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

// V10IdentDottedNode  ident_dotted = ident_name { "." ident_name }
type V10IdentDottedNode struct {
	v10BaseNode
	Parts []string // each trimmed ident_name segment
}

// V10IdentPrefixNode  ident_prefix = ( "../" { "../" } ) | "./"
type V10IdentPrefixNode struct {
	v10BaseNode
	Value string // "./" or "../" or "../../" etc.
}

// V10IdentRefNode  ident_ref = [ ident_prefix ] ident_dotted
type V10IdentRefNode struct {
	v10BaseNode
	Prefix *V10IdentPrefixNode // nil if no prefix
	Dotted *V10IdentDottedNode
}

// ---------- Numeric expressions ----------

// V10SingleNumExprNode  single_num_expr = numeric_const | [ inline_incr ] ident_ref
// (TYPE_OF numeric_const<ident_ref> is a grammar-level annotation; at parse
// time the ident_ref branch carries no extra token.)
type V10SingleNumExprNode struct {
	v10BaseNode
	Literal    *V10NumericConstNode // set when the value is a literal numeric_const
	InlineIncr string               // "", "++" or "--" — only when IdentRef != nil
	IdentRef   *V10IdentRefNode     // set when the value is an identifier reference
}

// V10NumExprTerm is one element of a num_expr_list chain.
type V10NumExprTerm struct {
	Oper string // "" for the first element; "+", "-", "*", "**", "/", "%" for rest
	Expr *V10SingleNumExprNode
}

// V10NumExprListNode  num_expr_list = single_num_expr { numeric_oper single_num_expr }
type V10NumExprListNode struct {
	v10BaseNode
	Terms []V10NumExprTerm
}

// V10NumGroupTerm is one element in a num_grouping.
type V10NumGroupTerm struct {
	Oper string  // "" for first element, else numeric_oper
	Expr V10Node // *V10NumExprListNode | *V10NumGroupingNode
}

// V10NumGroupingNode  num_grouping = "(" ( num_expr_list | num_grouping ) { numeric_oper … } ")"
type V10NumGroupingNode struct {
	v10BaseNode
	Terms []V10NumGroupTerm
}

// V10NumericExprNode  numeric_expr = num_expr_list | num_grouping
type V10NumericExprNode struct {
	v10BaseNode
	Value V10Node // *V10NumExprListNode | *V10NumGroupingNode
}

// ---------- String expressions ----------

// V10StringExprTerm is one element of a string_expr_list chain.
type V10StringExprTerm struct {
	Oper string // "" for first; "+" for subsequent elements
	Str  *V10StringNode
}

// V10StringExprListNode  string_expr_list = string { "+" string }
type V10StringExprListNode struct {
	v10BaseNode
	Terms []V10StringExprTerm
}

// V10StringGroupTerm is one element in a string_grouping.
type V10StringGroupTerm struct {
	Oper string  // "" for first; "+"
	Expr V10Node // *V10StringExprListNode | *V10StringGroupingNode
}

// V10StringGroupingNode  string_grouping = "(" ( string_expr_list | string_grouping ) { "+" … } ")"
type V10StringGroupingNode struct {
	v10BaseNode
	Terms []V10StringGroupTerm
}

// V10StringExprNode  string_expr = string_expr_list | string_grouping
type V10StringExprNode struct {
	v10BaseNode
	Value V10Node // *V10StringExprListNode | *V10StringGroupingNode
}

// ---------- Compare expressions ----------

// V10NumCompareNode  TYPE_OF boolean<numeric_expr compare_oper numeric_expr>
// (TYPE_OF boolean<…> is the grammar annotation; at parse time this is simply
// left compare_oper right.)
type V10NumCompareNode struct {
	v10BaseNode
	Left  *V10NumericExprNode
	Oper  string // "!=", "==", ">", ">=", "<", "<="
	Right *V10NumericExprNode
}

// V10StringCompareNode  TYPE_OF boolean<string_expr compare_oper string_expr>
type V10StringCompareNode struct {
	v10BaseNode
	Left  *V10StringExprNode
	Oper  string
	Right *V10StringExprNode
}

// V10StringRegexpCompNode  TYPE_OF boolean<string_expr "=~" regexp>
type V10StringRegexpCompNode struct {
	v10BaseNode
	Left   *V10StringExprNode
	Regexp *V10RegexpNode
}

// V10CompareExprNode  compare_expr = num_compare | string_compare | string_regexp_comp
type V10CompareExprNode struct {
	v10BaseNode
	Value V10Node // *V10NumCompareNode | *V10StringCompareNode | *V10StringRegexpCompNode
}

// ---------- Logic expressions ----------

// V10SingleLogicExprNode  single_logic_expr
// grammar:
//
//	[ not_oper ] ( ident_dotted | TYPE_OF boolean<ident_ref> | compare_expr )
//	| numeric_expr | string_expr
//
// Exactly one of the Value fields is non-nil.
type V10SingleLogicExprNode struct {
	v10BaseNode
	Negated   bool
	IdentRef  *V10IdentRefNode // bare ident used as boolean expression
	Compare   *V10CompareExprNode
	Numeric   *V10NumericExprNode
	StringVal *V10StringExprNode
}

// V10LogicExprTerm is one element of a logic_expr_list.
type V10LogicExprTerm struct {
	Oper string // "" for first; "&", "|", "^" for rest
	Expr *V10SingleLogicExprNode
}

// V10LogicExprListNode  logic_expr_list = single_logic_expr { logic_oper single_logic_expr }
type V10LogicExprListNode struct {
	v10BaseNode
	Terms []V10LogicExprTerm
}

// V10LogicGroupTerm is one element of a logic_grouping.
type V10LogicGroupTerm struct {
	Oper string  // "" for first; "&", "|", "^"
	Expr V10Node // *V10LogicExprListNode | *V10LogicGroupingNode
}

// V10LogicGroupingNode  logic_grouping = "(" ( logic_expr_list | logic_grouping ) { logic_oper … } ")"
type V10LogicGroupingNode struct {
	v10BaseNode
	Terms []V10LogicGroupTerm
}

// V10LogicExprNode  logic_expr = logic_expr_list | logic_grouping
type V10LogicExprNode struct {
	v10BaseNode
	Value V10Node // *V10LogicExprListNode | *V10LogicGroupingNode
}

// ---------- Top-level expression ----------

// V10CalcUnitNode  calc_unit = numeric_expr | string_expr | logic_expr
type V10CalcUnitNode struct {
	v10BaseNode
	Value V10Node // *V10NumericExprNode | *V10StringExprNode | *V10LogicExprNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (02_operators.sqg)
// =============================================================================

// ---------- Position save/restore for local backtracking ----------

func (p *V10Parser) savePos() int       { return p.pos }
func (p *V10Parser) restorePos(pos int) { p.pos = pos }

// ---------- Operator classification helpers ----------

// v10numericOper returns the string form of tok if it is a numeric_oper, else "".
func v10numericOper(t V10TokenType) string {
	switch t {
	case V10_PLUS:
		return "+"
	case V10_MINUS:
		return "-"
	case V10_STAR:
		return "*"
	case V10_POW:
		return "**"
	case V10_SLASH:
		return "/"
	case V10_PERCENT:
		return "%"
	}
	return ""
}

// v10inlineIncr returns "++" or "--" if tok is an inline_incr, else "".
func v10inlineIncr(t V10TokenType) string {
	switch t {
	case V10_INC:
		return "++"
	case V10_DEC:
		return "--"
	}
	return ""
}

// v10compareOper returns the string form of tok if it is a compare_oper, else "".
func v10compareOper(t V10TokenType) string {
	switch t {
	case V10_NEQ:
		return "!="
	case V10_EQEQ:
		return "=="
	case V10_GT:
		return ">"
	case V10_GEQ:
		return ">="
	case V10_LT:
		return "<"
	case V10_LEQ:
		return "<="
	}
	return ""
}

// v10logicOper returns the string form of tok if it is a logic_oper, else "".
func v10logicOper(t V10TokenType) string {
	switch t {
	case V10_AMP:
		return "&"
	case V10_PIPE:
		return "|"
	case V10_CARET:
		return "^"
	}
	return ""
}

// v10isStringTok returns true if tok can begin a string expression.
func v10isStringTok(t V10TokenType) bool {
	switch t {
	case V10_STRING, V10_EMPTY_STR_D, V10_EMPTY_STR_S, V10_EMPTY_STR_T:
		return true
	}
	return false
}

// ---------- Identifier parsers ----------

// ParseIdentDotted parses:  ident_dotted = ident_name { "." ident_name }
func (p *V10Parser) ParseIdentDotted() (*V10IdentDottedNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V10_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected identifier, got %s %q", tok.Type, tok.Value))
	}
	parts := []string{tok.Value}
	p.advance()

	for p.cur().Type == V10_DOT {
		saved := p.savePos()
		p.advance() // consume "."
		if p.cur().Type != V10_IDENT {
			p.restorePos(saved)
			break
		}
		parts = append(parts, p.cur().Value)
		p.advance()
	}

	return &V10IdentDottedNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Parts:       parts,
	}, nil
}

// ParseIdentPrefix parses:  ident_prefix = ( "../" { "../" } ) | "./"
// Token mapping:
//
//	"./"  → V10_DOT followed by V10_SLASH
//	"../" → V10_DOTDOT followed by V10_SLASH
func (p *V10Parser) ParseIdentPrefix() (*V10IdentPrefixNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var sb strings.Builder

	switch tok.Type {
	case V10_DOT: // "./"
		p.advance()
		if _, err := p.expect(V10_SLASH); err != nil {
			return nil, err
		}
		sb.WriteString("./")

	case V10_DOTDOT: // "../" { "../" }
		for p.cur().Type == V10_DOTDOT {
			p.advance()
			if _, err := p.expect(V10_SLASH); err != nil {
				return nil, err
			}
			sb.WriteString("../")
		}

	default:
		return nil, p.errAt(fmt.Sprintf("expected './' or '../', got %s %q", tok.Type, tok.Value))
	}

	return &V10IdentPrefixNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Value:       sb.String(),
	}, nil
}

// ParseIdentRef parses:  ident_ref = [ ident_prefix ] ident_dotted
// The prefix is attempted speculatively: if a "." or ".." is followed by "/"
// it is a prefix; otherwise we skip the prefix and parse ident_dotted directly.
func (p *V10Parser) ParseIdentRef() (*V10IdentRefNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V10IdentRefNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	tok := p.cur()
	// A prefix starts with "." or ".." — but only when the very next
	// significant token is "/" (slash), distinguishing "./" and "../" from
	// the "." member-access in ident_dotted.
	if tok.Type == V10_DOT || tok.Type == V10_DOTDOT {
		saved := p.savePos()
		prefix, err := p.ParseIdentPrefix()
		if err != nil || p.cur().Type != V10_IDENT {
			// Not a valid prefix context — roll back.
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

// ---------- Numeric expression parsers ----------

// ParseSingleNumExpr parses:
//
//	single_num_expr = numeric_const | [ inline_incr ] ident_ref
func (p *V10Parser) ParseSingleNumExpr() (*V10SingleNumExprNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Literal numeric branch
	switch tok.Type {
	case V10_INTEGER, V10_DECIMAL:
		lit, err := p.ParseNumericConst()
		if err != nil {
			return nil, err
		}
		return &V10SingleNumExprNode{
			v10BaseNode: v10BaseNode{Line: line, Col: col},
			Literal:     lit,
		}, nil

	case V10_PLUS, V10_MINUS:
		next := p.peek(1)
		if next.Type == V10_INTEGER || next.Type == V10_DECIMAL {
			lit, err := p.ParseNumericConst()
			if err != nil {
				return nil, err
			}
			return &V10SingleNumExprNode{
				v10BaseNode: v10BaseNode{Line: line, Col: col},
				Literal:     lit,
			}, nil
		}
	}

	// Optional inline_incr + ident_ref branch
	incr := v10inlineIncr(tok.Type)
	if incr != "" {
		p.advance()
	}
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	return &V10SingleNumExprNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		InlineIncr:  incr,
		IdentRef:    ref,
	}, nil
}

// ParseNumExprList parses:
//
//	num_expr_list = single_num_expr { numeric_oper single_num_expr }
func (p *V10Parser) ParseNumExprList() (*V10NumExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col

	first, err := p.ParseSingleNumExpr()
	if err != nil {
		return nil, err
	}
	terms := []V10NumExprTerm{{Oper: "", Expr: first}}

	for {
		oper := v10numericOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		next, err := p.ParseSingleNumExpr()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V10NumExprTerm{Oper: oper, Expr: next})
	}

	return &V10NumExprListNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseNumGrouping parses:
//
//	num_grouping = "(" ( num_expr_list | num_grouping ) { numeric_oper ( num_expr_list | num_grouping ) } ")"
func (p *V10Parser) ParseNumGrouping() (*V10NumGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if _, err := p.expect(V10_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseNumGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V10NumGroupTerm{{Oper: "", Expr: first}}

	for {
		oper := v10numericOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		item, err := p.parseNumGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V10NumGroupTerm{Oper: oper, Expr: item})
	}

	if _, err := p.expect(V10_RPAREN); err != nil {
		return nil, err
	}
	return &V10NumGroupingNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V10Parser) parseNumGroupItem() (V10Node, error) {
	if p.cur().Type == V10_LPAREN {
		return p.ParseNumGrouping()
	}
	return p.ParseNumExprList()
}

// ParseNumericExpr parses:  numeric_expr = num_expr_list | num_grouping
func (p *V10Parser) ParseNumericExpr() (*V10NumericExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V10Node
	var err error
	if p.cur().Type == V10_LPAREN {
		val, err = p.ParseNumGrouping()
	} else {
		val, err = p.ParseNumExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V10NumericExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- String expression parsers ----------

// ParseStringExprList parses:  string_expr_list = string { "+" string }
func (p *V10Parser) ParseStringExprList() (*V10StringExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col

	first, err := p.ParseString()
	if err != nil {
		return nil, err
	}
	terms := []V10StringExprTerm{{Oper: "", Str: first}}

	for p.cur().Type == V10_PLUS {
		p.advance()
		str, err := p.ParseString()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V10StringExprTerm{Oper: "+", Str: str})
	}

	return &V10StringExprListNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseStringGrouping parses:
//
//	string_grouping = "(" ( string_expr_list | string_grouping ) { "+" ( … ) } ")"
func (p *V10Parser) ParseStringGrouping() (*V10StringGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if _, err := p.expect(V10_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseStringGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V10StringGroupTerm{{Oper: "", Expr: first}}

	for p.cur().Type == V10_PLUS {
		p.advance()
		item, err := p.parseStringGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V10StringGroupTerm{Oper: "+", Expr: item})
	}

	if _, err := p.expect(V10_RPAREN); err != nil {
		return nil, err
	}
	return &V10StringGroupingNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V10Parser) parseStringGroupItem() (V10Node, error) {
	if p.cur().Type == V10_LPAREN {
		return p.ParseStringGrouping()
	}
	return p.ParseStringExprList()
}

// ParseStringExpr parses:  string_expr = string_expr_list | string_grouping
func (p *V10Parser) ParseStringExpr() (*V10StringExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V10Node
	var err error
	if p.cur().Type == V10_LPAREN {
		val, err = p.ParseStringGrouping()
	} else {
		val, err = p.ParseStringExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V10StringExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- Compare expression parsers ----------

// ParseCompareExpr parses:
//
//	compare_expr = num_compare | string_compare | string_regexp_comp
//
// Disambiguation: if the left-hand token is a string literal, try
// string_regexp_comp (=~) then string_compare; otherwise try num_compare.
func (p *V10Parser) ParseCompareExpr() (*V10CompareExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V10Node
	var err error

	if v10isStringTok(p.cur().Type) {
		// Try string_regexp_comp (left "=~" regexp), fall back to string_compare.
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
	return &V10CompareExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: val}, nil
}

func (p *V10Parser) parseNumCompare() (*V10NumCompareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseNumericExpr()
	if err != nil {
		return nil, err
	}
	oper := v10compareOper(p.cur().Type)
	if oper == "" {
		return nil, p.errAt(fmt.Sprintf("expected compare operator, got %s %q", p.cur().Type, p.cur().Value))
	}
	p.advance()
	right, err := p.ParseNumericExpr()
	if err != nil {
		return nil, err
	}
	return &V10NumCompareNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        oper,
		Right:       right,
	}, nil
}

func (p *V10Parser) parseStringCompare() (*V10StringCompareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	oper := v10compareOper(p.cur().Type)
	if oper == "" {
		return nil, p.errAt(fmt.Sprintf("expected compare operator, got %s %q", p.cur().Type, p.cur().Value))
	}
	p.advance()
	right, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	return &V10StringCompareNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        oper,
		Right:       right,
	}, nil
}

func (p *V10Parser) parseStringRegexpComp() (*V10StringRegexpCompNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_MATCH_OP); err != nil {
		return nil, err
	}
	re, err := p.ParseRegexp()
	if err != nil {
		return nil, err
	}
	return &V10StringRegexpCompNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Left:        left,
		Regexp:      re,
	}, nil
}

// ---------- Logic expression parsers ----------

// ParseSingleLogicExpr parses:
//
//	single_logic_expr = [ not_oper ] ( ident_dotted | TYPE_OF boolean<ident_ref> | compare_expr )
//	                  | numeric_expr | string_expr
//
// Note: the negation (not_oper "!") only applies to the first alternative.
//
// Strategy:
//  1. If negation is present, expect ident_ref or compare_expr.
//  2. If string token is at head, parse string_expr (no negation allowed).
//  3. If pure numeric head (INTEGER/DECIMAL) with no following compare/logic
//     operator, parse numeric_expr.
//  4. Otherwise try compare_expr (with backtracking), then fall back to
//     ident_ref used as a bare boolean reference.
func (p *V10Parser) ParseSingleLogicExpr() (*V10SingleLogicExprNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	node := &V10SingleLogicExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	// Optional negation — only valid before the first three alternatives.
	if tok.Type == V10_BANG {
		node.Negated = true
		p.advance()
		tok = p.cur()
	}

	// string_expr branch (no negation in grammar for this path)
	if !node.Negated && v10isStringTok(tok.Type) {
		str, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		node.StringVal = str
		return node, nil
	}

	// Pure numeric literal → numeric_expr (covers strings like "42 + x")
	if !node.Negated {
		switch tok.Type {
		case V10_INTEGER, V10_DECIMAL:
			num, err := p.ParseNumericExpr()
			if err != nil {
				return nil, err
			}
			node.Numeric = num
			return node, nil
		}
	}

	// Parenthesised grouping — try numeric, then string as fallback.
	if tok.Type == V10_LPAREN {
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

	// ident_ref / compare_expr path —
	// Try compare_expr first (it will consume an ident_ref internally); if no
	// compare operator follows the expression, backtrack and return just ident_ref.
	saved := p.savePos()
	if cmp, err := p.ParseCompareExpr(); err == nil {
		node.Compare = cmp
		return node, nil
	}
	p.restorePos(saved)

	// Bare ident_ref used as a boolean expression.
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, p.errAt(fmt.Sprintf("unexpected token %s %q in single_logic_expr", tok.Type, tok.Value))
	}
	node.IdentRef = ref
	return node, nil
}

// ParseLogicExprList parses:
//
//	logic_expr_list = single_logic_expr { logic_oper single_logic_expr }
func (p *V10Parser) ParseLogicExprList() (*V10LogicExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col

	first, err := p.ParseSingleLogicExpr()
	if err != nil {
		return nil, err
	}
	terms := []V10LogicExprTerm{{Oper: "", Expr: first}}

	for {
		oper := v10logicOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		next, err := p.ParseSingleLogicExpr()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V10LogicExprTerm{Oper: oper, Expr: next})
	}

	return &V10LogicExprListNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseLogicGrouping parses:
//
//	logic_grouping = "(" ( logic_expr_list | logic_grouping ) { logic_oper … } ")"
func (p *V10Parser) ParseLogicGrouping() (*V10LogicGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if _, err := p.expect(V10_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseLogicGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V10LogicGroupTerm{{Oper: "", Expr: first}}

	for {
		oper := v10logicOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		item, err := p.parseLogicGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V10LogicGroupTerm{Oper: oper, Expr: item})
	}

	if _, err := p.expect(V10_RPAREN); err != nil {
		return nil, err
	}
	return &V10LogicGroupingNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V10Parser) parseLogicGroupItem() (V10Node, error) {
	if p.cur().Type == V10_LPAREN {
		return p.ParseLogicGrouping()
	}
	return p.ParseLogicExprList()
}

// ParseLogicExpr parses:  logic_expr = logic_expr_list | logic_grouping
func (p *V10Parser) ParseLogicExpr() (*V10LogicExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V10Node
	var err error
	if p.cur().Type == V10_LPAREN {
		val, err = p.ParseLogicGrouping()
	} else {
		val, err = p.ParseLogicExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V10LogicExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- Calc unit ----------

// ParseCalcUnit parses:  calc_unit = numeric_expr | string_expr | logic_expr
//
// Disambiguation strategy based on the leading token:
//  1. String literal token → string_expr
//  2. Pure numeric literal with no following compare/logic operator → numeric_expr
//  3. Otherwise → try logic_expr (handles comparisons, negations, bare idents)
//  4. Fallback → numeric_expr (covers signed idents and edge cases)
func (p *V10Parser) ParseCalcUnit() (*V10CalcUnitNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// String head → string_expr directly.
	if v10isStringTok(tok.Type) {
		str, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		return &V10CalcUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: str}, nil
	}

	// Pure numeric literal: try numeric_expr; if a compare or logic oper
	// follows, fall through to the logic_expr path.
	if tok.Type == V10_INTEGER || tok.Type == V10_DECIMAL {
		saved := p.savePos()
		if num, err := p.ParseNumericExpr(); err == nil {
			if v10compareOper(p.cur().Type) == "" && v10logicOper(p.cur().Type) == "" {
				return &V10CalcUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: num}, nil
			}
		}
		p.restorePos(saved)
	}

	// General case: try logic_expr (covers comparisons, negation, ident refs).
	saved := p.savePos()
	if logic, err := p.ParseLogicExpr(); err == nil {
		return &V10CalcUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: logic}, nil
	}
	p.restorePos(saved)

	// Final fallback: numeric_expr.
	num, err := p.ParseNumericExpr()
	if err != nil {
		return nil, p.errAt(fmt.Sprintf("could not parse calc_unit: unexpected token %s %q", tok.Type, tok.Value))
	}
	return &V10CalcUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: num}, nil
}
