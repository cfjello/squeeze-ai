// parser_v13_functions.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/06_functions.sqg.
//
// V13 changes vs V12:
//   - inspect_type includes any_type (@?) as a valid type reference
//   - string_append_mutable is "+=" only (V12 had "+~" as an additional form)
//   - dependency_oper / func_store_stmt formalised as V13 (were inline "=>" in V12)
//   - ParseFuncUnit and ParseReturnFuncUnit are forward-referenced by assignment.go
//
// Covered rules:
//
//	type_prefix, inspect_type, inspect_type_name, type_declare
//	assign_func_rhs, func_header_assign, func_header_user_params
//	ident_static_store_name
//	dependency_oper
//	func_args, func_stream_args, func_deps, func_args_decl
//	func_fixed_num_range, func_fixed_list_range, func_range_args
//	func_call_args, func_call, func_call_chain
//	func_stream_loop, func_call_final
//	func_inject, func_stmt, func_assign
//	func_return_stmt, func_store_stmt, func_body_stmt
//	func_unit_header, func_unit
//	return_func_unit, EXTEND<func_stmt>, update_func_unit
//	array_idx_recursive, EXTEND<ident_ref>
//	numeric_stmt, numeric_rhs, update_number
//	string_stmt, string_rhs, string_update_oper, update_string
//	ident_ref_update
//	MERGE<assignment>
package parser

import (
	"fmt"
	"regexp"
)

// V13urlRe matches http/https URLs.
var V13urlRe = regexp.MustCompile(`^https?://`)

// =============================================================================
// PHASE 2 — AST NODE TYPES  (06_functions.sqg)
// =============================================================================

// ---------- inspect_type / inspect_type_name ----------

// V13InspectTypeNode  inspect_type = type_prefix !WS! ident_ref | any_type
// Source form: @MyType (or @? for any_type)
type V13InspectTypeNode struct {
	V13BaseNode
	Name string // e.g. "@MyType" or "@?"
}

// V13InspectTypeNameNode  inspect_type_name = "@" ident_ref "." "name"
type V13InspectTypeNameNode struct {
	V13BaseNode
	Ref  *V13IdentRefNode
	Prop string // always "name"
}

// V13TypeDeclareNode  type_declare = ident_name inspect_type
type V13TypeDeclareNode struct {
	V13BaseNode
	Name    string
	Inspect *V13InspectTypeNode
}

// V13AssignFuncRHSNode  assign_func_rhs = constant | func_string_decl | regexp | ident_ref | calc_unit | object_final | array_final
type V13AssignFuncRHSNode struct {
	V13BaseNode
	Value V13Node
}

// V13FuncHeaderAssignNode  func_header_assign = assign_lhs assign_oper assign_func_rhs
type V13FuncHeaderAssignNode struct {
	V13BaseNode
	LHS  *V13AssignLHSNode
	Oper *V13AssignOperNode
	RHS  *V13AssignFuncRHSNode
}

// V13FuncHeaderUserParamsNode  func_header_user_params = func_header_assign { EOL func_header_assign }
type V13FuncHeaderUserParamsNode struct {
	V13BaseNode
	Items []*V13FuncHeaderAssignNode
}

// V13ArgsDeclNode  args_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] inspect_type
type V13ArgsDeclNode struct {
	V13BaseNode
	LHS  *V13AssignLHSNode
	Oper *V13AssignOperNode // nil when absent
	Type *V13InspectTypeNode
}

// V13FuncArgsNode  func_args = "->" args_decl { "," args_decl }
type V13FuncArgsNode struct {
	V13BaseNode
	Entries []*V13ArgsDeclNode
}

// V13FuncStreamArgsNode  func_stream_args = ">>" args_decl { "," args_decl }
type V13FuncStreamArgsNode struct {
	V13BaseNode
	Entries []*V13ArgsDeclNode
}

// V13FuncDepsNode  func_deps = dependency_oper UNIQUE< ident_static_store_name { "," ident_static_store_name } >
// dependency_oper = "=>" (V13_STORE token)
type V13FuncDepsNode struct {
	V13BaseNode
	StoreNames []string
}

// V13FuncArgsDeclNode  func_args_decl = [ func_args ] [ func_stream_args ] [ func_deps ]
type V13FuncArgsDeclNode struct {
	V13BaseNode
	Args       *V13FuncArgsNode
	StreamArgs *V13FuncStreamArgsNode
	Deps       *V13FuncDepsNode
}

// V13FuncFixedNumRangeNode  numeric_const ".." numeric_const
type V13FuncFixedNumRangeNode struct {
	V13BaseNode
	Lo V13Node
	Hi V13Node
}

// V13FuncRangeArgsNode  func_range_args = num_range | str_range | list_range
type V13FuncRangeArgsNode struct {
	V13BaseNode
	Value V13Node
}

