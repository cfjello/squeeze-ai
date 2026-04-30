// parser_v12_objects.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V12 grammar rule set defined in spec/04_objects.sqg.
//
// Covered rules:
//
//	empty_decl, empty_array_decl, func_stream_decl, func_regexp_decl,
//	empty_scope_decl, func_string_decl
//	empty_array_typed (V12 new)
//	array_value, values_list, array_uniform, array_list,
//	array_append_tail, array_omit_tail, array_final,
//	lookup_idx_expr, array_lookup
//	object_init, object_merge_tail, object_omit_tail,
//	object_merge_or_omit, object_final, object_lookup
//	table_header, table_row, table_objects, table_init,
//	table_ins_tail, table_final
//	EXTEND<assign_rhs>
package parser

import "fmt"

// =============================================================================
// PHASE 2 — AST NODE TYPES  (04_objects.sqg)
// =============================================================================

// V12EmptyDeclKind classifies the empty declaration token.
type V12EmptyDeclKind int

const (
	V12EmptyArray   V12EmptyDeclKind = iota // []   empty_array_decl
	V12EmptyStream                          // >>   func_stream_decl
	V12EmptyRegexp                          // //   func_regexp_decl
	V12EmptyScope                           // {}   empty_scope_decl
	V12EmptyStringD                         // ""
	V12EmptyStringS                         // ''
	V12EmptyStringT                         // ``
)

// V12EmptyDeclNode  empty_decl = empty_array_decl | func_stream_decl | func_regexp_decl | empty_scope_decl | func_string_decl
type V12EmptyDeclNode struct {
	V12BaseNode
	Kind V12EmptyDeclKind
}

// V12TypeOfRefNode  TYPE_OF typeName<ident_ref>
//
// e.g. TYPE_OF array_list<myArr>
type V12TypeOfRefNode struct {
	V12BaseNode
	TypeName string // grammar-rule name used as type annotation
	Ref      *V12IdentRefNode
}

// V12ArrayValueNode  array_value = constant | range | ident_ref | calc_unit | array_final | object_final
type V12ArrayValueNode struct {
	V12BaseNode
	Value V12Node // see above alternates
}

// V12ArrayUniformNode  array_uniform = "[" UNIFORM INFER<( values_list | array_uniform )> [","] "]"
type V12ArrayUniformNode struct {
	V12BaseNode
	Items []V12Node // *V12ArrayValueNode or *V12ArrayUniformNode (nested)
}

// V12ArrayAppendTailNode  array_append_tail = "+" array_uniform { "+" array_uniform }
type V12ArrayAppendTailNode struct {
	V12BaseNode
	Arrays []*V12ArrayUniformNode
}

// V12ArrayOmitTailNode  array_omit_tail = "-" integer { "," integer }
type V12ArrayOmitTailNode struct {
	V12BaseNode
	Indices []*V12IntegerNode
}

// V12EmptyArrayTypedNode  empty_array_typed = ident_ref empty_array_decl
// e.g.  MyType[]
type V12EmptyArrayTypedNode struct {
	V12BaseNode
	Ref *V12IdentRefNode // the ident_ref naming the element type
}

// V12ArrayFinalNode
//
//	array_final = ( TYPE_OF array_list<ident_ref> | array_list ) { array_append_tail | array_omit_tail }
//	array_list  = array_uniform | empty_array_typed  (V12: added empty_array_typed)
type V12ArrayFinalNode struct {
	V12BaseNode
	TypeRef *V12TypeOfRefNode // non-nil when TYPE_OF form
	List    V12Node           // *V12ArrayUniformNode | *V12EmptyArrayTypedNode | *V12EmptyDeclNode (nil when TypeRef set)
	Tails   []V12Node         // *V12ArrayAppendTailNode | *V12ArrayOmitTailNode
}

// V12LookupIdxExprNode
//
//	lookup_idx_expr = integer | numeric_expr | string | string_expr
//	                | TYPE_OF integer<ident_ref> | TYPE_OF string<ident_ref>
type V12LookupIdxExprNode struct {
	V12BaseNode
	Value V12Node
}

