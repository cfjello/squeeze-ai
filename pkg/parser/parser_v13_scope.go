// parser_v13_scope.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/07_types_scope.sqg.
//
// V13 changes vs V12:  none (all rules carry forward unchanged).
//
// Covered rules:
//
//	extend_scope_oper
//	scope_inject
//	import_assign
//	scope_assign
//	parser_root
package parser

import "fmt"

// =============================================================================
// PHASE 2 — AST NODE TYPES  (07_types_scope.sqg)
// =============================================================================

// V13ScopeInjectBind  assign_lhs ":" ident_ref — one binding inside scope_inject
type V13ScopeInjectBind struct {
	LHS *V13AssignLHSNode
	Ref *V13IdentRefNode
}

// V13ScopeInjectNode  scope_inject = "(" assign_lhs ":" ident_ref { "," ... } ")"
type V13ScopeInjectNode struct {
	V13BaseNode
	Binds []V13ScopeInjectBind
}

// V13ImportAssignNode  import_assign = assign_lhs ":" ( http_url | file_path )
type V13ImportAssignNode struct {
	V13BaseNode
	LHS    *V13AssignLHSNode
	Target V13Node // *V13URLNode | *V13FilePathNode
}

// V13ScopeBodyItemNode  one element inside a scope body
type V13ScopeBodyItemNode struct {
	V13BaseNode
	Value V13Node
}

// V13ScopeAssignNode
//
//	scope_assign = [ scope_inject ] assign_lhs ( equal_assign | "+=" )
//	               "{" body_item { ( NL | "," ) body_item } "}"
type V13ScopeAssignNode struct {
	V13BaseNode
	Inject *V13ScopeInjectNode
	LHS    *V13AssignLHSNode
	Oper   *V13AssignOperNode
	Body   []*V13ScopeBodyItemNode
}

// V13ParserRootNode  parser_root = import_assign | scope_assign
type V13ParserRootNode struct {
	V13BaseNode
	Value V13Node
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
//	scope_inject = "(" assign_lhs ":" ident_ref { "," assign_lhs ":" ident_ref } ")"
func (p *V13Parser) ParseScopeInject() (*V13ScopeInjectNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}

	parseBind := func() (*V13ScopeInjectBind, error) {
		lhs, err := p.ParseAssignLHS()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V13_COLON); err != nil {
			return nil, err
		}
		ref, err := p.ParseIdentRef()
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

// ParseImportAssign parses:  import_assign = assign_lhs ":" ( http_url | file_path )
func (p *V13Parser) ParseImportAssign() (*V13ImportAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}

	var target V13Node
	saved := p.savePos()
	if u, err := p.ParseHTTPURL(); err == nil {
		target = u
	} else {
		p.restorePos(saved)
		fp, err := p.ParseFilePath()
		if err != nil {
			return nil, fmt.Errorf("import_assign: expected http_url or file_path: %w", err)
		}
		target = fp
	}
	return &V13ImportAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Target:      target,
	}, nil
}

// parseScopeBodyItem parses one body item:  import_assign | assignment | scope_assign
func (p *V13Parser) parseScopeBodyItem() (*V13ScopeBodyItemNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13ScopeBodyItemNode {
		return &V13ScopeBodyItemNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	saved := p.savePos()
	if sa, err := p.ParseScopeAssign(); err == nil {
		return wrap(sa), nil
	}
	p.restorePos(saved)

	saved = p.savePos()
	if ia, err := p.ParseImportAssign(); err == nil {
		return wrap(ia), nil
	}
	p.restorePos(saved)

	a, err := p.ParseAssignment()
	if err != nil {
		return nil, err
	}
	return wrap(a), nil
}

// ParseScopeAssign parses:
//
//	scope_assign = [ scope_inject ] assign_lhs ( equal_assign | "+=" )
//	               "{" body_item { ( NL | "," ) body_item } "}"
func (p *V13Parser) ParseScopeAssign() (*V13ScopeAssignNode, error) {
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

	if _, err := p.expect(V13_LBRACE); err != nil {
		return nil, err
	}
	p.V13skipNLs()

	node := &V13ScopeAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Inject:      inj,
		LHS:         lhs,
		Oper:        oper,
	}

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

// ParseParserRoot parses:  parser_root = import_assign | scope_assign
func (p *V13Parser) ParseParserRoot() (*V13ParserRootNode, error) {
	line, col := p.cur().Line, p.cur().Col

	saved := p.savePos()
	if sa, err := p.ParseScopeAssign(); err == nil {
		return &V13ParserRootNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: sa}, nil
	}
	p.restorePos(saved)

	ia, err := p.ParseImportAssign()
	if err != nil {
		return nil, err
	}
	return &V13ParserRootNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: ia}, nil
}

var _ = fmt.Sprintf
