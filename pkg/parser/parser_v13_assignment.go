// parser_v13_assignment.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/03_assignment.sqg.
//
// V13 changes vs V12:
//   - assign_version: the "v" prefix is optional (V12 required it).
//     A bare digit sequence like "1" or "1.0" is accepted as a version.
//   - type_ref = ident_name "." "@type"  can appear in assign_rhs
//     (defined in parser_v13_operators.go, used here)
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

// V13AssignOperKind classifies the assignment operator.
type V13AssignOperKind int

const (
	V13AssignMutable     V13AssignOperKind = iota // =
	V13AssignImmutable                            // :
	V13AssignReadOnlyRef                          // :~
	V13IncrAddImmutable                           // +:
	V13IncrSubImmutable                           // -:
	V13IncrMulImmutable                           // *:
	V13IncrDivImmutable                           // /:
	V13IncrAddMutable                             // +=
	V13IncrSubMutable                             // -=
	V13IncrMulMutable                             // *=
	V13IncrDivMutable                             // /=
)

// V13AssignOperNode  assign_oper = incr_assign_immutable | incr_assign_mutable | equal_assign
type V13AssignOperNode struct {
	V13BaseNode
	Kind  V13AssignOperKind
	Value string
}

// V13AssignVersionNode  assign_version = [ "v" ] digits { "." digits }
// V13 makes the leading "v" optional.
type V13AssignVersionNode struct {
	V13BaseNode
	Raw string // e.g. "v1.0.2" or "1.0.2"
}

// V13AssignAnnotation is one comma-separated annotation on an LHS:
// either another ident_name, a cardinality, or an assign_version.
type V13AssignAnnotation struct {
	IdentName string
	Cardin    *V13CardinalityNode
	Version   *V13AssignVersionNode
}

// V13AssignLHSNode  assign_lhs = UNIQUE<ident_name [, annotation…]>
type V13AssignLHSNode struct {
	V13BaseNode
	Name        string
	Annotations []V13AssignAnnotation
}

// V13AssignRHSNode  assign_rhs = constant | calc_unit
type V13AssignRHSNode struct {
	V13BaseNode
	Value V13Node // *V13ConstantNode | *V13CalcUnitNode | structural form
}

// V13AssignmentNode  assignment = assign_lhs assign_oper assign_rhs
type V13AssignmentNode struct {
	V13BaseNode
	LHS  *V13AssignLHSNode
	Oper *V13AssignOperNode
	RHS  *V13AssignRHSNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (03_assignment.sqg)
// =============================================================================

// ---------- assign_oper ----------

// ParseAssignOper parses:
//
//	assign_oper = incr_assign_immutable | incr_assign_mutable | equal_assign
func (p *V13Parser) ParseAssignOper() (*V13AssignOperNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var kind V13AssignOperKind
	var val string

	switch tok.Type {
	case V13_EQ:
		kind, val = V13AssignMutable, "="
	case V13_COLON:
		kind, val = V13AssignImmutable, ":"
	case V13_READONLY:
		kind, val = V13AssignReadOnlyRef, ":~"
	case V13_IADD_IMM:
		kind, val = V13IncrAddImmutable, "+:"
	case V13_ISUB_IMM:
		kind, val = V13IncrSubImmutable, "-:"
	case V13_IMUL_IMM:
		kind, val = V13IncrMulImmutable, "*:"
	case V13_IDIV_IMM:
		kind, val = V13IncrDivImmutable, "/:"
	case V13_EXTEND_ASSIGN:
		kind, val = V13IncrAddMutable, "+="
	case V13_ISUB_ASSIGN:
		kind, val = V13IncrSubMutable, "-="
	case V13_IMUL_ASSIGN:
		kind, val = V13IncrMulMutable, "*="
	case V13_IDIV_ASSIGN:
		kind, val = V13IncrDivMutable, "/="
	default:
		return nil, p.errAt(fmt.Sprintf("expected assignment operator, got %s %q", tok.Type, tok.Value))
	}

	p.advance()
	return &V13AssignOperNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Kind: kind, Value: val}, nil
}

// ---------- assign_version ----------

