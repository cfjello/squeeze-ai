// parser_v10_functions.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V10 grammar rule set defined in spec/06_functions.sqg.
//
// Covered rules:
//
//	inspect_store_name, inspect_type_name, inspect_data, inspect_type
//	data_assign, store_name_assign, func_header_buildin_params
//	func_enclosure_params
//	assign_func_rhs, func_header_assign, func_header_user_params
//	ident_static_store_name, ident_static_str, ident_static_boolean,
//	  ident_static_error, ident_static_deps, ident_static_func, ident_static_fn
//	func_args, func_stream_args, func_deps, func_args_decl
//	func_fixed_num_range, func_fixed_str_range, func_fixed_list_range, func_range_args
//	func_call_args, func_call, func_call_chain
//	func_stream_loop, func_call_final
//	self_ref
//	func_inject, func_stmt, func_assign
//	func_return_stmt, func_store_stmt, func_body_stmt
//	scope_begin, scope_end, func_unit_header, func_unit
//	return_func_unit, EXTEND<func_stmt>, update_func_unit
//	array_idx_recursive, EXTEND<ident_ref>
//	numeric_stmt, numeric_rhs, update_number
//	string_stmt, string_rhs, string_append_immutable, string_append_mutable,
//	  string_update_oper, update_string
//	ident_ref_update
//	MERGE<assignment>
package parser

import "fmt"

// =============================================================================
// PHASE 2 — AST NODE TYPES  (06_functions.sqg)
// =============================================================================

// ---------- built-in variable references ----------

// V10InspectAccessNode represents  ident_ref "." "@<property>"
// Used by inspect_store_name, inspect_type_name, inspect_data, inspect_type.
type V10InspectAccessNode struct {
	v10BaseNode
	Ref      *V10IdentRefNode
	PropName string // "@storeName", "@typeName", "@data", "@type"
}

// ---------- data_assign / store_name_assign ----------

// V10DataAssignNode  data_assign = ident_ref "." "@data" "=" ( object_final | array_final | constant )
type V10DataAssignNode struct {
	v10BaseNode
	Target *V10InspectAccessNode
	Value  V10Node // *V10ObjectFinalNode | *V10ArrayFinalNode | *V10ConstantNode
}

// V10StoreNameAssignNode  store_name_assign = ident_ref "." "@storeName" "=" string
type V10StoreNameAssignNode struct {
	v10BaseNode
	Target *V10InspectAccessNode
	Value  *V10StringNode
}

// ---------- func header / enclosure ----------

// V10FuncHeaderBuildinNode  func_header_buildin_params = store_name_assign | data_assign
type V10FuncHeaderBuildinNode struct {
	v10BaseNode
	Value V10Node // *V10StoreNameAssignNode | *V10DataAssignNode
}

// V10FuncEnclosureParamsNode  func_enclosure_params = ( buildin | user ) { EOL ( buildin | user ) }
type V10FuncEnclosureParamsNode struct {
	v10BaseNode
	Items []V10Node // *V10FuncHeaderBuildinNode | *V10FuncHeaderAssignNode
}

// V10AssignFuncRHSNode  assign_func_rhs = constant | func_string_decl | (regexp | func_regexp_decl)
//
//	| ident_ref | calc_unit | object_final | array_final
type V10AssignFuncRHSNode struct {
	v10BaseNode
	Value V10Node
}

// V10FuncHeaderAssignNode  func_header_assign = assign_lhs assign_oper assign_func_rhs | func_header_buildin_params
type V10FuncHeaderAssignNode struct {
	v10BaseNode
	Buildin *V10FuncHeaderBuildinNode // non-nil when buildin form
	LHS     *V10AssignLHSNode         // non-nil when assign form
	Oper    *V10AssignOperNode
	RHS     *V10AssignFuncRHSNode
}

// V10FuncHeaderUserParamsNode  func_header_user_params = func_header_assign { EOL func_header_assign }
type V10FuncHeaderUserParamsNode struct {
	v10BaseNode
	Items []*V10FuncHeaderAssignNode
}

// ---------- ident_static_fn ----------

// V10IdentStaticFnNode  ident_static_fn = ident_ref "." "@" ( str | bool | error | deps | func )
type V10IdentStaticFnNode struct {
	v10BaseNode
	Ref    *V10IdentRefNode
	Static string // "@name", "@type", "@storeName", "ok", "error", "deps", "next", "value"
}

// ---------- func_args_decl ----------

// V10FuncArgEntryNode  (V10 legacy) assign_lhs assign_oper ( empty_decl | ident_static_fn )
// Kept for backward compatibility; V11 uses V10ArgsDeclNode instead.
type V10FuncArgEntryNode struct {
	v10BaseNode
	LHS   *V10AssignLHSNode
	Oper  *V10AssignOperNode
	Value V10Node // *V10EmptyDeclNode | *V10IdentStaticFnNode
}

// V10ArgsDeclNode  (V11) args_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] inspect_type { "," args_decl }
type V10ArgsDeclNode struct {
	v10BaseNode
	LHS  *V10AssignLHSNode
	Oper *V10AssignOperNode    // nil when operator is absent
	Type *V10InspectAccessNode // the inspect_type  ident_ref "." "@<prop>"
}

// V10FuncArgsNode  func_args = "->" args_decl { "," args_decl }
type V10FuncArgsNode struct {
	v10BaseNode
	Entries []*V10ArgsDeclNode
}

// V10FuncStreamArgsNode  func_stream_args = ">>" args_decl { "," args_decl }
type V10FuncStreamArgsNode struct {
	v10BaseNode
	Entries []*V10ArgsDeclNode
}

// V10FuncDepsNode  func_deps = "=>" UNIQUE< ident_static_store_name { "," ident_static_store_name } >
type V10FuncDepsNode struct {
	v10BaseNode
	StoreNames []string // "@storeName" strings
}

// V10FuncArgsDeclNode  func_args_decl = [ func_args ] [ func_stream_args ] [ func_deps ]
type V10FuncArgsDeclNode struct {
	v10BaseNode
	Args       *V10FuncArgsNode
	StreamArgs *V10FuncStreamArgsNode
	Deps       *V10FuncDepsNode
}

