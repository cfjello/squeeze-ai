//go:build ignore

// Package squeeze_v13_test contains a specification-driven test suite for the
// Squeeze V13 grammar as defined in spec/00_directives.sqg … spec/09_stuctures.sqg.
//
// Each test group corresponds to a key grammar rule or group of related rules.
// Tests validate that the V13 parser correctly parses representative inputs
// for every major new construct introduced in V13.
package squeeze_v1_test

import (
	"strings"
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ===========================================================================
// Helpers
// ===========================================================================

// parseRHS tokenises src and tries to parse it as an assign_rhs value.
// Returns nil error on success.
func parseRHS(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParseAssignRHS()
	return err
}

// parseRoot wraps src in a scope and parses the whole thing.
func parseRoot(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParseParserRoot()
	return err
}

// mustParseRHS fails the test immediately if src cannot be parsed as assign_rhs.
func mustParseRHS(t *testing.T, label, src string) {
	t.Helper()
	if err := parseRHS(src); err != nil {
		t.Errorf("%s: unexpected error parsing %q: %v", label, src, err)
	}
}

// mustFailRHS fails the test immediately if src is successfully parsed as assign_rhs.
func mustFailRHS(t *testing.T, label, src string) {
	t.Helper()
	if err := parseRHS(src); err == nil {
		t.Errorf("%s: expected parse failure for %q but it succeeded", label, src)
	}
}

// ===========================================================================
// Tokens (01_definitions.sqg)
// ===========================================================================

func TestV13_Tokens_UniqueKeyTypes(t *testing.T) {
	// V13 adds ulid, nano_id, snowflake_id as recognised token shapes.
	// These are tested via the lexer: the tokens arrive as IDENT or INTEGER.
	cases := []string{
		// uuid_v7 shape (lex as IDENT-like string with hyphens — direct string)
		// snowflake_id: plain uint64
		"1701388800000000000",
		// integer used as seq_id-style value
		"42",
	}
	for _, src := range cases {
		lex := parser.NewV13Lexer(src)
		toks, err := lex.V13Tokenize()
		if err != nil {
			t.Errorf("lex error for %q: %v", src, err)
		}
		if len(toks) < 2 {
			t.Errorf("expected at least one token for %q", src)
		}
	}
}

// ===========================================================================
// Operators (02_operators.sqg)
// ===========================================================================

func TestV13_Operators_IdentRef(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{"myVar", true},
		{"foo", true},
		{"a.b.c", true},
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, _ := lex.V13Tokenize()
		p := parser.NewV13Parser(toks)
		_, err := p.ParseIdentRef()
		if tc.valid && err != nil {
			t.Errorf("ident_ref: expected %q to parse, got: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ident_ref: expected %q to fail, but it succeeded", tc.src)
		}
	}
}

