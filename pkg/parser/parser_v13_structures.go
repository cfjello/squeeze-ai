// parser_v13_structures.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/09_stuctures.sqg.
//
// New in V13: Table (structured), Trees (general, string, keyed, sorted),
// Set, Enum, Graph, Bitfield.
//
// This file also provides shared parsers for:
//   - unique_key  (uuid | uuid_v7 | ulid | snowflake_id | nano_id | hash_key | seq_id | composite_key)
//   - hashable    (runtime concept; grammar: unique_key | string_quoted | integer | boolean)
//   - sortable    (runtime concept; grammar: subset of unique_key that has a total order)
package parser

import "fmt"

// =============================================================================
// SHARED TYPE NODES
// =============================================================================

// V13UniqueKeyNode  unique_key = uuid | uuid_v7 | ulid | snowflake_id | nano_id |
//
//	hash_key | seq_id | composite_key
//
// Already declared in parser_v13.go.  ParseUniqueKey is defined below.

// V13HashableNode  hashable = unique_key | string_quoted | integer | boolean
// Wraps any value that can serve as a set member or object key.
type V13HashableNode struct {
	V13BaseNode
	Value V13Node
}

// V13SortableNode  sortable ⊂ unique_key (total order at runtime).
// Same grammar as unique_key; semantic distinction is enforced at runtime.
type V13SortableNode struct {
	V13BaseNode
	Value V13Node // *V13UniqueKeyNode
}

// =============================================================================
// TABLE  (spec/09_stuctures.sqg — Table section)
// =============================================================================

// V13TableColNode  table_col = ident_ref inspect_type | inspect_type
type V13TableColNode struct {
	V13BaseNode
	Name     *V13IdentRefNode // nil for anonymous (@Type only)
	InspType *V13InspectTypeNode
}

// V13TableColumnsNode  table_columns = HAS_ONE<unique_key>< "[" table_col { "," table_col } "]" >
type V13TableColumnsNode struct {
	V13BaseNode
	Cols []*V13TableColNode
}

// V13TableColFromObjNode  table_col_from_obj = HAS_ONE<unique_key>< TYPE_OF table_col_hashable<object_final> >
type V13TableColFromObjNode struct {
	V13BaseNode
	ObjRef *V13ObjectFinalNode
}

// V13KeyColNode  key_col = ident_ref TYPE_OF unique_key<inspect_type> | TYPE_OF unique_key<inspect_type>
type V13KeyColNode struct {
	V13BaseNode
	Name     *V13IdentRefNode
	InspType *V13InspectTypeNode
}

// V13KeyColumnsNode  key_columns = SUBSET_OF<table_columns>< "[" key_col { "," key_col } "]" >
type V13KeyColumnsNode struct {
	V13BaseNode
	Cols []*V13KeyColNode
}

// V13TableRowNode  table_row = "[" array_value { "," array_value } "]"
type V13TableRowNode struct {
	V13BaseNode
	Values []*V13ArrayValueNode
}

// V13TableRowsListNode  table_rows_list = UNIFORM INFER< table_row { "," table_row } >
type V13TableRowsListNode struct {
	V13BaseNode
	Rows []*V13TableRowNode
}

// V13TableInitNode  table_init = "[" "columns" ":" … "," "key_columns" ":" … "," "rows" ":" … "]"
type V13TableInitNode struct {
	V13BaseNode
	Columns    V13Node // *V13TableColumnsNode | *V13TableColFromObjNode
	KeyColumns *V13KeyColumnsNode
	Rows       *V13TableRowsListNode
}

// V13TableInsTailStructNode  table_ins_tail = "+" (TYPE_OF table_row<ident_ref> | table_row) { "+" … }
type V13TableInsTailStructNode struct {
	V13BaseNode
	Additions []V13Node // *V13IdentRefNode | *V13TableRowNode
}

// V13TableFinalNode  table_final = ( TYPE_OF table_init<ident_ref> | table_init ) { table_ins_tail }
type V13TableFinalNode struct {
	V13BaseNode
	Base  V13Node // *V13IdentRefNode | *V13TableInitNode
	Tails []*V13TableInsTailStructNode
}

// =============================================================================
// TREE  (general)
// =============================================================================

// V13TreeValueNode  tree_value = constant | ident_ref | calc_unit | object_final
type V13TreeValueNode struct {
	V13BaseNode
	Value V13Node
}

// V13TreeNodeNode  tree_node = "[" tree_value [ "," "children" ":" tree_children ] "]"
// tree_children = "[" tree_node { "," tree_node } "]"
type V13TreeNodeNode struct {
	V13BaseNode
	Value    *V13TreeValueNode
	Children []*V13TreeNodeNode
}

// V13TreeInitNode  tree_init = "[" "type" ":" inspect_type "," "root" ":" tree_node "]"
type V13TreeInitNode struct {
	V13BaseNode
	TypeAnnot *V13InspectTypeNode
	Root      *V13TreeNodeNode
}

// V13TreeInsTailNode  tree_ins_tail = "+" (TYPE_OF tree_node<ident_ref> | tree_node) { "+" … }
type V13TreeInsTailNode struct {
	V13BaseNode
	Additions []V13Node // *V13IdentRefNode | *V13TreeNodeNode
}

// V13TreeFinalNode  tree_final = ( TYPE_OF tree_init<ident_ref> | tree_init ) { tree_ins_tail }
type V13TreeFinalNode struct {
	V13BaseNode
	Base  V13Node // *V13IdentRefNode | *V13TreeInitNode
	Tails []*V13TreeInsTailNode
}

// =============================================================================
// STRING TREE
// =============================================================================

// V13StringTreeValueNode  string_tree_value = string_quoted | TYPE_OF string<ident_ref>
type V13StringTreeValueNode struct {
	V13BaseNode
	Str     *V13StringQuotedNode
	TypedOf *V13IdentRefNode
}

// V13StringTreeNodeNode  string_tree_node = "[" string_tree_value [ "," "children" ":" string_tree_children ] "]"
type V13StringTreeNodeNode struct {
	V13BaseNode
	Value    *V13StringTreeValueNode
	Children []*V13StringTreeNodeNode
}

// V13StringTreeInitNode  string_tree_init = "[" "root" ":" string_tree_node "]"
type V13StringTreeInitNode struct {
	V13BaseNode
	Root *V13StringTreeNodeNode
}

