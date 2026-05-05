---
name: grammar-test
description: "Use when: writing or updating functional tests for any grammar rule in spec/*.sqg. Covers bottom-up rule ordering, per-rule test anatomy, EXTEND/MERGE audit, test file layout, AST field assertions, no-consume guarantees, and coverage tracking. INVOKE whenever adding or updating tests for grammar rules."
---

# Grammar Functional Test Workflow

A grammar test verifies that a single `ParseXxx()` method accepts exactly the
inputs its spec rule describes and rejects everything else.  Tests are written
**bottom-up**: leaf rules (terminals and simple regex matches) first, composite
rules that call them second, entry-point rules last.  This ordering means a
failing test always points at the broken rule, not at a transitive caller.

Follow all steps in order.

---

## Strict Testing Rules  (MANDATORY — read before any other step)

### STR-1 — One test file per spec file

Every `spec/NN_short_name.sqg` has a corresponding test file:

```
spec/01_definitions.sqg  →  spec_test/squeeze_v17_01_definitions_test.go
spec/02_operators.sqg    →  spec_test/squeeze_v17_02_operators_test.go
spec/03_assignment.sqg   →  spec_test/squeeze_v17_03_assignment_test.go
spec/04_objects.sqg      →  spec_test/squeeze_v17_04_objects_test.go
spec/05_json_path.sqg    →  spec_test/squeeze_v17_05_json_path_test.go
spec/06_functions.sqg    →  spec_test/squeeze_v17_06_functions_test.go
spec/07_types_scope.sqg  →  spec_test/squeeze_v17_07_types_scope_test.go
spec/08_range.sqg        →  spec_test/squeeze_v17_08_range_test.go
spec/09_stuctures.sqg    →  spec_test/squeeze_v17_09_structures_test.go
spec/10_error_handler.sqg →  spec_test/squeeze_v17_10_error_handler_test.go
spec/11_dependencies.sqg →  spec_test/squeeze_v17_11_dependencies_test.go
spec/12_iterators.sqg    →  spec_test/squeeze_v17_12_iterators_test.go
spec/13_push_pull.sqg    →  spec_test/squeeze_v17_13_push_pull_test.go
spec/14_templates.sqg    →  spec_test/squeeze_v17_14_templates_test.go
spec/15_collections.sqg  →  spec_test/squeeze_v17_15_collections_test.go
spec/16_exceptions.sqg   →  spec_test/squeeze_v17_16_exceptions_test.go
spec/17_database.sqg     →  spec_test/squeeze_v17_17_database_test.go
spec/18_inspect.sqg      →  spec_test/squeeze_v17_18_inspect_test.go
```

If a new `spec/NN_*.sqg` file is added, create its test file immediately (even
if empty except for the package declaration).  Do not append tests for a rule to
the wrong spec file's test file.

The legacy `squeeze_v17_grammar_test.go` is being migrated out over time.
When tests for a rule are moved to the per-spec file, remove them from the
monolithic file to avoid duplicate test names.

### STR-2 — Test function naming

```
TestV17_<RuleName>               ← primary function (one per grammar rule)
TestV17_<RuleName>_<Variant>     ← when multiple groupings are required
```

`<RuleName>` is the UpperCamelCase of the grammar rule (same transformation as
`ParseXxx` — snake_case → UpperCamelCase, no abbreviations).  Use `_` suffix
variants for RANGE boundaries, valid/invalid split, or EXTEND sub-cases, not
for trivial structural variants.

### STR-3 — Spec citation comment

Every `TestV17_*` function opens with a comment citing the spec location:

```go
// spec/03_assignment.sqg — assign_version
func TestV17_AssignVersion(t *testing.T) {
```

When the rule is extended by a later spec file, cite both:

```go
// spec/02_operators.sqg — statement
// EXTEND: spec/06_functions.sqg (inspect_type), spec/12_iterators.sqg (iterator_loop)
func TestV17_Statement(t *testing.T) {
```

