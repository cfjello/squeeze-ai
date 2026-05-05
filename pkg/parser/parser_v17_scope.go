// parser_v17_scope.go — Parse methods for spec/07_types_scope.sqg.
//
// Covered rules:
//
//	assign_oper, extend_scope_oper,
//	scope_assign_inline,
//	scope_inject,
//	import_assign, other_inline_assign,
//	scope_body_item, private_block, scope_body,
//	scope_assign, scope_merge_tail, scope_final,
//	parser_root,
//	func_inject, func_scope_assign
//
// EXTEND<scope_body_item> = | scope_final | func_scope_assign
//
//	(both defined in this file)
package parser

import "fmt"

// =============================================================================
// OPERATOR TERMINALS
// assign_oper        = assign_immutable | assign_read_only_ref | assign_mutable
// extend_scope_oper  = "+="
// =============================================================================

// V17AssignOperNode  assign_oper = assign_immutable | assign_read_only_ref | assign_mutable
type V17AssignOperNode struct {
	V17BaseNode
	// Op is one of: *V17AssignImmutableNode | *V17AssignReadOnlyRefNode | *V17AssignMutableNode
	Op interface{}
}

// ParseAssignOper parses assign_oper = assign_immutable | assign_read_only_ref | assign_mutable.
// Tries assign_read_only_ref (":~") before assign_immutable (":") to avoid prefix match.
func (p *V17Parser) ParseAssignOper() (node *V17AssignOperNode, err error) {
	done := p.debugEnter("assign_oper")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// ":~" must precede ":"
	if p.peekLit(":~") {
		if ror, rorerr := p.ParseAssignReadOnlyRef(); rorerr == nil {
			return &V17AssignOperNode{V17BaseNode{line, col}, ror}, nil
		}
	}
	if p.peekAfterWS() == ':' {
		if ai, aierr := p.ParseAssignImmutable(); aierr == nil {
			return &V17AssignOperNode{V17BaseNode{line, col}, ai}, nil
		}
	}
	if p.peekAfterWS() == '=' && !p.peekLit("==") && !p.peekLit("=>") {
		if am, amerr := p.ParseAssignMutable(); amerr == nil {
			return &V17AssignOperNode{V17BaseNode{line, col}, am}, nil
		}
	}
	return nil, p.errAt("assign_oper: expected ':', ':~', or '='")
}

// V17ExtendScopeOperNode  extend_scope_oper = "+="
type V17ExtendScopeOperNode struct{ V17BaseNode }