// ---------- func_range_args ----------

// V10FuncFixedNumRangeNode  numeric_const ".." numeric_const  (or TYPE_OF variants)
type V10FuncFixedNumRangeNode struct {
	v10BaseNode
	Lo V10Node // *V10NumericConstNode | *V10TypeOfRefNode
	Hi V10Node // *V10NumericConstNode | *V10TypeOfRefNode
}

// V10FuncRangeArgsNode  func_range_args = num_range | str_range | list_range
type V10FuncRangeArgsNode struct {
	v10BaseNode
	Value V10Node // *V10FuncFixedNumRangeNode | *V10StringNode | *V10TypeOfRefNode | *V10ArrayFinalNode | *V10ObjectFinalNode
}

// ---------- func_call / func_call_chain / func_stream_loop ----------

// V10FuncCallNode  func_call = TYPE_OF func_unit<ident_ref> [ func_call_args ]
type V10FuncCallNode struct {
	v10BaseNode
	Ref  *V10TypeOfRefNode
	Args []V10Node // *V10AssignFuncRHSNode items
}

// V10FuncCallChainStepNode  one ( "->" | ">>" | logic_oper ) TYPE_OF func_unit<ident_ref> step
type V10FuncCallChainStepNode struct {
	v10BaseNode
	Op  string // "->", ">>", or logic operator string
	Ref *V10TypeOfRefNode
}

// V10FuncCallChainNode  func_call_chain = func_call [ step { step } ]
type V10FuncCallChainNode struct {
	v10BaseNode
	Head  *V10FuncCallNode
	Steps []V10FuncCallChainStepNode
}

// V10FuncStreamLoopNode
//
//	func_stream_loop = ( func_range_args | ident_ref | boolean_true ) ">>" ( func_unit | func_call )
type V10FuncStreamLoopNode struct {
	v10BaseNode
	Source V10Node // *V10FuncRangeArgsNode | *V10IdentRefNode | *V10BooleanNode ("true")
	Body   V10Node // *V10FuncUnitNode | *V10FuncCallNode
}

// V10FuncCallFinalNode  func_call_final = func_call_chain | func_stream_loop
type V10FuncCallFinalNode struct {
	v10BaseNode
	Value V10Node // *V10FuncCallChainNode | *V10FuncStreamLoopNode
}

// ---------- func_unit body ----------

// V10SelfRefNode  self_ref = "$"
type V10SelfRefNode struct{ v10BaseNode }

// V10FuncInjectNode  (V11)
//
//	func_inject = "(" [ ( assign_lhs inspect_type [ empty_array_decl ] )
//	                   | ( [ assign_lhs ( ":" | ":~" ) ] ident_ref ) ]
//	               { "," assign_lhs ( ":" | ":~" ) ident_ref } ")"
type V10FuncInjectNode struct {
	v10BaseNode
	Head  V10Node             // *V10FuncInjectHeadInspectNode | *V10FuncInjectBindNode | nil (empty)
	Binds []V10FuncInjectBind // additional  assign_lhs ( ":" | ":~" ) ident_ref items
}

// V10FuncInjectBind  assign_lhs ( ":" | ":~" ) ident_ref  (operator is optional for the head)
type V10FuncInjectBind struct {
	LHS  *V10AssignLHSNode  // nil when no lhs in head
	Oper *V10AssignOperNode // nil when operator absent (head only)
	Ref  *V10IdentRefNode
}

// V10FuncInjectBindNode  wraps V10FuncInjectBind as a V10Node so it can be stored in Head.
type V10FuncInjectBindNode struct {
	v10BaseNode
	Bind V10FuncInjectBind
}

// V10FuncInjectHeadInspectNode  assign_lhs inspect_type [ empty_array_decl ]
type V10FuncInjectHeadInspectNode struct {
	v10BaseNode
	LHS      *V10AssignLHSNode
	Inspect  *V10InspectAccessNode
	HasArray bool // true when followed by "[]"
}

// V10FuncStmtNode  func_stmt = regexp | ident_ref | object_final | array_final
//
//	| func_call_chain | func_unit | calc_unit | self_ref
type V10FuncStmtNode struct {
	v10BaseNode
	// Extended by EXTEND<func_stmt> to also include return_func_unit.
	Value V10Node
}

// V10FuncAssignNode  func_assign = [ func_inject ] assign_lhs assign_oper func_stmt
type V10FuncAssignNode struct {
	v10BaseNode
	Inject *V10FuncInjectNode // nil when no injection
	LHS    *V10AssignLHSNode
	Oper   *V10AssignOperNode
	Stmt   *V10FuncStmtNode
}

// V10FuncReturnStmtNode  func_return_stmt = "<-" func_stmt
type V10FuncReturnStmtNode struct {
	v10BaseNode
	Stmt *V10FuncStmtNode
}

// V10FuncStoreStmtNode  func_store_stmt = "=>" ( object_final | TYPE_OF object_final<ident_ref> ) { "," ... }
type V10FuncStoreStmtNode struct {
	v10BaseNode
	Items []V10Node // *V10ObjectFinalNode | *V10TypeOfRefNode
}

// V10FuncBodyStmtNode  func_body_stmt = func_assign | func_return_stmt | func_store_stmt
//
//	| func_stream_loop | func_call_final
type V10FuncBodyStmtNode struct {
	v10BaseNode
	Value V10Node
}

// ---------- func_unit ----------

// V10FuncUnitHeaderNode  func_unit_header = [ (enclosure func_args_decl) | func_args_decl ]
type V10FuncUnitHeaderNode struct {
	v10BaseNode
	Enclosure *V10FuncEnclosureParamsNode // nil when absent
	ArgsDecl  *V10FuncArgsDeclNode
}

// V10FuncUnitNode  func_unit = "{" func_unit_header func_body_stmt { NL | func_body_stmt } "}"
type V10FuncUnitNode struct {
	v10BaseNode
	Header *V10FuncUnitHeaderNode
	Body   []*V10FuncBodyStmtNode
}

// ---------- function-related updates ----------

// V10ReturnFuncUnitNode  return_func_unit = "<-" func_unit
type V10ReturnFuncUnitNode struct {
	v10BaseNode
	Unit *V10FuncUnitNode
}