### STR-4 — Test file header

Every per-spec test file starts with this header (adjust spec file name and
description):

```go
// Package squeeze_v1_test — functional tests for spec/NN_short_name.sqg rules.
//
// Rules covered (in bottom-up order):
//   rule_a, rule_b, rule_c
package squeeze_v1_test

import (
    "testing"
    "github.com/cfjello/squeeze-ai/pkg/parser"
)
```

The `newV17` helper is already defined in `squeeze_v17_grammar_test.go` and
shared by all files in the package; do not redeclare it.

---

## Step 1 — Determine bottom-up level for each rule

Before writing a single test, classify every rule you are about to test into a
level.  A rule is at Level K if all rules it calls directly are at Level < K.

**Pre-computed level table** (use as starting point; extend when new rules are added):

| Level | Rules |
|-------|-------|
| 0 | `digits`, `digits2`, `digits3`, `digits4`, `sign_prefix`, `boolean_true`, `boolean_false`, `null`, `nan`, `infinity`, `group_begin`, `group_end`, `scope_begin`, `scope_end`, `comment_begin`, `comment_end`, `comment_txt`, `eol`, `nl`, `assign_immutable`, `assign_mutable`, `assign_read_only_ref`, `private_modifier`, `extend_scope_oper`, `return_arrow`, `into_arrow`, `iterator_oper`, `push_oper`, `deps_oper`, `range_oper`, `spread_oper`, `inline_incr`, `not_oper`, `numeric_oper`, `string_oper`, `compare_oper`, `logic_and`, `logic_or`, `logic_exclusive_or`, `assign_push`, `any_type`, `self_ref`, `duration_unit`, `regexp_flags` |
| 1 | `comment`, `comment_TBD_stub`, `boolean`, `integer`, `decimal`, `ident_name`, `hex_seg2`, `hex_seg4`, `hex_seg8`, `hex_seg12`, `hex_seg32`, `hex_seg40`, `hex_seg64`, `hex_seg128`, `date_year`, `date_month`, `date_day`, `time_hour`, `time_minute`, `time_second`, `time_millis`, `digits2`, `digits3`, `digits4`, `single_quoted`, `double_quoted`, `tmpl_quoted`, `regexp_expr`, `cardinality`, `range`, `assign_oper` |
| 2 | `numeric_const`, `string`, `string_quoted`, `date`, `time`, `byte`, `uint16`, `uint32`, `uint64`, `uint128`, `float32`, `float64`, `decimal8`, `decimal16`, `decimal32`, `decimal64`, `decimal128`, `decimal_num`, `uuid`, `uuid_v7`, `ulid`, `nano_id`, `hash_md5`, `hash_sha1`, `hash_sha256`, `hash_sha512`, `hash_key`, `duration`, `ident_dotted`, `ident_prefix`, `assign_version` |
| 3 | `date_time`, `time_stamp`, `date_range`, `time_range`, `seq_id16`, `seq_id32`, `seq_id64`, `seq_id`, `snowflake_id`, `unique_key`, `http_url`, `file_url`, `ident_ref`, `constant`, `empty_array_decl`, `empty_scope_decl`, `empty_decl`, `inspect_type`, `func_stream_decl`, `func_regexp_decl`, `func_string_decl` |
| 4 | `single_num_expr`, `num_expr_chain`, `num_grouping`, `numeric_calc`, `string_expr_chain`, `string_grouping`, `string_concat`, `num_compare`, `string_compare`, `condition`, `single_logic_expr`, `logic_expr_chain`, `logic_grouping`, `logic_expr`, `statement`, `assign_lhs`, `assign_rhs`, `assign_cond_rhs`, `values_list`, `spread_array`, `lookup_idx_expr`, `lookup_txt_expr`, `object_init`, `jp_name`, `jp_index`, `jp_wildcard`, `jp_slice`, `jp_filter_value`, `jp_filter_atom` |
| 5 | `assign_single`, `assign_new_var`, `assign_annotation` (private), `array_uniform`, `empty_array_typed`, `array_append_tail`, `array_omit_tail`, `array_lookup`, `array_final`, `object_merge_tail`, `object_omit_tail`, `object_merge_or_omit`, `object_lookup`, `object_final`, `collection`, `jp_filter_oper`, `jp_filter_unary`, `jp_filter_cmp`, `jp_filter_not`, `jp_filter_expr`, `jp_filter_logic`, `jp_filter`, `jp_selector`, `jp_selector_list`, `jp_dot_seg`, `jp_bracket_seg`, `jp_child_seg`, `jp_desc_seg`, `jp_current_path`, `jp_segment`, `json_path` |
| 6 | `assignment`, `update_mutable`, `scope_assign_inline`, `other_inline_assign`, `func_args`, `func_call_args`, `func_call`, `func_call_chain`, `func_call_final`, `func_return_stmt`, `func_deps`, `func_store_stmt`, `func_body_stmt`, `func_scope_assign`, `args_single_decl`, `args_decl`, `func_args_decl`, `iterator_yield_stmt`, `iterator_recv_decl`, `iterator_loop`, `push_recv_decl`, `push_forward_stmt`, `push_stream_bind`, `push_loop`, `return_func_unit`, `assign_private_single`, `assign_private_grouping`, `import_assign` |
| 7 | `func_unit`, `scope_body_item`, `scope_inject`, `func_inject`, `decl_types`, `scope_body_catch`, `assign_iterator` |
| 8 | `scope_body`, `private_block`, `scope_assign`, `scope_merge_tail` |
| 9 | `scope_final`, `scope_inject` (full), `func_unit_with_catch` |
| 10 | `parser_root` |