// V12ArrayLookupNode  array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]"
type V12ArrayLookupNode struct {
	V12BaseNode
	Array *V12TypeOfRefNode
	Index *V12LookupIdxExprNode
}

// V12ObjectEntryNode  a single key-value pair inside object_init
type V12ObjectEntryNode struct {
	V12BaseNode
	LHS   *V12AssignLHSNode
	Oper  *V12AssignOperNode
	Value *V12ArrayValueNode
}

// V12ObjectInitNode
//
//	object_init = "[" UNIFORM INFER<assign_lhs assign_oper array_value { "," ... }> [","] "]"
type V12ObjectInitNode struct {
	V12BaseNode
	Entries []V12ObjectEntryNode
}

// V12ObjectMergeTailNode
//
//	object_merge_tail = "+" ( TYPE_OF object_init<ident_ref> | object_init ) { "+" ... }
type V12ObjectMergeTailNode struct {
	V12BaseNode
	Items []V12Node // *V12TypeOfRefNode | *V12ObjectInitNode
}

// V12ObjectOmitTailNode
//
//	object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," ... }
type V12ObjectOmitTailNode struct {
	V12BaseNode
	Items []V12Node // *V12IdentDottedNode | *V12LookupIdxExprNode
}

// V12ObjectMergeOrOmitNode
//
//	object_merge_or_omit = UNIFORM INFER<
//	    ( TYPE_OF object_init<ident_ref> | object_init )
//	    ( object_merge_tail | object_omit_tail )
//	>
type V12ObjectMergeOrOmitNode struct {
	V12BaseNode
	Base     V12Node // *V12TypeOfRefNode | *V12ObjectInitNode
	Modifier V12Node // *V12ObjectMergeTailNode | *V12ObjectOmitTailNode
}

// V12ObjectFinalNode  object_final = object_init | object_merge_or_omit | empty_array_typed  (V12)
type V12ObjectFinalNode struct {
	V12BaseNode
	Value V12Node // *V12ObjectInitNode | *V12ObjectMergeOrOmitNode | *V12EmptyArrayTypedNode
}

// V12ObjectLookupNode  object_lookup = TYPE_OF object_final<ident_ref> [ "[" lookup_idx_expr "]" ]
type V12ObjectLookupNode struct {
	V12BaseNode
	Object *V12TypeOfRefNode
	Index  *V12LookupIdxExprNode // may be nil
}

// V12TableHeaderNode  table_header = UNIFORM string<( array_uniform | TYPE_OF array_uniform<ident_ref> )>
type V12TableHeaderNode struct {
	V12BaseNode
	Value V12Node // *V12ArrayUniformNode | *V12TypeOfRefNode
}

// V12TableObjectsNode  table_objects = UNIFORM INFER<table_row { "," table_row }>
type V12TableObjectsNode struct {
	V12BaseNode
	Rows []V12Node // *V12ArrayFinalNode | *V12ObjectFinalNode
}

// V12TableInitNode  table_init = table_header table_objects | object_final
type V12TableInitNode struct {
	V12BaseNode
	Header  *V12TableHeaderNode  // non-nil when header+objects form
	Objects *V12TableObjectsNode // non-nil when header+objects form
	AltObj  *V12ObjectFinalNode  // non-nil when object_final-only form
}

// V12TableInsTailNode
//
//	table_ins_tail = "+" ( TYPE_OF table_row<ident_ref> | table_row ) { "+" ... }
type V12TableInsTailNode struct {
	V12BaseNode
	Items []V12Node // *V12TypeOfRefNode | *V12ArrayFinalNode | *V12ObjectFinalNode
}

