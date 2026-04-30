// parser_v13_scope.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V14 grammar rule set defined in spec/07_types_scope.sqg.
//
// V14 changes vs V13:
//   - private_modifier moved to 03_assignment.sqg
//   - assignment gains optional private_modifier prefix
//   - scope_inject Ref generalized to V13Node (type_prefix ident_ref | any_type)
//   - import_assign / other_inline_assign gain EOL + optional private_modifier prefix
//   - private_block reshaped (no private_item; uses scope_body_item separators)
//   - scope_body new rule wrapping scoped body + { scope_body_catch } tail
//   - scope_assign delegates body to ParseScopeBody()
//   - scope_merge_tail, scope_final new rules
//   - parser_root = import_assign | scope_final
//   - scope_with_catch, receiver_clause, receiver_method_assign, private_item DELETED
//
// Covered rules:
//
//	extend_scope_oper
//	scope_inject
//	scope_assign_inline
//	import_assign
//	other_inline_assign
//	private_block
//	scope_body_item
//	scope_body
//	scope_assign
//	scope_merge_tail
//	scope_final
//	parser_root
package parser

import (
	"fmt"
)

// =============================================================================
// PHASE 2 — AST NODE TYPES  (07_types_scope.sqg)
// =============================================================================

// V13ScopeInjectRefNode — ORPHANED: superseded by returning *V13InspectTypeNode or *V13IdentRefNode
// directly from ParseScopeInject.parseRef after the lexer no longer fuses @+ident tokens.
// Kept until confirmed unused.
type V13ScopeInjectRefNode struct {
	V13BaseNode
	TypePrefix string           // e.g. "@MyType"; empty when absent
	Ref        *V13IdentRefNode // the bound ident_ref
}

// V13ScopeInjectBind  assign_lhs ":" ( inspect_type | ident_ref ) — one binding inside scope_inject.
// Ref is either *V13InspectTypeNode or *V13IdentRefNode.
type V13ScopeInjectBind struct {
	LHS *V13AssignLHSNode
	Ref V13Node // *V13InspectTypeNode | *V13IdentRefNode
}

// V13ScopeInjectNode  scope_inject = "(" assign_lhs ":" ident_ref { "," ... } ")"
type V13ScopeInjectNode struct {
	V13BaseNode
	Binds []V13ScopeInjectBind
}

// V13ScopeAssignInlineNode  scope_assign_inline = "_"
// A wildcard import: all exported names from the target source units are
// injected directly into the current scope with no binding name.
// Use at your own peril — see spec/07_types_scope.sqg.
type V13ScopeAssignInlineNode struct {
	V13BaseNode
}

// V13OtherInlineAssignNode  other_inline_assign = EOL [ private_modifier ] scope_assign_inline assign_oper assign_rhs
// A discarded-result assignment: the RHS is evaluated (for side effects);
// the result is not bound to any name in the current scope.
type V13OtherInlineAssignNode struct {
	V13BaseNode
	Private bool
	Oper    *V13AssignOperNode
	RHS     V13Node
}

// V13ImportAssignNode  import_assign = EOL [ private_modifier ] ( assign_lhs | scope_assign_inline ) assign_immutable ( http_url | file_url ) { "," ... }
// LHS is either *V13AssignLHSNode (named import) or *V13ScopeAssignInlineNode (wildcard import).
type V13ImportAssignNode struct {
	V13BaseNode
	Private bool
	LHS     V13Node   // *V13AssignLHSNode | *V13ScopeAssignInlineNode
	Targets []V13Node // each element: *V13URLNode | *V13FileURLNode
}

// V13ScopeBodyItemNode  one element inside a scope body
type V13ScopeBodyItemNode struct {
	V13BaseNode
	Value V13Node
}

// V13PrivateItemNode  private_item = "-" scope_body_item
// ORPHANED: private_item deleted in V14 — concept dissolved into assignment's [ private_modifier ].
// Kept until confirmed unused throughout the codebase.
type V13PrivateItemNode struct {
	V13BaseNode
	Item *V13ScopeBodyItemNode
}

// V13PrivateBlockNode  private_block = "-" ( "(" body_items ")" | "{" body_items "}" )
//
// A parenthesised or braced group whose contents are all implicitly private.
// UseGroups tracks whether the outer delimiter was "(" (true) or "{" (false).
type V13PrivateBlockNode struct {
	V13BaseNode
	UseParen bool // true = "( ... )", false = "{ ... }"
	Items    []*V13ScopeBodyItemNode
}