Write tests for all rules at Level K before starting Level K+1.

---

## Step 2 — Read the spec rule before writing a test

For each rule you are about to test:

1. Open the spec file and read the full rule definition.
2. Note every alternative (`|`), every optional component (`[…]`), every
   repeated component (`{…}`), and every directive (`EXTEND`, `RANGE`, etc.).
3. Search for all `EXTEND<rule_name>` and `MERGE<rule_name>` across all spec files:
   ```
   grep_search(query: "EXTEND<rule_name>|MERGE<rule_name>", includePattern: "spec/**")
   ```
   Every extension must have its own labelled test case.
4. Check for `RANGE` directives — note min and max values for boundary tests.

---

## Step 3 — Per-rule test anatomy

Every `TestV17_RuleName` function must contain all of the following, in order:

### 3.1 — Valid cases table

Use a `[]struct{ label, src string }` table for valid inputs:

```go
valids := []struct{ label, src string }{
    {"minimal",        "v1"},
    {"with_minor",     "v1.2"},
    {"full_semver",    "v2.0.1"},
    // EXTEND from spec/06_functions.sqg — one labelled case per extension
}
for _, tc := range valids {
    p := newV17(t, tc.src)
    node, err := p.ParseAssignVersion()
    if err != nil {
        t.Errorf("%s: unexpected error for %q: %v", tc.label, tc.src, err)
    }
    _ = node
}
```

Minimum 2 valid inputs; one must be the **simplest possible** (mandatory parts
only), one must include **all optional components**.

### 3.2 — Invalid cases table

Use a second table for inputs that must fail:

```go
invalids := []struct{ label, src string }{
    {"wrong_first_token",  "1.2.3"},  // no 'v' prefix
    {"bare_letters",       "abc"},
    {"empty",              ""},
}
for _, tc := range invalids {
    p := newV17(t, tc.src)
    _, err := p.ParseAssignVersion()
    if err == nil {
        t.Errorf("%s: expected failure for %q", tc.label, tc.src)
    }
}
```

The **first** invalid case must use a wrong first token (not a truncated or
garbled form of a valid input).  This guards the dispatcher's discriminating
token check.

