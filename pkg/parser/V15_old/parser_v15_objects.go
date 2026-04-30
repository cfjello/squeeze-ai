// parser_v13_objects.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/04_objects.sqg.
//
// V13 changes vs V12:
//   - split_oper  "..."  (Added)
//   - split_array split_oper ( string | digets | integer | date | date_time | time | time_stamp )  (Added)
//
// Covered rules:
//
//	empty_decl, empty_array_decl, func_stream_decl, func_regexp_decl,
//	empty_scope_decl, func_string_decl
//	empty_array_typed (V12)
//	array_value, values_list, array_uniform, array_list,
//	array_append_tail, array_omit_tail, array_final,
//	split_oper, split_array
//	lookup_idx_expr, array_lookup
//	object_init, object_merge_tail, object_omit_tail,
//	object_merge_or_omit, object_final, object_lookup
//	table_header, table_row, table_objects, table_init,
//	table_ins_tail, table_final  ← V12 "simple" table (not V13 structured table)
//	EXTEND<assign_rhs>
package parser

import "fmt"

// =============================================================================
// PHASE 2 — AST NODE TYPES  (04_objects.sqg)
// =============================================================================

// V13EmptyDeclKind classifies the empty-declaration token.
type V13EmptyDeclKind int

const (
	V13EmptyArray   V13EmptyDeclKind = iota // []
	V13EmptyStream                          // >>
	V13EmptyRegexp                          // //
	V13EmptyScope                           // {}
	V13EmptyStringD                         // ""
	V13EmptyStringS                         // ''
	V13EmptyStringT                         // ``
)

// V13EmptyDeclNode  empty_decl = empty_array_decl | func_stream_decl | func_regexp_decl | empty_scope_decl | func_string_decl
type V13EmptyDeclNode struct {
	V13BaseNode
	Kind V13EmptyDeclKind
}

// V13TypeOfRefNode  TYPE_OF typeName<ident_ref>
type V13TypeOfRefNode struct {
	V13BaseNode
	TypeName string
	Ref      *V13IdentRefNode
}

// V13ArrayValueNode  array_value = constant | range | ident_ref | calc_unit | array_final | object_final
type V13ArrayValueNode struct {
	V13BaseNode
	Value V13Node
}

// V13ArrayUniformNode  array_uniform = "[" UNIFORM INFER<( values_list | array_uniform )> [","] "]"
type V13ArrayUniformNode struct {
	V13BaseNode
	Items []V13Node
}

// V13ArrayAppendTailNode  array_append_tail = "+" array_uniform { "+" array_uniform }
type V13ArrayAppendTailNode struct {
	V13BaseNode
	Arrays []*V13ArrayUniformNode
}

// V13ArrayOmitTailNode  array_omit_tail = "-" integer { "," integer }
type V13ArrayOmitTailNode struct {
	V13BaseNode
	Indices []*V13IntegerNode
}

// V13EmptyArrayTypedNode  empty_array_typed = ident_ref empty_array_decl
type V13EmptyArrayTypedNode struct {
	V13BaseNode
	Ref *V13IdentRefNode
}

// V13PlainArrayNode  plain_array = "[" array_value { "," array_value } "]"
// A bracket-enclosed list of constants / ident_refs with no UNIFORM INFER<> wrapper.
// Used by lib header fields (names:) and wherever a literal list is needed.
type V13PlainArrayNode struct {
	V13BaseNode
	Items []*V13ArrayValueNode
}

// V13ArrayFinalNode  array_final = ( TYPE_OF array_list<ident_ref> | array_list ) { tails }
type V13ArrayFinalNode struct {
	V13BaseNode
	TypeRef *V13TypeOfRefNode // non-nil for TYPE_OF form
	List    V13Node           // *V13ArrayUniformNode | *V13EmptyArrayTypedNode | *V13PlainArrayNode
	Tails   []V13Node         // *V13ArrayAppendTailNode | *V13ArrayOmitTailNode
}

// V13LookupIdxExprNode  lookup_idx_expr = integer | numeric_expr | string | string_expr | TYPE_OF …<ident_ref>
type V13LookupIdxExprNode struct {
	V13BaseNode
	Value V13Node
}