// V13StringTreeInsTailNode  string_tree_ins_tail = "+" … { "+" … }
type V13StringTreeInsTailNode struct {
	V13BaseNode
	Additions []V13Node
}

// V13StringTreeFinalNode  string_tree_final = ( TYPE_OF string_tree_init<ident_ref> | string_tree_init ) { … }
type V13StringTreeFinalNode struct {
	V13BaseNode
	Base  V13Node
	Tails []*V13StringTreeInsTailNode
}

// =============================================================================
// KEYED TREE
// =============================================================================

// V13KeyedTreeNodeNode  tree_node_keyed = "[" "key" ":" unique_key "," "value" ":" tree_value
//
//	[ "," "children" ":" tree_children_keyed ] "]"
type V13KeyedTreeNodeNode struct {
	V13BaseNode
	Key      *V13UniqueKeyNode
	Value    *V13TreeValueNode
	Children []*V13KeyedTreeNodeNode
}

// V13KeyedTreeInitNode  keyed_tree_init = "[" "type" ":" inspect_type "," "root" ":" tree_node_keyed "]"
type V13KeyedTreeInitNode struct {
	V13BaseNode
	TypeAnnot *V13InspectTypeNode
	Root      *V13KeyedTreeNodeNode
}

// V13KeyedTreeInsTailNode  keyed_tree_ins_tail = "+" … { "+" … }
type V13KeyedTreeInsTailNode struct {
	V13BaseNode
	Additions []V13Node
}

// V13KeyedTreeFinalNode  keyed_tree_final = ( TYPE_OF keyed_tree_init<ident_ref> | keyed_tree_init ) { … }
type V13KeyedTreeFinalNode struct {
	V13BaseNode
	Base  V13Node
	Tails []*V13KeyedTreeInsTailNode
}

// =============================================================================
// SORTED TREE
// =============================================================================

// V13SortedTreeNodeNode  tree_node_sorted = "[" "key" ":" sortable "," "value" ":" tree_value
//
//	[ "," "children" ":" tree_children_sorted ] "]"
type V13SortedTreeNodeNode struct {
	V13BaseNode
	Key      *V13SortableNode
	Value    *V13TreeValueNode
	Children []*V13SortedTreeNodeNode
}

// V13SortedTreeInitNode  sorted_tree_init = "[" "type" ":" inspect_type "," "root" ":" tree_node_sorted "]"
type V13SortedTreeInitNode struct {
	V13BaseNode
	TypeAnnot *V13InspectTypeNode
	Root      *V13SortedTreeNodeNode
}

// V13SortedTreeInsTailNode  sorted_tree_ins_tail = "+" … { "+" … }
type V13SortedTreeInsTailNode struct {
	V13BaseNode
	Additions []V13Node
}

// V13SortedTreeFinalNode  sorted_tree_final = ( TYPE_OF sorted_tree_init<ident_ref> | sorted_tree_init ) { … }
type V13SortedTreeFinalNode struct {
	V13BaseNode
	Base  V13Node
	Tails []*V13SortedTreeInsTailNode
}

// =============================================================================
// SET
// =============================================================================

// V13SetInitNode  set_init = "{" UNIQUE< set_value { "," set_value } > "}"
type V13SetInitNode struct {
	V13BaseNode
	Values []*V13HashableNode
}

// V13SetAddTailNode  set_add_tail = "+" ( TYPE_OF set_init<ident_ref> | set_init ) { "+" … }
type V13SetAddTailNode struct {
	V13BaseNode
	Additions []V13Node // *V13IdentRefNode | *V13SetInitNode
}

// V13SetOmitTailNode  set_omit_tail = "-" set_value { "," set_value }
type V13SetOmitTailNode struct {
	V13BaseNode
	Values []*V13HashableNode
}

// V13SetFinalNode  set_final = ( TYPE_OF set_init<ident_ref> | set_init ) { set_add_tail | set_omit_tail }
type V13SetFinalNode struct {
	V13BaseNode
	Base  V13Node   // *V13IdentRefNode | *V13SetInitNode
	Tails []V13Node // *V13SetAddTailNode | *V13SetOmitTailNode
}

// =============================================================================
// ENUM
// =============================================================================

// V13EnumMembersNode  enum_members = "[" UNIQUE< enum_member { "," enum_member } "]"
// enum_member = string_quoted | integer
type V13EnumMembersNode struct {
	V13BaseNode
	Members []V13Node // *V13StringQuotedNode | *V13IntegerNode
}

// V13EnumDeclNode  enum_decl = "ENUM" enum_members
type V13EnumDeclNode struct {
	V13BaseNode
	Members *V13EnumMembersNode
}

// V13EnumExtendNode  enum_extend = "EXTEND" enum_members
type V13EnumExtendNode struct {
	V13BaseNode
	Members *V13EnumMembersNode
}

// V13EnumFinalNode  enum_final = ( TYPE_OF enum_decl<ident_ref> | enum_decl ) { enum_extend }
type V13EnumFinalNode struct {
	V13BaseNode
	Base    V13Node // *V13IdentRefNode | *V13EnumDeclNode
	Extends []*V13EnumExtendNode
}

// =============================================================================
// GRAPH
// =============================================================================

// V13GraphNodeNode  graph_node = "[" "key" ":" unique_key "," "value" ":" tree_value "]"
type V13GraphNodeNode struct {
	V13BaseNode
	Key   *V13UniqueKeyNode
	Value *V13TreeValueNode
}

// V13GraphNodesNode  graph_nodes = HAS_ONE<unique_key>< "[" graph_node { "," graph_node } "]" >
type V13GraphNodesNode struct {
	V13BaseNode
	Nodes []*V13GraphNodeNode
}

// V13GraphEdgeNode  graph_edge = "[" "from" ":" unique_key "," "to" ":" unique_key
//
//	[ "," "label" ":" string_quoted ] "]"
type V13GraphEdgeNode struct {
	V13BaseNode
	From  *V13UniqueKeyNode
	To    *V13UniqueKeyNode
	Label *V13StringQuotedNode // optional
}

// V13GraphEdgesNode  graph_edges = "[" graph_edge { "," graph_edge } "]"
type V13GraphEdgesNode struct {
	V13BaseNode
	Edges []*V13GraphEdgeNode
}