### 3.3 — No-consume assertion (required for every failing parse)

After a failing parse, assert the parser did not advance:

```go
p := newV17(t, "bad_input")
posBefore := p.SavePos()
_, err := p.ParseRuleName()
if err == nil {
    t.Fatal("expected error")
}
posAfter := p.SavePos()
if posBefore != posAfter {
    t.Errorf("ParseRuleName consumed input on failure: pos moved from %v to %v",
        posBefore, posAfter)
}
```

Use this assertion for at least one invalid case per rule.

### 3.4 — AST field assertions (required for composite rules)

For rules that produce a node with fields, assert the field values — not just
that `err == nil`:

```go
p := newV17(t, "v1.2.3")
node, err := p.ParseAssignVersion()
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
if node.Major != 1 {
    t.Errorf("Major: got %d, want 1", node.Major)
}
if node.Minor != 2 {
    t.Errorf("Minor: got %d, want 2", node.Minor)
}
if node.Patch != 3 {
    t.Errorf("Patch: got %d, want 3", node.Patch)
}
```

At minimum verify: (a) mandatory child fields are non-nil, (b) optional child
fields are nil when absent and non-nil when present, (c) one representative
value (string/int) is correct.

### 3.5 — Whitespace tolerance (required for non-terminal rules)

Confirm that leading horizontal whitespace does not break the parse:

```go
for _, ws := range []string{"", " ", "\t", "  "} {
    p := newV17(t, ws+validInput)
    if _, err := p.ParseRuleName(); err != nil {
        t.Errorf("leading whitespace %q: unexpected error: %v", ws, err)
    }
}
```

### 3.6 — RANGE boundary cases (required for rules with RANGE directive)

For every `RANGE min..max<expr>` in the rule:

```go
boundaries := []struct{ label, src string; valid bool }{
    {"at_min",      "0",    true},
    {"at_max",      "255",  true},
    {"below_min",   "-1",   false},
    {"above_max",   "256",  false},
}
```

### 3.7 — EXTEND/MERGE coverage

For every extension found in Step 2, add a labelled valid case and at least one
labelled invalid case showing that the wrong alternative for that extension fails:

```go
// EXTEND<statement> from spec/06_functions.sqg: inspect_type
{"extend:inspect_type valid",  "@MyType",  true},
{"extend:inspect_type valid2", "@?",        true},
```

---

## Step 4 — EXTEND audit before finalising a spec file's tests

Before marking a spec file's tests as complete, run:

```
grep_search(query: "EXTEND<rule>|MERGE<rule>", includePattern: "spec/**")
```
for every rule tested in that file.  Confirm every extension has a test case.
Record any missing ones in `spec_test/COVERAGE.md` (see Step 6).

---

## Step 5 — Run and verify

After writing tests for a level, run the suite:

```powershell
go test ./spec_test/... -run TestV17 -count=1
```

All tests must pass before moving to the next level.  If a test fails because
the parser has a bug, fix the parser (using the implement-parse-method skill)
before adding more tests.  Do **not** write a test that is known to fail and
leave it; mark it in `COVERAGE.md` as `❌ failing — known parser bug` and skip
it with `t.Skip`.

When tests are added to a new per-spec file for the first time, also run the
full suite to confirm no name collisions with the monolithic file:

```powershell
go test ./spec_test/... -count=1
```

---

## Step 6 — Update COVERAGE.md

`spec_test/COVERAGE.md` is the single source of truth for test coverage.  It
must be updated after every work session.

### Format

```markdown
# Grammar Test Coverage

Generated: YYYY-MM-DD

| Rule | Spec file | Level | Status | Notes |
|------|-----------|-------|--------|-------|
| digits | 01_definitions | 0 | ✅ done | |
| assign_version | 03_assignment | 2 | ✅ done | |
| statement | 02_operators | 4 | 🔄 partial | missing EXTEND for iterator_loop |
| private_block | 07_types_scope | 8 | ❌ missing | |
| parser_root | 07_types_scope | 10 | ❌ missing | |
```

