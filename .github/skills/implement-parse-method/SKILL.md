---
name: implement-parse-method
description: "Use when: writing or updating a ParseXxx() method in pkg/parser/parser_v15_*.go after a grammar rule has been added or changed. Covers version-bump inventory, removed-rule protocol, AST node struct conventions, EBNF-to-Go coding patterns, MERGE/EXTEND cross-file integration, forward-reference stubs, unknown-token logging, lookahead/backtracking rules, error message conventions, and the debug/test cycle. INVOKE for step 3.4 of the grammar-change skill."
---

# Implement Parse Method Workflow

A parse method translates one EBNF production rule from `spec/*.sqg` into a Go
method on `*V13Parser` that builds an AST node.  Follow all steps in order.

---

## Strict Implementation Rules  (MANDATORY — read before any other step)

These three rules override all other guidance.  Violating any of them is a
hard error.  **Stop and report to the user rather than working around them.**

### SIR-1 — Function name = grammar rule name (camelCase)

Every grammar rule `rule_name` must map to **exactly one** Go method named
`ParseRuleName` (snake_case → UpperCamelCase, one word per underscore segment,
no abbreviations or renames).

```
spec rule           Go method
──────────────────────────────
func_assign      →  ParseFuncAssign
receiver_clause  →  ParseReceiverClause
num_range_valid  →  ParseNumRangeValid
parser_root      →  ParseParserRoot
```

**Name conflicts with Go reserved words**: Go has no reserved identifiers that
map to Squeeze rule names via this transformation.  However if a conflict is
ever detected, **stop immediately and report the conflict to the user**.  You
are not permitted to rename the function unilaterally.

**Helper functions** that decompose one rule into smaller sub-tasks are allowed
(e.g. `parseNumCompareSide`, `parseFuncInjectItem`) but must be private (lower
camelCase with `parse` prefix) and must not implement a named grammar rule —
they exist only to reduce method length.  Every **named grammar rule** must have
its own exported `ParseXxx` method.

### SIR-2 — Debug trace in every ParseXxx (when debug is enabled)

Every `ParseXxx()` method, new or updated, must emit a structured trace line
when `p.DebugFlag` is true.  The trace must include:

- **Rule name** (exactly as in the spec, not the Go function name).
- **Parser depth** — number of `ParseXxx` calls currently on the call stack
  (i.e. nesting level from the top-level token).
- **File position** — `L<line>:C<col>` of the current token at entry.
- **20-character source preview** — the first ≤ 20 characters of the token
  stream from the current position, with whitespace normalised to a single space.

Use the `debugEnter`/`defer done` pattern from Step 9.3:

```go
func (p *V13Parser) ParseFuncAssign() (node *V13FuncAssignNode, err error) {
    done := p.debugEnter("func_assign")
    defer func() { done(err == nil) }()
    // ... implementation ...
}
```

`debugEnter` (defined in Step 9.2) prints:
```
  → func_assign  [L12:C4  (arr : @array) su]
```
where the rightmost field is the 20-char preview.  **Do not inline the debug
print** — always use `debugEnter`/`done`; they manage the depth counter.

Adding a `ParseXxx` without debug trace is a **build-time error** (enforced by
a `TestAllParsersHaveDebugTrace` test in `spec_test/` — see Step 10.5).

### SIR-3 — Parser follows the grammar; grammar is not patched in the parser

The parser must implement **exactly** the grammar rule as written in
`spec/*.sqg`.  No deviation is permitted:

- If a language construct does not parse under the current grammar, the fix
  belongs in the **spec** (via the grammar-change skill with user approval).
- **Forbidden in the parser**: adding extra token alternatives, silent fallbacks,
  special-case branches, or "helpful" type coercions that the spec does not
  describe.
- If implementation of a spec rule is genuinely impossible without a spec change
  (e.g. an ambiguous first token), **stop and report the ambiguity to the user**
  with a concrete suggestion for a grammar fix.  Wait for approval before
  writing any workaround.
- If a `spec/*.sqg` file and the parser have drifted apart, the spec wins.
  Bring the parser into line; do not bring the spec into line with bad parser
  behaviour.

**Corollary — no grammar fixes buried in parser helpers**: a helper function
that silently expands the set of accepted tokens (e.g. accepting bare `integer`
where the spec says `inspect_type`) is a grammar fix, not an implementation
detail.  Surface it as a proposed spec change first.

### SIR-4 — Never pre-lex; all token classification happens at parse time

You are not allowed to create a separate pre-lexing pass that classifies tokens into categories or types before parsing.  All token classification must be done in the context of the grammar rules:

1. For "non-terminal tokens" a parser rule function must call another grammer rule function.
2. For "terminal tokens" the parser must call the specific regular expression or text-match.

There can be no exceptions to these 2 rules, because you are dealing with a context sensitive grammer. If you diviate, the parser will not work. So pre-lexing is strictly forbitten, only rule function by rule function and lokkup of specific match patterns are allowed.

---

## Step 0 — Version bump inventory  (only when a version tag such as V14 is introduced)

Run this step **once** at the start of a version upgrade, before touching any
parser file.  Its output is the ordered work list for all subsequent steps.

