---
name: grammar-change
description: "Use when: making any change to the Squeeze grammar (spec/*.sqg EBNF rules). Covers adding, removing, or renaming grammar rules or terminals; changing cardinality, operators, or structural patterns. Guides through every downstream artifact that must be kept in sync: spec files, AST nodes, parser methods, checker spec, AST_spec.md, and library files. INVOKE for any edit to spec/0*.sqg or spec/1*.sqg or spec/2*.sqg files."
---

# Squeeze Grammar Change Workflow

A grammar change is any addition, removal, or structural modification to a rule
in `spec/*.sqg` (the Squeeze modified EBNF).  Every such change has a fixed set
of downstream artifacts that must be kept in sync.  Follow all steps in order.

---

## Step 0 — New spec file onboarding  (only when a new `spec/*.sqg` file is introduced)

A new spec file arrives when a version bump (e.g. V14) introduces grammar that
does not fit in any existing `spec/NN_*.sqg` file.  Complete every item in this
checklist **before** applying rules from the new file to the parser.

### 0.1 — Name the new file

Follow the existing naming convention:
```
spec/NN_short_topic_name.sqg
```
- `NN` is the next available two-digit prefix (check `spec/` directory listing).
- `short_topic_name` uses underscores, all lowercase, matches the dominant rule
  family (e.g. `exceptions`, `templates`, `push_pull`).

### 0.2 — Name the new parser file

The corresponding parser file is:
```
pkg/parser/parser_v15_short_topic_name.go
```
- Same root as the spec file name.
- Prefix is always `parser_v15_` — the Go parser package uses this version prefix
  for all current parser files.

### 0.3 — Create the parser file stub

Create `pkg/parser/parser_v15_short_topic_name.go` with this header and confirm
the build is clean before adding any rules:

```go
// parser_v15_short_topic_name.go — Phase 2 AST nodes and Phase 3 parse methods
// for the Squeeze grammar rules defined in spec/NN_short_topic_name.sqg.
//
// Covered rules:
//   (fill in as rules are implemented)
package parser
```

### 0.4 — Wire the include chain

Check whether the new spec file needs to be pulled into the chain via an
`include` directive in an earlier file.  Search for the last `include` in the
closest preceding spec file and add the new one after it.

### 0.5 — Update the master registry in this skill (Step 1 table)

Add a row to the **Key spec files** table below:
```
| `spec/NN_short_topic_name.sqg` | one-line description | `parser_v15_short_topic_name.go` |
```

### 0.6 — Update the spec→parser table in `implement-parse-method`

Open `.github/skills/implement-parse-method/SKILL.md` and add the matching row
to the table in **Step 1**:
```
| `spec/NN_short_topic_name.sqg` | `parser_v15_short_topic_name.go` |
```

### 0.7 — Add a test group

Add a group header to `spec_test/squeeze_v13_grammar_test.go`:
```go
// ===========================================================================
// Topic name (NN_short_topic_name.sqg)
// ===========================================================================
```
The group starts empty; tests are added as rules are implemented.

---

## Step 1 — Read the affected spec file first

Before touching anything else, read the full rule in context:

- What production rule changed?  (e.g. `import_assign`, `scope_assign`, `func_unit`)
- What changed?  New alternative? Removed terminal? New sub-rule?
- Does it introduce a new keyword / reserved word?
- Does it change cardinality of an existing field?

**Key spec files — master registry** (source of truth; update via Step 0 when a new file is added):