// V13FuncCallNode  func_call = TYPE_OF func_unit<ident_ref> [ func_call_args ]
type V13FuncCallNode struct {
	V13BaseNode
	Ref  *V13TypeOfRefNode
	Args []V13Node
}

// V13FuncCallChainStepNode  one step in a func_call_chain
type V13FuncCallChainStepNode struct {
	V13BaseNode
	Op  string
	Ref *V13TypeOfRefNode
}

// V13FuncCallChainNode  func_call_chain = func_call [ step { step } ]
type V13FuncCallChainNode struct {
	V13BaseNode
	Head  *V13FuncCallNode
	Steps []V13FuncCallChainStepNode
}

// V13FuncStreamLoopNode  func_stream_loop = source ">>" body
type V13FuncStreamLoopNode struct {
	V13BaseNode
	Source V13Node
	Body   V13Node
}

// V13FuncCallFinalNode  func_call_final = func_call_chain | func_stream_loop
type V13FuncCallFinalNode struct {
	V13BaseNode
	Value V13Node
}

// V13FuncInjectBind  assign_lhs ( ":" | ":~" ) ident_ref
type V13FuncInjectBind struct {
	LHS  *V13AssignLHSNode
	Oper *V13AssignOperNode // nil allowed for head
	Ref  *V13IdentRefNode
}

// V13FuncInjectBindNode  wraps V13FuncInjectBind as a V13Node
type V13FuncInjectBindNode struct {
	V13BaseNode
	Bind V13FuncInjectBind
}

// V13FuncInjectHeadInspectNode  assign_lhs inspect_type [ "[]" ]
type V13FuncInjectHeadInspectNode struct {
	V13BaseNode
	LHS      *V13AssignLHSNode
	Inspect  *V13InspectTypeNode
	HasArray bool
}

// V13FuncInjectNode  func_inject
type V13FuncInjectNode struct {
	V13BaseNode
	Head  V13Node
	Binds []V13FuncInjectBind
}

// V13FuncStmtNode  func_stmt = regexp | ident_ref | object_final | array_final | func_call_chain | func_unit | calc_unit | self_ref
type V13FuncStmtNode struct {
	V13BaseNode
	Value V13Node
}

// V13FuncAssignNode  func_assign = [ func_inject ] assign_lhs assign_oper func_stmt
type V13FuncAssignNode struct {
	V13BaseNode
	Inject *V13FuncInjectNode
	LHS    *V13AssignLHSNode
	Oper   *V13AssignOperNode
	Stmt   *V13FuncStmtNode
}

// V13FuncReturnStmtNode  func_return_stmt = "<-" func_stmt
type V13FuncReturnStmtNode struct {
	V13BaseNode
	Stmt *V13FuncStmtNode
}

// V13CondReturnStmtNode  cond_return_stmt = "(" logic_expr ")" logic_oper func_return_stmt
type V13CondReturnStmtNode struct {
	V13BaseNode
	Cond   V13Node
	Oper   string
	Return *V13FuncReturnStmtNode
}

// V13FuncStoreStmtNode  func_store_stmt = dependency_oper ( object_final | TYPE_OF object_final<ident_ref> ) { "," ... }
// dependency_oper = "=>" (V13_STORE); publishes a new UUIDv7-stamped version of a named data object.
type V13FuncStoreStmtNode struct {
	V13BaseNode
	Items []V13Node
}

// V13FuncBodyStmtNode  func_body_stmt = func_assign | func_return_stmt | func_store_stmt | ...
type V13FuncBodyStmtNode struct {
	V13BaseNode
	Value V13Node
}

// V13FuncUnitHeaderNode  func_unit_header
type V13FuncUnitHeaderNode struct {
	V13BaseNode
	UserParams *V13FuncHeaderUserParamsNode
	ArgsDecl   *V13FuncArgsDeclNode
}

// V13FuncUnitNode  func_unit = ( "{" ... "}" ) | ( "(" ... ")" )
type V13FuncUnitNode struct {
	V13BaseNode
	Header        *V13FuncUnitHeaderNode
	Body          []*V13FuncBodyStmtNode
	UseGroupDelim bool
}

// V13ReturnFuncUnitNode  return_func_unit = "<-" func_unit
type V13ReturnFuncUnitNode struct {
	V13BaseNode
	Unit *V13FuncUnitNode
}

// V13UpdateFuncUnitNode  update_func_unit = TYPE_OF func_unit<ident_ref> equal_assign return_func_unit
type V13UpdateFuncUnitNode struct {
	V13BaseNode
	Ref     *V13TypeOfRefNode
	Assign  *V13AssignOperNode
	NewUnit *V13ReturnFuncUnitNode
}

// V13ArrayIdxRecursiveNode  array_idx_recursive = "[" numeric_rhs "]" { "[" numeric_rhs "]" }
type V13ArrayIdxRecursiveNode struct {
	V13BaseNode
	Indices []V13Node
}

