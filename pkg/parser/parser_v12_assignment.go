// parser_V12_assignment.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V12 grammar rule set defined in spec/03_assignment.sqg.
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

// V12AssignOperKind classifies the assignment operator.
type V12AssignOperKind int

const (
	// equal-assign variants
	V12AssignMutable     V12AssignOperKind = iota // =
	V12AssignImmutable                            // :
	V12AssignReadOnlyRef                          // :~
	// incremental-immutable variants (+: -: *: /:)
	V12IncrAddImmutable // +:
	V12IncrSubImmutable // -:
	V12IncrMulImmutable // *:
	V12IncrDivImmutable // /:
	// incremental-mutable variants (+= -= *= /=)
	V12IncrAddMutable // +=
	V12IncrSubMutable // -=
	V12IncrMulMutable // *=
	V12IncrDivMutable // /=
)

// V12AssignOperNode  assign_oper = incr_assign_immutable | incr_assign_mutable | equal_assign
type V12AssignOperNode struct {
	V12BaseNode
	Kind  V12AssignOperKind
	Value string // verbatim source text of the operator
}

// V12AssignVersionNode  assign_version = "v" digits { "." digits }
// e.g. "v1", "v1.0", "v2.3.14"
type V12AssignVersionNode struct {
	V12BaseNode
	Raw string // e.g. "v1.0.2"
}

// V12AssignAnnotation is one optional comma-separated LHS annotation:
// either another ident_name, a cardinality, or an assign_version.
// Exactly one field is non-nil / non-empty.
type V12AssignAnnotation struct {
	IdentName string                // plain identifier annotation
	Cardin    *V12CardinalityNode   // cardinality  e.g. 0..many
	Version   *V12AssignVersionNode // version      e.g. v1.0
}

// V12AssignLHSNode  assign_lhs = UNIQUE<ident_name [, annotation [, annotation [, annotation]]]>
// UNIQUE is a grammar-level directive (uniqueness constraint enforced by the
// runtime).  At parse time the content is just: ident_name + up to 3 optional
// comma-prefixed annotations.
type V12AssignLHSNode struct {
	V12BaseNode
	Name        string                // the primary ident_name
	Annotations []V12AssignAnnotation // 0..3 entries
}

// V12AssignRHSNode  assign_rhs = constant | calc_unit
type V12AssignRHSNode struct {
	V12BaseNode
	Value V12Node // *V12ConstantNode | *V12CalcUnitNode
}

// V12AssignmentNode  assignment = assign_lhs assign_oper assign_rhs
type V12AssignmentNode struct {
	V12BaseNode
	LHS  *V12AssignLHSNode
	Oper *V12AssignOperNode
	RHS  *V12AssignRHSNode
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
func (p *V12Parser) ParseAssignOper() (*V12AssignOperNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var kind V12AssignOperKind
	var val string

	switch tok.Type {
	// equal-assign
	case V12_EQ:
		kind, val = V12AssignMutable, "="
	case V12_COLON:
		kind, val = V12AssignImmutable, ":"
	case V12_READONLY:
		kind, val = V12AssignReadOnlyRef, ":~"

	// incr_assign_immutable
	case V12_IADD_IMM:
		kind, val = V12IncrAddImmutable, "+:"
	case V12_ISUB_IMM:
		kind, val = V12IncrSubImmutable, "-:"
	case V12_IMUL_IMM:
		kind, val = V12IncrMulImmutable, "*:"
	case V12_IDIV_IMM:
		kind, val = V12IncrDivImmutable, "/:"

	// incr_assign_mutable
	case V12_EXTEND_ASSIGN:
		kind, val = V12IncrAddMutable, "+="
	case V12_ISUB_ASSIGN:
		kind, val = V12IncrSubMutable, "-="
	case V12_IMUL_ASSIGN:
		kind, val = V12IncrMulMutable, "*="
	case V12_IDIV_ASSIGN:
		kind, val = V12IncrDivMutable, "/="

	default:
		return nil, p.errAt(fmt.Sprintf("expected assignment operator, got %s %q", tok.Type, tok.Value))
	}

	p.advance()
	return &V12AssignOperNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Kind: kind, Value: val}, nil
}

// ---------- assign_version ----------

// ParseAssignVersion parses:  assign_version = "v" digits { "." digits }
//
// In source text this appears as an identifier that starts with 'v' followed
// immediately by digits (e.g. "v1", "V12"), then optionally ".digits" chains.
// The lexer merges "v1" into a single IDENT token; subsequent ".2.3" come as
// separate DOT + INTEGER tokens.
func (p *V12Parser) ParseAssignVersion() (*V12AssignVersionNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V12_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected version (e.g. v1), got %s %q", tok.Type, tok.Value))
	}
	if !V12looksLikeVersion(tok.Value) {
		return nil, p.errAt(fmt.Sprintf("expected version identifier (v<digits>), got %q", tok.Value))
	}

	var sb strings.Builder
	sb.WriteString(tok.Value) // e.g. "v1"
	p.advance()

	// Consume optional ".digits" continuations.
	for p.cur().Type == V12_DOT {
		saved := p.savePos()
		p.advance() // consume "."
		if p.cur().Type != V12_INTEGER {
			p.restorePos(saved)
			break
		}
		sb.WriteByte('.')
		sb.WriteString(p.cur().Value)
		p.advance()
	}

	return &V12AssignVersionNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		Raw:         sb.String(),
	}, nil
}

