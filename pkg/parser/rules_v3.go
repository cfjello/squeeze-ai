// rules_v3.go — Recursive-descent parsing functions for the V3 grammar.
//
// Rule names map 1-to-1 to the EBNF in spec/squeeze_v3.ebnf.txt.
// V1 rules (from rules_v1.go) are composed freely here.
//
// Convention — same as rules_v1.go:
//   - Returns *Node on success, nil if this rule does not apply.
//   - Position is never advanced on a nil return
//     (save/restore via p.try() handles backtracking).
//   - Grammar directives (UNIQUE, TYPE_OF, …) produce NodeDirective* wrappers
//     for Phase-3 processing.
package parser

// =============================================================================
// Empty declarators
// =============================================================================

// parseEmptyArrayDecl parses "[]".
//   empty_array_decl = "[]";
func (p *Parser) parseEmptyArrayDecl() *Node {
	if !p.match(TOK_EMPTY_ARR) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeEmptyArrayDecl, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseFuncStreamDecl parses ">>".
//   func_stream_decl = ">>";
func (p *Parser) parseFuncStreamDecl() *Node {
	if !p.match(TOK_STREAM) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeFuncStreamDecl, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseFuncRegexpDecl parses "//".
//   func_regexp_decl = "//";
func (p *Parser) parseFuncRegexpDecl() *Node {
	if !p.match(TOK_REGEXP_DECL) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeFuncRegexpDecl, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseEmptyObjectDecl parses "{}".
//   empty_object_decl = "{}";
func (p *Parser) parseEmptyObjectDecl() *Node {
	if !p.match(TOK_EMPTY_OBJ) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeEmptyObjectDecl, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseFuncStringDecl parses an empty string declarator: "" | '' | ``.
//   func_string_decl = "\"\"" | "''" | "``";
func (p *Parser) parseFuncStringDecl() *Node {
	if !p.matchAny(TOK_EMPTY_STR_D, TOK_EMPTY_STR_S, TOK_EMPTY_STR_T) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeFuncStringDecl, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseEmptyDecl parses any empty declarator.
//   empty_decl = empty_array_decl | func_stream_decl | func_regexp_decl
//              | empty_object_decl | func_string_decl;
func (p *Parser) parseEmptyDecl() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_EMPTY_ARR:
		child = p.parseEmptyArrayDecl()
	case TOK_STREAM:
		child = p.parseFuncStreamDecl()
	case TOK_REGEXP_DECL:
		child = p.parseFuncRegexpDecl()
	case TOK_EMPTY_OBJ:
		child = p.parseEmptyObjectDecl()
	case TOK_EMPTY_STR_D, TOK_EMPTY_STR_S, TOK_EMPTY_STR_T:
		child = p.parseFuncStringDecl()
	default:
		return nil
	}
	return NewNode(NodeEmptyDecl, pos, child)
}

// =============================================================================
// calc_unit
// =============================================================================

// parseCalcUnit parses a calculation unit: numeric, string or logic expression.
//   calc_unit = numeric_expr | string_expr | logic_expr;
func (p *Parser) parseCalcUnit() *Node {
	if !p.enter("calc_unit") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()

	// Try from most-specific to most-general.
	if n := p.try(p.parseStringExpr); n != nil {
		return NewNode(NodeCalcUnit, pos, n)
	}
	if n := p.try(p.parseNumericExpr); n != nil {
		return NewNode(NodeCalcUnit, pos, n)
	}
	if n := p.try(p.parseLogicExpr); n != nil {
		return NewNode(NodeCalcUnit, pos, n)
	}
	return nil
}

// =============================================================================
// assign_lhs
// =============================================================================

// parseAssignLHS parses a UNIQUE-guarded list of identifier names with
// optional range entries separated by commas.
//   assign_lhs = UNIQUE<ident_name { "," ( ident_name | range ) }>;
func (p *Parser) parseAssignLHS() *Node {
	pos := p.curPos()
	first := p.parseIdentName()
	if first == nil {
		return nil
	}
	inner := NewNode(NodeAssignLHS, pos, first)
	for p.match(TOK_COMMA) {
		c := p.save()
		p.advance() // ","
		if r := p.try(p.parseRange); r != nil {
			inner.Append(r)
		} else if name := p.parseIdentName(); name != nil {
			inner.Append(name)
		} else {
			p.restore(c)
			break
		}
	}
	return NewDirectiveNode(NodeDirectiveUnique, "", pos, inner)
}

// =============================================================================
// Assignment RHS helpers
// =============================================================================

// parseAssignRHS4Object parses the RHS options allowed inside an object literal.
//   assign_rhs_4_object = constant | regexp | range | ident_ref | calc_unit | array_list;
func (p *Parser) parseAssignRHS4Object() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_REGEXP:
		child = p.parseRegexp()
	case TOK_INTEGER:
		// Could be a range (1..N) or a constant.
		if p.peek2().Type == TOK_DOTDOT {
			child = p.try(p.parseRange)
		}
		if child == nil {
			child = p.parseConstant()
		}
	case TOK_DECIMAL, TOK_STRING, TOK_TRUE, TOK_FALSE:
		child = p.parseConstant()
	case TOK_PLUS, TOK_MINUS:
		child = p.parseConstant()
	case TOK_EMPTY_ARR, TOK_LBRACKET:
		child = p.parseArrayList()
	case TOK_EMPTY_OBJ:
		child = p.parseEmptyObjectDecl()
	case TOK_IDENT:
		// ident_ref or calc_unit — calc_unit subsumes ident_ref in logic/numeric
		// contexts, so we try calc_unit first via backtracking, falling back to
		// a plain ident_ref.
		if n := p.try(p.parseCalcUnit); n != nil {
			child = n
		} else {
			child = p.parseIdentRef()
		}
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeAssignRHS4Object, pos, child)
}

// parseAssignRHS parses the full set of RHS values allowed in a func_header_assign.
//   assign_rhs = constant | func_string_decl | (regexp | func_regexp_decl)
//              | ident_ref | calc_unit | object_list | array_list;
func (p *Parser) parseAssignRHS() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_EMPTY_STR_D, TOK_EMPTY_STR_S, TOK_EMPTY_STR_T:
		child = p.parseFuncStringDecl()
	case TOK_REGEXP_DECL:
		child = p.parseFuncRegexpDecl()
	case TOK_REGEXP:
		child = p.parseRegexp()
	case TOK_EMPTY_ARR, TOK_LBRACKET:
		child = p.parseArrayList()
	case TOK_LBRACE:
		child = p.parseObjectList()
	case TOK_EMPTY_OBJ:
		child = p.parseEmptyObjectDecl()
	case TOK_INTEGER, TOK_DECIMAL, TOK_STRING, TOK_TRUE, TOK_FALSE:
		child = p.parseConstant()
	case TOK_PLUS, TOK_MINUS:
		child = p.parseConstant()
	case TOK_IDENT:
		if n := p.try(p.parseCalcUnit); n != nil {
			child = n
		} else {
			child = p.parseIdentRef()
		}
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeAssignRHS, pos, child)
}

