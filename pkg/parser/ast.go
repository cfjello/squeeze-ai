// ast.go — AST node types for the Squeeze language parser (Phase 2 of plan).
//
// Design principles (from parser_v3.md §7):
//   - Every node carries: rule name, ordered children, metadata bag, source position.
//   - Token-level leaf nodes wrap a Token directly.
//   - Directive wrappers (UNIQUE, RANGE, TYPE_OF, …) are first-class node kinds
//     so the Phase 3 directive processor can walk and act on them uniformly.
package parser

import (
	"fmt"
	"strings"
)

// =============================================================================
// NodeKind — the kind of every AST node
// =============================================================================

// NodeKind identifies what a node represents in the grammar.
type NodeKind int

const (
	// ---- Leaf / terminal nodes ----
	NodeToken NodeKind = iota // wraps a single Token (identifier, literal, operator, keyword …)

	// ---- V1 — Literals & primitives ----
	NodeDigits
	NodeInteger
	NodeDecimal
	NodeNumericConst
	NodeSingleQuoted
	NodeDoubleQuoted
	NodeTmplQuoted
	NodeString
	NodeRegexpFlags
	NodeRegexp
	NodeBoolean
	NodeBooleanTrue
	NodeBooleanFalse
	NodeConstant

	// ---- V1 — Identifiers ----
	NodeIdentName
	NodeIdentDotted
	NodeIdentPrefix
	NodeIdentRef

	// ---- V1 — Numeric expressions ----
	NodeNumericOper
	NodeInlineIncr
	NodeSingleNumExpr
	NodeNumExprList
	NodeNumGrouping
	NodeNumericExpr

	// ---- V1 — String expressions ----
	NodeStringOper
	NodeStringExprList
	NodeStringGrouping
	NodeStringExpr

	// ---- V1 — Compare expressions ----
	NodeCompareOper
	NodeNumCompare
	NodeStringCompare
	NodeStringRegexpComp
	NodeCompareExpr

	// ---- V1 — Logic expressions ----
	NodeNotOper
	NodeLogicOper
	NodeSingleLogicExpr
	NodeLogicExprList
	NodeLogicGrouping
	NodeLogicExpr

	// ---- V1 — Assignment operators ----
	NodeIncrAssignImmutable
	NodeIncrAssignMutable
	NodeAssignMutable
	NodeAssignImmutable
	NodeAssignReadOnlyRef
	NodeEqualAssign
	NodeAssignOper

	// ---- V1 — Range ----
	NodeRange

	// ---- V3 — Empty initialisers ----
	NodeEmptyArrayDecl
	NodeFuncStreamDecl
	NodeFuncRegexpDecl
	NodeEmptyObjectDecl
	NodeFuncStringDecl
	NodeEmptyDecl

	// ---- V3 — Calc unit ----
	NodeCalcUnit

	// ---- V3 — LHS / RHS ----
	NodeAssignLHS
	NodeAssignRHS
	NodeAssignRHS4Object

	// ---- V3 — Arrays ----
	NodeArrayValues
	NodeArrayBegin
	NodeArrayEnd
	NodeArrayInit
	NodeArrayList
	NodeArrayLookup

	// ---- V3 — Objects ----
	NodeObjectBegin
	NodeObjectEnd
	NodeObjectInit
	NodeObjectList
	NodeObjectLookup

	// ---- V3 — Tables ----
	NodeTableHeader
	NodeTableEntry
	NodeTableObjects
	NodeTableInit

	// ---- V3 — Reflection getters ----
	NodeGetName
	NodeGetData
	NodeGetStoreName
	NodeGetType
	NodeGetTypeName

	// ---- V3 — Built-in header params ----
	NodeDataAssign
	NodeStoreNameAssign
	NodeFuncHeaderBuildinParams

	// ---- V3 — Function header ----
	NodeFuncHeaderAssign
	NodeFuncHeaderUserParams

	// ---- V3 — Static field identifiers ----
	NodeIdentStaticStoreName
	NodeIdentStaticStr
	NodeIdentStaticBoolean
	NodeIdentStaticError
	NodeIdentStaticDeps
	NodeIdentStaticFunc
	NodeIdentStaticFn

	// ---- V3 — Function argument declarations ----
	NodeFuncArgs
	NodeFuncStreamArgs
	NodeFuncDeps
	NodeFuncArgsDecl

	// ---- V3 — Function range declarations ----
	NodeFuncFixedNumRange
	NodeFuncFixedStrRange
	NodeFuncFixedListRange
	NodeFuncRangeArgs

	// ---- V3 — Function calls ----
	NodeFuncCallArgs
	NodeFuncCall1
	NodeFuncCall2
	NodeFuncCallDataChain
	NodeFuncCallLogicChain
	NodeFuncCallMixedChain

	// ---- V3 — Function body ----
	NodeFuncStreamLoop
	NodeFuncStmt
	NodeAssignment
	NodeFuncReturnStmt
	NodeFuncStoreStmt
	NodeFuncBodyStmt

	// ---- V3 — Function unit ----
	NodeFuncPrivateParams
	NodeFuncUnitHeader
	NodeFuncUnit

	// ---- V3 — Scope ----
	NodeScopeAssign
	NodeScopeUnit

	// ---- Top-level program node ----
	NodeProgram

	// ---- Directive wrapper nodes (Phase 3 processed) ----
	NodeDirectiveUnique    // UNIQUE<…>
	NodeDirectiveRange     // RANGE n..m<…>
	NodeDirectiveTypeOf    // TYPE_OF token<…>
	NodeDirectiveValueOf   // VALUE_OF<…>
	NodeDirectiveAddressOf // ADDRESS_OF<…>
	NodeDirectiveReturn    // RETURN (:|~)<…>
	NodeDirectiveUniform   // UNIFORM (token|INFER)<…>
	NodeDirectiveInfer     // INFER<…>

	// ---- Error / recovery node ----
	NodeError
)