// V12TableFinalNode
//
//	table_final = ( TYPE_OF table_init<ident_ref> | table_init ) table_ins_tail { table_ins_tail }
type V12TableFinalNode struct {
	V12BaseNode
	Base  V12Node // *V12TypeOfRefNode | *V12TableInitNode
	Tails []*V12TableInsTailNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (04_objects.sqg)
// =============================================================================

// V12isAssignOp returns true when the token type is one of the assignment
// operators, used to disambiguate object_init entries from plain ident_refs
// when both appear inside "[UNIFORM INFER<...>]".
func V12isAssignOp(t V12TokenType) bool {
	switch t {
	case V12_EQ, V12_COLON, V12_READONLY,
		V12_IADD_IMM, V12_ISUB_IMM, V12_IMUL_IMM, V12_IDIV_IMM,
		V12_EXTEND_ASSIGN, V12_ISUB_ASSIGN, V12_IMUL_ASSIGN, V12_IDIV_ASSIGN:
		return true
	}
	return false
}

// ---------- empty_decl ----------

// ParseEmptyDecl parses:
//
//	empty_decl = "[]" | ">>" | "//" | "{}" | '""' | "''" | "``"
func (p *V12Parser) ParseEmptyDecl() (*V12EmptyDeclNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var kind V12EmptyDeclKind
	switch tok.Type {
	case V12_EMPTY_ARR:
		kind = V12EmptyArray
	case V12_STREAM:
		kind = V12EmptyStream
	case V12_REGEXP_DECL:
		kind = V12EmptyRegexp
	case V12_EMPTY_OBJ:
		kind = V12EmptyScope
	case V12_EMPTY_STR_D:
		kind = V12EmptyStringD
	case V12_EMPTY_STR_S:
		kind = V12EmptyStringS
	case V12_EMPTY_STR_T:
		kind = V12EmptyStringT
	default:
		return nil, p.errAt(fmt.Sprintf("expected empty declaration ([], >>, //, {}, \"\", '', ``), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V12EmptyDeclNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Kind: kind}, nil
}

// ---------- TYPE_OF reference ----------

// parseTypeOfRef parses the pattern:  TYPE_OF typeName<ident_ref>
//
// typeName is an identifier naming a grammar rule (e.g. "array_list",
// "integer", "string", "object_init").  It is stored verbatim.
func (p *V12Parser) parseTypeOfRef() (*V12TypeOfRefNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_TYPE_OF); err != nil {
		return nil, err
	}
	if p.cur().Type != V12_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected type name after TYPE_OF, got %s", p.cur().Type))
	}
	typeName := p.cur().Value
	p.advance()
	if _, err := p.expect(V12_LT); err != nil {
		return nil, err
	}
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_GT); err != nil {
		return nil, err
	}
	return &V12TypeOfRefNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, TypeName: typeName, Ref: ref}, nil
}

// ---------- array_uniform / array_list ----------

// ParseArrayUniform parses:
//
//	array_uniform = "[" UNIFORM INFER<( values_list | array_uniform )> [","] "]"
func (p *V12Parser) ParseArrayUniform() (*V12ArrayUniformNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_LT); err != nil {
		return nil, err
	}

	var items []V12Node
	// Alternative: nested array_uniform or values_list.
	if p.cur().Type == V12_LBRACKET {
		inner, err := p.ParseArrayUniform()
		if err != nil {
			return nil, err
		}
		items = append(items, inner)
	} else {
		list, err := p.parseValuesList()
		if err != nil {
			return nil, err
		}
		items = list
	}

	// Optional trailing comma before ">".
	if p.cur().Type == V12_COMMA {
		p.advance()
	}
	if _, err := p.expect(V12_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_RBRACKET); err != nil {
		return nil, err
	}
	return &V12ArrayUniformNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Items: items}, nil
}

// parseValuesList parses:  values_list = array_value { "," array_value }
func (p *V12Parser) parseValuesList() ([]V12Node, error) {
	val, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	items := []V12Node{val}

	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance() // consume ","
		next, err := p.ParseArrayValue()
		if err != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}
	return items, nil
}

// ParseEmptyArrayTyped parses:  empty_array_typed = ident_ref empty_array_decl
// e.g.  MyType[]
func (p *V12Parser) ParseEmptyArrayTyped() (*V12EmptyArrayTypedNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_EMPTY_ARR); err != nil {
		return nil, err
	}
	return &V12EmptyArrayTypedNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Ref: ref}, nil
}