// V13ArrayLookupNode  array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]"
type V13ArrayLookupNode struct {
	V13BaseNode
	Array *V13TypeOfRefNode
	Index *V13LookupIdxExprNode
}

// V13ObjectEntryNode  assign_lhs assign_oper array_value  (one object key-value pair)
type V13ObjectEntryNode struct {
	V13BaseNode
	LHS   *V13AssignLHSNode
	Oper  *V13AssignOperNode
	Value *V13ArrayValueNode
}

// V13ObjectInitNode  object_init = "[" UNIFORM INFER<…> [","] "]"
type V13ObjectInitNode struct {
	V13BaseNode
	Entries []V13ObjectEntryNode
}

// V13ObjectMergeTailNode  object_merge_tail = "+" … { "+" … }
type V13ObjectMergeTailNode struct {
	V13BaseNode
	Items []V13Node // *V13TypeOfRefNode | *V13ObjectInitNode
}

// V13ObjectOmitTailNode  object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," … }
type V13ObjectOmitTailNode struct {
	V13BaseNode
	Items []V13Node // *V13IdentDottedNode | *V13LookupIdxExprNode
}

// V13ObjectMergeOrOmitNode  object_merge_or_omit = UNIFORM INFER<base (merge|omit)>
type V13ObjectMergeOrOmitNode struct {
	V13BaseNode
	Base     V13Node // *V13TypeOfRefNode | *V13ObjectInitNode
	Modifier V13Node // *V13ObjectMergeTailNode | *V13ObjectOmitTailNode
}

// V13ObjectFinalNode  object_final = object_init | object_merge_or_omit | empty_array_typed
type V13ObjectFinalNode struct {
	V13BaseNode
	Value V13Node
}

// V13ObjectLookupNode  object_lookup = TYPE_OF object_final<ident_ref> [ "[" lookup_idx_expr "]" ]
type V13ObjectLookupNode struct {
	V13BaseNode
	Object *V13TypeOfRefNode
	Index  *V13LookupIdxExprNode // nil when absent
}

// V13LhsCallerNode  LHS_CALLER<§assign_lhs.ident_name|cardinality>
// A bootstrap directive that resolves at parse time to either the list of
// identifier names or the cardinality declared on the left-hand side of the
// enclosing assignment.
// Path is the dotted argument path, e.g. "assign_lhs.ident_name" or
// "assign_lhs.cardinality".
type V13LhsCallerNode struct {
	V13BaseNode
	Path string // "assign_lhs.ident_name" | "assign_lhs.cardinality"
}

// V13BootstrapCallNode  UPPERCASE_IDENT [pre_arg] "<" ident_ref ">"
// Represents an angle-bracket bootstrap/intrinsic call such as:
//   LENGTH<arr.data>            — element count
//   ARRAY_REVERSE<arr.data>     — reverse copy
//   TO_JSON<elem>               — JSON serialise
//   SUB_RANGE sub_range<arr>    — sub-range slice (pre_arg = "sub_range")
//   CAST string<val>            — widen/coerce (pre_arg = "string")
type V13BootstrapCallNode struct {
	V13BaseNode
	Name   string // e.g. "LENGTH", "ARRAY_REVERSE", "CAST"
	PreArg string // optional IDENT appearing between Name and "<"
	Arg    V13Node
}

// V13TableHeaderNode  table_header = UNIFORM string<( array_uniform | TYPE_OF array_uniform<ident_ref> )>
// Note: This is the V12-style simple table header. The V13 structured table is in parser_v13_structures.go.
type V13TableHeaderNode struct {
	V13BaseNode
	Value V13Node // *V13ArrayUniformNode | *V13TypeOfRefNode
}

// V13TableObjectsNode  table_objects = UNIFORM INFER<table_row { "," table_row }>
type V13TableObjectsNode struct {
	V13BaseNode
	Rows []V13Node
}