| File | Content | Parser file |
|---|---|---|
| `spec/00_directives.sqg` | Parser directives (UNIQUE, TYPE_OF, MERGE, EXTEND…) | *(directives only — no parser file)* |
| `spec/01_definitions.sqg` | Primitives, regexps (http_url, file_url, constants) | `parser_v15.go` |
| `spec/02_operators.sqg` | Operators and precedence | `parser_v15_operators.go` |
| `spec/03_assignment.sqg` | Assignment operators and LHS | `parser_v15_assignment.go` |
| `spec/04_objects.sqg` | object_final, object_merge_tail, object_omit_tail | `parser_v15_objects.go` |
| `spec/05_json_path.sqg` | ident_ref, json_path, inspect_type | `parser_v15_json_path.go` |
| `spec/06_functions.sqg` | func_unit, func_args, func_store_stmt, store_ctrl convention | `parser_v15_functions.go` |
| `spec/07_types_scope.sqg` | scope_assign, import_assign, scope_assign_inline, parser_root | `parser_v15_scope.go` |
| `spec/08_range.sqg` | Range constraints (><) | `parser_v15_range.go` |
| `spec/09_stuctures.sqg` | enum, bitfield, table, tree, graph | `parser_v15_structures.go` |
| `spec/10_error_handler.sqg` | Error handler modes, store_ctrl re-entry | `parser_v15_error_handler.go` |
| `spec/11_dependencies.sqg` | Dependency graph, versioned storage, execution model | `parser_v15_dependencies.go` |
| `spec/12_iterators.sqg` | Iterator protocol (>>) | `parser_v15_functions.go` |
| `spec/13_push_pull.sqg` | Push model (~>) | `parser_v15_functions.go` |
| `spec/14_templates.sqg` | Template subsystem | `parser_v15_structures.go` |
| `spec/16_exceptions.sqg` | catch_oper, scope_body_catch, caught_group, caught_scope | `parser_v15_scope.go` |
| `spec/21_checker.sqg` | Linter (Phase A) + type-checker (Phase B) specification | *(checker only — no parser file)* |

---

## Step 2 — Identify all downstream artifacts

For each grammar change, check which of the following are affected:

### 2a. Other spec files
Search for cross-references to the changed rule name:
```
grep_search(query: "<rule_name>", includePattern: "spec/**")
```
Common cross-references:
- `spec/10_error_handler.sqg` — references execution entry points
- `spec/11_dependencies.sqg` — references func_store_stmt, store_ctrl
- `spec/21_checker.sqg` — references every grammar rule that has a linter or type-check pass

### 2b. AST node types (parser_v15.go / parser_v15_*.go)
Each grammar rule that produces a node has a corresponding `V13XxxNode` struct.
Affected files:
- `pkg/parser/parser_v15.go` — primitives, constants, operators
- `pkg/parser/parser_v15_assignment.go` — assign_lhs, assign_oper
- `pkg/parser/parser_v15_functions.go` — func_unit, func_args, func_store_stmt
- `pkg/parser/parser_v13_scope.go` — scope_assign, import_assign, parser_root
- `pkg/parser/parser_v13_objects.go` — object_final, merge/omit tails
- `pkg/parser/parser_v13_operators.go` — calc_unit, logic_expr
- `pkg/parser/parser_v13_json_path.go` — ident_ref, json_path
- `pkg/parser/parser_v13_range.go` — range constraints
- `pkg/parser/parser_v13_structures.go` — enum, bitfield, table, tree

### 2c. Parse methods
Each `V13XxxNode` has a corresponding `ParseXxx()` method in the same file.
When a rule changes:
- Add new fields to the node struct if new sub-rules are added
- Update `ParseXxx()` to match the new production
- If a new alternative is added, update the union-type comment `// each element: *V13ANode | *V13BNode`
- If a rule is renamed, use `vscode_renameSymbol` to propagate the rename

**Load the `implement-parse-method` skill** for the full step-by-step workflow,
including MERGE/EXTEND cross-file integration and forward-reference stub protocol:
`c:\Work\squeeze-ai\.github\skills\implement-parse-method\SKILL.md`

### 2d. Checker spec (spec/21_checker.sqg)
Every grammar rule that is validated at compile time is referenced in one or more
Phase A (linter) or Phase B (type-checker) passes.  Check:
- Does Phase A pass A1–A11 reference this rule?
- Does Phase B pass T1–T19 reference this rule?
- Does a diagnostic code mention this rule by name?
- Is a reserved word added or removed?  → Update A6 reserved-word list
- Is a new node kind added?  → Check T1 (annotation collection) and T2 (inference)

### 2e. AST documentation (docs/AST_spec.md)
Every `V13XxxNode` struct has an entry in `docs/AST_spec.md`.
- Table row: `| V13XxxNode | fields | grammar rule |`
- Union-type prose: `` `V13XxxNode.Field` is `*V13ANode` or `*V13BNode`. ``
- Update on every field add/remove/rename

### 2f. Standard library files (pkg/lib/*.sqz)
If the grammar change affects syntax used inside library files
(e.g. operator change, new conditional, renamed terminal), search:
```
grep_search(query: "<old_syntax>", includePattern: "pkg/lib/**")
```
Fix every occurrence.  Key syntax invariants:
- Conditional is `condition & trueVal ^ falseVal`  (NOT `? :`)
- Imports use `file://./filename.sqz` URLs
- Wildcard imports use `_: "file://./..."` inside a scope_assign body

