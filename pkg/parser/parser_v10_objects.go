// parser_v10_objects.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V10 grammar rule set defined in spec/04_objects.sqg.
//
// Covered rules:
//
//	empty_decl, empty_array_decl, func_stream_decl, func_regexp_decl,
//	empty_scope_decl, func_string_decl
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

// V10EmptyDeclKind classifies the empty declaration token.
type V10EmptyDeclKind int

const (
	EmptyArray  V10EmptyDeclKind = iota // []   empty_array_decl
	EmptyStream                         // >>   func_stream_decl
	EmptyRegexp                         // //   func_regexp_decl
	EmptyScope                          // {}   empty_scope_decl
	EmptyStringD                        // ""
	EmptyStringS                        // ''
	EmptyStringT                        // ``
)

// V10EmptyDeclNode  empty_decl = empty_array_decl | func_stream_decl | func_regexp_decl | empty_scope_decl | func_string_decl
type V10EmptyDeclNode struct {
	v10BaseNode
	Kind V10EmptyDeclKind
}

// V10TypeOfRefNode  TYPE_OF typeName<ident_ref>
//
// e.g. TYPE_OF array_list<myArr>
type V10TypeOfRefNode struct {
	v10BaseNode
	TypeName string        // grammar-rule name used as type annotation
	Ref      *V10IdentRefNode
}

// V10ArrayValueNode  array_value = constant | range | ident_ref | calc_unit | array_final | object_final
type V10ArrayValueNode struct {
	v10BaseNode
	Value V10Node // see above alternates
}

// V10ArrayUniformNode  array_uniform = "[" UNIFORM INFER<( values_list | array_uniform )> [","] "]"
type V10ArrayUniformNode struct {
	v10BaseNode
	Items []V10Node // *V10ArrayValueNode or *V10ArrayUniformNode (nested)
}

// V10ArrayAppendTailNode  array_append_tail = "+" array_uniform { "+" array_uniform }
type V10ArrayAppendTailNode struct {
	v10BaseNode
	Arrays []*V10ArrayUniformNode
}

// V10ArrayOmitTailNode  array_omit_tail = "-" integer { "," integer }
type V10ArrayOmitTailNode struct {
	v10BaseNode
	Indices []*V10IntegerNode
}

// V10ArrayFinalNode
//
//	array_final = ( TYPE_OF array_list<ident_ref> | array_list ) { array_append_tail | array_omit_tail }
type V10ArrayFinalNode struct {
	v10BaseNode
	TypeRef *V10TypeOfRefNode // non-nil when TYPE_OF form
	List    V10Node           // *V10ArrayUniformNode | *V10EmptyDeclNode (nil when TypeRef set)
	Tails   []V10Node         // *V10ArrayAppendTailNode | *V10ArrayOmitTailNode
}

// V10LookupIdxExprNode
//
//	lookup_idx_expr = integer | numeric_expr | string | string_expr
//	                | TYPE_OF integer<ident_ref> | TYPE_OF string<ident_ref>
type V10LookupIdxExprNode struct {
	v10BaseNode
	Value V10Node
}

// V10ArrayLookupNode  array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]"
type V10ArrayLookupNode struct {
	v10BaseNode
	Array *V10TypeOfRefNode
	Index *V10LookupIdxExprNode
}

// V10ObjectEntryNode  a single key-value pair inside object_init
type V10ObjectEntryNode struct {
	v10BaseNode
	LHS   *V10AssignLHSNode
	Oper  *V10AssignOperNode
	Value *V10ArrayValueNode
}

// V10ObjectInitNode
//
//	object_init = "[" UNIFORM INFER<assign_lhs assign_oper array_value { "," ... }> [","] "]"
type V10ObjectInitNode struct {
	v10BaseNode
	Entries []V10ObjectEntryNode
}

// V10ObjectMergeTailNode
//
//	object_merge_tail = "+" ( TYPE_OF object_init<ident_ref> | object_init ) { "+" ... }
type V10ObjectMergeTailNode struct {
	v10BaseNode
	Items []V10Node // *V10TypeOfRefNode | *V10ObjectInitNode
}