// V13GraphInitNode  graph_init = "[" "nodes" ":" graph_nodes "," "edges" ":" graph_edges "]"
type V13GraphInitNode struct {
	V13BaseNode
	Nodes *V13GraphNodesNode
	Edges *V13GraphEdgesNode
}

// V13GraphAddTailNode  graph_add_tail = "+" ( TYPE_OF graph_init<ident_ref> | graph_init ) { "+" … }
type V13GraphAddTailNode struct {
	V13BaseNode
	Additions []V13Node
}

// V13GraphFinalNode  graph_final = ( TYPE_OF graph_init<ident_ref> | graph_init ) { graph_add_tail }
type V13GraphFinalNode struct {
	V13BaseNode
	Base  V13Node // *V13IdentRefNode | *V13GraphInitNode
	Tails []*V13GraphAddTailNode
}

// =============================================================================
// BITFIELD
// =============================================================================

// V13BitfieldFlagNode  bitfield_flag = ident_name ":" uint8
type V13BitfieldFlagNode struct {
	V13BaseNode
	Name     string
	Position uint8
}

// V13BitfieldFlagsNode  bitfield_flags = "[" UNIQUE< bitfield_flag { "," bitfield_flag } "]"
type V13BitfieldFlagsNode struct {
	V13BaseNode
	Flags []*V13BitfieldFlagNode
}

// V13BitfieldDeclNode  bitfield_decl = "BITFIELD" bitfield_base bitfield_flags
type V13BitfieldDeclNode struct {
	V13BaseNode
	Base  string // "uint8" | "uint16" | "uint32" | "uint64"
	Flags *V13BitfieldFlagsNode
}

// V13BitfieldFinalNode  bitfield_final = TYPE_OF bitfield_decl<ident_ref> | bitfield_decl
type V13BitfieldFinalNode struct {
	V13BaseNode
	Base V13Node // *V13IdentRefNode | *V13BitfieldDeclNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS
// =============================================================================

// ---------- unique_key / hashable / sortable ----------

// ParseUniqueKey parses:
//
//	unique_key = uuid_v7 | uuid | ulid | snowflake_id | nano_id |
//	             hash_key | seq_id | composite_key
//
// Tries most-specific variants first.
func (p *V13Parser) ParseUniqueKey() (*V13UniqueKeyNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13UniqueKeyNode {
		return &V13UniqueKeyNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	// composite_key starts with "("
	if p.cur().Type == V13_LPAREN {
		saved := p.savePos()
		ck, err := p.parseCompositeKey()
		if err == nil {
			return wrap(ck), nil
		}
		p.restorePos(saved)
	}

	if p.cur().Type == V13_IDENT {
		val := p.cur().Value
		// hash_*
		switch val {
		case "hash_md5", "hash_sha1", "hash_sha256", "hash_sha512":
			saved := p.savePos()
			hk, err := p.parseHashKey()
			if err == nil {
				return wrap(hk), nil
			}
			p.restorePos(saved)
		case "seq_id16", "seq_id32", "seq_id64":
			saved := p.savePos()
			si, err := p.parseSeqID()
			if err == nil {
				return wrap(si), nil
			}
			p.restorePos(saved)
		}
	}

	// uuid_v7 (needs version check) — try before uuid
	saved := p.savePos()
	if v7, err := p.ParseUUIDV7(); err == nil {
		return wrap(v7), nil
	}
	p.restorePos(saved)

	// uuid
	saved = p.savePos()
	if u, err := p.ParseUUID(); err == nil {
		return wrap(u), nil
	}
	p.restorePos(saved)

	// ulid
	saved = p.savePos()
	if ul, err := p.ParseULID(); err == nil {
		return wrap(ul), nil
	}
	p.restorePos(saved)

	// nano_id
	saved = p.savePos()
	if ni, err := p.ParseNanoID(); err == nil {
		return wrap(ni), nil
	}
	p.restorePos(saved)

	// snowflake_id (uint64 integer)
	saved = p.savePos()
	if sf, err := p.ParseSnowflakeID(); err == nil {
		return wrap(sf), nil
	}
	p.restorePos(saved)

	return nil, p.errAt(fmt.Sprintf("expected unique_key, got %s %q", p.cur().Type, p.cur().Value))
}

// parseHashKey parses:  hash_<algo> "(" string_quoted ")"
func (p *V13Parser) parseHashKey() (*V13HashKeyNode, error) {
	line, col := p.cur().Line, p.cur().Col
	kindTok := p.cur()
	if kindTok.Type != V13_IDENT {
		return nil, p.errAt("expected hash_key kind")
	}
	kind := kindTok.Value
	p.advance()
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	strTok := p.cur()
	if strTok.Type != V13_STRING && strTok.Type != V13_EMPTY_STR_D && strTok.Type != V13_EMPTY_STR_S {
		return nil, p.errAt("expected string inside hash_key")
	}
	val := strTok.Value
	p.advance()
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return &V13HashKeyNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Kind: kind, Value: val}, nil
}

// parseSeqID parses:  seq_id<N> "(" integer ")"
func (p *V13Parser) parseSeqID() (*V13SeqIDNode, error) {
	line, col := p.cur().Line, p.cur().Col
	kindTok := p.cur()
	if kindTok.Type != V13_IDENT {
		return nil, p.errAt("expected seq_id kind")
	}
	kind := kindTok.Value
	p.advance()
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	intTok := p.cur()
	if intTok.Type != V13_INTEGER {
		return nil, p.errAt("expected integer inside seq_id")
	}
	val := intTok.Value
	p.advance()
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return &V13SeqIDNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Kind: kind, Value: val}, nil
}

// parseCompositeKey parses:  composite_key = "(" array_value "," array_value { "," array_value } ")"
func (p *V13Parser) parseCompositeKey() (*V13CompositeKeyNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	first, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	second, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	parts := []V13Node{first, second}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		av, err := p.ParseArrayValue()
		if err != nil {
			p.restorePos(saved)
			break
		}
		parts = append(parts, av)
	}
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return &V13CompositeKeyNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Parts: parts}, nil
}

