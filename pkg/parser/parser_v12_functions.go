// parser_v12_functions.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V12 grammar rule set defined in spec/06_functions.sqg.
//
// Covered rules:
//
//	type_prefix, inspect_type, inspect_type_name, type_declare (V12)
//	assign_func_rhs, func_header_assign, func_header_user_params
//	ident_static_store_name (V12: ident_name | inspect_type_name)
//	func_args, func_stream_args, func_deps, func_args_decl
//	func_fixed_num_range, func_fixed_str_range, func_fixed_list_range, func_range_args
//	func_call_args, func_call, func_call_chain
//	func_stream_loop, func_call_final
//	self_ref
//	func_inject, func_stmt, func_assign
//	func_return_stmt, func_store_stmt, func_body_stmt
//	scope_begin, scope_end, group_begin, group_end (V12)
//	func_unit_header, func_unit (V12: {..} and (..) forms)
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

// ---------- inspect_type / inspect_type_name (V12) ----------

// V12InspectTypeNode  inspect_type = type_prefix !WS! ident_ref
// Source form: @MyType  (scanned as a single V12_AT_IDENT token)
type V12InspectTypeNode struct {
	V12BaseNode
	Name string // the full AT_IDENT value, e.g. "@MyType"
}

// V12InspectTypeNameNode  inspect_type_name = type_prefix !WS! TYPE_OF string<ident_ref "." "name">
// Source form: @myRef.name
type V12InspectTypeNameNode struct {
	V12BaseNode
	Ref  *V12IdentRefNode // the ident_ref part
	Prop string           // always "name" in V12
}

// V12TypeDeclareNode  type_declare = ident_name inspect_type
// Source form: myName @MyType
type V12TypeDeclareNode struct {
	V12BaseNode
	Name    string // ident_name
	Inspect *V12InspectTypeNode
}

// V12AssignFuncRHSNode  assign_func_rhs = constant | func_string_decl | (regexp | func_regexp_decl)
//
//	| ident_ref | calc_unit | object_final | array_final
type V12AssignFuncRHSNode struct {
	V12BaseNode
	Value V12Node
}

// V12FuncHeaderAssignNode  func_header_assign = assign_lhs assign_oper assign_func_rhs  (V12: no buildin form)
type V12FuncHeaderAssignNode struct {
	V12BaseNode
	LHS  *V12AssignLHSNode
	Oper *V12AssignOperNode
	RHS  *V12AssignFuncRHSNode
}

// V12FuncHeaderUserParamsNode  func_header_user_params = func_header_assign { EOL func_header_assign }
type V12FuncHeaderUserParamsNode struct {
	V12BaseNode
	Items []*V12FuncHeaderAssignNode
}

// ---------- ident_static_store_name (V12) ----------
// V12 removed ident_static_fn. ident_static_store_name = ident_name | inspect_type_name.
// Represented as plain string (ident_name) or *V12InspectTypeNameNode in func_deps.

// ---------- func_args_decl ----------

// V12ArgsDeclNode  args_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] inspect_type
// inspect_type in V12 = type_prefix !WS! ident_ref = "@ident" (AT_IDENT)
type V12ArgsDeclNode struct {
	V12BaseNode
	LHS  *V12AssignLHSNode
	Oper *V12AssignOperNode  // nil when operator is absent
	Type *V12InspectTypeNode // the inspect_type "@TypeName"
}

// V12FuncArgsNode  func_args = "->" args_decl { "," args_decl }
type V12FuncArgsNode struct {
	V12BaseNode
	Entries []*V12ArgsDeclNode
}

// V12FuncStreamArgsNode  func_stream_args = ">>" args_decl { "," args_decl }
type V12FuncStreamArgsNode struct {
	V12BaseNode
	Entries []*V12ArgsDeclNode
}

// V12FuncDepsNode  func_deps = "=>" UNIQUE< ident_static_store_name { "," ident_static_store_name } >
type V12FuncDepsNode struct {
	V12BaseNode
	StoreNames []string // "@storeName" strings
}

// V12FuncArgsDeclNode  func_args_decl = [ func_args ] [ func_stream_args ] [ func_deps ]
type V12FuncArgsDeclNode struct {
	V12BaseNode
	Args       *V12FuncArgsNode
	StreamArgs *V12FuncStreamArgsNode
	Deps       *V12FuncDepsNode
}

// ---------- func_range_args ----------

// V12FuncFixedNumRangeNode  numeric_const ".." numeric_const  (or TYPE_OF variants)
type V12FuncFixedNumRangeNode struct {
	V12BaseNode
	Lo V12Node // *V12NumericConstNode | *V12TypeOfRefNode
	Hi V12Node // *V12NumericConstNode | *V12TypeOfRefNode
}

// V12FuncRangeArgsNode  func_range_args = num_range | str_range | list_range
type V12FuncRangeArgsNode struct {
	V12BaseNode
	Value V12Node // *V12FuncFixedNumRangeNode | *V12StringNode | *V12TypeOfRefNode | *V12ArrayFinalNode | *V12ObjectFinalNode
}

// ---------- func_call / func_call_chain / func_stream_loop ----------

// V12FuncCallNode  func_call = TYPE_OF func_unit<ident_ref> [ func_call_args ]
type V12FuncCallNode struct {
	V12BaseNode
	Ref  *V12TypeOfRefNode
	Args []V12Node // *V12AssignFuncRHSNode items
}

// V12FuncCallChainStepNode  one ( "->" | ">>" | logic_oper ) TYPE_OF func_unit<ident_ref> step
type V12FuncCallChainStepNode struct {
	V12BaseNode
	Op  string // "->", ">>", or logic operator string
	Ref *V12TypeOfRefNode
}

