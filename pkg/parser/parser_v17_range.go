// parser_v17_range.go — Parse methods for spec/08_range.sqg.
//
// Covered rules:
//
//	range_oper, validate_oper,
//	num_range_valid,
//	date_range, date_range_valid,
//	time_range, time_range_valid
//
// EXTEND<collection> = | date_range | time_range
// EXTEND<condition>  = | num_range_valid | date_range_valid | time_range_valid
// EXTEND<statement>  = | date_range | time_range
//
//	(EXTEND wiring is in parser_v17_objects.go and parser_v17_operators.go)
package parser

import "fmt"

// =============================================================================
// PHASE 2 — AST NODE TYPES  (08_range.sqg)
// =============================================================================

// V17RangeOperNode  range_oper = ".."
type V17RangeOperNode struct{ V17BaseNode }

// V17ValidateOperNode  validate_oper = "><"
type V17ValidateOperNode struct{ V17BaseNode }

// V17NumRangeValidNode  num_range_valid = TYPE_OF boolean< numeric_calc >< func_fixed_num_range >
//
// Note: func_fixed_num_range is not defined as a grammar rule — it is parsed
// as ident_ref per the V13 precedent.
type V17NumRangeValidNode struct {
	V17BaseNode
	Expr     *V17NumericCalcNode
	RangeRef *V17IdentRefNode // func_fixed_num_range is referenced by ident_ref
}

// V17DateRangeSide is one side of a date_range:
// either a raw date literal or TYPE_OF date<ident_ref>.
type V17DateRangeSide struct {
	Date    *V17DateNode     // non-nil when a literal date is used
	TypedOf *V17IdentRefNode // non-nil when TYPE_OF date<ident_ref>
}

// V17DateRangeNode  date_range = ( date | TYPE_OF date<ident_ref> ) ".." ( date | TYPE_OF date<ident_ref> )
type V17DateRangeNode struct {
	V17BaseNode
	Lo V17DateRangeSide
	Hi V17DateRangeSide
}

// V17DateRangeValidNode  date_range_valid = TYPE_OF boolean< date >< date_range >
type V17DateRangeValidNode struct {
	V17BaseNode
	Date  *V17DateNode
	Range *V17DateRangeNode
}

// V17TimeRangeSide is one side of a time_range:
// either a raw time literal or TYPE_OF time<ident_ref>.
type V17TimeRangeSide struct {
	Time    *V17TimeNode     // non-nil when a literal time is used
	TypedOf *V17IdentRefNode // non-nil when TYPE_OF time<ident_ref>
}

// V17TimeRangeNode  time_range = ( time | TYPE_OF time<ident_ref> ) ".." ( time | TYPE_OF time<ident_ref> )
type V17TimeRangeNode struct {
	V17BaseNode
	Lo V17TimeRangeSide
	Hi V17TimeRangeSide
}

// V17TimeRangeValidNode  time_range_valid = TYPE_OF boolean< time >< time_range >
type V17TimeRangeValidNode struct {
	V17BaseNode
	Time  *V17TimeNode
	Range *V17TimeRangeNode
}

// =============================================================================
// PHASE 3 — PARSE METHODS  (08_range.sqg)
// =============================================================================

// ---------- terminals ----------

// ParseRangeOper parses range_oper = ".."
func (p *V17Parser) ParseRangeOper() (node *V17RangeOperNode, err error) {
	done := p.debugEnter("range_oper")
	defer func() { done(err == nil) }()
	tok, err := p.matchLit("..")
	if err != nil {
		return nil, fmt.Errorf("range_oper: %w", err)
	}
	return &V17RangeOperNode{V17BaseNode{tok.Line, tok.Col}}, nil
}

// ParseValidateOper parses validate_oper = "><"
//
// Note: "><" is not a single compound token in V17; it is two consecutive
// tokens V17_GT (">") followed by V17_LT ("<").
func (p *V17Parser) ParseValidateOper() (node *V17ValidateOperNode, err error) {
	done := p.debugEnter("validate_oper")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol
	if _, err = p.matchLit(">"); err != nil {
		return nil, fmt.Errorf("validate_oper: expected '>': %w", err)
	}
	if _, err = p.matchLit("<"); err != nil {
		return nil, fmt.Errorf("validate_oper: expected '<' after '>': %w", err)
	}
	return &V17ValidateOperNode{V17BaseNode{line, col}}, nil
}

// ---------- private helpers ----------