// V13TableInitSimpleNode  table_init (V12 style) = table_header table_objects | object_final
// Renamed to avoid collision with V13TableInitNode in parser_v13_structures.go.
type V13TableInitSimpleNode struct {
	V13BaseNode
	Header  *V13TableHeaderNode  // non-nil for header+objects form
	Objects *V13TableObjectsNode // non-nil for header+objects form
	AltObj  *V13ObjectFinalNode  // non-nil for object_final fallback
}

// V13TableInsTailNode  table_ins_tail = "+" … { "+" … }
type V13TableInsTailNode struct {
	V13BaseNode
	Items []V13Node // *V13TypeOfRefNode | array/object final
}

// V13TableFinalSimpleNode  table_final (V12 style)
// Renamed to avoid collision with V13TableFinalNode in parser_v13_structures.go.
type V13TableFinalSimpleNode struct {
	V13BaseNode
	Base  V13Node // *V13TypeOfRefNode | *V13TableInitSimpleNode
	Tails []*V13TableInsTailNode
}

// V13SplitArrayNode  split_array = "..." ( string | digets | integer | date | date_time | time | time_stamp )
// split_oper is the fixed token "..." (V13_ELLIPSIS); Separator holds the constant following it.
type V13SplitArrayNode struct {
	V13BaseNode
	Separator *V13ConstantNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (04_objects.sqg)
// =============================================================================

// V13isAssignOp returns true for any assignment operator token.
func V13isAssignOp(t V13TokenType) bool {
	switch t {
	case V13_EQ, V13_COLON, V13_READONLY,
		V13_IADD_IMM, V13_ISUB_IMM, V13_IMUL_IMM, V13_IDIV_IMM,
		V13_EXTEND_ASSIGN, V13_ISUB_ASSIGN, V13_IMUL_ASSIGN, V13_IDIV_ASSIGN:
		return true
	}
	return false
}

// ---------- empty_decl ----------

// ParseEmptyDecl parses:  empty_decl = "[]" | ">>" | "//" | "{}" | '""' | "''" | "``"
func (p *V13Parser) ParseEmptyDecl() (*V13EmptyDeclNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var kind V13EmptyDeclKind
	switch tok.Type {
	case V13_EMPTY_ARR:
		kind = V13EmptyArray
	case V13_STREAM:
		kind = V13EmptyStream
	case V13_REGEXP_DECL:
		kind = V13EmptyRegexp
	case V13_EMPTY_OBJ:
		kind = V13EmptyScope
	case V13_EMPTY_STR_D:
		kind = V13EmptyStringD
	case V13_EMPTY_STR_S:
		kind = V13EmptyStringS
	case V13_EMPTY_STR_T:
		kind = V13EmptyStringT
	default:
		return nil, p.errAt(fmt.Sprintf("expected empty declaration, got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13EmptyDeclNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Kind: kind}, nil
}

// ---------- TYPE_OF reference ----------

// parseTypeOfRef parses:  TYPE_OF typeName<ident_ref>
func (p *V13Parser) parseTypeOfRef() (*V13TypeOfRefNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_TYPE_OF); err != nil {
		return nil, err
	}
	if p.cur().Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected type name after TYPE_OF, got %s", p.cur().Type))
	}
	typeName := p.cur().Value
	p.advance()
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13TypeOfRefNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, TypeName: typeName, Ref: ref}, nil
}

// ---------- array_uniform / array_list ----------

// ParseArrayUniform parses:
//
//	array_uniform = "[" UNIFORM INFER<( values_list | array_uniform )> [","] "]"
func (p *V13Parser) ParseArrayUniform() (*V13ArrayUniformNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}

	var items []V13Node
	if p.cur().Type == V13_LBRACKET {
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

	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13ArrayUniformNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Items: items}, nil
}

// parseValuesList parses:  values_list = array_value { "," array_value }
func (p *V13Parser) parseValuesList() ([]V13Node, error) {
	val, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	items := []V13Node{val}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
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
func (p *V13Parser) ParseEmptyArrayTyped() (*V13EmptyArrayTypedNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_EMPTY_ARR); err != nil {
		return nil, err
	}
	return &V13EmptyArrayTypedNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Ref: ref}, nil
}