// V12FuncCallChainNode  func_call_chain = func_call [ step { step } ]
type V12FuncCallChainNode struct {
	V12BaseNode
	Head  *V12FuncCallNode
	Steps []V12FuncCallChainStepNode
}

// V12FuncStreamLoopNode
//
//	func_stream_loop = ( func_range_args | ident_ref | boolean_true ) ">>" ( func_unit | func_call )
type V12FuncStreamLoopNode struct {
	V12BaseNode
	Source V12Node // *V12FuncRangeArgsNode | *V12IdentRefNode | *V12BooleanNode ("true")
	Body   V12Node // *V12FuncUnitNode | *V12FuncCallNode
}

// V12FuncCallFinalNode  func_call_final = func_call_chain | func_stream_loop
type V12FuncCallFinalNode struct {
	V12BaseNode
	Value V12Node // *V12FuncCallChainNode | *V12FuncStreamLoopNode
}

// ---------- func_unit body ----------

// V12SelfRefNode  self_ref = "$"
type V12SelfRefNode struct{ V12BaseNode }

// V12FuncInjectNode  (V11)
//
//	func_inject = "(" [ ( assign_lhs inspect_type [ empty_array_decl ] )
//	                   | ( [ assign_lhs ( ":" | ":~" ) ] ident_ref ) ]
//	               { "," assign_lhs ( ":" | ":~" ) ident_ref } ")"
type V12FuncInjectNode struct {
	V12BaseNode
	Head  V12Node             // *V12FuncInjectHeadInspectNode | *V12FuncInjectBindNode | nil (empty)
	Binds []V12FuncInjectBind // additional  assign_lhs ( ":" | ":~" ) ident_ref items
}

// V12FuncInjectBind  assign_lhs ( ":" | ":~" ) ident_ref  (operator is optional for the head)
type V12FuncInjectBind struct {
	LHS  *V12AssignLHSNode  // nil when no lhs in head
	Oper *V12AssignOperNode // nil when operator absent (head only)
	Ref  *V12IdentRefNode
}

// V12FuncInjectBindNode  wraps V12FuncInjectBind as a V12Node so it can be stored in Head.
type V12FuncInjectBindNode struct {
	V12BaseNode
	Bind V12FuncInjectBind
}

// V12FuncInjectHeadInspectNode  assign_lhs inspect_type [ empty_array_decl ]
// inspect_type in V12 = "@TypeName" (AT_IDENT)
type V12FuncInjectHeadInspectNode struct {
	V12BaseNode
	LHS      *V12AssignLHSNode
	Inspect  *V12InspectTypeNode // "@TypeName"
	HasArray bool                // true when followed by "[]"
}

// V12FuncStmtNode  func_stmt = regexp | ident_ref | object_final | array_final
//
//	| func_call_chain | func_unit | calc_unit | self_ref
type V12FuncStmtNode struct {
	V12BaseNode
	// Extended by EXTEND<func_stmt> to also include return_func_unit.
	Value V12Node
}

// V12FuncAssignNode  func_assign = [ func_inject ] assign_lhs assign_oper func_stmt
type V12FuncAssignNode struct {
	V12BaseNode
	Inject *V12FuncInjectNode // nil when no injection
	LHS    *V12AssignLHSNode
	Oper   *V12AssignOperNode
	Stmt   *V12FuncStmtNode
}

// V12FuncReturnStmtNode  func_return_stmt = "<-" func_stmt
type V12FuncReturnStmtNode struct {
	V12BaseNode
	Stmt *V12FuncStmtNode
}

// V12CondReturnStmtNode  cond_return_stmt = "(" logic_expr ")" logic_oper func_return_stmt
// Squeeze conditional return:  ( condition ) & <- result
type V12CondReturnStmtNode struct {
	V12BaseNode
	Cond   V12Node // logic_expr inside ( ... )
	Oper   string  // logic_oper token value, e.g. "&"
	Return *V12FuncReturnStmtNode
}

// V12FuncStoreStmtNode  func_store_stmt = "=>" ( object_final | TYPE_OF object_final<ident_ref> ) { "," ... }
type V12FuncStoreStmtNode struct {
	V12BaseNode
	Items []V12Node // *V12ObjectFinalNode | *V12TypeOfRefNode
}

// V12FuncBodyStmtNode  func_body_stmt = func_assign | func_return_stmt | func_store_stmt
//
//	| func_stream_loop | func_call_final | cond_return_stmt
type V12FuncBodyStmtNode struct {
	V12BaseNode
	Value V12Node
}

// ---------- func_unit ----------

// V12FuncUnitHeaderNode  func_unit_header = [ (func_header_user_params func_args_decl) | func_args_decl ]  (V12: no enclosure)
type V12FuncUnitHeaderNode struct {
	V12BaseNode
	UserParams *V12FuncHeaderUserParamsNode // optional func_header_user_params (assign stmts before args)
	ArgsDecl   *V12FuncArgsDeclNode
}

// V12FuncUnitNode  func_unit = ( "{" func_unit_header func_body_stmt { NL | func_body_stmt } "}" )
//
//	                           | ( "(" func_unit_header func_body_stmt { NL | func_body_stmt } ")" )  (V12 group form)
type V12FuncUnitNode struct {
	V12BaseNode
	Header        *V12FuncUnitHeaderNode
	Body          []*V12FuncBodyStmtNode
	UseGroupDelim bool // true when delimited with () instead of {}
}

// ---------- function-related updates ----------