// =============================================================================
// Arrays
// =============================================================================

// parseArrayValues parses a single element within an array initialiser.
//   array_values = constant | regexp | range | ident_ref | calc_unit | object_list;
func (p *Parser) parseArrayValues() *Node {
	if !p.enter("array_values") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_REGEXP:
		child = p.parseRegexp()
	case TOK_INTEGER:
		if p.peek2().Type == TOK_DOTDOT {
			child = p.try(p.parseRange)
		}
		if child == nil {
			child = p.parseConstant()
		}
	case TOK_DECIMAL, TOK_STRING, TOK_TRUE, TOK_FALSE:
		child = p.parseConstant()
	case TOK_PLUS, TOK_MINUS:
		child = p.parseConstant()
	case TOK_LBRACE:
		child = p.parseObjectList()
	case TOK_EMPTY_OBJ:
		child = p.parseEmptyObjectDecl()
	case TOK_IDENT:
		if n := p.try(p.parseCalcUnit); n != nil {
			child = n
		} else {
			child = p.parseIdentRef()
		}
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeArrayValues, pos, child)
}

// parseArrayInit parses a non-empty array literal "[" … "]".
//   array_init = "[" ( array_values | array_list )
//                    { "," ( array_values | array_list ) } [","] "]";
func (p *Parser) parseArrayInit() *Node {
	if !p.enter("array_init") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	if !p.match(TOK_LBRACKET) {
		return nil
	}
	pos := p.curPos()
	n := NewNode(NodeArrayInit, pos, NewTokenNode(p.advance())) // "["

	// parse first element
	first := p.parseArrayElementEntry()
	if first == nil {
		// empty array with explicit brackets — tolerate and close
		p.consume(TOK_RBRACKET)
		return n
	}
	n.Append(first)
	for p.match(TOK_COMMA) {
		commaPos := p.pos
		p.advance() // ","
		// trailing comma is allowed
		if p.match(TOK_RBRACKET) {
			break
		}
		elem := p.parseArrayElementEntry()
		if elem == nil {
			p.pos = commaPos // undo comma consumption
			break
		}
		n.Append(elem)
	}
	n.Append(NewTokenNode(p.advance())) // "]"  (consume even on soft error)
	return n
}

// parseArrayElementEntry parses array_values or array_list (the two element
// alternatives in array_init / array_list).
func (p *Parser) parseArrayElementEntry() *Node {
	if p.matchAny(TOK_LBRACKET, TOK_EMPTY_ARR) {
		return p.parseArrayList()
	}
	return p.parseArrayValues()
}

// parseArrayList parses an array literal or an empty "[]" declarator.
//   array_list = array_init | empty_array_decl;
func (p *Parser) parseArrayList() *Node {
	if !p.enter("array_list") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	var child *Node
	if p.match(TOK_EMPTY_ARR) {
		child = p.parseEmptyArrayDecl()
	} else {
		child = p.parseArrayInit()
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeArrayList, pos, child)
}

// parseArrayLookup parses ident_ref "[" integer "]".
//   array_lookup = ident_ref "[" integer "]";
func (p *Parser) parseArrayLookup() *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	c := p.save()
	pos := p.curPos()
	ref := p.parseIdentRef()
	if ref == nil || !p.match(TOK_LBRACKET) {
		p.restore(c)
		return nil
	}
	lbTok := p.advance() // "["
	idx := p.parseInteger()
	if idx == nil {
		p.restore(c)
		return nil
	}
	if !p.match(TOK_RBRACKET) {
		p.restore(c)
		return nil
	}
	rbTok := p.advance() // "]"
	return NewNode(NodeArrayLookup, pos, ref,
		NewTokenNode(lbTok), idx, NewTokenNode(rbTok))
}

// =============================================================================
// Objects
// =============================================================================

// parseObjectInit parses a brace-delimited object literal.
//   object_init = "{" assign_lhs assign_oper ( assign_rhs_4_object | object_list )
//                     { "," assign_lhs assign_oper ( assign_rhs_4_object | object_list ) }
//                     [","] "}";
func (p *Parser) parseObjectInit() *Node {
	if !p.enter("object_init") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	if !p.match(TOK_LBRACE) {
		return nil
	}
	pos := p.curPos()
	n := NewNode(NodeObjectInit, pos, NewTokenNode(p.advance())) // "{"

	// First field
	if entry := p.parseObjectEntry(); entry != nil {
		n.Append(entry)
		for p.match(TOK_COMMA) {
			commaPos := p.pos
			p.advance()
			if p.match(TOK_RBRACE) {
				break // trailing comma
			}
			entry = p.parseObjectEntry()
			if entry == nil {
				p.pos = commaPos
				break
			}
			n.Append(entry)
		}
	}
	p.consume(TOK_RBRACE)
	return n
}

// parseObjectEntry parses one key:oper:value triplet inside an object.
func (p *Parser) parseObjectEntry() *Node {
	lhs := p.parseAssignLHS()
	if lhs == nil {
		return nil
	}
	oper := p.parseAssignOper()
	if oper == nil {
		p.errorf("expected assignment operator after object key")
		return nil
	}
	// RHS: object_list takes priority when '{' follows, otherwise assign_rhs_4_object.
	var rhs *Node
	if p.matchAny(TOK_LBRACE, TOK_EMPTY_OBJ) {
		rhs = p.parseObjectList()
	} else {
		rhs = p.parseAssignRHS4Object()
	}
	if rhs == nil {
		p.errorf("expected value after object assignment operator")
		return nil
	}
	pos := lhs.Pos
	return NewNode(NodeObjectInit, pos, lhs, oper, rhs)
}

