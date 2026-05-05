// Package squeeze_v1_test — functional tests for spec/08_range.sqg rules.
//
// Rules covered (bottom-up order):
//
//	Level 0: range_oper, validate_oper
//	Level 1: num_range_valid, date_range, time_range
//	Level 2: date_range_valid, time_range_valid
package squeeze_v1_test

import (
	"testing"
)

// ===========================================================================
// range_oper / validate_oper  (spec/08_range.sqg)
// ===========================================================================

// TestV17_RangeOper tests range_oper = "..".
func TestV17_RangeOper(t *testing.T) {
	p := newV17(t, "..")
	if _, err := p.ParseRangeOper(); err != nil {
		t.Errorf("range_oper '..': unexpected error: %v", err)
	}
	p2 := newV17(t, ".")
	if _, err := p2.ParseRangeOper(); err == nil {
		t.Error("range_oper '.': expected failure for single dot")
	}
	p3 := newV17(t, "+")
	if _, err := p3.ParseRangeOper(); err == nil {
		t.Error("range_oper '+': expected failure")
	}
}

// TestV17_ValidateOper tests validate_oper = "><".
func TestV17_ValidateOper(t *testing.T) {
	p := newV17(t, "><")
	if _, err := p.ParseValidateOper(); err != nil {
		t.Errorf("validate_oper '><': unexpected error: %v", err)
	}
	p2 := newV17(t, "<>")
	if _, err := p2.ParseValidateOper(); err == nil {
		t.Error("validate_oper '<>': expected failure (wrong order)")
	}
	p3 := newV17(t, ">")
	if _, err := p3.ParseValidateOper(); err == nil {
		t.Error("validate_oper '>': expected failure for single token")
	}
	p4 := newV17(t, "<")
	if _, err := p4.ParseValidateOper(); err == nil {
		t.Error("validate_oper '<': expected failure for single token")
	}
}

// ===========================================================================
// num_range_valid  (spec/08_range.sqg)
// ===========================================================================

// TestV17_NumRangeValid tests num_range_valid = "TYPE_OF" base_type "<" statement "><" ident_ref ">".
func TestV17_NumRangeValid(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"TYPE_OF boolean<42 >< myRange>", true},
		// Missing TYPE_OF keyword
		{"boolean<42 >< myRange>", false},
		// Missing validate_oper
		{"TYPE_OF boolean<42 myRange>", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseNumRangeValid()
		if tc.valid && err != nil {
			t.Errorf("num_range_valid %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("num_range_valid %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// date_range / date_range_valid  (spec/08_range.sqg)
// ===========================================================================

// TestV17_DateRange tests date_range = date_or_ref range_oper date_or_ref.
func TestV17_DateRange(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"2024-01-01..2024-12-31", true},
		{"2024..2025", true},
		{"TYPE_OF date<startDate>..2024-12-31", true},
		// RANGE boundaries (spec 3.6): edge of valid date tokens
		{"2024-01-01..2024-01-01", true}, // same start/end allowed by parser
		{"2024-12-31..2025-01-01", true}, // year boundary
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDateRange()
		if tc.valid && err != nil {
			t.Errorf("date_range %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("date_range %q: expected failure", tc.src)
		}
	}
}

// TestV17_DateRangeValid tests date_range_valid = "TYPE_OF" "boolean" "<" date_value "><" date_range ">".
func TestV17_DateRangeValid(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"TYPE_OF boolean<2024-06-15 >< 2024-01-01..2024-12-31>", true},
		// Missing TYPE_OF
		{"boolean<2024-06-15 >< 2024-01-01..2024-12-31>", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseDateRangeValid()
		if tc.valid && err != nil {
			t.Errorf("date_range_valid %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("date_range_valid %q: expected failure", tc.src)
		}
	}
}

// ===========================================================================
// time_range / time_range_valid  (spec/08_range.sqg)
// ===========================================================================

// TestV17_TimeRange tests time_range = time_or_ref range_oper time_or_ref.
func TestV17_TimeRange(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"08:30:00..17:00:00", true},
		{"08..17", true},
		{"TYPE_OF time<startHour>..17:00:00", true},
		// RANGE boundaries
		{"00:00:00..23:59:59", true}, // midnight to last second
		{"00..23", true},             // hour-only boundary
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseTimeRange()
		if tc.valid && err != nil {
			t.Errorf("time_range %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("time_range %q: expected failure", tc.src)
		}
	}
}

// TestV17_TimeRangeValid tests time_range_valid = "TYPE_OF" "boolean" "<" time_value "><" time_range ">".
func TestV17_TimeRangeValid(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"TYPE_OF boolean<09:00:00 >< 08:00:00..17:00:00>", true},
		// Missing TYPE_OF
		{"boolean<09:00:00 >< 08:00:00..17:00:00>", false},
	}
	for _, tc := range cases {
		p := newV17(t, tc.src)
		_, err := p.ParseTimeRangeValid()
		if tc.valid && err != nil {
			t.Errorf("time_range_valid %q: unexpected error: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("time_range_valid %q: expected failure", tc.src)
		}
	}
}