// V13NumericStmtNode  numeric_stmt = TYPE_OF numeric_const<ident_ref | func_call_chain | func_unit | calc_unit>
type V13NumericStmtNode struct {
	V13BaseNode
	Ref V13TypeOfRefNode
}

// V13UpdateNumberNode  update_number = TYPE_OF numeric_const<ident_ref> assign_oper numeric_rhs
type V13UpdateNumberNode struct {
	V13BaseNode
	Target *V13TypeOfRefNode
	Oper   *V13AssignOperNode
	RHS    V13Node
}

// V13StringStmtNode  string_stmt = TYPE_OF string<ident_ref | func_call_chain | func_unit | calc_unit>
type V13StringStmtNode struct {
	V13BaseNode
	Ref V13TypeOfRefNode
}

// V13StringUpdateOperKind classifies the string update operator.
type V13StringUpdateOperKind int

const (
	V13StringAppendImmutable V13StringUpdateOperKind = iota // +:
	V13StringAppendMutable                                  // += (V13; V12 used +~)
	V13StringEqualAssign                                    // = : :~
)

// V13StringUpdateOperNode  string_update_oper
type V13StringUpdateOperNode struct {
	V13BaseNode
	Kind  V13StringUpdateOperKind
	Token string
}

// V13UpdateStringNode  update_string = TYPE_OF string<ident_ref> string_update_oper string_rhs
type V13UpdateStringNode struct {
	V13BaseNode
	Target *V13TypeOfRefNode
	Oper   *V13StringUpdateOperNode
	RHS    V13Node
}

// V13IdentRefUpdateNode  ident_ref_update = ident_ref assign_oper assign_rhs
type V13IdentRefUpdateNode struct {
	V13BaseNode
	Ref  *V13IdentRefNode
	Oper *V13AssignOperNode
	RHS  *V13AssignRHSNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (06_functions.sqg)
// =============================================================================

// ---------- helpers ----------

func V13isBinaryLogicOper(t V13TokenType) string {
	switch t {
	case V13_AMP_AMP:
		return "&&"
	case V13_PIPE_PIPE:
		return "||"
	}
	return ""
}

func (p *V13Parser) V13skipNLs() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == V13_NL {
		p.pos++
	}
}

// V13isAssignLHSStart returns true when tok can begin an assign_lhs.
func V13isAssignLHSStart(t V13TokenType) bool {
	return t == V13_IDENT
}

// ---------- inspect_type ----------

// ParseInspectType parses:  inspect_type = "@TypeName" | "@?" | "§ident"
func (p *V13Parser) ParseInspectType() (*V13InspectTypeNode, error) {
	tok := p.cur()
	switch tok.Type {
	case V13_ANY_TYPE:
		// V13: @? is a first-class inspect_type
		node := &V13InspectTypeNode{V13BaseNode: V13BaseNode{Line: tok.Line, Col: tok.Col}, Name: "@?"}
		p.advance()
		return node, nil
	case V13_AT_IDENT:
		node := &V13InspectTypeNode{V13BaseNode: V13BaseNode{Line: tok.Line, Col: tok.Col}, Name: tok.Value}
		p.advance()
		return node, nil
	case V13_SECTION:
		line, col := tok.Line, tok.Col
		p.advance()
		if p.cur().Type != V13_IDENT {
			return nil, p.errAt(fmt.Sprintf("expected identifier after §, got %s", p.cur().Type))
		}
		name := "§" + p.cur().Value
		p.advance()
		return &V13InspectTypeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: name}, nil
	}
	return nil, p.errAt(fmt.Sprintf("expected @TypeName or @? (inspect_type), got %s %q", tok.Type, tok.Value))
}

func (p *V13Parser) parseInspectTypeName() (*V13InspectTypeNameNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type != V13_AT_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected @ref (inspect_type_name), got %s", p.cur().Type))
	}
	atTok := p.cur()
	p.advance()
	ref := &V13IdentRefNode{
		V13BaseNode: V13BaseNode{Line: atTok.Line, Col: atTok.Col},
		Dotted:      &V13IdentDottedNode{V13BaseNode: V13BaseNode{Line: atTok.Line, Col: atTok.Col}, Parts: []string{atTok.Value[1:]}},
	}
	if _, err := p.expect(V13_DOT); err != nil {
		return nil, err
	}
	tok := p.cur()
	if tok.Type != V13_IDENT || tok.Value != "name" {
		return nil, p.errAt(fmt.Sprintf("expected 'name' after '.', got %q", tok.Value))
	}
	p.advance()
	return &V13InspectTypeNameNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Ref: ref, Prop: "name"}, nil
}

// ParseTypeDeclare parses:  type_declare = ident_name inspect_type
func (p *V13Parser) ParseTypeDeclare() (*V13TypeDeclareNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected ident_name in type_declare, got %s", p.cur().Type))
	}
	name := p.cur().Value
	p.advance()
	inspect, err := p.ParseInspectType()
	if err != nil {
		return nil, err
	}
	return &V13TypeDeclareNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: name, Inspect: inspect}, nil
}