// V10UpdateFuncUnitNode  update_func_unit = TYPE_OF func_unit<ident_ref> equal_assign return_func_unit
type V10UpdateFuncUnitNode struct {
	v10BaseNode
	Ref     *V10TypeOfRefNode
	Assign  *V10AssignOperNode
	NewUnit *V10ReturnFuncUnitNode
}

// V10ArrayIdxRecursiveNode  array_idx_recursive = "[" numeric_rhs "]" { "[" numeric_rhs "]" }
type V10ArrayIdxRecursiveNode struct {
	v10BaseNode
	Indices []V10Node // numeric_rhs values
}

// V10NumericStmtNode  numeric_stmt = TYPE_OF numeric_const<ident_ref | func_call_chain | func_unit | calc_unit>
type V10NumericStmtNode struct {
	v10BaseNode
	Ref V10TypeOfRefNode
}

// V10UpdateNumberNode  update_number = TYPE_OF numeric_const<ident_ref> assign_oper numeric_rhs
type V10UpdateNumberNode struct {
	v10BaseNode
	Target *V10TypeOfRefNode
	Oper   *V10AssignOperNode
	RHS    V10Node // *V10NumericConstNode | *V10NumericStmtNode
}

// V10StringStmtNode  string_stmt = TYPE_OF string<ident_ref | func_call_chain | func_unit | calc_unit>
type V10StringStmtNode struct {
	v10BaseNode
	Ref V10TypeOfRefNode
}

// V10StringUpdateOperKind classifies the string update operator.
type V10StringUpdateOperKind int

const (
	StringAppendImmutable V10StringUpdateOperKind = iota // +:
	StringAppendMutable                                  // +~
	StringEqualAssign                                    // = : :~
)

// V10StringUpdateOperNode  string_update_oper = "+:" | "+~" | equal_assign
type V10StringUpdateOperNode struct {
	v10BaseNode
	Kind  V10StringUpdateOperKind
	Token string
}

// V10UpdateStringNode  update_string = TYPE_OF string<ident_ref> string_update_oper string_rhs
type V10UpdateStringNode struct {
	v10BaseNode
	Target *V10TypeOfRefNode
	Oper   *V10StringUpdateOperNode
	RHS    V10Node // *V10StringNode | *V10StringStmtNode
}

// V10IdentRefUpdateNode  ident_ref_update = ident_ref assign_oper assign_rhs
type V10IdentRefUpdateNode struct {
	v10BaseNode
	Ref  *V10IdentRefNode
	Oper *V10AssignOperNode
	RHS  *V10AssignRHSNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (06_functions.sqg)
// =============================================================================

// ---------- helpers ----------

// v10isLogicOper returns the string form of a logic operator token, or "".
// Covers "&&", "||" — used in func_call_chain step discrimination.
func v10isBinaryLogicOper(t V10TokenType) string {
	switch t {
	case V10_AMP_AMP:
		return "&&"
	case V10_PIPE_PIPE:
		return "||"
	}
	return ""
}

// v10skipNLs advances past any NL tokens (for rules that use "EOL" as separator).
func (p *V10Parser) v10skipNLs() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == V10_NL {
		p.pos++
	}
}

// ---------- inspect_access ----------

// parseInspectAccess parses  ident_ref "." "@<property>" and validates the property name.
func (p *V10Parser) parseInspectAccess(allowedProps []string) (*V10InspectAccessNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_DOT); err != nil {
		return nil, err
	}
	tok := p.cur()
	if tok.Type != V10_AT_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected @<property> after '.', got %s %q", tok.Type, tok.Value))
	}
	if len(allowedProps) > 0 {
		found := false
		for _, a := range allowedProps {
			if tok.Value == a {
				found = true
				break
			}
		}
		if !found {
			return nil, p.errAt(fmt.Sprintf("expected one of %v, got %q", allowedProps, tok.Value))
		}
	}
	prop := tok.Value
	p.advance()
	return &V10InspectAccessNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Ref:         ref,
		PropName:    prop,
	}, nil
}

// ---------- data_assign / store_name_assign ----------

// ParseDataAssign parses:  ident_ref "." "@data" "=" ( object_final | array_final | constant )
func (p *V10Parser) ParseDataAssign() (*V10DataAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	access, err := p.parseInspectAccess([]string{"@data"})
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_EQ); err != nil {
		return nil, err
	}
	var val V10Node
	switch p.cur().Type {
	case V10_LBRACKET, V10_EMPTY_ARR, V10_UNIFORM:
		saved := p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			val = of
		} else {
			p.restorePos(saved)
			af, err := p.ParseArrayFinal()
			if err != nil {
				return nil, err
			}
			val = af
		}
	default:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		val = c
	}
	return &V10DataAssignNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Target: access, Value: val}, nil
}

// ParseStoreNameAssign parses:  ident_ref "." "@storeName" "=" string
func (p *V10Parser) ParseStoreNameAssign() (*V10StoreNameAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	access, err := p.parseInspectAccess([]string{"@storeName"})
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_EQ); err != nil {
		return nil, err
	}
	str, err := p.ParseString()
	if err != nil {
		return nil, err
	}
	return &V10StoreNameAssignNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Target: access, Value: str}, nil
}

// parseFuncHeaderBuildin parses:  store_name_assign | data_assign
// Tries store_name_assign first (more specific property name).
func (p *V10Parser) parseFuncHeaderBuildin() (*V10FuncHeaderBuildinNode, error) {
	line, col := p.cur().Line, p.cur().Col
	saved := p.savePos()
	if sna, err := p.ParseStoreNameAssign(); err == nil {
		return &V10FuncHeaderBuildinNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: sna}, nil
	}
	p.restorePos(saved)
	da, err := p.ParseDataAssign()
	if err != nil {
		return nil, err
	}
	return &V10FuncHeaderBuildinNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: da}, nil
}

// ---------- assign_func_rhs ----------