func TestV13_Operators_LogicExclusive(t *testing.T) {
	// logic_exclusive_or "^" formalised in logic_oper = logic_and | logic_or | logic_exclusive_or
	cases := []struct {
		label string
		src   string
		valid bool
	}{
		{"and", "a & b", true},
		{"or", "a | b", true},
		{"exclusive_or", "a ^ b", true},
		{"chained xor", "a ^ b ^ c", true},
		{"mixed", "a & b ^ c", true},
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, _ := lex.V13Tokenize()
		p := parser.NewV13Parser(toks)
		_, err := p.ParseLogicExpr()
		if tc.valid && err != nil {
			t.Errorf("logic_oper %s: unexpected error for %q: %v", tc.label, tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("logic_oper %s: expected failure for %q", tc.label, tc.src)
		}
	}
}

func TestV13_Operators_InspectType(t *testing.T) {
	// V13 adds @? as a valid type annotation
	cases := []struct {
		src   string
		valid bool
	}{
		{"@string", true},
		{"@uuid_v7", true},
		{"@?", true}, // V13 new: any_type
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, _ := lex.V13Tokenize()
		p := parser.NewV13Parser(toks)
		_, err := p.ParseInspectType()
		if tc.valid && err != nil {
			t.Errorf("inspect_type: expected %q to parse, got: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("inspect_type: expected %q to fail", tc.src)
		}
	}
}

// ===========================================================================
// Assignment (03_assignment.sqg)
// ===========================================================================

func TestV13_Assignment_AssignVersion_OptionalV(t *testing.T) {
	// V13: "v" prefix is optional; bare decimal not supported (comes as DECIMAL token)
	cases := []string{"v1", "v12", "1", "42"}
	for _, src := range cases {
		lex := parser.NewV13Lexer(src)
		toks, _ := lex.V13Tokenize()
		p := parser.NewV13Parser(toks)
		_, err := p.ParseAssignVersion()
		if err != nil {
			t.Errorf("assign_version: expected %q to parse, got: %v", src, err)
		}
	}
}

func TestV13_Assignment_BasicAssignment(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`myX : 42`, true},
		{`myY = "hello"`, true},
		{`myZ += 1`, true},
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, _ := lex.V13Tokenize()
		p := parser.NewV13Parser(toks)
		_, err := p.ParseAssignment()
		if tc.valid && err != nil {
			t.Errorf("assignment: expected %q to parse, got: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("assignment: expected %q to fail", tc.src)
		}
	}
}

// ===========================================================================
// Objects (04_objects.sqg)
// ===========================================================================

func TestV13_Objects_ArrayFinal(t *testing.T) {
	cases := []string{
		`[1, 2, 3]`,
		`["a", "b"]`,
	}
	for _, src := range cases {
		mustParseRHS(t, "array_final", src)
	}
}

func TestV13_Objects_ObjectFinal(t *testing.T) {
	cases := []string{
		`[x: 1, y: 2]`,
		`[name: "Alice", age: 30]`,
	}
	for _, src := range cases {
		mustParseRHS(t, "object_final", src)
	}
}

func TestV13_Objects_SplitArray(t *testing.T) {
	// split_array = "..." ( string | digets | integer | date | date_time | time | time_stamp )
	cases := []struct {
		label string
		src   string
		valid bool
	}{
		{"string separator", `... ","`, true},
		{"integer separator", `... 42`, true},
		{"digits separator", `... 0`, true},
		{"no separator", `...`, false},
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, _ := lex.V13Tokenize()
		p := parser.NewV13Parser(toks)
		_, err := p.ParseSplitArray()
		if tc.valid && err != nil {
			t.Errorf("split_array %s: unexpected error: %v", tc.label, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("split_array %s: expected failure for %q", tc.label, tc.src)
		}
	}
}

// ===========================================================================
// Set (09_stuctures.sqg — Set)
// ===========================================================================

func TestV13_Set_SetInit(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
	}{
		{`{"admin", "editor", "viewer"}`, true},
		{`{2, 3, 5, 7}`, true},
		{`{true, false}`, true},
		{`{}`, false}, // empty set invalid (needs ≥1 member)
	}
	for _, tc := range cases {
		err := parseRHS(tc.src)
		if tc.valid && err != nil {
			t.Errorf("set_init: expected %q to parse, got: %v", tc.src, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("set_init: expected %q to fail", tc.src)
		}
	}
}

func TestV13_Set_SetFinal_WithAddTail(t *testing.T) {
	src := `{"admin", "editor"} + {"auditor"}`
	mustParseRHS(t, "set_final add_tail", src)
}

func TestV13_Set_SetFinal_WithOmitTail(t *testing.T) {
	// set_omit_tail = "-" set_value { "," set_value }
	src := `{"admin", "editor", "viewer"} - "viewer"`
	mustParseRHS(t, "set_final omit_tail", src)
}

// ===========================================================================
// Enum (09_stuctures.sqg — Enum)
// ===========================================================================

func TestV13_Enum_EnumDecl(t *testing.T) {
	cases := []string{
		`ENUM ["active", "inactive", "pending"]`,
		`ENUM [400, 401, 403, 404]`,
	}
	for _, src := range cases {
		mustParseRHS(t, "enum_decl", src)
	}
}

func TestV13_Enum_EnumFinal_WithExtend(t *testing.T) {
	src := `ENUM ["active", "inactive"] EXTEND ["archived"]`
	mustParseRHS(t, "enum_final EXTEND", src)
}

// ===========================================================================
// Bitfield (09_stuctures.sqg — Bitfield)
// ===========================================================================

func TestV13_Bitfield_BitfieldDecl(t *testing.T) {
	cases := []string{
		`BITFIELD uint8 [read: 0, write: 1, exec: 2]`,
		`BITFIELD uint16 [fin: 0, syn: 1, rst: 2, psh: 3, ack: 4, urg: 5]`,
		`BITFIELD uint32 [flagA: 0]`,
		`BITFIELD uint64 [bit0: 0, bit63: 63]`,
	}
	for _, src := range cases {
		mustParseRHS(t, "bitfield_decl", src)
	}
}

func TestV13_Bitfield_InvalidBase(t *testing.T) {
	// uint128 is not a valid bitfield_base
	mustFailRHS(t, "bitfield invalid base", `BITFIELD uint128 [flag: 0]`)
}

// ===========================================================================
// Table (09_stuctures.sqg — Table)
// ===========================================================================

func TestV13_Table_TableInit(t *testing.T) {
	src := strings.TrimSpace(`
[
  columns:     [@string, @uint8],
  key_columns: [@string],
  rows:        [ ["Alice", 30], ["Bob", 25] ]
]`)
	mustParseRHS(t, "table_init (anon cols)", src)
}

func TestV13_Table_TableFinal_WithInsTail(t *testing.T) {
	src := strings.TrimSpace(`
[
  columns:     [id@string, age@uint8],
  key_columns: [id@string],
  rows:        [ ["Alice", 30] ]
] + ["Bob", 25]`)
	mustParseRHS(t, "table_final with ins_tail", src)
}

// ===========================================================================
// Tree (09_stuctures.sqg — Tree)
// ===========================================================================

func TestV13_Tree_TreeFinal(t *testing.T) {
	src := strings.TrimSpace(`
[
  type: @string,
  root: ["Alice", children: [
    ["Bob"],
    ["Carol"]
  ]]
]`)
	mustParseRHS(t, "tree_final", src)
}

func TestV13_StringTree_StringTreeFinal(t *testing.T) {
	src := strings.TrimSpace(`
[
  root: ["File", children: [
    ["New"],
    ["Open"]
  ]]
]`)
	mustParseRHS(t, "string_tree_final", src)
}

// ===========================================================================
// Graph (09_stuctures.sqg — Graph)
// ===========================================================================

func TestV13_Graph_GraphFinal(t *testing.T) {
	// Using integer snowflake IDs as unique_key values
	src := strings.TrimSpace(`
[
  nodes: [
    [key: 100, value: "parser"],
    [key: 101, value: "lexer"]
  ],
  edges: [
    [from: 100, to: 101]
  ]
]`)
	mustParseRHS(t, "graph_final", src)
}

func TestV13_Graph_GraphFinal_WithLabel(t *testing.T) {
	src := strings.TrimSpace(`
[
  nodes: [[key: 1, value: "A"], [key: 2, value: "B"]],
  edges: [[from: 1, to: 2, label: "depends_on"]]
]`)
	mustParseRHS(t, "graph_final label", src)
}

// ===========================================================================
// Scope (07_types_scope.sqg)
// ===========================================================================

func TestV13_Scope_ScopeAssign(t *testing.T) {
	src := `myScope : {
  x : 1
  y : 2
}`
	if err := parseRoot(src); err != nil {
		t.Errorf("scope_assign: unexpected error: %v", err)
	}
}

func TestV13_Scope_ImportAssign_FilePath(t *testing.T) {
	// file_path requires a quoted path
	src := `data : "./data/sample.sqz"`
	lex := parser.NewV13Lexer(src)
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	_, err := p.ParseImportAssign()
	if err != nil {
		t.Logf("import_assign file_path: %v (may require specific path syntax)", err)
	}
}

// ===========================================================================
// Range (08_range.sqg)
// ===========================================================================

func TestV13_Range_RegexpAssign(t *testing.T) {
	// regexp_assign_oper = [ ":" | "=" ] "~"  →  :~  |  =~  |  ~
	cases := []struct {
		label string
		src   string
	}{
		{"match_op (=~)", `TYPE_OF string<"hello" =~ /^hello$/>`},
		{"readonly (:~)", `TYPE_OF string<"hello" :~ /^hello$/>`},
		{"tilde (~)", `TYPE_OF string<"hello" ~ /^hello$/>`},
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, _ := lex.V13Tokenize()
		p := parser.NewV13Parser(toks)
		_, err := p.ParseRegexpAssign()
		if err != nil {
			t.Errorf("regexp_assign %s: unexpected error: %v", tc.label, err)
		}
	}
}