// V12ReturnFuncUnitNode  return_func_unit = "<-" func_unit
type V12ReturnFuncUnitNode struct {
	V12BaseNode
	Unit *V12FuncUnitNode
}

// V12UpdateFuncUnitNode  update_func_unit = TYPE_OF func_unit<ident_ref> equal_assign return_func_unit
type V12UpdateFuncUnitNode struct {
	V12BaseNode
	Ref     *V12TypeOfRefNode
	Assign  *V12AssignOperNode
	NewUnit *V12ReturnFuncUnitNode
}

// V12ArrayIdxRecursiveNode  array_idx_recursive = "[" numeric_rhs "]" { "[" numeric_rhs "]" }
type V12ArrayIdxRecursiveNode struct {
	V12BaseNode
	Indices []V12Node // numeric_rhs values
}

// V12NumericStmtNode  numeric_stmt = TYPE_OF numeric_const<ident_ref | func_call_chain | func_unit | calc_unit>
type V12NumericStmtNode struct {
	V12BaseNode
	Ref V12TypeOfRefNode
}

// V12UpdateNumberNode  update_number = TYPE_OF numeric_const<ident_ref> assign_oper numeric_rhs
type V12UpdateNumberNode struct {
	V12BaseNode
	Target *V12TypeOfRefNode
	Oper   *V12AssignOperNode
	RHS    V12Node // *V12NumericConstNode | *V12NumericStmtNode
}

// V12StringStmtNode  string_stmt = TYPE_OF string<ident_ref | func_call_chain | func_unit | calc_unit>
type V12StringStmtNode struct {
	V12BaseNode
	Ref V12TypeOfRefNode
}

// V12StringUpdateOperKind classifies the string update operator.
type V12StringUpdateOperKind int

const (
	V12StringAppendImmutable V12StringUpdateOperKind = iota // +:
	V12StringAppendMutable                                  // +~
	V12StringEqualAssign                                    // = : :~
)

// V12StringUpdateOperNode  string_update_oper = "+:" | "+~" | equal_assign
type V12StringUpdateOperNode struct {
	V12BaseNode
	Kind  V12StringUpdateOperKind
	Token string
}

// V12UpdateStringNode  update_string = TYPE_OF string<ident_ref> string_update_oper string_rhs
type V12UpdateStringNode struct {
	V12BaseNode
	Target *V12TypeOfRefNode
	Oper   *V12StringUpdateOperNode
	RHS    V12Node // *V12StringNode | *V12StringStmtNode
}

// V12IdentRefUpdateNode  ident_ref_update = ident_ref assign_oper assign_rhs
type V12IdentRefUpdateNode struct {
	V12BaseNode
	Ref  *V12IdentRefNode
	Oper *V12AssignOperNode
	RHS  *V12AssignRHSNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (06_functions.sqg)
// =============================================================================

// ---------- helpers ----------

// V12isLogicOper returns the string form of a logic operator token, or "".
// Covers "&&", "||" — used in func_call_chain step discrimination.
func V12isBinaryLogicOper(t V12TokenType) string {
	switch t {
	case V12_AMP_AMP:
		return "&&"
	case V12_PIPE_PIPE:
		return "||"
	}
	return ""
}

// V12skipNLs advances past any NL tokens (for rules that use "EOL" as separator).
func (p *V12Parser) V12skipNLs() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == V12_NL {
		p.pos++
	}
}

// ---------- inspect_type / inspect_type_name (V12) ----------

// parseInspectType parses:  inspect_type = type_prefix !WS! ident_ref | any_type
// Also accepts V1 legacy form "§ident" (SECTION + IDENT) as a parser-rule type reference.
func (p *V12Parser) parseInspectType() (*V12InspectTypeNode, error) {
	tok := p.cur()
	switch tok.Type {
	case V12_ANY_TYPE:
		// any_type = "@?" — wildcard, matches any value of any type
		node := &V12InspectTypeNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}, Name: "@?"}
		p.advance()
		return node, nil
	case V12_AT_IDENT:
		node := &V12InspectTypeNode{V12BaseNode: V12BaseNode{Line: tok.Line, Col: tok.Col}, Name: tok.Value}
		p.advance()
		return node, nil
	case V12_SECTION:
		// V1 legacy: §ident  — direct parser-rule type reference
		line, col := tok.Line, tok.Col
		p.advance()
		if p.cur().Type != V12_IDENT {
			return nil, p.errAt(fmt.Sprintf("expected identifier after §, got %s", p.cur().Type))
		}
		name := "§" + p.cur().Value
		p.advance()
		return &V12InspectTypeNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Name: name}, nil
	default:
		return nil, p.errAt(fmt.Sprintf("expected @TypeName or @? (inspect_type), got %s %q", tok.Type, tok.Value))
	}
}

// parseInspectTypeName parses:  inspect_type_name = "@" ident_ref "." "name"
// Source form: @myRef.name  where @myRef is AT_IDENT and .name follows.
func (p *V12Parser) parseInspectTypeName() (*V12InspectTypeNameNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type != V12_AT_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected @ref (inspect_type_name), got %s", p.cur().Type))
	}
	// Re-interpret the AT_IDENT as an ident_ref by stripping the leading '@'.
	atTok := p.cur()
	p.advance()
	ref := &V12IdentRefNode{
		V12BaseNode: V12BaseNode{Line: atTok.Line, Col: atTok.Col},
		Dotted:      &V12IdentDottedNode{V12BaseNode: V12BaseNode{Line: atTok.Line, Col: atTok.Col}, Parts: []string{atTok.Value[1:]}},
	}
	if _, err := p.expect(V12_DOT); err != nil {
		return nil, err
	}
	tok := p.cur()
	if tok.Type != V12_IDENT || tok.Value != "name" {
		return nil, p.errAt(fmt.Sprintf("expected 'name' after '.', got %q", tok.Value))
	}
	p.advance()
	return &V12InspectTypeNameNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Ref: ref, Prop: "name"}, nil
}