// ParseAssignFuncRHS parses:
//
//	assign_func_rhs = constant | func_string_decl | (regexp | func_regexp_decl) | ident_ref | calc_unit | object_final | array_final
func (p *V10Parser) ParseAssignFuncRHS() (*V10AssignFuncRHSNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V10Node) *V10AssignFuncRHSNode {
		return &V10AssignFuncRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	// func_string_decl (empty string literals)
	case V10_EMPTY_STR_D, V10_EMPTY_STR_S, V10_EMPTY_STR_T:
		n, err := p.ParseEmptyDecl()
		if err != nil {
			return nil, err
		}
		return wrap(n), nil

	// regexp | func_regexp_decl
	case V10_REGEXP, V10_REGEXP_DECL:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil

	// other constant starts
	case V10_TRUE, V10_FALSE, V10_NULL, V10_NAN, V10_INFINITY, V10_STRING:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil

	// object / array
	case V10_LBRACKET, V10_EMPTY_ARR, V10_UNIFORM:
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

	// numeric literal — could be a constant or calc_unit; try constant first
	case V10_INTEGER, V10_DECIMAL, V10_PLUS, V10_MINUS:
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil && !v10isExprContinuation(p.cur().Type) {
			return wrap(c), nil
		}
		p.restorePos(saved)
	}

	// ident_ref or calc_unit — try calc_unit (which includes ident_ref)
	cu, err := p.ParseCalcUnit()
	if err != nil {
		return nil, err
	}
	return wrap(cu), nil
}

// ---------- func_header_user_params ----------

// parseFuncHeaderAssign parses:
//
//	func_header_assign = assign_lhs assign_oper assign_func_rhs | func_header_buildin_params
func (p *V10Parser) parseFuncHeaderAssign() (*V10FuncHeaderAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	// Try buildin form first.
	saved := p.savePos()
	if bd, err := p.parseFuncHeaderBuildin(); err == nil {
		return &V10FuncHeaderAssignNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Buildin: bd}, nil
	}
	p.restorePos(saved)

	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}
	rhs, err := p.ParseAssignFuncRHS()
	if err != nil {
		return nil, err
	}
	return &V10FuncHeaderAssignNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		RHS:         rhs,
	}, nil
}

// parseFuncHeaderUserParams parses:  func_header_user_params = func_header_assign { EOL func_header_assign }
func (p *V10Parser) parseFuncHeaderUserParams() (*V10FuncHeaderUserParamsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.parseFuncHeaderAssign()
	if err != nil {
		return nil, err
	}
	node := &V10FuncHeaderUserParamsNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Items:       []*V10FuncHeaderAssignNode{first},
	}
	for p.tokens[p.pos].Type == V10_NL {
		saved := p.savePos()
		p.v10skipNLs()
		next, err := p.parseFuncHeaderAssign()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, next)
	}
	return node, nil
}

// parseFuncEnclosureParams parses:
//
//	func_enclosure_params = ( buildin | user ) { EOL ( buildin | user ) }
func (p *V10Parser) parseFuncEnclosureParams() (*V10FuncEnclosureParamsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V10FuncEnclosureParamsNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	parseOne := func() (V10Node, error) {
		saved := p.savePos()
		if bd, err := p.parseFuncHeaderBuildin(); err == nil {
			return bd, nil
		}
		p.restorePos(saved)
		return p.parseFuncHeaderUserParams()
	}

	first, err := parseOne()
	if err != nil {
		return nil, err
	}
	node.Items = append(node.Items, first)

	for p.tokens[p.pos].Type == V10_NL {
		saved := p.savePos()
		p.v10skipNLs()
		next, err := parseOne()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, next)
	}
	return node, nil
}

// ---------- ident_static_fn ----------

// ParseIdentStaticFn parses:  ident_static_fn = ident_ref "." "@" static_prop
// The static property must start with '@' or be one of the keyword forms.
func (p *V10Parser) ParseIdentStaticFn() (*V10IdentStaticFnNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_DOT); err != nil {
		return nil, err
	}
	tok := p.cur()
	var static string
	switch tok.Type {
	case V10_AT_IDENT:
		static = tok.Value
		p.advance()
	case V10_AT:
		// bare "@" — grammar shows "@(...)" but AT is standalone, then type follows
		p.advance()
		tok2 := p.cur()
		if tok2.Type != V10_IDENT {
			return nil, p.errAt(fmt.Sprintf("expected identifier after '@', got %s", tok2.Type))
		}
		static = "@" + tok2.Value
		p.advance()
	case V10_IDENT:
		// keyword-like static props: ok, error, deps, next, value
		switch tok.Value {
		case "ok", "error", "deps", "next", "value":
			static = tok.Value
			p.advance()
		default:
			return nil, p.errAt(fmt.Sprintf("expected static identifier (ok, error, deps, next, value, or @<prop>), got %q", tok.Value))
		}
	default:
		return nil, p.errAt(fmt.Sprintf("expected static property after '.', got %s %q", tok.Type, tok.Value))
	}
	return &V10IdentStaticFnNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Ref: ref, Static: static}, nil
}

// ---------- func_args_decl ----------

// ParseArgsDecl parses (V11):
//
//	args_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] inspect_type { "," args_decl }
//
// The recursive comma-separated list is flattened into the caller's slice.
func (p *V10Parser) ParseArgsDecl() ([]*V10ArgsDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col

	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}

	// Optional operator: ":" or ":~"
	var oper *V10AssignOperNode
	if p.cur().Type == V10_COLON || p.cur().Type == V10_READONLY {
		oper, err = p.ParseAssignOper()
		if err != nil {
			return nil, err
		}
	}

	// inspect_type: ident_ref "." "@<prop>"
	inspect, err := p.parseInspectAccess(nil)
	if err != nil {
		return nil, err
	}

	entry := &V10ArgsDeclNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		Type:        inspect,
	}
	result := []*V10ArgsDeclNode{entry}

	// Recursively parse comma-separated additional entries.
	for p.cur().Type == V10_COMMA {
		saved := p.savePos()
		p.advance()
		more, err := p.ParseArgsDecl()
		if err != nil {
			p.restorePos(saved)
			break
		}
		result = append(result, more...)
	}
	return result, nil
}

// ParseFuncArgs parses (V11):  func_args = "->" args_decl
func (p *V10Parser) ParseFuncArgs() (*V10FuncArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_ARROW); err != nil {
		return nil, err
	}
	node := &V10FuncArgsNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}
	if !v10isAssignLHSStart(p.cur().Type) {
		return node, nil // args_decl is optional per func_unit_header usage
	}
	entries, err := p.ParseArgsDecl()
	if err != nil {
		return nil, err
	}
	node.Entries = entries
	return node, nil
}