---

## Step 3 — Make changes in the correct order

Apply changes in this order to avoid reading stale content:

1. **Spec file(s)** — update the EBNF rule(s) first
2. **Cross-referenced spec files** — update any spec that mentions the rule
3. **AST node struct** — update or add `V13XxxNode` fields
4. **Parse method** — update `ParseXxx()` to match new production
5. **docs/AST_spec.md** — update table rows and union-type prose
6. **spec/21_checker.sqg** — update all affected passes and diagnostics
7. **pkg/lib/*.sqz** — fix any library files that use the changed syntax
8. **pkg/lib/std.sqz** — re-check for name conflicts if new lib exports appear

Use `multi_replace_string_in_file` for independent edits within the same file.
Use parallel tool calls for independent edits across different files.

---

## Step 4 — Validate

After all edits:
```
get_errors(filePaths: ["pkg/parser/..."])
```
Check that all modified Go files are error-free.  The spec and library files
are not Go — they have no compiler to validate them, so review manually.

---

## Step 5 — Reserved word audit

If the change introduces or removes a reserved identifier, update **all** of:

| Location | What to update |
|---|---|
| `spec/06_functions.sqg` | Prose comment in the relevant convention section |
| `spec/21_checker.sqg` — Pass A6 | Reserved-word list `(next, self, store_ctrl, null, true, false, ok)` |
| `spec/21_checker.sqg` — Pass A5 | Exceptions list (symbols that are always treated as "used") |
| `spec/21_checker.sqg` — Pass A1` | Skip conditions (symbols that don't produce SymbolRecords) |

---

## Step 6 — Diagnostic code audit

If the change creates a new compile-time error or warning:

1. Assign the next available code:
   - `L001–L099` — linter (Phase A)
   - `T001–T042` — type-checker (Phase B)
2. Add it to the relevant pass in `spec/21_checker.sqg` (Sections 22 or 23)
3. Add it to Section 25.2 diagnostic code range table

---

## Common Change Patterns

### New sub-rule or alternative added to an existing rule
- Add a new `V13XxxNode` struct (or add a field to the existing one)
- Update `ParseXxx()` to parse the new branch
- Add a row to `docs/AST_spec.md`
- Update checker pass if the new branch carries semantics

### Rule renamed
- Use `vscode_renameSymbol` for the Go struct and method names
- Update the spec comment header in the parser file
- Update `docs/AST_spec.md` table entry
- Search all spec files for the old name and update prose

### Terminal / keyword renamed (e.g. `main` → `store_ctrl`)
- Update spec file where the keyword is documented
- Update all cross-referenced spec files (grep first)
- Update checker spec A6 reserved-word list
- Update checker spec A3/A5 exception lists
- Update any `pkg/lib/*.sqz` files that use the keyword

### Regex pattern changed (e.g. `file_path` → `file_url`)
- Update `spec/01_definitions.sqg` regex
- Update the `var V13xxxRe` variable in the parser Go file
- Update `ParseXxx()` validation logic
- Update `docs/AST_spec.md` node name if renamed
- Update `pkg/lib/std.sqz` if the URL format of imports changed

### New import form added (e.g. `scope_assign_inline`)
- Add new AST node struct
- Update the parent node's field type from concrete to interface (`V13Node`)
- Update `ParseImportAssign()` to detect and branch
- Update checker A1 (no SymbolRecord for wildcard)
- Update checker A2 (inject semantics for wildcard)
- Update checker A5 (suppress unused warning for injected symbols)
- Add new diagnostic if new warnings apply (L021 for wildcard import)

---

## Checklist (copy per change)

```
[ ] 1. Read current spec rule in full context
[ ] 2. Identified all cross-referenced spec files (grep done)
[ ] 3. AST node struct updated (field added/removed/type-changed)
[ ] 4. Parse method updated to match new production
[ ] 5. docs/AST_spec.md table row updated
[ ] 6. docs/AST_spec.md union-type prose updated
[ ] 7. spec/21_checker.sqg affected passes updated
[ ] 8. spec/21_checker.sqg Section 25.2 diagnostic range updated (if new code)
[ ] 9. Reserved word list updated across all locations (if keyword added/removed)
[  ] 10. pkg/lib/*.sqz files checked for syntax affected by change
[ ] 11. pkg/lib/std.sqz conflict check re-run (if new exports added)
[ ] 12. get_errors() run on all modified Go files — zero errors
```