// ---------- assign_func_rhs ----------

func (p *V13Parser) ParseAssignFuncRHS() (*V13AssignFuncRHSNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13AssignFuncRHSNode {
		return &V13AssignFuncRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T:
		n, err := p.ParseEmptyDecl()
		if err != nil {
			return nil, err
		}
		return wrap(n), nil

	case V13_REGEXP, V13_REGEXP_DECL:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil

	case V13_TRUE, V13_FALSE, V13_NULL, V13_NAN, V13_INFINITY, V13_STRING:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil

	case V13_LBRACKET, V13_EMPTY_ARR, V13_UNIFORM:
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

	case V13_INTEGER, V13_DECIMAL, V13_PLUS, V13_MINUS:
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil && !V13isExprContinuation(p.cur().Type) {
			return wrap(c), nil
		}
		p.restorePos(saved)
	}

	cu, err := p.ParseCalcUnit()
	if err != nil {
		return nil, err
	}
	return wrap(cu), nil
}

// ---------- func_header_user_params ----------

func (p *V13Parser) parseFuncHeaderAssign() (*V13FuncHeaderAssignNode, error) {
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
	return &V13FuncHeaderAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		RHS:         rhs,
	}, nil
}

func (p *V13Parser) parseFuncHeaderUserParams() (*V13FuncHeaderUserParamsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	first, err := p.parseFuncHeaderAssign()
	if err != nil {
		return nil, err
	}
	node := &V13FuncHeaderUserParamsNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Items:       []*V13FuncHeaderAssignNode{first},
	}
	for p.tokens[p.pos].Type == V13_NL {
		saved := p.savePos()
		p.V13skipNLs()
		next, err := p.parseFuncHeaderAssign()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Items = append(node.Items, next)
	}
	return node, nil
}

// ---------- ident_static_store_name ----------

func (p *V13Parser) parseIdentStaticStoreName() (string, *V13InspectTypeNameNode, error) {
	if p.cur().Type == V13_AT_IDENT {
		saved := p.savePos()
		if itn, err := p.parseInspectTypeName(); err == nil {
			return "", itn, nil
		}
		p.restorePos(saved)
	}
	if p.cur().Type == V13_IDENT {
		name := p.cur().Value
		p.advance()
		return name, nil, nil
	}
	return "", nil, p.errAt(fmt.Sprintf("expected ident_name or @ref.name, got %s", p.cur().Type))
}

// ---------- func_args_decl ----------

func (p *V13Parser) ParseArgsDecl() ([]*V13ArgsDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col

	lhs, err := p.ParseAssignLHS()
	if err != nil {
		return nil, err
	}

	var oper *V13AssignOperNode
	if p.cur().Type == V13_COLON || p.cur().Type == V13_READONLY {
		oper, err = p.ParseAssignOper()
		if err != nil {
			return nil, err
		}
	}

	inspect, err := p.ParseInspectType()
	if err != nil {
		return nil, err
	}

	entry := &V13ArgsDeclNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		Type:        inspect,
	}
	result := []*V13ArgsDeclNode{entry}

	for p.cur().Type == V13_COMMA {
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

func (p *V13Parser) ParseFuncArgs() (*V13FuncArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_ARROW); err != nil {
		return nil, err
	}
	node := &V13FuncArgsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}
	if !V13isAssignLHSStart(p.cur().Type) {
		return node, nil
	}
	entries, err := p.ParseArgsDecl()
	if err != nil {
		return nil, err
	}
	node.Entries = entries
	return node, nil
}

func (p *V13Parser) ParseFuncStreamArgs() (*V13FuncStreamArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_STREAM); err != nil {
		return nil, err
	}
	entries, err := p.ParseArgsDecl()
	if err != nil {
		return nil, err
	}
	return &V13FuncStreamArgsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Entries: entries}, nil
}

func (p *V13Parser) ParseFuncDeps() (*V13FuncDepsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_STORE); err != nil {
		return nil, err
	}
	name, itn, err := p.parseIdentStaticStoreName()
	if err != nil {
		return nil, err
	}
	encodeStoreName := func(n string, it *V13InspectTypeNameNode) string {
		if it != nil && len(it.Ref.Dotted.Parts) > 0 {
			return "@" + it.Ref.Dotted.Parts[0] + "." + it.Prop
		}
		return n
	}
	names := []string{encodeStoreName(name, itn)}
	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		n2, it2, err2 := p.parseIdentStaticStoreName()
		if err2 != nil {
			p.restorePos(saved)
			break
		}
		names = append(names, encodeStoreName(n2, it2))
	}
	return &V13FuncDepsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, StoreNames: names}, nil
}