// v10isAssignLHSStart returns true when tok can begin an assign_lhs (just IDENT).
func v10isAssignLHSStart(t V10TokenType) bool {
	return t == V10_IDENT
}

// ParseFuncStreamArgs parses (V11):  func_stream_args = ">>" args_decl
func (p *V10Parser) ParseFuncStreamArgs() (*V10FuncStreamArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_STREAM); err != nil {
		return nil, err
	}
	entries, err := p.ParseArgsDecl()
	if err != nil {
		return nil, err
	}
	return &V10FuncStreamArgsNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Entries:     entries,
	}, nil
}

// ParseFuncDeps parses:  func_deps = "=>" UNIQUE< ident_static_store_name { "," ident_static_store_name } >
//
// ident_static_store_name = "@storeName" which is scanned as V10_AT_IDENT.
func (p *V10Parser) ParseFuncDeps() (*V10FuncDepsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_STORE); err != nil {
		return nil, err
	}
	tok := p.cur()
	if tok.Type != V10_AT_IDENT || tok.Value != "@storeName" {
		return nil, p.errAt(fmt.Sprintf("expected @storeName in func_deps, got %s %q", tok.Type, tok.Value))
	}
	names := []string{tok.Value}
	p.advance()
	for p.cur().Type == V10_COMMA {
		saved := p.savePos()
		p.advance()
		t2 := p.cur()
		if t2.Type != V10_AT_IDENT || t2.Value != "@storeName" {
			p.restorePos(saved)
			break
		}
		names = append(names, t2.Value)
		p.advance()
	}
	return &V10FuncDepsNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, StoreNames: names}, nil
}

// ParseFuncArgsDecl parses:  func_args_decl = [ func_args ] [ func_stream_args ] [ func_deps ]
func (p *V10Parser) ParseFuncArgsDecl() (*V10FuncArgsDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V10FuncArgsDeclNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	if p.cur().Type == V10_ARROW {
		args, err := p.ParseFuncArgs()
		if err != nil {
			return nil, err
		}
		node.Args = args
	}
	if p.cur().Type == V10_STREAM {
		sa, err := p.ParseFuncStreamArgs()
		if err != nil {
			return nil, err
		}
		node.StreamArgs = sa
	}
	if p.cur().Type == V10_STORE {
		saved := p.savePos()
		deps, err := p.ParseFuncDeps()
		if err != nil {
			p.restorePos(saved)
		} else {
			node.Deps = deps
		}
	}
	return node, nil
}

// ---------- func_range_args ----------

// ParseFuncRangeArgs parses:  func_range_args = func_fixed_num_range | func_fixed_str_range | func_fixed_list_range
func (p *V10Parser) ParseFuncRangeArgs() (*V10FuncRangeArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V10Node) *V10FuncRangeArgsNode {
		return &V10FuncRangeArgsNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: v}
	}

	// String range: string | TYPE_OF string<ident_ref>
	if v10isStringTok(p.cur().Type) {
		s, err := p.ParseString()
		if err != nil {
			return nil, err
		}
		return wrap(s), nil
	}

	// List range: array_final | object_final
	switch p.cur().Type {
	case V10_LBRACKET, V10_EMPTY_ARR, V10_UNIFORM:
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
	}

	// Numeric range or TYPE_OF variants.
	parseFuncNum := func() (V10Node, error) {
		if p.cur().Type == V10_TYPE_OF {
			return p.parseTypeOfRef()
		}
		return p.ParseNumericConst()
	}

	lo, err := parseFuncNum()
	if err != nil {
		// TYPE_OF string form
		if p.cur().Type == V10_TYPE_OF {
			return wrap(lo), nil
		}
		return nil, err
	}
	if p.cur().Type != V10_DOTDOT {
		// Single string TYPE_OF or plain string already handled; this must be an error.
		return nil, p.errAt(fmt.Sprintf("expected '..' in func numeric range, got %s", p.cur().Type))
	}
	p.advance() // consume ".."
	hi, err := parseFuncNum()
	if err != nil {
		return nil, err
	}
	numRange := &V10FuncFixedNumRangeNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Lo:          lo,
		Hi:          hi,
	}
	return wrap(numRange), nil
}

// ---------- func_call / func_call_chain ----------

// ParseFuncCall parses:  func_call = TYPE_OF func_unit<ident_ref> [ func_call_args ]
func (p *V10Parser) ParseFuncCall() (*V10FuncCallNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V10FuncCallNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Ref: ref}

	// Optional call args: assign_func_rhs { "," assign_func_rhs }
	if v10isFuncCallArgStart(p.cur().Type) {
		first, err := p.ParseAssignFuncRHS()
		if err == nil {
			node.Args = append(node.Args, first)
			for p.cur().Type == V10_COMMA {
				saved := p.savePos()
				p.advance()
				arg, err := p.ParseAssignFuncRHS()
				if err != nil {
					p.restorePos(saved)
					break
				}
				node.Args = append(node.Args, arg)
			}
		}
	}
	return node, nil
}

// v10isFuncCallArgStart returns true when the current token can start an assign_func_rhs.
func v10isFuncCallArgStart(t V10TokenType) bool {
	switch t {
	case V10_IDENT, V10_INTEGER, V10_DECIMAL, V10_STRING,
		V10_EMPTY_STR_D, V10_EMPTY_STR_S, V10_EMPTY_STR_T,
		V10_REGEXP, V10_REGEXP_DECL,
		V10_TRUE, V10_FALSE, V10_NULL, V10_NAN, V10_INFINITY,
		V10_PLUS, V10_MINUS,
		V10_LBRACKET, V10_EMPTY_ARR, V10_UNIFORM:
		return true
	}
	return false
}

