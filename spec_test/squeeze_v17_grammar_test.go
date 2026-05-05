// Package squeeze_v1_test - shared test helpers for all V17 grammar tests.
// Per-spec test files:
//   squeeze_v17_01_definitions_test.go
//   squeeze_v17_02_operators_test.go
//   squeeze_v17_03_assignment_test.go
//   squeeze_v17_04_objects_test.go
//   squeeze_v17_05_json_path_test.go
//   squeeze_v17_06_functions_test.go
//   squeeze_v17_07_types_scope_test.go
//   squeeze_v17_08_range_test.go
//   squeeze_v17_12_iterators_test.go
//   squeeze_v17_13_push_pull_test.go
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// newV17 creates a V17Parser from src, fataling the test on lex error.
func newV17(t *testing.T, src string) *parser.V17Parser {
	t.Helper()
	p, err := parser.NewV17ParserFromSource(src)
	if err != nil {
		t.Fatalf("lex error for %q: %v", src, err)
	}
	return p
}