// V10ObjectOmitTailNode
//
//	object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," ... }
type V10ObjectOmitTailNode struct {
	v10BaseNode
	Items []V10Node // *V10IdentDottedNode | *V10LookupIdxExprNode
}

// V10ObjectMergeOrOmitNode
//
//	object_merge_or_omit = UNIFORM INFER<
//	    ( TYPE_OF object_init<ident_ref> | object_init )
//	    ( object_merge_tail | object_omit_tail )
//	>
type V10ObjectMergeOrOmitNode struct {
	v10BaseNode
	Base     V10Node // *V10TypeOfRefNode | *V10ObjectInitNode
	Modifier V10Node // *V10ObjectMergeTailNode | *V10ObjectOmitTailNode
}

// V10ObjectFinalNode  object_final = object_init | object_merge_or_omit | empty_array_decl
type V10ObjectFinalNode struct {
	v10BaseNode
	Value V10Node // *V10ObjectInitNode | *V10ObjectMergeOrOmitNode | *V10EmptyDeclNode
}

// V10ObjectLookupNode  object_lookup = TYPE_OF object_final<ident_ref> [ "[" lookup_idx_expr "]" ]
type V10ObjectLookupNode struct {
	v10BaseNode
	Object *V10TypeOfRefNode
	Index  *V10LookupIdxExprNode // may be nil
}

// V10TableHeaderNode  table_header = UNIFORM string<( array_uniform | TYPE_OF array_uniform<ident_ref> )>
type V10TableHeaderNode struct {
	v10BaseNode
	Value V10Node // *V10ArrayUniformNode | *V10TypeOfRefNode
}

// V10TableObjectsNode  table_objects = UNIFORM INFER<table_row { "," table_row }>
type V10TableObjectsNode struct {
	v10BaseNode
	Rows []V10Node // *V10ArrayFinalNode | *V10ObjectFinalNode
}

// V10TableInitNode  table_init = table_header table_objects | object_final
type V10TableInitNode struct {
	v10BaseNode
	Header  *V10TableHeaderNode  // non-nil when header+objects form
	Objects *V10TableObjectsNode // non-nil when header+objects form
	AltObj  *V10ObjectFinalNode  // non-nil when object_final-only form
}

// V10TableInsTailNode
//
//	table_ins_tail = "+" ( TYPE_OF table_row<ident_ref> | table_row ) { "+" ... }
type V10TableInsTailNode struct {
	v10BaseNode
	Items []V10Node // *V10TypeOfRefNode | *V10ArrayFinalNode | *V10ObjectFinalNode
}