// ParseFuncCallChain parses:  func_call_chain = func_call [ step { step } ]
func (p *V10Parser) ParseFuncCallChain() (*V10FuncCallChainNode, error) {
	line, col := p.cur().Line, p.cur().Col
	head, err := p.ParseFuncCall()
	if err != nil {
		return nil, err
	}
	node := &V10FuncCallChainNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Head: head}

	for {
		var opStr string
		switch p.cur().Type {
		case V10_ARROW:
			opStr = "->"
		case V10_STREAM:
			opStr = ">>"
		default:
			opStr = v10isBinaryLogicOper(p.cur().Type)
		}
		if opStr == "" {
			break
		}
		saved := p.savePos()
		p.advance() // consume op
		if p.cur().Type != V10_TYPE_OF {
			p.restorePos(saved)
			break
		}
		ref, err := p.parseTypeOfRef()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Steps = append(node.Steps, V10FuncCallChainStepNode{
			v10BaseNode: v10BaseNode{Line: p.cur().Line, Col: p.cur().Col},
			Op:          opStr,
			Ref:         ref,
		})
	}
	return node, nil
}

// ---------- func_stream_loop / func_call_final ----------

// ParseFuncStreamLoop parses:
//
//	func_stream_loop = ( func_range_args | ident_ref | boolean_true ) ">>" ( func_unit | func_call )
func (p *V10Parser) ParseFuncStreamLoop() (*V10FuncStreamLoopNode, error) {
	line, col := p.cur().Line, p.cur().Col

	var source V10Node
	// boolean_true: ident "true"
	if p.cur().Type == V10_TRUE {
		b, err := p.ParseBoolean()
		if err != nil {
			return nil, err
		}
		source = b
	} else if p.cur().Type == V10_IDENT {
		saved := p.savePos()
		ref, err := p.ParseIdentRef()
		if err == nil {
			source = ref
		} else {
			p.restorePos(saved)
			ra, err := p.ParseFuncRangeArgs()
			if err != nil {
				return nil, err
			}
			source = ra
		}
	} else {
		ra, err := p.ParseFuncRangeArgs()
		if err != nil {
			return nil, err
		}
		source = ra
	}

	if _, err := p.expect(V10_STREAM); err != nil {
		return nil, err
	}

	var body V10Node
	var err error
	if p.cur().Type == V10_LBRACE {
		body, err = p.ParseFuncUnit()
	} else {
		body, err = p.ParseFuncCall()
	}
	if err != nil {
		return nil, err
	}
	return &V10FuncStreamLoopNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Source: source, Body: body}, nil
}

// ParseFuncCallFinal parses:  func_call_final = func_call_chain | func_stream_loop
func (p *V10Parser) ParseFuncCallFinal() (*V10FuncCallFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	// Both alternatives can start with TYPE_OF or with an ident/literal.
	// func_call starts with TYPE_OF; func_stream_loop can start with anything.
	// If current token is TYPE_OF, try func_call_chain first.
	if p.cur().Type == V10_TYPE_OF {
		saved := p.savePos()
		if cc, err := p.ParseFuncCallChain(); err == nil {
			return &V10FuncCallFinalNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: cc}, nil
		}
		p.restorePos(saved)
	}
	sl, err := p.ParseFuncStreamLoop()
	if err != nil {
		return nil, err
	}
	return &V10FuncCallFinalNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: sl}, nil
}

// ---------- func_inject ----------

// ParseFuncInject parses:
//
//	func_inject = "(" ( inspect_type [ "[]" ] | assign_lhs ":" ident_ref ) { "," assign_lhs ":" ident_ref } ")"
// ParseFuncInject parses (V11):
//
//	func_inject = "(" [ ( assign_lhs inspect_type [ empty_array_decl ] )
//	                   | ( [ assign_lhs ( ":" | ":~" ) ] ident_ref ) ]
//	               { "," assign_lhs ( ":" | ":~" ) ident_ref } ")"
func (p *V10Parser) ParseFuncInject() (*V10FuncInjectNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_LPAREN); err != nil {
		return nil, err
	}

	node := &V10FuncInjectNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}

	// Head is entirely optional — if the next token is ")" skip it.
	if p.cur().Type != V10_RPAREN {
		node.Head = p.parseFuncInjectHead(line, col)
	}

	// Additional "," assign_lhs ( ":" | ":~" ) ident_ref entries.
	for p.cur().Type == V10_COMMA {
		saved := p.savePos()
		p.advance()
		lhs, err := p.ParseAssignLHS()
		if err != nil {
			p.restorePos(saved)
			break
		}
		if p.cur().Type != V10_COLON && p.cur().Type != V10_READONLY {
			p.restorePos(saved)
			break
		}
		oper, err := p.ParseAssignOper()
		if err != nil {
			p.restorePos(saved)
			break
		}
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Binds = append(node.Binds, V10FuncInjectBind{LHS: lhs, Oper: oper, Ref: ref})
	}

	if _, err := p.expect(V10_RPAREN); err != nil {
		return nil, err
	}
	return node, nil
}

// parseFuncInjectHead tries to parse the optional head element of func_inject.
// Returns nil if nothing matches (empty head).
func (p *V10Parser) parseFuncInjectHead(line, col int) V10Node {
	// Try: assign_lhs inspect_type [ empty_array_decl ]
	// Both forms start with IDENT. Discriminate by what follows the lhs.
	saved := p.savePos()
	lhs, lhsErr := p.ParseAssignLHS()
	if lhsErr == nil {
		// Case 1: assign_lhs then inspect_type (ident_ref "." "@<prop>")
		if p.cur().Type == V10_IDENT {
			innerSaved := p.savePos()
			access, aErr := p.parseInspectAccess(nil)
			if aErr == nil {
				head := &V10FuncInjectHeadInspectNode{
					v10BaseNode: v10BaseNode{Line: line, Col: col},
					LHS:         lhs,
					Inspect:     access,
				}
				if p.cur().Type == V10_EMPTY_ARR {
					head.HasArray = true
					p.advance()
				}
				return head
			}
			p.restorePos(innerSaved)
		}
		// Case 2: assign_lhs ( ":" | ":~" ) ident_ref
		if p.cur().Type == V10_COLON || p.cur().Type == V10_READONLY {
			oper, operErr := p.ParseAssignOper()
			if operErr == nil {
				ref, refErr := p.ParseIdentRef()
				if refErr == nil {
					return &V10FuncInjectBindNode{
						v10BaseNode: v10BaseNode{Line: line, Col: col},
						Bind:        V10FuncInjectBind{LHS: lhs, Oper: oper, Ref: ref},
					}
				}
			}
		}
	}
	p.restorePos(saved)
	// Case 3: bare ident_ref (no lhs, no operator)
	ref, refErr := p.ParseIdentRef()
	if refErr == nil {
		return &V10FuncInjectBindNode{
			v10BaseNode: v10BaseNode{Line: line, Col: col},
			Bind:        V10FuncInjectBind{Ref: ref},
		}
	}
	p.restorePos(saved)
	return nil
}

