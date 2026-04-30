// parser_v10_assignment.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V10 grammar rule set defined in spec/03_assignment.sqg.
//
// Covered rules:
//
//	incr_assign_immutable, incr_assign_mutable, assign_mutable, assign_immutable,
//	assign_read_only_ref, equal_assign, assign_oper
//	assign_version
//	assign_lhs  (UNIQUE<ident_name [ , annotation ]...>)
//	assign_rhs
//	assignment
package parser

import (
	"fmt"
	"strings"
)

// =============================================================================
// PHASE 2 — AST NODE TYPES  (03_assignment.sqg)
// =============================================================================

// V10AssignOperKind classifies the assignment operator.
type V10AssignOperKind int

const (
	// equal-assign variants
	AssignMutable     V10AssignOperKind = iota // =
	AssignImmutable                            // :
	AssignReadOnlyRef                          // :~
	// incremental-immutable variants (+: -: *: /:)
	IncrAddImmutable // +:
	IncrSubImmutable // -:
	IncrMulImmutable // *:
	IncrDivImmutable // /:
	// incremental-mutable variants (+= -= *= /=)
	IncrAddMutable // +=
	IncrSubMutable // -=
	IncrMulMutable // *=
	IncrDivMutable // /=
)

// V10AssignOperNode  assign_oper = incr_assign_immutable | incr_assign_mutable | equal_assign
type V10AssignOperNode struct {
	v10BaseNode
	Kind  V10AssignOperKind
	Value string // verbatim source text of the operator
}

// V10AssignVersionNode  assign_version = "v" digits { "." digits }
// e.g. "v1", "v1.0", "v2.3.14"
type V10AssignVersionNode struct {
	v10BaseNode
	Raw string // e.g. "v1.0.2"
}

// V10AssignAnnotation is one optional comma-separated LHS annotation:
// either another ident_name, a cardinality, or an assign_version.
// Exactly one field is non-nil / non-empty.
type V10AssignAnnotation struct {
	IdentName string                // plain identifier annotation
	Cardin    *V10CardinalityNode   // cardinality  e.g. 0..many
	Version   *V10AssignVersionNode // version      e.g. v1.0
}

// V10AssignLHSNode  assign_lhs = UNIQUE<ident_name [, annotation [, annotation [, annotation]]]>
// UNIQUE is a grammar-level directive (uniqueness constraint enforced by the
// runtime).  At parse time the content is just: ident_name + up to 3 optional
// comma-prefixed annotations.
type V10AssignLHSNode struct {
	v10BaseNode
	Name        string                // the primary ident_name
	Annotations []V10AssignAnnotation // 0..3 entries
}

// V10AssignRHSNode  assign_rhs = constant | calc_unit
type V10AssignRHSNode struct {
	v10BaseNode
	Value V10Node // *V10ConstantNode | *V10CalcUnitNode
}

// V10AssignmentNode  assignment = assign_lhs assign_oper assign_rhs
type V10AssignmentNode struct {
	v10BaseNode
	LHS  *V10AssignLHSNode
	Oper *V10AssignOperNode
	RHS  *V10AssignRHSNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (03_assignment.sqg)
// =============================================================================

// ---------- assign_oper ----------

// ParseAssignOper parses:
//
//	assign_oper = incr_assign_immutable | incr_assign_mutable | equal_assign
//
//	incr_assign_immutable = "+:" | "-:" | "*:" | "/:"
//	incr_assign_mutable   = "+=" | "-=" | "*=" | "/="
//	equal_assign          = "=" | ":" | ":~"
func (p *V10Parser) ParseAssignOper() (*V10AssignOperNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var kind V10AssignOperKind
	var val string

	switch tok.Type {
	// equal-assign
	case V10_EQ:
		kind, val = AssignMutable, "="
	case V10_COLON:
		kind, val = AssignImmutable, ":"
	case V10_READONLY:
		kind, val = AssignReadOnlyRef, ":~"

	// incr_assign_immutable
	case V10_IADD_IMM:
		kind, val = IncrAddImmutable, "+:"
	case V10_ISUB_IMM:
		kind, val = IncrSubImmutable, "-:"
	case V10_IMUL_IMM:
		kind, val = IncrMulImmutable, "*:"
	case V10_IDIV_IMM:
		kind, val = IncrDivImmutable, "/:"

	// incr_assign_mutable
	case V10_EXTEND_ASSIGN:
		kind, val = IncrAddMutable, "+="
	case V10_ISUB_ASSIGN:
		kind, val = IncrSubMutable, "-="
	case V10_IMUL_ASSIGN:
		kind, val = IncrMulMutable, "*="
	case V10_IDIV_ASSIGN:
		kind, val = IncrDivMutable, "/="

	default:
		return nil, p.errAt(fmt.Sprintf("expected assignment operator, got %s %q", tok.Type, tok.Value))
	}

	p.advance()
	return &V10AssignOperNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Kind: kind, Value: val}, nil
}