// ParseArrayList parses:  array_list = array_uniform | empty_array_typed  (V12)
func (p *V12Parser) ParseArrayList() (V12Node, error) {
	// empty_array_typed = ident_ref "[]" — starts with IDENT followed by EMPTY_ARR
	if p.cur().Type == V12_IDENT {
		saved := p.savePos()
		if eat, err := p.ParseEmptyArrayTyped(); err == nil {
			return eat, nil
		}
		p.restorePos(saved)
	}
	return p.ParseArrayUniform()
}

// ---------- array_final ----------

// ParseArrayAppendTail parses:  array_append_tail = "+" array_uniform { "+" array_uniform }
func (p *V12Parser) ParseArrayAppendTail() (*V12ArrayAppendTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_PLUS); err != nil {
		return nil, err
	}
	first, err := p.ParseArrayUniform()
	if err != nil {
		return nil, err
	}
	node := &V12ArrayAppendTailNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Arrays:      []*V12ArrayUniformNode{first},
	}
	for p.cur().Type == V12_PLUS {
		saved := p.savePos()
		p.advance()
		au, err := p.ParseArrayUniform()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Arrays = append(node.Arrays, au)
	}
	return node, nil
}

// ParseArrayOmitTail parses:  array_omit_tail = "-" integer { "," integer }
func (p *V12Parser) ParseArrayOmitTail() (*V12ArrayOmitTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_MINUS); err != nil {
		return nil, err
	}
	first, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	node := &V12ArrayOmitTailNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Indices:     []*V12IntegerNode{first},
	}
	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		idx, err := p.ParseInteger()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Indices = append(node.Indices, idx)
	}
	return node, nil
}

// ParseArrayFinal parses:
//
//	array_final = ( TYPE_OF array_list<ident_ref> | array_list ) { array_append_tail | array_omit_tail }
func (p *V12Parser) ParseArrayFinal() (*V12ArrayFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V12ArrayFinalNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}

	if p.cur().Type == V12_TYPE_OF {
		ref, err := p.parseTypeOfRef()
		if err != nil {
			return nil, err
		}
		node.TypeRef = ref
	} else {
		list, err := p.ParseArrayList()
		if err != nil {
			return nil, err
		}
		node.List = list
	}

	// Collect optional tails: each "+" means append, each "-" means omit.
	for p.cur().Type == V12_PLUS || p.cur().Type == V12_MINUS {
		saved := p.savePos()
		if p.cur().Type == V12_PLUS {
			tail, err := p.ParseArrayAppendTail()
			if err != nil {
				p.restorePos(saved)
				break
			}
			node.Tails = append(node.Tails, tail)
		} else {
			tail, err := p.ParseArrayOmitTail()
			if err != nil {
				p.restorePos(saved)
				break
			}
			node.Tails = append(node.Tails, tail)
		}
	}
	return node, nil
}

// ---------- lookup_idx_expr / array_lookup ----------

// ParseLookupIdxExpr parses:
//
//	lookup_idx_expr = integer | numeric_expr | string | string_expr
//	                | TYPE_OF integer<ident_ref> | TYPE_OF string<ident_ref>
func (p *V12Parser) ParseLookupIdxExpr() (*V12LookupIdxExprNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V12_TYPE_OF {
		ref, err := p.parseTypeOfRef()
		if err != nil {
			return nil, err
		}
		return &V12LookupIdxExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: ref}, nil
	}

	if p.cur().Type == V12_INTEGER {
		saved := p.savePos()
		// Try scalar integer first; if an arithmetic op follows, parse it as numeric_expr.
		if integer, err := p.ParseInteger(); err == nil {
			if !V12isExprContinuation(p.cur().Type) {
				return &V12LookupIdxExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: integer}, nil
			}
		}
		p.restorePos(saved)
		ne, err := p.ParseNumericExpr()
		if err != nil {
			return nil, err
		}
		return &V12LookupIdxExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: ne}, nil
	}

	if V12isStringTok(p.cur().Type) {
		saved := p.savePos()
		if str, err := p.ParseString(); err == nil {
			if p.cur().Type != V12_PLUS { // not a concat expr
				return &V12LookupIdxExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: str}, nil
			}
		}
		p.restorePos(saved)
		se, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		return &V12LookupIdxExprNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: se}, nil
	}

	return nil, p.errAt(fmt.Sprintf("expected lookup index expression, got %s %q", p.cur().Type, p.cur().Value))
}