// ParseHashable parses:  hashable = unique_key | string_quoted | integer | boolean
func (p *V13Parser) ParseHashable() (*V13HashableNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13HashableNode {
		return &V13HashableNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V13_STRING, V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T:
		s, err := p.ParseStringQuoted()
		if err != nil {
			return nil, err
		}
		return wrap(s), nil
	case V13_INTEGER:
		i, err := p.ParseInteger()
		if err != nil {
			return nil, err
		}
		return wrap(i), nil
	case V13_TRUE, V13_FALSE:
		b, err := p.ParseBoolean()
		if err != nil {
			return nil, err
		}
		return wrap(b), nil
	}

	uk, err := p.ParseUniqueKey()
	if err != nil {
		return nil, fmt.Errorf("hashable: expected unique_key, string, integer, or boolean: %w", err)
	}
	return wrap(uk), nil
}

// ParseSortable parses a sortable value (grammar-identical to unique_key; runtime validates order).
func (p *V13Parser) ParseSortable() (*V13SortableNode, error) {
	line, col := p.cur().Line, p.cur().Col
	uk, err := p.ParseUniqueKey()
	if err != nil {
		return nil, err
	}
	return &V13SortableNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: uk}, nil
}

// ---------- Table ----------

// parseTableCol parses:  table_col = ident_ref inspect_type | inspect_type
func (p *V13Parser) parseTableCol() (*V13TableColNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// inspect_type starts with @, @?, TYPE_OF — try type-only first
	if p.cur().Type == V13_AT_IDENT || p.cur().Type == V13_ANY_TYPE || p.cur().Type == V13_TYPE_OF {
		it, err := p.ParseInspectType()
		if err != nil {
			return nil, err
		}
		return &V13TableColNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, InspType: it}, nil
	}

	// ident_ref followed by inspect_type
	saved := p.savePos()
	nameRef, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	it, err := p.ParseInspectType()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	return &V13TableColNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: nameRef, InspType: it}, nil
}

// ParseTableColumns parses:  table_columns = HAS_ONE<unique_key>< "[" table_col { "," table_col } "]" >
func (p *V13Parser) ParseTableColumns() (*V13TableColumnsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseTableCol()
	if err != nil {
		return nil, err
	}
	cols := []*V13TableColNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		// optional trailing comma
		if p.cur().Type == V13_RBRACKET {
			break
		}
		c, err := p.parseTableCol()
		if err != nil {
			p.restorePos(saved)
			break
		}
		cols = append(cols, c)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13TableColumnsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Cols: cols}, nil
}

// parseKeyCol parses:  key_col = ident_ref TYPE_OF unique_key<inspect_type> | TYPE_OF unique_key<inspect_type>
func (p *V13Parser) parseKeyCol() (*V13KeyColNode, error) {
	line, col := p.cur().Line, p.cur().Col

	if p.cur().Type == V13_TYPE_OF {
		it, err := p.ParseInspectType()
		if err != nil {
			return nil, err
		}
		return &V13KeyColNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, InspType: it}, nil
	}

	saved := p.savePos()
	nameRef, err := p.ParseIdentRef()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	it, err := p.ParseInspectType()
	if err != nil {
		p.restorePos(saved)
		return nil, err
	}
	return &V13KeyColNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: nameRef, InspType: it}, nil
}

// ParseKeyColumns parses:  key_columns = SUBSET_OF<table_columns>< "[" key_col { "," key_col } "]" >
func (p *V13Parser) ParseKeyColumns() (*V13KeyColumnsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseKeyCol()
	if err != nil {
		return nil, err
	}
	cols := []*V13KeyColNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		c, err := p.parseKeyCol()
		if err != nil {
			p.restorePos(saved)
			break
		}
		cols = append(cols, c)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13KeyColumnsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Cols: cols}, nil
}

// ParseTableRow parses:  table_row = "[" array_value { "," array_value } "]"
func (p *V13Parser) ParseTableRow() (*V13TableRowNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.ParseArrayValue()
	if err != nil {
		return nil, err
	}
	vals := []*V13ArrayValueNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		av, err := p.ParseArrayValue()
		if err != nil {
			p.restorePos(saved)
			break
		}
		vals = append(vals, av)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13TableRowNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Values: vals}, nil
}

// parseTableRowsList parses:  table_rows_list = UNIFORM INFER< table_row { "," table_row } >
func (p *V13Parser) parseTableRowsList() (*V13TableRowsListNode, error) {
	line, col := p.cur().Line, p.cur().Col
	// UNIFORM and INFER are directive annotations — skip them if present
	if p.cur().Type == V13_UNIFORM {
		p.advance()
	}
	if p.cur().Type == V13_INFER {
		p.advance()
	}
	first, err := p.ParseTableRow()
	if err != nil {
		return nil, err
	}
	rows := []*V13TableRowNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		r, err := p.ParseTableRow()
		if err != nil {
			p.restorePos(saved)
			break
		}
		rows = append(rows, r)
	}
	return &V13TableRowsListNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Rows: rows}, nil
}

// ParseTableInit parses:
//
//	table_init = "[" "columns" ":" (table_columns | table_col_from_obj) ","
//	                 "key_columns" ":" key_columns ","
//	                 "rows" ":" table_rows_list "]"
func (p *V13Parser) ParseTableInit() (*V13TableInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("columns"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}

	var columns V13Node
	if p.cur().Type == V13_TYPE_OF {
		// table_col_from_obj = HAS_ONE<unique_key>< TYPE_OF table_col_hashable<object_final> >
		saved := p.savePos()
		p.advance() // consume TYPE_OF
		of, err := p.ParseObjectFinal()
		if err != nil {
			p.restorePos(saved)
			tc, err2 := p.ParseTableColumns()
			if err2 != nil {
				return nil, err2
			}
			columns = tc
		} else {
			columns = &V13TableColFromObjNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, ObjRef: of}
		}
	} else {
		tc, err := p.ParseTableColumns()
		if err != nil {
			return nil, err
		}
		columns = tc
	}

	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("key_columns"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	kc, err := p.ParseKeyColumns()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("rows"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	rows, err := p.parseTableRowsList()
	if err != nil {
		return nil, err
	}
	// optional trailing comma
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13TableInitNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Columns:     columns,
		KeyColumns:  kc,
		Rows:        rows,
	}, nil
}