// ParseTypeDeclare parses:  type_declare = ident_name inspect_type
// Source form: myName @MyType
func (p *V12Parser) ParseTypeDeclare() (*V12TypeDeclareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type != V12_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected ident_name in type_declare, got %s", p.cur().Type))
	}
	name := p.cur().Value
	p.advance()
	inspect, err := p.parseInspectType()
	if err != nil {
		return nil, err
	}
	return &V12TypeDeclareNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Name: name, Inspect: inspect}, nil
}

// ---------- assign_func_rhs ----------

// ParseAssignFuncRHS parses:
//
//	assign_func_rhs = constant | func_string_decl | (regexp | func_regexp_decl) | ident_ref | calc_unit | object_final | array_final
func (p *V12Parser) ParseAssignFuncRHS() (*V12AssignFuncRHSNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V12Node) *V12AssignFuncRHSNode {
		return &V12AssignFuncRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	// func_string_decl (empty string literals)
	case V12_EMPTY_STR_D, V12_EMPTY_STR_S, V12_EMPTY_STR_T:
		n, err := p.ParseEmptyDecl()
		if err != nil {
			return nil, err
		}
		return wrap(n), nil

	// regexp | func_regexp_decl
	case V12_REGEXP, V12_REGEXP_DECL:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil

	// other constant starts
	case V12_TRUE, V12_FALSE, V12_NULL, V12_NAN, V12_INFINITY, V12_STRING:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil

	// object / array
	case V12_LBRACKET, V12_EMPTY_ARR, V12_UNIFORM:
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
	case V12_INTEGER, V12_DECIMAL, V12_PLUS, V12_MINUS:
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil && !V12isExprContinuation(p.cur().Type) {
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

// parseFuncHeaderAssign parses:  func_header_assign = assign_lhs assign_oper assign_func_rhs  (V12: no buildin form)
func (p *V12Parser) parseFuncHeaderAssign() (*V12FuncHeaderAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
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
	return &V12FuncHeaderAssignNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		RHS:         rhs,
	}, nil
}

// parseFuncHeaderUserParams parses:  func_header_user_params = func_header_assign { EOL func_header_assign }
func (p *V12Parser) parseFuncHeaderUserParams() (*V12FuncHeaderUserParamsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.parseFuncHeaderAssign()
	if err != nil {
		return nil, err
	}
	node := &V12FuncHeaderUserParamsNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Items:       []*V12FuncHeaderAssignNode{first},
	}
	for p.tokens[p.pos].Type == V12_NL {
		saved := p.savePos()
		p.V12skipNLs()
		next, err := p.parseFuncHeaderAssign()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, next)
	}
	return node, nil
}

// ---------- ident_static_store_name (V12) ----------
// parseIdentStaticStoreName parses:  ident_static_store_name = ident_name | inspect_type_name
// Returns a V12Node: *V12InspectTypeNameNode for inspect_type_name, or stores a plain string.
// Callers receive the string form directly or *V12InspectTypeNameNode.
func (p *V12Parser) parseIdentStaticStoreName() (string, *V12InspectTypeNameNode, error) {
	// Try inspect_type_name first: AT_IDENT + "." + "name"
	if p.cur().Type == V12_AT_IDENT {
		saved := p.savePos()
		if itn, err := p.parseInspectTypeName(); err == nil {
			return "", itn, nil
		}
		p.restorePos(saved)
	}
	// ident_name: plain IDENT
	if p.cur().Type == V12_IDENT {
		name := p.cur().Value
		p.advance()
		return name, nil, nil
	}
	return "", nil, p.errAt(fmt.Sprintf("expected ident_name or @ref.name in ident_static_store_name, got %s", p.cur().Type))
}

// ---------- func_args_decl ----------

// ParseArgsDecl parses (V12):
//
//	args_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] inspect_type
//	inspect_type = "@TypeName" (AT_IDENT token)
//
// The recursive comma-separated list is flattened into the caller's slice.
func (p *V12Parser) ParseArgsDecl() ([]*V12ArgsDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col

	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}

	// Optional operator: ":" or ":~"
	var oper *V12AssignOperNode
	if p.cur().Type == V12_COLON || p.cur().Type == V12_READONLY {
		oper, err = p.ParseAssignOper()
		if err != nil {
			return nil, err
		}
	}

	// inspect_type (V12): "@TypeName" as AT_IDENT
	inspect, err := p.parseInspectType()
	if err != nil {
		return nil, err
	}

	entry := &V12ArgsDeclNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		Type:        inspect,
	}
	result := []*V12ArgsDeclNode{entry}

	// Recursively parse comma-separated additional entries.
	for p.cur().Type == V12_COMMA {
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
func (p *V12Parser) ParseFuncArgs() (*V12FuncArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_ARROW); err != nil {
		return nil, err
	}
	node := &V12FuncArgsNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}
	if !V12isAssignLHSStart(p.cur().Type) {
		return node, nil // args_decl is optional per func_unit_header usage
	}
	entries, err := p.ParseArgsDecl()
	if err != nil {
		return nil, err
	}
	node.Entries = entries
	return node, nil
}

// V12isAssignLHSStart returns true when tok can begin an assign_lhs (just IDENT).
func V12isAssignLHSStart(t V12TokenType) bool {
	return t == V12_IDENT
}