// ParseArrayLookup parses:  array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]"
func (p *V12Parser) ParseArrayLookup() (*V12ArrayLookupNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_LBRACKET); err != nil {
		return nil, err
	}
	idx, err := p.ParseLookupIdxExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_RBRACKET); err != nil {
		return nil, err
	}
	return &V12ArrayLookupNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Array: ref, Index: idx}, nil
}

// ---------- object_init ----------

// ParseObjectEntry parses a single  assign_lhs assign_oper array_value  inside object_init.
func (p *V12Parser) ParseObjectEntry() (*V12ObjectEntryNode, error) {
	line, col := p.cur().Line, p.cur().Col
	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}
	val, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	return &V12ObjectEntryNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, LHS: lhs, Oper: oper, Value: val}, nil
}

// ParseObjectInit parses:
//
//	object_init = "[" UNIFORM INFER<assign_lhs assign_oper array_value { "," ... }> [","] "]"
func (p *V12Parser) ParseObjectInit() (*V12ObjectInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_LT); err != nil {
		return nil, err
	}

	first, err := p.ParseObjectEntry()
	if err != nil {
		return nil, err
	}
	entries := []V12ObjectEntryNode{*first}

	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		// Stop if this comma is the trailing comma before ">".
		if p.cur().Type == V12_GT {
			break
		}
		e, err := p.ParseObjectEntry()
		if err != nil {
			p.restorePos(saved)
			break
		}
		entries = append(entries, *e)
	}

	// Optional trailing comma.
	if p.cur().Type == V12_COMMA {
		p.advance()
	}
	if _, err := p.expect(V12_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_RBRACKET); err != nil {
		return nil, err
	}
	return &V12ObjectInitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Entries: entries}, nil
}

// ParseSimpleObjectInit parses V1-style objects:  "[" assign_lhs assign_oper array_value { "," ... } [","] "]"
// Used as a fallback when UNIFORM INFER<...> is absent.
func (p *V12Parser) ParseSimpleObjectInit() (*V12ObjectInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_LBRACKET); err != nil {
		return nil, err
	}
	if p.cur().Type == V12_RBRACKET {
		// Empty object []
		p.advance()
		return &V12ObjectInitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}, nil
	}
	first, err := p.ParseObjectEntry()
	if err != nil {
		return nil, err
	}
	entries := []V12ObjectEntryNode{*first}
	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V12_RBRACKET {
			break // trailing comma
		}
		e, err := p.ParseObjectEntry()
		if err != nil {
			p.restorePos(saved)
			break
		}
		entries = append(entries, *e)
	}
	if _, err := p.expect(V12_RBRACKET); err != nil {
		return nil, err
	}
	return &V12ObjectInitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Entries: entries}, nil
}

// ParseObjectMergeTail parses:
//
//	object_merge_tail = "+" ( TYPE_OF object_init<ident_ref> | object_init ) { "+" ... }
func (p *V12Parser) ParseObjectMergeTail() (*V12ObjectMergeTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_PLUS); err != nil {
		return nil, err
	}
	item, err := p.parseObjectOrTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V12ObjectMergeTailNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Items:       []V12Node{item},
	}
	for p.cur().Type == V12_PLUS {
		saved := p.savePos()
		p.advance()
		next, err := p.parseObjectOrTypeOfRef()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, next)
	}
	return node, nil
}

// parseObjectOrTypeOfRef returns TYPE_OF object_init<ident_ref> or *V12ObjectInitNode.
func (p *V12Parser) parseObjectOrTypeOfRef() (V12Node, error) {
	if p.cur().Type == V12_TYPE_OF {
		return p.parseTypeOfRef()
	}
	return p.ParseObjectInit()
}