// V13ScopeAssignNode
//
//	scope_assign = [ scope_inject ] assign_lhs ( equal_assign | extend_scope_oper ) ( scope_body | empty_scope_decl )
type V13ScopeAssignNode struct {
	V13BaseNode
	Inject *V13ScopeInjectNode
	LHS    *V13AssignLHSNode
	Oper   *V13AssignOperNode
	Body   *V13ScopeBodyNode // nil when empty_scope_decl ( "{}" )
}

// V13ScopeWithCatchNode — ORPHANED: scope_with_catch deleted in V14.
// Superseded by scope_body { scope_body_catch } in 07_types_scope.sqg + 16_exceptions.sqg.
// Kept until confirmed unused throughout the codebase.
type V13ScopeWithCatchNode struct {
	V13BaseNode
	Scope   *V13ScopeAssignNode
	Handler *V13FuncUnitNode
}

// V13ParserRootNode  parser_root = import_assign | scope_final
// Note: in V13, scope_final did not exist; this remains scope_assign | import_assign
// until ParseScopeFinal() is implemented.
type V13ParserRootNode struct {
	V13BaseNode
	Value V13Node // *V13ImportAssignNode | *V13ScopeFinalNode
}

// V13FuncScopeAssignNode  func_scope_assign = [ scope_inject ] assign_lhs assign_oper func_unit
//
// Captures outer-scope ident_refs as immutable closure variables that are
// visible inside the func_unit body.
type V13FuncScopeAssignNode struct {
	V13BaseNode
	Inject *V13ScopeInjectNode // nil when no scope_inject prefix is present
	LHS    *V13AssignLHSNode
	Oper   *V13AssignOperNode
	Func   *V13FuncUnitNode
}

// V13ReceiverClauseNode — ORPHANED: receiver_clause deleted in V14 (AI-generated rule removed).
// Kept until confirmed unused throughout the codebase.
type V13ReceiverClauseNode struct {
	V13BaseNode
	Name string              // the ident_name token value
	Type *V13InspectTypeNode // the inspect_type annotation
}

// V13ReceiverMethodAssignNode — ORPHANED: receiver_method_assign deleted in V14 (AI-generated rule removed).
// Kept until confirmed unused throughout the codebase.
type V13ReceiverMethodAssignNode struct {
	V13BaseNode
	Receiver     *V13ReceiverClauseNode
	LHS          *V13AssignLHSNode
	Oper         *V13AssignOperNode
	RHS          *V13AssignRHSNode       // non-nil when RHS is an assign_rhs expression
	Body         []*V13ScopeBodyItemNode // non-nil when RHS is a scope { ... }
	CatchHandler *V13FuncUnitNode        // non-nil when a ^ catch_oper func_unit follows
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (07_types_scope.sqg)
// =============================================================================

// V13isScopeAssignOper returns true for tokens that serve as scope_assign operators.
func V13isScopeAssignOper(t V13TokenType) bool {
	switch t {
	case V13_EQ, V13_COLON, V13_READONLY, V13_EXTEND_ASSIGN:
		return true
	}
	return false
}

// ParseScopeInject parses:
//
//	scope_inject = "(" assign_lhs assign_immutable ( ( [ type_prefix ] ident_ref ) | any_type ) { "," ... } ")"
func (p *V13Parser) ParseScopeInject() (*V13ScopeInjectNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}

	// parseRef parses ( [ type_prefix ] ident_ref ) | any_type
	// type_prefix = "@"; ident_ref may be dotted (e.g. std.collections)
	parseRef := func() (V13Node, error) {
		if p.cur().Type == V13_ANY_TYPE || p.cur().Type == V13_AT {
			return p.ParseInspectType()
		}
		return p.ParseIdentRef()
	}

	parseBind := func() (*V13ScopeInjectBind, error) {
		lhs, err := p.ParseAssignLHS()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V13_COLON); err != nil {
			return nil, err
		}
		ref, err := parseRef()
		if err != nil {
			return nil, err
		}
		return &V13ScopeInjectBind{LHS: lhs, Ref: ref}, nil
	}

	first, err := parseBind()
	if err != nil {
		return nil, err
	}
	node := &V13ScopeInjectNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Binds:       []V13ScopeInjectBind{*first},
	}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		b, err := parseBind()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Binds = append(node.Binds, *b)
	}
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return node, nil
}