func (p *V13Parser) ParseFuncArgsDecl() (*V13FuncArgsDeclNode, error) {
	line, col := p.cur().Line, p.cur().Col
	node := &V13FuncArgsDeclNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}

	if p.cur().Type == V13_ARROW {
		args, err := p.ParseFuncArgs()
		if err != nil {
			return nil, err
		}
		node.Args = args
	}
	if p.cur().Type == V13_STREAM {
		sa, err := p.ParseFuncStreamArgs()
		if err != nil {
			return nil, err
		}
		node.StreamArgs = sa
	}
	if p.cur().Type == V13_STORE {
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

func (p *V13Parser) ParseFuncRangeArgs() (*V13FuncRangeArgsNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13FuncRangeArgsNode {
		return &V13FuncRangeArgsNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	if V13isStringTok(p.cur().Type) {
		s, err := p.ParseString()
		if err != nil {
			return nil, err
		}
		return wrap(s), nil
	}

	switch p.cur().Type {
	case V13_LBRACKET, V13_EMPTY_ARR, V13_UNIFORM:
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

	parseFuncNum := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			return p.parseTypeOfRef()
		}
		return p.ParseNumericConst()
	}

	lo, err := parseFuncNum()
	if err != nil {
		if p.cur().Type == V13_TYPE_OF {
			return wrap(lo), nil
		}
		return nil, err
	}
	if p.cur().Type != V13_DOTDOT {
		return nil, p.errAt(fmt.Sprintf("expected '..' in func numeric range, got %s", p.cur().Type))
	}
	p.advance()
	hi, err := parseFuncNum()
	if err != nil {
		return nil, err
	}
	numRange := &V13FuncFixedNumRangeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}
	return wrap(numRange), nil
}

// ---------- func_call / func_call_chain ----------

// V13isFuncCallArgStart returns true when the current token can start an assign_func_rhs.
func V13isFuncCallArgStart(t V13TokenType) bool {
	switch t {
	case V13_IDENT, V13_INTEGER, V13_DECIMAL, V13_STRING,
		V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T,
		V13_REGEXP, V13_REGEXP_DECL,
		V13_TRUE, V13_FALSE, V13_NULL, V13_NAN, V13_INFINITY,
		V13_PLUS, V13_MINUS,
		V13_LBRACKET, V13_EMPTY_ARR, V13_UNIFORM:
		return true
	}
	return false
}

func (p *V13Parser) ParseFuncCall() (*V13FuncCallNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	node := &V13FuncCallNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Ref: ref}

	if V13isFuncCallArgStart(p.cur().Type) {
		first, err := p.ParseAssignFuncRHS()
		if err == nil {
			node.Args = append(node.Args, first)
			for p.cur().Type == V13_COMMA {
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

func (p *V13Parser) ParseFuncCallChain() (*V13FuncCallChainNode, error) {
	line, col := p.cur().Line, p.cur().Col
	head, err := p.ParseFuncCall()
	if err != nil {
		return nil, err
	}
	node := &V13FuncCallChainNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Head: head}

	for {
		var opStr string
		switch p.cur().Type {
		case V13_ARROW:
			opStr = "->"
		case V13_STREAM:
			opStr = ">>"
		default:
			opStr = V13isBinaryLogicOper(p.cur().Type)
		}
		if opStr == "" {
			break
		}
		saved := p.savePos()
		p.advance()
		if p.cur().Type != V13_TYPE_OF {
			p.restorePos(saved)
			break
		}
		ref, err := p.parseTypeOfRef()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Steps = append(node.Steps, V13FuncCallChainStepNode{
			V13BaseNode: V13BaseNode{Line: p.cur().Line, Col: p.cur().Col},
			Op:          opStr,
			Ref:         ref,
		})
	}
	return node, nil
}

// ---------- func_stream_loop / func_call_final ----------

func (p *V13Parser) ParseFuncStreamLoop() (*V13FuncStreamLoopNode, error) {
	line, col := p.cur().Line, p.cur().Col

	var source V13Node
	if p.cur().Type == V13_TRUE {
		b, err := p.ParseBoolean()
		if err != nil {
			return nil, err
		}
		source = b
	} else if p.cur().Type == V13_IDENT {
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

	if _, err := p.expect(V13_STREAM); err != nil {
		return nil, err
	}

	var body V13Node
	var err error
	if p.cur().Type == V13_LBRACE {
		body, err = p.ParseFuncUnit()
	} else {
		body, err = p.ParseFuncCall()
	}
	if err != nil {
		return nil, err
	}
	return &V13FuncStreamLoopNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Source: source, Body: body}, nil
}

func (p *V13Parser) ParseFuncCallFinal() (*V13FuncCallFinalNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		if cc, err := p.ParseFuncCallChain(); err == nil {
			return &V13FuncCallFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: cc}, nil
		}
		p.restorePos(saved)
	}
	sl, err := p.ParseFuncStreamLoop()
	if err != nil {
		return nil, err
	}
	return &V13FuncCallFinalNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: sl}, nil
}

