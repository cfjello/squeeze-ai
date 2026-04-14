// parser_V12_operators.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V12 grammar rule set defined in spec/02_operators.sqg.
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

// V12IdentDottedNode  ident_dotted = ident_name { "." ident_name }
type V12IdentDottedNode struct {
	V12BaseNode
	Parts []string // each trimmed ident_name segment
}

// V12IdentPrefixNode  ident_prefix = ( "../" { "../" } ) | "./"
type V12IdentPrefixNode struct {
	V12BaseNode
	Value string // "./" or "../" or "../../" etc.
}

// V12IdentRefNode  ident_ref = [ ident_prefix ] ident_dotted
type V12IdentRefNode struct {
	V12BaseNode
	Prefix *V12IdentPrefixNode // nil if no prefix
	Dotted *V12IdentDottedNode
}

// ---------- Numeric expressions ----------

// V12SingleNumExprNode  single_num_expr = numeric_const | [ inline_incr ] ident_ref
// (TYPE_OF numeric_const<ident_ref> is a grammar-level annotation; at parse
// time the ident_ref branch carries no extra token.)
type V12SingleNumExprNode struct {
	V12BaseNode
	Literal    *V12NumericConstNode // set when the value is a literal numeric_const
	InlineIncr string               // "", "++" or "--" — only when IdentRef != nil
	IdentRef   *V12IdentRefNode     // set when the value is an identifier reference
}

// V12NumExprTerm is one element of a num_expr_list chain.
type V12NumExprTerm struct {
	Oper string // "" for the first element; "+", "-", "*", "**", "/", "%" for rest
	Expr *V12SingleNumExprNode
}

// V12NumExprListNode  num_expr_list = single_num_expr { numeric_oper single_num_expr }
type V12NumExprListNode struct {
	V12BaseNode
	Terms []V12NumExprTerm
}

// V12NumGroupTerm is one element in a num_grouping.
type V12NumGroupTerm struct {
	Oper string  // "" for first element, else numeric_oper
	Expr V12Node // *V12NumExprListNode | *V12NumGroupingNode
}

// V12NumGroupingNode  num_grouping = "(" ( num_expr_list | num_grouping ) { numeric_oper … } ")"
type V12NumGroupingNode struct {
	V12BaseNode
	Terms []V12NumGroupTerm
}

// V12NumericExprNode  numeric_expr = num_expr_list | num_grouping
type V12NumericExprNode struct {
	V12BaseNode
	Value V12Node // *V12NumExprListNode | *V12NumGroupingNode
}

// ---------- String expressions ----------

// V12StringExprTerm is one element of a string_expr_list chain.
type V12StringExprTerm struct {
	Oper string // "" for first; "+" for subsequent elements
	Str  *V12StringNode
}

// V12StringExprListNode  string_expr_list = string { "+" string }
type V12StringExprListNode struct {
	V12BaseNode
	Terms []V12StringExprTerm
}

// V12StringGroupTerm is one element in a string_grouping.
type V12StringGroupTerm struct {
	Oper string  // "" for first; "+"
	Expr V12Node // *V12StringExprListNode | *V12StringGroupingNode
}

// V12StringGroupingNode  string_grouping = "(" ( string_expr_list | string_grouping ) { "+" … } ")"
type V12StringGroupingNode struct {
	V12BaseNode
	Terms []V12StringGroupTerm
}

// V12StringExprNode  string_expr = string_expr_list | string_grouping
type V12StringExprNode struct {
	V12BaseNode
	Value V12Node // *V12StringExprListNode | *V12StringGroupingNode
}

// ---------- Compare expressions ----------

// V12NumCompareNode  TYPE_OF boolean<numeric_expr compare_oper numeric_expr>
// (TYPE_OF boolean<…> is the grammar annotation; at parse time this is simply
// left compare_oper right.)
type V12NumCompareNode struct {
	V12BaseNode
	Left  *V12NumericExprNode
	Oper  string // "!=", "==", ">", ">=", "<", "<="
	Right *V12NumericExprNode
}

// V12StringCompareNode  TYPE_OF boolean<string_expr compare_oper string_expr>
type V12StringCompareNode struct {
	V12BaseNode
	Left  *V12StringExprNode
	Oper  string
	Right *V12StringExprNode
}

// V12StringRegexpCompNode  TYPE_OF boolean<string_expr "=~" regexp>
type V12StringRegexpCompNode struct {
	V12BaseNode
	Left   *V12StringExprNode
	Regexp *V12RegexpNode
}