// ParseImportAssign parses:
//
//	import_assign = EOL [ private_modifier ] ( assign_lhs | scope_assign_inline ) assign_immutable ( http_url | file_url ) { "," ... }
func (p *V13Parser) ParseImportAssign() (*V13ImportAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// EOL is required
	if p.cur().Type != V13_NL && p.cur().Type != V13_BOF {
		return nil, p.errAt("import_assign: expected EOL")
	}
	p.advance()

	// optional private_modifier = "-"
	private := false
	if p.cur().Type == V13_MINUS {
		private = true
		p.advance()
	}

	// Determine LHS: scope_assign_inline ("_") or a normal assign_lhs.
	var lhs V13Node
	if p.cur().Type == V13_IDENT && p.cur().Value == "_" {
		lhs = &V13ScopeAssignInlineNode{V13BaseNode: V13BaseNode{Line: p.cur().Line, Col: p.cur().Col}}
		p.advance()
	} else {
		al, err := p.ParseAssignLHS()
		if err != nil {
			return nil, err
		}
		lhs = al
	}

	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}

	parseTarget := func() (V13Node, error) {
		saved := p.savePos()
		if u, err := p.ParseHTTPURL(); err == nil {
			return u, nil
		}
		p.restorePos(saved)
		fu, err := p.ParseFileURL()
		if err != nil {
			return nil, fmt.Errorf("import_assign: expected http_url or file_url: %w", err)
		}
		return fu, nil
	}

	first, err := parseTarget()
	if err != nil {
		return nil, err
	}
	node := &V13ImportAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Private:     private,
		LHS:         lhs,
		Targets:     []V13Node{first},
	}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		t, err := parseTarget()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Targets = append(node.Targets, t)
	}
	return node, nil
}

// ParseOtherInlineAssign parses:
//
//	other_inline_assign = EOL [ private_modifier ] scope_assign_inline assign_oper assign_rhs
func (p *V13Parser) ParseOtherInlineAssign() (*V13OtherInlineAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// EOL is required
	if p.cur().Type != V13_NL && p.cur().Type != V13_BOF {
		return nil, p.errAt("other_inline_assign: expected EOL")
	}
	p.advance()

	// optional private_modifier = "-"
	private := false
	if p.cur().Type == V13_MINUS {
		private = true
		p.advance()
	}

	// scope_assign_inline = "_"
	if p.cur().Type != V13_IDENT || p.cur().Value != "_" {
		return nil, p.errAt("other_inline_assign: expected '_'")
	}
	p.advance()

	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, fmt.Errorf("other_inline_assign: expected assign_oper: %w", err)
	}

	rhs, err := p.ParseAssignRHS()
	if err != nil {
		return nil, fmt.Errorf("other_inline_assign: expected assign_rhs: %w", err)
	}

	return &V13OtherInlineAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Private:     private,
		Oper:        oper,
		RHS:         rhs,
	}, nil
}