// ParseAssignVersion parses:  assign_version = [ "v" | "V" ] digits { "." digits }
//
// V13: the "v" prefix is optional.  Source may be:
//
//	"v1", "v1.0", "1", "1.0", "V12"
func (p *V13Parser) ParseAssignVersion() (*V13AssignVersionNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	var sb strings.Builder

	switch tok.Type {
	case V13_IDENT:
		if V13looksLikeVersionIdent(tok.Value) {
			// IDENT of the form  v<digits>  or  V<digits>
			sb.WriteString(tok.Value)
			p.advance()
		} else {
			return nil, p.errAt(fmt.Sprintf("expected version identifier, got %q", tok.Value))
		}
	case V13_INTEGER:
		// Bare digit sequence — V13 extension.
		sb.WriteString(tok.Value)
		p.advance()
	default:
		return nil, p.errAt(fmt.Sprintf("expected version, got %s %q", tok.Type, tok.Value))
	}

	// Consume optional ".digits" continuations.
	for p.cur().Type == V13_DOT {
		saved := p.savePos()
		p.advance()
		if p.cur().Type != V13_INTEGER {
			p.restorePos(saved)
			break
		}
		sb.WriteByte('.')
		sb.WriteString(p.cur().Value)
		p.advance()
	}

	return &V13AssignVersionNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Raw:         sb.String(),
	}, nil
}