// parseTableInsTailStruct parses:  table_ins_tail = "+" ( TYPE_OF table_row<ident_ref> | table_row ) { "+" … }
func (p *V13Parser) parseTableInsTailStruct() (*V13TableInsTailStructNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			saved := p.savePos()
			p.advance()
			ref, err := p.ParseIdentRef()
			if err != nil {
				p.restorePos(saved)
				return nil, err
			}
			return ref, nil
		}
		return p.ParseTableRow()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	adds := []V13Node{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		adds = append(adds, item)
	}
	return &V13TableInsTailStructNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Additions: adds}, nil
}

// ParseTableFinal parses:
//
//	table_final = ( TYPE_OF table_init<ident_ref> | table_init ) { table_ins_tail }
//
// This is the V13 structured form.  The V12-style form is ParseTableFinalSimple.
func (p *V13Parser) ParseTableFinal() (*V13TableFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col

	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance() // consume TYPE_OF
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			// fall through to table_init
			ti, err2 := p.ParseTableInit()
			if err2 != nil {
				return nil, err2
			}
			base = ti
		} else {
			base = ref
		}
	} else {
		ti, err := p.ParseTableInit()
		if err != nil {
			return nil, err
		}
		base = ti
	}

	var tails []*V13TableInsTailStructNode
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		tail, err := p.parseTableInsTailStruct()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}
	return &V13TableFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Tails: tails}, nil
}

// ---------- Tree helpers ----------

// parseTreeValue parses:  tree_value = constant | ident_ref | calc_unit | object_final
func (p *V13Parser) parseTreeValue() (*V13TreeValueNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13TreeValueNode {
		return &V13TreeValueNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	saved := p.savePos()
	if of, err := p.ParseObjectFinal(); err == nil {
		return wrap(of), nil
	}
	p.restorePos(saved)

	saved = p.savePos()
	if cu, err := p.ParseCalcUnit(); err == nil {
		return wrap(cu), nil
	}
	p.restorePos(saved)

	saved = p.savePos()
	if ref, err := p.ParseIdentRef(); err == nil {
		return wrap(ref), nil
	}
	p.restorePos(saved)

	c, err := p.ParseConstant()
	if err != nil {
		return nil, err
	}
	return wrap(c), nil
}

// parseTreeNode is forward-declared via parseTreeNodeInner.
func (p *V13Parser) ParseTreeNode() (*V13TreeNodeNode, error) {
	return p.parseTreeNodeInner()
}

func (p *V13Parser) parseTreeNodeInner() (*V13TreeNodeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	val, err := p.parseTreeValue()
	if err != nil {
		return nil, err
	}
	node := &V13TreeNodeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}
	if p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			// trailing comma before ']'
		} else if p.cur().Value == "children" {
			p.advance() // consume "children"
			if _, err := p.expect(V13_COLON); err != nil {
				p.restorePos(saved)
			} else {
				children, err := p.parseTreeChildren()
				if err != nil {
					p.restorePos(saved)
				} else {
					node.Children = children
					// optional trailing comma
					if p.cur().Type == V13_COMMA {
						p.advance()
					}
				}
			}
		} else {
			p.restorePos(saved)
		}
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *V13Parser) parseTreeChildren() ([]*V13TreeNodeNode, error) {
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseTreeNodeInner()
	if err != nil {
		return nil, err
	}
	nodes := []*V13TreeNodeNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		n, err := p.parseTreeNodeInner()
		if err != nil {
			p.restorePos(saved)
			break
		}
		nodes = append(nodes, n)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (p *V13Parser) ParseTreeInit() (*V13TreeInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("type"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	typeAnnot, err := p.ParseInspectType()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("root"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	root, err := p.parseTreeNodeInner()
	if err != nil {
		return nil, err
	}
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13TreeInitNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		TypeAnnot:   typeAnnot,
		Root:        root,
	}, nil
}

func (p *V13Parser) parseTreeInsTail() (*V13TreeInsTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			saved := p.savePos()
			p.advance()
			ref, err := p.ParseIdentRef()
			if err != nil {
				p.restorePos(saved)
				return nil, err
			}
			return ref, nil
		}
		return p.parseTreeNodeInner()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	adds := []V13Node{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		adds = append(adds, item)
	}
	return &V13TreeInsTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Additions: adds}, nil
}

// ParseTreeFinal parses:  tree_final = ( TYPE_OF tree_init<ident_ref> | tree_init ) { tree_ins_tail }
func (p *V13Parser) ParseTreeFinal() (*V13TreeFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			ti, err2 := p.ParseTreeInit()
			if err2 != nil {
				return nil, err2
			}
			base = ti
		} else {
			base = ref
		}
	} else {
		ti, err := p.ParseTreeInit()
		if err != nil {
			return nil, err
		}
		base = ti
	}
	var tails []*V13TreeInsTailNode
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		tail, err := p.parseTreeInsTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}
	return &V13TreeFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Tails: tails}, nil
}

// ---------- String Tree ----------

func (p *V13Parser) parseStringTreeValue() (*V13StringTreeValueNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		if _, err := p.expectLit("string"); err != nil {
			p.restorePos(saved)
			goto fallback
		}
		if _, err := p.expect(V13_LT); err != nil {
			p.restorePos(saved)
			goto fallback
		}
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			goto fallback
		}
		if _, err := p.expect(V13_GT); err != nil {
			p.restorePos(saved)
			goto fallback
		}
		return &V13StringTreeValueNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, TypedOf: ref}, nil
	}
fallback:
	s, err := p.ParseStringQuoted()
	if err != nil {
		return nil, err
	}
	return &V13StringTreeValueNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Str: s}, nil
}

func (p *V13Parser) parseStringTreeNode() (*V13StringTreeNodeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	val, err := p.parseStringTreeValue()
	if err != nil {
		return nil, err
	}
	node := &V13StringTreeNodeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: val}
	if p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Value == "children" {
			p.advance()
			if _, err := p.expect(V13_COLON); err != nil {
				p.restorePos(saved)
			} else {
				children, err := p.parseStringTreeChildren()
				if err != nil {
					p.restorePos(saved)
				} else {
					node.Children = children
					if p.cur().Type == V13_COMMA {
						p.advance()
					}
				}
			}
		} else if p.cur().Type == V13_RBRACKET {
			// trailing comma
		} else {
			p.restorePos(saved)
		}
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *V13Parser) parseStringTreeChildren() ([]*V13StringTreeNodeNode, error) {
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseStringTreeNode()
	if err != nil {
		return nil, err
	}
	nodes := []*V13StringTreeNodeNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		n, err := p.parseStringTreeNode()
		if err != nil {
			p.restorePos(saved)
			break
		}
		nodes = append(nodes, n)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (p *V13Parser) ParseStringTreeInit() (*V13StringTreeInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("root"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	root, err := p.parseStringTreeNode()
	if err != nil {
		return nil, err
	}
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13StringTreeInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Root: root}, nil
}

func (p *V13Parser) parseStringTreeInsTail() (*V13StringTreeInsTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			saved := p.savePos()
			p.advance()
			ref, err := p.ParseIdentRef()
			if err != nil {
				p.restorePos(saved)
				return nil, err
			}
			return ref, nil
		}
		return p.parseStringTreeNode()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	adds := []V13Node{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		adds = append(adds, item)
	}
	return &V13StringTreeInsTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Additions: adds}, nil
}