// parseScopeBodyItem parses one scope_body_item:
//
//	scope_body_item = import_assign | other_inline_assign | assignment
//	                | private_block
//	EXTEND<scope_body_item> = | scope_final  (from spec/07_types_scope.sqg)
func (p *V13Parser) parseScopeBodyItem() (*V13ScopeBodyItemNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13ScopeBodyItemNode {
		return &V13ScopeBodyItemNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	// PIPELINE<name> directive (library source only).
	if p.cur().Type == V13_PIPELINE {
		pd, err := p.ParsePipelineDecl()
		if err != nil {
			return nil, err
		}
		return wrap(pd), nil
	}

	// private_block: starts with "-"
	if p.cur().Type == V13_MINUS {
		pi, err := p.parsePrivateModifierItem()
		if err != nil {
			return nil, err
		}
		return wrap(pi), nil
	}

	// scope_final (EXTEND): scope_assign | scope_merge_tail
	// Try before the fall-through assignment since scope_assign is more specific.
	saved := p.savePos()
	if sf, err := p.ParseScopeFinal(); err == nil {
		return wrap(sf), nil
	}
	p.restorePos(saved)

	// import_assign: starts with EOL + optional "-" + (assign_lhs | "_") + ":"
	saved = p.savePos()
	if ia, err := p.ParseImportAssign(); err == nil {
		return wrap(ia), nil
	}
	p.restorePos(saved)

	// other_inline_assign: starts with EOL + optional "-" + "_" + assign_oper
	saved = p.savePos()
	if oia, err := p.ParseOtherInlineAssign(); err == nil {
		return wrap(oia), nil
	}
	p.restorePos(saved)

	// func_assign with inject prefix: ( inject ) assign_lhs assign_oper func_stmt
	// Handles method definitions like  (arr : @array) length: <- (func_unit)
	// that cannot be parsed by plain ParseAssignment (which has no inject prefix).
	if p.cur().Type == V13_LPAREN {
		saved = p.savePos()
		if fa, err := p.ParseFuncAssign(); err == nil {
			return wrap(fa), nil
		}
		p.restorePos(saved)
	}

	// assignment (plain): the most generic form
	a, err := p.ParseAssignment()
	if err != nil {
		p.trackUnknown(p.cur())
		return nil, err
	}
	return wrap(a), nil
}

// parsePrivateItems parses a sequence of scope_body_items (without nested
// private blocks) separated by EOL or comma, terminated by endTok.
func (p *V13Parser) parsePrivateItems(endTok V13TokenType) ([]*V13ScopeBodyItemNode, error) {
	var items []*V13ScopeBodyItemNode
	p.V13skipNLs()
	for p.cur().Type != endTok && p.cur().Type != V13_EOF {
		// Reject nested private modifier inside a private block.
		if p.cur().Type == V13_MINUS {
			return nil, p.errAt("private_block: nested '-' private modifier is not permitted")
		}
		line, col := p.cur().Line, p.cur().Col
		wrap := func(v V13Node) *V13ScopeBodyItemNode {
			return &V13ScopeBodyItemNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
		}

		saved := p.savePos()
		if sa, err := p.ParseScopeAssign(); err == nil {
			items = append(items, wrap(sa))
		} else {
			p.restorePos(saved)
			saved = p.savePos()
			if ia, err := p.ParseImportAssign(); err == nil {
				items = append(items, wrap(ia))
			} else {
				p.restorePos(saved)
				if p.cur().Type == V13_IDENT && p.cur().Value == "_" {
					saved = p.savePos()
					if oia, err := p.ParseOtherInlineAssign(); err == nil {
						items = append(items, wrap(oia))
					} else {
						p.restorePos(saved)
						a, err := p.ParseAssignment()
						if err != nil {
							return nil, err
						}
						items = append(items, wrap(a))
					}
				} else {
					a, err := p.ParseAssignment()
					if err != nil {
						return nil, err
					}
					items = append(items, wrap(a))
				}
			}
		}
		for p.cur().Type == V13_NL || p.cur().Type == V13_COMMA {
			p.pos++
		}
	}
	return items, nil
}

// parsePrivateModifierItem parses after consuming the "-" token either a
// private_block with "(...)" or "{...}" delimiters.
// V14: private_item (single "-" body_item) is removed — use "assignment" with Private=true instead.
func (p *V13Parser) parsePrivateModifierItem() (V13Node, error) {
	line, col := p.cur().Line, p.cur().Col
	// consume "-"
	p.advance()

	switch p.cur().Type {
	case V13_LPAREN:
		// private_block with "(" ... ")"
		p.advance() // consume "("
		items, err := p.parsePrivateItems(V13_RPAREN)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V13_RPAREN); err != nil {
			return nil, err
		}
		return &V13PrivateBlockNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			UseParen:    true,
			Items:       items,
		}, nil

	case V13_LBRACE:
		// private_block with "{" ... "}"
		p.advance() // consume "{"
		items, err := p.parsePrivateItems(V13_RBRACE)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V13_RBRACE); err != nil {
			return nil, err
		}
		return &V13PrivateBlockNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			UseParen:    false,
			Items:       items,
		}, nil

	default:
		// V14: single-item private_item removed — "-" must be followed by "(" or "{"
		return nil, p.errAt(fmt.Sprintf("private_block: expected '(' or '{' after '-', got %s %q", p.cur().Type, p.cur().Value))
	}
}