// ParseArrayList parses:  array_list = array_uniform | empty_array_typed | plain_array
func (p *V13Parser) ParseArrayList() (V13Node, error) {
	if p.cur().Type == V13_IDENT {
		saved := p.savePos()
		if eat, err := p.ParseEmptyArrayTyped(); err == nil {
			return eat, nil
		}
		p.restorePos(saved)
	}
	saved := p.savePos()
	if pa, err := p.ParsePlainArray(); err == nil {
		return pa, nil
	}
	p.restorePos(saved)
	return p.ParseArrayUniform()
}

// ParsePlainArray parses:  plain_array = "[" array_value { "," array_value } "]"
func (p *V13Parser) ParsePlainArray() (*V13PlainArrayNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	node := &V13PlainArrayNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}
	if p.cur().Type == V13_RBRACKET {
		p.advance()
		return node, nil
	}
	first, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	node.Items = append(node.Items, first)
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		v, err := p.ParseArrayValue()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, v)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return node, nil
}

// ---------- array_final ----------

// ParseArrayAppendTail parses:  array_append_tail = "+" array_uniform { "+" array_uniform }
func (p *V13Parser) ParseArrayAppendTail() (*V13ArrayAppendTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	first, err := p.ParseArrayUniform()
	if err != nil {
		return nil, err
	}
	node := &V13ArrayAppendTailNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Arrays:      []*V13ArrayUniformNode{first},
	}
	for p.cur().Type == V13_PLUS {
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
func (p *V13Parser) ParseArrayOmitTail() (*V13ArrayOmitTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_MINUS); err != nil {
		return nil, err
	}
	first, err := p.ParseInteger()
	if err != nil {
		return nil, err
	}
	node := &V13ArrayOmitTailNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Indices:     []*V13IntegerNode{first},
	}
	for p.cur().Type == V13_COMMA {
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
func (p *V13Parser) ParseArrayFinal() (*V13ArrayFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V13ArrayFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}

	if p.cur().Type == V13_TYPE_OF {
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

	for p.cur().Type == V13_PLUS || p.cur().Type == V13_MINUS {
		saved := p.savePos()
		if p.cur().Type == V13_PLUS {
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

// ParseLookupIdxExpr parses:  lookup_idx_expr = integer | numeric_expr | string | string_expr | TYPE_OF …<ident_ref>
func (p *V13Parser) ParseLookupIdxExpr() (*V13LookupIdxExprNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V13_TYPE_OF {
		ref, err := p.parseTypeOfRef()
		if err != nil {
			return nil, err
		}
		return &V13LookupIdxExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: ref}, nil
	}

	if p.cur().Type == V13_INTEGER {
		saved := p.savePos()
		if integer, err := p.ParseInteger(); err == nil {
			if !V13isExprContinuation(p.cur().Type) {
				return &V13LookupIdxExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: integer}, nil
			}
		}
		p.restorePos(saved)
		ne, err := p.ParseNumericExpr()
		if err != nil {
			return nil, err
		}
		return &V13LookupIdxExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: ne}, nil
	}

	if V13isStringTok(p.cur().Type) {
		saved := p.savePos()
		if str, err := p.ParseString(); err == nil {
			if p.cur().Type != V13_PLUS {
				return &V13LookupIdxExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: str}, nil
			}
		}
		p.restorePos(saved)
		se, err := p.ParseStringExpr()
		if err != nil {
			return nil, err
		}
		return &V13LookupIdxExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: se}, nil
	}

	// ident_ref used as a numeric index: arr.data[n], arr.data[i+1], etc.
	if p.cur().Type == V13_IDENT {
		ne, err := p.ParseNumericExpr()
		if err != nil {
			return nil, err
		}
		return &V13LookupIdxExprNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: ne}, nil
	}

	return nil, p.errAt(fmt.Sprintf("expected lookup index expression, got %s %q", p.cur().Type, p.cur().Value))
}