// V10TableFinalNode
//
//	table_final = ( TYPE_OF table_init<ident_ref> | table_init ) table_ins_tail { table_ins_tail }
type V10TableFinalNode struct {
	v10BaseNode
	Base  V10Node              // *V10TypeOfRefNode | *V10TableInitNode
	Tails []*V10TableInsTailNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (04_objects.sqg)
// =============================================================================

// v10isAssignOp returns true when the token type is one of the assignment
// operators, used to disambiguate object_init entries from plain ident_refs
// when both appear inside "[UNIFORM INFER<...>]".
func v10isAssignOp(t V10TokenType) bool {
	switch t {
	case V10_EQ, V10_COLON, V10_READONLY,
		V10_IADD_IMM, V10_ISUB_IMM, V10_IMUL_IMM, V10_IDIV_IMM,
		V10_EXTEND_ASSIGN, V10_ISUB_ASSIGN, V10_IMUL_ASSIGN, V10_IDIV_ASSIGN:
		return true
	}
	return false
}

// ---------- empty_decl ----------

// ParseEmptyDecl parses:
//
//	empty_decl = "[]" | ">>" | "//" | "{}" | '""' | "''" | "``"
func (p *V10Parser) ParseEmptyDecl() (*V10EmptyDeclNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var kind V10EmptyDeclKind
	switch tok.Type {
	case V10_EMPTY_ARR:
		kind = EmptyArray
	case V10_STREAM:
		kind = EmptyStream
	case V10_REGEXP_DECL:
		kind = EmptyRegexp
	case V10_EMPTY_OBJ:
		kind = EmptyScope
	case V10_EMPTY_STR_D:
		kind = EmptyStringD
	case V10_EMPTY_STR_S:
		kind = EmptyStringS
	case V10_EMPTY_STR_T:
		kind = EmptyStringT
	default:
		return nil, p.errAt(fmt.Sprintf("expected empty declaration ([], >>, //, {}, \"\", '', ``), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V10EmptyDeclNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Kind: kind}, nil
}

// ---------- TYPE_OF reference ----------

// parseTypeOfRef parses the pattern:  TYPE_OF typeName<ident_ref>
//
// typeName is an identifier naming a grammar rule (e.g. "array_list",
// "integer", "string", "object_init").  It is stored verbatim.
func (p *V10Parser) parseTypeOfRef() (*V10TypeOfRefNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_TYPE_OF); err != nil {
		return nil, err
	}
	if p.cur().Type != V10_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected type name after TYPE_OF, got %s", p.cur().Type))
	}
	typeName := p.cur().Value
	p.advance()
	if _, err := p.expect(V10_LT); err != nil {
		return nil, err
	}
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_GT); err != nil {
		return nil, err
	}
	return &V10TypeOfRefNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, TypeName: typeName, Ref: ref}, nil
}

// ---------- array_uniform / array_list ----------

// ParseArrayUniform parses:
//
//	array_uniform = "[" UNIFORM INFER<( values_list | array_uniform )> [","] "]"
func (p *V10Parser) ParseArrayUniform() (*V10ArrayUniformNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_LT); err != nil {
		return nil, err
	}

	var items []V10Node
	// Alternative: nested array_uniform or values_list.
	if p.cur().Type == V10_LBRACKET {
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
	if p.cur().Type == V10_COMMA {
		p.advance()
	}
	if _, err := p.expect(V10_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_RBRACKET); err != nil {
		return nil, err
	}
	return &V10ArrayUniformNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Items: items}, nil
}

// parseValuesList parses:  values_list = array_value { "," array_value }
func (p *V10Parser) parseValuesList() ([]V10Node, error) {
	val, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	items := []V10Node{val}

	for p.cur().Type == V10_COMMA {
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

// ParseArrayList parses:  array_list = array_uniform | empty_array_decl
func (p *V10Parser) ParseArrayList() (V10Node, error) {
	if p.cur().Type == V10_EMPTY_ARR {
		line, col := p.cur().Line, p.cur().Col
		p.advance()
		return &V10EmptyDeclNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Kind: EmptyArray}, nil
	}
	return p.ParseArrayUniform()
}

// ---------- array_final ----------

// ParseArrayAppendTail parses:  array_append_tail = "+" array_uniform { "+" array_uniform }
func (p *V10Parser) ParseArrayAppendTail() (*V10ArrayAppendTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_PLUS); err != nil {
		return nil, err
	}
	first, err := p.ParseArrayUniform()
	if err != nil {
		return nil, err
	}
	node := &V10ArrayAppendTailNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Arrays:      []*V10ArrayUniformNode{first},
	}
	for p.cur().Type == V10_PLUS {
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
func (p *V10Parser) ParseArrayOmitTail() (*V10ArrayOmitTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_MINUS); err != nil {
		return nil, err
	}
	first, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	node := &V10ArrayOmitTailNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Indices:     []*V10IntegerNode{first},
	}
	for p.cur().Type == V10_COMMA {
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
func (p *V10Parser) ParseArrayFinal() (*V10ArrayFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V10ArrayFinalNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	if p.cur().Type == V10_TYPE_OF {
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
	for p.cur().Type == V10_PLUS || p.cur().Type == V10_MINUS {
		saved := p.savePos()
		if p.cur().Type == V10_PLUS {
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
func (p *V10Parser) ParseLookupIdxExpr() (*V10LookupIdxExprNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V10_TYPE_OF {
		ref, err := p.parseTypeOfRef()
		if err != nil {
			return nil, err
		}
		return &V10LookupIdxExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: ref}, nil
	}

	if p.cur().Type == V10_INTEGER {
		saved := p.savePos()
		// Try scalar integer first; if an arithmetic op follows, parse it as numeric_expr.
		if integer, err := p.ParseInteger(); err == nil {
			if !v10isExprContinuation(p.cur().Type) {
				return &V10LookupIdxExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: integer}, nil
			}
		}
		p.restorePos(saved)
		ne, err := p.ParseNumericExpr()
		if err != nil {
			return nil, err
		}
		return &V10LookupIdxExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: ne}, nil
	}

	if v10isStringTok(p.cur().Type) {
		saved := p.savePos()
		if str, err := p.ParseString(); err == nil {
			if p.cur().Type != V10_PLUS { // not a concat expr
				return &V10LookupIdxExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: str}, nil
			}
		}
		p.restorePos(saved)
		se, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		return &V10LookupIdxExprNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: se}, nil
	}

	return nil, p.errAt(fmt.Sprintf("expected lookup index expression, got %s %q", p.cur().Type, p.cur().Value))
}