// ParseScopeAssign parses:
//
//	scope_assign = [ scope_inject ] assign_lhs ( equal_assign | extend_scope_oper ) ( scope_body | empty_scope_decl )
func (p *V13Parser) ParseScopeAssign() (node *V13ScopeAssignNode, err error) {
	done := p.debugEnter("scope_assign")
	defer func() { done(err == nil) }()

	line, col := p.cur().Line, p.cur().Col

	var inj *V13ScopeInjectNode
	if p.cur().Type == V13_LPAREN {
		saved := p.savePos()
		si, err := p.ParseScopeInject()
		if err == nil {
			inj = si
		} else {
			p.restorePos(saved)
		}
	}

	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}

	if !V13isScopeAssignOper(p.cur().Type) {
		return nil, p.errAt(fmt.Sprintf("scope_assign: expected '=', ':', ':~', or '+=', got %s %q", p.cur().Type, p.cur().Value))
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}

	node = &V13ScopeAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Inject:      inj,
		LHS:         lhs,
		Oper:        oper,
	}

	if p.cur().Type != V13_LBRACE {
		return nil, p.errAt(fmt.Sprintf("scope_assign: expected '{', got %s %q", p.cur().Type, p.cur().Value))
	}

	// empty_scope_decl: "{}" — an empty scope body
	saved := p.savePos()
	p.advance() // consume "{"
	p.V13skipNLs()
	if p.cur().Type == V13_RBRACE {
		bodyLine, bodyCol := p.cur().Line, p.cur().Col
		p.advance() // consume "}"
		node.Body = &V13ScopeBodyNode{V13BaseNode: V13BaseNode{Line: bodyLine, Col: bodyCol}}
		return node, nil
	}
	p.restorePos(saved)
	body, err := p.ParseScopeBody()
	if err != nil {
		return nil, err
	}
	node.Body = body
	return node, nil
}

// ParseFuncScopeAssign parses:
//
//	func_scope_assign = [ scope_inject ] assign_lhs assign_oper func_unit
//
// The scope_inject is the discriminating feature: it binds outer-scope
// ident_refs as immutable closure variables inside the func_unit body.
// When no scope_inject is present the rule still applies, but the caller
// should try this before the generic func_assign so that the presence of a
// scope_inject prefix always produces a V13FuncScopeAssignNode.
func (p *V13Parser) ParseFuncScopeAssign() (*V13FuncScopeAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Optionally parse scope_inject. The lookahead check (LPAREN present) avoids
	// wasting a speculative parse when there is clearly no inject prefix.
	var inj *V13ScopeInjectNode
	if p.cur().Type == V13_LPAREN {
		saved := p.savePos()
		si, err := p.ParseScopeInject()
		if err == nil {
			inj = si
		} else {
			p.restorePos(saved)
		}
	}

	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}

	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}

	fu, err := p.ParseFuncUnit()
	if err != nil {
		return nil, fmt.Errorf("func_scope_assign: RHS must be a func_unit: %w", err)
	}

	return &V13FuncScopeAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Inject:      inj,
		LHS:         lhs,
		Oper:        oper,
		Func:        fu,
	}, nil
}

// ParseReceiverClause — ORPHANED: receiver_clause deleted in V14.
// Kept until confirmed unused throughout the codebase.
func (p *V13Parser) ParseReceiverClause() (*V13ReceiverClauseNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}

	if p.cur().Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("receiver_clause: expected identifier, got %s %q", p.cur().Type, p.cur().Value))
	}
	name := p.cur().Value
	p.pos++

	typ, err := p.ParseInspectType()
	if err != nil {
		return nil, fmt.Errorf("receiver_clause: expected inspect_type: %w", err)
	}

	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}

	return &V13ReceiverClauseNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Name:        name,
		Type:        typ,
	}, nil
}

// ParseReceiverMethodAssign — ORPHANED: receiver_method_assign deleted in V14.
// Kept until confirmed unused throughout the codebase.
func (p *V13Parser) ParseReceiverMethodAssign() (*V13ReceiverMethodAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col

	recv, err := p.ParseReceiverClause()
	if err != nil {
		return nil, err
	}

	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}

	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}

	node := &V13ReceiverMethodAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Receiver:    recv,
		LHS:         lhs,
		Oper:        oper,
	}

	if p.cur().Type == V13_LBRACE {
		// scope-style body: { scope_body_items }
		p.pos++
		p.V13skipNLs()
		for p.cur().Type != V13_RBRACE && p.cur().Type != V13_EOF {
			item, err := p.parseScopeBodyItem()
			if err != nil {
				return nil, err
			}
			node.Body = append(node.Body, item)
			for p.cur().Type == V13_NL || p.cur().Type == V13_COMMA {
				p.pos++
			}
		}
		if _, err := p.expect(V13_RBRACE); err != nil {
			return nil, err
		}
	} else {
		// assign_rhs: covers func_unit, <- func_unit, expressions, etc.
		rhs, err := p.ParseAssignRHS()
		if err != nil {
			return nil, fmt.Errorf("receiver_method_assign: invalid RHS: %w", err)
		}
		node.RHS = rhs
	}

	// optional catch clause: ^ func_unit
	if p.cur().Type == V13_CARET {
		p.advance()
		handler, err := p.ParseFuncUnit()
		if err != nil {
			return nil, fmt.Errorf("receiver_method_assign: expected handler func_unit after '^': %w", err)
		}
		node.CatchHandler = handler
	}

	return node, nil
}

