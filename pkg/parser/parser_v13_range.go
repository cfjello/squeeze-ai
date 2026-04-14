// parser_v13_range.go — Phase 2 AST nodes and Phase 3 parse methods for
// the Squeeze V13 grammar rule set defined in spec/08_range.sqg.
//
// New in V13 (no V12 equivalent rules):
//   - range_oper        ".."
//   - validate_oper     "=~"
//   - num_range_valid   TYPE_OF boolean<numeric_expr =~ func_fixed_num_range>
//   - date_range        ( date | TYPE_OF date<ident_ref> ) ".." ( date | TYPE_OF date<ident_ref> )
//   - date_range_valid  TYPE_OF boolean<date =~ date_range>
//   - time_range        ( time | TYPE_OF time<ident_ref> ) ".." ( time | TYPE_OF time<ident_ref> )
//   - time_range_val    TYPE_OF boolean<time =~ time_range>
//   - regexp_valid      TYPE_OF boolean<string_quoted =~ regexp>
//
// NOTE: range_oper and validate_oper map to existing tokens V13_DOTDOT and
// V13_MATCH_OP respectively — no new token types are required.
package parser

import "fmt"

// =============================================================================
// PHASE 2 — AST NODE TYPES  (08_range.sqg)
// =============================================================================

// V13NumRangeValidNode  num_range_valid = TYPE_OF boolean< numeric_expr =~ func_fixed_num_range >
type V13NumRangeValidNode struct {
	V13BaseNode
	Expr     *V13NumericExprNode
	RangeRef *V13IdentRefNode // func_fixed_num_range is referenced by ident_ref
}

// V13DateRangeSide is one side of a date_range: either a raw date or TYPE_OF date<ident_ref>
type V13DateRangeSide struct {
	Date    *V13DateNode     // non-nil when a literal date is used
	TypedOf *V13IdentRefNode // non-nil when TYPE_OF date<ident_ref>
}

// V13DateRangeNode  date_range = ( date | TYPE_OF date<ident_ref> ) ".." ( date | TYPE_OF date<ident_ref> )
type V13DateRangeNode struct {
	V13BaseNode
	Lo V13DateRangeSide
	Hi V13DateRangeSide
}

// V13DateRangeValidNode  date_range_valid = TYPE_OF boolean< date =~ date_range >
type V13DateRangeValidNode struct {
	V13BaseNode
	Date  *V13DateNode
	Range *V13DateRangeNode
}

// V13TimeRangeSide is one side of a time_range.
type V13TimeRangeSide struct {
	Time    *V13TimeNode
	TypedOf *V13IdentRefNode
}

// V13TimeRangeNode  time_range = ( time | TYPE_OF time<ident_ref> ) ".." ( time | TYPE_OF time<ident_ref> )
type V13TimeRangeNode struct {
	V13BaseNode
	Lo V13TimeRangeSide
	Hi V13TimeRangeSide
}

// V13TimeRangeValidNode  time_range_val = TYPE_OF boolean< time =~ time_range >
type V13TimeRangeValidNode struct {
	V13BaseNode
	Time  *V13TimeNode
	Range *V13TimeRangeNode
}

// V13RegexpValidNode  regexp_valid = TYPE_OF boolean< string_quoted =~ regexp >
type V13RegexpValidNode struct {
	V13BaseNode
	Str    *V13StringQuotedNode
	Regexp *V13RegexpNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (08_range.sqg)
// =============================================================================

// ---------- helpers ----------

// parseDateRangeSide parses one side of a date_range:  date | TYPE_OF date<ident_ref>
func (p *V13Parser) parseDateRangeSide() (V13DateRangeSide, error) {
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance() // consume TYPE_OF
		if _, err := p.expectLit("date"); err != nil {
			p.restorePos(saved)
			return V13DateRangeSide{}, err
		}
		if _, err := p.expect(V13_LT); err != nil {
			p.restorePos(saved)
			return V13DateRangeSide{}, err
		}
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			return V13DateRangeSide{}, err
		}
		if _, err := p.expect(V13_GT); err != nil {
			p.restorePos(saved)
			return V13DateRangeSide{}, err
		}
		return V13DateRangeSide{TypedOf: ref}, nil
	}
	d, err := p.ParseDate()
	if err != nil {
		return V13DateRangeSide{}, err
	}
	return V13DateRangeSide{Date: d}, nil
}