Status values:
- `✅ done` — all anatomy steps (3.1–3.7) complete; no known gaps
- `🔄 partial` — at least one required step missing (note what)
- `❌ missing` — no test function exists yet
- `⏭ skipped` — `t.Skip` in place; notes field must explain why

---

## Step 7 — Migration from the monolithic file

When tests for a rule are written in their per-spec file:

1. Confirm the new test function name is identical to the old one.
2. Delete the old function from `squeeze_v17_grammar_test.go`.
3. Run `go test ./spec_test/... -count=1` to confirm no name collision and no
   regression.
4. Update `COVERAGE.md`.

Do **not** leave duplicate `TestV17_RuleName` functions across files — Go will
report a compile error, but the intent here is to prevent it proactively.

---

## Worked Example — `assign_version`  (spec/03_assignment.sqg, Level 2)

```go
// spec/03_assignment.sqg — assign_version
// assign_version = ["v"] digits { "." digits } ;
// Note: "v" prefix is MANDATORY in V17 (bare-digits form removed).
func TestV17_AssignVersion(t *testing.T) {
    // 3.1 Valid cases
    valids := []struct{ label, src string }{
        {"v_only_major",    "v1"},
        {"v_major_minor",   "v1.2"},
        {"v_full_semver",   "v2.0.1"},
        {"v_many_segments", "v10.20.30.4"},
    }
    for _, tc := range valids {
        p := newV17(t, tc.src)
        node, err := p.ParseAssignVersion()
        if err != nil {
            t.Errorf("%s: unexpected error for %q: %v", tc.label, tc.src, err)
            continue
        }
        // 3.4 AST field assertion: HasV must be true
        if !node.HasV {
            t.Errorf("%s: HasV should be true for %q", tc.label, tc.src)
        }
    }

    // 3.2 Invalid cases
    invalids := []struct{ label, src string }{
        {"wrong_first_token_digit", "1"},
        {"wrong_first_token_digit_semver", "1.2.3"},
        {"bare_letters", "abc"},
        {"empty", ""},
    }
    for _, tc := range invalids {
        p := newV17(t, tc.src)
        _, err := p.ParseAssignVersion()
        if err == nil {
            t.Errorf("%s: expected failure for %q", tc.label, tc.src)
        }
    }

    // 3.3 No-consume assertion
    p := newV17(t, "1.2.3")
    posBefore := p.SavePos()
    _, err := p.ParseAssignVersion()
    if err == nil {
        t.Fatal("no-consume: expected error for bare digits")
    }
    posAfter := p.SavePos()
    if posBefore != posAfter {
        t.Errorf("no-consume: parser moved from %v to %v on failure", posBefore, posAfter)
    }

    // 3.5 Whitespace tolerance
    for _, ws := range []string{" ", "\t"} {
        p2 := newV17(t, ws+"v1.0")
        if _, err2 := p2.ParseAssignVersion(); err2 != nil {
            t.Errorf("leading whitespace %q: unexpected error: %v", ws, err2)
        }
    }
}
```

---

## Quick reference — anatomy checklist

Use this as a per-function checklist when writing or reviewing a test:

- [ ] STR-3 spec citation comment present
- [ ] 3.1 Valid table — ≥ 2 cases (minimal + all-optional)
- [ ] 3.2 Invalid table — ≥ 1 case; first entry has wrong first token
- [ ] 3.3 No-consume assertion for at least one invalid input
- [ ] 3.4 AST field assertions (composite rules only)
- [ ] 3.5 Whitespace tolerance check (non-terminals only)
- [ ] 3.6 RANGE boundaries (rules with RANGE directive only)
- [ ] 3.7 EXTEND/MERGE cases (rules extended by other spec files)
- [ ] `COVERAGE.md` updated