// parseObjectList parses an object literal or the empty "{}" declarator.
//   object_list = object_init | empty_object_decl;
func (p *Parser) parseObjectList() *Node {
	if !p.enter("object_list") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	var child *Node
	if p.match(TOK_EMPTY_OBJ) {
		child = p.parseEmptyObjectDecl()
	} else {
		child = p.parseObjectInit()
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeObjectList, pos, child)
}

// parseObjectLookup parses ident_ref "[" ident_name "]".
//   object_lookup = ident_ref "[" ident_name "]";
func (p *Parser) parseObjectLookup() *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	c := p.save()
	pos := p.curPos()
	ref := p.parseIdentRef()
	if ref == nil || !p.match(TOK_LBRACKET) {
		p.restore(c)
		return nil
	}
	lbTok := p.advance()
	key := p.parseIdentName()
	if key == nil {
		p.restore(c)
		return nil
	}
	if !p.match(TOK_RBRACKET) {
		p.restore(c)
		return nil
	}
	rbTok := p.advance()
	return NewNode(NodeObjectLookup, pos, ref,
		NewTokenNode(lbTok), key, NewTokenNode(rbTok))
}

// =============================================================================
// Tables
// =============================================================================

// parseTableHeader parses a UNIFORM string<array_init> directive.
//   table_header = UNIFORM string<array_init>;
func (p *Parser) parseTableHeader() *Node {
	if !p.match(TOK_UNIFORM) {
		return nil
	}
	pos := p.curPos()
	p.advance() // consume UNIFORM
	inner := p.parseDirectiveBrackets(func() *Node {
		return p.parseArrayInit()
	})
	if inner == nil {
		return nil
	}
	return NewNode(NodeTableHeader, pos,
		NewDirectiveNode(NodeDirectiveUniform, "string", pos, inner))
}

// parseTableEntry parses a TYPE_OF INFER<…> guarded row entry.
//   table_entry = TYPE_OF INFER<( assign_rhs_4_object | object_list )>;
func (p *Parser) parseTableEntry() *Node {
	if !p.match(TOK_TYPE_OF) {
		return nil
	}
	pos := p.curPos()
	p.advance() // consume TYPE_OF
	if !p.match(TOK_INFER) {
		p.errorf("expected INFER after TYPE_OF in table_entry")
		return nil
	}
	p.advance() // consume INFER
	inner := p.parseDirectiveBrackets(func() *Node {
		if p.matchAny(TOK_LBRACE, TOK_EMPTY_OBJ) {
			return p.parseObjectList()
		}
		return p.parseAssignRHS4Object()
	})
	if inner == nil {
		return nil
	}
	inferNode := NewDirectiveNode(NodeDirectiveInfer, "", pos, inner)
	wrapped := NewDirectiveNode(NodeDirectiveTypeOf, "INFER", pos, inferNode)
	return NewNode(NodeTableEntry, pos, wrapped)
}

// parseTableObjects parses the row list of a table.
//   table_objects = table_entry { "," table_entry } EOL;
func (p *Parser) parseTableObjects() *Node {
	first := p.parseTableEntry()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeTableObjects, pos, first)
	for p.match(TOK_COMMA) {
		c := p.save()
		p.advance()
		entry := p.parseTableEntry()
		if entry == nil {
			p.restore(c)
			break
		}
		n.Append(entry)
	}
	if eol := p.parseEOL(); eol != nil {
		n.Append(eol)
	}
	return n
}

// parseTableInit parses a complete table definition.
//   table_init = table_header table_objects;
func (p *Parser) parseTableInit() *Node {
	c := p.save()
	pos := p.curPos()
	header := p.parseTableHeader()
	if header == nil {
		return nil
	}
	objects := p.parseTableObjects()
	if objects == nil {
		p.restore(c)
		return nil
	}
	return NewNode(NodeTableInit, pos, header, objects)
}

// =============================================================================
// Reflection getters
// =============================================================================

// parseGetName parses   ident_ref "." "@name".
//   get_name = ident_ref "." "@name";
func (p *Parser) parseGetName() *Node {
	return p.parseAtGetter(NodeGetName, "@name")
}

// parseGetData parses   ident_ref "." "@data".
func (p *Parser) parseGetData() *Node {
	return p.parseAtGetter(NodeGetData, "@data")
}

// parseGetStoreName parses   ident_ref "." "@storeName".
func (p *Parser) parseGetStoreName() *Node {
	return p.parseAtGetter(NodeGetStoreName, "@storeName")
}

// parseGetType parses   ident_ref "." "@type".
func (p *Parser) parseGetType() *Node {
	return p.parseAtGetter(NodeGetType, "@type")
}

// parseGetTypeName parses   get_type "." "@name".
//   get_type_name = get_type "." "@name";
func (p *Parser) parseGetTypeName() *Node {
	c := p.save()
	pos := p.curPos()
	gt := p.parseGetType()
	if gt == nil {
		return nil
	}
	if !p.match(TOK_DOT) {
		p.restore(c)
		return nil
	}
	p.advance()
	if !p.matchValue(TOK_AT_IDENT, "@name") {
		p.restore(c)
		return nil
	}
	nameTok := p.advance()
	return NewNode(NodeGetTypeName, pos, gt, NewTokenNode(nameTok))
}

// parseAtGetter is a helper for the simple "ident_ref . @atName" pattern.
func (p *Parser) parseAtGetter(kind NodeKind, atName string) *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	c := p.save()
	pos := p.curPos()
	ref := p.parseIdentRef()
	if ref == nil || !p.match(TOK_DOT) {
		p.restore(c)
		return nil
	}
	p.advance() // "."
	if !p.matchValue(TOK_AT_IDENT, atName) {
		p.restore(c)
		return nil
	}
	atTok := p.advance()
	return NewNode(kind, pos, ref, NewTokenNode(atTok))
}

// =============================================================================
// Built-in header params
// =============================================================================

