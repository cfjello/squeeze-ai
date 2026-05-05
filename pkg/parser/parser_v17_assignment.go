// parser_v17_assignment.go — Parse methods for spec/03_assignment.sqg
// Rules: update_mutable_oper, assign_mutable, assign_immutable, assign_read_only_ref,
//
//	assign_version, assign_lhs, assign_cond_rhs, assign_single, private_modifier,
//	assign_private_single, assign_private_grouping, assign_rhs,
//	assign_new_var, update_mutable, assignment
//
// Forward-reference stubs (implemented fully in later spec files):
//
//	self_ref  (spec/04_functions.sqg — V17_DOLLAR token)
package parser

import "fmt"

// =============================================================================
// ASSIGNMENT OPERATOR TOKENS
// update_mutable_oper = "+=" | "-=" | "*=" | "/=" ;

// V17UpdateMutableOperNode  update_mutable_oper = "+=" | "-=" | "*=" | "/="
type V17UpdateMutableOperNode struct {
	V17BaseNode
	Value string // "+=", "-=", "*=", "/="
}

// ParseUpdateMutableOper parses update_mutable_oper = "+=" | "-=" | "*=" | "/="
func (p *V17Parser) ParseUpdateMutableOper() (node *V17UpdateMutableOperNode, err error) {
	done := p.debugEnter("update_mutable_oper")
	defer func() { done(err == nil) }()
	ch := p.peekAfterWS()
	var op string
	switch ch {
	case '+':
		op = "+="
	case '-':
		op = "-="
	case '*':
		op = "*="
	case '/':
		op = "/="
	default:
		return nil, p.errAt("update_mutable_oper: expected +=, -=, *= or /=")
	}
	tok, terr := p.matchLit(op)
	if terr != nil {
		return nil, p.errAt("update_mutable_oper: expected %s", op)
	}
	return &V17UpdateMutableOperNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// =============================================================================
// SIMPLE ASSIGN-OPERATOR TOKENS

// V17AssignMutableNode  assign_mutable = "="
type V17AssignMutableNode struct{ V17BaseNode }

// ParseAssignMutable parses assign_mutable = "="
func (p *V17Parser) ParseAssignMutable() (node *V17AssignMutableNode, err error) {
	done := p.debugEnter("assign_mutable")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("=")
	if err != nil {
		return nil, fmt.Errorf("assign_mutable: %w", err)
	}
	return &V17AssignMutableNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17AssignImmutableNode  assign_immutable = ":"
type V17AssignImmutableNode struct{ V17BaseNode }

// ParseAssignImmutable parses assign_immutable = ":"
func (p *V17Parser) ParseAssignImmutable() (node *V17AssignImmutableNode, err error) {
	done := p.debugEnter("assign_immutable")
	defer func() { done(err == nil) }()
	saved := p.savePos()
	tok, err := p.matchLit(":")
	if err != nil {
		return nil, fmt.Errorf("assign_immutable: %w", err)
	}
	// Reject ":~" — that belongs to assign_read_only_ref
	if p.runePos < len(p.input) && p.input[p.runePos] == '~' {
		p.restorePos(saved)
		return nil, p.errAt("assign_immutable: ':~' is assign_read_only_ref, not assign_immutable")
	}
	return &V17AssignImmutableNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// V17AssignReadOnlyRefNode  assign_read_only_ref = ":~"
type V17AssignReadOnlyRefNode struct{ V17BaseNode }

// ParseAssignReadOnlyRef parses assign_read_only_ref = ":~"
func (p *V17Parser) ParseAssignReadOnlyRef() (node *V17AssignReadOnlyRefNode, err error) {
	done := p.debugEnter("assign_read_only_ref")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit(":~")
	if err != nil {
		return nil, fmt.Errorf("assign_read_only_ref: %w", err)
	}
	return &V17AssignReadOnlyRefNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// =============================================================================
// ASSIGN VERSION
// assign_version = ["v"] digits { "." digits }

// V17AssignVersionNode  assign_version = ["v"] digits { "." digits }
type V17AssignVersionNode struct {
	V17BaseNode
	HasV  bool     // true when the optional "v" prefix is present
	Parts []string // digit strings, e.g. ["1", "2", "3"] for "1.2.3"
}

// ParseAssignVersion parses assign_version = ["v"] digits { "." digits }
func (p *V17Parser) ParseAssignVersion() (node *V17AssignVersionNode, err error) {
	done := p.debugEnter("assign_version")
	defer func() { done(err == nil) }()

	hasV := false
	var firstPart string
	var line, col int

	ch := p.peekAfterWS()
	if ch != 'v' {
		return nil, p.errAt("assign_version: expected 'v'+digits identifier")
	}
	saved := p.savePos()
	tok, terr := p.matchRe(reIdentScan)
	if terr != nil || len(tok.Value) < 2 || tok.Value[0] != 'v' {
		p.restorePos(saved)
		return nil, p.errAt("assign_version: expected 'v'+digits identifier")
	}
	for i := 1; i < len(tok.Value); i++ {
		if tok.Value[i] < '0' || tok.Value[i] > '9' {
			p.restorePos(saved)
			return nil, p.errAt("assign_version: invalid version identifier %q", tok.Value)
		}
	}
	line, col = tok.Line, tok.Col
	hasV = true
	firstPart = tok.Value[1:]

	parts := []string{firstPart}

	// { "." digits }
	for p.peekAfterWS() == '.' {
		saved := p.savePos()
		if _, merr := p.matchLit("."); merr != nil {
			p.restorePos(saved)
			break
		}
		extra, eerr := p.ParseDigits()
		if eerr != nil {
			p.restorePos(saved)
			break
		}
		parts = append(parts, extra.Value)
	}

	return &V17AssignVersionNode{V17BaseNode{line, col}, hasV, parts}, nil
}

// =============================================================================
// ASSIGN LHS
// assign_lhs = UNIQUE<ident_name ["," (ident_name | cardinality | assign_version)]
//                               ["," (ident_name | cardinality | assign_version)]
//                               ["," (ident_name | cardinality | assign_version)]>
// UNIQUE is a checker-time directive; the parser collects at most 3 annotations.

// V17AssignLhsNode  assign_lhs = ident_name with up to 3 optional comma-separated annotations
type V17AssignLhsNode struct {
	V17BaseNode
	Name        *V17IdentNameNode
	Annotations []interface{} // each: *V17IdentNameNode | *V17CardinalityNode | *V17AssignVersionNode
}

// parseAssignAnnotation tries assign_version | cardinality | ident_name in precedence order.
func (p *V17Parser) parseAssignAnnotation() (interface{}, error) {
	// assign_version first — more specific (may start with "v" + digits or bare digits)
	if saved := p.savePos(); true {
		if av, averr := p.ParseAssignVersion(); averr == nil {
			return av, nil
		}
		p.restorePos(saved)
	}
	// cardinality
	if saved := p.savePos(); true {
		if ca, caerr := p.ParseCardinality(); caerr == nil {
			return ca, nil
		}
		p.restorePos(saved)
	}
	// ident_name
	if saved := p.savePos(); true {
		if in, inerr := p.ParseIdentName(); inerr == nil {
			return in, nil
		}
		p.restorePos(saved)
	}
	return nil, p.errAt("assign_lhs annotation: expected ident_name, cardinality or assign_version")
}

// ParseAssignLhs parses assign_lhs = ident_name [, annotation] [, annotation] [, annotation]
func (p *V17Parser) ParseAssignLhs() (node *V17AssignLhsNode, err error) {
	done := p.debugEnter("assign_lhs")
	defer func() { done(err == nil) }()

	// Required: ident_name
	name, nerr := p.ParseIdentName()
	if nerr != nil {
		return nil, p.errAt("assign_lhs: expected ident_name")
	}
	line, col := name.Line, name.Col

	// Optional: up to 3 comma-separated annotations
	var annotations []interface{}
	for len(annotations) < 3 && p.peekAfterWS() == ',' {
		saved := p.savePos()
		if _, cerr := p.matchLit(","); cerr != nil {
			p.restorePos(saved)
			break
		}
		ann, aerr := p.parseAssignAnnotation()
		if aerr != nil {
			p.restorePos(saved)
			break
		}
		annotations = append(annotations, ann)
	}

	return &V17AssignLhsNode{V17BaseNode{line, col}, name, annotations}, nil
}

// =============================================================================
// ASSIGN COND RHS
// assign_cond_rhs = condition logic_and statement { logic_exclusive_or condition logic_and statement }

// V17AssignCondRhsExtraNode is one additional { logic_exclusive_or condition logic_and statement } tuple.
type V17AssignCondRhsExtraNode struct {
	Xor  *V17LogicExclusiveOrNode
	Cond *V17ConditionNode
	And  *V17LogicAndNode
	Stmt *V17StatementNode
}

// V17AssignCondRhsNode  assign_cond_rhs = condition logic_and statement { ... }
type V17AssignCondRhsNode struct {
	V17BaseNode
	Cond   *V17ConditionNode
	And    *V17LogicAndNode
	Stmt   *V17StatementNode
	Extras []V17AssignCondRhsExtraNode
}

// ParseAssignCondRhs parses assign_cond_rhs = condition logic_and statement { logic_exclusive_or condition logic_and statement }
func (p *V17Parser) ParseAssignCondRhs() (node *V17AssignCondRhsNode, err error) {
	done := p.debugEnter("assign_cond_rhs")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	cond, cerr := p.ParseCondition()
	if cerr != nil {
		return nil, fmt.Errorf("assign_cond_rhs: %w", cerr)
	}
	and, aerr := p.ParseLogicAnd()
	if aerr != nil {
		return nil, fmt.Errorf("assign_cond_rhs: %w", aerr)
	}
	stmt, serr := p.ParseStatement()
	if serr != nil {
		return nil, fmt.Errorf("assign_cond_rhs: %w", serr)
	}

	var extras []V17AssignCondRhsExtraNode
	for {
		saved := p.savePos()
		xor, xerr := p.ParseLogicExclusiveOr()
		if xerr != nil {
			p.restorePos(saved)
			break
		}
		econd, ecerr := p.ParseCondition()
		if ecerr != nil {
			p.restorePos(saved)
			break
		}
		eand, eaerr := p.ParseLogicAnd()
		if eaerr != nil {
			p.restorePos(saved)
			break
		}
		estmt, eserr := p.ParseStatement()
		if eserr != nil {
			p.restorePos(saved)
			break
		}
		extras = append(extras, V17AssignCondRhsExtraNode{xor, econd, eand, estmt})
	}

	return &V17AssignCondRhsNode{V17BaseNode{line, col}, cond, and, stmt, extras}, nil
}

// =============================================================================
// ASSIGN SINGLE
// assign_single = assign_lhs ( assign_mutable | assign_immutable | assign_read_only_ref ) statement
//               | assign_cond_rhs
// (spec/03_assignment.sqg line 19 — V18 fix added explicit operator between lhs and statement)

// V17AssignLhsStmtNode is the assign_lhs oper statement pairing within assign_single.
type V17AssignLhsStmtNode struct {
	V17BaseNode
	Lhs  *V17AssignLhsNode
	Oper interface{} // *V17AssignMutableNode | *V17AssignImmutableNode | *V17AssignReadOnlyRefNode
	Stmt *V17StatementNode
}

// V17AssignSingleNode  assign_single = assign_lhs oper statement | assign_cond_rhs
type V17AssignSingleNode struct {
	V17BaseNode
	// Value is one of: *V17AssignLhsStmtNode | *V17AssignCondRhsNode
	Value interface{}
}

// ParseAssignSingle parses assign_single = assign_lhs ( assign_mutable | assign_immutable | assign_read_only_ref ) statement
//
//	| assign_cond_rhs
func (p *V17Parser) ParseAssignSingle() (node *V17AssignSingleNode, err error) {
	done := p.debugEnter("assign_single")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Try assign_lhs oper statement first.
	// The operator ( assign_mutable | assign_immutable | assign_read_only_ref ) is mandatory.
	if saved := p.savePos(); true {
		if lhs, lerr := p.ParseAssignLhs(); lerr == nil {
			var oper interface{}
			if p.peekLit(":~") {
				oper, _ = p.ParseAssignReadOnlyRef()
			} else if p.peekAfterWS() == ':' {
				oper, _ = p.ParseAssignImmutable()
			} else if p.peekAfterWS() == '=' {
				oper, _ = p.ParseAssignMutable()
			}
			if oper != nil {
				if stmt, serr := p.ParseStatement(); serr == nil {
					inner := &V17AssignLhsStmtNode{V17BaseNode{line, col}, lhs, oper, stmt}
					return &V17AssignSingleNode{V17BaseNode{line, col}, inner}, nil
				}
			}
		}
		p.restorePos(saved)
	}

	// Try assign_cond_rhs
	if saved := p.savePos(); true {
		if cond, cerr := p.ParseAssignCondRhs(); cerr == nil {
			return &V17AssignSingleNode{V17BaseNode{line, col}, cond}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("assign_single: expected assign_lhs oper statement or assign_cond_rhs")
}

// =============================================================================
// PRIVATE MODIFIER
// private_modifier = "-"

// V17PrivateModifierNode  private_modifier = "-"
type V17PrivateModifierNode struct{ V17BaseNode }

// ParsePrivateModifier parses private_modifier = "-"
func (p *V17Parser) ParsePrivateModifier() (node *V17PrivateModifierNode, err error) {
	done := p.debugEnter("private_modifier")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("-")
	if err != nil {
		return nil, fmt.Errorf("private_modifier: %w", err)
	}
	return &V17PrivateModifierNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// =============================================================================
// ASSIGN PRIVATE SINGLE
// assign_private_single = private_modifier assign_single
// (spec has typo "single_assign" — treated as "assign_single")

// V17AssignPrivateSingleNode  assign_private_single = private_modifier assign_single
type V17AssignPrivateSingleNode struct {
	V17BaseNode
	Modifier *V17PrivateModifierNode
	Single   *V17AssignSingleNode
}

// ParseAssignPrivateSingle parses assign_private_single = private_modifier assign_single
func (p *V17Parser) ParseAssignPrivateSingle() (node *V17AssignPrivateSingleNode, err error) {
	done := p.debugEnter("assign_private_single")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	mod, merr := p.ParsePrivateModifier()
	if merr != nil {
		return nil, fmt.Errorf("assign_private_single: %w", merr)
	}
	single, serr := p.ParseAssignSingle()
	if serr != nil {
		return nil, fmt.Errorf("assign_private_single: %w", serr)
	}
	return &V17AssignPrivateSingleNode{V17BaseNode{line, col}, mod, single}, nil
}

// =============================================================================
// ASSIGN PRIVATE GROUPING
// assign_private_grouping = private_modifier group_begin assign_single { EOL assign_single } group_end
// (spec/03_assignment.sqg line 24 — V18 added private_modifier before group_begin)

// V17AssignPrivateGroupingNode  assign_private_grouping = private_modifier group_begin assign_single { EOL assign_single } group_end
type V17AssignPrivateGroupingNode struct {
	V17BaseNode
	Modifier *V17PrivateModifierNode
	Begin    *V17GroupBeginNode
	Items    []*V17AssignSingleNode
	End      *V17GroupEndNode
}

// ParseAssignPrivateGrouping parses assign_private_grouping = private_modifier group_begin assign_single { EOL assign_single } group_end
func (p *V17Parser) ParseAssignPrivateGrouping() (node *V17AssignPrivateGroupingNode, err error) {
	done := p.debugEnter("assign_private_grouping")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	mod, merr := p.ParsePrivateModifier()
	if merr != nil {
		return nil, fmt.Errorf("assign_private_grouping: %w", merr)
	}

	begin, berr := p.ParseGroupBegin()
	if berr != nil {
		return nil, fmt.Errorf("assign_private_grouping: %w", berr)
	}

	// First assign_single (required)
	first, ferr := p.ParseAssignSingle()
	if ferr != nil {
		return nil, fmt.Errorf("assign_private_grouping: %w", ferr)
	}
	items := []*V17AssignSingleNode{first}

	// { EOL assign_single }
	for {
		saved := p.savePos()
		if _, eolerr := p.ParseEol(); eolerr != nil {
			p.restorePos(saved)
			break
		}
		extra, eerr := p.ParseAssignSingle()
		if eerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, extra)
	}

	end, eerr := p.ParseGroupEnd()
	if eerr != nil {
		return nil, fmt.Errorf("assign_private_grouping: %w", eerr)
	}

	return &V17AssignPrivateGroupingNode{V17BaseNode{line, col}, mod, begin, items, end}, nil
}

// =============================================================================
// ASSIGN RHS
// assign_rhs = ( assign_cond_rhs | statement ) | assign_private_single | assign_private_grouping
// (spec/03_assignment.sqg line 26 — V18 replaced assign_single with ( assign_cond_rhs | statement ))

// V17AssignRhsNode  assign_rhs = ( assign_cond_rhs | statement ) | assign_private_single | assign_private_grouping
type V17AssignRhsNode struct {
	V17BaseNode
	// Value is one of: *V17AssignCondRhsNode | *V17StatementNode |
	//                  *V17AssignPrivateSingleNode | *V17AssignPrivateGroupingNode |
	//                  *V17AssignIteratorNode | *V17AssignPushNode
	Value interface{}
}

// ParseAssignRhs parses assign_rhs = ( assign_cond_rhs | statement ) | assign_private_single | assign_private_grouping
func (p *V17Parser) ParseAssignRhs() (node *V17AssignRhsNode, err error) {
	done := p.debugEnter("assign_rhs")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Both assign_private_single and assign_private_grouping start with '-' (private_modifier).
	// Try assign_private_grouping first (more specific: '-' then '(').
	if saved := p.savePos(); true {
		if pg, pgerr := p.ParseAssignPrivateGrouping(); pgerr == nil {
			return &V17AssignRhsNode{V17BaseNode{line, col}, pg}, nil
		}
		p.restorePos(saved)
	}

	// assign_private_single — '-' then assign_single
	if saved := p.savePos(); true {
		if ps, pserr := p.ParseAssignPrivateSingle(); pserr == nil {
			return &V17AssignRhsNode{V17BaseNode{line, col}, ps}, nil
		}
		p.restorePos(saved)
	}

	// assign_iterator (spec/12_iterators.sqg §12.6) — collection ">>" EOL
	// Tried before statement so that "my_collection >> EOL" as an assignment
	// RHS is correctly captured as a lazy iterator binding rather than a
	// partial statement (ParseStatement stops at ">>" without consuming it).
	if saved := p.savePos(); true {
		if ai, aierr := p.ParseAssignIterator(); aierr == nil {
			return &V17AssignRhsNode{V17BaseNode{line, col}, ai}, nil
		}
		p.restorePos(saved)
	}

	// assign_push (spec/13_push_pull.sqg §13.6) — collection "~>" EOL
	// Tried before statement for the same reason as assign_iterator above.
	if saved := p.savePos(); true {
		if ap, aperr := p.ParseAssignPush(); aperr == nil {
			return &V17AssignRhsNode{V17BaseNode{line, col}, ap}, nil
		}
		p.restorePos(saved)
	}

	// iterator_loop — collection ">>" ( func_unit | func_call | statement )
	// Tried AFTER assign_iterator so that "collection >> EOL" (lazy binding) is still caught
	// by assign_iterator, while "collection >> body" is correctly captured here.
	if saved := p.savePos(); true {
		if il, ilerr := p.ParseIteratorLoop(); ilerr == nil {
			return &V17AssignRhsNode{V17BaseNode{line, col}, il}, nil
		}
		p.restorePos(saved)
	}

	// ( assign_cond_rhs | statement ) — try assign_cond_rhs first (more specific)
	if saved := p.savePos(); true {
		if ac, acerr := p.ParseAssignCondRhs(); acerr == nil {
			return &V17AssignRhsNode{V17BaseNode{line, col}, ac}, nil
		}
		p.restorePos(saved)
	}
	if saved := p.savePos(); true {
		if st, sterr := p.ParseStatement(); sterr == nil {
			return &V17AssignRhsNode{V17BaseNode{line, col}, st}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("assign_rhs: expected assign_cond_rhs, statement, assign_private_single, assign_private_grouping, or assign_iterator")
}

// =============================================================================
// ASSIGN NEW VAR
// assign_new_var = assign_lhs ( assign_immutable | assign_read_only_ref | assign_mutable ) assign_rhs

// V17AssignNewVarNode  assign_new_var = assign_lhs oper assign_rhs
type V17AssignNewVarNode struct {
	V17BaseNode
	Lhs *V17AssignLhsNode
	// Oper is one of: *V17AssignImmutableNode | *V17AssignReadOnlyRefNode | *V17AssignMutableNode
	Oper interface{}
	Rhs  *V17AssignRhsNode
}

// ParseAssignNewVar parses assign_new_var = assign_lhs ( assign_immutable | assign_read_only_ref | assign_mutable ) assign_rhs
func (p *V17Parser) ParseAssignNewVar() (node *V17AssignNewVarNode, err error) {
	done := p.debugEnter("assign_new_var")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	lhs, lerr := p.ParseAssignLhs()
	if lerr != nil {
		return nil, fmt.Errorf("assign_new_var: %w", lerr)
	}

	// Operator: try assign_read_only_ref (":~") first — must precede assign_immutable (":") to avoid prefix match
	var oper interface{}
	if p.peekLit(":~") {
		ro, roerr := p.ParseAssignReadOnlyRef()
		if roerr != nil {
			return nil, fmt.Errorf("assign_new_var: %w", roerr)
		}
		oper = ro
	} else if p.peekAfterWS() == ':' {
		im, imerr := p.ParseAssignImmutable()
		if imerr != nil {
			return nil, fmt.Errorf("assign_new_var: %w", imerr)
		}
		oper = im
	} else if p.peekAfterWS() == '=' {
		mu, muerr := p.ParseAssignMutable()
		if muerr != nil {
			return nil, fmt.Errorf("assign_new_var: %w", muerr)
		}
		oper = mu
	} else {
		return nil, p.errAt("assign_new_var: expected :, :~ or =")
	}

	rhs, rerr := p.ParseAssignRhs()
	if rerr != nil {
		return nil, fmt.Errorf("assign_new_var: %w", rerr)
	}

	return &V17AssignNewVarNode{V17BaseNode{line, col}, lhs, oper, rhs}, nil
}

// =============================================================================
// UPDATE MUTABLE
// update_mutable = ident_ref update_mutable_oper assign_rhs

// V17UpdateMutableNode  update_mutable = ident_ref update_mutable_oper assign_rhs
type V17UpdateMutableNode struct {
	V17BaseNode
	Ref  *V17IdentRefNode
	Oper *V17UpdateMutableOperNode
	Rhs  *V17AssignRhsNode
}

// ParseUpdateMutable parses update_mutable = ident_ref update_mutable_oper assign_rhs
func (p *V17Parser) ParseUpdateMutable() (node *V17UpdateMutableNode, err error) {
	done := p.debugEnter("update_mutable")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	ref, rerr := p.ParseIdentRef()
	if rerr != nil {
		return nil, fmt.Errorf("update_mutable: %w", rerr)
	}
	oper, oerr := p.ParseUpdateMutableOper()
	if oerr != nil {
		return nil, fmt.Errorf("update_mutable: %w", oerr)
	}
	rhs, rherr := p.ParseAssignRhs()
	if rherr != nil {
		return nil, fmt.Errorf("update_mutable: %w", rherr)
	}
	return &V17UpdateMutableNode{V17BaseNode{line, col}, ref, oper, rhs}, nil
}

// =============================================================================
// ASSIGNMENT
// assignment = assign_new_var | update_mutable

// V17AssignmentNode  assignment = assign_new_var | update_mutable
type V17AssignmentNode struct {
	V17BaseNode
	// Value is one of: *V17AssignNewVarNode | *V17UpdateMutableNode
	Value interface{}
}

// ParseAssignment parses assignment = assign_new_var | update_mutable
func (p *V17Parser) ParseAssignment() (node *V17AssignmentNode, err error) {
	done := p.debugEnter("assignment")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// Try assign_new_var first — uses ":", ":~", "=" operators
	if saved := p.savePos(); true {
		if nv, nverr := p.ParseAssignNewVar(); nverr == nil {
			return &V17AssignmentNode{V17BaseNode{line, col}, nv}, nil
		}
		p.restorePos(saved)
	}

	// Try update_mutable — uses "+=", "-=", "*=", "/=" operators
	if saved := p.savePos(); true {
		if um, umerr := p.ParseUpdateMutable(); umerr == nil {
			return &V17AssignmentNode{V17BaseNode{line, col}, um}, nil
		}
		p.restorePos(saved)
	}

	return nil, p.errAt("assignment: expected assign_new_var or update_mutable")
}

// =============================================================================
// SELF REF  (forward-reference stub — spec/04_functions.sqg)
// self_ref = "$"
// Full implementation lives in parser_v17_functions.go once 04_functions is added.

// V17SelfRefNode  self_ref = "$"
type V17SelfRefNode struct{ V17BaseNode }

// ParseSelfRef parses self_ref = "$"
func (p *V17Parser) ParseSelfRef() (node *V17SelfRefNode, err error) {
	done := p.debugEnter("self_ref")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("$")
	if err != nil {
		return nil, fmt.Errorf("self_ref: %w", err)
	}
	return &V17SelfRefNode{V17BaseNode{tok.Line, tok.Col}}, nil
}