// ParseArrayLookup parses:  array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]"
func (p *V10Parser) ParseArrayLookup() (*V10ArrayLookupNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_LBRACKET); err != nil {
		return nil, err
	}
	idx, err := p.ParseLookupIdxExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_RBRACKET); err != nil {
		return nil, err
	}
	return &V10ArrayLookupNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Array: ref, Index: idx}, nil
}

// ---------- object_init ----------

// ParseObjectEntry parses a single  assign_lhs assign_oper array_value  inside object_init.
func (p *V10Parser) ParseObjectEntry() (*V10ObjectEntryNode, error) {
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
	return &V10ObjectEntryNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, LHS: lhs, Oper: oper, Value: val}, nil
}

// ParseObjectInit parses:
//
//	object_init = "[" UNIFORM INFER<assign_lhs assign_oper array_value { "," ... }> [","] "]"
func (p *V10Parser) ParseObjectInit() (*V10ObjectInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_LT); err != nil {
		return nil, err
	}

	first, err := p.ParseObjectEntry()
	if err != nil {
		return nil, err
	}
	entries := []V10ObjectEntryNode{*first}

	for p.cur().Type == V10_COMMA {
		saved := p.savePos()
		p.advance()
		// Stop if this comma is the trailing comma before ">".
		if p.cur().Type == V10_GT {
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
	if p.cur().Type == V10_COMMA {
		p.advance()
	}
	if _, err := p.expect(V10_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_RBRACKET); err != nil {
		return nil, err
	}
	return &V10ObjectInitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Entries: entries}, nil
}

// ---------- object_merge_tail / object_omit_tail ----------