### 0.1 — Enumerate all changed rules

Search every spec file for the version tag:
```
grep_search(query: "V14", includePattern: "spec/**", isRegexp: false)
```
For each match, record:
- The **spec file** containing the change.
- The **rule name** (the EBNF `rule_name =` line nearest to the tag).
- Whether the tag appears in a **live rule** or in a **comment block** (`(* … *)`).

### 0.2 — Classify each change

For every rule identified in 0.1, assign one of three classes:

| Class | Marker | Meaning |
|---|---|---|
| **Modified** | `(* V14 modified … *)` above a live rule | Rule still exists; shape changed |
| **New** | `(* V14 added … *)` above a live rule | Rule did not exist before |
| **Deleted** | Rule body replaced by or wrapped in `(* V14 - Deleted … *)` | Rule no longer active |

A rule is **deleted** when its production is commented out entirely.  A rule is
**orphaned** (special case of deleted) when no other rule references it after
the deletion — see Step 0.4.

### 0.3 — Build a dependency-ordered work list

For each **modified** and **new** rule, determine which other rules it depends
on (i.e. rules it calls on its RHS).  Rules whose dependencies are all already
implemented go first.  Rules that introduce forward references go later (stubs
will be written first per Step 4).

Record the final ordered list before starting any implementation work.

### 0.4 — Handle deleted rules

For each **deleted** rule:

1. **Find all call sites** in the parser:
   ```
   grep_search(query: "ParseRuleName|V13RuleNameNode", includePattern: "pkg/parser/**")
   ```
2. **Check for orphans**: also search all active (non-commented) spec rules:
   ```
   grep_search(query: "rule_name", includePattern: "spec/**")
   ```
   Filter results to exclude comment blocks.  If no active rule references it,
   the rule is **orphaned**.

3. **Orphaned rule**: do not delete the Go code immediately.  Instead:
   - Add a `// ORPHANED: rule_name deleted in VNN — safe to remove when confirmed unused` comment to the struct and parse method.
   - Log the orphan name to the feedback report (see Step 0.5).
   - Do **not** remove the code until explicitly confirmed with the user.

4. **Referenced deleted rule**: the deletion breaks a live rule.  The referencing
   rule must be updated to remove the deleted alternative from its dispatcher
   (see Step 0b below) **before** deleting the Go code.

5. Once all call sites are removed, delete:
   - The `V13XxxNode` struct.
   - The `ParseXxx()` method.
   - Any `parsePrivateXxx` helpers used only by the deleted rule.
   - The corresponding row in `docs/AST_spec.md`.
   - All tests in `spec_test/` that exercise only the deleted rule.

### 0.5 — Produce a feedback report before coding

After 0.1–0.4, output a plain list to the user:
```
Version bump inventory — VNN

NEW rules:      rule_a (spec/07), rule_b (spec/16)
MODIFIED rules: scope_assign (spec/07), private_block (spec/07)
DELETED rules:  private_item (spec/07)          → orphaned, stub comment added
                scope_with_catch (spec/07, /16)  → 2 call sites to remove first

Work order: scope_assign → private_block → rule_a → rule_b
```
Wait for user acknowledgement before proceeding to parser changes.

---

## Step 0b — Dispatcher revision for removed alternatives

When a rule is **deleted** or an alternative is **removed from a MERGE/EXTEND**,
the dispatcher of every rule that called it must be updated.

1. **Find all dispatcher branches** that call the deleted method:
   ```
   grep_search(query: "ParseDeletedRule", includePattern: "pkg/parser/**")
   ```
2. **Remove the branch** — the entire `savePos / try / restorePos` block for
   that alternative.  Do not leave a dead branch or an unreachable `restorePos`.
3. **Re-check ordering**: after removal, confirm the remaining branches are still
   ordered from most-specific to most-generic (see Step 6 discriminating-token
   table).  A removal can expose a previously-shadowed branch that now needs to
   move up.
4. **Update the union-type comment** on the parent node's `Value V13Node` field
   to remove the deleted type from `// each element: *V13ANode | *V13BNode`.
5. **Run `get_errors`** — confirm clean build after the removal.
6. **Update tests**: remove or rewrite test cases that specifically exercised the
   deleted alternative.  Run the full suite to confirm no regression.

---

## Step 1 — Read before writing

Before touching any `.go` file:

1. Re-read the **spec rule** in full (including any `MERGE`/`EXTEND` directives that
   reference it — see Step 3 below).