// ParseFuncStreamArgs parses (V11):  func_stream_args = ">>" args_decl
func (p *V12Parser) ParseFuncStreamArgs() (*V12FuncStreamArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_STREAM); err != nil {
		return nil, err
	}
	entries, err := p.ParseArgsDecl()
	if err != nil {
		return nil, err
	}
	return &V12FuncStreamArgsNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Entries:     entries,
	}, nil
}

// ParseFuncDeps parses (V12):  func_deps = "=>" UNIQUE< ident_static_store_name { "," ident_static_store_name } >
//
// ident_static_store_name = ident_name | inspect_type_name
func (p *V12Parser) ParseFuncDeps() (*V12FuncDepsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_STORE); err != nil {
		return nil, err
	}
	name, itn, err := p.parseIdentStaticStoreName()
	if err != nil {
		return nil, err
	}
	// Encode: plain strings go in names; inspect_type_name represented as "@ref.name"
	encodeStoreName := func(n string, it *V12InspectTypeNameNode) string {
		if it != nil && len(it.Ref.Dotted.Parts) > 0 {
			return "@" + it.Ref.Dotted.Parts[0] + "." + it.Prop
		}
		return n
	}
	names := []string{encodeStoreName(name, itn)}
	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		n2, it2, err2 := p.parseIdentStaticStoreName()
		if err2 != nil {
			p.restorePos(saved)
			break
		}
		names = append(names, encodeStoreName(n2, it2))
	}
	return &V12FuncDepsNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, StoreNames: names}, nil
}

// ParseFuncArgsDecl parses:  func_args_decl = [ func_args ] [ func_stream_args ] [ func_deps ]
func (p *V12Parser) ParseFuncArgsDecl() (*V12FuncArgsDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V12FuncArgsDeclNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}

	if p.cur().Type == V12_ARROW {
		args, err := p.ParseFuncArgs()
		if err != nil {
			return nil, err
		}
		node.Args = args
	}
	if p.cur().Type == V12_STREAM {
		sa, err := p.ParseFuncStreamArgs()
		if err != nil {
			return nil, err
		}
		node.StreamArgs = sa
	}
	if p.cur().Type == V12_STORE {
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
func (p *V12Parser) ParseFuncRangeArgs() (*V12FuncRangeArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V12Node) *V12FuncRangeArgsNode {
		return &V12FuncRangeArgsNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: v}
	}

	// String range: string | TYPE_OF string<ident_ref>
	if V12isStringTok(p.cur().Type) {
		s, err := p.ParseString()
		if err != nil {
			return nil, err
		}
		return wrap(s), nil
	}

	// List range: array_final | object_final
	switch p.cur().Type {
	case V12_LBRACKET, V12_EMPTY_ARR, V12_UNIFORM:
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
	parseFuncNum := func() (V12Node, error) {
		if p.cur().Type == V12_TYPE_OF {
			return p.parseTypeOfRef()
		}
		return p.ParseNumericConst()
	}

	lo, err := parseFuncNum()
	if err != nil {
		// TYPE_OF string form
		if p.cur().Type == V12_TYPE_OF {
			return wrap(lo), nil
		}
		return nil, err
	}
	if p.cur().Type != V12_DOTDOT {
		// Single string TYPE_OF or plain string already handled; this must be an error.
		return nil, p.errAt(fmt.Sprintf("expected '..' in func numeric range, got %s", p.cur().Type))
	}
	p.advance() // consume ".."
	hi, err := parseFuncNum()
	if err != nil {
		return nil, err
	}
	numRange := &V12FuncFixedNumRangeNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Lo:          lo,
		Hi:          hi,
	}
	return wrap(numRange), nil
}

// ---------- func_call / func_call_chain ----------