// parseDataAssign parses "@data" ":" ( object_list | get_data ).
//   data_assign = "@data" assign_immutable ( object_list | get_data );
func (p *Parser) parseDataAssign() *Node {
	if !p.matchValue(TOK_AT_IDENT, "@data") {
		return nil
	}
	pos := p.curPos()
	p.advance() // "@data"
	colon := p.parseAssignImmutable()
	if colon == nil {
		p.errorf("expected ':' after @data")
		return nil
	}
	var rhs *Node
	if p.matchAny(TOK_LBRACE, TOK_EMPTY_OBJ) {
		rhs = p.parseObjectList()
	} else {
		rhs = p.try(p.parseGetData)
	}
	if rhs == nil {
		p.errorf("expected object_list or get_data after @data :")
		return nil
	}
	return NewNode(NodeDataAssign, pos, colon, rhs)
}

// parseStoreNameAssign parses "@storeName" "~" string.
//   store_name_assign = "@storeName" assign_mutable string;
func (p *Parser) parseStoreNameAssign() *Node {
	if !p.matchValue(TOK_AT_IDENT, "@storeName") {
		return nil
	}
	pos := p.curPos()
	p.advance() // "@storeName"
	tilde := p.parseAssignMutable()
	if tilde == nil {
		p.errorf("expected '~' after @storeName")
		return nil
	}
	str := p.parseString()
	if str == nil {
		p.errorf("expected string after @storeName ~")
		return nil
	}
	return NewNode(NodeStoreNameAssign, pos, tilde, str)
}

// parseFuncHeaderBuildinParams parses a builtin header param.
//   func_header_buildin_params = store_name_assign | data_assign;
func (p *Parser) parseFuncHeaderBuildinParams() *Node {
	pos := p.curPos()
	var child *Node
	if p.matchValue(TOK_AT_IDENT, "@storeName") {
		child = p.parseStoreNameAssign()
	} else if p.matchValue(TOK_AT_IDENT, "@data") {
		child = p.parseDataAssign()
	} else {
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeFuncHeaderBuildinParams, pos, child)
}

// =============================================================================
// func_header_assign / func_header_user_params
// =============================================================================

// parseFuncHeaderAssign parses one assignment line in a function header.
//   func_header_assign = assign_lhs assign_oper assign_rhs | func_header_buildin_params;
func (p *Parser) parseFuncHeaderAssign() *Node {
	pos := p.curPos()
	// Prefer builtin params when the current token is an @-ident.
	if p.match(TOK_AT_IDENT) {
		if n := p.try(p.parseFuncHeaderBuildinParams); n != nil {
			return NewNode(NodeFuncHeaderAssign, pos, n)
		}
	}
	// Regular: assign_lhs assign_oper assign_rhs.
	lhs := p.parseAssignLHS()
	if lhs == nil {
		return nil
	}
	oper := p.parseAssignOper()
	if oper == nil {
		p.errorf("expected assignment operator in func_header_assign")
		return nil
	}
	rhs := p.parseAssignRHS()
	if rhs == nil {
		p.errorf("expected RHS value in func_header_assign")
		return nil
	}
	return NewNode(NodeFuncHeaderAssign, pos, lhs, oper, rhs)
}

// parseFuncHeaderUserParams parses user-defined parameters in a function header.
//   func_header_user_params = func_header_assign { EOL func_header_assign };
func (p *Parser) parseFuncHeaderUserParams() *Node {
	first := p.parseFuncHeaderAssign()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeFuncHeaderUserParams, pos, first)
	for {
		if !p.matchAny(TOK_NL, TOK_SEMICOLON) {
			break
		}
		c := p.save()
		p.skipEOL()
		next := p.parseFuncHeaderAssign()
		if next == nil {
			p.restore(c)
			break
		}
		n.Append(next)
	}
	return n
}

// =============================================================================
// ident_static_* and ident_static_fn
// =============================================================================