// parseTimeRangeSide parses one side of a time_range:  time | TYPE_OF time<ident_ref>
func (p *V13Parser) parseTimeRangeSide() (V13TimeRangeSide, error) {
	if p.cur().Type == V13_TYPE_OF {
		saved := p.savePos()
		p.advance() // consume TYPE_OF
		if _, err := p.expectLit("time"); err != nil {
			p.restorePos(saved)
			return V13TimeRangeSide{}, err
		}
		if _, err := p.expect(V13_LT); err != nil {
			p.restorePos(saved)
			return V13TimeRangeSide{}, err
		}
		ref, err := p.ParseIdentRef()
		if err != nil {
			p.restorePos(saved)
			return V13TimeRangeSide{}, err
		}
		if _, err := p.expect(V13_GT); err != nil {
			p.restorePos(saved)
			return V13TimeRangeSide{}, err
		}
		return V13TimeRangeSide{TypedOf: ref}, nil
	}
	t, err := p.ParseTime()
	if err != nil {
		return V13TimeRangeSide{}, err
	}
	return V13TimeRangeSide{Time: t}, nil
}

// ---------- Public parsers ----------

// ParseNumRangeValid parses:
//
//	num_range_valid = TYPE_OF boolean< numeric_expr =~ func_fixed_num_range >
func (p *V13Parser) ParseNumRangeValid() (*V13NumRangeValidNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_TYPE_OF); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("boolean"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	expr, err := p.ParseNumericExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_MATCH_OP); err != nil {
		return nil, err
	}
	rangeRef, err := p.ParseIdentRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13NumRangeValidNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Expr:        expr,
		RangeRef:    rangeRef,
	}, nil
}

// ParseDateRange parses:
//
//	date_range = ( date | TYPE_OF date<ident_ref> ) ".." ( date | TYPE_OF date<ident_ref> )
func (p *V13Parser) ParseDateRange() (*V13DateRangeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	lo, err := p.parseDateRangeSide()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_DOTDOT); err != nil {
		return nil, err
	}
	hi, err := p.parseDateRangeSide()
	if err != nil {
		return nil, err
	}
	return &V13DateRangeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// ParseDateRangeValid parses:
//
//	date_range_valid = TYPE_OF boolean< date =~ date_range >
func (p *V13Parser) ParseDateRangeValid() (*V13DateRangeValidNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_TYPE_OF); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("boolean"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	d, err := p.ParseDate()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_MATCH_OP); err != nil {
		return nil, err
	}
	rng, err := p.ParseDateRange()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13DateRangeValidNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Date:        d,
		Range:       rng,
	}, nil
}

// ParseTimeRange parses:
//
//	time_range = ( time | TYPE_OF time<ident_ref> ) ".." ( time | TYPE_OF time<ident_ref> )
func (p *V13Parser) ParseTimeRange() (*V13TimeRangeNode, error) {
	line, col := p.cur().Line, p.cur().Col
	lo, err := p.parseTimeRangeSide()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_DOTDOT); err != nil {
		return nil, err
	}
	hi, err := p.parseTimeRangeSide()
	if err != nil {
		return nil, err
	}
	return &V13TimeRangeNode{V13BaseNode: V13BaseNode{Line: line, Col: col}, Lo: lo, Hi: hi}, nil
}

// ParseTimeRangeValid parses:
//
//	time_range_val = TYPE_OF boolean< time =~ time_range >
func (p *V13Parser) ParseTimeRangeValid() (*V13TimeRangeValidNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_TYPE_OF); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("boolean"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	t, err := p.ParseTime()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_MATCH_OP); err != nil {
		return nil, err
	}
	rng, err := p.ParseTimeRange()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13TimeRangeValidNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Time:        t,
		Range:       rng,
	}, nil
}

// ParseRegexpValid parses:
//
//	regexp_valid = TYPE_OF boolean< string_quoted =~ regexp >
func (p *V13Parser) ParseRegexpValid() (*V13RegexpValidNode, error) {
	line, col := p.cur().Line, p.cur().Col
	if _, err := p.expect(V13_TYPE_OF); err != nil {
		return nil, err
	}
	if _, err := p.expectLit("boolean"); err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_LT); err != nil {
		return nil, err
	}
	s, err := p.ParseStringQuoted()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_MATCH_OP); err != nil {
		return nil, err
	}
	r, err := p.ParseRegexp()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(V13_GT); err != nil {
		return nil, err
	}
	return &V13RegexpValidNode{
		V13BaseNode: V13BaseNode{Line: line, Col: col},
		Str:         s,
		Regexp:      r,
	}, nil
}

var _ = fmt.Sprintf