// ---------- func_inject ----------

func (p *V13Parser) ParseFuncInject() (*V13FuncInjectNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}

	node := &V13FuncInjectNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}

	if p.cur().Type != V13_RPAREN {
		node.Head = p.parseFuncInjectHead(line, col)
	}

	for p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		lhs, err := p.ParseAssignLHS()
		if err != nil {
			p.restorePos(saved)
			break
		}
		if p.cur().Type != V13_COLON && p.cur().Type != V13_READONLY {
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
		node.Binds = append(node.Binds, V13FuncInjectBind{LHS: lhs, Oper: oper, Ref: ref})
	}

	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *V13Parser) parseFuncInjectHead(line, col int) V13Node {
	saved := p.savePos()
	lhs, lhsErr := p.ParseAssignLHS()
	if lhsErr == nil {
		if p.cur().Type == V13_AT_IDENT || p.cur().Type == V13_ANY_TYPE {
			innerSaved := p.savePos()
			access, aErr := p.ParseInspectType()
			if aErr == nil {
				head := &V13FuncInjectHeadInspectNode{
					V13BaseNode: V13BaseNode{Line: line, Col: col},
					LHS:         lhs,
					Inspect:     access,
				}
				if p.cur().Type == V13_EMPTY_ARR {
					head.HasArray = true
					p.advance()
				}
				return head
			}
			p.restorePos(innerSaved)
		}
		if p.cur().Type == V13_COLON || p.cur().Type == V13_READONLY {
			oper, operErr := p.ParseAssignOper()
			if operErr == nil {
				ref, refErr := p.ParseIdentRef()
				if refErr == nil {
					return &V13FuncInjectBindNode{
						V13BaseNode: V13BaseNode{Line: line, Col: col},
						Bind:        V13FuncInjectBind{LHS: lhs, Oper: oper, Ref: ref},
					}
				}
			}
		}
	}
	p.restorePos(saved)
	ref, refErr := p.ParseIdentRef()
	if refErr == nil {
		return &V13FuncInjectBindNode{
			V13BaseNode: V13BaseNode{Line: line, Col: col},
			Bind:        V13FuncInjectBind{Ref: ref},
		}
	}
	p.restorePos(saved)
	return nil
}

// ---------- func_stmt ----------

// ParseFuncStmt parses:  func_stmt = regexp | ident_ref | object_final | array_final | func_call_chain | func_unit | calc_unit | self_ref | return_func_unit
func (p *V13Parser) ParseFuncStmt() (*V13FuncStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13FuncStmtNode {
		return &V13FuncStmtNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V13_RETURN_STMT:
		saved := p.savePos()
		p.advance()
		if p.cur().Type == V13_LBRACE || p.cur().Type == V13_LPAREN {
			unit, err := p.ParseFuncUnit()
			if err == nil {
				return wrap(&V13ReturnFuncUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Unit: unit}), nil
			}
		}
		p.restorePos(saved)

	case V13_DOLLAR:
		p.advance()
		return wrap(&V13SelfRefNode{V13BaseNode: V13BaseNode{Line: line, Col: col}}), nil

	case V13_LBRACE:
		unit, err := p.ParseFuncUnit()
		if err != nil {
			return nil, err
		}
		return wrap(unit), nil

	case V13_LBRACKET, V13_EMPTY_ARR, V13_UNIFORM:
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

	case V13_TYPE_OF:
		saved := p.savePos()
		if cc, err := p.ParseFuncCallChain(); err == nil {
			return wrap(cc), nil
		}
		p.restorePos(saved)
	}

	if p.cur().Type == V13_REGEXP || p.cur().Type == V13_REGEXP_DECL {
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return wrap(c), nil
	}

	cu, err := p.ParseCalcUnit()
	if err != nil {
		return nil, err
	}
	return wrap(cu), nil
}

// ---------- func_assign ----------

func (p *V13Parser) ParseFuncAssign() (*V13FuncAssignNode, error) {
	line, col := p.cur().Line, p.cur().Col
	var inj *V13FuncInjectNode
	if p.cur().Type == V13_LPAREN {
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
	return &V13FuncAssignNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Inject:      inj,
		LHS:         lhs,
		Oper:        oper,
		Stmt:        stmt,
	}, nil
}

// ---------- func body statements ----------

func (p *V13Parser) ParseFuncReturnStmt() (*V13FuncReturnStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_RETURN_STMT); err != nil {
		return nil, err
	}
	stmt, err := p.ParseFuncStmt()
	if err != nil {
		return nil, err
	}
	return &V13FuncReturnStmtNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Stmt: stmt}, nil
}