// parseIdentStaticStoreName parses the literal "@storeName" token.
//   ident_static_store_name = "@storeName";
func (p *Parser) parseIdentStaticStoreName() *Node {
	if !p.matchValue(TOK_AT_IDENT, "@storeName") {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIdentStaticStoreName,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIdentStaticStr parses "@name" | "@type" | "@storeName".
//   ident_static_str = "@name" | "@type" | ident_static_store_name;
func (p *Parser) parseIdentStaticStr() *Node {
	if !p.match(TOK_AT_IDENT) {
		return nil
	}
	v := p.peek().Value
	if v != "@name" && v != "@type" && v != "@storeName" {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIdentStaticStr,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIdentStaticBoolean parses the "ok" identifier.
//   ident_static_boolean = "ok";
func (p *Parser) parseIdentStaticBoolean() *Node {
	if !p.matchValue(TOK_IDENT, "ok") {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIdentStaticBoolean,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIdentStaticError parses the "error" identifier.
//   ident_static_error = "error";
func (p *Parser) parseIdentStaticError() *Node {
	if !p.matchValue(TOK_IDENT, "error") {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIdentStaticError,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIdentStaticDeps parses the "deps" identifier.
//   ident_static_deps = "deps";
func (p *Parser) parseIdentStaticDeps() *Node {
	if !p.matchValue(TOK_IDENT, "deps") {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIdentStaticDeps,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIdentStaticFunc parses "next" or "value".
//   ident_static_func = "next" | "value";
func (p *Parser) parseIdentStaticFunc() *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	v := p.peek().Value
	if v != "next" && v != "value" {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIdentStaticFunc,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIdentStaticFn parses  ident_ref "." "@" ( str | boolean | error | deps | func ).
//   ident_static_fn = ident_ref "." "@" ( ident_static_str | ident_static_boolean
//                   | ident_static_error | ident_static_deps | ident_static_func );
func (p *Parser) parseIdentStaticFn() *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	c := p.save()
	pos := p.curPos()
	ref := p.parseIdentRef()
	if ref == nil || !p.match(TOK_DOT) {
		p.restore(c)
		return nil
	}
	p.advance() // "."
	if !p.match(TOK_AT_IDENT) {
		p.restore(c)
		return nil
	}
	v := p.peek().Value
	var staticNode *Node
	switch v {
	case "@name", "@type", "@storeName":
		staticNode = p.parseIdentStaticStr()
	case "@ok": // map "ok" key
		staticNode = p.parseIdentStaticBoolean()
	case "@error":
		staticNode = p.parseIdentStaticError()
	case "@deps":
		staticNode = p.parseIdentStaticDeps()
	case "@next", "@value":
		staticNode = p.parseIdentStaticFunc()
	default:
		p.restore(c)
		return nil
	}
	if staticNode == nil {
		p.restore(c)
		return nil
	}
	return NewNode(NodeIdentStaticFn, pos, ref, staticNode)
}

// =============================================================================
// Function argument declarations
// =============================================================================

// parseFuncArgEntry parses one argument binding: assign_lhs assign_oper (empty_decl | ident_static_fn).
func (p *Parser) parseFuncArgEntry() *Node {
	lhs := p.parseAssignLHS()
	if lhs == nil {
		return nil
	}
	oper := p.parseAssignOper()
	if oper == nil {
		p.errorf("expected assignment operator in func arg entry")
		return nil
	}
	var rhs *Node
	if isEmptyDeclToken(p.peek().Type) {
		rhs = p.parseEmptyDecl()
	} else {
		rhs = p.try(p.parseIdentStaticFn)
		if rhs == nil {
			p.errorf("expected empty_decl or ident_static_fn in func arg")
			return nil
		}
	}
	pos := lhs.Pos
	return NewNode(NodeFuncArgs, pos, lhs, oper, rhs)
}

// parseFuncArgs parses "-> [ arg { , arg } ]".
//   func_args = "->" [ assign_lhs assign_oper (empty_decl|ident_static_fn)
//                     { "," assign_lhs assign_oper (empty_decl|ident_static_fn) } ];
func (p *Parser) parseFuncArgs() *Node {
	if !p.match(TOK_ARROW) {
		return nil
	}
	pos := p.curPos()
	p.advance() // "->"
	n := NewNode(NodeFuncArgs, pos)
	if first := p.parseFuncArgEntry(); first != nil {
		n.Append(first)
		for p.match(TOK_COMMA) {
			c := p.save()
			p.advance()
			entry := p.parseFuncArgEntry()
			if entry == nil {
				p.restore(c)
				break
			}
			n.Append(entry)
		}
	}
	return n
}

// parseFuncStreamArgs parses ">> arg { , arg }".
//   func_stream_args = ">>" assign_lhs assign_oper (empty_decl|ident_static_fn)
//                          { "," assign_lhs assign_oper (empty_decl|ident_static_fn) };
func (p *Parser) parseFuncStreamArgs() *Node {
	if !p.match(TOK_STREAM) {
		return nil
	}
	pos := p.curPos()
	p.advance() // ">>"
	n := NewNode(NodeFuncStreamArgs, pos)
	first := p.parseFuncArgEntry()
	if first == nil {
		p.errorf("expected at least one argument after '>>' in func_stream_args")
		return n
	}
	n.Append(first)
	for p.match(TOK_COMMA) {
		c := p.save()
		p.advance()
		entry := p.parseFuncArgEntry()
		if entry == nil {
			p.restore(c)
			break
		}
		n.Append(entry)
	}
	return n
}

// parseFuncDeps parses "=> UNIQUE< @storeName { , @storeName } >".
//   func_deps = "=>" UNIQUE< ident_static_store_name { "," ident_static_store_name } >;
func (p *Parser) parseFuncDeps() *Node {
	if !p.match(TOK_STORE) {
		return nil
	}
	pos := p.curPos()
	p.advance() // "=>"
	inner := p.parseDirectiveBrackets(func() *Node {
		first := p.parseIdentStaticStoreName()
		if first == nil {
			p.errorf("expected @storeName inside func_deps UNIQUE<…>")
			return nil
		}
		list := NewNode(NodeFuncDeps, pos, first)
		for p.match(TOK_COMMA) {
			c := p.save()
			p.advance()
			sn := p.parseIdentStaticStoreName()
			if sn == nil {
				p.restore(c)
				break
			}
			list.Append(sn)
		}
		return list
	})
	if inner == nil {
		return nil
	}
	return NewNode(NodeFuncDeps, pos,
		NewDirectiveNode(NodeDirectiveUnique, "", pos, inner))
}

// parseFuncArgsDecl parses the optional args/stream-args/deps clause.
//   func_args_decl = [ func_args ] [ func_stream_args ] [ func_deps ];
func (p *Parser) parseFuncArgsDecl() *Node {
	pos := p.curPos()
	n := NewNode(NodeFuncArgsDecl, pos)
	if args := p.try(p.parseFuncArgs); args != nil {
		n.Append(args)
	}
	if streamArgs := p.try(p.parseFuncStreamArgs); streamArgs != nil {
		n.Append(streamArgs)
	}
	if deps := p.try(p.parseFuncDeps); deps != nil {
		n.Append(deps)
	}
	return n
}

// =============================================================================
// Function range args
// =============================================================================

// parseFuncFixedNumRange parses a numeric range for function overloading.
//   func_fixed_num_range = ( numeric_const | TYPE_OF numeric_const<ident_ref> )
//                          ".." ( numeric_const | TYPE_OF numeric_const<ident_ref> );
func (p *Parser) parseFuncFixedNumRange() *Node {
	c := p.save()
	pos := p.curPos()
	lo := p.parseFuncNumRangeBound()
	if lo == nil {
		return nil
	}
	if !p.match(TOK_DOTDOT) {
		p.restore(c)
		return nil
	}
	p.advance() // ".."
	hi := p.parseFuncNumRangeBound()
	if hi == nil {
		p.restore(c)
		return nil
	}
	return NewNode(NodeFuncFixedNumRange, pos, lo, hi)
}

// parseFuncNumRangeBound parses one bound of a func_fixed_num_range.
func (p *Parser) parseFuncNumRangeBound() *Node {
	pos := p.curPos()
	if p.matchAny(TOK_INTEGER, TOK_DECIMAL, TOK_PLUS, TOK_MINUS) {
		if nc := p.parseNumericConst(); nc != nil {
			return nc
		}
	}
	// TYPE_OF numeric_const<ident_ref>
	if p.match(TOK_IDENT) {
		ref := p.parseIdentRef()
		if ref != nil {
			inner := NewNode(NodeFuncFixedNumRange, pos, ref)
			return NewDirectiveNode(NodeDirectiveTypeOf, "numeric_const", pos, inner)
		}
	}
	return nil
}

// parseFuncFixedStrRange parses a string or TYPE_OF string<ident_ref> range bound.
//   func_fixed_str_range = string | TYPE_OF string<ident_ref>;
func (p *Parser) parseFuncFixedStrRange() *Node {
	pos := p.curPos()
	if p.match(TOK_STRING) {
		str := p.parseString()
		if str != nil {
			return NewNode(NodeFuncFixedStrRange, pos, str)
		}
	}
	if p.match(TOK_IDENT) {
		ref := p.parseIdentRef()
		if ref != nil {
			inner := NewNode(NodeFuncFixedStrRange, pos, ref)
			return NewDirectiveNode(NodeDirectiveTypeOf, "string", pos, inner)
		}
	}
	return nil
}

// parseFuncFixedListRange parses array_list or object_list as a range specifier.
//   func_fixed_list_range = array_list | object_list;
func (p *Parser) parseFuncFixedListRange() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_LBRACKET, TOK_EMPTY_ARR:
		child = p.parseArrayList()
	case TOK_LBRACE, TOK_EMPTY_OBJ:
		child = p.parseObjectList()
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeFuncFixedListRange, pos, child)
}

// parseFuncRangeArgs parses any of the function range argument forms.
//   func_range_args = func_fixed_num_range | func_fixed_str_range | func_fixed_list_range;
func (p *Parser) parseFuncRangeArgs() *Node {
	pos := p.curPos()
	if n := p.try(p.parseFuncFixedNumRange); n != nil {
		return NewNode(NodeFuncRangeArgs, pos, n)
	}
	if n := p.try(p.parseFuncFixedListRange); n != nil {
		return NewNode(NodeFuncRangeArgs, pos, n)
	}
	if n := p.try(p.parseFuncFixedStrRange); n != nil {
		return NewNode(NodeFuncRangeArgs, pos, n)
	}
	return nil
}

// =============================================================================
// Function calls
// =============================================================================

// parseFuncCallArgs parses a comma-separated list of assign_rhs values.
//   func_call_args = assign_rhs { "," assign_rhs };
func (p *Parser) parseFuncCallArgs() *Node {
	first := p.parseAssignRHS()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeFuncCallArgs, pos, first)
	for p.match(TOK_COMMA) {
		c := p.save()
		p.advance()
		next := p.parseAssignRHS()
		if next == nil {
			p.restore(c)
			break
		}
		n.Append(next)
	}
	return n
}

// parseFuncUnitTypeRef parses TYPE_OF func_unit<ident_ref>.
// Used inside func_call_1, func_call_2, and chained forms.
// Returns the NodeDirectiveTypeOf wrapper.
func (p *Parser) parseFuncUnitTypeRef() *Node {
	if !p.match(TOK_TYPE_OF) {
		return nil
	}
	pos := p.curPos()
	p.advance() // TYPE_OF
	inner := p.parseDirectiveBrackets(func() *Node {
		return p.parseIdentRef()
	})
	if inner == nil {
		return nil
	}
	return NewDirectiveNode(NodeDirectiveTypeOf, "func_unit", pos, inner)
}

// parseFuncCall1 parses:   (func_call_args | func_range_args) ( "->" | ">>" ) TYPE_OF func_unit<ident_ref>.
//   func_call_1 = (func_call_args | func_range_args) ("->" | ">>") TYPE_OF func_unit<ident_ref>;
func (p *Parser) parseFuncCall1() *Node {
	if !p.enter("func_call_1") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	c := p.save()
	pos := p.curPos()

	// args or range
	var args *Node
	if n := p.try(p.parseFuncRangeArgs); n != nil {
		args = n
	} else if n := p.try(p.parseFuncCallArgs); n != nil {
		args = n
	} else {
		return nil
	}
	if !p.matchAny(TOK_ARROW, TOK_STREAM) {
		p.restore(c)
		return nil
	}
	connTok := p.advance() // "->" or ">>"
	ref := p.parseFuncUnitTypeRef()
	if ref == nil {
		p.restore(c)
		return nil
	}
	return NewNode(NodeFuncCall1, pos, args, NewTokenNode(connTok), ref)
}

// parseFuncCall2 parses:   TYPE_OF func_unit<ident_ref> [ func_call_args ].
//   func_call_2 = TYPE_OF func_unit<ident_ref> [ func_call_args ];
func (p *Parser) parseFuncCall2() *Node {
	if !p.enter("func_call_2") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	if !p.match(TOK_TYPE_OF) {
		return nil
	}
	pos := p.curPos()
	ref := p.parseFuncUnitTypeRef()
	if ref == nil {
		return nil
	}
	n := NewNode(NodeFuncCall2, pos, ref)
	if args := p.try(p.parseFuncCallArgs); args != nil {
		n.Append(args)
	}
	return n
}

// parseFuncCallBase parses either func_call_1 or func_call_2.
func (p *Parser) parseFuncCallBase() *Node {
	if n := p.try(p.parseFuncCall1); n != nil {
		return n
	}
	return p.try(p.parseFuncCall2)
}

// parseFuncCallDataChain parses a data-piped chain of function calls.
//   func_call_data_chain = (func_call_1 | func_call_2) { ( "->" | ">>" ) TYPE_OF func_unit<ident_ref> };
func (p *Parser) parseFuncCallDataChain() *Node {
	if !p.enter("func_call_data_chain") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	first := p.parseFuncCallBase()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeFuncCallDataChain, pos, first)
	for p.matchAny(TOK_ARROW, TOK_STREAM) {
		c := p.save()
		connTok := p.advance()
		ref := p.parseFuncUnitTypeRef()
		if ref == nil {
			p.restore(c)
			break
		}
		n.Append(NewTokenNode(connTok), ref)
	}
	return n
}

// parseFuncCallLogicChain parses a logic-conditional chain of function calls.
//   func_call_logic_chain = (func_call_1 | func_call_2) { logic_oper TYPE_OF func_unit<ident_ref> };
func (p *Parser) parseFuncCallLogicChain() *Node {
	if !p.enter("func_call_logic_chain") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	first := p.parseFuncCallBase()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeFuncCallLogicChain, pos, first)
	for p.matchAny(TOK_AMP, TOK_PIPE, TOK_CARET) {
		c := p.save()
		operTok := p.advance()
		ref := p.parseFuncUnitTypeRef()
		if ref == nil {
			p.restore(c)
			break
		}
		n.Append(NewTokenNode(operTok), ref)
	}
	return n
}

// parseFuncCallMixedChain parses the outermost function-call chain.
//   func_call_mixed_chain = ( func_call_data_chain | func_call_logic_chain )
//                           { func_call_data_chain | func_call_logic_chain };
func (p *Parser) parseFuncCallMixedChain() *Node {
	if !p.enter("func_call_mixed_chain") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	var first *Node
	if n := p.try(p.parseFuncCallDataChain); n != nil {
		first = n
	} else if n := p.try(p.parseFuncCallLogicChain); n != nil {
		first = n
	} else {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeFuncCallMixedChain, pos, first)
	for {
		var next *Node
		if m := p.try(p.parseFuncCallDataChain); m != nil {
			next = m
		} else if m := p.try(p.parseFuncCallLogicChain); m != nil {
			next = m
		} else {
			break
		}
		n.Append(next)
	}
	return n
}

// =============================================================================
// func_stream_loop
// =============================================================================

// parseFuncStreamLoop parses a stream loop expression.
//   func_stream_loop = ( func_range_args | ident_ref | boolean_true ) ">>" ( func_unit | func_call_2 );
func (p *Parser) parseFuncStreamLoop() *Node {
	if !p.enter("func_stream_loop") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	c := p.save()
	pos := p.curPos()

	var source *Node
	if n := p.try(p.parseFuncRangeArgs); n != nil {
		source = n
	} else if p.match(TOK_TRUE) {
		source = p.parseBooleanTrue()
	} else if p.match(TOK_IDENT) {
		source = p.parseIdentRef()
	}
	if source == nil || !p.match(TOK_STREAM) {
		p.restore(c)
		return nil
	}
	streamTok := p.advance() // ">>"

	var sink *Node
	if n := p.try(p.parseFuncCall2); n != nil {
		sink = n
	} else {
		sink = p.parseFuncUnit()
	}
	if sink == nil {
		p.restore(c)
		return nil
	}
	return NewNode(NodeFuncStreamLoop, pos, source, NewTokenNode(streamTok), sink)
}

// =============================================================================
// func_stmt / assignment / func body statements
// =============================================================================

// parseFuncStmt parses the RHS of a function body statement.
//   func_stmt = regexp | ident_ref | object_list | array_list
//             | func_call_mixed_chain | func_unit | calc_unit;
func (p *Parser) parseFuncStmt() *Node {
	if !p.enter("func_stmt") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	var child *Node

	switch p.peek().Type {
	case TOK_REGEXP:
		child = p.parseRegexp()
	case TOK_LBRACE, TOK_EMPTY_OBJ:
		child = p.parseObjectList()
	case TOK_LBRACKET, TOK_EMPTY_ARR:
		child = p.parseArrayList()
	case TOK_LPAREN:
		// func_unit starts with "("
		child = p.try(p.parseFuncUnit)
		if child == nil {
			child = p.try(p.parseCalcUnit)
		}
	case TOK_TYPE_OF:
		// func_call_2 or func_call_mixed_chain
		child = p.try(p.parseFuncCallMixedChain)
		if child == nil {
			child = p.try(p.parseFuncCall2)
		}
	case TOK_IDENT:
		// Ordered: func_call_mixed_chain > calc_unit > ident_ref
		if n := p.try(p.parseFuncCallMixedChain); n != nil {
			child = n
		} else if n := p.try(p.parseCalcUnit); n != nil {
			child = n
		} else {
			child = p.parseIdentRef()
		}
	default:
		// calc_unit for literals
		child = p.try(p.parseCalcUnit)
	}

	if child == nil {
		return nil
	}
	return NewNode(NodeFuncStmt, pos, child)
}

// parseAssignment parses a complete assignment statement.
//   assignment = assign_lhs assign_oper func_stmt;
func (p *Parser) parseAssignment() *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	c := p.save()
	pos := p.curPos()
	lhs := p.parseAssignLHS()
	if lhs == nil {
		return nil
	}
	oper := p.parseAssignOper()
	if oper == nil {
		p.restore(c)
		return nil
	}
	stmt := p.parseFuncStmt()
	if stmt == nil {
		p.restore(c)
		return nil
	}
	return NewNode(NodeAssignment, pos, lhs, oper, stmt)
}

// parseFuncReturnStmt parses "<- func_stmt".
//   func_return_stmt = "<-" func_stmt;
func (p *Parser) parseFuncReturnStmt() *Node {
	if !p.match(TOK_RETURN_STMT) {
		return nil
	}
	pos := p.curPos()
	p.advance() // "<-"
	stmt := p.parseFuncStmt()
	if stmt == nil {
		p.errorf("expected expression after '<-'")
		return nil
	}
	return NewNode(NodeFuncReturnStmt, pos, stmt)
}

// parseFuncStoreStmt parses "=> (object_list | TYPE_OF object_list<ident_ref>) { , … }".
//   func_store_stmt = "=>" ( object_list | TYPE_OF object_list<ident_ref> )
//                     { "," ( object_list | TYPE_OF object_list<ident_ref> ) };
func (p *Parser) parseFuncStoreStmt() *Node {
	if !p.match(TOK_STORE) {
		return nil
	}
	pos := p.curPos()
	p.advance() // "=>"
	n := NewNode(NodeFuncStoreStmt, pos)
	entry := p.parseStoreEntry()
	if entry == nil {
		p.errorf("expected object_list or TYPE_OF after '=>'")
		return n
	}
	n.Append(entry)
	for p.match(TOK_COMMA) {
		c := p.save()
		p.advance()
		e := p.parseStoreEntry()
		if e == nil {
			p.restore(c)
			break
		}
		n.Append(e)
	}
	return n
}

// parseStoreEntry parses one store RHS: object_list or TYPE_OF object_list<ident_ref>.
func (p *Parser) parseStoreEntry() *Node {
	if p.matchAny(TOK_LBRACE, TOK_EMPTY_OBJ) {
		return p.parseObjectList()
	}
	if p.match(TOK_TYPE_OF) {
		pos := p.curPos()
		p.advance()
		inner := p.parseDirectiveBrackets(func() *Node {
			return p.parseIdentRef()
		})
		if inner == nil {
			return nil
		}
		return NewDirectiveNode(NodeDirectiveTypeOf, "object_list", pos, inner)
	}
	return nil
}

// parseFuncBodyStmt parses one statement in a function body.
//   func_body_stmt = assignment | func_return_stmt | func_store_stmt | func_stream_loop;
func (p *Parser) parseFuncBodyStmt() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_RETURN_STMT:
		child = p.parseFuncReturnStmt()
	case TOK_STORE:
		child = p.parseFuncStoreStmt()
	default:
		// Try stream loop first (needs func_range_args OR ident OR true >> …)
		if n := p.try(p.parseFuncStreamLoop); n != nil {
			child = n
		} else {
			child = p.try(p.parseAssignment)
		}
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeFuncBodyStmt, pos, child)
}

// =============================================================================
// func_private_params / func_unit_header
// =============================================================================

// parseFuncPrivateParams parses the private parameters section of a function header.
//   func_private_params = ( func_header_buildin_params | func_header_user_params )
//                         { EOL ( func_header_buildin_params | func_header_user_params ) };
func (p *Parser) parseFuncPrivateParams() *Node {
	pos := p.curPos()
	n := NewNode(NodeFuncPrivateParams, pos)
	first := p.parseFuncPrivateParamsEntry()
	if first == nil {
		return nil
	}
	n.Append(first)
	for {
		if !p.matchAny(TOK_NL, TOK_SEMICOLON) {
			break
		}
		c := p.save()
		p.skipEOL()
		entry := p.parseFuncPrivateParamsEntry()
		if entry == nil {
			p.restore(c)
			break
		}
		n.Append(entry)
	}
	return n
}

// parseFuncPrivateParamsEntry is one line of func_private_params.
func (p *Parser) parseFuncPrivateParamsEntry() *Node {
	if p.match(TOK_AT_IDENT) {
		if n := p.try(p.parseFuncHeaderBuildinParams); n != nil {
			return n
		}
	}
	return p.try(p.parseFuncHeaderUserParams)
}

// parseFuncUnitHeader parses the optional header of a func_unit.
//   func_unit_header = [ (func_private_params func_args_decl) | func_args_decl ];
func (p *Parser) parseFuncUnitHeader() *Node {
	pos := p.curPos()
	n := NewNode(NodeFuncUnitHeader, pos)
	// Try private params first; if present, must be followed by args_decl.
	if pp := p.try(p.parseFuncPrivateParams); pp != nil {
		n.Append(pp)
	}
	argsDecl := p.parseFuncArgsDecl()
	if argsDecl != nil && len(argsDecl.Children) > 0 {
		n.Append(argsDecl)
	}
	return n
}

// =============================================================================
// func_unit
// =============================================================================

// parseFuncUnit parses a function unit literal.
//   func_unit  = "(" func_unit_header func_body_stmt { EOL | func_body_stmt }
//              | func_unit_header "(" func_body_stmt { EOL | func_body_stmt } ")"
//              | ")";
func (p *Parser) parseFuncUnit() *Node {
	if !p.enter("func_unit") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	if !p.match(TOK_LPAREN) {
		return nil
	}
	pos := p.curPos()
	n := NewNode(NodeFuncUnit, pos)

	p.advance() // consume "("

	// Grammar alternative 3: immediate ")" → empty body func.
	if p.match(TOK_RPAREN) {
		p.advance()
		return n
	}

	// parseFuncBodyStmts reads one required stmt then repeats { EOL | stmt }.
	parseFuncBodyStmts := func() {
		stmt := p.parseFuncBodyStmt()
		if stmt != nil {
			n.Append(stmt)
		}
		for {
			if p.match(TOK_RPAREN) || p.match(TOK_EOF) {
				break
			}
			if p.matchAny(TOK_NL, TOK_SEMICOLON) {
				p.skipEOL()
				continue
			}
			stmt = p.parseFuncBodyStmt()
			if stmt == nil {
				break
			}
			n.Append(stmt)
		}
	}

	// Alternative 1: "(" func_unit_header func_body_stmt … ")"
	// We already consumed "("; now check if there's a header before the body.
	header := p.parseFuncUnitHeader()
	if header != nil && len(header.Children) > 0 {
		n.Append(header)
	}

	// Check for nested "(" (alternative 2 — body wrapped in extra parens).
	if p.match(TOK_LPAREN) {
		p.advance() // consume nested "("
		parseFuncBodyStmts()
		p.consume(TOK_RPAREN)
	} else {
		parseFuncBodyStmts()
	}
	p.consume(TOK_RPAREN)
	return n
}

// =============================================================================
// scope_assign / scope_unit
// =============================================================================

// parseScopeUnit parses a scope block body.
//   scope_unit = assignment { ( assignment | scope_assign ) };
func (p *Parser) parseScopeUnit() *Node {
	if !p.enter("scope_unit") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	first := p.parseAssignment()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeScopeUnit, pos, first)
	for {
		p.skipNL()
		// Try scope_assign before assignment — both start with ident_name.
		if sa := p.try(p.parseScopeAssign); sa != nil {
			n.Append(sa)
			continue
		}
		if a := p.try(p.parseAssignment); a != nil {
			n.Append(a)
			continue
		}
		break
	}
	return n
}

// parseScopeAssign parses a scoped assignment.
//   scope_assign = assign_lhs equal_assign "<" scope_unit ">" EOL;
func (p *Parser) parseScopeAssign() *Node {
	if !p.enter("scope_assign") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	if !p.match(TOK_IDENT) {
		return nil
	}
	c := p.save()
	pos := p.curPos()
	lhs := p.parseAssignLHS()
	if lhs == nil {
		return nil
	}
	oper := p.parseEqualAssign()
	if oper == nil || !p.match(TOK_LT) {
		p.restore(c)
		return nil
	}
	p.advance() // "<"

	unit := p.withScope(p.parseScopeUnit)
	if unit == nil {
		p.restore(c)
		return nil
	}
	if !p.match(TOK_GT) {
		p.restore(c)
		return nil
	}
	p.advance() // ">"
	p.parseEOL()
	return NewNode(NodeScopeAssign, pos, lhs, oper, unit)
}