// ParseStringTreeFinal parses:  string_tree_final = (TYPE_OF … | string_tree_init) { … }
func (p *V13Parser) ParseStringTreeFinal() (*V13StringTreeFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			si, err2 := p.ParseStringTreeInit()
			if err2 != nil {
				return nil, err2
			}
			base = si
		} else {
			base = ref
		}
	} else {
		si, err := p.ParseStringTreeInit()
		if err != nil {
			return nil, err
		}
		base = si
	}
	var tails []*V13StringTreeInsTailNode
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		tail, err := p.parseStringTreeInsTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}
	return &V13StringTreeFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Tails: tails}, nil
}

// ---------- Keyed Tree ----------

func (p *V13Parser) ParseKeyedTreeNode() (*V13KeyedTreeNodeNode, error) {
	return p.parseKeyedTreeNodeInner()
}

func (p *V13Parser) parseKeyedTreeNodeInner() (*V13KeyedTreeNodeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("key"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	key, err := p.ParseUniqueKey()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("value"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	val, err := p.parseTreeValue()
	if err != nil {
		return nil, err
	}
	node := &V13KeyedTreeNodeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Key: key, Value: val}
	if p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Value == "children" {
			p.advance()
			if _, err := p.expect(V13_COLON); err != nil {
				p.restorePos(saved)
			} else {
				children, err := p.parseKeyedTreeChildren()
				if err != nil {
					p.restorePos(saved)
				} else {
					node.Children = children
					if p.cur().Type == V13_COMMA {
						p.advance()
					}
				}
			}
		} else if p.cur().Type == V13_RBRACKET {
			// trailing comma
		} else {
			p.restorePos(saved)
		}
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *V13Parser) parseKeyedTreeChildren() ([]*V13KeyedTreeNodeNode, error) {
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseKeyedTreeNodeInner()
	if err != nil {
		return nil, err
	}
	nodes := []*V13KeyedTreeNodeNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		n, err := p.parseKeyedTreeNodeInner()
		if err != nil {
			p.restorePos(saved)
			break
		}
		nodes = append(nodes, n)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (p *V13Parser) ParseKeyedTreeInit() (*V13KeyedTreeInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("type"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	typeAnnot, err := p.ParseInspectType()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("root"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	root, err := p.parseKeyedTreeNodeInner()
	if err != nil {
		return nil, err
	}
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13KeyedTreeInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, TypeAnnot: typeAnnot, Root: root}, nil
}

func (p *V13Parser) parseKeyedTreeInsTail() (*V13KeyedTreeInsTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			saved := p.savePos()
			p.advance()
			ref, err := p.ParseIdentRef()
			if err != nil {
				p.restorePos(saved)
				return nil, err
			}
			return ref, nil
		}
		return p.parseKeyedTreeNodeInner()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	adds := []V13Node{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		adds = append(adds, item)
	}
	return &V13KeyedTreeInsTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Additions: adds}, nil
}

// ParseKeyedTreeFinal parses:  keyed_tree_final = (TYPE_OF … | keyed_tree_init) { … }
func (p *V13Parser) ParseKeyedTreeFinal() (*V13KeyedTreeFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			ki, err2 := p.ParseKeyedTreeInit()
			if err2 != nil {
				return nil, err2
			}
			base = ki
		} else {
			base = ref
		}
	} else {
		ki, err := p.ParseKeyedTreeInit()
		if err != nil {
			return nil, err
		}
		base = ki
	}
	var tails []*V13KeyedTreeInsTailNode
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		tail, err := p.parseKeyedTreeInsTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}
	return &V13KeyedTreeFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Tails: tails}, nil
}

// ---------- Sorted Tree ----------

func (p *V13Parser) parseSortedTreeNodeInner() (*V13SortedTreeNodeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("key"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	key, err := p.ParseSortable()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("value"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	val, err := p.parseTreeValue()
	if err != nil {
		return nil, err
	}
	node := &V13SortedTreeNodeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Key: key, Value: val}
	if p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Value == "children" {
			p.advance()
			if _, err := p.expect(V13_COLON); err != nil {
				p.restorePos(saved)
			} else {
				children, err := p.parseSortedTreeChildren()
				if err != nil {
					p.restorePos(saved)
				} else {
					node.Children = children
					if p.cur().Type == V13_COMMA {
						p.advance()
					}
				}
			}
		} else if p.cur().Type == V13_RBRACKET {
			// trailing comma
		} else {
			p.restorePos(saved)
		}
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *V13Parser) parseSortedTreeChildren() ([]*V13SortedTreeNodeNode, error) {
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseSortedTreeNodeInner()
	if err != nil {
		return nil, err
	}
	nodes := []*V13SortedTreeNodeNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		n, err := p.parseSortedTreeNodeInner()
		if err != nil {
			p.restorePos(saved)
			break
		}
		nodes = append(nodes, n)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (p *V13Parser) ParseSortedTreeInit() (*V13SortedTreeInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("type"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	typeAnnot, err := p.ParseInspectType()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("root"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	root, err := p.parseSortedTreeNodeInner()
	if err != nil {
		return nil, err
	}
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13SortedTreeInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, TypeAnnot: typeAnnot, Root: root}, nil
}

func (p *V13Parser) parseSortedTreeInsTail() (*V13SortedTreeInsTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			saved := p.savePos()
			p.advance()
			ref, err := p.ParseIdentRef()
			if err != nil {
				p.restorePos(saved)
				return nil, err
			}
			return ref, nil
		}
		return p.parseSortedTreeNodeInner()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	adds := []V13Node{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		adds = append(adds, item)
	}
	return &V13SortedTreeInsTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Additions: adds}, nil
}

// ParseSortedTreeFinal parses:  sorted_tree_final = (TYPE_OF … | sorted_tree_init) { … }
func (p *V13Parser) ParseSortedTreeFinal() (*V13SortedTreeFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			si, err2 := p.ParseSortedTreeInit()
			if err2 != nil {
				return nil, err2
			}
			base = si
		} else {
			base = ref
		}
	} else {
		si, err := p.ParseSortedTreeInit()
		if err != nil {
			return nil, err
		}
		base = si
	}
	var tails []*V13SortedTreeInsTailNode
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		tail, err := p.parseSortedTreeInsTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}
	return &V13SortedTreeFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Tails: tails}, nil
}