// ParseFuncCall parses:  func_call = TYPE_OF func_unit<ident_ref> [ func_call_args ]
func (p *V12Parser) ParseFuncCall() (*V12FuncCallNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V12FuncCallNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Ref: ref}

	// Optional call args: assign_func_rhs { "," assign_func_rhs }
	if V12isFuncCallArgStart(p.cur().Type) {
		first, err := p.ParseAssignFuncRHS()
		if err == nil {
			node.Args = append(node.Args, first)
			for p.cur().Type == V12_COMMA {
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

// V12isFuncCallArgStart returns true when the current token can start an assign_func_rhs.
func V12isFuncCallArgStart(t V12TokenType) bool {
	switch t {
	case V12_IDENT, V12_INTEGER, V12_DECIMAL, V12_STRING,
		V12_EMPTY_STR_D, V12_EMPTY_STR_S, V12_EMPTY_STR_T,
		V12_REGEXP, V12_REGEXP_DECL,
		V12_TRUE, V12_FALSE, V12_NULL, V12_NAN, V12_INFINITY,
		V12_PLUS, V12_MINUS,
		V12_LBRACKET, V12_EMPTY_ARR, V12_UNIFORM:
		return true
	}
	return false
}

// ParseFuncCallChain parses:  func_call_chain = func_call [ step { step } ]
func (p *V12Parser) ParseFuncCallChain() (*V12FuncCallChainNode, error) {
	line, col := p.cur().Line, p.cur().Col
	head, err := p.ParseFuncCall()
	if err != nil {
		return nil, err
	}
	node := &V12FuncCallChainNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Head: head}

	for {
		var opStr string
		switch p.cur().Type {
		case V12_ARROW:
			opStr = "->"
		case V12_STREAM:
			opStr = ">>"
		default:
			opStr = V12isBinaryLogicOper(p.cur().Type)
		}
		if opStr == "" {
			break
		}
		saved := p.savePos()
		p.advance() // consume op
		if p.cur().Type != V12_TYPE_OF {
			p.restorePos(saved)
			break
		}
		ref, err := p.parseTypeOfRef()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Steps = append(node.Steps, V12FuncCallChainStepNode{
			V12BaseNode: V12BaseNode{Line: p.cur().Line, Col: p.cur().Col},
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
func (p *V12Parser) ParseFuncStreamLoop() (*V12FuncStreamLoopNode, error) {
	line, col := p.cur().Line, p.cur().Col

	var source V12Node
	// boolean_true: ident "true"
	if p.cur().Type == V12_TRUE {
		b, err := p.ParseBoolean()
		if err != nil {
			return nil, err
		}
		source = b
	} else if p.cur().Type == V12_IDENT {
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

	if _, err := p.expect(V12_STREAM); err != nil {
		return nil, err
	}

	var body V12Node
	var err error
	if p.cur().Type == V12_LBRACE {
		body, err = p.ParseFuncUnit()
	} else {
		body, err = p.ParseFuncCall()
	}
	if err != nil {
		return nil, err
	}
	return &V12FuncStreamLoopNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Source: source, Body: body}, nil
}

// ParseFuncCallFinal parses:  func_call_final = func_call_chain | func_stream_loop
func (p *V12Parser) ParseFuncCallFinal() (*V12FuncCallFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	// Both alternatives can start with TYPE_OF or with an ident/literal.
	// func_call starts with TYPE_OF; func_stream_loop can start with anything.
	// If current token is TYPE_OF, try func_call_chain first.
	if p.cur().Type == V12_TYPE_OF {
		saved := p.savePos()
		if cc, err := p.ParseFuncCallChain(); err == nil {
			return &V12FuncCallFinalNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: cc}, nil
		}
		p.restorePos(saved)
	}
	sl, err := p.ParseFuncStreamLoop()
	if err != nil {
		return nil, err
	}
	return &V12FuncCallFinalNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: sl}, nil
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
func (p *V12Parser) ParseFuncInject() (*V12FuncInjectNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_LPAREN); err != nil {
		return nil, err
	}

	node := &V12FuncInjectNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}

	// Head is entirely optional — if the next token is ")" skip it.
	if p.cur().Type != V12_RPAREN {
		node.Head = p.parseFuncInjectHead(line, col)
	}

	// Additional "," assign_lhs ( ":" | ":~" ) ident_ref entries.
	for p.cur().Type == V12_COMMA {
		saved := p.savePos()
		p.advance()
		lhs, err := p.ParseAssignLHS()
		if err != nil {
			p.restorePos(saved)
			break
		}
		if p.cur().Type != V12_COLON && p.cur().Type != V12_READONLY {
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
		node.Binds = append(node.Binds, V12FuncInjectBind{LHS: lhs, Oper: oper, Ref: ref})
	}

	if _, err := p.expect(V12_RPAREN); err != nil {
		return nil, err
	}
	return node, nil
}

// parseFuncInjectHead tries to parse the optional head element of func_inject.
// Returns nil if nothing matches (empty head).
func (p *V12Parser) parseFuncInjectHead(line, col int) V12Node {
	// Try: assign_lhs inspect_type [ empty_array_decl ]
	// Both forms start with IDENT. Discriminate by what follows the lhs.
	saved := p.savePos()
	lhs, lhsErr := p.ParseAssignLHS()
	if lhsErr == nil {
		// Case 1: assign_lhs then inspect_type (V12: "@TypeName" AT_IDENT)
		if p.cur().Type == V12_AT_IDENT {
			innerSaved := p.savePos()
			access, aErr := p.parseInspectType()
			if aErr == nil {
				head := &V12FuncInjectHeadInspectNode{
					V12BaseNode: V12BaseNode{Line: line, Col: col},
					LHS:         lhs,
					Inspect:     access,
				}
				if p.cur().Type == V12_EMPTY_ARR {
					head.HasArray = true
					p.advance()
				}
				return head
			}
			p.restorePos(innerSaved)
		}
		// Case 2: assign_lhs ( ":" | ":~" ) ident_ref
		if p.cur().Type == V12_COLON || p.cur().Type == V12_READONLY {
			oper, operErr := p.ParseAssignOper()
			if operErr == nil {
				ref, refErr := p.ParseIdentRef()
				if refErr == nil {
					return &V12FuncInjectBindNode{
						V12BaseNode: V12BaseNode{Line: line, Col: col},
						Bind:        V12FuncInjectBind{LHS: lhs, Oper: oper, Ref: ref},
					}
				}
			}
		}
	}
	p.restorePos(saved)
	// Case 3: bare ident_ref (no lhs, no operator)
	ref, refErr := p.ParseIdentRef()
	if refErr == nil {
		return &V12FuncInjectBindNode{
			V12BaseNode: V12BaseNode{Line: line, Col: col},
			Bind:        V12FuncInjectBind{Ref: ref},
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
func (p *V12Parser) ParseFuncStmt() (*V12FuncStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V12Node) *V12FuncStmtNode {
		return &V12FuncStmtNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	// return_func_unit (EXTEND): "<-" func_unit
	case V12_RETURN_STMT:
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V12_LBRACE || p.cur().Type == V12_LPAREN {
			unit, err := p.ParseFuncUnit()
			if err == nil {
				return wrap(&V12ReturnFuncUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Unit: unit}), nil
			}
		}
		p.restorePos(saved)

	// self_ref: "$"
	case V12_DOLLAR:
		p.advance()
		return wrap(&V12SelfRefNode{V12BaseNode: V12BaseNode{Line: line, Col: col}}), nil

	// func_unit: "{"
	case V12_LBRACE:
		unit, err := p.ParseFuncUnit()
		if err != nil {
			return nil, err
		}
		return wrap(unit), nil

	// object_final / array_final
	case V12_LBRACKET, V12_EMPTY_ARR, V12_UNIFORM:
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
	case V12_TYPE_OF:
		saved := p.savePos()
		if cc, err := p.ParseFuncCallChain(); err == nil {
			return wrap(cc), nil
		}
		p.restorePos(saved)
	}

	// regexp / constant
	if p.cur().Type == V12_REGEXP || p.cur().Type == V12_REGEXP_DECL {
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
func (p *V12Parser) ParseFuncAssign() (*V12FuncAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var inj *V12FuncInjectNode
	if p.cur().Type == V12_LPAREN {
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
	return &V12FuncAssignNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Inject:      inj,
		LHS:         lhs,
		Oper:        oper,
		Stmt:        stmt,
	}, nil
}

// ---------- func body statements ----------

// ParseFuncReturnStmt parses:  func_return_stmt = "<-" func_stmt
func (p *V12Parser) ParseFuncReturnStmt() (*V12FuncReturnStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_RETURN_STMT); err != nil {
		return nil, err
	}
	stmt, err := p.ParseFuncStmt()
	if err != nil {
		return nil, err
	}
	return &V12FuncReturnStmtNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Stmt: stmt}, nil
}

// ParseFuncStoreStmt parses:
//
//	func_store_stmt = "=>" ( object_final | TYPE_OF object_final<ident_ref> ) { "," ... }
func (p *V12Parser) ParseFuncStoreStmt() (*V12FuncStoreStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_STORE); err != nil {
		return nil, err
	}
	parseItem := func() (V12Node, error) {
		if p.cur().Type == V12_TYPE_OF {
			return p.parseTypeOfRef()
		}
		return p.ParseObjectFinal()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	node := &V12FuncStoreStmtNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Items: []V12Node{first}}
	for p.cur().Type == V12_COMMA {
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

// parseCondReturnStmt parses the Squeeze conditional-return form:
//
//	cond_return_stmt = "(" logic_expr ")" logic_oper func_return_stmt
//
// Example:  ( a >= -128 & a <= 127 ) & <- [ type: $, value: a ]
func (p *V12Parser) parseCondReturnStmt() (*V12CondReturnStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_LPAREN); err != nil {
		return nil, err
	}
	cond, err := p.ParseLogicExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_RPAREN); err != nil {
		return nil, err
	}
	// logic_oper: &, |, ^
	operTok := p.cur()
	switch operTok.Type {
	case V12_AMP, V12_PIPE, V12_CARET:
		p.advance()
	default:
		return nil, p.errAt(fmt.Sprintf("expected logic operator after condition, got %s %q", operTok.Type, operTok.Value))
	}
	ret, err := p.ParseFuncReturnStmt()
	if err != nil {
		return nil, err
	}
	return &V12CondReturnStmtNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Cond:        cond,
		Oper:        operTok.Value,
		Return:      ret,
	}, nil
}

// ParseFuncBodyStmt parses:
//
//	func_body_stmt = func_assign | func_return_stmt | func_store_stmt | func_stream_loop | func_call_final | cond_return_stmt
func (p *V12Parser) ParseFuncBodyStmt() (*V12FuncBodyStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V12Node) *V12FuncBodyStmtNode {
		return &V12FuncBodyStmtNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V12_RETURN_STMT:
		rs, err := p.ParseFuncReturnStmt()
		if err != nil {
			return nil, err
		}
		return wrap(rs), nil

	case V12_STORE:
		ss, err := p.ParseFuncStoreStmt()
		if err != nil {
			return nil, err
		}
		return wrap(ss), nil

	case V12_LPAREN:
		// cond_return_stmt: "(" logic_expr ")" logic_oper func_return_stmt
		saved := p.savePos()
		if crs, err := p.parseCondReturnStmt(); err == nil {
			return wrap(crs), nil
		}
		p.restorePos(saved)
	}

	// func_call_final or func_stream_loop (both can start with TYPE_OF or ident)
	// Try func_call_final first.
	if p.cur().Type == V12_TYPE_OF {
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

// ParseFuncUnit parses (V12):
//
//	func_unit = "{" func_unit_header func_body_stmt { NL | func_body_stmt } "}"
//	          | "(" func_unit_header func_body_stmt { NL | func_body_stmt } ")"
func (p *V12Parser) ParseFuncUnit() (*V12FuncUnitNode, error) {
	line, col := p.cur().Line, p.cur().Col

	// Determine delimiter form: "{...}" (scope) or "(...)" (group)
	useGroup := false
	switch p.cur().Type {
	case V12_LBRACE:
		p.advance()
	case V12_LPAREN:
		useGroup = true
		p.advance()
	default:
		return nil, p.errAt(fmt.Sprintf("expected '{' or '(' to begin func_unit, got %s", p.cur().Type))
	}

	// func_unit_header: optional func_header_user_params then args_decl (V12: no enclosure)
	hdr := &V12FuncUnitHeaderNode{V12BaseNode: V12BaseNode{Line: p.cur().Line, Col: p.cur().Col}}
	if !V12isFuncBodyStart(p.cur()) {
		// Try func_header_user_params (assign stmts like  name: "val")
		saved := p.savePos()
		if up, err := p.parseFuncHeaderUserParams(); err == nil {
			hdr.UserParams = up
		} else {
			p.restorePos(saved)
		}
		argsDecl, _ := p.ParseFuncArgsDecl()
		hdr.ArgsDecl = argsDecl
	}
	p.V12skipNLs()

	// At least one func_body_stmt required.
	first, err := p.ParseFuncBodyStmt()
	if err != nil {
		return nil, err
	}
	body := []*V12FuncBodyStmtNode{first}

	closeType := V12_RBRACE
	if useGroup {
		closeType = V12_RPAREN
	}
	for {
		p.V12skipNLs()
		if p.cur().Type == closeType || p.cur().Type == V12_EOF {
			break
		}
		stmt, err := p.ParseFuncBodyStmt()
		if err != nil {
			break
		}
		body = append(body, stmt)
	}

	if _, err := p.expect(closeType); err != nil {
		return nil, err
	}
	return &V12FuncUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Header: hdr, Body: body, UseGroupDelim: useGroup}, nil
}

// V12isFuncBodyStart returns true for tokens that can only begin a func_body_stmt,
// not a header (used to decide whether to try parsing headers at all).
func V12isFuncBodyStart(tok V12Token) bool {
	switch tok.Type {
	case V12_RETURN_STMT, V12_STORE, V12_DOLLAR:
		return true
	}
	return false
}

// ---------- function-related updates ----------

// ParseReturnFuncUnit parses:  return_func_unit = "<-" func_unit
func (p *V12Parser) ParseReturnFuncUnit() (*V12ReturnFuncUnitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V12_RETURN_STMT); err != nil {
		return nil, err
	}
	unit, err := p.ParseFuncUnit()
	if err != nil {
		return nil, err
	}
	return &V12ReturnFuncUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Unit: unit}, nil
}

// ParseUpdateFuncUnit parses:  update_func_unit = TYPE_OF func_unit<ident_ref> equal_assign return_func_unit
func (p *V12Parser) ParseUpdateFuncUnit() (*V12UpdateFuncUnitNode, error) {
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
	return &V12UpdateFuncUnitNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Ref: ref, Assign: oper, NewUnit: rfu}, nil
}