// ---------- assign_version ----------

// ParseAssignVersion parses:  assign_version = "v" digits { "." digits }
//
// In source text this appears as an identifier that starts with 'v' followed
// immediately by digits (e.g. "v1", "v10"), then optionally ".digits" chains.
// The lexer merges "v1" into a single IDENT token; subsequent ".2.3" come as
// separate DOT + INTEGER tokens.
func (p *V10Parser) ParseAssignVersion() (*V10AssignVersionNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V10_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected version (e.g. v1), got %s %q", tok.Type, tok.Value))
	}
	if !v10looksLikeVersion(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected version identifier (v<digits>), got %q", tok.Value))
	}

	var sb strings.Builder
	sb.WriteString(tok.Value) // e.g. "v1"
	p.advance()

	// Consume optional ".digits" continuations.
	for p.cur().Type == V10_DOT {
		saved := p.savePos()
		p.advance() // consume "."
		if p.cur().Type != V10_INTEGER {
			p.restorePos(saved)
			break
		}
		sb.WriteByte('.')
		sb.WriteString(p.cur().Value)
		p.advance()
	}

	return &V10AssignVersionNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		Raw:         sb.String(),
	}, nil
}

// v10looksLikeVersion reports whether s has the form "v<one-or-more-digits>".
func v10looksLikeVersion(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ---------- assign_lhs ----------

// ParseAssignLHS parses:
//
//	assign_lhs = UNIQUE<ident_name
//	             [ "," ( ident_name | cardinality | assign_version ) ]
//	             [ "," ( ident_name | cardinality | assign_version ) ]
//	             [ "," ( ident_name | cardinality | assign_version ) ]>
//
// UNIQUE is a grammar directive; at parse time we simply parse the content.
// The primary name is the first ident_name; up to three optional annotations
// follow as comma-separated alternatives.
func (p *V10Parser) ParseAssignLHS() (*V10AssignLHSNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V10_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected identifier on LHS, got %s %q", tok.Type, tok.Value))
	}
	name := tok.Value
	p.advance()

	node := &V10AssignLHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Name: name}

	// Up to 3 optional comma-separated annotations.
	for len(node.Annotations) < 3 && p.cur().Type == V10_COMMA {
		saved := p.savePos()
		p.advance() // consume ","

		ann, err := p.parseAssignAnnotation()
		if err != nil {
			// Not a valid annotation — roll back the comma and stop.
			p.restorePos(saved)
			break
		}
		node.Annotations = append(node.Annotations, ann)
	}

	return node, nil
}

// parseAssignAnnotation parses one of: ident_name | cardinality | assign_version
// at the current position.  Uses backtracking to try each alternative.
func (p *V10Parser) parseAssignAnnotation() (V10AssignAnnotation, error) {
	tok := p.cur()

	// assign_version: IDENT starting with "v" + digits
	if tok.Type == V10_IDENT && v10looksLikeVersion(tok.Value) {
		ver, err := p.ParseAssignVersion()
		if err == nil {
			return V10AssignAnnotation{Version: ver}, nil
		}
	}

	// cardinality: INTEGER ".." ( INTEGER | many-keyword )
	if tok.Type == V10_INTEGER {
		saved := p.savePos()
		card, err := p.ParseCardinality()
		if err == nil {
			return V10AssignAnnotation{Cardin: card}, nil
		}
		p.restorePos(saved)
	}

	// ident_name: plain identifier
	if tok.Type == V10_IDENT {
		name := tok.Value
		p.advance()
		return V10AssignAnnotation{IdentName: name}, nil
	}

	return V10AssignAnnotation{}, p.errAt(
		fmt.Sprintf("expected ident, cardinality, or version annotation, got %s %q", tok.Type, tok.Value))
}