// V12CompareExprNode  compare_expr = num_compare | string_compare | string_regexp_comp
type V12CompareExprNode struct {
	V12BaseNode
	Value V12Node // *V12NumCompareNode | *V12StringCompareNode | *V12StringRegexpCompNode
}

// ---------- Logic expressions ----------

// V12SingleLogicExprNode  single_logic_expr
// grammar:
//
//	[ not_oper ] ( ident_dotted | TYPE_OF boolean<ident_ref> | compare_expr )
//	| numeric_expr | string_expr
//
// Exactly one of the Value fields is non-nil.
type V12SingleLogicExprNode struct {
	V12BaseNode
	Negated   bool
	IdentRef  *V12IdentRefNode // bare ident used as boolean expression
	Compare   *V12CompareExprNode
	Numeric   *V12NumericExprNode
	StringVal *V12StringExprNode
}

// V12LogicExprTerm is one element of a logic_expr_list.
type V12LogicExprTerm struct {
	Oper string // "" for first; "&", "|", "^" for rest
	Expr *V12SingleLogicExprNode
}

// V12LogicExprListNode  logic_expr_list = single_logic_expr { logic_oper single_logic_expr }
type V12LogicExprListNode struct {
	V12BaseNode
	Terms []V12LogicExprTerm
}

// V12LogicGroupTerm is one element of a logic_grouping.
type V12LogicGroupTerm struct {
	Oper string  // "" for first; "&", "|", "^"
	Expr V12Node // *V12LogicExprListNode | *V12LogicGroupingNode
}

// V12LogicGroupingNode  logic_grouping = "(" ( logic_expr_list | logic_grouping ) { logic_oper … } ")"
type V12LogicGroupingNode struct {
	V12BaseNode
	Terms []V12LogicGroupTerm
}

// V12LogicExprNode  logic_expr = logic_expr_list | logic_grouping
type V12LogicExprNode struct {
	V12BaseNode
	Value V12Node // *V12LogicExprListNode | *V12LogicGroupingNode
}

// ---------- Top-level expression ----------

// V12CalcUnitNode  calc_unit = numeric_expr | string_expr | logic_expr
type V12CalcUnitNode struct {
	V12BaseNode
	Value V12Node // *V12NumericExprNode | *V12StringExprNode | *V12LogicExprNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (02_operators.sqg)
// =============================================================================

// ---------- Position save/restore for local backtracking ----------

func (p *V12Parser) savePos() int       { return p.pos }
func (p *V12Parser) restorePos(pos int) { p.pos = pos }

// ---------- Operator classification helpers ----------

// V12numericOper returns the string form of tok if it is a numeric_oper, else "".
func V12numericOper(t V12TokenType) string {
	switch t {
	case V12_PLUS:
		return "+"
	case V12_MINUS:
		return "-"
	case V12_STAR:
		return "*"
	case V12_POW:
		return "**"
	case V12_SLASH:
		return "/"
	case V12_PERCENT:
		return "%"
	}
	return ""
}

// V12inlineIncr returns "++" or "--" if tok is an inline_incr, else "".
func V12inlineIncr(t V12TokenType) string {
	switch t {
	case V12_INC:
		return "++"
	case V12_DEC:
		return "--"
	}
	return ""
}

// V12compareOper returns the string form of tok if it is a compare_oper, else "".
func V12compareOper(t V12TokenType) string {
	switch t {
	case V12_NEQ:
		return "!="
	case V12_EQEQ:
		return "=="
	case V12_GT:
		return ">"
	case V12_GEQ:
		return ">="
	case V12_LT:
		return "<"
	case V12_LEQ:
		return "<="
	}
	return ""
}

// V12logicOper returns the string form of tok if it is a logic_oper, else "".
func V12logicOper(t V12TokenType) string {
	switch t {
	case V12_AMP:
		return "&"
	case V12_PIPE:
		return "|"
	case V12_CARET:
		return "^"
	}
	return ""
}

// V12isStringTok returns true if tok can begin a string expression.
func V12isStringTok(t V12TokenType) bool {
	switch t {
	case V12_STRING, V12_EMPTY_STR_D, V12_EMPTY_STR_S, V12_EMPTY_STR_T:
		return true
	}
	return false
}

// ---------- Identifier parsers ----------

// ParseIdentDotted parses:  ident_dotted = ident_name { "." ident_name }
func (p *V12Parser) ParseIdentDotted() (*V12IdentDottedNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V12_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected identifier, got %s %q", tok.Type, tok.Value))
	}
	parts := []string{tok.Value}
	p.advance()

	for p.cur().Type == V12_DOT {
		saved := p.savePos()
		p.advance() // consume "."
		if p.cur().Type != V12_IDENT {
			p.restorePos(saved)
			break
		}
		parts = append(parts, p.cur().Value)
		p.advance()
	}

	return &V12IdentDottedNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Parts:       parts,
	}, nil
}