2. Read the **existing `ParseXxx()` method** if it exists (or confirm it doesn't).
3. Read the **`V13XxxNode` struct** for the same rule.
4. Identify the **correct parser file** for this rule using the table below.

> **Kept in sync by grammar-change Step 0 onboarding.**
> If the spec file you are working on is not listed here, stop and run the
> grammar-change onboarding checklist (Step 0) first — it will create the parser
> file stub and add the row to both tables.

| Spec file | Parser file |
|---|---|
| `spec/01_definitions.sqg` | `parser_v15.go` |
| `spec/02_operators.sqg` | `parser_v15_operators.go` |
| `spec/03_assignment.sqg` | `parser_v15_assignment.go` |
| `spec/04_objects.sqg` | `parser_v15_objects.go` |
| `spec/05_json_path.sqg` | `parser_v15_json_path.go` |
| `spec/06_functions.sqg` | `parser_v15_functions.go` |
| `spec/07_types_scope.sqg` | `parser_v15_scope.go` |
| `spec/08_range.sqg` | `parser_v15_range.go` |
| `spec/09_stuctures.sqg` | `parser_v15_structures.go` |
| `spec/10_error_handler.sqg` | `parser_v15_error_handler.go` |
| `spec/11_dependencies.sqg` | `parser_v15_dependencies.go` |
| `spec/12_iterators.sqg` | `parser_v15_functions.go` |
| `spec/13_push_pull.sqg` | `parser_v15_functions.go` |
| `spec/14_templates.sqg` | `parser_v15_structures.go` |
| `spec/16_exceptions.sqg` | `parser_v15_scope.go` |

---

## Step 2 — AST node struct conventions

Every node struct must follow these conventions exactly:

```go
// V13XxxNode  rule_name = ebnf rule here
type V13XxxNode struct {
    V13BaseNode
    Field1 *V13SubNodeA          // optional: nil when absent
    Field2 []*V13SubNodeB        // repeated: empty slice when zero items
    Field3 V13Node               // union: each element: *V13ANode | *V13BNode
    Field4 *V13FuncUnitNode      // catch handler: nil when ^ func_unit absent
}
```

Rules:
- Embed `V13BaseNode` always (carries `Line`, `Col`).
- Optional sub-rule → pointer field, `nil` when absent.
- Repeated sub-rule → slice field.
- Union-type field → `V13Node` interface + doc comment listing concrete types.
- Catch handler → always named `CatchHandler *V13FuncUnitNode`.

**When changing an existing struct**: update the struct first, run `get_errors`,
fix all compile errors before touching `ParseXxx()`.

---

## Step 2b — Directives: know before you code

Every directive defined in `spec/00_directives.sqg` (and extended in later spec
files) has a defined parser-side implementation pattern.  Read the directive
definition **before** implementing any rule that uses it.  The table below is the
authoritative mapping; the spec file is always the final arbiter.

| Directive | Spec section | Parser implementation pattern |
|---|---|---|
| `UNIQUE<…>` | 2.1 | Collect results into `map[string]bool`; `return nil, p.errAt(…)` on duplicate key |
| `RANGE min..max<expr>` | 2.2 | Parse `expr`, then check `val >= min && val <= max`; error if out of range |
| `TYPE_OF token<expr>` | 2.3 | Parse `expr`, then type-switch/assert the returned node is the named type; `INFER` means deduce from node's own type field |
| `VALUE_OF<expr>` | 2.4 | Parse `expr`, set a `ValueOf bool` flag on the node — semantic resolution deferred to checker |
| `ADDRESS_OF<expr>` | 2.5 | Parse `expr`, set an `AddressOf bool` flag — semantic resolution deferred to checker |
| `RETURN op<expr>` | 2.6 | Parse `op` then `expr`; derive value/address semantics from operator at checker time |
| `UNIFORM token<expr>` / `UNIFORM INFER<expr>` | 2.7 | Parse `expr`, store expected element type; checker enforces uniformity |
| `INFER` | 2.8 | Used only inside `TYPE_OF`/`UNIFORM` — no extra parser action; store `Infer: true` on node |
| `EXTEND<rule> = \| alts` | 2.9 | Add new alternatives to the base rule's dispatcher (see Step 3) |
| `CAST<src> = wider…` | 2.10 | Record the chain in `p.CastChains`; apply during constant folding in checker |
| `HTTP_EXISTS<url>` | 2.11 | Parse `url`; emit a `V13HTTPExistsNode` — actual HTTP check is a runtime/checker pass |
| `FILE_EXISTS<path>` | 2.12 | Parse `path`; emit a `V13FileExistsNode` — actual check is a runtime/checker pass |
| `CODE<\` sqz \`>` | 2.13 | Lex and parse the embedded Squeeze source recursively; store resulting AST as child |
| `SQZ<ident_ref>` | 2.14 | Parse `ident_ref`; resolve against `p.CodeSymbols` map at checker time |
| `MERGE<rule> = \| alts` | 2.15 | Identical parser effect to EXTEND; idempotent — skip if alternative already present |
| `UNQUOTE<expr>` | 2.16 | Parse `expr`, strip outer quote characters, store unquoted value in node |
| `HAS_ONE<type><expr>` | 2.17 | Parse `expr` (a list); verify at least one element matches `type` — emit `V13HasOneNode` |
| `SUBSET_OF<rule><expr>` | 2.18 | Parse `expr` (a list); verify all elements appear in the result set of `rule` — emit `V13SubsetOfNode` |
| `!WS!` | 1.1 | Disable whitespace skipping for the specific adjacency: call `p.curRaw()` / `p.advanceRaw()` instead of `p.cur()` / `p.advance()` |
| `§token` | 3.1 | The argument is a **rule reference**, not a value — store the rule name string, not a parsed instance |
| `$` | 3.2 | Emit a `V13SelfRefNode` at the call site; no sub-parse needed |
| `@` | 3.3 | Signals `inspect_type` — handled inside `ParseInspectType()` |
| `BOF` / `EOF` | 4.1–4.2 | Test `p.tokens[0].Type == V13_BOF` / `p.cur().Type == V13_EOF` |
| `include` | 5.1 | Spec-composition only; no runtime parser action |

**Directive discovery**: When reading a spec rule that contains an unfamiliar
directive keyword, search for it in all spec files:
```
grep_search(query: "DIRECTIVENAME", includePattern: "spec/**")
```
If the directive is not in the table above and not found in any spec file, it
may be a **proposed new directive**.  Do not implement it silently — see the
rule below.

**Proposing new directives**: If a parse situation seems to require a new
directive (a semantic constraint not expressible with existing ones),
**stop and surface it to the user** before writing any code:
- Describe what constraint you think is needed.
- Suggest the directive name, syntax, and spec section where it belongs.
- Wait for explicit user approval before adding it to any spec file or parser.

---

## Step 3 — Find all MERGE / EXTEND for the rule

MERGE and EXTEND in later spec files silently extends a rule defined
in an earlier file.  Always search before implementing a dispatcher.

```
grep_search(query: "MERGE<rule_name>|EXTEND<rule_name>", includePattern: "spec/**")
```

Example: `EXTEND<scope_body_item>` in `07_types_scope.sqg` and
`scope_body_catch` in `16_exceptions.sqg` both modify `scope_body` which is
defined in `07_types_scope.sqg`.

**What to do for each MERGE/EXTEND found:**

1. Identify the new alternative rule (e.g. `scope_body_catch`).
2. Check whether its `V13XxxNode` struct and `ParseXxx()` method exist
   (grep for the struct name in `pkg/parser/`).  If not, write a stub first
   (see Step 4 — Forward References).
3. Add a new `savePos/try/restorePos` branch to the **base rule's dispatcher**
   (the `ParseXxx()` for the original rule).
4. The new branch's code lives in the file that defines the new rule, but is
   called from the base rule's file — this is fine, it is one Go package.

**Ordering in the dispatcher matters:**

- More specific alternatives (unique first token) must appear **before** generic
  ones (e.g. before `ParseAssignment`).
- Placing a generic alternative first causes spurious matches and hides specific ones.

---

## Step 4 — Forward references: write stubs first

A forward reference occurs when rule A (in file N) references rule B that is
defined in a later file M (M > N).  In Go this is not a circular-import problem
(one package), but it causes build failures if the referenced type or method
doesn't exist yet.

**Protocol:**

1. Check if the forward-referenced struct and method already exist:
   ```
   grep_search(query: "V13XxxNode|ParseXxx", includePattern: "pkg/parser/**")
   ```
2. If missing, write the **minimum stub** in the target file before implementing
   the caller:

   ```go
   // V13XxxNode  rule_name — stub, full definition in spec/NN_file.sqg
   type V13XxxNode struct {
       V13BaseNode
   }

   // ParseXxx parses rule_name.
   // TODO: implement fully.
   func (p *V13Parser) ParseXxx() (*V13XxxNode, error) {
       return nil, p.errAt("ParseXxx: not yet implemented")
   }
   ```

3. Run `get_errors` — confirm the build is clean with the stub.
4. Only then implement the calling method that references `V13XxxNode` / `ParseXxx`.
5. Come back and implement the stub fully afterwards.

**Never** leave calls to non-existent methods between steps — the build must be
clean after every step.

---

## Step 4b — Code generation constraints

These rules are non-negotiable and apply to every line of parser code written:

1. **Grammar is the single source of truth.**  The spec rules in `spec/*.sqg`
   define what is valid Squeeze.  The parser must implement exactly what the spec
   says — no more, no less.

2. **Never extend the grammar inside the parser.**  If a construct does not parse
   correctly, the fix must be in the spec rule (with user approval via the
   grammar-change skill) or in a correct reading of the existing rule.  Do **not**
   add extra token checks, silent fallbacks, or special-case branches that accept
   inputs the spec does not allow.

3. **No quick-fix workarounds.**  If an existing parser method handles a case
   the spec does not define (e.g. accepting an extra token type to paper over a
   parse failure), remove the workaround and fix the spec or the parse logic
   correctly.  Document the removal in the commit or PR description.

4. **Directives must be implemented as specified.**  Do not approximate a
   directive's semantics.  If the full implementation is not feasible yet, emit a
   stub node and leave a `// TODO: directive not yet enforced` comment — never
   silently ignore the directive or implement a different semantic.

5. **Do not import packages not already in the file** without confirming the
   package is used for a spec-driven need.  (`fmt` and `strings` are always fine;
   `os` is only for debug output and must be removed before final review.)

---

## Step 5 — EBNF-to-Go coding patterns

Map each EBNF construct to the correct Go idiom:

| EBNF construct | Go pattern |
|---|---|
| `"literal"` terminal | `p.expect(V13_TOKEN)` — advances and returns token or error |
| `rule_name` (required) | `p.ParseRuleName()` — error stops parse |
| `[ optional_rule ]` | `saved := p.savePos()` / try / `if err != nil { p.restorePos(saved) }` |
| `alt1 \| alt2` | try alt1 with savePos/restorePos, fall through to alt2 |
| `item { item }` | `for p.cur().Type != stopTok { … }` loop |
| `{ ( EOL \| "," ) item }` | consume `V13_NL` or `V13_COMMA` then parse item inside loop |
| `{ scope_body_catch }` trailing suffix | second loop **after** the main closing brace |
| `UNIQUE<…>` | collect into `map[string]bool`, `return nil, p.errAt(…)` on duplicate |
| `TYPE_OF x<expr>` | parse `expr`, then type-assert the returned node kind matches `x` |

**Always capture position at method entry:**
```go
line, col := p.cur().Line, p.cur().Col
```
Use it when constructing the returned node:
```go
V13BaseNode: V13BaseNode{Line: line, Col: col},
```

**Use a plain `if p.cur().Type == …` guard** when the first token of an
alternative is unambiguous — it is cheaper and clearer than a speculative
savePos/try/restorePos.

---

## Step 6 — Discriminating tokens for `scope_body_item`

The `parseScopeBodyItem` dispatcher is called in a tight loop.  Use this table
to order the branches:

| First token | Alternative tried |
|---|---|
| `V13_MINUS` | `parsePrivateModifierItem` |
| `V13_LPAREN` | `ParseScopeInject`→`ParseScopeAssign`, then `ParseReceiverMethodAssign` |
| `V13_IDENT` value `"_"` | `ParseOtherInlineAssign`, fall back `ParseImportAssign` |
| `V13_IDENT` (URL follows `:`) | `ParseImportAssign` |
| `V13_IDENT` (general) | `ParseScopeAssign`, then `ParseAssignment` |
| `V13_CARET` | `ParseScopeBodyCatch` (trailing suffix; not a standalone item) |

When a MERGE/EXTEND adds a new alternative, insert it at the correct position in
this ordering — not at the bottom.

---

## Step 7 — Error message conventions

- Position-stamped error (no upstream cause):
  ```go
  return nil, p.errAt("rule_name: description, got %s %q", p.cur().Type, p.cur().Value)
  ```
- Wrapping an upstream error:
  ```go
  return nil, fmt.Errorf("rule_name: context: %w", err)
  ```
- Never expose internal parser state (token slice indices, Go type names) in error
  messages — only spec-level rule names and token values.

---

## Step 8 — After writing the method

In order:

1. `get_errors(filePaths: ["pkg/parser/"])` — fix all compile errors.
2. Confirm every struct field is assigned in the `ParseXxx()` body and no variables
   are declared but unused.
3. Remove any `fmt.Fprintf(os.Stderr, "[DEBUG]…")` trace lines.
4. Remove `"os"` from the file's import block if it was only used for debug output.
5. Update `docs/AST_spec.md` — table row and union-type prose (grammar-change step 2e).
6. Return to the grammar-change skill for remaining steps (checker spec, lib files).

---

## Step 9 — Debug flag protocol

### 9.1 — Adding the debug flag

`V13Parser` carries a `DebugFlag bool` field.  It is off by default.  When it is
true, every `ParseXxx()` method must emit structured trace output.

If `DebugFlag` is not yet present in the `V13Parser` struct in `parser_v13.go`,
add it:
```go
type V13Parser struct {
    tokens     []V13Token
    pos        int
    CastChains []V13CastDirective
    DebugFlag  bool   // enable parse trace output
    debugDepth int    // current nesting depth (internal)
}
```
Also add a constructor option:
```go
func (p *V13Parser) EnableDebug() { p.DebugFlag = true }
```

### 9.2 — Trace helper (add once to `parser_v13.go`)

```go
// debugPreview returns the first ≤20 characters of the token stream starting
// at the current position, with whitespace normalised to single spaces.
func (p *V13Parser) debugPreview() string {
    var sb strings.Builder
    for i := p.pos; i < len(p.tokens) && sb.Len() < 20; i++ {
        if sb.Len() > 0 {
            sb.WriteByte(' ')
        }
        s := p.tokens[i].Value
        if sb.Len()+len(s) > 20 {
            s = s[:20-sb.Len()]
        }
        sb.WriteString(s)
    }
    preview := sb.String()
    if len(preview) > 20 {
        preview = preview[:20]
    }
    return preview
}

// debugEnter prints entry for a parse rule and increments depth.
// Prints: indent → rule_name  [L<line>:C<col>  <20-char preview>]
// Returns a done function to be deferred; call it with the rule's error status.
func (p *V13Parser) debugEnter(rule string) func(ok bool) {
    if !p.DebugFlag {
        return func(bool) {}
    }
    tok := p.cur()
    indent := strings.Repeat("  ", p.debugDepth)
    fmt.Fprintf(os.Stderr, "%s→ %s  [L%d:C%d  %s]\n",
        indent, rule, tok.Line, tok.Col, p.debugPreview())
    p.debugDepth++
    return func(ok bool) {
        p.debugDepth--
        indent := strings.Repeat("  ", p.debugDepth)
        status := "OK"
        if !ok {
            status = "FAIL"
        }
        fmt.Fprintf(os.Stderr, "%s← %s  %s\n", indent, rule, status)
    }
}
```

### 9.3 — Using the helper in every ParseXxx

At the **top of every new or updated `ParseXxx()` method**, add exactly two lines:
```go
done := p.debugEnter("rule_name")
defer func() { done(err == nil) }()
```
The `defer` captures the named return value `err`; use a named return for
`ParseXxx` when `DebugFlag` is enabled:
```go
func (p *V13Parser) ParseScopeAssign() (node *V13ScopeAssignNode, err error) {
    done := p.debugEnter("scope_assign")
    defer func() { done(err == nil) }()
    // … rest of method …
}
```

### 9.4 — Failed-line printout

When the top-level `ParseParserRoot()` returns an error **and** `DebugFlag` is
true, print the source line that contains the failing token:
```go
if err != nil && p.DebugFlag {
    tok := p.cur()
    fmt.Fprintf(os.Stderr, "PARSE FAIL at L%d:C%d — unrecognised token %s %q\n",
        tok.Line, tok.Col, tok.Type, tok.Value)
    // The lexer stores source lines; access via p.sourceLines[tok.Line-1] if available.
}
```
If the lexer does not yet expose source lines, add a `SourceLines []string` field
to `V13Parser` and populate it from the input in `ParseV13FromSource`.

### 9.5 — Unknown token logging

The debug system must also catch tokens the parser encounters but has no rule
for.  This is different from a backtrack failure — it means a token arrived that
no active `ParseXxx()` can consume at all.

**Implementation — `trackUnknown` helper** (add once to `parser_v13.go`):

```go
// unknownTokens accumulates tokens that were not consumed by any parse method.
// Populated only when DebugFlag is true.
var unknownTokens []V13Token

// trackUnknown records a token that no parse method could consume.
// Call from any point where savePos/restorePos exhausts all alternatives
// without success and the token is not EOF.
func (p *V13Parser) trackUnknown(tok V13Token) {
    if !p.DebugFlag {
        return
    }
    unknownTokens = append(unknownTokens, tok)
    fmt.Fprintf(os.Stderr, "[UNKNOWN TOKEN] L%d:C%d  %s %q\n",
        tok.Line, tok.Col, tok.Type, tok.Value)
}
```

**Where to call it**: at the fall-through position of every dispatcher (the
point reached only when all alternatives have been tried and restored):

```go
// inside parseScopeBodyItem, after all savePos/restorePos attempts:
p.trackUnknown(p.cur())
return nil, p.errAt("scope_body_item: unrecognised token")
```

**Resolution check at top-level parse**: after `ParseParserRoot()` returns,
check whether any unknowns were logged that were not subsequently resolved
(i.e. consumed by a later parse attempt after a successful backtrack above them).
If any remain unresolved, they are hard errors:

```go
if len(unknownTokens) > 0 {
    for _, tok := range unknownTokens {
        fmt.Fprintf(os.Stderr, "UNRESOLVED TOKEN: L%d:C%d  %s %q\n",
            tok.Line, tok.Col, tok.Type, tok.Value)
    }
    return nil, fmt.Errorf("parse failed: %d unresolved token(s)", len(unknownTokens))
}
```

**Reset between parses**: clear `unknownTokens` at the start of each
`ParseParserRoot()` call so it does not accumulate across files.

**Relationship to version bumps**: after a V14 upgrade, running the parser
over all `pkg/lib/*.sqz` files with `DebugFlag = true` and inspecting the
`[UNKNOWN TOKEN]` list is the primary mechanism for detecting rules that were
deleted from the spec but whose syntax still appears in library files.

### 9.6 — Build and run

Build or run the debug binary directly with `go run`:
```powershell
go run ./cmd/v13parse pkg\lib\array.sqz
```
Enable debug output (add a `--debug` flag in `cmd/v13parse/main.go` that calls
`p.EnableDebug()` before parsing).

### 9.7 — Rule-targeted inline testing (`--token` / `--code`)

`cmd/v13parse` supports two flags for isolating parse failures **without editing
any source file or creating temporary files**:

| Flag | Purpose |
|---|---|
| `--token <rule>` | Use `<rule>` as the entry point instead of `parser_root` |
| `--code <string>` | Lex and parse this inline Squeeze snippet (no file needed) |
| `--list` | Print all available rule names and exit |

**Standard diagnostic workflow — use these in order:**

1. **Get the full error** from the file parse:
   ```powershell
   go run ./cmd/v13parse pkg/lib/array.sqz 2>&1 | Select-String "PARSE|Parse OK"
   ```

2. **Identify the failing grammar rule** from the error message (e.g. `L50:C8`
   → look up that line in the source, find which method covers it).

3. **Test the exact sub-expression in isolation** against the specific rule:
   ```powershell
   go run ./cmd/v13parse --token func_return_stmt --code "<- null"
   go run ./cmd/v13parse --token func_body_stmt --code "( LENGTH<arr.data> > 0 ) & <- arr.data[1]"
   go run ./cmd/v13parse --token func_unit --code "(
       ( LENGTH<arr.data> > 0 ) & <- arr.data[1]
       <- null
   )"
   ```
   If the sub-expression **passes**, the rule itself is fine — the problem is in
   a calling rule's dispatcher.  If it **fails**, it shows the exact token where
   the rule breaks.

4. **Walk up the call stack** by testing progressively larger context until the
   failure is localised:
   ```powershell
   go run ./cmd/v13parse --token func_body_stmt --code "start <= 0 & start = n + start"
   ```

5. **Confirm the fix** by retesting the exact same `--token`/`--code` invocation
   after the parser change, then re-run the full file.

**Rule** — do NOT proceed with a parser fix until `--token`/`--code` confirms
exactly which rule and which token is the sticking point.  Guessing the failure
source without an isolated test wastes implementation cycles.

**When `--code` input spans multiple lines** use a PowerShell here-string:
```powershell
$code = @"
(
    ( LENGTH<arr.data> > 0 ) & <- arr.data[1]
    <- null
)
"@
go run ./cmd/v13parse --token func_unit --code $code
```

**Common failure patterns and fixes:**

| Symptom | Likely cause |
|---|---|
| `expected identifier on LHS, got X` | Token `X` is a keyword (e.g. `m`=minutes, `s`=seconds); use `V13isIdentLike()` instead of `V13_IDENT` check |
| `expected '{', got …` | Caller consumed too many tokens before reaching `ParseScopeAssign`; check savePos/restorePos pairing |
| `not yet implemented` | A stub ParseXxx was called; implement or remove stub |
| Parse succeeds but node fields are nil | Optional sub-rule restorePos branching is wrong; enable `DebugFlag` and inspect the trace |

---

## Step 10 — Tests

### 10.1 — File and structure

All parser tests live in `spec_test/squeeze_v13_grammar_test.go`.  Tests are
grouped by spec file.  Each group has a header comment:
```go
// ===========================================================================
// Rule group name (NN_filename.sqg)
// ===========================================================================
```
When adding a test for a rule from `spec/07_types_scope.sqg`, add it inside the
`// 07_types_scope.sqg` group.  If the group does not exist, create it in
spec-file order.

### 10.2 — Required test helpers

Use the existing helpers already defined in the file:

```go
// parseRoot(src string) error        — lex + ParseParserRoot
// parseRHS(src string) error         — lex + ParseAssignRHS
// mustParseRHS(t, label, src)        — fail if parse errors
// mustFailRHS(t, label, src)         — fail if parse succeeds
```

For a new rule `ParseXxx`, add a matching helper if it is called from more than
one test:
```go
func parseXxx(src string) error {
    lex := parser.NewV13Lexer(src)
    toks, err := lex.V13Tokenize()
    if err != nil {
        return err
    }
    p := parser.NewV13Parser(toks)
    _, err = p.ParseXxx()
    return err
}
```

### 10.3 — Required test cases per rule

For every `ParseXxx()` written or updated, provide **at minimum**:

1. **Happy-path cases** — one test per distinct alternative in the spec rule.
2. **Optional-field absent** — confirm `[ optional ]` parses correctly without it.
3. **Repeated-zero-items** — confirm `{ item }` parses correctly with zero repetitions.
4. **Rejection cases** — at least one input that must fail, to guard against
   over-permissive parsing.
5. **MERGE/EXTEND alternatives** — one test per alternative added by a MERGE or EXTEND.

Table-driven style is preferred:
```go
func TestV13_ScopeAssign(t *testing.T) {
    cases := []struct {
        label string
        src   string
        valid bool
    }{
        {"empty body",      "myScope = {}", true},
        {"with inject",     "(x: myType) myScope = { x: 1 }", true},
        {"extend oper",     "myScope += { y: 2 }", true},
        {"missing body",    "myScope =", false},
    }
    for _, tc := range cases {
        err := parseRoot(tc.src)
        if tc.valid && err != nil {
            t.Errorf("%s: unexpected error: %v", tc.label, err)
        }
        if !tc.valid && err == nil {
            t.Errorf("%s: expected failure but succeeded", tc.label)
        }
    }
}
```

### 10.4 — Running tests

After writing tests, run only the affected test file:
```powershell
go test ./spec_test/ -run TestV13_YourRule -v
```
Then run the full suite to confirm no regressions:
```powershell
go test ./spec_test/ -v
```
All tests must pass before the implementation is considered complete.

### 10.5 — Debug-trace coverage test (enforces SIR-2)

A test must exist that verifies every exported `ParseXxx` method calls
`debugEnter`.  This is the build-time enforcement of SIR-2.

Add to `spec_test/squeeze_v13_grammar_test.go`:

```go
// TestAllParsersHaveDebugTrace verifies that every exported ParseXxx method
// on *V13Parser calls p.debugEnter (enforcing SIR-2 from the implement-parse-method skill).
// It does this by inspecting the source files via go/ast.
func TestAllParsersHaveDebugTrace(t *testing.T) {
    // Walk pkg/parser/*.go, collect exported methods named ParseXxx on *V13Parser,
    // and assert each contains a call to p.debugEnter.
    // Implementation uses go/parser + go/ast to read source without reflection.
    // Detailed implementation: search for "debugEnter" in parsed function bodies.
    // Fail with the rule name (extracted from the method name via camelCase split)
    // for each method that is missing the call.
    t.Log("SIR-2 coverage check: all ParseXxx methods must call debugEnter")
    // TODO: implement AST walk — stub here as a reminder.
}
```

Until the AST walk is implemented, manually verify after every new `ParseXxx`
by grepping:
```powershell
# Find ParseXxx methods that do NOT contain debugEnter
Get-ChildItem pkg\parser\parser_v15_*.go | ForEach-Object {
    $content = Get-Content $_ -Raw
    $methods = [regex]::Matches($content, 'func \(p \*V13Parser\) (Parse\w+)\(')
    foreach ($m in $methods) {
        $name = $m.Groups[1].Value
        # Extract method body (rough heuristic: next 20 lines after func decl)
        if ($content -notmatch "func \(p \*V13Parser\) $name\([\s\S]{0,500}?debugEnter") {
            Write-Warning "MISSING debugEnter: $name in $($_.Name)"
        }
    }
}
```

---

## Step 11 — Parse-error reporting blueprint

When parsing a `.sqz` file produces an error, report it in exactly this
four-section format.  **Do not speculate** — every piece of information must
come from the token stream and the parser source code.

### 11.1 — Required sections

#### 1. Short error framing (one sentence)
State the parse rule that failed and the token it rejected.

> `ParseAssignment` failed because `ParseAssignOper` saw `,` instead of an
> assignment operator.

#### 2. Failure position with code excerpt
Show the exact `L<n>:C<col>` position and mark it in the source line.

```
array.sqz  L27:C20

        data,  d:       @?
                    ^ col 20  ← ","  (V13_COMMA)  — expected assign_oper
```

#### 3. Call stack — last 10 parser functions / grammar rules
List as a numbered table, outermost call first, innermost (failing) call last.
Each row carries the Go method name and the grammar rule it implements.

| # | Method | Grammar rule |
|---|---|---|
| 1 | `ParseParserRoot` | `parser_root = import_assign \| scope_final` |
| 2 | `ParseScopeFinal` | `scope_final = scope_assign \| scope_merge_tail` |
| 3 | `ParseScopeAssign` | `scope_assign = [ scope_inject ] assign_lhs equal_assign scope_body` |
| 4 | `ParseScopeBody` | `scope_body = "{" ( scope_body_item \| private_block ) { (EOL\|",") … } "}"` |
| 5 | `parseScopeBodyItem` (→ `-` branch) | `scope_body_item = … \| private_block` |
| 6 | `parsePrivateModifierItem` | `private_block = "-" "(" body_items ")"` |
| 7 | `parsePrivateItems` (3rd iteration) | `{ (EOL\|",") scope_body_item }` |
| 8 | `ParseAssignment` | `assignment = [ private_modifier ] assign_lhs assign_oper assign_rhs` |
| 9 | `ParseAssignLHS` | `assign_lhs = UNIQUE<ident_name [, annotation]…>` — consumed `data`; annotation rejected; restores to `,` |
| 10 | `ParseAssignOper` ← **FAIL** | `assign_oper = incr… \| equal_assign` — got `,` |

#### 4. Root cause (two to four sentences)
Explain *why* the failing token arrived, tracing back to the lexer or grammar
rule that produced the wrong state.  Name the specific map, function, or rule
involved.

> `V13scanIdent` in `parser_v13.go` maps single-character identifiers `s m h d w`
> to duration-unit token types (`V13_DAY`, etc.) via `V13durationUnits`,
> unconditionally and regardless of context.  `parseV13AssignAnnotation` only
> accepts `V13_IDENT` for an `ident_name` annotation, so the `V13_DAY` token for
> `d` is rejected, the position is restored to the `,`, and `ParseAssignLHS`
> returns only `data`.  `ParseAssignOper` then sees `,` and fails.

---

### 11.2 — How to obtain the data

**Failure position**: the error message from `ParseParserRoot()` includes
`L<n>:C<col>` — that is the exact token position.

**Call stack**: run the parser with `p.EnableDebug()` to get the `→`/`←` trace
on stderr, then read backward from the last `← … FAIL` line to collect the
enclosing rule names in order.  Alternatively, trace manually through the source
code starting from `ParseParserRoot` and following the call path to the failing
`errAt`.

**Token stream at the failure point**: run `cmd/v13parse/main.go` and look for
lines matching `line <n>,` in the token dump printed before the parse section.
Examine the tokens on the failing line and the lines immediately before it.

**Root cause**: compare the token type shown in the error (e.g. `d`) with what
`V13_IDENT` would look like, then cross-reference the lexer's keyword and
specialised-token maps to explain the mismatch.

---

### 11.3 — Checklist before reporting

- [ ] Error position confirmed against the token dump (not guessed from the source line alone).
- [ ] All 10 call-stack entries have both the Go method name and the spec grammar rule.
- [ ] The code excerpt uses the actual source line from the `.sqz` file, not a paraphrase.
- [ ] Root cause names the exact Go symbol (map, function, token type) responsible.
- [ ] No speculation — every claim is backed by a token value or a line of Go source.