// ---------- func_stmt ----------

// ParseFuncStmt parses:
//
//	func_stmt = regexp | ident_ref | object_final | array_final | func_call_chain | func_unit | calc_unit | self_ref
//	            | return_func_unit  (EXTEND<func_stmt>)
func (p *V10Parser) ParseFuncStmt() (*V10FuncStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V10Node) *V10FuncStmtNode {
		return &V10FuncStmtNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	// return_func_unit (EXTEND): "<-" func_unit
	case V10_RETURN_STMT:
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V10_LBRACE {
			unit, err := p.ParseFuncUnit()
			if err == nil {
				return wrap(&V10ReturnFuncUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Unit: unit}), nil
			}
		}
		p.restorePos(saved)

	// self_ref: "$"
	case V10_DOLLAR:
		p.advance()
		return wrap(&V10SelfRefNode{v10BaseNode: v10BaseNode{Line: line, Col: col}}), nil

	// func_unit: "{"
	case V10_LBRACE:
		unit, err := p.ParseFuncUnit()
		if err != nil {
			return nil, err
		}
		return wrap(unit), nil

	// object_final / array_final
	case V10_LBRACKET, V10_EMPTY_ARR, V10_UNIFORM:
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

	// func_call_chain starts with TYPE_OF
	case V10_TYPE_OF:
		saved := p.savePos()
		if cc, err := p.ParseFuncCallChain(); err == nil {
			return wrap(cc), nil
		}
		p.restorePos(saved)
	}

	// regexp / constant
	if p.cur().Type == V10_REGEXP || p.cur().Type == V10_REGEXP_DECL {
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil
	}

	// calc_unit (covers ident_ref and expressions)
	cu, err := p.ParseCalcUnit()
	if err != nil {
		return nil, err
	}
	return wrap(cu), nil
}

// ---------- func_assign ----------

// ParseFuncAssign parses:  func_assign = [ func_inject ] assign_lhs assign_oper func_stmt
func (p *V10Parser) ParseFuncAssign() (*V10FuncAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var inj *V10FuncInjectNode
	if p.cur().Type == V10_LPAREN {
		saved := p.savePos()
		fi, err := p.ParseFuncInject()
		if err == nil {
			inj = fi
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
	stmt, err := p.ParseFuncStmt()
	if err != nil {
		return nil, err
	}
	return &V10FuncAssignNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Inject:      inj,
		LHS:         lhs,
		Oper:        oper,
		Stmt:        stmt,
	}, nil
}

// ---------- func body statements ----------

// ParseFuncReturnStmt parses:  func_return_stmt = "<-" func_stmt
func (p *V10Parser) ParseFuncReturnStmt() (*V10FuncReturnStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_RETURN_STMT); err != nil {
		return nil, err
	}
	stmt, err := p.ParseFuncStmt()
	if err != nil {
		return nil, err
	}
	return &V10FuncReturnStmtNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Stmt: stmt}, nil
}

// ParseFuncStoreStmt parses:
//
//	func_store_stmt = "=>" ( object_final | TYPE_OF object_final<ident_ref> ) { "," ... }
func (p *V10Parser) ParseFuncStoreStmt() (*V10FuncStoreStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_STORE); err != nil {
		return nil, err
	}
	parseItem := func() (V10Node, error) {
		if p.cur().Type == V10_TYPE_OF {
			return p.parseTypeOfRef()
		}
		return p.ParseObjectFinal()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	node := &V10FuncStoreStmtNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Items: []V10Node{first}}
	for p.cur().Type == V10_COMMA {
		saved := p.savePos()
		p.advance()
		item, err := parseItem()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, item)
	}
	return node, nil
}

// ParseFuncBodyStmt parses:
//
//	func_body_stmt = func_assign | func_return_stmt | func_store_stmt | func_stream_loop | func_call_final
func (p *V10Parser) ParseFuncBodyStmt() (*V10FuncBodyStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V10Node) *V10FuncBodyStmtNode {
		return &V10FuncBodyStmtNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V10_RETURN_STMT:
		rs, err := p.ParseFuncReturnStmt()
		if err != nil {
			return nil, err
		}
		return wrap(rs), nil

	case V10_STORE:
		ss, err := p.ParseFuncStoreStmt()
		if err != nil {
			return nil, err
		}
		return wrap(ss), nil
	}

	// func_call_final or func_stream_loop (both can start with TYPE_OF or ident)
	// Try func_call_final first.
	if p.cur().Type == V10_TYPE_OF {
		saved := p.savePos()
		if cf, err := p.ParseFuncCallFinal(); err == nil {
			return wrap(cf), nil
		}
		p.restorePos(saved)
	}

	// func_assign — covers inject, assign_lhs, oper, stmt.
	saved := p.savePos()
	if fa, err := p.ParseFuncAssign(); err == nil {
		return wrap(fa), nil
	}
	p.restorePos(saved)

	// func_stream_loop
	sl, err := p.ParseFuncStreamLoop()
	if err != nil {
		return nil, err
	}
	return wrap(sl), nil
}

// ---------- func_unit ----------