// nodeKindNames maps NodeKind values to human-readable names.
var nodeKindNames = map[NodeKind]string{
	NodeToken: "Token",

	NodeDigits: "digits", NodeInteger: "integer", NodeDecimal: "decimal",
	NodeNumericConst: "numeric_const",
	NodeSingleQuoted: "single_quoted", NodeDoubleQuoted: "double_quoted",
	NodeTmplQuoted: "tmpl_quoted", NodeString: "string",
	NodeRegexpFlags: "regexp_flags", NodeRegexp: "regexp",
	NodeBoolean: "boolean", NodeBooleanTrue: "boolean_true",
	NodeBooleanFalse: "boolean_false", NodeConstant: "constant",

	NodeIdentName: "ident_name", NodeIdentDotted: "ident_dotted",
	NodeIdentPrefix: "ident_prefix", NodeIdentRef: "ident_ref",

	NodeNumericOper: "numeric_oper", NodeInlineIncr: "inline_incr",
	NodeSingleNumExpr: "single_num_expr", NodeNumExprList: "num_expr_list",
	NodeNumGrouping: "num_grouping", NodeNumericExpr: "numeric_expr",

	NodeStringOper: "string_oper", NodeStringExprList: "string_expr_list",
	NodeStringGrouping: "string_grouping", NodeStringExpr: "string_expr",

	NodeCompareOper: "compare_oper", NodeNumCompare: "num_compare",
	NodeStringCompare: "string_compare", NodeStringRegexpComp: "string_regexp_comp",
	NodeCompareExpr: "compare_expr",

	NodeNotOper: "not_oper", NodeLogicOper: "logic_oper",
	NodeSingleLogicExpr: "single_logic_expr", NodeLogicExprList: "logic_expr_list",
	NodeLogicGrouping: "logic_grouping", NodeLogicExpr: "logic_expr",

	NodeIncrAssignImmutable: "incr_assign_immutable",
	NodeIncrAssignMutable:   "incr_assign_mutable",
	NodeAssignMutable:       "assign_mutable", NodeAssignImmutable: "assign_immutable",
	NodeAssignReadOnlyRef: "assign_read_only_ref", NodeEqualAssign: "equal_assign",
	NodeAssignOper: "assign_oper",

	NodeRange: "range",

	NodeEmptyArrayDecl: "empty_array_decl", NodeFuncStreamDecl: "func_stream_decl",
	NodeFuncRegexpDecl: "func_regexp_decl", NodeEmptyObjectDecl: "empty_object_decl",
	NodeFuncStringDecl: "func_string_decl", NodeEmptyDecl: "empty_decl",

	NodeCalcUnit: "calc_unit",

	NodeAssignLHS: "assign_lhs", NodeAssignRHS: "assign_rhs",
	NodeAssignRHS4Object: "assign_rhs_4_object",

	NodeArrayValues: "array_values", NodeArrayBegin: "array_begin",
	NodeArrayEnd: "array_end", NodeArrayInit: "array_init",
	NodeArrayList: "array_list", NodeArrayLookup: "array_lookup",

	NodeObjectBegin: "object_begin", NodeObjectEnd: "object_end",
	NodeObjectInit: "object_init", NodeObjectList: "object_list",
	NodeObjectLookup: "object_lookup",

	NodeTableHeader: "table_header", NodeTableEntry: "table_entry",
	NodeTableObjects: "table_objects", NodeTableInit: "table_init",

	NodeGetName: "get_name", NodeGetData: "get_data",
	NodeGetStoreName: "get_store_name", NodeGetType: "get_type",
	NodeGetTypeName: "get_type_name",

	NodeDataAssign: "data_assign", NodeStoreNameAssign: "store_name_assign",
	NodeFuncHeaderBuildinParams: "func_header_buildin_params",

	NodeFuncHeaderAssign:     "func_header_assign",
	NodeFuncHeaderUserParams: "func_header_user_params",

	NodeIdentStaticStoreName: "ident_static_store_name",
	NodeIdentStaticStr:       "ident_static_str", NodeIdentStaticBoolean: "ident_static_boolean",
	NodeIdentStaticError: "ident_static_error", NodeIdentStaticDeps: "ident_static_deps",
	NodeIdentStaticFunc: "ident_static_func", NodeIdentStaticFn: "ident_static_fn",

	NodeFuncArgs: "func_args", NodeFuncStreamArgs: "func_stream_args",
	NodeFuncDeps: "func_deps", NodeFuncArgsDecl: "func_args_decl",

	NodeFuncFixedNumRange:  "func_fixed_num_range",
	NodeFuncFixedStrRange:  "func_fixed_str_range",
	NodeFuncFixedListRange: "func_fixed_list_range",
	NodeFuncRangeArgs:      "func_range_args",

	NodeFuncCallArgs: "func_call_args",
	NodeFuncCall1:    "func_call_1", NodeFuncCall2: "func_call_2",
	NodeFuncCallDataChain:  "func_call_data_chain",
	NodeFuncCallLogicChain: "func_call_logic_chain",
	NodeFuncCallMixedChain: "func_call_mixed_chain",

	NodeFuncStreamLoop: "func_stream_loop", NodeFuncStmt: "func_stmt",
	NodeAssignment: "assignment", NodeFuncReturnStmt: "func_return_stmt",
	NodeFuncStoreStmt: "func_store_stmt", NodeFuncBodyStmt: "func_body_stmt",

	NodeFuncPrivateParams: "func_private_params",
	NodeFuncUnitHeader:    "func_unit_header",
	NodeFuncUnit:          "func_unit",

	NodeScopeAssign: "scope_assign", NodeScopeUnit: "scope_unit",

	NodeProgram: "program",

	NodeDirectiveUnique: "UNIQUE", NodeDirectiveRange: "RANGE",
	NodeDirectiveTypeOf: "TYPE_OF", NodeDirectiveValueOf: "VALUE_OF",
	NodeDirectiveAddressOf: "ADDRESS_OF", NodeDirectiveReturn: "RETURN",
	NodeDirectiveUniform: "UNIFORM", NodeDirectiveInfer: "INFER",

	NodeError: "ERROR",
}

