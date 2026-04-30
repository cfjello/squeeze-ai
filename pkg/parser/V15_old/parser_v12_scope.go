// parser_V12_scope.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V12 grammar rule set defined in spec/07_scope.sqg.
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
// PHASE 2 — AST NODE TYPES  (07_scope.sqg)
// =============================================================================

// V12ScopeInjectBind  assign_lhs ":" ident_ref — one binding inside scope_inject
type V12ScopeInjectBind struct {
	LHS *V12AssignLHSNode
	Ref *V12IdentRefNode
}

// V12ScopeInjectNode  scope_inject = "(" assign_lhs ":" ident_ref { "," ... } ")"
type V12ScopeInjectNode struct {
	V12BaseNode
	Binds []V12ScopeInjectBind
}

// V12ImportAssignNode  import_assign = assign_lhs ":" ( http_url | file_path )
type V12ImportAssignNode struct {
	V12BaseNode
	LHS    *V12AssignLHSNode
	Target V12Node // *V12URLNode | *V12FilePathNode
}

// V12ScopeBodyItemNode  one element inside a scope body:
// import_assign | assignment | scope_assign
type V12ScopeBodyItemNode struct {
	V12BaseNode
	Value V12Node // *V12ImportAssignNode | *V12AssignmentNode | *V12ScopeAssignNode
}

// V12ScopeAssignNode
//
//	scope_assign = [ scope_inject ] assign_lhs ( equal_assign | "+=" )
//	               "{" body_item { ( NL | "," ) body_item } "}"
type V12ScopeAssignNode struct {
	V12BaseNode
	Inject *V12ScopeInjectNode // nil when absent
	LHS    *V12AssignLHSNode
	Oper   *V12AssignOperNode // equal_assign or extend_scope_oper
	Body   []*V12ScopeBodyItemNode
}

// V12ParserRootNode  parser_root = import_assign | scope_assign
type V12ParserRootNode struct {
	V12BaseNode
	Value V12Node // *V12ImportAssignNode | *V12ScopeAssignNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (07_scope.sqg)
// =============================================================================

// ParseScopeInject parses:
//
//	scope_inject = "(" assign_lhs ":" ident_ref { "," assign_lhs ":" ident_ref } ")"
func (p *V12Parser) ParseScopeInject() (*V12ScopeInjectNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_LPAREN); err != nil {
		return nil, err
	}

	parseBind := func() (*V12ScopeInjectBind, error) {
		lhs, err := p.ParseAssignLHS()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V12_COLON); err != nil {
			return nil, err
		}
		ref, err := p.ParseIdentRef()
		if err != nil {
			return nil, err
		}
		return &V12ScopeInjectBind{LHS: lhs, Ref: ref}, nil
	}

	first, err := parseBind()
	if err != nil {
		return nil, err
	}
	node := &V12ScopeInjectNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Binds:       []V12ScopeInjectBind{*first},
	}
	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		b, err := parseBind()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Binds = append(node.Binds, *b)
	}
	if _, err := p.expect(V12_RPAREN); err != nil {
		return nil, err
	}
	return node, nil
}

// ParseImportAssign parses:
//
//	import_assign = assign_lhs ":" ( http_url | file_path )
func (p *V12Parser) ParseImportAssign() (*V12ImportAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_COLON); err != nil {
		return nil, err
	}

	// Both http_url and file_path appear as quoted V12_STRING tokens.
	// Try http_url first (validates URL scheme); fall back to file_path.
	var target V12Node
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
	return &V12ImportAssignNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Target:      target,
	}, nil
}

// V12isScopeAssignOper returns true for tokens that can serve as
// the scope_assign operator (equal_assign or extend_scope_oper).
func V12isScopeAssignOper(t V12TokenType) bool {
	switch t {
	case V12_EQ, V12_COLON, V12_READONLY, V12_EXTEND_ASSIGN:
		return true
	}
	return false
}

// parseScopeBodyItem parses one body item:  import_assign | assignment | scope_assign
func (p *V12Parser) parseScopeBodyItem() (*V12ScopeBodyItemNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V12Node) *V12ScopeBodyItemNode {
		return &V12ScopeBodyItemNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: v}
	}

	// scope_assign starts with optional "(" (scope_inject) or an ident followed
	// by a scope operator and then "{".  Try it first since it's the most specific.
	saved := p.savePos()
	if sa, err := p.ParseScopeAssign(); err == nil {
		return wrap(sa), nil
	}
	p.restorePos(saved)

	// import_assign: ident ":" <quoted-url-or-path>
	// We can tell it's an import if after ident ":" the next token is a STRING
	// that looks like a URL or path.  Try it with backtracking.
	saved = p.savePos()
	if ia, err := p.ParseImportAssign(); err == nil {
		return wrap(ia), nil
	}
	p.restorePos(saved)

	// Fallback: assignment
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
func (p *V12Parser) ParseScopeAssign() (*V12ScopeAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Optional scope_inject (only starts with "(")
	var inj *V12ScopeInjectNode
	if p.cur().Type == V12_LPAREN {
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

	// Operator must be equal_assign or extend_scope_oper.
	if !V12isScopeAssignOper(p.cur().Type) {
		return nil, p.errAt(fmt.Sprintf("scope_assign: expected '=', ':', ':~', or '+=', got %s %q", p.cur().Type, p.cur().Value))
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}

	// scope body delimited by "{" ... "}"
	if _, err := p.expect(V12_LBRACE); err != nil {
		return nil, err
	}
	p.V12skipNLs()

	node := &V12ScopeAssignNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Inject:      inj,
		LHS:         lhs,
		Oper:        oper,
	}

	// Parse body items separated by EOL or ","
	for p.cur().Type != V12_RBRACE && p.cur().Type != V12_EOF {
		item, err := p.parseScopeBodyItem()
		if err != nil {
			return nil, err
		}
		node.Body = append(node.Body, item)

		// Consume optional separators (NL or ",") before next item.
		for p.cur().Type == V12_NL || p.cur().Type == V12_COMMA {
			p.pos++
		}
	}

	if _, err := p.expect(V12_RBRACE); err != nil {
		return nil, err
	}
	return node, nil
}

// ParseParserRoot parses the top-level grammar entry point:
//
//	parser_root = import_assign | scope_assign
func (p *V12Parser) ParseParserRoot() (*V12ParserRootNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Try scope_assign first (more specific).
	saved := p.savePos()
	if sa, err := p.ParseScopeAssign(); err == nil {
		return &V12ParserRootNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: sa}, nil
	}
	p.restorePos(saved)

	ia, err := p.ParseImportAssign()
	if err != nil {
		return nil, err
	}
	return &V12ParserRootNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: ia}, nil
}

// Suppress unused import warning.
var _ = fmt.Sprintf