func (p *V13Parser) ParseFuncStoreStmt() (*V13FuncStoreStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_STORE); err != nil {
		return nil, err
	}
	parseItem := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			return p.parseTypeOfRef()
		}
		return p.ParseObjectFinal()
	}
	first, err := parseItem()
	if err != nil {
		return nil, err
	}
	node := &V13FuncStoreStmtNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Items: []V13Node{first}}
	for p.cur().Type == V13_COMMA {
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

func (p *V13Parser) parseCondReturnStmt() (*V13CondReturnStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_LPAREN); err != nil {
		return nil, err
	}
	cond, err := p.ParseLogicExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_RPAREN); err != nil {
		return nil, err
	}
	operTok := p.cur()
	switch operTok.Type {
	case V13_AMP, V13_PIPE, V13_CARET:
		p.advance()
	default:
		return nil, p.errAt(fmt.Sprintf("expected logic operator after condition, got %s %q", operTok.Type, operTok.Value))
	}
	ret, err := p.ParseFuncReturnStmt()
	if err != nil {
		return nil, err
	}
	return &V13CondReturnStmtNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Cond:        cond,
		Oper:        operTok.Value,
		Return:      ret,
	}, nil
}

// ParseFuncBodyStmt parses:  func_body_stmt
func (p *V13Parser) ParseFuncBodyStmt() (*V13FuncBodyStmtNode, error) {
	line, col := p.cur().Line, p.cur().Col
	wrap := func(v V13Node) *V13FuncBodyStmtNode {
		return &V13FuncBodyStmtNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: v}
	}

	switch p.cur().Type {
	case V13_RETURN_STMT:
		rs, err := p.ParseFuncReturnStmt()
		if err != nil {
			return nil, err
		}
		return wrap(rs), nil

	case V13_STORE:
		ss, err := p.ParseFuncStoreStmt()
		if err != nil {
			return nil, err
		}
		return wrap(ss), nil

	case V13_LPAREN:
		saved := p.savePos()
		if crs, err := p.parseCondReturnStmt(); err == nil {
			return wrap(crs), nil
		}
		p.restorePos(saved)
	}

	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		if cf, err := p.ParseFuncCallFinal(); err == nil {
			return wrap(cf), nil
		}
		p.restorePos(saved)
	}

	saved := p.savePos()
	if fa, err := p.ParseFuncAssign(); err == nil {
		return wrap(fa), nil
	}
	p.restorePos(saved)

	sl, err := p.ParseFuncStreamLoop()
	if err != nil {
		return nil, err
	}
	return wrap(sl), nil
}

// ---------- func_unit ----------

// V13isFuncBodyStart returns true for tokens that can only begin a func_body_stmt.
func V13isFuncBodyStart(tok V13Token) bool {
	switch tok.Type {
	case V13_RETURN_STMT, V13_STORE, V13_DOLLAR:
		return true
	}
	return false
}

// ParseFuncUnit parses:  func_unit = "{" func_unit_header body "}" | "(" func_unit_header body ")"
func (p *V13Parser) ParseFuncUnit() (*V13FuncUnitNode, error) {
	line, col := p.cur().Line, p.cur().Col

	useGroup := false
	switch p.cur().Type {
	case V13_LBRACE:
		p.advance()
	case V13_LPAREN:
		useGroup = true
		p.advance()
	default:
		return nil, p.errAt(fmt.Sprintf("expected '{' or '(' to begin func_unit, got %s", p.cur().Type))
	}

	hdr := &V13FuncUnitHeaderNode{V13BaseNode: V13BaseNode{Line: p.cur().Line, Col: p.cur().Col}}
	if !V13isFuncBodyStart(p.cur()) {
		saved := p.savePos()
		if up, err := p.parseFuncHeaderUserParams(); err == nil {
			hdr.UserParams = up
		} else {
			p.restorePos(saved)
		}
		argsDecl, _ := p.ParseFuncArgsDecl()
		hdr.ArgsDecl = argsDecl
	}
	p.V13skipNLs()

	first, err := p.ParseFuncBodyStmt()
	if err != nil {
		return nil, err
	}
	body := []*V13FuncBodyStmtNode{first}

	closeType := V13_RBRACE
	if useGroup {
		closeType = V13_RPAREN
	}
	for {
		p.V13skipNLs()
		if p.cur().Type == closeType || p.cur().Type == V13_EOF {
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
	return &V13FuncUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Header: hdr, Body: body, UseGroupDelim: useGroup}, nil
}

// ---------- function-related updates ----------

// ParseReturnFuncUnit parses:  return_func_unit = "<-" func_unit
func (p *V13Parser) ParseReturnFuncUnit() (*V13ReturnFuncUnitNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_RETURN_STMT); err != nil {
		return nil, err
	}
	unit, err := p.ParseFuncUnit()
	if err != nil {
		return nil, err
	}
	return &V13ReturnFuncUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Unit: unit}, nil
}