// ParseExtendScopeOper parses extend_scope_oper = "+=".
func (p *V17Parser) ParseExtendScopeOper() (node *V17ExtendScopeOperNode, err error) {
	done := p.debugEnter("extend_scope_oper")
	defer func() { done(err == nil) }()
	if !p.peekLit("+=") {
		return nil, p.errAt("extend_scope_oper: expected '+='")
	}
	line, col := p.runeLine, p.runeCol
	if _, err = p.matchLit("+="); err != nil {
		return nil, err
	}
	return &V17ExtendScopeOperNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// SCOPE ASSIGN INLINE
// scope_assign_inline = "_"
// =============================================================================

// V17ScopeAssignInlineNode  scope_assign_inline = "_"
type V17ScopeAssignInlineNode struct{ V17BaseNode }

// ParseScopeAssignInline parses scope_assign_inline = "_".
func (p *V17Parser) ParseScopeAssignInline() (node *V17ScopeAssignInlineNode, err error) {
	done := p.debugEnter("scope_assign_inline")
	defer func() { done(err == nil) }()
	// "_" is a single-char ident — use matchRe for single underscore not followed by ident char
	if p.peekAfterWS() != '_' {
		return nil, p.errAt("scope_assign_inline: expected '_'")
	}
	line, col := p.runeLine, p.runeCol
	tok, terr := p.matchRe(reIdentScan)
	if terr != nil || tok.Value != "_" {
		return nil, p.errAt("scope_assign_inline: expected '_'")
	}
	return &V17ScopeAssignInlineNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// SCOPE INJECT
// scope_inject = group_begin
//                  assign_lhs assign_immutable
//                      ( ( ident_ref !WS! "." !WS! reflect_prefix !WS! "type" ) | any_type )
//                  { "," assign_lhs assign_immutable
//                      ( ( ident_ref !WS! "." !WS! reflect_prefix !WS! "type" ) | any_type ) }
//                group_end
// =============================================================================

// V17ScopeInjectItem is one binding in a scope_inject list.
type V17ScopeInjectItem struct {
	V17BaseNode
	Lhs      *V17AssignLhsNode
	TypeExpr interface{} // *V17IdentRefNode (with .@type suffix consumed) | *V17AnyTypeNode
}

// V17ScopeInjectNode  scope_inject = "(" assign_lhs ":" type_expr { "," ... } ")"
type V17ScopeInjectNode struct {
	V17BaseNode
	Items []V17ScopeInjectItem
}

// ParseScopeInject parses scope_inject = group_begin bindings group_end.
func (p *V17Parser) ParseScopeInject() (node *V17ScopeInjectNode, err error) {
	done := p.debugEnter("scope_inject")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, gerr := p.ParseGroupBegin(); gerr != nil {
		return nil, fmt.Errorf("scope_inject: %w", gerr)
	}

	first, ferr := p.parseScopeInjectItem()
	if ferr != nil {
		return nil, fmt.Errorf("scope_inject: %w", ferr)
	}
	items := []V17ScopeInjectItem{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.parseScopeInjectItem()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	if _, gerr := p.ParseGroupEnd(); gerr != nil {
		return nil, fmt.Errorf("scope_inject: expected ')': %w", gerr)
	}
	return &V17ScopeInjectNode{V17BaseNode{line, col}, items}, nil
}

// parseScopeInjectItem parses one item: assign_lhs assign_immutable
// ( ( ident_ref !WS! "." !WS! reflect_prefix !WS! "type" ) | any_type ).
func (p *V17Parser) parseScopeInjectItem() (V17ScopeInjectItem, error) {
	line, col := p.runeLine, p.runeCol

	lhs, lerr := p.ParseAssignLhs()
	if lerr != nil {
		return V17ScopeInjectItem{}, fmt.Errorf("scope_inject item: %w", lerr)
	}
	if _, cerr := p.ParseAssignImmutable(); cerr != nil {
		return V17ScopeInjectItem{}, fmt.Errorf("scope_inject item: expected ':': %w", cerr)
	}

	var typeExpr interface{}
	if p.peekLit("@?") {
		at, aterr := p.ParseAnyType()
		if aterr != nil {
			return V17ScopeInjectItem{}, fmt.Errorf("scope_inject item: %w", aterr)
		}
		typeExpr = at
	} else {
		// ident_ref !WS! ".@type"  — ParseTypeRef handles the full form
		tr, trerr := p.ParseTypeRef()
		if trerr != nil {
			return V17ScopeInjectItem{}, fmt.Errorf("scope_inject item: expected ident_ref.@type or @?: %w", trerr)
		}
		typeExpr = tr
	}

	return V17ScopeInjectItem{V17BaseNode{line, col}, lhs, typeExpr}, nil
}

// =============================================================================
// IMPORT ASSIGN
// import_assign = EOL [ private_modifier ] ( assign_lhs | scope_assign_inline )
//                 assign_immutable ( http_url | file_url ) { "," ( http_url | file_url ) }
// =============================================================================

// V17ImportAssignNode  import_assign = EOL [private_modifier] lhs ":" url { "," url }
type V17ImportAssignNode struct {
	V17BaseNode
	Modifier *V17PrivateModifierNode // nil when not private
	// LHS is one of: *V17AssignLhsNode | *V17ScopeAssignInlineNode
	LHS  interface{}
	URLs []*V17ConstantNode // http_url | file_url — parser captures as constant
}

// ParseImportAssign parses import_assign = EOL [private_modifier] lhs assign_immutable url { "," url }.
func (p *V17Parser) ParseImportAssign() (node *V17ImportAssignNode, err error) {
	done := p.debugEnter("import_assign")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, eolerr := p.ParseEol(); eolerr != nil {
		return nil, fmt.Errorf("import_assign: expected EOL: %w", eolerr)
	}

	// optional private_modifier "-"
	var mod *V17PrivateModifierNode
	if p.peekAfterWS() == '-' {
		if m, merr := p.ParsePrivateModifier(); merr == nil {
			mod = m
		}
	}

	// LHS: either scope_assign_inline ("_") or assign_lhs
	var lhs interface{}
	if saved := p.savePos(); true {
		if si, sierr := p.ParseScopeAssignInline(); sierr == nil {
			lhs = si
		} else {
			p.restorePos(saved)
			al, alerr := p.ParseAssignLhs()
			if alerr != nil {
				return nil, fmt.Errorf("import_assign: expected assign_lhs or '_': %w", alerr)
			}
			lhs = al
		}
	}

	if _, cerr := p.ParseAssignImmutable(); cerr != nil {
		return nil, fmt.Errorf("import_assign: expected ':': %w", cerr)
	}

	// First URL
	firstURL, furlerr := p.parseImportURL()
	if furlerr != nil {
		return nil, fmt.Errorf("import_assign: expected http_url or file_url: %w", furlerr)
	}
	urls := []*V17ConstantNode{firstURL}

	// Additional URLs
	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		u, uerr := p.parseImportURL()
		if uerr != nil {
			p.restorePos(saved)
			break
		}
		urls = append(urls, u)
	}

	return &V17ImportAssignNode{V17BaseNode{line, col}, mod, lhs, urls}, nil
}

// parseImportURL parses a single http_url or file_url as a V17ConstantNode.
// http_url and file_url are defined as regexps in spec/01_definitions.sqg and
// are parsed as string literals or constants at the lexer level.
func (p *V17Parser) parseImportURL() (*V17ConstantNode, error) {
	return p.ParseConstant()
}

// =============================================================================
// OTHER INLINE ASSIGN
// other_inline_assign = EOL [ private_modifier ] scope_assign_inline assign_oper assign_rhs
// =============================================================================

// V17OtherInlineAssignNode  other_inline_assign = EOL [private_modifier] "_" assign_oper assign_rhs
type V17OtherInlineAssignNode struct {
	V17BaseNode
	Modifier *V17PrivateModifierNode // nil when not private
	LHS      *V17ScopeAssignInlineNode
	Oper     *V17AssignOperNode
	Rhs      *V17AssignRhsNode
}

// ParseOtherInlineAssign parses other_inline_assign = EOL [private_modifier] "_" assign_oper assign_rhs.
// parseOtherInlineAssignBody parses the body of other_inline_assign after any
// leading EOL has already been consumed (or is optional, as inside scope_body).
func (p *V17Parser) parseOtherInlineAssignBody(line, col int) (*V17OtherInlineAssignNode, error) {
	var mod *V17PrivateModifierNode
	if p.peekAfterWS() == '-' {
		if m, merr := p.ParsePrivateModifier(); merr == nil {
			mod = m
		}
	}

	si, sierr := p.ParseScopeAssignInline()
	if sierr != nil {
		return nil, fmt.Errorf("other_inline_assign: expected '_': %w", sierr)
	}

	oper, oerr := p.ParseAssignOper()
	if oerr != nil {
		return nil, fmt.Errorf("other_inline_assign: %w", oerr)
	}

	rhs, rerr := p.ParseAssignRhs()
	if rerr != nil {
		return nil, fmt.Errorf("other_inline_assign: %w", rerr)
	}

	return &V17OtherInlineAssignNode{V17BaseNode{line, col}, mod, si, oper, rhs}, nil
}

func (p *V17Parser) ParseOtherInlineAssign() (node *V17OtherInlineAssignNode, err error) {
	done := p.debugEnter("other_inline_assign")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, eolerr := p.ParseEol(); eolerr != nil {
		return nil, fmt.Errorf("other_inline_assign: expected EOL: %w", eolerr)
	}

	return p.parseOtherInlineAssignBody(line, col)
}

// =============================================================================
// SCOPE BODY ITEM
// scope_body_item = import_assign | other_inline_assign | assignment
// EXTEND<scope_body_item> = | scope_final         (from this file, below)
// EXTEND<scope_body_item> = | func_scope_assign   (from this file, below)
//
// Combined dispatcher (most-specific first):
//   1. import_assign      — starts with EOL + '_'|lhs + ':' + url
//   2. other_inline_assign — starts with EOL + '_' + assign_oper
//   3. scope_final        — starts with '(' (scope_inject) | ident
//   4. func_scope_assign  — starts with '(' (func_inject) | ident + ':' + func_unit
//   5. assignment         — fallback
//
// NOTE: scope_final and func_scope_assign both start with an optional '(' group
// or an ident_name. Disambiguation relies on what follows the LHS:
//   scope_assign:     lhs ('=' | '+=') scope_body_or_empty
//   func_scope_assign: lhs ':' func_unit  (func_unit starts with '(')
//   import_assign:    must start with EOL
// =============================================================================

// V17ScopeBodyItemNode  scope_body_item (combined rule with EXTENDs)
type V17ScopeBodyItemNode struct {
	V17BaseNode
	// Value is one of:
	//   *V17ImportAssignNode | *V17OtherInlineAssignNode
	//   *V17ScopeFinalNode   | *V17FuncScopeAssignNode
	//   *V17AssignmentNode
	Value interface{}
}

// ParseScopeBodyItem parses scope_body_item (EXTEND included).
func (p *V17Parser) ParseScopeBodyItem() (node *V17ScopeBodyItemNode, err error) {
	done := p.debugEnter("scope_body_item")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// 1. import_assign / 2. other_inline_assign — both start with EOL
	ch := p.peekAfterWS()
	if ch == '\n' || ch == '\r' || ch == ';' {
		// other_inline_assign: EOL + '_'
		if saved := p.savePos(); true {
			if oi, oierr := p.ParseOtherInlineAssign(); oierr == nil {
				return &V17ScopeBodyItemNode{V17BaseNode{line, col}, oi}, nil
			}
			p.restorePos(saved)
		}
		// import_assign: EOL + optional'-' + lhs/_ + ':' + url
		if saved := p.savePos(); true {
			if ia, iaerr := p.ParseImportAssign(); iaerr == nil {
				return &V17ScopeBodyItemNode{V17BaseNode{line, col}, ia}, nil
			}
			p.restorePos(saved)
		}
	}
	// other_inline_assign when '_' is at cursor (EOL already consumed by scope_body's skipNL)
	if ch == '_' {
		if saved := p.savePos(); true {
			if oi, oierr := p.parseOtherInlineAssignBody(line, col); oierr == nil {
				return &V17ScopeBodyItemNode{V17BaseNode{line, col}, oi}, nil
			}
			p.restorePos(saved)
		}
	}

	// 3. scope_final (EXTEND): scope_inject or ident + '=' | "+="
	if saved := p.savePos(); true {
		if sf, sferr := p.ParseScopeFinal(); sferr == nil {
			return &V17ScopeBodyItemNode{V17BaseNode{line, col}, sf}, nil
		}
		p.restorePos(saved)
	}

	// 4. func_scope_assign (EXTEND): optional func_inject + ident ':' func_unit
	if saved := p.savePos(); true {
		if fsa, fsaerr := p.ParseFuncScopeAssign(); fsaerr == nil {
			return &V17ScopeBodyItemNode{V17BaseNode{line, col}, fsa}, nil
		}
		p.restorePos(saved)
	}

	// 5. assignment (fallback)
	if saved := p.savePos(); true {
		if a, aerr := p.ParseAssignment(); aerr == nil {
			return &V17ScopeBodyItemNode{V17BaseNode{line, col}, a}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("scope_body_item: expected import_assign, other_inline_assign, scope_final, func_scope_assign, or assignment")
}

// =============================================================================
// PRIVATE BLOCK
// private_block = private_modifier
//                 ( ( group_begin scope_body_item { ( EOL | "," ) scope_body_item } group_end )
//                 | ( scope_begin scope_body_item { ( EOL | "," ) scope_body_item } scope_end ) )
// =============================================================================

// V17PrivateBlockNode  private_block = "-" ( "(" items ")" | "{" items "}" )
type V17PrivateBlockNode struct {
	V17BaseNode
	Modifier *V17PrivateModifierNode
	Items    []*V17ScopeBodyItemNode
	// UseParens distinguishes "(…)" from "{…}" for round-trip fidelity
	UseParens bool
}

// ParsePrivateBlock parses private_block = private_modifier ( "(" bodies ")" | "{" bodies "}" )
func (p *V17Parser) ParsePrivateBlock() (node *V17PrivateBlockNode, err error) {
	done := p.debugEnter("private_block")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	mod, merr := p.ParsePrivateModifier()
	if merr != nil {
		return nil, fmt.Errorf("private_block: %w", merr)
	}

	var items []*V17ScopeBodyItemNode
	var useParens bool

	switch p.peekAfterWS() {
	case '(':
		useParens = true
		if _, err = p.matchLit("("); err != nil {
			return nil, err
		}
		items, err = p.parseScopeBodyItemList(V17_RPAREN)
		if err != nil {
			return nil, fmt.Errorf("private_block: %w", err)
		}
		if _, gerr := p.matchLit(")"); gerr != nil {
			return nil, fmt.Errorf("private_block: expected ')': %w", gerr)
		}
	case '{':
		if _, err = p.matchLit("{"); err != nil {
			return nil, err
		}
		items, err = p.parseScopeBodyItemList(V17_RBRACE)
		if err != nil {
			return nil, fmt.Errorf("private_block: %w", err)
		}
		if _, gerr := p.matchLit("}"); gerr != nil {
			return nil, fmt.Errorf("private_block: expected '}': %w", gerr)
		}
	default:
		return nil, p.errAt("private_block: expected '(' or '{' after '-'")
	}

	return &V17PrivateBlockNode{V17BaseNode{line, col}, mod, items, useParens}, nil
}

// parseScopeBodyItemList parses scope_body_item { ( EOL | "," ) scope_body_item } up to stopRune.
func (p *V17Parser) parseScopeBodyItemList(stopTok V17TokenType) ([]*V17ScopeBodyItemNode, error) {
	// Map legacy stopTok to rune for rune-stream check
	var stopRune rune
	switch stopTok {
	case V17_RPAREN:
		stopRune = ')'
	case V17_RBRACE:
		stopRune = '}'
	default:
		stopRune = 0
	}
	p.skipNLAndComments()
	var items []*V17ScopeBodyItemNode
	for {
		ch := p.peekAfterWS()
		if ch == stopRune || ch == 0 {
			break
		}
		item, ierr := p.ParseScopeBodyItem()
		if ierr != nil {
			return nil, ierr
		}
		items = append(items, item)
		p.skipNLAndComments()
		if p.peekAfterWS() == ',' {
			if _, err := p.matchLit(","); err == nil {
				p.skipNLAndComments()
			}
		}
	}
	return items, nil
}

// =============================================================================
// SCOPE BODY
// scope_body = scope_begin
//                ( scope_body_item | private_block ) { ( EOL | "," ) ( scope_body_item | private_block ) }
//              scope_end
// =============================================================================

// V17ScopeBodyNode  scope_body = "{" body_elem { sep body_elem } "}"
type V17ScopeBodyNode struct {
	V17BaseNode
	// Items contains *V17ScopeBodyItemNode | *V17PrivateBlockNode entries
	Items []interface{}
}

// ParseScopeBody parses scope_body = "{" items… "}".
func (p *V17Parser) ParseScopeBody() (node *V17ScopeBodyNode, err error) {
	done := p.debugEnter("scope_body")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, berr := p.matchLit("{"); berr != nil {
		return nil, fmt.Errorf("scope_body: expected '{': %w", berr)
	}
	p.skipNLAndComments()

	var items []interface{}
	for {
		ch := p.peekAfterWS()
		if ch == '}' || ch == 0 {
			break
		}
		item, ierr := p.parseScopeBodyOrPrivate()
		if ierr != nil {
			return nil, fmt.Errorf("scope_body: %w", ierr)
		}
		items = append(items, item)
		p.skipNLAndComments()
		if p.peekAfterWS() == ',' {
			if _, err := p.matchLit(","); err == nil {
				p.skipNLAndComments()
			}
		}
	}

	if _, eerr := p.matchLit("}"); eerr != nil {
		return nil, fmt.Errorf("scope_body: expected '}': %w", eerr)
	}
	return &V17ScopeBodyNode{V17BaseNode{line, col}, items}, nil
}

// parseScopeBodyOrPrivate tries private_block first (starts with "-"), then scope_body_item.
func (p *V17Parser) parseScopeBodyOrPrivate() (interface{}, error) {
	if p.peekAfterWS() == '-' {
		if saved := p.savePos(); true {
			if pb, pberr := p.ParsePrivateBlock(); pberr == nil {
				return pb, nil
			}
			p.restorePos(saved)
		}
	}
	return p.ParseScopeBodyItem()
}

// =============================================================================
// SCOPE ASSIGN / SCOPE MERGE TAIL / SCOPE FINAL
// scope_assign     = [ scope_inject ] assign_lhs ( assign_oper | extend_scope_oper )
//                    ( scope_body | empty_scope_decl )
// scope_merge_tail = ( TYPE_OF scope_assign<ident_ref> | scope_assign ) "+"
//                    ( TYPE_OF scope_body<ident_ref> | scope_body )
//                    { "+" ( TYPE_OF scope_body<ident_ref> | scope_body ) }
// scope_final      = scope_assign | scope_merge_tail
// =============================================================================

// V17ScopeAssignNode  scope_assign = [scope_inject] assign_lhs (assign_oper|extend_scope_oper) (scope_body|empty_scope_decl)
type V17ScopeAssignNode struct {
	V17BaseNode
	Inject *V17ScopeInjectNode // nil when absent
	LHS    *V17AssignLhsNode
	// Oper is one of: *V17AssignOperNode | *V17ExtendScopeOperNode
	Oper interface{}
	// Body is one of: *V17ScopeBodyNode | *V17EmptyScopeDeclNode
	Body interface{}
}

// ParseScopeAssign parses scope_assign = [scope_inject] assign_lhs (assign_oper|extend_scope_oper) body.
func (p *V17Parser) ParseScopeAssign() (node *V17ScopeAssignNode, err error) {
	done := p.debugEnter("scope_assign")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// optional scope_inject — starts with "(" but must NOT be a block comment "(*"
	var inject *V17ScopeInjectNode
	if p.peekAfterWS() == '(' && !p.peekLit("(*") {
		if saved := p.savePos(); true {
			if si, sierr := p.ParseScopeInject(); sierr == nil {
				inject = si
			} else {
				p.restorePos(saved)
			}
		}
	}

	lhs, lerr := p.ParseAssignLhs()
	if lerr != nil {
		return nil, fmt.Errorf("scope_assign: %w", lerr)
	}

	// oper: extend_scope_oper ("+=") | assign_oper (":" | ":~" | "=")
	var oper interface{}
	if p.peekLit("+=") {
		eso, esoerr := p.ParseExtendScopeOper()
		if esoerr != nil {
			return nil, fmt.Errorf("scope_assign: %w", esoerr)
		}
		oper = eso
	} else {
		ao, aoerr := p.ParseAssignOper()
		if aoerr != nil {
			return nil, fmt.Errorf("scope_assign: expected assign_oper or '+=': %w", aoerr)
		}
		oper = ao
	}

	// body: scope_body ("{") | empty_scope_decl ("{}")
	var body interface{}
	if saved := p.savePos(); true {
		if sd, sderr := p.ParseEmptyScopeDecl(); sderr == nil {
			body = sd
		} else {
			p.restorePos(saved)
		}
	}
	if body == nil {
		sb, sberr := p.ParseScopeBody()
		if sberr != nil {
			return nil, fmt.Errorf("scope_assign: expected scope_body or empty_scope_decl: %w", sberr)
		}
		body = sb
	}

	return &V17ScopeAssignNode{V17BaseNode{line, col}, inject, lhs, oper, body}, nil
}

// V17ScopeMergeTailSegment is one "+" tail in scope_merge_tail.
type V17ScopeMergeTailSegment struct {
	// Body is one of: *V17ScopeBodyNode | *V17IdentRefNode (TYPE_OF scope_body<ident_ref>)
	Body interface{}
}

// V17ScopeMergeTailNode  scope_merge_tail = (base | ident_ref) "+" body { "+" body }
type V17ScopeMergeTailNode struct {
	V17BaseNode
	// Base is one of: *V17ScopeAssignNode | *V17IdentRefNode (TYPE_OF scope_assign<ident_ref>)
	Base     interface{}
	Segments []V17ScopeMergeTailSegment // at least one
}

// ParseScopeMergeTail parses scope_merge_tail.
func (p *V17Parser) ParseScopeMergeTail() (node *V17ScopeMergeTailNode, err error) {
	done := p.debugEnter("scope_merge_tail")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Base: TYPE_OF scope_assign<ident_ref> | scope_assign
	var base interface{}

	// Try ident_ref first (it is a prefix of scope_assign which requires lhs + oper + body)
	if saved := p.savePos(); true {
		if ref, rerr := p.ParseIdentRef(); rerr == nil {
			// Only accept as bare ident_ref base if "+" follows (otherwise fall through to scope_assign)
			if p.peekAfterWS() == '+' && !p.peekLit("+=") {
				base = ref
			} else {
				p.restorePos(saved)
			}
		} else {
			p.restorePos(saved)
		}
	}
	if base == nil {
		sa, saerr := p.ParseScopeAssign()
		if saerr != nil {
			return nil, fmt.Errorf("scope_merge_tail: expected scope_assign or ident_ref: %w", saerr)
		}
		base = sa
	}

	// Require at least one "+" segment
	if p.peekAfterWS() != '+' || p.peekLit("+=") {
		return nil, p.errAt("scope_merge_tail: expected '+' after base")
	}

	var segments []V17ScopeMergeTailSegment
	for p.peekAfterWS() == '+' && !p.peekLit("+=") {
		if _, err = p.matchLit("+"); err != nil {
			break
		}
		seg, segerr := p.parseScopeMergeBody()
		if segerr != nil {
			return nil, fmt.Errorf("scope_merge_tail: expected scope_body or ident_ref after '+': %w", segerr)
		}
		segments = append(segments, V17ScopeMergeTailSegment{seg})
	}

	return &V17ScopeMergeTailNode{V17BaseNode{line, col}, base, segments}, nil
}

// parseScopeMergeBody parses TYPE_OF scope_body<ident_ref> | scope_body.
func (p *V17Parser) parseScopeMergeBody() (interface{}, error) {
	// Try ident_ref (TYPE_OF scope_body<ident_ref>); fall back to scope_body.
	if saved := p.savePos(); true {
		if ref, rerr := p.ParseIdentRef(); rerr == nil {
			return ref, nil
		}
		p.restorePos(saved)
	}
	return p.ParseScopeBody()
}

// V17ScopeFinalNode  scope_final = scope_assign | scope_merge_tail
type V17ScopeFinalNode struct {
	V17BaseNode
	// Value is one of: *V17ScopeAssignNode | *V17ScopeMergeTailNode
	Value interface{}
}

// ParseScopeFinal parses scope_final = scope_assign | scope_merge_tail.
// scope_merge_tail requires "+" after the base, so we try scope_merge_tail first
// (it will fail fast if no "+" follows the base).
func (p *V17Parser) ParseScopeFinal() (node *V17ScopeFinalNode, err error) {
	done := p.debugEnter("scope_final")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Try scope_merge_tail first — it subsumes scope_assign as its base
	if saved := p.savePos(); true {
		if smt, smterr := p.ParseScopeMergeTail(); smterr == nil {
			return &V17ScopeFinalNode{V17BaseNode{line, col}, smt}, nil
		}
		p.restorePos(saved)
	}

	sa, saerr := p.ParseScopeAssign()
	if saerr != nil {
		return nil, fmt.Errorf("scope_final: %w", saerr)
	}
	return &V17ScopeFinalNode{V17BaseNode{line, col}, sa}, nil
}

// =============================================================================
// PARSER ROOT
// parser_root = import_assign | scope_final | func_scope_assign
// =============================================================================

// V17ParserRootNode  parser_root = import_assign | scope_final | func_scope_assign
type V17ParserRootNode struct {
	V17BaseNode
	// Value is one of: *V17ImportAssignNode | *V17ScopeFinalNode | *V17FuncScopeAssignNode
	Value interface{}
}

// ParseParserRoot parses parser_root = import_assign | scope_final | func_scope_assign.
// This is the grammar entry point.
func (p *V17Parser) ParseParserRoot() (node *V17ParserRootNode, err error) {
	done := p.debugEnter("parser_root")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	p.skipNL()

	// At file start (BOF), a leading block comment is conventional in .sqz files.
	// Consume it now so the grammar rules beneath can see the actual construct.
	if p.atBOF && p.peekLit("(*") {
		saved := p.savePos()
		if _, cerr := p.ParseComment(); cerr == nil {
			p.skipNL()
		} else {
			p.restorePos(saved)
		}
	}

	// import_assign starts with EOL; BOF counts as an implicit EOL
	if rch := p.peekAfterWS(); p.atBOF || rch == '\n' || rch == '\r' || rch == ';' {
		if saved := p.savePos(); true {
			if ia, iaerr := p.ParseImportAssign(); iaerr == nil {
				return &V17ParserRootNode{V17BaseNode{line, col}, ia}, nil
			}
			p.restorePos(saved)
		}
	}

	// func_scope_assign — tried before scope_final because both can start with an
	// ident_ref, but func_scope_assign requires a trailing func_unit and is more
	// specific than scope_final.
	if saved := p.savePos(); true {
		if fsa, fsaerr := p.ParseFuncScopeAssign(); fsaerr == nil {
			return &V17ParserRootNode{V17BaseNode{line, col}, fsa}, nil
		}
		p.restorePos(saved)
	}

	sf, sferr := p.ParseScopeFinal()
	if sferr != nil {
		return nil, fmt.Errorf("parser_root: expected import_assign, scope_final or func_scope_assign: %w", sferr)
	}
	return &V17ParserRootNode{V17BaseNode{line, col}, sf}, nil
}

// =============================================================================
// FUNC INJECT
// func_inject = group_begin
//                 ( assign_lhs assign_immutable inspect_type [ empty_array_decl ]
//                 | assign_lhs assign_immutable ident_ref )
//                 { "," ( assign_lhs assign_immutable inspect_type [ empty_array_decl ]
//                       | assign_lhs assign_immutable ident_ref ) }
//               group_end
// =============================================================================

// V17FuncInjectItem is one binding in a func_inject list.
type V17FuncInjectItem struct {
	V17BaseNode
	LHS *V17AssignLhsNode
	// Value is one of: *V17InspectTypeNode (possibly + EmptyArrayDecl) | *V17IdentRefNode
	Value          interface{}
	EmptyArrayDecl *V17EmptyArrayDeclNode // non-nil only when Value is *V17InspectTypeNode
}

// V17FuncInjectNode  func_inject = "(" items ")"
type V17FuncInjectNode struct {
	V17BaseNode
	Items []V17FuncInjectItem
}

// ParseFuncInject parses func_inject = group_begin items group_end.
func (p *V17Parser) ParseFuncInject() (node *V17FuncInjectNode, err error) {
	done := p.debugEnter("func_inject")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, gerr := p.ParseGroupBegin(); gerr != nil {
		return nil, fmt.Errorf("func_inject: %w", gerr)
	}

	first, ferr := p.parseFuncInjectItem()
	if ferr != nil {
		return nil, fmt.Errorf("func_inject: %w", ferr)
	}
	items := []V17FuncInjectItem{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.parseFuncInjectItem()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	if _, gerr := p.ParseGroupEnd(); gerr != nil {
		return nil, fmt.Errorf("func_inject: expected ')': %w", gerr)
	}
	return &V17FuncInjectNode{V17BaseNode{line, col}, items}, nil
}

// parseFuncInjectItem parses one item: assign_lhs ":" ( inspect_type [empty_array_decl] | ident_ref ).
func (p *V17Parser) parseFuncInjectItem() (V17FuncInjectItem, error) {
	line, col := p.runeLine, p.runeCol

	lhs, lerr := p.ParseAssignLhs()
	if lerr != nil {
		return V17FuncInjectItem{}, fmt.Errorf("func_inject item: %w", lerr)
	}
	if _, cerr := p.ParseAssignImmutable(); cerr != nil {
		return V17FuncInjectItem{}, fmt.Errorf("func_inject item: expected ':': %w", cerr)
	}

	// Try inspect_type first (starts with "@" or "@?")
	if saved := p.savePos(); true {
		if it, iterr := p.ParseInspectType(); iterr == nil {
			var ead *V17EmptyArrayDeclNode
			if p.peekAfterWS() == '[' {
				if saved2 := p.savePos(); true {
					if e, eerr := p.ParseEmptyArrayDecl(); eerr == nil {
						ead = e
					} else {
						p.restorePos(saved2)
					}
				}
			}
			return V17FuncInjectItem{V17BaseNode{line, col}, lhs, it, ead}, nil
		}
		p.restorePos(saved)
	}

	// Fall back to ident_ref
	ref, rerr := p.ParseIdentRef()
	if rerr != nil {
		return V17FuncInjectItem{}, fmt.Errorf("func_inject item: expected inspect_type or ident_ref: %w", rerr)
	}
	return V17FuncInjectItem{V17BaseNode{line, col}, lhs, ref, nil}, nil
}

// =============================================================================
// FUNC SCOPE ASSIGN
// func_scope_assign = [ func_inject ] assign_lhs assign_immutable func_unit
// =============================================================================

// V17FuncScopeAssignNode  func_scope_assign = [func_inject] assign_lhs ":" ( func_unit | return_func_unit )
type V17FuncScopeAssignNode struct {
	V17BaseNode
	Inject *V17FuncInjectNode // nil when absent
	LHS    *V17AssignLhsNode
	// FUnit is one of: *V17FuncUnitNode | *V17ReturnFuncUnitNode
	FUnit interface{}
}

// ParseFuncScopeAssign parses func_scope_assign = [func_inject] assign_lhs ":" func_unit.
func (p *V17Parser) ParseFuncScopeAssign() (node *V17FuncScopeAssignNode, err error) {
	done := p.debugEnter("func_scope_assign")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// optional func_inject — starts with "(" but we must distinguish it from
	// a bare func_unit (which also starts with "("). func_inject is "(ident :"
	// whereas func_unit header starts with "(" then func_args_decl.
	var inject *V17FuncInjectNode
	if p.peekAfterWS() == '(' {
		if saved := p.savePos(); true {
			if fi, fierr := p.ParseFuncInject(); fierr == nil {
				inject = fi
			} else {
				p.restorePos(saved)
			}
		}
	}

	lhs, lerr := p.ParseAssignLhs()
	if lerr != nil {
		return nil, fmt.Errorf("func_scope_assign: %w", lerr)
	}

	if _, cerr := p.ParseAssignImmutable(); cerr != nil {
		return nil, fmt.Errorf("func_scope_assign: expected ':': %w", cerr)
	}

	// Accept return_func_unit ("<-" func_unit) or plain func_unit.
	var fUnit interface{}
	if p.peekLit("<-") {
		rfu, rfuerr := p.ParseReturnFuncUnit()
		if rfuerr != nil {
			return nil, fmt.Errorf("func_scope_assign: expected return_func_unit: %w", rfuerr)
		}
		fUnit = rfu
	} else {
		fu, fuerr := p.ParseFuncUnit()
		if fuerr != nil {
			return nil, fmt.Errorf("func_scope_assign: expected func_unit or return_func_unit: %w", fuerr)
		}
		fUnit = fu
	}

	return &V17FuncScopeAssignNode{V17BaseNode{line, col}, inject, lhs, fUnit}, nil
}

// Ensure fmt is used.
var _ = fmt.Sprintf