// ---------- assign_rhs ----------

// ParseAssignRHS parses:  assign_rhs = constant | calc_unit
//
// Strategy: try constant first (it handles all pure literal tokens); if the
// leading token is not a literal, parse calc_unit (which handles identifier
// references and expressions).
func (p *V10Parser) ParseAssignRHS() (*V10AssignRHSNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Tokens that unambiguously begin a constant literal.
	switch tok.Type {
	case V10_REGEXP, V10_REGEXP_DECL,
		V10_TRUE, V10_FALSE, V10_NULL,
		V10_NAN, V10_INFINITY,
		V10_STRING, V10_EMPTY_STR_D, V10_EMPTY_STR_S, V10_EMPTY_STR_T:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: c}, nil

	// Signed or unsigned numeric literals — could be a constant or
	// the start of a numeric expression in calc_unit.  Try constant first
	// (simpler); backtrack to calc_unit if a subsequent expression operator follows.
	case V10_INTEGER, V10_DECIMAL:
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil {
			// Only commit if what follows is an assignment/EOL context, not an operator.
			if !v10isExprContinuation(p.cur().Type) {
				return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: c}, nil
			}
		}
		p.restorePos(saved)

	case V10_PLUS, V10_MINUS:
		next := p.peek(1)
		if next.Type == V10_INTEGER || next.Type == V10_DECIMAL || next.Type == V10_INFINITY {
			saved := p.savePos()
			if c, err := p.ParseConstant(); err == nil {
				if !v10isExprContinuation(p.cur().Type) {
					return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: c}, nil
				}
			}
			p.restorePos(saved)
		}
	}

	// EXTEND<assign_rhs> — array_final | object_final | table_final
	// These structural forms start with "[", "[]", UNIFORM, or TYPE_OF.
	switch tok.Type {
	case V10_LBRACKET, V10_EMPTY_ARR:
		// Try table_final → object_final → array_final (most-to-least restrictive).
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: of}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if af, err := p.ParseArrayFinal(); err == nil {
			return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: af}, nil
		}
		p.restorePos(saved)

	case V10_UNIFORM:
		// UNIFORM string<...> → table_final;  UNIFORM INFER<...> → object_final.
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: of}, nil
		}
		p.restorePos(saved)

	case V10_TYPE_OF:
		// TYPE_OF can start array_final or table_final.
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if af, err := p.ParseArrayFinal(); err == nil {
			return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: af}, nil
		}
		p.restorePos(saved)
	}

	// General case: parse as calc_unit.
	cu, err := p.ParseCalcUnit()
	if err != nil {
		return nil, err
	}
	return &V10AssignRHSNode{v10BaseNode: v10BaseNode{Line: line, Col: col}, Value: cu}, nil
}

// v10isExprContinuation returns true when tok would continue an arithmetic/
// logic expression — meaning the preceding number literal is actually the
// start of a larger calc_unit rather than a bare constant.
func v10isExprContinuation(t V10TokenType) bool {
	switch t {
	case V10_PLUS, V10_MINUS, V10_STAR, V10_SLASH, V10_PERCENT, V10_POW,
		V10_AMP, V10_PIPE, V10_CARET,
		V10_AMP_AMP, V10_PIPE_PIPE,
		V10_GT, V10_GEQ, V10_LT, V10_LEQ, V10_NEQ, V10_EQEQ, V10_MATCH_OP:
		return true
	}
	return false
}

// ---------- assignment ----------

// ParseAssignment parses:  assignment = assign_lhs assign_oper assign_rhs
func (p *V10Parser) ParseAssignment() (*V10AssignmentNode, error) {
	line, col := p.cur().Line, p.cur().Col

	lhs, err := p.ParseAssignLHS()
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

	return &V10AssignmentNode{
		v10BaseNode: v10BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		RHS:         rhs,
	}, nil
}
