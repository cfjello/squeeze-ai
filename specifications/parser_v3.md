# Parser Plan for the Squeeze Language (V3)

## 1. Grammar Classification

The grammar is **LL(k)** — a top-down recursive descent parser is the right fit. Most choices can be made with k=1 or k=2 lookahead. A few rules (`func_stmt`, `scope_unit` body, `single_logic_expr`) need small bounded lookahead to disambiguate alternatives that share a common first token. There is no LR-style shift/reduce conflict, so LR/LALR tooling is not needed and would be harder to integrate with the parser directive system.

---

## 2. Architecture: Three Distinct Phases

```
Source text
    │
    ▼
 Phase 1: Lexer / Tokenizer
    │
    ▼
 Phase 2: Recursive Descent Parser  →  Raw AST
    │
    ▼
 Phase 3: Directive Processor       →  Decorated / Validated AST
```

Each phase has a clearly scoped responsibility.

---

## 3. Phase 1 — Lexer

**Responsibilities:**
- Implicitly skip spaces and tabs between tokens
- Emit `NL` tokens from the `NL` regexp `/([ \t]*[\r\n]+)+/`
- Inject synthetic `BOF` token at the start of input and `EOF` at the end
- Recognize multi-character operators with **max-munch**: `..`, `->`, `>>`, `=>`, `<-`, `++`, `--`, `**`, `+:`, `-:`, `*:`, `/:`, `+~`, `-~`, `*~`, `/~`, `:~`, `>=`, `<=`, `!=`
- Recognize `//` as a distinct token (`func_regexp_decl`) — not a comment
- Tokenize regexp literals: `/` … `/` with optional flags `g`, `i` — needs special mode to avoid confusing `/` with the division operator
- Tokenize all three string quote styles: `'…'`, `"…"`, `` `…` ``
- Tokenize `@`-prefixed identifiers (`@name`, `@type`, `@storeName`, etc.) as distinct tokens to avoid ambiguity with `ident_name`
- **`!WS!` sentinel**: when encountered between two grammar items, suppresses whitespace skipping for that specific pair — the lexer needs a mode flag the parser can toggle

**Key challenge — identifiers with spaces:**
`ident_name = /(?<value>[\p{L}][\p{L}0-9_ ]*)/` — Unicode letters, digits, underscores, and embedded spaces are all valid. This means the lexer cannot tokenize identifiers naively; it must use greedy regexp matching and then trim trailing spaces. The `"."` separator in `ident_dotted` and operators serve as the natural terminators.

---

## 4. Phase 2 — Recursive Descent Parser

**One parsing function per grammar rule**, returning an AST node (or nil on non-match).

### Disambiguation Strategy

| Rule | Ambiguity | Resolution |
|---|---|---|
| `func_stmt` | `regexp` vs `/` numeric operator; `ident_ref` vs `func_call` vs `calc_unit` | Check next token: `/` starts regexp; `(` starts func_unit; `TYPE_OF` before ident starts func_call |
| `scope_unit` body | `assignment` vs `scope_assign` | Both start with `ident_name`; peek 2 ahead — `< ident` before `=` means `assignment`, `ident equal_assign "<"` means `scope_assign` |
| `single_logic_expr` | `ident_dotted` vs `TYPE_OF boolean<ident_ref>` vs `compare_expr` | `TYPE_OF` keyword disambiguates; otherwise try numeric/string first |
| `func_call_1` vs `func_call_2` | Both involve `TYPE_OF func_unit<ident_ref>` | `func_call_1` requires `->` or `>>` after args; `func_call_2` starts with `TYPE_OF` immediately |
| `<` token | compare operator vs scope_assign delimiter vs parser directive | Context-driven: after `equal_assign` at top level → scope delimiter; after `UNIQUE/RANGE/TYPE_OF/etc.` → directive bracket; inside expression → compare operator |

### Mutual Recursion Pairs

These must be handled carefully to avoid stack overflow on deeply nested input:
- `array_list` ↔ `array_values` ↔ `object_list`
- `num_grouping` ↔ `num_expr_list`
- `scope_assign` ↔ `scope_unit`
- `func_unit` ↔ `func_call_mixed_chain`

Standard recursive descent handles these naturally; no special treatment needed unless very deep nesting is expected (in which case an explicit stack could replace the call stack).

### Parser Directive Syntax in the Grammar

Rules like `UNIQUE<...>`, `TYPE_OF token<...>`, `RANGE n..m<...>` are **grammar-level constructs**, not user-level syntax. The parser must recognise the directive keywords as special tokens and parse the `<` … `>` brackets as directive argument wrappers — distinct from `compare_oper` use of `<` and `>`.

---

## 5. Phase 3 — Directive Processor

Runs as a **post-parse tree walk** over the raw AST. Each directive wrapping becomes a node type that the processor acts on:

| Directive | Processor action |
|---|---|
| `UNIQUE<…>` | Walk child nodes; fail AST node if any token value appears more than once |
| `RANGE n..m<…>` | Evaluate enclosed value; compare against bounds; fail node if out of range |
| `TYPE_OF token<…>` | Check inferred type of enclosed result matches `token`; attach type to AST node metadata |
| `TYPE_OF INFER<…>` | Infer type from enclosed expression; attach inferred type to AST node |
| `VALUE_OF<…>` | Tag node: return the value, not the reference |
| `ADDRESS_OF<…>` | Tag node: return a reference/address |
| `RETURN :<…>` / `RETURN ~<…>` | Tag node: immutable means VALUE_OF, mutable means ADDRESS_OF |
| `UNIFORM token<…>` / `UNIFORM INFER<…>` | Verify all elements of enclosed array/list share the same type |
| `INFER<…>` | Standalone type inference — look up `ident_ref.@type` or first-element type for arrays |

`INFER` requires a **symbol table** with type bindings, populated during the parse and queried during directive processing. This is the only part that requires inter-node communication during the tree walk.

---

## 6. `include` Pre-processing

Before parsing begins, resolve `include "./squeeze_v1.ebnf.txt"` by reading the referenced file and merging its rule definitions into the current rule set. This is a **pre-parser step**, not a runtime operation. Circular include detection is required.

---

## 7. AST Node Structure

Every node needs at minimum:
- **Rule name** (which production produced it)
- **Children** (ordered list of sub-nodes or token values)
- **Metadata bag**: for directive annotations — `@name`, `@type`, `@typeName`, `@data`, `@storeName`, `@ok`, `@error`, `@deps`
- **Source position** (line, column) for error reporting

---

## 8. Key Implementation Risks

| Risk | Mitigation |
|---|---|
| Identifier-with-spaces vs whitespace skipping | Lexer must consume greedily then check if next non-space char is a valid continuation |
| `<` overloaded in 3 contexts | Parser context flag: `in_directive`, `in_expression`, `in_scope` |
| `//` not a comment | Reserve as a token before any future comment syntax is added |
| `@storeName` vs `@store_name` inconsistency | Standardise before implementation begins |
| Deep mutual recursion | Monitor; add depth limit and clear error message |
| `BOF`/`EOF` injection | Lexer must guarantee exactly one `BOF` at stream start and one `EOF` at end |

---

## 9. Summary

| Component | Type |
|---|---|
| Lexer | Hand-written scanner with XRegExp, whitespace modes, `!WS!` flag |
| Parser | Hand-written recursive descent, LL(k≤2) |
| Directive processor | AST tree-walk pass |
| Type inference | Symbol table built during parse, queried in post-pass |
| Include resolution | Pre-parse file merging step |