// ParseArrayLookup parses:  array_lookup = TYPE_OF array_final<ident_ref> "[" lookup_idx_expr "]"
func (p *V13Parser) ParseArrayLookup() (*V13ArrayLookupNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	idx, err := p.ParseLookupIdxExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13ArrayLookupNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Array: ref, Index: idx}, nil
}

// ---------- object_init ----------

// ParseObjectEntry parses:  assign_lhs assign_oper array_value  (one entry inside object_init)
func (p *V13Parser) ParseObjectEntry() (*V13ObjectEntryNode, error) {
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
	return &V13ObjectEntryNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, LHS: lhs, Oper: oper, Value: val}, nil
}

// ParseObjectInit parses:  object_init = "[" UNIFORM INFER<…> [","] "]"
func (p *V13Parser) ParseObjectInit() (*V13ObjectInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}

	first, err := p.ParseObjectEntry()
	if err != nil {
		return nil, err
	}
	entries := []V13ObjectEntryNode{*first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_GT {
			break
		}
		e, err := p.ParseObjectEntry()
		if err != nil {
			p.restorePos(saved)
			break
		}
		entries = append(entries, *e)
	}
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13ObjectInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Entries: entries}, nil
}

// ParseSimpleObjectInit parses V1-style objects:  "[" entry { "," entry } [","] "]"
func (p *V13Parser) ParseSimpleObjectInit() (*V13ObjectInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if p.cur().Type == V13_RBRACKET {
		p.advance()
		return &V13ObjectInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}, nil
	}
	first, err := p.ParseObjectEntry()
	if err != nil {
		return nil, err
	}
	entries := []V13ObjectEntryNode{*first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		e, err := p.ParseObjectEntry()
		if err != nil {
			p.restorePos(saved)
			break
		}
		entries = append(entries, *e)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13ObjectInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Entries: entries}, nil
}

// ParseObjectMergeTail parses:  object_merge_tail = "+" ( TYPE_OF … | object_init ) { "+" … }
func (p *V13Parser) ParseObjectMergeTail() (*V13ObjectMergeTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	item, err := p.parseObjectOrTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V13ObjectMergeTailNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Items:       []V13Node{item},
	}
	for p.cur().Type == V13_PLUS {
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

func (p *V13Parser) parseObjectOrTypeOfRef() (V13Node, error) {
	if p.cur().Type == V13_TYPE_OF {
		return p.parseTypeOfRef()
	}
	return p.ParseObjectInit()
}

// ParseObjectOmitTail parses:  object_omit_tail = "-" (ident_name | lookup_idx_expr) { "," … }
func (p *V13Parser) ParseObjectOmitTail() (*V13ObjectOmitTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_MINUS); err != nil {
		return nil, err
	}
	first, err := p.parseIdentOrLookupIdx()
	if err != nil {
		return nil, err
	}
	node := &V13ObjectOmitTailNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Items:       []V13Node{first},
	}
	for p.cur().Type == V13_COMMA {
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

func (p *V13Parser) parseIdentOrLookupIdx() (V13Node, error) {
	if p.cur().Type == V13_IDENT {
		return p.ParseIdentDotted()
	}
	return p.ParseLookupIdxExpr()
}

// ParseObjectMergeOrOmit parses:  object_merge_or_omit = UNIFORM INFER<base (merge|omit)>
func (p *V13Parser) ParseObjectMergeOrOmit() (*V13ObjectMergeOrOmitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	base, err := p.parseObjectOrTypeOfRef()
	if err != nil {
		return nil, err
	}
	var modifier V13Node
	if p.cur().Type == V13_PLUS {
		modifier, err = p.ParseObjectMergeTail()
	} else if p.cur().Type == V13_MINUS {
		modifier, err = p.ParseObjectOmitTail()
	} else {
		err = p.errAt(fmt.Sprintf("expected '+' or '-' for object merge/omit, got %s", p.cur().Type))
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13ObjectMergeOrOmitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Modifier: modifier}, nil
}

// ParseObjectFinal parses:  object_final = object_init | object_merge_or_omit | empty_array_typed
func (p *V13Parser) ParseObjectFinal() (*V13ObjectFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	switch p.cur().Type {
	case V13_IDENT:
		saved := p.savePos()
		if eat, err := p.ParseEmptyArrayTyped(); err == nil {
			return &V13ObjectFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: eat}, nil
		}
		p.restorePos(saved)
		return nil, p.errAt(fmt.Sprintf("expected object, got %s", p.cur().Type))

	case V13_UNIFORM:
		moo, err := p.ParseObjectMergeOrOmit()
		if err != nil {
			return nil, err
		}
		return &V13ObjectFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: moo}, nil

	case V13_LBRACKET:
		saved := p.savePos()
		if oi, err := p.ParseObjectInit(); err == nil {
			return &V13ObjectFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: oi}, nil
		}
		p.restorePos(saved)
		if oi, err := p.ParseSimpleObjectInit(); err == nil {
			return &V13ObjectFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: oi}, nil
		}
		p.restorePos(saved)
		return nil, p.errAt(fmt.Sprintf("expected object literal, got %s", p.cur().Type))

	default:
		return nil, p.errAt(fmt.Sprintf("expected object, got %s", p.cur().Type))
	}
}