func (k NodeKind) String() string {
	if s, ok := nodeKindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("NodeKind(%d)", int(k))
}

// =============================================================================
// Pos — source position
// =============================================================================

// Pos carries a (line, col) source location. Line is 1-based, Col is 0-based,
// matching the values the Lexer attaches to every Token.
type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string { return fmt.Sprintf("L%d:C%d", p.Line, p.Col) }

// =============================================================================
// Metadata — directive annotation bag
// =============================================================================

// Metadata holds the @-prefixed annotations that the directive processor
// attaches to AST nodes (§7 of parser_v3.md).
// Fields are pointers so their zero value (nil) means "not set".
type Metadata struct {
	Name      *string  // @name  — the identifier name as assigned by the parser
	TypeRef   *string  // @type  — ident_ref pointing to the type
	TypeName  *string  // @typeName — the static name of the type
	Data      *Node    // @data  — sub-tree holding the data object
	StoreName *string  // @storeName — string used when storing via =>
	Ok        *bool    // @ok    — last-operation success flag
	Err       *string  // @error — last-operation error message
	Deps      []string // @deps  — list of store-name dependencies

	// Directive-specific fields set by the directive processor.
	DirectiveKind NodeKind // which directive wraps this node (if any)
	DirectiveArg  string   // the string argument to RANGE / TYPE_OF / RETURN / UNIFORM
	InferredType  string   // type inferred by INFER post-pass
	IsValueOf     bool     // true when VALUE_OF applies
	IsAddressOf   bool     // true when ADDRESS_OF applies
}

// =============================================================================
// Node — the core AST type
// =============================================================================

// Node is a single node in the abstract syntax tree.
//
// Leaf nodes (NodeToken) have a non-nil Tok and an empty Children slice.
// Interior nodes have a nil Tok and a non-empty Children slice.
// Directive nodes have Kind set to one of NodeDirective* and carry their
// enclosed child sub-tree in Children[0].
type Node struct {
	Kind     NodeKind // which grammar rule / construct this node represents
	Tok      *Token   // non-nil only for NodeToken leaf nodes
	Children []*Node  // ordered list of child nodes
	Meta     Metadata // directive annotations & @-properties
	Pos      Pos      // source position of the first token in this production
}

