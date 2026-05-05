// parser_v17_objects.go — ParseXxx methods for spec/04_objects.sqg.
//
// Rules implemented:
//
//	empty_array_decl, func_stream_decl, func_regexp_decl, empty_scope_decl,
//	func_string_decl, empty_decl,
//	spread_oper, spread_array, values_list,
//	array_uniform, empty_array_typed, array_append_tail, array_omit_tail, array_final,
//	lookup_idx_expr, array_lookup,
//	object_init, object_merge_tail, object_omit_tail, object_merge_or_omit, object_final,
//	lookup_txt_expr, object_lookup, collection
//
// EXTEND<statement> = | array_final | object_final | array_lookup | object_lookup
//
//	(branches added to ParseStatement in parser_v17_operators.go)
package parser

import "fmt"

// =============================================================================
// empty_array_decl = "[]"
// =============================================================================

// V17EmptyArrayDeclNode  empty_array_decl = "[]"
type V17EmptyArrayDeclNode struct{ V17BaseNode }

// ParseEmptyArrayDecl parses empty_array_decl = "[]".
func (p *V17Parser) ParseEmptyArrayDecl() (node *V17EmptyArrayDeclNode, err error) {
	done := p.debugEnter("empty_array_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	if !p.peekLit("[]") {
		return nil, p.errAt("empty_array_decl: expected '[]'")
	}
	if _, err = p.matchLit("[]"); err != nil {
		return nil, err
	}
	return &V17EmptyArrayDeclNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// func_stream_decl = ">>"
// =============================================================================

// V17FuncStreamDeclNode  func_stream_decl = ">>"
type V17FuncStreamDeclNode struct{ V17BaseNode }

// ParseFuncStreamDecl parses func_stream_decl = ">>".
func (p *V17Parser) ParseFuncStreamDecl() (node *V17FuncStreamDeclNode, err error) {
	done := p.debugEnter("func_stream_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	if !p.peekLit(">>") {
		return nil, p.errAt("func_stream_decl: expected '>>'")
	}
	if _, err = p.matchLit(">>"); err != nil {
		return nil, err
	}
	return &V17FuncStreamDeclNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// func_regexp_decl = "//"  (empty regexp literal)
// =============================================================================

// V17FuncRegexpDeclNode  func_regexp_decl = "//"
type V17FuncRegexpDeclNode struct{ V17BaseNode }

// ParseFuncRegexpDecl parses func_regexp_decl = "//" (an empty regexp literal).
// The lexer produces V17_REGEXP with Value="" when two slashes appear in a
// non-value position.
func (p *V17Parser) ParseFuncRegexpDecl() (node *V17FuncRegexpDeclNode, err error) {
	done := p.debugEnter("func_regexp_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	tok, terr := p.matchRegexpLiteral()
	if terr != nil || tok.Value != "" {
		return nil, p.errAt("func_regexp_decl: expected empty regexp '//'")
	}
	return &V17FuncRegexpDeclNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// empty_scope_decl = "{}"
// =============================================================================

// V17EmptyScopeDeclNode  empty_scope_decl = "{}"
type V17EmptyScopeDeclNode struct{ V17BaseNode }

// ParseEmptyScopeDecl parses empty_scope_decl = "{}".
func (p *V17Parser) ParseEmptyScopeDecl() (node *V17EmptyScopeDeclNode, err error) {
	done := p.debugEnter("empty_scope_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	if !p.peekLit("{}") {
		return nil, p.errAt("empty_scope_decl: expected '{}'")
	}
	if _, err = p.matchLit("{}"); err != nil {
		return nil, err
	}
	return &V17EmptyScopeDeclNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// func_string_decl = '""' | "''" | "``"
// =============================================================================

// V17FuncStringDeclNode  func_string_decl = '""' | "''" | "``"
type V17FuncStringDeclNode struct {
	V17BaseNode
	QuoteType V17TokenType // V17_STRING_DQ, V17_STRING_SQ, or V17_STRING_TQ
}

// ParseFuncStringDecl parses func_string_decl = '""' | "''" | "``".
// Matches only string tokens whose Value is empty (i.e. the string is empty).
func (p *V17Parser) ParseFuncStringDecl() (node *V17FuncStringDeclNode, err error) {
	done := p.debugEnter("func_string_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	var tok V17Token
	var terr error
	ch := p.peekAfterWS()
	switch ch {
	case '"':
		tok, terr = p.matchDoubleQuoted()
		if terr == nil && tok.Value == "" {
			return &V17FuncStringDeclNode{V17BaseNode{line, col}, V17_STRING_DQ}, nil
		}
	case '\'':
		tok, terr = p.matchSingleQuoted()
		if terr == nil && tok.Value == "" {
			return &V17FuncStringDeclNode{V17BaseNode{line, col}, V17_STRING_SQ}, nil
		}
	case '`':
		tok, terr = p.matchTemplateQuoted()
		if terr == nil && tok.Value == "" {
			return &V17FuncStringDeclNode{V17BaseNode{line, col}, V17_STRING_TQ}, nil
		}
	}
	_ = tok
	return nil, p.errAt("func_string_decl: expected empty string literal (\"\", '', or ``)")
}

// =============================================================================
// empty_decl = empty_array_decl | func_stream_decl | func_regexp_decl
//            | empty_scope_decl | func_string_decl
// =============================================================================

// V17EmptyDeclNode  empty_decl = empty_array_decl | func_stream_decl | ...
type V17EmptyDeclNode struct {
	V17BaseNode
	// Value is one of: *V17EmptyArrayDeclNode | *V17FuncStreamDeclNode |
	//                  *V17FuncRegexpDeclNode | *V17EmptyScopeDeclNode | *V17FuncStringDeclNode
	Value interface{}
}

// ParseEmptyDecl parses empty_decl = empty_array_decl | func_stream_decl
// | func_regexp_decl | empty_scope_decl | func_string_decl.
func (p *V17Parser) ParseEmptyDecl() (node *V17EmptyDeclNode, err error) {
	done := p.debugEnter("empty_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if saved := p.savePos(); true {
		if ad, aderr := p.ParseEmptyArrayDecl(); aderr == nil {
			return &V17EmptyDeclNode{V17BaseNode{line, col}, ad}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if sd, sderr := p.ParseFuncStreamDecl(); sderr == nil {
			return &V17EmptyDeclNode{V17BaseNode{line, col}, sd}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if rd, rderr := p.ParseFuncRegexpDecl(); rderr == nil {
			return &V17EmptyDeclNode{V17BaseNode{line, col}, rd}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if sc, scerr := p.ParseEmptyScopeDecl(); scerr == nil {
			return &V17EmptyDeclNode{V17BaseNode{line, col}, sc}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if fsd, fsderr := p.ParseFuncStringDecl(); fsderr == nil {
			return &V17EmptyDeclNode{V17BaseNode{line, col}, fsd}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("empty_decl: expected '[]', '>>', '//', '{}', or empty string literal")
}

// =============================================================================
// spread_oper = "..."
// =============================================================================

// V17SpreadOperNode  spread_oper = "..."
type V17SpreadOperNode struct{ V17BaseNode }

// ParseSpreadOper parses spread_oper = "...".
// The lexer produces V17_DOTDOT (".." ) + V17_DOT (".") for "...".
func (p *V17Parser) ParseSpreadOper() (node *V17SpreadOperNode, err error) {
	done := p.debugEnter("spread_oper")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	if !p.peekLit("...") {
		return nil, p.errAt("spread_oper: expected '...'")
	}
	if _, err = p.matchLit("..."); err != nil {
		return nil, err
	}
	return &V17SpreadOperNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// spread_array = spread_oper ( string | digits | integer | date | date_time | time | time_stamp )
// =============================================================================

// V17SpreadArrayNode  spread_array = spread_oper (string | digits | integer | ...)
type V17SpreadArrayNode struct {
	V17BaseNode
	Oper *V17SpreadOperNode
	// Value is one of: *V17StringNode | *V17DigitsNode | *V17IntegerNode |
	//                  *V17DateTimeNode | *V17DateNode | *V17TimeStampNode | *V17TimeNode
	Value interface{}
}

// ParseSpreadArray parses spread_array = spread_oper (string | digits | integer | date | date_time | time | time_stamp).
// More-specific date/time alternatives are tried before less-specific ones.
func (p *V17Parser) ParseSpreadArray() (node *V17SpreadArrayNode, err error) {
	done := p.debugEnter("spread_array")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	oper, oerr := p.ParseSpreadOper()
	if oerr != nil {
		return nil, fmt.Errorf("spread_array: %w", oerr)
	}

	// string — unique token type, try first
	if saved := p.savePos(); true {
		if s, serr := p.ParseString(); serr == nil {
			return &V17SpreadArrayNode{V17BaseNode{line, col}, oper, s}, nil
		}
		p.restorePos(saved)
	}
	// date_time before date (date_time is a superset)
	if saved := p.savePos(); true {
		if dt, dterr := p.ParseDateTime(); dterr == nil {
			return &V17SpreadArrayNode{V17BaseNode{line, col}, oper, dt}, nil
		}
		p.restorePos(saved)
	}
	// date before time
	if saved := p.savePos(); true {
		if d, derr := p.ParseDate(); derr == nil {
			return &V17SpreadArrayNode{V17BaseNode{line, col}, oper, d}, nil
		}
		p.restorePos(saved)
	}
	// time_stamp before time (time_stamp is a superset)
	if saved := p.savePos(); true {
		if ts, tserr := p.ParseTimeStamp(); tserr == nil {
			return &V17SpreadArrayNode{V17BaseNode{line, col}, oper, ts}, nil
		}
		p.restorePos(saved)
	}
	// time
	if saved := p.savePos(); true {
		if t, terr := p.ParseTime(); terr == nil {
			return &V17SpreadArrayNode{V17BaseNode{line, col}, oper, t}, nil
		}
		p.restorePos(saved)
	}
	// integer before digits (integer includes optional sign prefix)
	if saved := p.savePos(); true {
		if i, ierr := p.ParseInteger(); ierr == nil {
			return &V17SpreadArrayNode{V17BaseNode{line, col}, oper, i}, nil
		}
		p.restorePos(saved)
	}
	// digits
	if saved := p.savePos(); true {
		if d, derr := p.ParseDigits(); derr == nil {
			return &V17SpreadArrayNode{V17BaseNode{line, col}, oper, d}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("spread_array: expected string, digits, integer, date, date_time, time, or time_stamp after '...'")
}

// =============================================================================
// values_list = statement { "," statement }
// =============================================================================

// V17ValuesListNode  values_list = statement { "," statement }
type V17ValuesListNode struct {
	V17BaseNode
	Items []*V17StatementNode
}

// ParseValuesList parses values_list = statement { "," statement }.
// The comma loop backtracks if the statement after a comma fails, allowing
// the outer parser to handle trailing commas.
func (p *V17Parser) ParseValuesList() (node *V17ValuesListNode, err error) {
	done := p.debugEnter("values_list")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseStatement()
	if ferr != nil {
		return nil, fmt.Errorf("values_list: %w", ferr)
	}
	items := []*V17StatementNode{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		stmt, sterr := p.ParseStatement()
		if sterr != nil {
			p.restorePos(saved) // backtrack the comma
			break
		}
		items = append(items, stmt)
	}

	return &V17ValuesListNode{V17BaseNode{line, col}, items}, nil
}

// =============================================================================
// array_uniform = "[" UNIFORM INFER<( values_list | spread_array | array_uniform )> [","] "]"
// =============================================================================

// V17ArrayUniformNode  array_uniform = "[" INFER<...> [","] "]"
type V17ArrayUniformNode struct {
	V17BaseNode
	// Content is one of: *V17ValuesListNode | *V17SpreadArrayNode | *V17ArrayUniformNode | nil (empty)
	Content       interface{}
	TrailingComma bool
}

// ParseArrayUniform parses array_uniform = "[" UNIFORM INFER<(values_list | spread_array | array_uniform)> [","] "]".
func (p *V17Parser) ParseArrayUniform() (node *V17ArrayUniformNode, err error) {
	done := p.debugEnter("array_uniform")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	tokBracket, terr := p.matchLit("[")
	if terr != nil {
		return nil, p.errAt("array_uniform: expected '['")
	}
	_ = tokBracket

	var content interface{}

	if p.peekAfterWS() != ']' && p.peekAfterWS() != ',' {
		// spread_array starts with "..."
		if p.peekLit("...") {
			sa, saerr := p.ParseSpreadArray()
			if saerr != nil {
				return nil, fmt.Errorf("array_uniform: %w", saerr)
			}
			content = sa
		} else {
			// Try values_list first (spec order)
			if saved := p.savePos(); true {
				if vl, vlerr := p.ParseValuesList(); vlerr == nil {
					content = vl
				} else {
					p.restorePos(saved)
				}
			}
			// If values_list failed, try array_uniform recursively
			if content == nil && p.peekAfterWS() == '[' {
				if saved := p.savePos(); true {
					if au, auerr := p.ParseArrayUniform(); auerr == nil {
						content = au
					} else {
						p.restorePos(saved)
					}
				}
			}
		}
	}

	// Optional trailing comma
	trailingComma := false
	if p.peekAfterWS() == ',' {
		trailingComma = true
		if _, err = p.matchLit(","); err != nil {
			return nil, err
		}
	}

	if _, err = p.matchLit("]"); err != nil {
		return nil, fmt.Errorf("array_uniform: %w", err)
	}

	return &V17ArrayUniformNode{V17BaseNode{line, col}, content, trailingComma}, nil
}

// =============================================================================
// empty_array_typed = ident_ref "[]"
// =============================================================================

// V17EmptyArrayTypedNode  empty_array_typed = ident_ref "[]"
type V17EmptyArrayTypedNode struct {
	V17BaseNode
	TypeRef *V17IdentRefNode
}

// ParseEmptyArrayTyped parses empty_array_typed = ident_ref "[]".
func (p *V17Parser) ParseEmptyArrayTyped() (node *V17EmptyArrayTypedNode, err error) {
	done := p.debugEnter("empty_array_typed")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	ref, rerr := p.ParseIdentRef()
	if rerr != nil {
		return nil, fmt.Errorf("empty_array_typed: %w", rerr)
	}
	if !p.peekLit("[]") {
		return nil, p.errAt("empty_array_typed: expected '[]' after ident_ref")
	}
	if _, err = p.matchLit("[]"); err != nil {
		return nil, err
	}
	return &V17EmptyArrayTypedNode{V17BaseNode{line, col}, ref}, nil
}

// =============================================================================
// array_append_tail = "+" array_uniform { "+" array_uniform }
// =============================================================================

// V17ArrayAppendTailNode  array_append_tail = "+" array_uniform { "+" array_uniform }
type V17ArrayAppendTailNode struct {
	V17BaseNode
	Items []*V17ArrayUniformNode
}

// ParseArrayAppendTail parses array_append_tail = "+" array_uniform { "+" array_uniform }.
func (p *V17Parser) ParseArrayAppendTail() (node *V17ArrayAppendTailNode, err error) {
	done := p.debugEnter("array_append_tail")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() != '+' {
		return nil, p.errAt("array_append_tail: expected '+'")
	}
	if _, err = p.matchLit("+"); err != nil {
		return nil, err
	}

	first, ferr := p.ParseArrayUniform()
	if ferr != nil {
		return nil, fmt.Errorf("array_append_tail: %w", ferr)
	}
	items := []*V17ArrayUniformNode{first}

	for p.peekAfterWS() == '+' {
		saved := p.savePos()
		if _, err = p.matchLit("+"); err != nil {
			p.restorePos(saved)
			break
		}
		au, auerr := p.ParseArrayUniform()
		if auerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, au)
	}

	return &V17ArrayAppendTailNode{V17BaseNode{line, col}, items}, nil
}

// =============================================================================
// array_omit_tail = "-" integer { "," integer }
// =============================================================================

// V17ArrayOmitTailNode  array_omit_tail = "-" integer { "," integer }
type V17ArrayOmitTailNode struct {
	V17BaseNode
	Indices []*V17IntegerNode
}

// ParseArrayOmitTail parses array_omit_tail = "-" integer { "," integer }.
func (p *V17Parser) ParseArrayOmitTail() (node *V17ArrayOmitTailNode, err error) {
	done := p.debugEnter("array_omit_tail")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() != '-' {
		return nil, p.errAt("array_omit_tail: expected '-'")
	}
	if _, err = p.matchLit("-"); err != nil {
		return nil, err
	}

	first, ferr := p.ParseInteger()
	if ferr != nil {
		return nil, fmt.Errorf("array_omit_tail: %w", ferr)
	}
	indices := []*V17IntegerNode{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		idx, idxerr := p.ParseInteger()
		if idxerr != nil {
			p.restorePos(saved)
			break
		}
		indices = append(indices, idx)
	}

	return &V17ArrayOmitTailNode{V17BaseNode{line, col}, indices}, nil
}

// =============================================================================
// array_final = ( TYPE_OF array_list<ident_ref> | array_list ) { array_append_tail | array_omit_tail }
// array_list   = array_uniform | empty_array_typed
// =============================================================================

// V17ArrayFinalNode  array_final = (TYPE_OF array_list<ident_ref> | array_list) { suffix }
type V17ArrayFinalNode struct {
	V17BaseNode
	// Base is one of: *V17ArrayUniformNode | *V17EmptyArrayTypedNode | *V17IdentRefNode
	Base interface{}
	// Suffixes is a mixed slice of *V17ArrayAppendTailNode | *V17ArrayOmitTailNode
	Suffixes []interface{}
}

// ParseArrayFinal parses array_final = (TYPE_OF array_list<ident_ref> | array_list) { array_append_tail | array_omit_tail }.
func (p *V17Parser) ParseArrayFinal() (node *V17ArrayFinalNode, err error) {
	done := p.debugEnter("array_final")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	var base interface{}

	if p.peekAfterWS() == '[' {
		// array_list → array_uniform
		au, auerr := p.ParseArrayUniform()
		if auerr != nil {
			return nil, fmt.Errorf("array_final: %w", auerr)
		}
		base = au
	} else {
		// ident_ref based: try empty_array_typed first (ident_ref + "[]"), then bare ident_ref
		if saved := p.savePos(); true {
			if at, aterr := p.ParseEmptyArrayTyped(); aterr == nil {
				base = at
			} else {
				p.restorePos(saved)
			}
		}
		if base == nil {
			ref, rerr := p.ParseIdentRef()
			if rerr != nil {
				return nil, fmt.Errorf("array_final: %w", rerr)
			}
			base = ref
		}
	}

	// Optional suffix loop: { array_append_tail | array_omit_tail }
	var suffixes []interface{}
	for {
		if p.peekAfterWS() == '+' {
			saved := p.savePos()
			if at, aterr := p.ParseArrayAppendTail(); aterr == nil {
				suffixes = append(suffixes, at)
				continue
			}
			p.restorePos(saved)
			break
		}
		if p.peekAfterWS() == '-' {
			saved := p.savePos()
			if ot, oterr := p.ParseArrayOmitTail(); oterr == nil {
				suffixes = append(suffixes, ot)
				continue
			}
			p.restorePos(saved)
			break
		}
		break
	}

	return &V17ArrayFinalNode{V17BaseNode{line, col}, base, suffixes}, nil
}

// =============================================================================
// lookup_idx_expr = integer | num_expr_chain | TYPE_OF integer<ident_ref>
// =============================================================================

// V17LookupIdxExprNode  lookup_idx_expr = integer | num_expr_chain | TYPE_OF integer<ident_ref>
type V17LookupIdxExprNode struct {
	V17BaseNode
	// Value is one of: *V17NumExprChainNode | *V17IdentRefNode
	Value interface{}
}

// ParseLookupIdxExpr parses lookup_idx_expr = integer | num_expr_chain | TYPE_OF integer<ident_ref>.
// num_expr_chain covers the integer case internally; ident_ref handles the TYPE_OF form.
func (p *V17Parser) ParseLookupIdxExpr() (node *V17LookupIdxExprNode, err error) {
	done := p.debugEnter("lookup_idx_expr")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// num_expr_chain covers integer and more complex numeric expressions
	if saved := p.savePos(); true {
		if nc, ncerr := p.ParseNumExprChain(); ncerr == nil {
			return &V17LookupIdxExprNode{V17BaseNode{line, col}, nc}, nil
		}
		p.restorePos(saved)
	}
	// TYPE_OF integer<ident_ref> — bare ident_ref
	if saved := p.savePos(); true {
		if ref, referr := p.ParseIdentRef(); referr == nil {
			return &V17LookupIdxExprNode{V17BaseNode{line, col}, ref}, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("lookup_idx_expr: expected integer, num_expr_chain, or ident_ref")
}

// =============================================================================
// array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]" { "[" lookup_idx_expr "]" }
// =============================================================================

// V17ArrayLookupNode  array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]" { ... }
type V17ArrayLookupNode struct {
	V17BaseNode
	Ref     *V17IdentRefNode
	Indices []*V17LookupIdxExprNode
}

// ParseArrayLookup parses array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]" { ... }.
func (p *V17Parser) ParseArrayLookup() (node *V17ArrayLookupNode, err error) {
	done := p.debugEnter("array_lookup")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// TYPE_OF array_final<ident_ref> — parse ident_ref
	ref, referr := p.ParseIdentRef()
	if referr != nil {
		return nil, fmt.Errorf("array_lookup: %w", referr)
	}

	// Must have at least one "[" lookup_idx_expr "]" (not "[]")
	if !(p.peekAfterWS() == '[' && !p.peekLit("[]")) {
		return nil, p.errAt("array_lookup: expected non-empty '[...]' after ident_ref")
	}

	var indices []*V17LookupIdxExprNode
	for p.peekAfterWS() == '[' && !p.peekLit("[]") {
		if _, err = p.matchLit("["); err != nil {
			return nil, err
		}
		idx, idxerr := p.ParseLookupIdxExpr()
		if idxerr != nil {
			return nil, fmt.Errorf("array_lookup: %w", idxerr)
		}
		if _, err = p.matchLit("]"); err != nil {
			return nil, fmt.Errorf("array_lookup: %w", err)
		}
		indices = append(indices, idx)
	}

	if len(indices) == 0 {
		return nil, p.errAt("array_lookup: expected at least one subscript")
	}

	return &V17ArrayLookupNode{V17BaseNode{line, col}, ref, indices}, nil
}

// =============================================================================
// object_init = "[" assignment { "," assignment } [","] "]"
// =============================================================================

// V17ObjectInitNode  object_init = "[" assignment { "," assignment } [","] "]"
type V17ObjectInitNode struct {
	V17BaseNode
	Assignments   []*V17AssignmentNode
	TrailingComma bool
}

// ParseObjectInit parses object_init = "[" assignment { "," assignment } [","] "]".
func (p *V17Parser) ParseObjectInit() (node *V17ObjectInitNode, err error) {
	done := p.debugEnter("object_init")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() != '[' {
		return nil, p.errAt("object_init: expected '['")
	}
	if _, err = p.matchLit("["); err != nil {
		return nil, err
	}

	var assignments []*V17AssignmentNode
	trailingComma := false

	if p.peekAfterWS() != ']' {
		first, ferr := p.ParseAssignment()
		if ferr != nil {
			return nil, fmt.Errorf("object_init: %w", ferr)
		}
		assignments = append(assignments, first)

		for p.peekAfterWS() == ',' {
			saved := p.savePos()
			if _, err = p.matchLit(","); err != nil {
				p.restorePos(saved)
				break
			}
			p.skipNLAndComments()
			if p.peekAfterWS() == ']' {
				trailingComma = true
				break
			}
			a, aerr := p.ParseAssignment()
			if aerr != nil {
				p.restorePos(saved) // backtrack comma
				break
			}
			assignments = append(assignments, a)
		}
	}

	if _, err = p.matchLit("]"); err != nil {
		return nil, fmt.Errorf("object_init: %w", err)
	}

	return &V17ObjectInitNode{V17BaseNode{line, col}, assignments, trailingComma}, nil
}

// =============================================================================
// object_merge_tail = "+" ( TYPE_OF object_init<ident_ref> | object_init ) { "+" ... }
// =============================================================================

// V17ObjectMergeTailNode  object_merge_tail = "+" (TYPE_OF object_init<ident_ref> | object_init) { "+" ... }
type V17ObjectMergeTailNode struct {
	V17BaseNode
	// Items is a slice of *V17IdentRefNode | *V17ObjectInitNode (each was preceded by "+")
	Items []interface{}
}

// ParseObjectMergeTail parses object_merge_tail = "+" (TYPE_OF object_init<ident_ref> | object_init) { "+" ... }.
func (p *V17Parser) ParseObjectMergeTail() (node *V17ObjectMergeTailNode, err error) {
	done := p.debugEnter("object_merge_tail")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() != '+' {
		return nil, p.errAt("object_merge_tail: expected '+'")
	}
	if _, err = p.matchLit("+"); err != nil {
		return nil, err
	}

	var items []interface{}
	first, firstErr := p.parseObjectOrRef()
	if firstErr != nil {
		return nil, fmt.Errorf("object_merge_tail: %w", firstErr)
	}
	items = append(items, first)

	for p.peekAfterWS() == '+' {
		saved := p.savePos()
		if _, err = p.matchLit("+"); err != nil {
			p.restorePos(saved)
			break
		}
		next, nexterr := p.parseObjectOrRef()
		if nexterr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	return &V17ObjectMergeTailNode{V17BaseNode{line, col}, items}, nil
}

// parseObjectOrRef parses ( TYPE_OF object_init<ident_ref> | object_init ).
// If current token is "[", parse object_init; otherwise parse ident_ref.
func (p *V17Parser) parseObjectOrRef() (interface{}, error) {
	if p.peekAfterWS() == '[' {
		return p.ParseObjectInit()
	}
	return p.ParseIdentRef()
}

// =============================================================================
// object_omit_tail = "-" ( ident_name | lookup_idx_expr ) { "," ( ident_name | lookup_idx_expr ) }
// =============================================================================

// V17ObjectOmitTailNode  object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," ... }
type V17ObjectOmitTailNode struct {
	V17BaseNode
	// Items is a slice of *V17IdentNameNode | *V17LookupIdxExprNode
	Items []interface{}
}

// ParseObjectOmitTail parses object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," ... }.
func (p *V17Parser) ParseObjectOmitTail() (node *V17ObjectOmitTailNode, err error) {
	done := p.debugEnter("object_omit_tail")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if p.peekAfterWS() != '-' {
		return nil, p.errAt("object_omit_tail: expected '-'")
	}
	if _, err = p.matchLit("-"); err != nil {
		return nil, err
	}

	var items []interface{}
	first, firstErr := p.parseNameOrIdx()
	if firstErr != nil {
		return nil, fmt.Errorf("object_omit_tail: %w", firstErr)
	}
	items = append(items, first)

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		next, nexterr := p.parseNameOrIdx()
		if nexterr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	return &V17ObjectOmitTailNode{V17BaseNode{line, col}, items}, nil
}

// parseNameOrIdx parses ( ident_name | lookup_idx_expr ).
// Tries ident_name first (simpler rule), then lookup_idx_expr.
func (p *V17Parser) parseNameOrIdx() (interface{}, error) {
	if saved := p.savePos(); true {
		if n, nerr := p.ParseIdentName(); nerr == nil {
			return n, nil
		}
		p.restorePos(saved)
	}
	return p.ParseLookupIdxExpr()
}

// =============================================================================
// object_merge_or_omit = ( TYPE_OF object_init<ident_ref> | object_init )
//                        ( object_merge_tail | object_omit_tail )
// =============================================================================

// V17ObjectMergeOrOmitNode  object_merge_or_omit = base + (merge_tail | omit_tail)
type V17ObjectMergeOrOmitNode struct {
	V17BaseNode
	// Base is *V17IdentRefNode | *V17ObjectInitNode
	Base interface{}
	// Tail is *V17ObjectMergeTailNode | *V17ObjectOmitTailNode
	Tail interface{}
}

// ParseObjectMergeOrOmit parses object_merge_or_omit = (TYPE_OF object_init<ident_ref> | object_init) (merge_tail | omit_tail).
func (p *V17Parser) ParseObjectMergeOrOmit() (node *V17ObjectMergeOrOmitNode, err error) {
	done := p.debugEnter("object_merge_or_omit")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// base: TYPE_OF object_init<ident_ref> | object_init
	base, baseErr := p.parseObjectOrRef()
	if baseErr != nil {
		return nil, fmt.Errorf("object_merge_or_omit: %w", baseErr)
	}

	// Required: object_merge_tail | object_omit_tail
	var tail interface{}
	if p.peekAfterWS() == '+' {
		mt, mterr := p.ParseObjectMergeTail()
		if mterr != nil {
			return nil, fmt.Errorf("object_merge_or_omit: %w", mterr)
		}
		tail = mt
	} else if p.peekAfterWS() == '-' {
		ot, oterr := p.ParseObjectOmitTail()
		if oterr != nil {
			return nil, fmt.Errorf("object_merge_or_omit: %w", oterr)
		}
		tail = ot
	} else {
		return nil, p.errAt("object_merge_or_omit: expected '+' (merge) or '-' (omit) tail")
	}

	return &V17ObjectMergeOrOmitNode{V17BaseNode{line, col}, base, tail}, nil
}

// =============================================================================
// object_final = object_init | object_merge_or_omit | empty_array_typed
// =============================================================================

// V17ObjectFinalNode  object_final = object_init | object_merge_or_omit | empty_array_typed
type V17ObjectFinalNode struct {
	V17BaseNode
	// Value is one of: *V17ObjectInitNode | *V17ObjectMergeOrOmitNode | *V17EmptyArrayTypedNode
	Value interface{}
}

// ParseObjectFinal parses object_final = object_init | object_merge_or_omit | empty_array_typed.
// empty_array_typed (ident_ref + "[]") is tried before object_merge_or_omit (ident_ref + suffix)
// to avoid consuming the ident_ref of a typed-empty-array as the base of a merge/omit.
func (p *V17Parser) ParseObjectFinal() (node *V17ObjectFinalNode, err error) {
	done := p.debugEnter("object_final")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// 1. empty_array_typed — ident_ref + "[]"
	if saved := p.savePos(); true {
		if at, aterr := p.ParseEmptyArrayTyped(); aterr == nil {
			return &V17ObjectFinalNode{V17BaseNode{line, col}, at}, nil
		}
		p.restorePos(saved)
	}
	// 2. object_merge_or_omit — requires a merge/omit tail after base
	if saved := p.savePos(); true {
		if mo, moerr := p.ParseObjectMergeOrOmit(); moerr == nil {
			return &V17ObjectFinalNode{V17BaseNode{line, col}, mo}, nil
		}
		p.restorePos(saved)
	}
	// 3. object_init — "[" assignment... "]"
	if saved := p.savePos(); true {
		if oi, oierr := p.ParseObjectInit(); oierr == nil {
			return &V17ObjectFinalNode{V17BaseNode{line, col}, oi}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("object_final: expected object_init, object_merge_or_omit, or empty_array_typed")
}

// =============================================================================
// lookup_txt_expr = string | string_expr_chain | TYPE_OF string<ident_ref>
// =============================================================================

// V17LookupTxtExprNode  lookup_txt_expr = string | string_expr_chain | TYPE_OF string<ident_ref>
type V17LookupTxtExprNode struct {
	V17BaseNode
	// Value is one of: *V17StringExprChainNode | *V17IdentRefNode
	Value interface{}
}

// ParseLookupTxtExpr parses lookup_txt_expr = string | string_expr_chain | TYPE_OF string<ident_ref>.
// string_expr_chain covers the plain string case (degenerate chain of one element).
func (p *V17Parser) ParseLookupTxtExpr() (node *V17LookupTxtExprNode, err error) {
	done := p.debugEnter("lookup_txt_expr")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// string_expr_chain = string { string_oper string } — covers bare string too
	if saved := p.savePos(); true {
		if sc, scerr := p.ParseStringExprChain(); scerr == nil {
			return &V17LookupTxtExprNode{V17BaseNode{line, col}, sc}, nil
		}
		p.restorePos(saved)
	}
	// TYPE_OF string<ident_ref> — bare ident_ref
	if saved := p.savePos(); true {
		if ref, referr := p.ParseIdentRef(); referr == nil {
			return &V17LookupTxtExprNode{V17BaseNode{line, col}, ref}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("lookup_txt_expr: expected string, string_expr_chain, or ident_ref")
}

// =============================================================================
// object_lookup = TYPE_OF object_final<ident_ref> "[" lookup_txt_expr "]" { "[" lookup_txt_expr "]" }
// =============================================================================

// V17ObjectLookupNode  object_lookup = TYPE_OF object_final<ident_ref> "[" lookup_txt_expr "]" { ... }
type V17ObjectLookupNode struct {
	V17BaseNode
	Ref  *V17IdentRefNode
	Keys []*V17LookupTxtExprNode
}

// ParseObjectLookup parses object_lookup = TYPE_OF object_final<ident_ref> "[" lookup_txt_expr "]" { ... }.
func (p *V17Parser) ParseObjectLookup() (node *V17ObjectLookupNode, err error) {
	done := p.debugEnter("object_lookup")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// TYPE_OF object_final<ident_ref> — parse ident_ref
	ref, referr := p.ParseIdentRef()
	if referr != nil {
		return nil, fmt.Errorf("object_lookup: %w", referr)
	}

	// Must have at least one "[" lookup_txt_expr "]" (not "[]")
	if !(p.peekAfterWS() == '[' && !p.peekLit("[]")) {
		return nil, p.errAt("object_lookup: expected non-empty '[...]' after ident_ref")
	}

	var keys []*V17LookupTxtExprNode
	for p.peekAfterWS() == '[' && !p.peekLit("[]") {
		if _, err = p.matchLit("["); err != nil {
			return nil, err
		}
		key, keyerr := p.ParseLookupTxtExpr()
		if keyerr != nil {
			return nil, fmt.Errorf("object_lookup: %w", keyerr)
		}
		if _, err = p.matchLit("]"); err != nil {
			return nil, fmt.Errorf("object_lookup: %w", err)
		}
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return nil, p.errAt("object_lookup: expected at least one subscript")
	}

	return &V17ObjectLookupNode{V17BaseNode{line, col}, ref, keys}, nil
}

// =============================================================================
// collection = range | array_final
// =============================================================================

// V17CollectionNode  collection = range | array_final
// EXTEND<collection> = | date_range | time_range  (08_range.sqg)
type V17CollectionNode struct {
	V17BaseNode
	// Value is one of: *V17RangeNode | *V17ArrayFinalNode |
	//                  *V17DateRangeNode | *V17TimeRangeNode
	Value interface{}
}

// ParseCollection parses collection = range | array_final.
// EXTEND<collection> = | date_range | time_range  (08_range.sqg)
func (p *V17Parser) ParseCollection() (node *V17CollectionNode, err error) {
	done := p.debugEnter("collection")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// range = integer ".." integer — try first (more specific when two integers appear)
	if saved := p.savePos(); true {
		if r, rerr := p.ParseRange(); rerr == nil {
			return &V17CollectionNode{V17BaseNode{line, col}, r}, nil
		}
		p.restorePos(saved)
	}
	// EXTEND<collection> = | date_range | time_range  (08_range.sqg)
	if saved := p.savePos(); true {
		if dr, drerr := p.ParseDateRange(); drerr == nil {
			return &V17CollectionNode{V17BaseNode{line, col}, dr}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if tr, trerr := p.ParseTimeRange(); trerr == nil {
			return &V17CollectionNode{V17BaseNode{line, col}, tr}, nil
		}
		p.restorePos(saved)
	}
	// array_final — general collection
	if saved := p.savePos(); true {
		if af, aferr := p.ParseArrayFinal(); aferr == nil {
			return &V17CollectionNode{V17BaseNode{line, col}, af}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("collection: expected range, date_range, time_range or array_final")
}

// Ensure fmt is used (referenced by error-wrapping in every Parse method).
var _ = fmt.Sprintf