// ParseObjectLookup parses:  object_lookup = TYPE_OF object_final<ident_ref> [ "[" lookup_idx_expr "]" ]
func (p *V13Parser) ParseObjectLookup() (*V13ObjectLookupNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V13ObjectLookupNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Object: ref}
	if p.cur().Type == V13_LBRACKET {
		p.advance()
		idx, err := p.ParseLookupIdxExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(V13_RBRACKET); err != nil {
			return nil, err
		}
		node.Index = idx
	}
	return node, nil
}

// ---------- array_value ----------

// ParseLhsCaller parses:  LHS_CALLER "<" "§" ident_name "." ident_name ">"
// It is a bootstrap look-behind directive that resolves to the ident_name(s)
// or cardinality from the LHS of the enclosing assignment at parse time.
func (p *V13Parser) ParseLhsCaller() (*V13LhsCallerNode, error) {
	line, col := p.cur().Line, p.cur().Col
	tok := p.cur()
	if tok.Type != V13_IDENT || tok.Value != "LHS_CALLER" {
		return nil, p.errAt(fmt.Sprintf("expected LHS_CALLER, got %s %q", tok.Type, tok.Value))
	}
	p.advance() // consume LHS_CALLER
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_SECTION); err != nil {
		return nil, err
	}
	first := p.cur()
	if first.Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected ident after § in LHS_CALLER, got %s %q", first.Type, first.Value))
	}
	p.advance()
	if _, err := p.expect(V13_DOT); err != nil {
		return nil, err
	}
	second := p.cur()
	if second.Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected ident after . in LHS_CALLER path, got %s %q", second.Type, second.Value))
	}
	p.advance()
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13LhsCallerNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Path:        first.Value + "." + second.Value,
	}, nil
}

// isBootstrapCallName returns true when s is an all-uppercase identifier
// that introduces an angle-bracket bootstrap intrinsic call
// (LENGTH, ARRAY_REVERSE, TO_JSON, SUB_RANGE, etc.).  LHS_CALLER is
// handled separately by ParseLhsCaller because its argument uses §.
func isBootstrapCallName(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || r == '_') {
			return false
		}
	}
	return s != "LHS_CALLER" // handled by its own parser
}

// looksLikeBootstrapCall returns true when the current position looks like the
// start of a bootstrap intrinsic call:
//   UPPERCASE_IDENT "<" …                         — e.g. LENGTH<arr>
//   UPPERCASE_IDENT IDENT "<" …                   — e.g. SUB_RANGE sub<arr>
// Also true when the current token is the CAST keyword:
//   V13_CAST IDENT "<" …                          — e.g. CAST string<val>
func (p *V13Parser) looksLikeBootstrapCall() bool {
	tok := p.cur()
	switch tok.Type {
	case V13_IDENT:
		if !isBootstrapCallName(tok.Value) {
			return false
		}
		next := p.peek(1)
		if next.Type == V13_LT {
			return true
		}
		if next.Type == V13_IDENT && p.peek(2).Type == V13_LT {
			return true
		}
	case V13_CAST:
		next := p.peek(1)
		if next.Type == V13_LT {
			return true
		}
		if next.Type == V13_IDENT && p.peek(2).Type == V13_LT {
			return true
		}
	}
	return false
}