// ParseFuncUnit parses:
//
//	func_unit = "{" func_unit_header func_body_stmt { NL | func_body_stmt } "}"
func (p *V10Parser) ParseFuncUnit() (*V10FuncUnitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_LBRACE); err != nil {
		return nil, err
	}

	// func_unit_header: optional enclosure + args_decl
	hdr := &V10FuncUnitHeaderNode{v10BaseNode: v10BaseNode{Line: p.cur().Line, Col: p.cur().Col}}
	// Try enclosure params followed by args_decl.
	if !v10isFuncBodyStart(p.cur()) {
		saved := p.savePos()
		enc, err := p.parseFuncEnclosureParams()
		if err == nil {
			hdr.Enclosure = enc
		} else {
			p.restorePos(saved)
		}
		argsDecl, _ := p.ParseFuncArgsDecl()
		hdr.ArgsDecl = argsDecl
	}
	p.v10skipNLs()

	// At least one func_body_stmt required.
	first, err := p.ParseFuncBodyStmt()
	if err != nil {
		return nil, err
	}
	body := []*V10FuncBodyStmtNode{first}

	for {
		p.v10skipNLs()
		if p.cur().Type == V10_RBRACE || p.cur().Type == V10_EOF {
			break
		}
		stmt, err := p.ParseFuncBodyStmt()
		if err != nil {
			break
		}
		body = append(body, stmt)
	}

	if _, err := p.expect(V10_RBRACE); err != nil {
		return nil, err
	}
	return &V10FuncUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Header: hdr, Body: body}, nil
}

// v10isFuncBodyStart returns true for tokens that can only begin a func_body_stmt,
// not a header (used to decide whether to try parsing headers at all).
func v10isFuncBodyStart(tok V10Token) bool {
	switch tok.Type {
	case V10_RETURN_STMT, V10_STORE, V10_DOLLAR:
		return true
	}
	return false
}

// ---------- function-related updates ----------

// ParseReturnFuncUnit parses:  return_func_unit = "<-" func_unit
func (p *V10Parser) ParseReturnFuncUnit() (*V10ReturnFuncUnitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V10_RETURN_STMT); err != nil {
		return nil, err
	}
	unit, err := p.ParseFuncUnit()
	if err != nil {
		return nil, err
	}
	return &V10ReturnFuncUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Unit: unit}, nil
}

// ParseUpdateFuncUnit parses:  update_func_unit = TYPE_OF func_unit<ident_ref> equal_assign return_func_unit
func (p *V10Parser) ParseUpdateFuncUnit() (*V10UpdateFuncUnitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}
	rfu, err := p.ParseReturnFuncUnit()
	if err != nil {
		return nil, err
	}
	return &V10UpdateFuncUnitNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Ref: ref, Assign: oper, NewUnit: rfu}, nil
}

// ---------- array_idx_recursive / ident_ref extension ----------

// ParseArrayIdxRecursive parses:  array_idx_recursive = "[" numeric_rhs "]" { "[" numeric_rhs "]" }
func (p *V10Parser) ParseArrayIdxRecursive() (*V10ArrayIdxRecursiveNode, error) {
	line, col := p.cur().Line, p.cur().Col
	parseNumRHS := func() (V10Node, error) {
		// numeric_rhs = numeric_const | numeric_stmt
		if p.cur().Type == V10_TYPE_OF {
			return p.parseTypeOfRef() // numeric_stmt is TYPE_OF numeric_const<...>
		}
		return p.ParseNumericConst()
	}
	if _, err := p.expect(V10_LBRACKET); err != nil {
		return nil, err
	}
	first, err := parseNumRHS()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V10_RBRACKET); err != nil {
		return nil, err
	}
	node := &V10ArrayIdxRecursiveNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Indices: []V10Node{first}}
	for p.cur().Type == V10_LBRACKET {
		saved := p.savePos()
		p.advance()
		idx, err := parseNumRHS()
		if err != nil {
			p.restorePos(saved)
			break
		}
		if _, err2 := p.expect(V10_RBRACKET); err2 != nil {
			p.restorePos(saved)
			break
		}
		node.Indices = append(node.Indices, idx)
	}
	return node, nil
}

// ---------- update_number / update_string ----------

// ParseUpdateNumber parses:
//
//	update_number = TYPE_OF numeric_const<ident_ref> assign_oper numeric_rhs
func (p *V10Parser) ParseUpdateNumber() (*V10UpdateNumberNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}
	var rhs V10Node
	if p.cur().Type == V10_TYPE_OF {
		rhs, err = p.parseTypeOfRef()
	} else {
		rhs, err = p.ParseNumericConst()
	}
	if err != nil {
		return nil, err
	}
	return &V10UpdateNumberNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Target: ref, Oper: oper, RHS: rhs}, nil
}

// ParseStringUpdateOper parses:  string_update_oper = "+:" | "+~" | equal_assign
func (p *V10Parser) ParseStringUpdateOper() (*V10StringUpdateOperNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var kind V10StringUpdateOperKind
	var tv string
	switch tok.Type {
	case V10_IADD_IMM:
		kind, tv = StringAppendImmutable, "+:"
	case V10_IADD_MUT:
		kind, tv = StringAppendMutable, "+~"
	case V10_EQ, V10_COLON, V10_READONLY:
		kind, tv = StringEqualAssign, tok.Value
	default:
		return nil, p.errAt(fmt.Sprintf("expected string update operator (+:, +~, =, :, :~), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V10StringUpdateOperNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Kind: kind, Token: tv}, nil
}

// ParseUpdateString parses:
//
//	update_string = TYPE_OF string<ident_ref> string_update_oper string_rhs
func (p *V10Parser) ParseUpdateString() (*V10UpdateStringNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseStringUpdateOper()
	if err != nil {
		return nil, err
	}
	var rhs V10Node
	if p.cur().Type == V10_TYPE_OF {
		rhs, err = p.parseTypeOfRef()
	} else {
		rhs, err = p.ParseString()
	}
	if err != nil {
		return nil, err
	}
	return &V10UpdateStringNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Target: ref, Oper: oper, RHS: rhs}, nil
}

// ---------- ident_ref_update ----------

// ParseIdentRefUpdate parses:  ident_ref_update = ident_ref assign_oper assign_rhs
func (p *V10Parser) ParseIdentRefUpdate() (*V10IdentRefUpdateNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}
	rhs, err := p.ParseAssignRHS()
	if err != nil {
		return nil, err
	}
	return &V10IdentRefUpdateNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Ref: ref, Oper: oper, RHS: rhs}, nil
}

// The _ below suppresses an unused import warning for "fmt" in case some
// error-path functions are elided by the compiler in certain build modes.
var _ = fmt.Sprintf