// ---------- Set ----------

// ParseSetInit parses:  set_init = "{" UNIQUE< set_value { "," set_value } > "}"
func (p *V13Parser) ParseSetInit() (*V13SetInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACE); err != nil {
		return nil, err
	}
	// UNIQUE is a directive annotation — skip if present
	if p.cur().Type == V13_UNIQUE {
		p.advance()
	}
	first, err := p.ParseHashable()
	if err != nil {
		return nil, err
	}
	vals := []*V13HashableNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACE {
			break
		}
		v, err := p.ParseHashable()
		if err != nil {
			p.restorePos(saved)
			break
		}
		vals = append(vals, v)
	}
	if _, err := p.expect(V13_RBRACE); err != nil {
		return nil, err
	}
	return &V13SetInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Values: vals}, nil
}

func (p *V13Parser) parseSetAddTail() (*V13SetAddTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			saved := p.savePos()
			p.advance()
			ref, err := p.ParseIdentRef()
			if err != nil {
				p.restorePos(saved)
				return nil, err
			}
			return ref, nil
		}
		return p.ParseSetInit()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	adds := []V13Node{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		adds = append(adds, item)
	}
	return &V13SetAddTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Additions: adds}, nil
}

func (p *V13Parser) parseSetOmitTail() (*V13SetOmitTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_MINUS); err != nil {
		return nil, err
	}
	first, err := p.ParseHashable()
	if err != nil {
		return nil, err
	}
	vals := []*V13HashableNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		v, err := p.ParseHashable()
		if err != nil {
			p.restorePos(saved)
			break
		}
		vals = append(vals, v)
	}
	return &V13SetOmitTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Values: vals}, nil
}

// ParseSetFinal parses:  set_final = ( TYPE_OF set_init<ident_ref> | set_init ) { set_add_tail | set_omit_tail }
func (p *V13Parser) ParseSetFinal() (*V13SetFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			si, err2 := p.ParseSetInit()
			if err2 != nil {
				return nil, err2
			}
			base = si
		} else {
			base = ref
		}
	} else {
		si, err := p.ParseSetInit()
		if err != nil {
			return nil, err
		}
		base = si
	}
	var tails []V13Node
	for p.cur().Type == V13_PLUS || p.cur().Type == V13_MINUS {
		saved := p.savePos()
		if p.cur().Type == V13_PLUS {
			tail, err := p.parseSetAddTail()
			if err != nil {
				p.restorePos(saved)
				break
			}
			tails = append(tails, tail)
		} else {
			tail, err := p.parseSetOmitTail()
			if err != nil {
				p.restorePos(saved)
				break
			}
			tails = append(tails, tail)
		}
	}
	return &V13SetFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Tails: tails}, nil
}

// ---------- Enum ----------

func (p *V13Parser) ParseEnumMembers() (*V13EnumMembersNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	// Skip UNIQUE directive if present
	if p.cur().Type == V13_UNIQUE {
		p.advance()
	}
	parseEnumMember := func() (V13Node, error) {
		switch p.cur().Type {
		case V13_STRING, V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T:
			return p.ParseStringQuoted()
		case V13_INTEGER, V13_PLUS, V13_MINUS:
			return p.ParseInteger()
		}
		return nil, p.errAt(fmt.Sprintf("expected string or integer enum member, got %s %q", p.cur().Type, p.cur().Value))
	}
	first, err := parseEnumMember()
	if err != nil {
		return nil, err
	}
	members := []V13Node{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		m, err := parseEnumMember()
		if err != nil {
			p.restorePos(saved)
			break
		}
		members = append(members, m)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13EnumMembersNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Members: members}, nil
}

func (p *V13Parser) ParseEnumDecl() (*V13EnumDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_ENUM); err != nil {
		return nil, err
	}
	members, err := p.ParseEnumMembers()
	if err != nil {
		return nil, err
	}
	return &V13EnumDeclNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Members: members}, nil
}

func (p *V13Parser) parseEnumExtend() (*V13EnumExtendNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_EXTEND); err != nil {
		return nil, err
	}
	members, err := p.ParseEnumMembers()
	if err != nil {
		return nil, err
	}
	return &V13EnumExtendNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Members: members}, nil
}

// ParseEnumFinal parses:  enum_final = ( TYPE_OF enum_decl<ident_ref> | enum_decl ) { enum_extend }
func (p *V13Parser) ParseEnumFinal() (*V13EnumFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			ed, err2 := p.ParseEnumDecl()
			if err2 != nil {
				return nil, err2
			}
			base = ed
		} else {
			base = ref
		}
	} else {
		ed, err := p.ParseEnumDecl()
		if err != nil {
			return nil, err
		}
		base = ed
	}
	var extends []*V13EnumExtendNode
	for p.cur().Type == V13_EXTEND {
		saved := p.savePos()
		ext, err := p.parseEnumExtend()
		if err != nil {
			p.restorePos(saved)
			break
		}
		extends = append(extends, ext)
	}
	return &V13EnumFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Extends: extends}, nil
}

// ---------- Graph ----------

func (p *V13Parser) parseGraphNode() (*V13GraphNodeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("key"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	key, err := p.ParseUniqueKey()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("value"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	val, err := p.parseTreeValue()
	if err != nil {
		return nil, err
	}
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13GraphNodeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Key: key, Value: val}, nil
}

func (p *V13Parser) parseGraphNodes() (*V13GraphNodesNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseGraphNode()
	if err != nil {
		return nil, err
	}
	nodes := []*V13GraphNodeNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		n, err := p.parseGraphNode()
		if err != nil {
			p.restorePos(saved)
			break
		}
		nodes = append(nodes, n)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13GraphNodesNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Nodes: nodes}, nil
}

