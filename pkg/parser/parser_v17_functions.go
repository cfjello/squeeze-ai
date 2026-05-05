// parser_v17_functions.go — Parse methods for spec/06_functions.sqg and
// MERGE contributions from spec/12_iterators.sqg Section 12.6
// and spec/13_push_pull.sqg Section 13.6.
//
// Covered rules (spec/06_functions.sqg):
//
//	into_arrow, return_arrow, deps_oper, iterator_oper, push_oper,
//	type_ref, inspect_type, decl_types,
//	args_single_decl, args_decl, func_args, func_deps, func_args_decl,
//	func_call_args, func_call, func_call_chain, func_call_final,
//	iterator_loop, push_loop,
//	func_return_stmt, func_store_stmt, func_body_stmt,
//	func_unit, return_func_unit
//
// MERGE contributions (spec/12_iterators.sqg §12.6):
//
//	iterator_recv_decl  → merged into func_args_decl
//	iterator_yield_stmt → merged into func_body_stmt
//	assign_iterator     → wired into ParseAssignRhs (parser_v17_assignment.go)
//
// MERGE contributions (spec/13_push_pull.sqg §13.6):
//
//	push_recv_decl    → merged into func_args_decl
//	push_forward_stmt → merged into func_body_stmt
//	push_stream_bind  → merged into func_body_stmt
//	assign_push       → wired into ParseAssignRhs (parser_v17_assignment.go)
//
// EXTEND<statement> = | return_func_unit | func_call_final
//
//	(branches added to ParseStatement in parser_v17_operators.go)
package parser

import (
	"fmt"
	"os"
)

// =============================================================================
// OPERATOR TERMINALS
// into_arrow    = "->"  (V17_MINUS + V17_GT — parse-time compound, SIR-4)
// return_arrow  = "<-"  (V17_LT   + V17_MINUS — parse-time compound, SIR-4)
// deps_oper     = "=>"  (V17_EQ   + V17_GT   — parse-time compound, SIR-4)
// iterator_oper = ">>"  (V17_GTGT — pre-lexed because it already existed)
// push_oper     = "~>"  (V17_TILDE_GT — pre-lexed because ~ alone is ILLEGAL)
// =============================================================================

// V17IntoArrowNode  into_arrow = "->"
type V17IntoArrowNode struct{ V17BaseNode }

// ParseIntoArrow parses into_arrow = "->".
// Represented as two consecutive tokens V17_MINUS + V17_GT (SIR-4).
func (p *V17Parser) ParseIntoArrow() (node *V17IntoArrowNode, err error) {
	done := p.debugEnter("into_arrow")
	defer func() { done(err == nil) }()
	if !p.peekLit("->") {
		return nil, p.errAt("into_arrow: expected '->'")
	}
	line, col := p.runeLine, p.runeCol
	if _, err = p.matchLit("->"); err != nil {
		return nil, err
	}
	return &V17IntoArrowNode{V17BaseNode{line, col}}, nil
}

// V17ReturnArrowNode  return_arrow = "<-"
type V17ReturnArrowNode struct{ V17BaseNode }

// ParseReturnArrow parses return_arrow = "<-".
// Represented as two consecutive tokens V17_LT + V17_MINUS (SIR-4).
func (p *V17Parser) ParseReturnArrow() (node *V17ReturnArrowNode, err error) {
	done := p.debugEnter("return_arrow")
	defer func() { done(err == nil) }()
	if !p.peekLit("<-") {
		return nil, p.errAt("return_arrow: expected '<-'")
	}
	line, col := p.runeLine, p.runeCol
	if _, err = p.matchLit("<-"); err != nil {
		return nil, err
	}
	return &V17ReturnArrowNode{V17BaseNode{line, col}}, nil
}

// V17DepsOperNode  deps_oper = "=>"
type V17DepsOperNode struct{ V17BaseNode }

// ParseDepsOper parses deps_oper = "=>".
// Represented as two consecutive tokens V17_EQ + V17_GT (SIR-4).
func (p *V17Parser) ParseDepsOper() (node *V17DepsOperNode, err error) {
	done := p.debugEnter("deps_oper")
	defer func() { done(err == nil) }()
	if !p.peekLit("=>") {
		return nil, p.errAt("deps_oper: expected '=>'")
	}
	line, col := p.runeLine, p.runeCol
	if _, err = p.matchLit("=>"); err != nil {
		return nil, err
	}
	return &V17DepsOperNode{V17BaseNode{line, col}}, nil
}

// V17IteratorOperNode  iterator_oper = ">>"
type V17IteratorOperNode struct{ V17BaseNode }

// ParseIteratorOper parses iterator_oper = ">>".
func (p *V17Parser) ParseIteratorOper() (node *V17IteratorOperNode, err error) {
	done := p.debugEnter("iterator_oper")
	defer func() { done(err == nil) }()
	if !p.peekLit(">>") {
		return nil, p.errAt("iterator_oper: expected '>>'")
	}
	line, col := p.runeLine, p.runeCol
	if _, err = p.matchLit(">>"); err != nil {
		return nil, err
	}
	return &V17IteratorOperNode{V17BaseNode{line, col}}, nil
}

// V17PushOperNode  push_oper = "~>"
type V17PushOperNode struct{ V17BaseNode }

// ParsePushOper parses push_oper = "~>".
func (p *V17Parser) ParsePushOper() (node *V17PushOperNode, err error) {
	done := p.debugEnter("push_oper")
	defer func() { done(err == nil) }()
	if !p.peekLit("~>") {
		return nil, p.errAt("push_oper: expected '~>'")
	}
	line, col := p.runeLine, p.runeCol
	if _, err = p.matchLit("~>"); err != nil {
		return nil, err
	}
	return &V17PushOperNode{V17BaseNode{line, col}}, nil
}

// =============================================================================
// TYPE REFERENCES
// reflect_prefix = "@"
// any_type       = "@?"  (V17_ANY_TYPE)
// type_ref       = ( ident_ref !WS! "." !WS! reflect_prefix !WS! "type" ) | any_type
// inspect_type   = type_ref
// decl_types     = type_ref
// =============================================================================

// V17TypeRefNode  type_ref = ( ident_ref !WS! "." !WS! reflect_prefix !WS! "type" ) | any_type
type V17TypeRefNode struct {
	V17BaseNode
	IsAnyType bool             // true when "@?" was parsed
	TypeName  *V17IdentRefNode // nil when IsAnyType; the ident_ref before ".@type" otherwise
}