// ---------- array_idx_recursive / ident_ref extension ----------

// ParseArrayIdxRecursive parses:  array_idx_recursive = "[" numeric_rhs "]" { "[" numeric_rhs "]" }
func (p *V12Parser) ParseArrayIdxRecursive() (*V12ArrayIdxRecursiveNode, error) {
	line, col := p.cur().Line, p.cur().Col
	parseNumRHS := func() (V12Node, error) {
		// numeric_rhs = numeric_const | numeric_stmt
		if p.cur().Type == V12_TYPE_OF {
			return p.parseTypeOfRef() // numeric_stmt is TYPE_OF numeric_const<...>
		}
		return p.ParseNumericConst()
	}
	if _, err := p.expect(V12_LBRACKET); err != nil {
		return nil, err
	}
	first, err := parseNumRHS()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V12_RBRACKET); err != nil {
		return nil, err
	}
	node := &V12ArrayIdxRecursiveNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Indices: []V12Node{first}}
	for p.cur().Type == V12_LBRACKET {
		saved := p.savePos()
		p.advance()
		idx, err := parseNumRHS()
		if err != nil {
			p.restorePos(saved)
			break
		}
		if _, err2 := p.expect(V12_RBRACKET); err2 != nil {
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
func (p *V12Parser) ParseUpdateNumber() (*V12UpdateNumberNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}
	var rhs V12Node
	if p.cur().Type == V12_TYPE_OF {
		rhs, err = p.parseTypeOfRef()
	} else {
		rhs, err = p.ParseNumericConst()
	}
	if err != nil {
		return nil, err
	}
	return &V12UpdateNumberNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Target: ref, Oper: oper, RHS: rhs}, nil
}