// ParseBootstrapCall parses:
//
//	bootstrap_call = ( UPPERCASE_IDENT | CAST ) [ IDENT ] "<" ident_ref ">"
//
// Covers LENGTH<arr.data>, ARRAY_REVERSE<arr>, TO_JSON<elem>,
// SUB_RANGE sub_range<arr>, CAST string<val>, etc.
func (p *V13Parser) ParseBootstrapCall() (*V13BootstrapCallNode, error) {
	line, col := p.cur().Line, p.cur().Col
	tok := p.cur()

	var name string
	switch tok.Type {
	case V13_IDENT:
		if !isBootstrapCallName(tok.Value) {
			return nil, p.errAt(fmt.Sprintf("expected uppercase bootstrap call name, got %q", tok.Value))
		}
		name = tok.Value
	case V13_CAST:
		name = "CAST"
	default:
		return nil, p.errAt(fmt.Sprintf("expected uppercase IDENT or CAST for bootstrap call, got %s", tok.Type))
	}
	p.advance()

	// Optional IDENT pre-arg that appears between the call name and "<".
	var preArg string
	if p.cur().Type == V13_IDENT && p.peek(1).Type == V13_LT {
		preArg = p.cur().Value
		p.advance()
	}

	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}

	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}

	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}

	return &V13BootstrapCallNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Name:        name,
		PreArg:      preArg,
		Arg:         ref,
	}, nil
}

// ParseArrayValue parses:
//
//	array_value = constant | range | ident_ref | calc_unit | array_final | object_final
func (p *V13Parser) ParseArrayValue() (*V13ArrayValueNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13ArrayValueNode {
		return &V13ArrayValueNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V13_EMPTY_ARR:
		af, err := p.ParseArrayFinal()
		if err != nil {
			return nil, err
		}
		return wrap(af), nil

	case V13_LBRACKET:
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

	case V13_UNIFORM:
		of, err := p.ParseObjectFinal()
		if err != nil {
			return nil, err
		}
		return wrap(of), nil

	case V13_TYPE_OF:
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

	case V13_INTEGER:
		saved := p.savePos()
		if rng, err := p.ParseRange(); err == nil {
			return wrap(rng), nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if c, err := p.ParseConstant(); err == nil && !V13isExprContinuation(p.cur().Type) {
			return wrap(c), nil
		}
		p.restorePos(saved)
		cu, err := p.ParseCalcUnit()
		if err != nil {
			return nil, err
		}
		return wrap(cu), nil

	case V13_DOLLAR:
		p.advance()
		return wrap(&V13SelfRefNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}), nil

	case V13_IDENT:
		// Bootstrap look-behind directive: LHS_CALLER<§assign_lhs.ident_name|cardinality>
		if p.cur().Value == "LHS_CALLER" && p.peek(1).Type == V13_LT {
			node, err := p.ParseLhsCaller()
			if err != nil {
				return nil, err
			}
			return wrap(node), nil
		}
		// General uppercase angle-bracket intrinsic: LENGTH<arr>, SUB_RANGE x<arr>, etc.
		if p.looksLikeBootstrapCall() {
			node, err := p.ParseBootstrapCall()
			if err != nil {
				return nil, err
			}
			return wrap(node), nil
		}
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

	default:
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

// ---------- Simple V12-style table (kept for backward compat in V13) ----------

// ParseTableHeader parses:  table_header = UNIFORM string<( array_uniform | TYPE_OF array_uniform<ident_ref> )>
func (p *V13Parser) ParseTableHeader() (*V13TableHeaderNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_UNIFORM); err != nil {
		return nil, err
	}
	tok := p.cur()
	if tok.Type != V13_IDENT || tok.Value != "string" {
		return nil, p.errAt(fmt.Sprintf("expected 'string' annotation in table header, got %q", tok.Value))
	}
	p.advance()
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	var val V13Node
	var err error
	if p.cur().Type == V13_TYPE_OF {
		val, err = p.parseTypeOfRef()
	} else {
		val, err = p.ParseArrayUniform()
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13TableHeaderNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}, nil
}

// parseTableRow parses:  table_row = array_final | object_final
func (p *V13Parser) parseTableRow() (V13Node, error) {
	switch p.cur().Type {
	case V13_EMPTY_ARR:
		return p.ParseArrayFinal()
	case V13_LBRACKET:
		saved := p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return of, nil
		}
		p.restorePos(saved)
		return p.ParseArrayFinal()
	case V13_UNIFORM:
		return p.ParseObjectFinal()
	case V13_TYPE_OF:
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
func (p *V13Parser) ParseTableObjects() (*V13TableObjectsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_UNIFORM); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_INFER); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	first, err := p.parseTableRow()
	if err != nil {
		return nil, err
	}
	rows := []V13Node{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_GT {
			break
		}
		row, err := p.parseTableRow()
		if err != nil {
			p.restorePos(saved)
			break
		}
		rows = append(rows, row)
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13TableObjectsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Rows: rows}, nil
}