// ParseTypeRef parses type_ref = ( ident_ref !WS! ".@type" ) | "@?".
// !WS! is enforced by requiring ".@type" to be immediately adjacent to the ident_ref
// (no whitespace between ident_ref and the dot).
func (p *V17Parser) ParseTypeRef() (node *V17TypeRefNode, err error) {
	done := p.debugEnter("type_ref")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// "@?" — any_type (two-char compound "@?")
	if p.peekLit("@?") {
		if _, err = p.matchLit("@?"); err != nil {
			return nil, err
		}
		return &V17TypeRefNode{V17BaseNode{line, col}, true, nil}, nil
	}

	// ident_ref !WS! "." !WS! "@" !WS! "type"
	saved := p.savePos()
	ref, rerr := p.ParseIdentRef()
	if rerr != nil {
		p.restorePos(saved)
		return nil, p.errAt("type_ref: expected ident_ref.@type or @?")
	}
	// !WS! — dot must be immediately adjacent (no whitespace after ident_ref)
	if p.runePos >= len(p.input) || p.input[p.runePos] != '.' {
		p.restorePos(saved)
		return nil, p.errAt("type_ref: expected '.@type' immediately after ident_ref")
	}
	if _, err = p.matchLit("."); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	// !WS! — "@" must follow immediately
	if p.runePos >= len(p.input) || p.input[p.runePos] != '@' {
		p.restorePos(saved)
		return nil, p.errAt("type_ref: expected '@' immediately after '.'")
	}
	if _, err = p.matchLit("@"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	// !WS! — "type" must follow immediately
	if p.runePos >= len(p.input) || p.input[p.runePos] == ' ' || p.input[p.runePos] == '\t' {
		p.restorePos(saved)
		return nil, p.errAt("type_ref: expected 'type' immediately after '@'")
	}
	if _, err = p.matchKeyword("type"); err != nil {
		p.restorePos(saved)
		return nil, fmt.Errorf("type_ref: expected 'type' after '.@': %w", err)
	}
	return &V17TypeRefNode{V17BaseNode{line, col}, false, ref}, nil
}

// V17InspectTypeNode  inspect_type = type_ref
type V17InspectTypeNode struct {
	V17BaseNode
	TypeRef *V17TypeRefNode
}

// ParseInspectType parses inspect_type = type_ref.
func (p *V17Parser) ParseInspectType() (node *V17InspectTypeNode, err error) {
	done := p.debugEnter("inspect_type")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	tr, trerr := p.ParseTypeRef()
	if trerr != nil {
		return nil, fmt.Errorf("inspect_type: %w", trerr)
	}
	return &V17InspectTypeNode{V17BaseNode{line, col}, tr}, nil
}

// V17DeclTypesNode  decl_types = type_ref
type V17DeclTypesNode struct {
	V17BaseNode
	TypeRef *V17TypeRefNode
}

// ParseDeclTypes parses decl_types = type_ref.
func (p *V17Parser) ParseDeclTypes() (node *V17DeclTypesNode, err error) {
	done := p.debugEnter("decl_types")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	tr, trerr := p.ParseTypeRef()
	if trerr != nil {
		return nil, fmt.Errorf("decl_types: %w", trerr)
	}
	return &V17DeclTypesNode{V17BaseNode{line, col}, tr}, nil
}

// =============================================================================
// ARGUMENT DECLARATIONS
// args_single_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] decl_types
// args_decl        = args_single_decl { "," args_single_decl }
// func_args        = into_arrow args_decl
// func_deps        = deps_oper UNIQUE< ident_ref { "," ident_ref } >
// =============================================================================

// V17ArgsSingleDeclNode  args_single_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] ( decl_types | ps_token_ref )
type V17ArgsSingleDeclNode struct {
	V17BaseNode
	LHS      *V17AssignLhsNode
	Modifier interface{} // nil | *V17AssignImmutableNode | *V17AssignReadOnlyRefNode
	// Type is one of: *V17DeclTypesNode | *V17PsTokenRefNode
	Type interface{}
}

// ParseArgsSingleDecl parses args_single_decl = assign_lhs [ assign_immutable | assign_read_only_ref ] decl_types.
func (p *V17Parser) ParseArgsSingleDecl() (node *V17ArgsSingleDeclNode, err error) {
	done := p.debugEnter("args_single_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	lhs, lerr := p.ParseAssignLhs()
	if lerr != nil {
		return nil, fmt.Errorf("args_single_decl: %w", lerr)
	}

	// Optional: assign_immutable (":") | assign_read_only_ref (":~")
	var modifier interface{}
	if p.peekLit(":~") {
		if ror, rorerr := p.ParseAssignReadOnlyRef(); rorerr == nil {
			modifier = ror
		}
	} else if p.peekAfterWS() == ':' {
		if ai, aierr := p.ParseAssignImmutable(); aierr == nil {
			modifier = ai
		}
	}

	dt, dterr := p.ParseDeclTypes()
	if dterr == nil {
		return &V17ArgsSingleDeclNode{V17BaseNode{line, col}, lhs, modifier, dt}, nil
	}
	// Bootstrap extension: "§ident" as a token-reference type annotation (__ps_token_ref).
	if p.peekLit("§") {
		tr, trerr := p.ParsePsTokenRef()
		if trerr != nil {
			return nil, fmt.Errorf("args_single_decl: expected decl_types or ps_token_ref: %w", trerr)
		}
		return &V17ArgsSingleDeclNode{V17BaseNode{line, col}, lhs, modifier, tr}, nil
	}
	return nil, fmt.Errorf("args_single_decl: expected decl_types or ps_token_ref: %w", dterr)
}

// V17ArgsDeclNode  args_decl = args_single_decl { "," args_single_decl }
type V17ArgsDeclNode struct {
	V17BaseNode
	Items []*V17ArgsSingleDeclNode
}

// ParseArgsDecl parses args_decl = args_single_decl { "," args_single_decl }.
func (p *V17Parser) ParseArgsDecl() (node *V17ArgsDeclNode, err error) {
	done := p.debugEnter("args_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseArgsSingleDecl()
	if ferr != nil {
		return nil, fmt.Errorf("args_decl: %w", ferr)
	}
	items := []*V17ArgsSingleDeclNode{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.ParseArgsSingleDecl()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	return &V17ArgsDeclNode{V17BaseNode{line, col}, items}, nil
}

// V17FuncArgsNode  func_args = into_arrow args_decl
type V17FuncArgsNode struct {
	V17BaseNode
	Arrow *V17IntoArrowNode
	Args  *V17ArgsDeclNode
}

// ParseFuncArgs parses func_args = into_arrow args_decl.
func (p *V17Parser) ParseFuncArgs() (node *V17FuncArgsNode, err error) {
	done := p.debugEnter("func_args")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	arrow, arrerr := p.ParseIntoArrow()
	if arrerr != nil {
		return nil, fmt.Errorf("func_args: %w", arrerr)
	}
	args, argserr := p.ParseArgsDecl()
	if argserr != nil {
		return nil, fmt.Errorf("func_args: %w", argserr)
	}
	return &V17FuncArgsNode{V17BaseNode{line, col}, arrow, args}, nil
}

// V17FuncDepsNode  func_deps = deps_oper UNIQUE< ident_ref { "," ident_ref } >
// UNIQUE is a checker-time directive; the parser collects the list and records it.
type V17FuncDepsNode struct {
	V17BaseNode
	Deps []*V17IdentRefNode
}

// ParseFuncDeps parses func_deps = deps_oper UNIQUE< ident_ref { "," ident_ref } >.
// Duplicate detection is deferred to the checker (UNIQUE directive, per SIR-3).
func (p *V17Parser) ParseFuncDeps() (node *V17FuncDepsNode, err error) {
	done := p.debugEnter("func_deps")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, derr := p.ParseDepsOper(); derr != nil {
		return nil, fmt.Errorf("func_deps: %w", derr)
	}

	first, ferr := p.ParseIdentRef()
	if ferr != nil {
		return nil, fmt.Errorf("func_deps: expected ident_ref after '=>': %w", ferr)
	}
	deps := []*V17IdentRefNode{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.ParseIdentRef()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		deps = append(deps, next)
	}

	return &V17FuncDepsNode{V17BaseNode{line, col}, deps}, nil
}

// =============================================================================
// ITERATOR RECV DECL  (MERGE from spec/12_iterators.sqg §12.6)
// iterator_recv_decl = iterator_oper ident_name ":" inspect_type
// =============================================================================

// V17IteratorRecvDeclNode  iterator_recv_decl = iterator_oper ident_name ":" inspect_type
type V17IteratorRecvDeclNode struct {
	V17BaseNode
	Name *V17IdentNameNode
	Type *V17InspectTypeNode
}

// ParseIteratorRecvDecl parses iterator_recv_decl = ">>" ident_name ":" inspect_type.
// This implements Role A' (raw iterator argument in func_unit header) from
// spec/12_iterators.sqg §12.1.
func (p *V17Parser) ParseIteratorRecvDecl() (node *V17IteratorRecvDeclNode, err error) {
	done := p.debugEnter("iterator_recv_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, ierr := p.ParseIteratorOper(); ierr != nil {
		return nil, fmt.Errorf("iterator_recv_decl: %w", ierr)
	}
	name, nerr := p.ParseIdentName()
	if nerr != nil {
		return nil, fmt.Errorf("iterator_recv_decl: expected ident_name after '>>': %w", nerr)
	}
	if _, cerr := p.matchLit(":"); cerr != nil {
		return nil, fmt.Errorf("iterator_recv_decl: expected ':' after name: %w", cerr)
	}
	it, iterr := p.ParseInspectType()
	if iterr != nil {
		return nil, fmt.Errorf("iterator_recv_decl: expected inspect_type: %w", iterr)
	}
	return &V17IteratorRecvDeclNode{V17BaseNode{line, col}, name, it}, nil
}

// =============================================================================
// PUSH RECV DECL  (MERGE from spec/13_push_pull.sqg §13.6 — Role A)
// push_recv_decl = push_oper ident_name ":" inspect_type
// =============================================================================

// V17PushRecvDeclNode  push_recv_decl = push_oper ident_name ":" inspect_type
type V17PushRecvDeclNode struct {
	V17BaseNode
	Name *V17IdentNameNode
	Type *V17InspectTypeNode
}

// ParsePushRecvDecl parses push_recv_decl = "~>" ident_name ":" inspect_type.
// This implements Role A (raw push stream argument in func_unit header) from
// spec/13_push_pull.sqg §13.1.2.
func (p *V17Parser) ParsePushRecvDecl() (node *V17PushRecvDeclNode, err error) {
	done := p.debugEnter("push_recv_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, perr := p.ParsePushOper(); perr != nil {
		return nil, fmt.Errorf("push_recv_decl: %w", perr)
	}
	name, nerr := p.ParseIdentName()
	if nerr != nil {
		return nil, fmt.Errorf("push_recv_decl: expected ident_name after '~>': %w", nerr)
	}
	if _, cerr := p.matchLit(":"); cerr != nil {
		return nil, fmt.Errorf("push_recv_decl: expected ':' after name: %w", cerr)
	}
	it, iterr := p.ParseInspectType()
	if iterr != nil {
		return nil, fmt.Errorf("push_recv_decl: expected inspect_type: %w", iterr)
	}
	return &V17PushRecvDeclNode{V17BaseNode{line, col}, name, it}, nil
}

// =============================================================================
// FUNC ARGS DECL  (with MERGE from spec/12_iterators.sqg §12.6 and spec/13_push_pull.sqg §13.6)
// func_args_decl = [ func_args ] [ func_deps ]
// MERGE<func_args_decl> = | iterator_recv_decl  (12_iterators.sqg)
//                        | push_recv_decl        (13_push_pull.sqg)
//
// Combined rule:
//   func_args_decl = iterator_recv_decl
//                  | push_recv_decl
//                  | ( [ func_args ] [ func_deps ] )
// =============================================================================

// V17FuncArgsDeclNode  func_args_decl (combined with MERGE)
type V17FuncArgsDeclNode struct {
	V17BaseNode
	// Exactly one of the following patterns is non-nil:
	// 1. IterRecvDecl is non-nil — Role A' raw iterator form
	// 2. PushRecvDecl is non-nil — Role A raw push stream form
	// 3. FuncArgs and/or FuncDeps (base form, both may be nil for empty header)
	IterRecvDecl *V17IteratorRecvDeclNode
	PushRecvDecl *V17PushRecvDeclNode
	FuncArgs     *V17FuncArgsNode
	FuncDeps     *V17FuncDepsNode
}

// ParseFuncArgsDecl parses func_args_decl (MERGE included).
// Disambiguation:
//
//	">>" + ident + ":" → iterator_recv_decl (Role A')
//	"~>" + ident + ":" → push_recv_decl (Role A)
//	"->"               → func_args (value form)
//	"=>"               → func_deps only
//	other              → empty header
func (p *V17Parser) ParseFuncArgsDecl() (node *V17FuncArgsDeclNode, err error) {
	done := p.debugEnter("func_args_decl")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// MERGE: iterator_recv_decl (">>" + ident + ":") — Role A'
	if p.peekLit(">>") {
		saved := p.savePos()
		if recv, rerr := p.ParseIteratorRecvDecl(); rerr == nil {
			return &V17FuncArgsDeclNode{V17BaseNode{line, col}, recv, nil, nil, nil}, nil
		}
		p.restorePos(saved)
	}

	// MERGE: push_recv_decl ("~>" + ident + ":") — Role A
	if p.peekLit("~>") {
		saved := p.savePos()
		if recv, rerr := p.ParsePushRecvDecl(); rerr == nil {
			return &V17FuncArgsDeclNode{V17BaseNode{line, col}, nil, recv, nil, nil}, nil
		}
		p.restorePos(saved)
	}

	// Base: [ func_args ] [ func_deps ]
	var funcArgs *V17FuncArgsNode
	var funcDeps *V17FuncDepsNode

	// Optional func_args: starts with "->"
	if p.peekLit("->") {
		if fa, faerr := p.ParseFuncArgs(); faerr == nil {
			funcArgs = fa
		}
	}

	// Optional func_deps: starts with "=>"
	if p.peekLit("=>") {
		if fd, fderr := p.ParseFuncDeps(); fderr == nil {
			funcDeps = fd
		}
	}

	return &V17FuncArgsDeclNode{V17BaseNode{line, col}, nil, nil, funcArgs, funcDeps}, nil
}

// =============================================================================
// FUNCTION CALLS
// func_call_args   = statement { "," statement }
// func_call        = TYPE_OF func_unit<ident_ref> [ func_call_args ]
//                    (TYPE_OF is a checker directive; parser records ident_ref)
// func_call_chain  = func_call [ ( into_arrow | iterator_oper | push_oper )
//                                 TYPE_OF func_unit<ident_ref>
//                               { ( into_arrow | iterator_oper | push_oper )
//                                 TYPE_OF func_unit<ident_ref> } ]
// func_call_final  = func_call_chain
// =============================================================================

// V17FuncCallArgsNode  func_call_args = statement { "," statement }
type V17FuncCallArgsNode struct {
	V17BaseNode
	Items []*V17StatementNode
}

// ParseFuncCallArgs parses func_call_args = statement { "," statement }.
func (p *V17Parser) ParseFuncCallArgs() (node *V17FuncCallArgsNode, err error) {
	done := p.debugEnter("func_call_args")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseStatement()
	if ferr != nil {
		return nil, fmt.Errorf("func_call_args: %w", ferr)
	}
	items := []*V17StatementNode{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.ParseStatement()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	return &V17FuncCallArgsNode{V17BaseNode{line, col}, items}, nil
}

// V17FuncCallNode  func_call = TYPE_OF func_unit<ident_ref> [ func_call_args ]
type V17FuncCallNode struct {
	V17BaseNode
	Func *V17IdentRefNode     // TYPE_OF func_unit<ident_ref> — type check deferred to checker
	Args *V17FuncCallArgsNode // nil when no arguments provided
}

// ParseFuncCall parses func_call = TYPE_OF func_unit<ident_ref> [ func_call_args ].
// TYPE_OF is a checker directive; the parser records the ident_ref and optional args.
func (p *V17Parser) ParseFuncCall() (node *V17FuncCallNode, err error) {
	done := p.debugEnter("func_call")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	ref, rerr := p.ParseIdentRef()
	if rerr != nil {
		return nil, fmt.Errorf("func_call: %w", rerr)
	}

	// Optional: func_call_args
	var args *V17FuncCallArgsNode
	ch := p.peekAfterWS()
	switch {
	case ch == 0: // EOF
		// no args
	case ch == '\n' || ch == ')' || ch == '}' || ch == ']' || ch == ',':
		// no args
	case p.peekLit(">>") || p.peekLit("~>"):
		// chain op, not an arg
	case p.peekLit("->"):
		// chain op, not an arg
	case p.peekLit("=>"):
		// deps_oper, not an arg
	default:
		saved := p.savePos()
		if fa, faerr := p.ParseFuncCallArgs(); faerr == nil {
			args = fa
		} else {
			p.restorePos(saved)
		}
	}

	return &V17FuncCallNode{V17BaseNode{line, col}, ref, args}, nil
}

// V17FuncCallChainSegment is one chain link after the initial func_call.
type V17FuncCallChainSegment struct {
	Op   string           // "->" | ">>" | "~>"
	Func *V17IdentRefNode // TYPE_OF func_unit<ident_ref>
}

// V17FuncCallChainNode  func_call_chain = func_call [ arrow func_ref { arrow func_ref } ]
type V17FuncCallChainNode struct {
	V17BaseNode
	First    *V17FuncCallNode
	Segments []V17FuncCallChainSegment
}

// ParseFuncCallChain parses func_call_chain = func_call [ (-> | >> | ~>) ident_ref ... ].
func (p *V17Parser) ParseFuncCallChain() (node *V17FuncCallChainNode, err error) {
	done := p.debugEnter("func_call_chain")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	first, ferr := p.ParseFuncCall()
	if ferr != nil {
		return nil, fmt.Errorf("func_call_chain: %w", ferr)
	}

	var segments []V17FuncCallChainSegment
	for {
		saved := p.savePos()
		var opStr string

		switch {
		case p.peekLit("->"):
			if _, err = p.matchLit("->"); err == nil {
				opStr = "->"
			}
		case p.peekLit(">>"):
			if _, err = p.matchLit(">>"); err == nil {
				opStr = ">>"
			}
		case p.peekLit("~>"):
			if _, err = p.matchLit("~>"); err == nil {
				opStr = "~>"
			}
		}

		if opStr == "" {
			p.restorePos(saved)
			break
		}

		// TYPE_OF func_unit<ident_ref> — parse the chained function reference
		nextFunc, nerr := p.ParseIdentRef()
		if nerr != nil {
			p.restorePos(saved) // undo the consumed arrow
			break
		}
		segments = append(segments, V17FuncCallChainSegment{opStr, nextFunc})
	}

	return &V17FuncCallChainNode{V17BaseNode{line, col}, first, segments}, nil
}

// V17FuncCallFinalNode  func_call_final = func_call_chain
type V17FuncCallFinalNode struct {
	V17BaseNode
	Chain *V17FuncCallChainNode
}

// ParseFuncCallFinal parses func_call_final = func_call_chain.
// Used directly in contexts that explicitly expect a func_call_final
// (e.g. iterator_loop, push_loop).  Always succeeds when func_call succeeds.
func (p *V17Parser) ParseFuncCallFinal() (node *V17FuncCallFinalNode, err error) {
	done := p.debugEnter("func_call_final")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	chain, cerr := p.ParseFuncCallChain()
	if cerr != nil {
		return nil, fmt.Errorf("func_call_final: %w", cerr)
	}
	return &V17FuncCallFinalNode{V17BaseNode{line, col}, chain}, nil
}

// parseFuncCallFinalStrict is used by ParseStatement EXTEND only.
// It only succeeds when the chain has at least one chain segment (-> >> ~>)
// OR the first func_call has explicit arguments.
// A bare ident_ref without chain or args is already handled by other
// ParseStatement alternatives (numeric_calc → single_num_expr → ident_ref).
func (p *V17Parser) parseFuncCallFinalStrict() (node *V17FuncCallFinalNode, err error) {
	saved := p.savePos()
	fcf, ferr := p.ParseFuncCallFinal()
	if ferr != nil {
		return nil, ferr
	}
	// Require chain OR explicit args — otherwise this is just a bare ident_ref
	hasChain := len(fcf.Chain.Segments) > 0
	hasArgs := fcf.Chain.First.Args != nil && len(fcf.Chain.First.Args.Items) > 0
	if !hasChain && !hasArgs {
		p.restorePos(saved)
		return nil, p.errAt("func_call_final: bare ident_ref — handled by other statement rules")
	}
	return fcf, nil
}

// =============================================================================
// LOOP CONSTRUCTS
// iterator_loop = collection iterator_oper ( func_unit | func_call )
// push_loop     = collection push_oper     ( func_unit | func_call )
// =============================================================================

// V17IteratorLoopNode  iterator_loop = collection ">>" ( func_unit | func_call | statement )
type V17IteratorLoopNode struct {
	V17BaseNode
	Collection *V17CollectionNode
	// Body is one of: *V17FuncUnitNode | *V17FuncCallNode | *V17StatementNode
	Body interface{}
}

// ParseIteratorLoop parses iterator_loop = collection iterator_oper ( func_unit | func_call | statement ).
func (p *V17Parser) ParseIteratorLoop() (node *V17IteratorLoopNode, err error) {
	done := p.debugEnter("iterator_loop")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	coll, collerr := p.ParseCollection()
	if collerr != nil {
		return nil, fmt.Errorf("iterator_loop: %w", collerr)
	}
	if _, oerr := p.ParseIteratorOper(); oerr != nil {
		return nil, fmt.Errorf("iterator_loop: %w", oerr)
	}

	// func_unit first (starts with "(")
	if p.peekAfterWS() == '(' {
		if saved := p.savePos(); true {
			if fu, fuerr := p.ParseFuncUnit(); fuerr == nil {
				return &V17IteratorLoopNode{V17BaseNode{line, col}, coll, fu}, nil
			}
			p.restorePos(saved)
		}
	}

	// statement — tried before func_call because func_call accepts a bare ident_ref
	// and would succeed on e.g. "cnt" in "cnt++" leaving "++" unconsumed.
	// statement subsumes func_call (func_call_final + numeric_calc cover all func_call forms).
	if saved := p.savePos(); true {
		if st, sterr := p.ParseStatement(); sterr == nil {
			return &V17IteratorLoopNode{V17BaseNode{line, col}, coll, st}, nil
		}
		p.restorePos(saved)
	}

	// func_call — fallback for any case statement couldn't handle
	fc, fcerr := p.ParseFuncCall()
	if fcerr != nil {
		return nil, fmt.Errorf("iterator_loop: expected func_unit, func_call, or statement after '>>': %w", fcerr)
	}
	return &V17IteratorLoopNode{V17BaseNode{line, col}, coll, fc}, nil
}

// V17PushLoopNode  push_loop = collection push_oper ( func_unit | func_call )
type V17PushLoopNode struct {
	V17BaseNode
	Collection *V17CollectionNode
	// Body is one of: *V17FuncUnitNode | *V17FuncCallNode
	Body interface{}
}

// ParsePushLoop parses push_loop = collection push_oper ( func_unit | func_call ).
func (p *V17Parser) ParsePushLoop() (node *V17PushLoopNode, err error) {
	done := p.debugEnter("push_loop")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	coll, collerr := p.ParseCollection()
	if collerr != nil {
		return nil, fmt.Errorf("push_loop: %w", collerr)
	}
	if _, oerr := p.ParsePushOper(); oerr != nil {
		return nil, fmt.Errorf("push_loop: %w", oerr)
	}

	// func_unit first (starts with "(")
	if p.peekAfterWS() == '(' {
		if saved := p.savePos(); true {
			if fu, fuerr := p.ParseFuncUnit(); fuerr == nil {
				return &V17PushLoopNode{V17BaseNode{line, col}, coll, fu}, nil
			}
			p.restorePos(saved)
		}
	}

	// func_call — ident_ref
	fc, fcerr := p.ParseFuncCall()
	if fcerr != nil {
		return nil, fmt.Errorf("push_loop: expected func_unit or func_call after '~>': %w", fcerr)
	}
	return &V17PushLoopNode{V17BaseNode{line, col}, coll, fc}, nil
}

// =============================================================================
// FUNCTION STATEMENTS
// func_return_stmt = return_arrow statement
// func_store_stmt  = deps_oper ( object_final | TYPE_OF object_final<ident_ref> )
//                    { "," ( object_final | TYPE_OF object_final<ident_ref> ) }
// =============================================================================

// V17FuncReturnStmtNode  func_return_stmt = return_arrow statement
type V17FuncReturnStmtNode struct {
	V17BaseNode
	Stmt *V17StatementNode
}

// ParseFuncReturnStmt parses func_return_stmt = return_arrow statement.
func (p *V17Parser) ParseFuncReturnStmt() (node *V17FuncReturnStmtNode, err error) {
	done := p.debugEnter("func_return_stmt")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, arrerr := p.ParseReturnArrow(); arrerr != nil {
		return nil, fmt.Errorf("func_return_stmt: %w", arrerr)
	}
	stmt, serr := p.ParseStatement()
	if serr != nil {
		return nil, fmt.Errorf("func_return_stmt: expected statement after '<-': %w", serr)
	}
	return &V17FuncReturnStmtNode{V17BaseNode{line, col}, stmt}, nil
}

// V17FuncStoreStmtNode  func_store_stmt = deps_oper ( object_final | ident_ref ) { "," ... }
type V17FuncStoreStmtNode struct {
	V17BaseNode
	// Items is a slice of: *V17ObjectFinalNode | *V17IdentRefNode
	Items []interface{}
}

// ParseFuncStoreStmt parses func_store_stmt = deps_oper ( object_final | TYPE_OF object_final<ident_ref> ) { "," ... }.
// TYPE_OF object_final<ident_ref> is parsed as bare ident_ref (type check deferred to checker).
func (p *V17Parser) ParseFuncStoreStmt() (node *V17FuncStoreStmtNode, err error) {
	done := p.debugEnter("func_store_stmt")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, derr := p.ParseDepsOper(); derr != nil {
		return nil, fmt.Errorf("func_store_stmt: %w", derr)
	}

	first, ferr := p.parseFuncStoreItem()
	if ferr != nil {
		return nil, fmt.Errorf("func_store_stmt: %w", ferr)
	}
	items := []interface{}{first}

	for p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, err = p.matchLit(","); err != nil {
			p.restorePos(saved)
			break
		}
		next, nerr := p.parseFuncStoreItem()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, next)
	}

	return &V17FuncStoreStmtNode{V17BaseNode{line, col}, items}, nil
}

// parseFuncStoreItem parses ( object_final | TYPE_OF object_final<ident_ref> ).
// Tries object_final first (literal), then ident_ref (named variable reference).
func (p *V17Parser) parseFuncStoreItem() (interface{}, error) {
	if saved := p.savePos(); true {
		if of, oferr := p.ParseObjectFinal(); oferr == nil {
			return of, nil
		}
		p.restorePos(saved)
	}
	return p.ParseIdentRef()
}

// =============================================================================
// ITERATOR YIELD STMT  (MERGE from spec/12_iterators.sqg §12.6 — Role B)
// iterator_yield_stmt = func_stmt iterator_oper
// "func_stmt" = any statement valid in a func_unit body (= statement rule)
// Parsed when ">>" immediately follows func_stmt AND next token is NL/EOF/bracket.
// =============================================================================

// V17IteratorYieldStmtNode  iterator_yield_stmt = func_stmt ">>"
type V17IteratorYieldStmtNode struct {
	V17BaseNode
	Stmt *V17StatementNode // the value being yielded downstream
}

// ParseIteratorYieldStmt parses iterator_yield_stmt = func_stmt ">>" (NL|EOF|bracket).
// Succeeds only when ">>" is immediately followed by NL, EOF, ")" or "}".
// Closing brackets are NOT consumed — they remain for the enclosing block parser.
func (p *V17Parser) ParseIteratorYieldStmt() (node *V17IteratorYieldStmtNode, err error) {
	done := p.debugEnter("iterator_yield_stmt")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	stmt, serr := p.ParseStatement()
	if serr != nil {
		return nil, fmt.Errorf("iterator_yield_stmt: %w", serr)
	}

	// Require ">>" immediately following the statement (no WS check needed; token-based check OK)
	if !p.peekLit(">>") {
		return nil, p.errAt("iterator_yield_stmt: expected '>>' after statement")
	}

	// Check what follows ">>" — must be NL, EOF, ")" or "}"
	// Save and matchLit first, then peek at rune-stream for terminator
	saved := p.savePos()
	if _, err = p.matchLit(">>"); err != nil {
		return nil, err
	}
	nextCh := p.peekAfterWS()
	switch nextCh {
	case 0, '\n', ')', '}':
		// valid yield terminator
	default:
		p.restorePos(saved)
		return nil, p.errAt("iterator_yield_stmt: '>>' must be followed by NL, EOF, ')' or '}'")
	}
	return &V17IteratorYieldStmtNode{V17BaseNode{line, col}, stmt}, nil
}

// =============================================================================
// ASSIGN ITERATOR  (spec/12_iterators.sqg §12.6 — Role C lazy binding)
// assign_iterator = collection iterator_oper EOL
// Used as the RHS of an assignment: my_iter = my_collection >> EOL
// Wired into ParseAssignRhs in parser_v17_assignment.go.
// =============================================================================

// V17AssignIteratorNode  assign_iterator = collection ">>" EOL
type V17AssignIteratorNode struct {
	V17BaseNode
	Collection *V17CollectionNode
}

// ParseAssignIterator parses assign_iterator = collection ">>" EOL.
func (p *V17Parser) ParseAssignIterator() (node *V17AssignIteratorNode, err error) {
	done := p.debugEnter("assign_iterator")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	coll, collerr := p.ParseCollection()
	if collerr != nil {
		return nil, fmt.Errorf("assign_iterator: %w", collerr)
	}
	if _, oerr := p.ParseIteratorOper(); oerr != nil {
		return nil, fmt.Errorf("assign_iterator: expected '>>' after collection: %w", oerr)
	}
	if _, eolerr := p.ParseEol(); eolerr != nil {
		return nil, fmt.Errorf("assign_iterator: expected EOL after '>>': %w", eolerr)
	}
	return &V17AssignIteratorNode{V17BaseNode{line, col}, coll}, nil
}

// =============================================================================
// PUSH FORWARD STMT  (MERGE from spec/13_push_pull.sqg §13.6 — Role B)
// push_forward_stmt = func_stmt push_oper
// "func_stmt" = any statement valid in a func_unit body (= statement rule).
// Parsed when "~>" immediately follows func_stmt AND next token is NL/EOF/bracket.
// Closing brackets are NOT consumed — they remain for the enclosing block parser.
// =============================================================================

// V17PushForwardStmtNode  push_forward_stmt = func_stmt "~>"
type V17PushForwardStmtNode struct {
	V17BaseNode
	Stmt *V17StatementNode // the value being pushed forward to registered sinks
}

// ParsePushForwardStmt parses push_forward_stmt = func_stmt "~>" (NL|EOF|bracket).
// Succeeds only when "~>" is immediately followed by NL, EOF, ")" or "}".
func (p *V17Parser) ParsePushForwardStmt() (node *V17PushForwardStmtNode, err error) {
	done := p.debugEnter("push_forward_stmt")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	stmt, serr := p.ParseStatement()
	if serr != nil {
		return nil, fmt.Errorf("push_forward_stmt: %w", serr)
	}

	if !p.peekLit("~>") {
		return nil, p.errAt("push_forward_stmt: expected '~>' after statement")
	}

	saved := p.savePos()
	if _, err = p.matchLit("~>"); err != nil {
		return nil, err
	}
	nextCh := p.peekAfterWS()
	switch nextCh {
	case 0, '\n', ')', '}':
		// valid forward terminator
	default:
		p.restorePos(saved)
		return nil, p.errAt("push_forward_stmt: '~>' must be followed by NL, EOF, ')' or '}'")
	}
	return &V17PushForwardStmtNode{V17BaseNode{line, col}, stmt}, nil
}

// =============================================================================
// PUSH STREAM BIND  (MERGE from spec/13_push_pull.sqg §13.6 — Role C)
// push_stream_bind = collection push_oper ( func_unit | func_call )
//                    { push_oper ( func_unit | func_call ) }
// Requires at least one handler stage after the initial push_oper.
// Additional "~>" chain operators are consumed only when NOT followed by
// NL/EOF/bracket (those would start a push_forward_stmt in the body).
// =============================================================================

// V17PushStreamBindSegment is one handler stage in a push_stream_bind chain.
type V17PushStreamBindSegment struct {
	// Handler is one of: *V17FuncUnitNode | *V17FuncCallNode
	Handler interface{}
}

// V17PushStreamBindNode  push_stream_bind = collection "~>" handler { "~>" handler }
type V17PushStreamBindNode struct {
	V17BaseNode
	Collection *V17CollectionNode
	Segments   []V17PushStreamBindSegment // at least one segment
}

// ParsePushStreamBind parses push_stream_bind = collection "~>" handler { "~>" handler }.
func (p *V17Parser) ParsePushStreamBind() (node *V17PushStreamBindNode, err error) {
	done := p.debugEnter("push_stream_bind")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	coll, collerr := p.ParseCollection()
	if collerr != nil {
		return nil, fmt.Errorf("push_stream_bind: %w", collerr)
	}
	if _, oerr := p.ParsePushOper(); oerr != nil {
		return nil, fmt.Errorf("push_stream_bind: expected '~>' after collection: %w", oerr)
	}

	var segments []V17PushStreamBindSegment
	for {
		// Try func_unit first (starts with "("), then func_call
		var handler interface{}
		if p.peekAfterWS() == '(' {
			if saved := p.savePos(); true {
				if fu, fuerr := p.ParseFuncUnit(); fuerr == nil {
					handler = fu
				} else {
					p.restorePos(saved)
				}
			}
		}
		if handler == nil {
			fc, fcerr := p.ParseFuncCall()
			if fcerr != nil {
				if len(segments) == 0 {
					return nil, fmt.Errorf("push_stream_bind: expected func_unit or func_call after '~>': %w", fcerr)
				}
				break
			}
			handler = fc
		}
		segments = append(segments, V17PushStreamBindSegment{handler})

		// Check for a chain "~>" operator — only consume it when NOT a terminator
		if !p.peekLit("~>") {
			break
		}
		// peek at char after "~>" — save, matchLit, check, restore if terminator
		preview := p.savePos()
		if _, err = p.matchLit("~>"); err != nil {
			break
		}
		nextCh := p.peekAfterWS()
		isTerminator := nextCh == 0 || nextCh == '\n' || nextCh == ')' || nextCh == '}'
		if isTerminator {
			p.restorePos(preview) // put "~>" back
			break
		}
		// chain "~>" consumed, continue loop
	}

	return &V17PushStreamBindNode{V17BaseNode{line, col}, coll, segments}, nil
}

// =============================================================================
// ASSIGN PUSH  (spec/13_push_pull.sqg §13.6 — cold push source)
// assign_push = collection push_oper EOL
// Used as the RHS of an assignment: my_push = my_collection ~> EOL
// Creates a cold push source that activates on the first "~>" sink binding.
// Wired into ParseAssignRhs in parser_v17_assignment.go.
// =============================================================================

// V17AssignPushNode  assign_push = collection "~>" EOL
type V17AssignPushNode struct {
	V17BaseNode
	Collection *V17CollectionNode
}

// ParseAssignPush parses assign_push = collection "~>" EOL.
func (p *V17Parser) ParseAssignPush() (node *V17AssignPushNode, err error) {
	done := p.debugEnter("assign_push")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	coll, collerr := p.ParseCollection()
	if collerr != nil {
		return nil, fmt.Errorf("assign_push: %w", collerr)
	}
	if _, oerr := p.ParsePushOper(); oerr != nil {
		return nil, fmt.Errorf("assign_push: expected '~>' after collection: %w", oerr)
	}
	if _, eolerr := p.ParseEol(); eolerr != nil {
		return nil, fmt.Errorf("assign_push: expected EOL after '~>': %w", eolerr)
	}
	return &V17AssignPushNode{V17BaseNode{line, col}, coll}, nil
}

// =============================================================================
// FUNC BODY STMT  (with MERGE from spec/12_iterators.sqg §12.6 and spec/13_push_pull.sqg §13.6)
// func_body_stmt = assignment | statement | iterator_loop | push_loop
//                | func_return_stmt | func_store_stmt
// MERGE<func_body_stmt> = | iterator_yield_stmt  (12_iterators.sqg)
//                        | push_forward_stmt     (13_push_pull.sqg)
//                        | push_stream_bind      (13_push_pull.sqg)
//
// Implementation ordering (most specific first):
//  1. func_return_stmt   ("<-" prefix — unambiguous)
//  2. func_store_stmt    ("=>" prefix — unambiguous)
//  3. iterator_yield_stmt (MERGE: stmt + ">>" + NL/EOF/bracket)
//  4. push_forward_stmt  (MERGE: stmt + "~>" + NL/EOF/bracket)
//  5. iterator_loop      (collection + ">>" + func)
//  6. push_stream_bind   (MERGE: collection + "~>" + handler — subsumes push_loop)
//  7. push_loop          (collection + "~>" + func — base spec; subsumed by push_stream_bind)
//  8. assignment         (assign_lhs + oper + rhs)
//  9. statement          (general fallback)
// =============================================================================

// V17FuncBodyStmtNode  func_body_stmt (combined rule with MERGE)
type V17FuncBodyStmtNode struct {
	V17BaseNode
	// Value is one of:
	//   *V17FuncReturnStmtNode    | *V17FuncStoreStmtNode
	//   *V17IteratorYieldStmtNode | *V17PushForwardStmtNode
	//   *V17IteratorLoopNode      | *V17PushStreamBindNode | *V17PushLoopNode
	//   *V17AssignmentNode        | *V17StatementNode
	Value interface{}
}

// ParseFuncBodyStmt parses func_body_stmt (MERGE included).
func (p *V17Parser) ParseFuncBodyStmt() (node *V17FuncBodyStmtNode, err error) {
	done := p.debugEnter("func_body_stmt")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// 1. func_return_stmt: starts with "<-"
	if p.DebugFlag {
		preview := string(p.input[p.runePos:])
		if len(preview) > 30 {
			preview = preview[:30]
		}
		_, _ = fmt.Fprintf(os.Stderr, "[FDBG] func_body_stmt pos=%d peek=%q peekLit(<-)=%v\n", p.runePos, preview, p.peekLit("<-"))
	}
	if p.peekLit("<-") {
		rs, rserr := p.ParseFuncReturnStmt()
		if rserr != nil {
			return nil, fmt.Errorf("func_body_stmt: %w", rserr)
		}
		return &V17FuncBodyStmtNode{V17BaseNode{line, col}, rs}, nil
	}

	// 2. func_store_stmt: starts with "=>"
	if p.peekLit("=>") {
		ss, sserr := p.ParseFuncStoreStmt()
		if sserr != nil {
			return nil, fmt.Errorf("func_body_stmt: %w", sserr)
		}
		return &V17FuncBodyStmtNode{V17BaseNode{line, col}, ss}, nil
	}

	// 3. iterator_yield_stmt (MERGE): stmt + ">>" + NL/EOF/bracket — try before plain statement
	if saved := p.savePos(); true {
		if iy, iyerr := p.ParseIteratorYieldStmt(); iyerr == nil {
			return &V17FuncBodyStmtNode{V17BaseNode{line, col}, iy}, nil
		}
		p.restorePos(saved)
	}

	// 4. push_forward_stmt (MERGE): stmt + "~>" + NL/EOF/bracket — try before plain statement
	if saved := p.savePos(); true {
		if pf, pferr := p.ParsePushForwardStmt(); pferr == nil {
			return &V17FuncBodyStmtNode{V17BaseNode{line, col}, pf}, nil
		}
		p.restorePos(saved)
	}

	// 5. iterator_loop: collection + ">>" + func
	if saved := p.savePos(); true {
		if il, ilerr := p.ParseIteratorLoop(); ilerr == nil {
			return &V17FuncBodyStmtNode{V17BaseNode{line, col}, il}, nil
		}
		p.restorePos(saved)
	}

	// 6. push_stream_bind (MERGE): collection + "~>" + handler — subsumes push_loop
	if saved := p.savePos(); true {
		if psb, psberr := p.ParsePushStreamBind(); psberr == nil {
			return &V17FuncBodyStmtNode{V17BaseNode{line, col}, psb}, nil
		}
		p.restorePos(saved)
	}

	// 7. push_loop: collection + "~>" + func (base spec; subsumed by push_stream_bind above)
	if saved := p.savePos(); true {
		if pl, plerr := p.ParsePushLoop(); plerr == nil {
			return &V17FuncBodyStmtNode{V17BaseNode{line, col}, pl}, nil
		}
		p.restorePos(saved)
	}

	// 8. assignment
	if saved := p.savePos(); true {
		if a, aerr := p.ParseAssignment(); aerr == nil {
			return &V17FuncBodyStmtNode{V17BaseNode{line, col}, a}, nil
		}
		p.restorePos(saved)
	}

	// 9. statement (general fallback — includes func_call_final via EXTEND)
	if saved := p.savePos(); true {
		if s, serr := p.ParseStatement(); serr == nil {
			return &V17FuncBodyStmtNode{V17BaseNode{line, col}, s}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("func_body_stmt: expected assignment, statement, iterator_loop, push_loop, push_stream_bind, func_return_stmt, func_store_stmt, iterator_yield_stmt, or push_forward_stmt")
}

// =============================================================================
// FUNCTION UNIT
// func_unit        = group_begin func_args_decl func_body_stmt { EOL | func_body_stmt } group_end
// return_func_unit = return_arrow func_unit
// =============================================================================

// V17FuncUnitNode  func_unit = "(" func_args_decl func_body_stmt { EOL | func_body_stmt } ")"
type V17FuncUnitNode struct {
	V17BaseNode
	ArgsDecl *V17FuncArgsDeclNode
	Body     []*V17FuncBodyStmtNode
}

// ParseFuncUnit parses func_unit = group_begin func_args_decl func_body_stmt { EOL | func_body_stmt } group_end.
func (p *V17Parser) ParseFuncUnit() (node *V17FuncUnitNode, err error) {
	done := p.debugEnter("func_unit")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, gerr := p.ParseGroupBegin(); gerr != nil {
		return nil, fmt.Errorf("func_unit: %w", gerr)
	}

	// Skip any leading newlines before args_decl
	p.skipNL()

	argsDecl, adeclerr := p.ParseFuncArgsDecl()
	if adeclerr != nil {
		return nil, fmt.Errorf("func_unit: %w", adeclerr)
	}

	// Skip newlines after args_decl (may be on its own line)
	p.skipNL()

	// Required: at least one func_body_stmt
	firstStmt, fserr := p.ParseFuncBodyStmt()
	if fserr != nil {
		return nil, fmt.Errorf("func_unit: expected at least one func_body_stmt: %w", fserr)
	}
	body := []*V17FuncBodyStmtNode{firstStmt}

	// { EOL | func_body_stmt } — zero or more additional EOLs or statements
	for {
		// Stop when we find the closing paren or EOF
		if p.peekAfterWS() == ')' || p.peekAfterWS() == 0 {
			break
		}
		// Try EOL first (blank lines between statements)
		if saved := p.savePos(); true {
			if _, eolerr := p.ParseEol(); eolerr == nil {
				continue
			}
			p.restorePos(saved)
		}
		// Try another func_body_stmt
		if saved := p.savePos(); true {
			if s, serr := p.ParseFuncBodyStmt(); serr == nil {
				body = append(body, s)
				continue
			}
			p.restorePos(saved)
		}
		break
	}

	// Skip trailing newlines before the closing paren
	p.skipNL()

	if _, gerr := p.ParseGroupEnd(); gerr != nil {
		return nil, fmt.Errorf("func_unit: %w", gerr)
	}

	return &V17FuncUnitNode{V17BaseNode{line, col}, argsDecl, body}, nil
}

// V17ReturnFuncUnitNode  return_func_unit = return_arrow func_unit
type V17ReturnFuncUnitNode struct {
	V17BaseNode
	FuncUnit *V17FuncUnitNode
}

// ParseReturnFuncUnit parses return_func_unit = return_arrow func_unit.
func (p *V17Parser) ParseReturnFuncUnit() (node *V17ReturnFuncUnitNode, err error) {
	done := p.debugEnter("return_func_unit")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, arrerr := p.ParseReturnArrow(); arrerr != nil {
		return nil, fmt.Errorf("return_func_unit: %w", arrerr)
	}
	fu, fuerr := p.ParseFuncUnit()
	if fuerr != nil {
		return nil, fmt.Errorf("return_func_unit: %w", fuerr)
	}
	return &V17ReturnFuncUnitNode{V17BaseNode{line, col}, fu}, nil
}

// Ensure fmt is used.
var _ = fmt.Sprintf