// IsLeaf reports whether the node is a terminal token node.
func (n *Node) IsLeaf() bool { return n.Kind == NodeToken && n.Tok != nil }

// IsError reports whether the node represents a parse error.
func (n *Node) IsError() bool { return n.Kind == NodeError }

// TokenValue returns the token value for a leaf NodeToken node, or "" otherwise.
func (n *Node) TokenValue() string {
	if n.Tok != nil {
		return n.Tok.Value
	}
	return ""
}

// String produces a compact single-line representation of the node.
func (n *Node) String() string {
	if n.IsLeaf() {
		return fmt.Sprintf("(%s %q %s)", n.Kind, n.Tok.Value, n.Pos)
	}
	return fmt.Sprintf("(%s [%d children] %s)", n.Kind, len(n.Children), n.Pos)
}

// =============================================================================
// Node constructor helpers
// =============================================================================

// NewTokenNode wraps a lexer Token as a leaf AST node.
func NewTokenNode(tok Token) *Node {
	return &Node{
		Kind: NodeToken,
		Tok:  &tok,
		Pos:  Pos{Line: tok.Line, Col: tok.Col},
	}
}

// NewNode creates an interior AST node with the given kind, source position,
// and zero or more children.
func NewNode(kind NodeKind, pos Pos, children ...*Node) *Node {
	return &Node{
		Kind:     kind,
		Children: children,
		Pos:      pos,
	}
}

// NewDirectiveNode creates a directive-wrapper node.
//
//	kind     — one of NodeDirective*
//	arg      — the string argument (type name for TYPE_OF, "n..m" for RANGE, etc.)
//	pos      — source position of the directive keyword
//	enclosed — the sub-tree inside the <…> brackets
func NewDirectiveNode(kind NodeKind, arg string, pos Pos, enclosed *Node) *Node {
	n := &Node{
		Kind:     kind,
		Children: []*Node{enclosed},
		Pos:      pos,
	}
	n.Meta.DirectiveKind = kind
	n.Meta.DirectiveArg = arg
	return n
}

// NewErrorNode creates an error node with a descriptive message and position.
func NewErrorNode(msg string, pos Pos) *Node {
	tok := Token{Type: TOK_ILLEGAL, Value: msg, Line: pos.Line, Col: pos.Col}
	return &Node{
		Kind: NodeError,
		Tok:  &tok,
		Pos:  pos,
	}
}

// =============================================================================
// Tree helpers
// =============================================================================

// Append adds children to an existing node and returns the node (fluent style).
func (n *Node) Append(children ...*Node) *Node {
	n.Children = append(n.Children, children...)
	return n
}

// Walk performs a depth-first pre-order traversal of the AST, calling fn on
// every node. If fn returns false the subtree below that node is skipped.
func (n *Node) Walk(fn func(*Node) bool) {
	if !fn(n) {
		return
	}
	for _, child := range n.Children {
		child.Walk(fn)
	}
}

// FindAll returns all nodes in the subtree (depth-first) whose Kind matches
// any of the supplied kinds.
func (n *Node) FindAll(kinds ...NodeKind) []*Node {
	set := make(map[NodeKind]struct{}, len(kinds))
	for _, k := range kinds {
		set[k] = struct{}{}
	}
	var result []*Node
	n.Walk(func(node *Node) bool {
		if _, ok := set[node.Kind]; ok {
			result = append(result, node)
		}
		return true
	})
	return result
}

// =============================================================================
// Pretty-printer
// =============================================================================

// Pretty returns a multi-line indented tree representation of the AST.
// Useful for debugging and test output.
func (n *Node) Pretty() string {
	var sb strings.Builder
	n.pretty(&sb, 0)
	return sb.String()
}

func (n *Node) pretty(sb *strings.Builder, depth int) {
	indent := strings.Repeat("  ", depth)
	if n.IsLeaf() {
		fmt.Fprintf(sb, "%s(%s %q @ %s)\n", indent, n.Kind, n.Tok.Value, n.Pos)
		return
	}
	directive := ""
	if n.Meta.DirectiveArg != "" {
		directive = fmt.Sprintf(" [arg=%q]", n.Meta.DirectiveArg)
	}
	fmt.Fprintf(sb, "%s(%s%s @ %s\n", indent, n.Kind, directive, n.Pos)
	for _, child := range n.Children {
		child.pretty(sb, depth+1)
	}
	fmt.Fprintf(sb, "%s)\n", indent)
}