// V12looksLikeVersion reports whether s has the form "v<one-or-more-digits>".
func V12looksLikeVersion(s string) bool {
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
func (p *V12Parser) ParseAssignLHS() (*V12AssignLHSNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	if tok.Type != V12_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected identifier on LHS, got %s %q", tok.Type, tok.Value))
	}
	name := tok.Value
	p.advance()

	node := &V12AssignLHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Name: name}

	// Up to 3 optional comma-separated annotations.
	for len(node.Annotations) < 3 && p.cur().Type == V12_COMMA {
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
func (p *V12Parser) parseAssignAnnotation() (V12AssignAnnotation, error) {
	tok := p.cur()

	// assign_version: IDENT starting with "v" + digits
	if tok.Type == V12_IDENT && V12looksLikeVersion(tok.Value) {
		ver, err := p.ParseAssignVersion()
		if err == nil {
			return V12AssignAnnotation{Version: ver}, nil
		}
	}

	// cardinality: INTEGER ".." ( INTEGER | many-keyword )
	if tok.Type == V12_INTEGER {
		saved := p.savePos()
		card, err := p.ParseCardinality()
		if err == nil {
			return V12AssignAnnotation{Cardin: card}, nil
		}
		p.restorePos(saved)
	}

	// ident_name: plain identifier
	if tok.Type == V12_IDENT {
		name := tok.Value
		p.advance()
		return V12AssignAnnotation{IdentName: name}, nil
	}

	return V12AssignAnnotation{}, p.errAt(
		fmt.Sprintf("expected ident, cardinality, or version annotation, got %s %q", tok.Type, tok.Value))
}

// ---------- assign_rhs ----------

// ParseAssignRHS parses:  assign_rhs = constant | calc_unit
//
// Strategy: try constant first (it handles all pure literal tokens); if the
// leading token is not a literal, parse calc_unit (which handles identifier
// references and expressions).
func (p *V12Parser) ParseAssignRHS() (*V12AssignRHSNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Tokens that unambiguously begin a constant literal.
	switch tok.Type {
	case V12_REGEXP, V12_REGEXP_DECL,
		V12_TRUE, V12_FALSE, V12_NULL,
		V12_NAN, V12_INFINITY,
		V12_STRING, V12_EMPTY_STR_D, V12_EMPTY_STR_S, V12_EMPTY_STR_T:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: c}, nil

	// Signed or unsigned numeric literals — could be a constant or
	// the start of a numeric expression in calc_unit.  Try constant first
	// (simpler); backtrack to calc_unit if a subsequent expression operator follows.
	case V12_INTEGER, V12_DECIMAL:
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil {
			// Only commit if what follows is an assignment/EOL context, not an operator.
			if !V12isExprContinuation(p.cur().Type) {
				return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: c}, nil
			}
		}
		p.restorePos(saved)

	case V12_PLUS, V12_MINUS:
		next := p.peek(1)
		if next.Type == V12_INTEGER || next.Type == V12_DECIMAL || next.Type == V12_INFINITY {
			saved := p.savePos()
			if c, err := p.ParseConstant(); err == nil {
				if !V12isExprContinuation(p.cur().Type) {
					return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: c}, nil
				}
			}
			p.restorePos(saved)
		}
	}

	// EXTEND<assign_rhs> — array_final | object_final | table_final
	// These structural forms start with "[", "[]", UNIFORM, or TYPE_OF.
	switch tok.Type {
	case V12_LBRACKET, V12_EMPTY_ARR:
		// Try table_final → object_final → array_final (most-to-least restrictive).
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: of}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if af, err := p.ParseArrayFinal(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: af}, nil
		}
		p.restorePos(saved)

	case V12_UNIFORM:
		// UNIFORM string<...> → table_final;  UNIFORM INFER<...> → object_final.
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: of}, nil
		}
		p.restorePos(saved)

	case V12_TYPE_OF:
		// TYPE_OF can start array_final or table_final.
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if af, err := p.ParseArrayFinal(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: af}, nil
		}
		p.restorePos(saved)
	}

	// EXTEND<assign_rhs>: func forms — return_func_unit "<-" or func_unit "{" / "("
	switch tok.Type {
	case V12_RETURN_STMT:
		// return_func_unit = "<-" func_unit
		saved := p.savePos()
		if rfu, err := p.ParseReturnFuncUnit(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: rfu}, nil
		}
		p.restorePos(saved)
	case V12_LBRACE, V12_LPAREN:
		// func_unit = "{" ... "}" or "(" ... ")"
		saved := p.savePos()
		if fu, err := p.ParseFuncUnit(); err == nil {
			return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: fu}, nil
		}
		p.restorePos(saved)
	}

	// General case: parse as calc_unit.
	cu, err := p.ParseCalcUnit()
	if err != nil {
		return nil, err
	}
	return &V12AssignRHSNode{V12BaseNode: V12BaseNode{Line: line, Col: col}, Value: cu}, nil
}

// V12isExprContinuation returns true when tok would continue an arithmetic/
// logic expression — meaning the preceding number literal is actually the
// start of a larger calc_unit rather than a bare constant.
func V12isExprContinuation(t V12TokenType) bool {
	switch t {
	case V12_PLUS, V12_MINUS, V12_STAR, V12_SLASH, V12_PERCENT, V12_POW,
		V12_AMP, V12_PIPE, V12_CARET,
		V12_AMP_AMP, V12_PIPE_PIPE,
		V12_GT, V12_GEQ, V12_LT, V12_LEQ, V12_NEQ, V12_EQEQ, V12_MATCH_OP:
		return true
	}
	return false
}

// ---------- assignment ----------

// ParseAssignment parses:  assignment = assign_lhs assign_oper assign_rhs
func (p *V12Parser) ParseAssignment() (*V12AssignmentNode, error) {
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

	return &V12AssignmentNode{
		V12BaseNode: V12BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		RHS:         rhs,
	}, nil
}