// ParseScopeWithCatch — ORPHANED: scope_with_catch deleted in V14.
// Superseded by scope_body { scope_body_catch }.
// Kept until confirmed unused throughout the codebase.
func (p *V13Parser) ParseScopeWithCatch() (*V13ScopeWithCatchNode, error) {
	line, col := p.cur().Line, p.cur().Col

	saved := p.savePos()
	sa, err := p.ParseScopeAssign()
	if err != nil {
		return nil, err
	}

	if p.cur().Type != V13_CARET {
		p.restorePos(saved)
		return nil, p.errAt("scope_with_catch: expected '^' after scope_assign")
	}
	p.advance()

	handler, err := p.ParseFuncUnit()
	if err != nil {
		return nil, fmt.Errorf("scope_with_catch: expected handler func_unit: %w", err)
	}

	return &V13ScopeWithCatchNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Scope:       sa,
		Handler:     handler,
	}, nil
}

// =============================================================================
// PHASE 3 — NEW RULES  (scope_body_catch, scope_body, scope_merge_tail, scope_final)
// =============================================================================

// V13ScopeBodyCatchNode  scope_body_catch = catch_oper func_unit
// catch_oper = "^" (V13_CARET); defined in spec/16_exceptions.sqg.
type V13ScopeBodyCatchNode struct {
	V13BaseNode
	Handler *V13FuncUnitNode
}

// ParseScopeBodyCatch parses:
//
//	scope_body_catch = catch_oper func_unit   (catch_oper = "^")
func (p *V13Parser) ParseScopeBodyCatch() (node *V13ScopeBodyCatchNode, err error) {
	done := p.debugEnter("scope_body_catch")
	defer func() { done(err == nil) }()

	line, col := p.cur().Line, p.cur().Col
	if _, err = p.expect(V13_CARET); err != nil {
		return nil, err
	}
	handler, err := p.ParseFuncUnit()
	if err != nil {
		return nil, fmt.Errorf("scope_body_catch: expected func_unit after '^': %w", err)
	}
	return &V13ScopeBodyCatchNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Handler:     handler,
	}, nil
}

// V13ScopeBodyNode  scope_body = "{" ( scope_body_item | private_block ) { (EOL|",") ... } "}" { scope_body_catch }
type V13ScopeBodyNode struct {
	V13BaseNode
	Items   []*V13ScopeBodyItemNode
	Catches []*V13ScopeBodyCatchNode
}

// ParseScopeBody parses:
//
//	scope_body = ( scope_begin ( scope_body_item | private_block ) { ( EOL | "," ) ( scope_body_item | private_block ) } scope_end )
//	             { scope_body_catch }
func (p *V13Parser) ParseScopeBody() (node *V13ScopeBodyNode, err error) {
	done := p.debugEnter("scope_body")
	defer func() { done(err == nil) }()

	line, col := p.cur().Line, p.cur().Col
	if _, err = p.expect(V13_LBRACE); err != nil {
		return nil, err
	}
	p.V13skipNLs()

	node = &V13ScopeBodyNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}

	for p.cur().Type != V13_RBRACE && p.cur().Type != V13_EOF {
		item, err := p.parseScopeBodyItem()
		if err != nil {
			return nil, err
		}
		node.Items = append(node.Items, item)
		for p.cur().Type == V13_NL || p.cur().Type == V13_COMMA {
			p.pos++
		}
	}
	if _, err = p.expect(V13_RBRACE); err != nil {
		return nil, err
	}

	// { scope_body_catch } trailing suffix
	for p.cur().Type == V13_CARET {
		c, err := p.ParseScopeBodyCatch()
		if err != nil {
			return nil, err
		}
		node.Catches = append(node.Catches, c)
	}
	return node, nil
}