// parseDateRangeSide parses one side of a date_range:
//
//	date | TYPE_OF date<ident_ref>
//
// In V17, TYPE_OF is not a keyword token; it is a V17_IDENT with value "TYPE_OF".
func (p *V17Parser) parseDateRangeSide() (V17DateRangeSide, error) {
	if p.peekLit("TYPE_OF") {
		saved := p.savePos()
		if _, err := p.matchKeyword("TYPE_OF"); err != nil {
			p.restorePos(saved)
			return V17DateRangeSide{}, fmt.Errorf("date range side: TYPE_OF: %w", err)
		}
		if _, err := p.expectLit("date"); err != nil {
			p.restorePos(saved)
			return V17DateRangeSide{}, fmt.Errorf("date range side: TYPE_OF: %w", err)
		}
		if _, err := p.matchLit("<"); err != nil {
			p.restorePos(saved)
			return V17DateRangeSide{}, fmt.Errorf("date range side: TYPE_OF date<: %w", err)
		}
		ref, refErr := p.ParseIdentRef()
		if refErr != nil {
			p.restorePos(saved)
			return V17DateRangeSide{}, fmt.Errorf("date range side: TYPE_OF date<ident_ref>: %w", refErr)
		}
		if _, err := p.matchLit(">"); err != nil {
			p.restorePos(saved)
			return V17DateRangeSide{}, fmt.Errorf("date range side: TYPE_OF date<...>: closing '>': %w", err)
		}
		return V17DateRangeSide{TypedOf: ref}, nil
	}
	d, dErr := p.ParseDate()
	if dErr != nil {
		return V17DateRangeSide{}, fmt.Errorf("date range side: %w", dErr)
	}
	return V17DateRangeSide{Date: d}, nil
}

// parseTimeRangeSide parses one side of a time_range:
//
//	time | TYPE_OF time<ident_ref>
func (p *V17Parser) parseTimeRangeSide() (V17TimeRangeSide, error) {
	if p.peekLit("TYPE_OF") {
		saved := p.savePos()
		if _, err := p.matchKeyword("TYPE_OF"); err != nil {
			p.restorePos(saved)
			return V17TimeRangeSide{}, fmt.Errorf("time range side: TYPE_OF: %w", err)
		}
		if _, err := p.expectLit("time"); err != nil {
			p.restorePos(saved)
			return V17TimeRangeSide{}, fmt.Errorf("time range side: TYPE_OF: %w", err)
		}
		if _, err := p.matchLit("<"); err != nil {
			p.restorePos(saved)
			return V17TimeRangeSide{}, fmt.Errorf("time range side: TYPE_OF time<: %w", err)
		}
		ref, refErr := p.ParseIdentRef()
		if refErr != nil {
			p.restorePos(saved)
			return V17TimeRangeSide{}, fmt.Errorf("time range side: TYPE_OF time<ident_ref>: %w", refErr)
		}
		if _, err := p.matchLit(">"); err != nil {
			p.restorePos(saved)
			return V17TimeRangeSide{}, fmt.Errorf("time range side: TYPE_OF time<...>: closing '>': %w", err)
		}
		return V17TimeRangeSide{TypedOf: ref}, nil
	}
	t, tErr := p.ParseTime()
	if tErr != nil {
		return V17TimeRangeSide{}, fmt.Errorf("time range side: %w", tErr)
	}
	return V17TimeRangeSide{Time: t}, nil
}

// ---------- public parsers ----------

// ParseNumRangeValid parses:
//
//	num_range_valid = TYPE_OF boolean< numeric_calc >< func_fixed_num_range >
//
// In source text: TYPE_OF boolean<numeric_expr >< rangeRef>
func (p *V17Parser) ParseNumRangeValid() (node *V17NumRangeValidNode, err error) {
	done := p.debugEnter("num_range_valid")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.expectLit("TYPE_OF"); err != nil {
		return nil, fmt.Errorf("num_range_valid: %w", err)
	}
	if _, err = p.expectLit("boolean"); err != nil {
		return nil, fmt.Errorf("num_range_valid: %w", err)
	}
	if _, err = p.matchLit("<"); err != nil {
		return nil, fmt.Errorf("num_range_valid: opening '<': %w", err)
	}
	expr, exprErr := p.ParseNumericCalc()
	if exprErr != nil {
		return nil, fmt.Errorf("num_range_valid: %w", exprErr)
	}
	// validate_oper is "><" — two tokens: V17_GT then V17_LT
	if _, err = p.matchLit(">"); err != nil {
		return nil, fmt.Errorf("num_range_valid: validate_oper '>': %w", err)
	}
	if _, err = p.matchLit("<"); err != nil {
		return nil, fmt.Errorf("num_range_valid: validate_oper '<': %w", err)
	}
	ref, refErr := p.ParseIdentRef()
	if refErr != nil {
		return nil, fmt.Errorf("num_range_valid: func_fixed_num_range: %w", refErr)
	}
	if _, err = p.matchLit(">"); err != nil {
		return nil, fmt.Errorf("num_range_valid: closing '>': %w", err)
	}
	return &V17NumRangeValidNode{V17BaseNode{line, col}, expr, ref}, nil
}