// ParseObjectMergeTail parses:
//
//	object_merge_tail = "+" ( TYPE_OF object_init<ident_ref> | object_init ) { "+" ... }
func (p *V10Parser) ParseObjectMergeTail() (*V10ObjectMergeTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_PLUS); err != nil {
		return nil, err
	}
	item, err := p.parseObjectOrTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V10ObjectMergeTailNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Items:       []V10Node{item},
	}
	for p.cur().Type == V10_PLUS {
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

// parseObjectOrTypeOfRef returns TYPE_OF object_init<ident_ref> or *V10ObjectInitNode.
func (p *V10Parser) parseObjectOrTypeOfRef() (V10Node, error) {
	if p.cur().Type == V10_TYPE_OF {
		return p.parseTypeOfRef()
	}
	return p.ParseObjectInit()
}

// ParseObjectOmitTail parses:
//
//	object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," ... }
func (p *V10Parser) ParseObjectOmitTail() (*V10ObjectOmitTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_MINUS); err != nil {
		return nil, err
	}
	first, err := p.parseIdentOrLookupIdx()
	if err != nil {
		return nil, err
	}
	node := &V10ObjectOmitTailNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Items:       []V10Node{first},
	}
	for p.cur().Type == V10_COMMA {
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

// parseIdentOrLookupIdx returns ident_name (as *V10IdentDottedNode) or *V10LookupIdxExprNode.
func (p *V10Parser) parseIdentOrLookupIdx() (V10Node, error) {
	if p.cur().Type == V10_IDENT {
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
func (p *V10Parser) ParseObjectMergeOrOmit() (*V10ObjectMergeOrOmitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_LT); err != nil {
		return nil, err
	}

	base, err := p.parseObjectOrTypeOfRef()
	if err != nil {
		return nil, err
	}

	var modifier V10Node
	if p.cur().Type == V10_PLUS {
		modifier, err = p.ParseObjectMergeTail()
	} else if p.cur().Type == V10_MINUS {
		modifier, err = p.ParseObjectOmitTail()
	} else {
		err = p.errAt(fmt.Sprintf("expected '+' or '-' for object merge/omit, got %s", p.cur().Type))
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_GT); err != nil {
		return nil, err
	}
	return &V10ObjectMergeOrOmitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Base: base, Modifier: modifier}, nil
}

// ParseObjectFinal parses:  object_final = object_init | object_merge_or_omit | empty_array_decl
func (p *V10Parser) ParseObjectFinal() (*V10ObjectFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col

	switch p.cur().Type {
	case V10_EMPTY_ARR:
		p.advance()
		return &V10ObjectFinalNode{
			v10BaseNode: v10BaseNode{Line: line, Col: col},
			Value:       &V10EmptyDeclNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Kind: EmptyArray},
		}, nil

	case V10_UNIFORM:
		moo, err := p.ParseObjectMergeOrOmit()
		if err != nil {
			return nil, err
		}
		return &V10ObjectFinalNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: moo}, nil

	case V10_LBRACKET:
		oi, err := p.ParseObjectInit()
		if err != nil {
			return nil, err
		}
		return &V10ObjectFinalNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: oi}, nil

	default:
		return nil, p.errAt(fmt.Sprintf("expected object ([], [UNIFORM INFER<...>], or UNIFORM INFER<...>), got %s", p.cur().Type))
	}
}