// V13ScopeMergeTailNode  scope_merge_tail = ( TYPE_OF scope_assign<ident_ref> | scope_assign ) "+" ( TYPE_OF scope_body<ident_ref> | scope_body ) { "+" ... }
// Head is the leading scope_assign (TYPE_OF wrapper optional).
// Tails are the "+" scope_body additions.
type V13ScopeMergeTailNode struct {
	V13BaseNode
	// Head is either *V13ScopeAssignNode or *V13TypeOfRefNode (when TYPE_OF is present)
	Head  V13Node   // *V13ScopeAssignNode | *V13TypeOfRefNode
	Tails []V13Node // each: *V13ScopeBodyNode | *V13TypeOfRefNode
}

// parseScopeMergeItem parses one "+" tail: ( TYPE_OF scope_body<ident_ref> | scope_body )
func (p *V13Parser) parseScopeMergeItem() (V13Node, error) {
	if p.cur().Type == V13_TYPE_OF {
		return p.parseTypeOfRef()
	}
	return p.ParseScopeBody()
}

// ParseScopeMergeTail parses:
//
//	scope_merge_tail = ( TYPE_OF scope_assign<ident_ref> | scope_assign ) "+" ( TYPE_OF scope_body<ident_ref> | scope_body ) { "+" ... }
func (p *V13Parser) ParseScopeMergeTail() (node *V13ScopeMergeTailNode, err error) {
	done := p.debugEnter("scope_merge_tail")
	defer func() { done(err == nil) }()

	line, col := p.cur().Line, p.cur().Col

	// Head: TYPE_OF scope_assign<ident_ref> | scope_assign
	var head V13Node
	if p.cur().Type == V13_TYPE_OF {
		h, err := p.parseTypeOfRef()
		if err != nil {
			return nil, err
		}
		head = h
	} else {
		sa, err := p.ParseScopeAssign()
		if err != nil {
			return nil, err
		}
		head = sa
	}

	// require at least one "+"
	if p.cur().Type != V13_PLUS {
		return nil, p.errAt("scope_merge_tail: expected '+' after head scope_assign")
	}

	node = &V13ScopeMergeTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Head: head}

	for p.cur().Type == V13_PLUS {
		p.advance() // consume "+"
		tail, err := p.parseScopeMergeItem()
		if err != nil {
			return nil, fmt.Errorf("scope_merge_tail: expected scope_body after '+': %w", err)
		}
		node.Tails = append(node.Tails, tail)
	}
	return node, nil
}

// V13ScopeFinalNode  scope_final = scope_assign | scope_merge_tail
type V13ScopeFinalNode struct {
	V13BaseNode
	// Value is either *V13ScopeAssignNode or *V13ScopeMergeTailNode
	Value V13Node // *V13ScopeAssignNode | *V13ScopeMergeTailNode
}

// ParseScopeFinal parses:
//
//	scope_final = scope_assign | scope_merge_tail
//
// scope_merge_tail is a superset of scope_assign (it starts with the same tokens then
// requires a "+"), so we try scope_merge_tail first (speculative); fall back to scope_assign.
func (p *V13Parser) ParseScopeFinal() (node *V13ScopeFinalNode, err error) {
	done := p.debugEnter("scope_final")
	defer func() { done(err == nil) }()

	line, col := p.cur().Line, p.cur().Col

	saved := p.savePos()
	if mt, err := p.ParseScopeMergeTail(); err == nil {
		return &V13ScopeFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: mt}, nil
	}
	p.restorePos(saved)

	sa, err := p.ParseScopeAssign()
	if err != nil {
		return nil, err
	}
	return &V13ScopeFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: sa}, nil
}

// ParseParserRoot parses:  parser_root = import_assign | scope_final
func (p *V13Parser) ParseParserRoot() (node *V13ParserRootNode, err error) {
	done := p.debugEnter("parser_root")
	defer func() { done(err == nil) }()

	// Reset unknown-token tracker for each fresh parse.
	p.unknownTokens = p.unknownTokens[:0]

	line, col := p.cur().Line, p.cur().Col

	saved := p.savePos()
	if ia, err := p.ParseImportAssign(); err == nil {
		return &V13ParserRootNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: ia}, nil
	}
	p.restorePos(saved)

	sf, err := p.ParseScopeFinal()
	if err != nil {
		p.trackUnknown(p.cur())
		return nil, err
	}
	return &V13ParserRootNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: sf}, nil
}

var _ = fmt.Sprintf