// V13looksLikeVersionIdent reports whether s starts with 'v'/'V' followed by
// at least one digit.
func V13looksLikeVersionIdent(s string) bool {
	if len(s) < 2 || (s[0] != 'v' && s[0] != 'V') {
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
//	assign_lhs = UNIQUE<ident_name [, (ident_name | cardinality | assign_version)]…>
func (p *V13Parser) ParseAssignLHS() (*V13AssignLHSNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col
	if tok.Type != V13_IDENT {
		return nil, p.errAt(fmt.Sprintf("expected identifier on LHS, got %s %q", tok.Type, tok.Value))
	}
	name := tok.Value
	p.advance()

	node := &V13AssignLHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Name: name}

	for len(node.Annotations) < 3 && p.cur().Type == V13_COMMA {
		saved := p.savePos()
		p.advance()
		ann, err := p.parseV13AssignAnnotation()
		if err != nil {
			p.restorePos(saved)
			break
		}
		node.Annotations = append(node.Annotations, ann)
	}
	return node, nil
}

// parseV13AssignAnnotation parses one of: ident_name | cardinality | assign_version
func (p *V13Parser) parseV13AssignAnnotation() (V13AssignAnnotation, error) {
	tok := p.cur()

	// assign_version: IDENT starting with v/V + digits, or bare INTEGER (V13)
	if (tok.Type == V13_IDENT && V13looksLikeVersionIdent(tok.Value)) || tok.Type == V13_INTEGER {
		saved := p.savePos()
		// Try cardinality first for INTEGER (integer ".." ...)
		if tok.Type == V13_INTEGER {
			if card, err := p.ParseCardinality(); err == nil {
				return V13AssignAnnotation{Cardin: card}, nil
			}
			p.restorePos(saved)
		}
		if ver, err := p.ParseAssignVersion(); err == nil {
			return V13AssignAnnotation{Version: ver}, nil
		}
		p.restorePos(saved)
	}

	if tok.Type == V13_IDENT {
		name := tok.Value
		p.advance()
		return V13AssignAnnotation{IdentName: name}, nil
	}

	return V13AssignAnnotation{}, p.errAt(
		fmt.Sprintf("expected ident, cardinality, or version annotation, got %s %q", tok.Type, tok.Value))
}

// ---------- assign_rhs ----------

// V13isExprContinuation returns true when tok would continue an arithmetic/
// logic expression after a numeric literal.
func V13isExprContinuation(t V13TokenType) bool {
	switch t {
	case V13_PLUS, V13_MINUS, V13_STAR, V13_SLASH, V13_PERCENT, V13_POW,
		V13_AMP, V13_PIPE, V13_CARET,
		V13_AMP_AMP, V13_PIPE_PIPE,
		V13_GT, V13_GEQ, V13_LT, V13_LEQ, V13_NEQ, V13_EQEQ, V13_MATCH_OP:
		return true
	}
	return false
}

// ParseAssignRHS parses:  assign_rhs = constant | calc_unit | structural forms
//
// Structural forms (via EXTEND<assign_rhs>) are tried before falling back to
// calc_unit so that they bind eagerly.
func (p *V13Parser) ParseAssignRHS() (*V13AssignRHSNode, error) {
	tok := p.cur()
	line, col := tok.Line, tok.Col

	// Pure-literal tokens that unambiguously begin a constant.
	switch tok.Type {
	case V13_REGEXP,
		V13_TRUE, V13_FALSE, V13_NULL,
		V13_NAN, V13_INFINITY,
		V13_STRING, V13_EMPTY_STR_D, V13_EMPTY_STR_S, V13_EMPTY_STR_T:
		c, err := p.ParseConstant()
		if err != nil {
			return nil, err
		}
		return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: c}, nil

	case V13_INTEGER, V13_DECIMAL:
		saved := p.savePos()
		if c, err := p.ParseConstant(); err == nil {
			if !V13isExprContinuation(p.cur().Type) {
				return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: c}, nil
			}
		}
		p.restorePos(saved)

	case V13_PLUS, V13_MINUS:
		next := p.peek(1)
		if next.Type == V13_INTEGER || next.Type == V13_DECIMAL || next.Type == V13_INFINITY {
			saved := p.savePos()
			if c, err := p.ParseConstant(); err == nil {
				if !V13isExprContinuation(p.cur().Type) {
					return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: c}, nil
				}
			}
			p.restorePos(saved)
		}
	}

	// ---- EXTEND<assign_rhs> structural forms ----

	// V13 structures: set {…}, enum ENUM[…], bitfield BITFIELD …[…]
	// These are tried before array_final to avoid mis-parsing { as array element.
	switch tok.Type {
	case V13_LBRACE:
		saved := p.savePos()
		if sf, err := p.ParseSetFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: sf}, nil
		}
		p.restorePos(saved)

	case V13_ENUM:
		saved := p.savePos()
		if ef, err := p.ParseEnumFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: ef}, nil
		}
		p.restorePos(saved)

	case V13_BITFIELD:
		saved := p.savePos()
		if bf, err := p.ParseBitfieldFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: bf}, nil
		}
		p.restorePos(saved)
	}

	// Array / object / table structural forms.
	switch tok.Type {
	case V13_LBRACKET, V13_EMPTY_ARR:
		// Try table_final → tree variants → object_final → array_final (most-to-least specific).
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)

		saved = p.savePos()
		if kf, err := p.ParseKeyedTreeFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: kf}, nil
		}
		p.restorePos(saved)

		saved = p.savePos()
		if sf, err := p.ParseSortedTreeFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: sf}, nil
		}
		p.restorePos(saved)

		saved = p.savePos()
		if stf, err := p.ParseStringTreeFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: stf}, nil
		}
		p.restorePos(saved)

		saved = p.savePos()
		if trf, err := p.ParseTreeFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: trf}, nil
		}
		p.restorePos(saved)

		saved = p.savePos()
		if gf, err := p.ParseGraphFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: gf}, nil
		}
		p.restorePos(saved)

		saved = p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: of}, nil
		}
		p.restorePos(saved)

		if af, err := p.ParseArrayFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: af}, nil
		}

	case V13_UNIFORM:
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if of, err := p.ParseObjectFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: of}, nil
		}
		p.restorePos(saved)

	case V13_TYPE_OF:
		saved := p.savePos()
		if tf, err := p.ParseTableFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: tf}, nil
		}
		p.restorePos(saved)
		saved = p.savePos()
		if af, err := p.ParseArrayFinal(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: af}, nil
		}
		p.restorePos(saved)

	// Function forms.
	case V13_RETURN_STMT:
		saved := p.savePos()
		if rfu, err := p.ParseReturnFuncUnit(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: rfu}, nil
		}
		p.restorePos(saved)

	case V13_LPAREN:
		saved := p.savePos()
		if fu, err := p.ParseFuncUnit(); err == nil {
			return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: fu}, nil
		}
		p.restorePos(saved)
	}

	// General case: calc_unit.
	cu, err := p.ParseCalcUnit()
	if err != nil {
		return nil, err
	}
	return &V13AssignRHSNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Value: cu}, nil
}

// ---------- assignment ----------

// ParseAssignment parses:  assignment = assign_lhs assign_oper assign_rhs
func (p *V13Parser) ParseAssignment() (*V13AssignmentNode, error) {
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
	return &V13AssignmentNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		LHS:         lhs,
		Oper:        oper,
		RHS:         rhs,
	}, nil
}