func (p *V13Parser) parseGraphEdge() (*V13GraphEdgeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("from"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	from, err := p.ParseUniqueKey()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("to"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	to, err := p.ParseUniqueKey()
	if err != nil {
		return nil, err
	}
	edge := &V13GraphEdgeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, From: from, To: to}
	if p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Value == "label" {
			p.advance()
			if _, err := p.expect(V13_COLON); err != nil {
				p.restorePos(saved)
			} else {
				lbl, err := p.ParseStringQuoted()
				if err != nil {
					p.restorePos(saved)
				} else {
					edge.Label = lbl
					if p.cur().Type == V13_COMMA {
						p.advance()
					}
				}
			}
		} else if p.cur().Type == V13_RBRACKET {
			// trailing comma
		} else {
			p.restorePos(saved)
		}
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return edge, nil
}

func (p *V13Parser) parseGraphEdges() (*V13GraphEdgesNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := p.parseGraphEdge()
	if err != nil {
		return nil, err
	}
	edges := []*V13GraphEdgeNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		e, err := p.parseGraphEdge()
		if err != nil {
			p.restorePos(saved)
			break
		}
		edges = append(edges, e)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13GraphEdgesNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Edges: edges}, nil
}

func (p *V13Parser) ParseGraphInit() (*V13GraphInitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("nodes"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	nodes, err := p.parseGraphNodes()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COMMA); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("edges"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	edges, err := p.parseGraphEdges()
	if err != nil {
		return nil, err
	}
	if p.cur().Type == V13_COMMA {
		p.advance()
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13GraphInitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Nodes: nodes, Edges: edges}, nil
}

func (p *V13Parser) parseGraphAddTail() (*V13GraphAddTailNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_PLUS); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			saved := p.savePos()
			p.advance()
			ref, err := p.ParseIdentRef()
			if err != nil {
				p.restorePos(saved)
				return nil, err
			}
			return ref, nil
		}
		return p.ParseGraphInit()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	adds := []V13Node{first}
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		adds = append(adds, item)
	}
	return &V13GraphAddTailNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Additions: adds}, nil
}

// ParseGraphFinal parses:  graph_final = ( TYPE_OF graph_init<ident_ref> | graph_init ) { graph_add_tail }
func (p *V13Parser) ParseGraphFinal() (*V13GraphFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var base V13Node
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			gi, err2 := p.ParseGraphInit()
			if err2 != nil {
				return nil, err2
			}
			base = gi
		} else {
			base = ref
		}
	} else {
		gi, err := p.ParseGraphInit()
		if err != nil {
			return nil, err
		}
		base = gi
	}
	var tails []*V13GraphAddTailNode
	for p.cur().Type == V13_PLUS {
		saved := p.savePos()
		tail, err := p.parseGraphAddTail()
		if err != nil {
			p.restorePos(saved)
			break
		}
		tails = append(tails, tail)
	}
	return &V13GraphFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Tails: tails}, nil
}

// ---------- Bitfield ----------

// parseBitfieldFlag parses:  bitfield_flag = ident_name ":" uint8
func (p *V13Parser) parseBitfieldFlag() (*V13BitfieldFlagNode, error) {
	line, col := p.cur().Line, p.cur().Col
	nameTok := p.cur()
	if nameTok.Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected flag name, got %s %q", nameTok.Type, nameTok.Value))
	}
	name := nameTok.Value
	p.advance()
	if _, err := p.expect(V13_COLON); err != nil {
		return nil, err
	}
	posTok := p.cur()
	if posTok.Type != V13_INTEGER {
		return nil, p.errAt(fmt.Sprintf("expected bit position (uint8), got %s %q", posTok.Type, posTok.Value))
	}
	var pos uint8
	for _, ch := range posTok.Value {
		pos = pos*10 + uint8(ch-'0')
	}
	p.advance()
	return &V13BitfieldFlagNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: name, Position: pos}, nil
}

// parseBitfieldFlags parses:  bitfield_flags = "[" UNIQUE< bitfield_flag { "," bitfield_flag } "]"
func (p *V13Parser) parseBitfieldFlags() (*V13BitfieldFlagsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	if p.cur().Type == V13_UNIQUE {
		p.advance()
	}
	first, err := p.parseBitfieldFlag()
	if err != nil {
		return nil, err
	}
	flags := []*V13BitfieldFlagNode{first}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_RBRACKET {
			break
		}
		f, err := p.parseBitfieldFlag()
		if err != nil {
			p.restorePos(saved)
			break
		}
		flags = append(flags, f)
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	return &V13BitfieldFlagsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Flags: flags}, nil
}

// ParseBitfieldDecl parses:  bitfield_decl = "BITFIELD" bitfield_base bitfield_flags
func (p *V13Parser) ParseBitfieldDecl() (*V13BitfieldDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_BITFIELD); err != nil {
		return nil, err
	}
	baseTok := p.cur()
	if baseTok.Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected bitfield_base (uint8/uint16/uint32/uint64), got %s %q", baseTok.Type, baseTok.Value))
	}
	switch baseTok.Value {
	case "uint8", "uint16", "uint32", "uint64":
	default:
		return nil, p.errAt(fmt.Sprintf("bitfield_base must be uint8 | uint16 | uint32 | uint64, got %q", baseTok.Value))
	}
	base := baseTok.Value
	p.advance()
	flags, err := p.parseBitfieldFlags()
	if err != nil {
		return nil, err
	}
	return &V13BitfieldDeclNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: base, Flags: flags}, nil
}

// ParseBitfieldFinal parses:  bitfield_final = TYPE_OF bitfield_decl<ident_ref> | bitfield_decl
func (p *V13Parser) ParseBitfieldFinal() (*V13BitfieldFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance()
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			bd, err2 := p.ParseBitfieldDecl()
			if err2 != nil {
				return nil, err2
			}
			return &V13BitfieldFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: bd}, nil
		}
		return &V13BitfieldFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: ref}, nil
	}
	bd, err := p.ParseBitfieldDecl()
	if err != nil {
		return nil, err
	}
	return &V13BitfieldFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Base: bd}, nil
}

var _ = fmt.Sprintf