// ParseObjectLookup parses:  object_lookup = TYPE_OF object_final<ident_ref> [ "[" lookup_idx_expr "]" ]
func (p *V10Parser) ParseObjectLookup() (*V10ObjectLookupNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V10ObjectLookupNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Object: ref}
	if p.cur().Type == V10_LBRACKET {
		p.advance()
		idx, err := p.ParseLookupIdxExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V10_RBRACKET); err != nil {
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
func (p *V10Parser) ParseArrayValue() (*V10ArrayValueNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V10Node) *V10ArrayValueNode {
		return &V10ArrayValueNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V10_EMPTY_ARR:
		// empty_array_decl — surface as array_final for simplicity
		af, err := p.ParseArrayFinal()
		if err != nil {
			return nil, err
		}
		return wrap(af), nil

	case V10_LBRACKET:
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

	case V10_UNIFORM:
		// Only object_merge_or_omit starts with a bare UNIFORM at value position.
		of, err := p.ParseObjectFinal()
		if err != nil {
			return nil, err
		}
		return wrap(of), nil

	case V10_TYPE_OF:
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

	case V10_INTEGER:
		// Try range first (N..N), then constant, then calc_unit.
		saved := p.savePos()
		if rng, err := p.ParseRange(); err == nil {
			return wrap(rng), nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if c, err := p.ParseConstant(); err == nil && !v10isExprContinuation(p.cur().Type) {
			return wrap(c), nil
		}
		p.restorePos(saved)
		cu, err := p.ParseCalcUnit()
		if err != nil {
			return nil, err
		}
		return wrap(cu), nil

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
// "string" here is a type-annotation identifier (not a literal), tokenised as V10_IDENT.
func (p *V10Parser) ParseTableHeader() (*V10TableHeaderNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_UNIFORM); err != nil {
		return nil, err
	}
	// Consume the "string" type-annotation identifier.
	tok := p.cur()
	if tok.Type != V10_IDENT || tok.Value != "string" {
		return nil, p.errAt(fmt.Sprintf("expected 'string' type annotation in table header, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	if _, err := p.expect(V10_LT); err != nil {
		return nil, err
	}
	var val V10Node
	var err error
	if p.cur().Type == V10_TYPE_OF {
		val, err = p.parseTypeOfRef()
	} else {
		val, err = p.ParseArrayUniform()
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_GT); err != nil {
		return nil, err
	}
	return &V10TableHeaderNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: val}, nil
}

// ParseTableRow parses:  table_row = array_final | object_final
func (p *V10Parser) ParseTableRow() (V10Node, error) {
	switch p.cur().Type {
	case V10_EMPTY_ARR:
		// Could be either; treat as array_final.
		return p.ParseArrayFinal()
	case V10_LBRACKET:
		// Try object_final first (more specific), then array_final.
		saved := p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return of, nil
		}
		p.restorePos(saved)
		return p.ParseArrayFinal()
	case V10_UNIFORM:
		return p.ParseObjectFinal()
	case V10_TYPE_OF:
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
func (p *V10Parser) ParseTableObjects() (*V10TableObjectsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_LT); err != nil {
		return nil, err
	}
	first, err := p.ParseTableRow()
	if err != nil {
		return nil, err
	}
	rows := []V10Node{first}
	for p.cur().Type == V10_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V10_GT {
			break
		}
		row, err := p.ParseTableRow()
		if err != nil {
			p.restorePos(saved)
			break
		}
		rows = append(rows, row)
	}
	if _, err := p.expect(V10_GT); err != nil {
		return nil, err
	}
	return &V10TableObjectsNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Rows: rows}, nil
}

// ParseTableInit parses:  table_init = table_header table_objects | object_final
//
// Disambiguation: UNIFORM followed by IDENT("string") → table_header; all
// other starts → object_final.
func (p *V10Parser) ParseTableInit() (*V10TableInitNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V10_UNIFORM && p.peek(1).Type == V10_IDENT && p.peek(1).Value == "string" {
		hdr, err := p.ParseTableHeader()
		if err != nil {
			return nil, err
		}
		objs, err := p.ParseTableObjects()
		if err != nil {
			return nil, err
		}
		return &V10TableInitNode{
			v10BaseNode: v10BaseNode{Line: line, Col: col},
			Header:      hdr,
			Objects:     objs,
		}, nil
	}

	of, err := p.ParseObjectFinal()
	if err != nil {
		return nil, err
	}
	return &V10TableInitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, AltObj: of}, nil
}

// ---------- table_ins_tail / table_final ----------

// ParseTableInsTail parses:
//
//	table_ins_tail = "+" ( TYPE_OF table_row<ident_ref> | table_row ) { "+" ... }
func (p *V10Parser) ParseTableInsTail() (*V10TableInsTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_PLUS); err != nil {
		return nil, err
	}
	first, err := p.parseTypeOfRowOrRow()
	if err != nil {
		return nil, err
	}
	node := &V10TableInsTailNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Items:       []V10Node{first},
	}
	for p.cur().Type == V10_PLUS {
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
func (p *V10Parser) parseTypeOfRowOrRow() (V10Node, error) {
	if p.cur().Type == V10_TYPE_OF {
		return p.parseTypeOfRef()
	}
	return p.ParseTableRow()
}

// ParseTableFinal parses:
//
//	table_final = ( TYPE_OF table_init<ident_ref> | table_init ) table_ins_tail { table_ins_tail }
func (p *V10Parser) ParseTableFinal() (*V10TableFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col

	var base V10Node
	var err error
	if p.cur().Type == V10_TYPE_OF {
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
	tails := []*V10TableInsTailNode{first}
	for p.cur().Type == V10_PLUS {
		saved := p.savePos()
		tail, err := p.ParseTableInsTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}

	return &V10TableFinalNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Base:        base,
		Tails:       tails,
	}, nil
}