// ParseObjectOmitTail parses:
//
//	object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," ... }
func (p *V12Parser) ParseObjectOmitTail() (*V12ObjectOmitTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_MINUS); err != nil {
		return nil, err
	}
	first, err := p.parseIdentOrLookupIdx()
	if err != nil {
		return nil, err
	}
	node := &V12ObjectOmitTailNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Items:       []V12Node{first},
	}
	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		next, err := p.parseIdentOrLookupIdx()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, next)
	}
	return node, nil
}

// parseIdentOrLookupIdx returns ident_name (as *V12IdentDottedNode) or *V12LookupIdxExprNode.
func (p *V12Parser) parseIdentOrLookupIdx() (V12Node, error) {
	if p.cur().Type == V12_IDENT {
		return p.ParseIdentDotted()
	}
	return p.ParseLookupIdxExpr()
}

// ---------- object_merge_or_omit / object_final ----------

// ParseObjectMergeOrOmit parses:
//
//	object_merge_or_omit = UNIFORM INFER<
//	    ( TYPE_OF object_init<ident_ref> | object_init )
//	    ( object_merge_tail | object_omit_tail )
//	>
func (p *V12Parser) ParseObjectMergeOrOmit() (*V12ObjectMergeOrOmitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_LT); err != nil {
		return nil, err
	}

	base, err := p.parseObjectOrTypeOfRef()
	if err != nil {
		return nil, err
	}

	var modifier V12Node
	if p.cur().Type == V12_PLUS {
		modifier, err = p.ParseObjectMergeTail()
	} else if p.cur().Type == V12_MINUS {
		modifier, err = p.ParseObjectOmitTail()
	} else {
		err = p.errAt(fmt.Sprintf("expected '+' or '-' for object merge/omit, got %s", p.cur().Type))
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_GT); err != nil {
		return nil, err
	}
	return &V12ObjectMergeOrOmitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Base: base, Modifier: modifier}, nil
}

// ParseObjectFinal parses:  object_final = object_init | object_merge_or_omit | empty_array_typed  (V12)
func (p *V12Parser) ParseObjectFinal() (*V12ObjectFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col

	switch p.cur().Type {
	case V12_IDENT:
		// empty_array_typed = ident_ref "[]"  (V12)
		saved := p.savePos()
		if eat, err := p.ParseEmptyArrayTyped(); err == nil {
			return &V12ObjectFinalNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: eat}, nil
		}
		p.restorePos(saved)
		return nil, p.errAt(fmt.Sprintf("expected object, got %s", p.cur().Type))

	case V12_UNIFORM:
		moo, err := p.ParseObjectMergeOrOmit()
		if err != nil {
			return nil, err
		}
		return &V12ObjectFinalNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: moo}, nil

	case V12_LBRACKET:
		// V12 form: "[" UNIFORM INFER<...> "]"
		saved := p.savePos()
		if oi, err := p.ParseObjectInit(); err == nil {
			return &V12ObjectFinalNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: oi}, nil
		}
		p.restorePos(saved)
		// V1 fallback: "[" key ":" value { "," key ":" value } "]"
		if oi, err := p.ParseSimpleObjectInit(); err == nil {
			return &V12ObjectFinalNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: oi}, nil
		}
		p.restorePos(saved)
		return nil, p.errAt(fmt.Sprintf("expected object literal after '[', got %s", p.cur().Type))

	default:
		return nil, p.errAt(fmt.Sprintf("expected object ([UNIFORM INFER<...>], UNIFORM INFER<...>, or TypeName[]), got %s", p.cur().Type))
	}
}

// ParseObjectLookup parses:  object_lookup = TYPE_OF object_final<ident_ref> [ "[" lookup_idx_expr "]" ]
func (p *V12Parser) ParseObjectLookup() (*V12ObjectLookupNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V12ObjectLookupNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Object: ref}
	if p.cur().Type == V12_LBRACKET {
		p.advance()
		idx, err := p.ParseLookupIdxExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V12_RBRACKET); err != nil {
			return nil, err
		}
		node.Index = idx
	}
	return node, nil
}

// ---------- array_value ----------