// ParseIdentPrefix parses:  ident_prefix = ( "../" { "../" } ) | "./"
// Token mapping:
//
//	"./"  → V12_DOT followed by V12_SLASH
//	"../" → V12_DOTDOT followed by V12_SLASH
func (p *V12Parser) ParseIdentPrefix() (*V12IdentPrefixNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var sb strings.Builder

	switch tok.Type {
	case V12_DOT: // "./"
		p.advance()
		if _, err := p.expect(V12_SLASH); err != nil {
			return nil, err
		}
		sb.WriteString("./")

	case V12_DOTDOT: // "../" { "../" }
		for p.cur().Type == V12_DOTDOT {
			p.advance()
			if _, err := p.expect(V12_SLASH); err != nil {
				return nil, err
			}
			sb.WriteString("../")
		}

	default:
		return nil, p.errAt(fmt.Sprintf("expected './' or '../', got %s %q", tok.Type, tok.Value))
	}

	return &V12IdentPrefixNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Value:       sb.String(),
	}, nil
}

// ParseIdentRef parses:  ident_ref = [ ident_prefix ] ident_dotted
// The prefix is attempted speculatively: if a "." or ".." is followed by "/"
// it is a prefix; otherwise we skip the prefix and parse ident_dotted directly.
func (p *V12Parser) ParseIdentRef() (*V12IdentRefNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V12IdentRefNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}

	tok := p.cur()
	// A prefix starts with "." or ".." — but only when the very next
	// significant token is "/" (slash), distinguishing "./" and "../" from
	// the "." member-access in ident_dotted.
	if tok.Type == V12_DOT || tok.Type == V12_DOTDOT {
		saved := p.savePos()
		prefix, err := p.ParseIdentPrefix()
		if err != nil || p.cur().Type != V12_IDENT {
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
func (p *V12Parser) ParseSingleNumExpr() (*V12SingleNumExprNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Literal numeric branch
	switch tok.Type {
	case V12_INTEGER, V12_DECIMAL:
		lit, err := p.ParseNumericConst()
		if err != nil {
			return nil, err
		}
		return &V12SingleNumExprNode{
			V12BaseNode: V12BaseNode{Line: line, Col: col},
			Literal:     lit,
		}, nil

	case V12_PLUS, V12_MINUS:
		next := p.peek(1)
		if next.Type == V12_INTEGER || next.Type == V12_DECIMAL {
			lit, err := p.ParseNumericConst()
			if err != nil {
				return nil, err
			}
			return &V12SingleNumExprNode{
				V12BaseNode: V12BaseNode{Line: line, Col: col},
				Literal:     lit,
			}, nil
		}
	}

	// Optional inline_incr + ident_ref branch
	incr := V12inlineIncr(tok.Type)
	if incr != "" {
		p.advance()
	}
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	return &V12SingleNumExprNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		InlineIncr:  incr,
		IdentRef:    ref,
	}, nil
}

// ParseNumExprList parses:
//
//	num_expr_list = single_num_expr { numeric_oper single_num_expr }
func (p *V12Parser) ParseNumExprList() (*V12NumExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col

	first, err := p.ParseSingleNumExpr()
	if err != nil {
		return nil, err
	}
	terms := []V12NumExprTerm{{Oper: "", Expr: first}}

	for {
		oper := V12numericOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		next, err := p.ParseSingleNumExpr()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V12NumExprTerm{Oper: oper, Expr: next})
	}

	return &V12NumExprListNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseNumGrouping parses:
//
//	num_grouping = "(" ( num_expr_list | num_grouping ) { numeric_oper ( num_expr_list | num_grouping ) } ")"
func (p *V12Parser) ParseNumGrouping() (*V12NumGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if _, err := p.expect(V12_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseNumGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V12NumGroupTerm{{Oper: "", Expr: first}}

	for {
		oper := V12numericOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		item, err := p.parseNumGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V12NumGroupTerm{Oper: oper, Expr: item})
	}

	if _, err := p.expect(V12_RPAREN); err != nil {
		return nil, err
	}
	return &V12NumGroupingNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V12Parser) parseNumGroupItem() (V12Node, error) {
	if p.cur().Type == V12_LPAREN {
		return p.ParseNumGrouping()
	}
	return p.ParseNumExprList()
}

// ParseNumericExpr parses:  numeric_expr = num_expr_list | num_grouping
func (p *V12Parser) ParseNumericExpr() (*V12NumericExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V12Node
	var err error
	if p.cur().Type == V12_LPAREN {
		val, err = p.ParseNumGrouping()
	} else {
		val, err = p.ParseNumExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V12NumericExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- String expression parsers ----------

// ParseStringExprList parses:  string_expr_list = string { "+" string }
func (p *V12Parser) ParseStringExprList() (*V12StringExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col

	first, err := p.ParseString()
	if err != nil {
		return nil, err
	}
	terms := []V12StringExprTerm{{Oper: "", Str: first}}

	for p.cur().Type == V12_PLUS {
		p.advance()
		str, err := p.ParseString()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V12StringExprTerm{Oper: "+", Str: str})
	}

	return &V12StringExprListNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseStringGrouping parses:
//
//	string_grouping = "(" ( string_expr_list | string_grouping ) { "+" ( … ) } ")"
func (p *V12Parser) ParseStringGrouping() (*V12StringGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if _, err := p.expect(V12_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseStringGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V12StringGroupTerm{{Oper: "", Expr: first}}

	for p.cur().Type == V12_PLUS {
		p.advance()
		item, err := p.parseStringGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V12StringGroupTerm{Oper: "+", Expr: item})
	}

	if _, err := p.expect(V12_RPAREN); err != nil {
		return nil, err
	}
	return &V12StringGroupingNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V12Parser) parseStringGroupItem() (V12Node, error) {
	if p.cur().Type == V12_LPAREN {
		return p.ParseStringGrouping()
	}
	return p.ParseStringExprList()
}

// ParseStringExpr parses:  string_expr = string_expr_list | string_grouping
func (p *V12Parser) ParseStringExpr() (*V12StringExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V12Node
	var err error
	if p.cur().Type == V12_LPAREN {
		val, err = p.ParseStringGrouping()
	} else {
		val, err = p.ParseStringExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V12StringExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- Compare expression parsers ----------

// ParseCompareExpr parses:
//
//	compare_expr = num_compare | string_compare | string_regexp_comp
//
// Disambiguation: if the left-hand token is a string literal, try
// string_regexp_comp (=~) then string_compare; otherwise try num_compare.
func (p *V12Parser) ParseCompareExpr() (*V12CompareExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V12Node
	var err error

	if V12isStringTok(p.cur().Type) {
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
	return &V12CompareExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: val}, nil
}

func (p *V12Parser) parseNumCompare() (*V12NumCompareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseNumericExpr()
	if err != nil {
		return nil, err
	}
	oper := V12compareOper(p.cur().Type)
	if oper == "" {
		return nil, p.errAt(fmt.Sprintf("expected compare operator, got %s %q", p.cur().Type, p.cur().Value))
	}
	p.advance()
	right, err := p.ParseNumericExpr()
	if err != nil {
		return nil, err
	}
	return &V12NumCompareNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        oper,
		Right:       right,
	}, nil
}

func (p *V12Parser) parseStringCompare() (*V12StringCompareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	oper := V12compareOper(p.cur().Type)
	if oper == "" {
		return nil, p.errAt(fmt.Sprintf("expected compare operator, got %s %q", p.cur().Type, p.cur().Value))
	}
	p.advance()
	right, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	return &V12StringCompareNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Left:        left,
		Oper:        oper,
		Right:       right,
	}, nil
}

func (p *V12Parser) parseStringRegexpComp() (*V12StringRegexpCompNode, error) {
	line, col := p.cur().Line, p.cur().Col
	left, err := p.ParseStringExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_MATCH_OP); err != nil {
		return nil, err
	}
	re, err := p.ParseRegexp()
	if err != nil {
		return nil, err
	}
	return &V12StringRegexpCompNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
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
func (p *V12Parser) ParseSingleLogicExpr() (*V12SingleLogicExprNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	node := &V12SingleLogicExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}

	// Optional negation — only valid before the first three alternatives.
	if tok.Type == V12_BANG {
		node.Negated = true
		p.advance()
		tok = p.cur()
	}

	// string_expr branch (no negation in grammar for this path)
	if !node.Negated && V12isStringTok(tok.Type) {
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
		case V12_INTEGER, V12_DECIMAL:
			num, err := p.ParseNumericExpr()
			if err != nil {
				return nil, err
			}
			node.Numeric = num
			return node, nil
		}
	}

	// Parenthesised grouping — try numeric, then string as fallback.
	if tok.Type == V12_LPAREN {
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
func (p *V12Parser) ParseLogicExprList() (*V12LogicExprListNode, error) {
	line, col := p.cur().Line, p.cur().Col

	first, err := p.ParseSingleLogicExpr()
	if err != nil {
		return nil, err
	}
	terms := []V12LogicExprTerm{{Oper: "", Expr: first}}

	for {
		oper := V12logicOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		next, err := p.ParseSingleLogicExpr()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V12LogicExprTerm{Oper: oper, Expr: next})
	}

	return &V12LogicExprListNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

// ParseLogicGrouping parses:
//
//	logic_grouping = "(" ( logic_expr_list | logic_grouping ) { logic_oper … } ")"
func (p *V12Parser) ParseLogicGrouping() (*V12LogicGroupingNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if _, err := p.expect(V12_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.parseLogicGroupItem()
	if err != nil {
		return nil, err
	}
	terms := []V12LogicGroupTerm{{Oper: "", Expr: first}}

	for {
		oper := V12logicOper(p.cur().Type)
		if oper == "" {
			break
		}
		p.advance()
		item, err := p.parseLogicGroupItem()
		if err != nil {
			return nil, err
		}
		terms = append(terms, V12LogicGroupTerm{Oper: oper, Expr: item})
	}

	if _, err := p.expect(V12_RPAREN); err != nil {
		return nil, err
	}
	return &V12LogicGroupingNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Terms: terms}, nil
}

func (p *V12Parser) parseLogicGroupItem() (V12Node, error) {
	if p.cur().Type == V12_LPAREN {
		return p.ParseLogicGrouping()
	}
	return p.ParseLogicExprList()
}

// ParseLogicExpr parses:  logic_expr = logic_expr_list | logic_grouping
func (p *V12Parser) ParseLogicExpr() (*V12LogicExprNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var val V12Node
	var err error
	if p.cur().Type == V12_LPAREN {
		val, err = p.ParseLogicGrouping()
	} else {
		val, err = p.ParseLogicExprList()
	}
	if err != nil {
		return nil, err
	}
	return &V12LogicExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ---------- Calc unit ----------

// ParseCalcUnit parses:  calc_unit = numeric_expr | string_expr | logic_expr
//
// Disambiguation strategy based on the leading token:
//  1. String literal token → string_expr
//  2. Pure numeric literal with no following compare/logic operator → numeric_expr
//  3. Otherwise → try logic_expr (handles comparisons, negations, bare idents)
//  4. Fallback → numeric_expr (covers signed idents and edge cases)
func (p *V12Parser) ParseCalcUnit() (*V12CalcUnitNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// self_ref "$" — used in object values like  type: $
	if tok.Type == V12_DOLLAR {
		p.advance()
		return &V12CalcUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: &V12SelfRefNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}}, nil
	}

	// String head → string_expr directly.
	if V12isStringTok(tok.Type) {
		str, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		return &V12CalcUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: str}, nil
	}

	// Pure numeric literal: try numeric_expr; if a compare or logic oper
	// follows, fall through to the logic_expr path.
	if tok.Type == V12_INTEGER || tok.Type == V12_DECIMAL {
		saved := p.savePos()
		if num, err := p.ParseNumericExpr(); err == nil {
			if V12compareOper(p.cur().Type) == "" && V12logicOper(p.cur().Type) == "" {
				return &V12CalcUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: num}, nil
			}
		}
		p.restorePos(saved)
	}

	// General case: try logic_expr (covers comparisons, negation, ident refs).
	saved := p.savePos()
	if logic, err := p.ParseLogicExpr(); err == nil {
		return &V12CalcUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: logic}, nil
	}
	p.restorePos(saved)

	// Final fallback: numeric_expr.
	num, err := p.ParseNumericExpr()
	if err != nil {
		return nil, p.errAt(fmt.Sprintf("could not parse calc_unit: unexpected token %s %q", tok.Type, tok.Value))
	}
	return &V12CalcUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: num}, nil
}