// ParseDateRange parses:
//
//	date_range = ( date | TYPE_OF date<ident_ref> ) ".." ( date | TYPE_OF date<ident_ref> )
func (p *V17Parser) ParseDateRange() (node *V17DateRangeNode, err error) {
	done := p.debugEnter("date_range")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	lo, loErr := p.parseDateRangeSide()
	if loErr != nil {
		return nil, fmt.Errorf("date_range: lo: %w", loErr)
	}
	if _, err = p.matchLit(".."); err != nil {
		return nil, fmt.Errorf("date_range: '..': %w", err)
	}
	hi, hiErr := p.parseDateRangeSide()
	if hiErr != nil {
		return nil, fmt.Errorf("date_range: hi: %w", hiErr)
	}
	return &V17DateRangeNode{V17BaseNode{line, col}, lo, hi}, nil
}

// ParseDateRangeValid parses:
//
//	date_range_valid = TYPE_OF boolean< date >< date_range >
func (p *V17Parser) ParseDateRangeValid() (node *V17DateRangeValidNode, err error) {
	done := p.debugEnter("date_range_valid")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.expectLit("TYPE_OF"); err != nil {
		return nil, fmt.Errorf("date_range_valid: %w", err)
	}
	if _, err = p.expectLit("boolean"); err != nil {
		return nil, fmt.Errorf("date_range_valid: %w", err)
	}
	if _, err = p.matchLit("<"); err != nil {
		return nil, fmt.Errorf("date_range_valid: opening '<': %w", err)
	}
	d, dErr := p.ParseDate()
	if dErr != nil {
		return nil, fmt.Errorf("date_range_valid: %w", dErr)
	}
	// validate_oper is "><" — two tokens: V17_GT then V17_LT
	if _, err = p.matchLit(">"); err != nil {
		return nil, fmt.Errorf("date_range_valid: validate_oper '>': %w", err)
	}
	if _, err = p.matchLit("<"); err != nil {
		return nil, fmt.Errorf("date_range_valid: validate_oper '<': %w", err)
	}
	rng, rngErr := p.ParseDateRange()
	if rngErr != nil {
		return nil, fmt.Errorf("date_range_valid: %w", rngErr)
	}
	if _, err = p.matchLit(">"); err != nil {
		return nil, fmt.Errorf("date_range_valid: closing '>': %w", err)
	}
	return &V17DateRangeValidNode{V17BaseNode{line, col}, d, rng}, nil
}

// ParseTimeRange parses:
//
//	time_range = ( time | TYPE_OF time<ident_ref> ) ".." ( time | TYPE_OF time<ident_ref> )
func (p *V17Parser) ParseTimeRange() (node *V17TimeRangeNode, err error) {
	done := p.debugEnter("time_range")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	lo, loErr := p.parseTimeRangeSide()
	if loErr != nil {
		return nil, fmt.Errorf("time_range: lo: %w", loErr)
	}
	if _, err = p.matchLit(".."); err != nil {
		return nil, fmt.Errorf("time_range: '..': %w", err)
	}
	hi, hiErr := p.parseTimeRangeSide()
	if hiErr != nil {
		return nil, fmt.Errorf("time_range: hi: %w", hiErr)
	}
	return &V17TimeRangeNode{V17BaseNode{line, col}, lo, hi}, nil
}

// ParseTimeRangeValid parses:
//
//	time_range_valid = TYPE_OF boolean< time >< time_range >
func (p *V17Parser) ParseTimeRangeValid() (node *V17TimeRangeValidNode, err error) {
	done := p.debugEnter("time_range_valid")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.expectLit("TYPE_OF"); err != nil {
		return nil, fmt.Errorf("time_range_valid: %w", err)
	}
	if _, err = p.expectLit("boolean"); err != nil {
		return nil, fmt.Errorf("time_range_valid: %w", err)
	}
	if _, err = p.matchLit("<"); err != nil {
		return nil, fmt.Errorf("time_range_valid: opening '<': %w", err)
	}
	t, tErr := p.ParseTime()
	if tErr != nil {
		return nil, fmt.Errorf("time_range_valid: %w", tErr)
	}
	// validate_oper is "><" — two tokens: V17_GT then V17_LT
	if _, err = p.matchLit(">"); err != nil {
		return nil, fmt.Errorf("time_range_valid: validate_oper '>': %w", err)
	}
	if _, err = p.matchLit("<"); err != nil {
		return nil, fmt.Errorf("time_range_valid: validate_oper '<': %w", err)
	}
	rng, rngErr := p.ParseTimeRange()
	if rngErr != nil {
		return nil, fmt.Errorf("time_range_valid: %w", rngErr)
	}
	if _, err = p.matchLit(">"); err != nil {
		return nil, fmt.Errorf("time_range_valid: closing '>': %w", err)
	}
	return &V17TimeRangeValidNode{V17BaseNode{line, col}, t, rng}, nil
}