// ParseTableInitSimple parses the V12-style:  table_init = table_header table_objects | object_final
func (p *V13Parser) ParseTableInitSimple() (*V13TableInitSimpleNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type == V13_UNIFORM && p.peek(1).Type == V13_IDENT && p.peek(1).Value == "string" {
		hdr, err := p.ParseTableHeader()
		if err != nil {
			return nil, err
		}
		objs, err := p.ParseTableObjects()
		if err != nil {
			return nil, err
		}
		return &V13TableInitSimpleNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			Header:      hdr,
			Objects:     objs,
		}, nil
	}
	of, err := p.ParseObjectFinal()
	if err != nil {
		return nil, err
	}
	return &V13TableInitSimpleNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, AltObj: of}, nil
}

// ParseTableInsTail parses:  table_ins_tail = "+" ( TYPE_OF … | table_row ) { "+" … }
func (p *V13Parser) ParseTableInsTail() (*V13TableInsTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	first, err := p.parseTypeOfRowOrRow()
	if err != nil {
		return nil, err
	}
	node := &V13TableInsTailNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Items:       []V13Node{first},
	}
	for p.cur().Type == V13_PLUS {
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

func (p *V13Parser) parseTypeOfRowOrRow() (V13Node, error) {
	if p.cur().Type == V13_TYPE_OF {
		return p.parseTypeOfRef()
	}
	return p.parseTableRow()
}

// ParseTableFinalSimple parses (V12-style):  table_final = ( TYPE_OF … | table_init ) table_ins_tail { … }
func (p *V13Parser) ParseTableFinalSimple() (*V13TableFinalSimpleNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	var err error
	if p.cur().Type == V13_TYPE_OF {
		base, err = p.parseTypeOfRef()
	} else {
		base, err = p.ParseTableInitSimple()
	}
	if err != nil {
		return nil, err
	}
	first, err := p.ParseTableInsTail()
	if err != nil {
		return nil, fmt.Errorf("table_final: expected at least one insert-tail (+row): %w", err)
	}
	tails := []*V13TableInsTailNode{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		tail, err := p.ParseTableInsTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}
	return &V13TableFinalSimpleNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Base:        base,
		Tails:       tails,
	}, nil
}

// ---------- split_array ----------

// ParseSplitArray parses:
//
//	split_array = "..." ( string | digets | integer | date | date_time | time | time_stamp )
//
// The separator is any constant whose type is one of the above;
// digets (unsigned digit sequence) is treated as integer here.
func (p *V13Parser) ParseSplitArray() (*V13SplitArrayNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_ELLIPSIS); err != nil {
		return nil, err
	}
	sep, err := p.ParseConstant()
	if err != nil {
		return nil, fmt.Errorf("%d:%d: split_array: expected separator (string|digits|integer|date|date_time|time|time_stamp): %w",
			line, col, err)
	}
	return &V13SplitArrayNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Separator:   sep,
	}, nil
}