// ParseArrayValue parses:
//
//	array_value = constant | range | ident_ref | calc_unit | array_final | object_final
//
// Disambiguation strategy:
//  1. "[]"       → array_final (empty_array_decl)
//  2. "["        → try object_final first (object_init is more restrictive), then array_final
//  3. UNIFORM    → object_final (object_merge_or_omit)
//  4. TYPE_OF    → try array_final, then object_final
//  5. INTEGER    → try range (N..N) then constant then calc_unit
//  6. otherwise  → try constant then calc_unit
func (p *V12Parser) ParseArrayValue() (*V12ArrayValueNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V12Node) *V12ArrayValueNode {
		return &V12ArrayValueNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V12_EMPTY_ARR:
		// empty_array_decl — surface as array_final for simplicity
		af, err := p.ParseArrayFinal()
		if err != nil {
			return nil, err
		}
		return wrap(af), nil

	case V12_LBRACKET:
		// Try object_final (object_init) before array_final (array_uniform);
		// both have the same "[UNIFORM INFER<..." prefix, so we backtrack.
		saved := p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return wrap(of), nil
		}
		p.restorePos(saved)
		af, err := p.ParseArrayFinal()
		if err != nil {
			return nil, err
		}
		return wrap(af), nil

	case V12_UNIFORM:
		// Only object_merge_or_omit starts with a bare UNIFORM at value position.
		of, err := p.ParseObjectFinal()
		if err != nil {
			return nil, err
		}
		return wrap(of), nil

	case V12_TYPE_OF:
		// Could be TYPE_OF array_list<ref> (array_final) or TYPE_OF object_init<ref>
		// (appears inside object_merge_or_omit, but also directly in array_value).
		saved := p.savePos()
		if af, err := p.ParseArrayFinal(); err == nil {
			return wrap(af), nil
		}
		p.restorePos(saved)
		of, err := p.ParseObjectFinal()
		if err != nil {
			return nil, err
		}
		return wrap(of), nil

	case V12_INTEGER:
		// Try range first (N..N), then constant, then calc_unit.
		saved := p.savePos()
		if rng, err := p.ParseRange(); err == nil {
			return wrap(rng), nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if c, err := p.ParseConstant(); err == nil && !V12isExprContinuation(p.cur().Type) {
			return wrap(c), nil
		}
		p.restorePos(saved)
		cu, err := p.ParseCalcUnit()
		if err != nil {
			return nil, err
		}
		return wrap(cu), nil

	case V12_DOLLAR:
		// self_ref "$"
		p.advance()
		return wrap(&V12SelfRefNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}), nil

	default:
		// Try constant first; fall back to calc_unit (covers ident_ref and expressions).
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil {
			return wrap(c), nil
		}
		p.restorePos(saved)
		cu, err := p.ParseCalcUnit()
		if err != nil {
			return nil, err
		}
		return wrap(cu), nil
	}
}

// ---------- table_header / table_objects / table_init ----------