// ParseStringUpdateOper parses:  string_update_oper = "+:" | "+~" | equal_assign
func (p *V12Parser) ParseStringUpdateOper() (*V12StringUpdateOperNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var kind V12StringUpdateOperKind
	var tv string
	switch tok.Type {
	case V12_IADD_IMM:
		kind, tv = V12StringAppendImmutable, "+:"
	case V12_IADD_MUT:
		kind, tv = V12StringAppendMutable, "+~"
	case V12_EQ, V12_COLON, V12_READONLY:
		kind, tv = V12StringEqualAssign, tok.Value
	default:
		return nil, p.errAt(fmt.Sprintf("expected string update operator (+:, +~, =, :, :~), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V12StringUpdateOperNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Kind: kind, Token: tv}, nil
}

// ParseUpdateString parses:
//
//	update_string = TYPE_OF string<ident_ref> string_update_oper string_rhs
func (p *V12Parser) ParseUpdateString() (*V12UpdateStringNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseStringUpdateOper()
	if err != nil {
		return nil, err
	}
	var rhs V12Node
	if p.cur().Type == V12_TYPE_OF {
		rhs, err = p.parseTypeOfRef()
	} else {
		rhs, err = p.ParseString()
	}
	if err != nil {
		return nil, err
	}
	return &V12UpdateStringNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Target: ref, Oper: oper, RHS: rhs}, nil
}

// ---------- ident_ref_update ----------

// ParseIdentRefUpdate parses:  ident_ref_update = ident_ref assign_oper assign_rhs
func (p *V12Parser) ParseIdentRefUpdate() (*V12IdentRefUpdateNode, error) {
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
	return &V12IdentRefUpdateNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Ref: ref, Oper: oper, RHS: rhs}, nil
}

// The _ below suppresses an unused import warning for "fmt" in case some
// error-path functions are elided by the compiler in certain build modes.
var _ = fmt.Sprintf