func (p *V13Parser) ParseUpdateFuncUnit() (*V13UpdateFuncUnitNode, error) {
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
	return &V13UpdateFuncUnitNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Ref: ref, Assign: oper, NewUnit: rfu}, nil
}

// ---------- array_idx_recursive ----------

func (p *V13Parser) ParseArrayIdxRecursive() (*V13ArrayIdxRecursiveNode, error) {
	line, col := p.cur().Line, p.cur().Col
	parseNumRHS := func() (V13Node, error) {
		if p.cur().Type == V13_TYPE_OF {
			return p.parseTypeOfRef()
		}
		return p.ParseNumericConst()
	}
	if _, err := p.expect(V13_LBRACKET); err != nil {
		return nil, err
	}
	first, err := parseNumRHS()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_RBRACKET); err != nil {
		return nil, err
	}
	node := &V13ArrayIdxRecursiveNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Indices: []V13Node{first}}
	for p.cur().Type == V13_LBRACKET {
		saved := p.savePos()
		p.advance()
		idx, err := parseNumRHS()
		if err != nil {
			p.restorePos(saved)
			break
		}
		if _, err2 := p.expect(V13_RBRACKET); err2 != nil {
			p.restorePos(saved)
			break
		}
		node.Indices = append(node.Indices, idx)
	}
	return node, nil
}

// ---------- update_number / update_string ----------

func (p *V13Parser) ParseUpdateNumber() (*V13UpdateNumberNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseAssignOper()
	if err != nil {
		return nil, err
	}
	var rhs V13Node
	if p.cur().Type == V13_TYPE_OF {
		rhs, err = p.parseTypeOfRef()
	} else {
		rhs, err = p.ParseNumericConst()
	}
	if err != nil {
		return nil, err
	}
	return &V13UpdateNumberNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Target: ref, Oper: oper, RHS: rhs}, nil
}

// ParseStringUpdateOper parses:  string_update_oper = "+:" | "+=" | "=" | ":" | ":~"
// V13 change: "+=" replaces V12's "+~" for mutable string append.
func (p *V13Parser) ParseStringUpdateOper() (*V13StringUpdateOperNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	var kind V13StringUpdateOperKind
	var tv string
	switch tok.Type {
	case V13_IADD_IMM:
		kind, tv = V13StringAppendImmutable, "+:"
	case V13_IADD_MUT:
		// V13: "+=" is the mutable string append operator (not "+~" as in V12)
		kind, tv = V13StringAppendMutable, "+="
	case V13_EQ, V13_COLON, V13_READONLY:
		kind, tv = V13StringEqualAssign, tok.Value
	default:
		return nil, p.errAt(fmt.Sprintf("expected string update operator (+:, +=, =, :, :~), got %s %q", tok.Type, tok.Value))
	}
	p.advance()
	return &V13StringUpdateOperNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Kind: kind, Token: tv}, nil
}

func (p *V13Parser) ParseUpdateString() (*V13UpdateStringNode, error) {
	line, col := p.cur().Line, p.cur().Col
	ref, err := p.parseTypeOfRef()
	if err != nil {
		return nil, err
	}
	oper, err := p.ParseStringUpdateOper()
	if err != nil {
		return nil, err
	}
	var rhs V13Node
	if p.cur().Type == V13_TYPE_OF {
		rhs, err = p.parseTypeOfRef()
	} else {
		rhs, err = p.ParseString()
	}
	if err != nil {
		return nil, err
	}
	return &V13UpdateStringNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Target: ref, Oper: oper, RHS: rhs}, nil
}

// ---------- ident_ref_update ----------

func (p *V13Parser) ParseIdentRefUpdate() (*V13IdentRefUpdateNode, error) {
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
	return &V13IdentRefUpdateNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Ref: ref, Oper: oper, RHS: rhs}, nil
}

// ParseHTTPURL parses an http_url (quoted STRING token with http/https scheme).
func (p *V13Parser) ParseHTTPURL() (*V13URLNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_STRING {
		return nil, p.errAt(fmt.Sprintf("expected quoted http_url, got %s %q", tok.Type, tok.Value))
	}
	raw := tok.Value
	if len(raw) >= 2 {
		raw = raw[1 : len(raw)-1]
	}
	p.advance()
	if !V13urlRe.MatchString(raw) {
		return nil, &V13ParseError{Line: line, Col: col, Message: fmt.Sprintf("invalid http_url: %q", raw)}
	}
	return &V13URLNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: raw}, nil
}

// ParseFilePath parses a file_path (any quoted STRING).
func (p *V13Parser) ParseFilePath() (*V13FilePathNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_STRING {
		return nil, p.errAt(fmt.Sprintf("expected quoted file_path, got %s %q", tok.Type, tok.Value))
	}
	raw := tok.Value
	if len(raw) >= 2 {
		raw = raw[1 : len(raw)-1]
	}
	p.advance()
	return &V13FilePathNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: raw}, nil
}

var _ = fmt.Sprintf