// ParseTableHeader parses:
//
//	table_header = UNIFORM string<( array_uniform | TYPE_OF array_uniform<ident_ref> )>
//
// "string" here is a type-annotation identifier (not a literal), tokenised as V12_IDENT.
func (p *V12Parser) ParseTableHeader() (*V12TableHeaderNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_UNIFORM); err != nil {
		return nil, err
	}
	// Consume the "string" type-annotation identifier.
	tok := p.cur()
	if tok.Type != V12_IDENT || tok.Value != "string" {
		return nil, p.errAt(fmt.Sprintf("expected 'string' type annotation in table header, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	if _, err := p.expect(V12_LT); err != nil {
		return nil, err
	}
	var val V12Node
	var err error
	if p.cur().Type == V12_TYPE_OF {
		val, err = p.parseTypeOfRef()
	} else {
		val, err = p.ParseArrayUniform()
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_GT); err != nil {
		return nil, err
	}
	return &V12TableHeaderNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ParseTableRow parses:  table_row = array_final | object_final
func (p *V12Parser) ParseTableRow() (V12Node, error) {
	switch p.cur().Type {
	case V12_EMPTY_ARR:
		// Could be either; treat as array_final.
		return p.ParseArrayFinal()
	case V12_LBRACKET:
		// Try object_final first (more specific), then array_final.
		saved := p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return of, nil
		}
		p.restorePos(saved)
		return p.ParseArrayFinal()
	case V12_UNIFORM:
		return p.ParseObjectFinal()
	case V12_TYPE_OF:
		saved := p.savePos()
		if af, err := p.ParseArrayFinal(); err == nil {
			return af, nil
		}
		p.restorePos(saved)
		return p.ParseObjectFinal()
	default:
		return nil, p.errAt(fmt.Sprintf("expected table row (array or object), got %s", p.cur().Type))
	}
}

// ParseTableObjects parses:  table_objects = UNIFORM INFER<table_row { "," table_row }>
func (p *V12Parser) ParseTableObjects() (*V12TableObjectsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_LT); err != nil {
		return nil, err
	}
	first, err := p.ParseTableRow()
	if err != nil {
		return nil, err
	}
	rows := []V12Node{first}
	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V12_GT {
			break
		}
		row, err := p.ParseTableRow()
		if err != nil {
			p.restorePos(saved)
			break
		}
		rows = append(rows, row)
	}
	if _, err := p.expect(V12_GT); err != nil {
		return nil, err
	}
	return &V12TableObjectsNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Rows: rows}, nil
}

// ParseTableInit parses:  table_init = table_header table_objects | object_final
//
// Disambiguation: UNIFORM followed by IDENT("string") → table_header; all
// other starts → object_final.
func (p *V12Parser) ParseTableInit() (*V12TableInitNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V12_UNIFORM && p.peek(1).Type == V12_IDENT && p.peek(1).Value == "string" {
		hdr, err := p.ParseTableHeader()
		if err != nil {
			return nil, err
		}
		objs, err := p.ParseTableObjects()
		if err != nil {
			return nil, err
		}
		return &V12TableInitNode{
			V12BaseNode: V12BaseNode{Line: line, Col: col},
			Header:      hdr,
			Objects:     objs,
		}, nil
	}

	of, err := p.ParseObjectFinal()
	if err != nil {
		return nil, err
	}
	return &V12TableInitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, AltObj: of}, nil
}

// ---------- table_ins_tail / table_final ----------

// ParseTableInsTail parses:
//
//	table_ins_tail = "+" ( TYPE_OF table_row<ident_ref> | table_row ) { "+" ... }
func (p *V12Parser) ParseTableInsTail() (*V12TableInsTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_PLUS); err != nil {
		return nil, err
	}
	first, err := p.parseTypeOfRowOrRow()
	if err != nil {
		return nil, err
	}
	node := &V12TableInsTailNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Items:       []V12Node{first},
	}
	for p.cur().Type == V12_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := p.parseTypeOfRowOrRow()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, item)
	}
	return node, nil
}

// parseTypeOfRowOrRow: TYPE_OF table_row<ident_ref> | table_row
func (p *V12Parser) parseTypeOfRowOrRow() (V12Node, error) {
	if p.cur().Type == V12_TYPE_OF {
		return p.parseTypeOfRef()
	}
	return p.ParseTableRow()
}

// ParseTableFinal parses:
//
//	table_final = ( TYPE_OF table_init<ident_ref> | table_init ) table_ins_tail { table_ins_tail }
func (p *V12Parser) ParseTableFinal() (*V12TableFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col

	var base V12Node
	var err error
	if p.cur().Type == V12_TYPE_OF {
		base, err = p.parseTypeOfRef()
	} else {
		base, err = p.ParseTableInit()
	}
	if err != nil {
		return nil, err
	}

	// At least one table_ins_tail is required (table_final requires it).
	first, err := p.ParseTableInsTail()
	if err != nil {
		return nil, fmt.Errorf("table_final: expected at least one insert-tail (+row): %w", err)
	}
	tails := []*V12TableInsTailNode{first}
	for p.cur().Type == V12_PLUS {
		saved := p.savePos()
		tail, err := p.ParseTableInsTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}

	return &V12TableFinalNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Base:        base,
		Tails:       tails,
	}, nil
}
