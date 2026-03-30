# Squeeze Parser V3 — User Guide

This guide explains how to use the Squeeze language parser, which lives in the `pkg/parser` package of the `github.com/cfjello/squeeze-ai` module, written in Go 1.22.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Quick Start](#2-quick-start)
3. [Architecture](#3-architecture)
4. [Lexer Reference](#4-lexer-reference)
5. [AST Node Reference](#5-ast-node-reference)
6. [Metadata Fields](#6-metadata-fields)
7. [Directives System](#7-directives-system)
8. [Traversing and Querying the Tree](#8-traversing-and-querying-the-tree)
9. [Error Handling](#9-error-handling)
10. [Writing Custom Tree Passes](#10-writing-custom-tree-passes)
11. [Parser Internals](#11-parser-internals)
12. [Grammar Quick Reference](#12-grammar-quick-reference)

---

## 1. Overview

The Squeeze parser converts Squeeze source text into an annotated Abstract Syntax Tree (AST). It is a hand-written, recursive-descent, LL(k≤2) parser that handles both V1 and V3 grammar constructs.

**What V1 provides**: literals, arithmetic, string, compare and logic expressions, assignment operators, identifiers (including names with embedded spaces), ranges.

**What V3 adds**: function units, scopes, tables, arrays, objects, empty declarators (`[]`, `{}`, `>>`, `//`, quoted forms), directives, function call chains, and parameter declarations.

---

## 2. Quick Start

### Importing

```go
import "github.com/cfjello/squeeze-ai/pkg/parser"
```

### Calling Parse

```go
result := parser.Parse(`
    x : 42
    y ~ "hello"
    z : x + 10
`)
```

### Checking for Errors

```go
if len(result.Errors) > 0 {
    for _, e := range result.Errors {
        fmt.Printf("line %d col %d: %s\n", e.Pos.Line, e.Pos.Col, e.Msg)
    }
}
```

### Walking the Result

```go
root := result.Root  // *parser.Node, Kind == NodeProgram

for _, stmt := range root.Children {
    fmt.Println(stmt.Kind)
}
```

### Pretty-Printing for Debugging

```go
fmt.Println(result.Root.Pretty())
```

---

## 3. Architecture

Parsing proceeds in three sequential phases:

```
Source text
    │
    ▼
┌────────────────────────────────┐
│  Phase 1 — Lexer               │
│  (parser_v3.go)                │
│  NewLexer(src).Tokenize()      │
│  Produces: []Token             │
└────────────────────────────────┘
    │
    ▼
┌────────────────────────────────┐
│  Phase 2 — Recursive Descent  │
│  (parser.go, rules_v1.go,      │
│   rules_v3.go, parse.go)       │
│  NewParser(tokens)             │
│  Produces: *Node (NodeProgram) │
└────────────────────────────────┘
    │
    ▼
┌────────────────────────────────┐
│  Phase 3 — Directive Processor │
│  (directive_processor.go)      │
│  NewDirectiveProcessor()       │
│  .Process(root)                │
│  Annotates Metadata fields,    │
│  checks constraints            │
└────────────────────────────────┘
    │
    ▼
ParseResult { Root *Node, Errors []ParseError }
```

All three phases are run automatically when you call `Parse()`.

---

## 4. Lexer Reference

The lexer is accessible independently if needed:

```go
l := parser.NewLexer(src)
tokens, err := l.Tokenize()
```

Every token stream begins with a synthetic `TOK_BOF` token at index 0, and ends with `TOK_EOF`.

### Token struct

```go
type Token struct {
    Type  TokenType
    Value string
    Line  int
    Col   int
}
```

### TokenType Groups

| Group | Constants |
|---|---|
| Control | `TOK_BOF`, `TOK_EOF`, `TOK_NL`, `TOK_ERROR` |
| Literals | `TOK_INTEGER`, `TOK_DECIMAL`, `TOK_SINGLE_QUOTED`, `TOK_DOUBLE_QUOTED`, `TOK_TMPL_QUOTED`, `TOK_REGEXP`, `TOK_BOOLEAN` |
| Identifiers | `TOK_IDENT`, `TOK_IDENT_DOTTED`, `TOK_IDENT_PREFIX`, `TOK_AT_IDENT` |
| Assignment ops | `TOK_ASSIGN_IMM` `:`), `TOK_ASSIGN_MUT` (`~`), `TOK_ASSIGN_READONLY_REF` (`<<<`), `TOK_INCR_ASSIGN_IMM` (`+:`), `TOK_DECR_ASSIGN_IMM` (`-:`), `TOK_MUL_ASSIGN_IMM` (`*:`), `TOK_DIV_ASSIGN_IMM` (`/:`) |
| Arithmetic ops | `TOK_PLUS`, `TOK_MINUS`, `TOK_STAR`, `TOK_SLASH`, `TOK_MODULO` |
| Compare ops | `TOK_EQ`, `TOK_NEQ`, `TOK_LT`, `TOK_GT`, `TOK_LTE`, `TOK_GTE` |
| Logic ops | `TOK_AND`, `TOK_OR`, `TOK_NOT` |
| Brackets | `TOK_LBRACKET`, `TOK_RBRACKET`, `TOK_LPAREN`, `TOK_RPAREN`, `TOK_LBRACE`, `TOK_RBRACE` |
| Delimiters | `TOK_COMMA`, `TOK_SEMICOLON`, `TOK_DOT`, `TOK_COLON` |
| Empty declarators | `TOK_EMPTY_ARR` (`[]`), `TOK_EMPTY_OBJ` (`{}`), `TOK_FUNC_STREAM_DECL` (`>>`), `TOK_REGEXP_DECL` (`//`), `TOK_FUNC_STR_SQ_DECL` (`''`), `TOK_FUNC_STR_DQ_DECL` (`""`), `TOK_FUNC_STR_TQ_DECL` (` `` `) |
| Special | `TOK_ARROW` (`=>`), `TOK_STREAM_IN` (`>`), `TOK_UNIFORM` (`UNIFORM`), `TOK_PIPE` (`\|`) |

### Lexer Notes

- `//` is **not** a comment — it tokenises as `TOK_REGEXP_DECL`, a function regexp declarator.
- Identifier names (`TOK_IDENT`) may contain embedded spaces. The lexer uses max-munch to read a multi-word identifier such as `my var name` as a single token.
- `@identifier` produces `TOK_AT_IDENT` (used for built-in parameters like `@name`, `@type`, `@data`, `@storeName`, `@ok`, `@error`, `@deps`).
- Empty declarators `[]` and `{}` produce single tokens, not bracket pairs.
- The disambiguation between `<` as *scope opener* versus `<` as *less-than* is handled in the parser via context flags, not the lexer.

---

## 5. AST Node Reference

### Node struct

```go
type Node struct {
    Kind     NodeKind
    Tok      Token    // terminal token (set for leaf nodes)
    Children []*Node
    Meta     Metadata
    Pos      Pos
}
```

`IsError()` returns true when `Kind == NodeError`. Terminal (leaf) nodes carry their token in `Tok`; inner nodes carry sub-trees in `Children`.

### Constructors (internal use / testing)

```go
NewTokenNode(tok Token) *Node           // leaf from token
NewNode(kind NodeKind, pos Pos, children ...*Node) *Node
NewDirectiveNode(kind NodeKind, arg string, pos Pos, enclosed *Node) *Node
NewErrorNode(msg string, pos Pos) *Node
```

### NodeKind Groups

#### Top-level

| Kind | Description |
|---|---|
| `NodeProgram` | Root of every parse tree; children are top-level statements |

#### Leaf / Error

| Kind | Description |
|---|---|
| `NodeToken` | Generic terminal node wrapping a single token |
| `NodeError` | Error recovery node; `Tok.Value` holds the error message |

#### V1 — Literals

| Kind | Description |
|---|---|
| `NodeDigits` | Unsigned integer digits |
| `NodeInteger` | Signed integer (optional leading `-`) |
| `NodeDecimal` | Decimal number |
| `NodeNumericConst` | Named numeric constant |
| `NodeSingleQuoted` | `'...'` string literal |
| `NodeDoubleQuoted` | `"..."` string literal |
| `NodeTmplQuoted` | `` `...` `` template string |
| `NodeString` | Any string variant |
| `NodeRegexpFlags` | Flags suffix on a regex (`i`, `g`, etc.) |
| `NodeRegexp` | Regular expression |
| `NodeBoolean` | Boolean value |
| `NodeBooleanTrue` | Literal `true` |
| `NodeBooleanFalse` | Literal `false` |
| `NodeConstant` | Any literal constant (wraps one of the above) |

#### V1 — Identifiers

| Kind | Description |
|---|---|
| `NodeIdentName` | Simple or multi-word identifier (`foo`, `my var`) |
| `NodeIdentDotted` | Dot-path identifier (`a.b.c`) |
| `NodeIdentPrefix` | Prefix-scoped identifier (`scope::name`) |
| `NodeIdentRef` | Any identifier form used as a reference |

#### V1 — Numeric Expressions

| Kind | Description |
|---|---|
| `NodeNumericOper` | `+`, `-`, `*`, `/`, `%` |
| `NodeInlineIncr` | Inline increment/decrement |
| `NodeSingleNumExpr` | A single term in a numeric expression |
| `NodeNumExprList` | Sequence of numeric terms and operators |
| `NodeNumGrouping` | Parenthesised numeric sub-expression |
| `NodeNumericExpr` | Complete numeric expression (top of subtree) |

#### V1 — String Expressions

| Kind | Description |
|---|---|
| `NodeStringOper` | String concatenation operator |
| `NodeStringExprList` | Sequence of string terms and operators |
| `NodeStringGrouping` | Parenthesised string sub-expression |
| `NodeStringExpr` | Complete string expression |

#### V1 — Comparison Expressions

| Kind | Description |
|---|---|
| `NodeCompareOper` | `==`, `!=`, `<`, `>`, `<=`, `>=` |
| `NodeNumCompare` | Numeric comparison |
| `NodeStringCompare` | String comparison |
| `NodeStringRegexpComp` | String-to-regexp comparison |
| `NodeCompareExpr` | Complete comparison expression |

#### V1 — Logic Expressions

| Kind | Description |
|---|---|
| `NodeNotOper` | Unary `not` / `!` |
| `NodeLogicOper` | `&&`, `\|\|` |
| `NodeSingleLogicExpr` | One logic operand |
| `NodeLogicExprList` | Sequence of logic operands and operators |
| `NodeLogicGrouping` | Parenthesised logic sub-expression |
| `NodeLogicExpr` | Complete logic expression |

#### V1 — Assignment Operators & Range

| Kind | Description |
|---|---|
| `NodeIncrAssignImmutable` | `+:` `-:` `*:` `/:` applied immutably |
| `NodeIncrAssignMutable` | Mutable increment form |
| `NodeAssignMutable` | `~` assignment |
| `NodeAssignImmutable` | `:` assignment |
| `NodeAssignReadOnlyRef` | `<<<` read-only reference |
| `NodeEqualAssign` | `=` plain equality assignment |
| `NodeAssignOper` | Any assignment operator node |
| `NodeRange` | `n..m` range literal |

#### V3 — Empty Declarators

| Kind | Description |
|---|---|
| `NodeEmptyArrayDecl` | `[]` — declares an empty array type |
| `NodeFuncStreamDecl` | `>>` — declares a stream function signature |
| `NodeFuncRegexpDecl` | `//` — declares a regexp function signature |
| `NodeEmptyObjectDecl` | `{}` — declares an empty object type |
| `NodeFuncStringDecl` | `''`, `""`, or ` `` ` — string function signature |
| `NodeEmptyDecl` | Any empty declarator (wraps one of the above) |

#### V3 — Calc Unit, LHS/RHS

| Kind | Description |
|---|---|
| `NodeCalcUnit` | Inline calc sub-expression |
| `NodeAssignLHS` | Left-hand side of an assignment (one or more targets) |
| `NodeAssignRHS` | Right-hand side of an assignment |
| `NodeAssignRHS4Object` | RHS when the value is an object |

#### V3 — Arrays

| Kind | Description |
|---|---|
| `NodeArrayValues` | Comma-separated value list inside an array |
| `NodeArrayBegin` | `[` opening bracket node |
| `NodeArrayEnd` | `]` closing bracket node |
| `NodeArrayInit` | Complete array initialiser |
| `NodeArrayList` | Array with multiple items |
| `NodeArrayLookup` | Array index lookup expression |

#### V3 — Objects

| Kind | Description |
|---|---|
| `NodeObjectInit` | Object literal `{ key: value, … }` |
| `NodeObjectList` | Object member list |
| `NodeObjectLookup` | Object property access |

#### V3 — Tables

| Kind | Description |
|---|---|
| `NodeTableHeader` | Column header row |
| `NodeTableEntry` | One row in a table |
| `NodeTableObjects` | Collection of table rows |
| `NodeTableInit` | Complete table initialiser (header + rows) |

#### V3 — Getter Annotations

| Kind | Description |
|---|---|
| `NodeGetName` | `@name` built-in getter |
| `NodeGetData` | `@data` built-in getter |
| `NodeGetStoreName` | `@storeName` built-in getter |
| `NodeGetType` | `@type` built-in getter |
| `NodeGetTypeName` | `@typeName` built-in getter |

#### V3 — Function Header

| Kind | Description |
|---|---|
| `NodeDataAssign` | `@data` assignment inside header |
| `NodeStoreNameAssign` | `=>` store-name assignment inside header |
| `NodeFuncHeaderBuildinParams` | Built-in `@`-parameter block |
| `NodeFuncHeaderAssign` | Complete function header assignment |
| `NodeFuncHeaderUserParams` | User-defined parameter block |

#### V3 — Static Identifiers

| Kind | Description |
|---|---|
| `NodeIdentStaticStoreName` | Static store-name reference |
| `NodeIdentStaticStr` | Static string reference |
| `NodeIdentStaticBoolean` | Static boolean reference |
| `NodeIdentStaticError` | Static error reference |
| `NodeIdentStaticDeps` | Static dependencies reference |
| `NodeIdentStaticFunc` | Static `func` keyword usage |
| `NodeIdentStaticFn` | Static `fn` keyword usage |

#### V3 — Function Arguments

| Kind | Description |
|---|---|
| `NodeFuncArgs` | Positional argument list |
| `NodeFuncStreamArgs` | Streaming argument list |
| `NodeFuncDeps` | Dependency identifiers |
| `NodeFuncArgsDecl` | Full argument declaration block |

#### V3 — Function Range Arguments

| Kind | Description |
|---|---|
| `NodeFuncFixedNumRange` | Fixed numeric range `n..m` in arg position |
| `NodeFuncFixedStrRange` | Fixed string range |
| `NodeFuncFixedListRange` | Fixed list range |
| `NodeFuncRangeArgs` | Any range-based argument specification |

#### V3 — Function Calls

| Kind | Description |
|---|---|
| `NodeFuncCallArgs` | Arguments passed at a call site |
| `NodeFuncCall1` | Simple function call form 1 |
| `NodeFuncCall2` | Simple function call form 2 |
| `NodeFuncCallDataChain` | Data-streaming call chain |
| `NodeFuncCallLogicChain` | Logic-branching call chain |
| `NodeFuncCallMixedChain` | Mixed data+logic call chain |

#### V3 — Function Body Statements

| Kind | Description |
|---|---|
| `NodeFuncStreamLoop` | `>>` streaming loop statement |
| `NodeFuncStmt` | General function body statement |
| `NodeAssignment` | Full assignment statement |
| `NodeFuncReturnStmt` | `return` statement |
| `NodeFuncStoreStmt` | Store (`=>`) statement |
| `NodeFuncBodyStmt` | Wrapper for any body statement |

#### V3 — Function Units

| Kind | Description |
|---|---|
| `NodeFuncPrivateParams` | Private parameter block |
| `NodeFuncUnitHeader` | Function unit header |
| `NodeFuncUnit` | Complete function unit definition |

#### V3 — Scopes

| Kind | Description |
|---|---|
| `NodeScopeAssign` | `name : < … >` scope block assignment |
| `NodeScopeUnit` | Inner scope body |

#### Directives

| Kind | Description |
|---|---|
| `NodeDirectiveUnique` | `UNIQUE<…>` — child identifiers must be distinct |
| `NodeDirectiveRange` | `RANGE n..m<…>` — value must lie within `[n, m]` |
| `NodeDirectiveTypeOf` | `TYPE_OF token<…>` — annotates expected type |
| `NodeDirectiveValueOf` | `VALUE_OF<…>` — dereference by value |
| `NodeDirectiveAddressOf` | `ADDRESS_OF<…>` — take address |
| `NodeDirectiveReturn` | `RETURN:<…>` or `RETURN~<…>` — return mode |
| `NodeDirectiveUniform` | `UNIFORM token<…>` — all elements same type |
| `NodeDirectiveInfer` | `INFER<…>` — infer type from symbol table |

---

## 6. Metadata Fields

Every node carries a `Metadata` struct, populated by the parser and the Directive Processor.

```go
type Metadata struct {
    // Populated during parsing (from @-prefixed identifiers)
    Name      *string  // @name  — the identifier name as written
    TypeRef   *string  // @type  — reference to a named type
    TypeName  *string  // @typeName — static type name literal
    Data      *Node    // @data  — attached sub-tree
    StoreName *string  // @storeName — store operation name (=> operator)
    Ok        *bool    // @ok    — last operation success flag
    Err       *string  // @error — last operation error string
    Deps      []string // @deps  — list of dependency names

    // Populated during Phase 3 (Directive Processor)
    DirectiveKind NodeKind // Which directive wraps this node (if any)
    DirectiveArg  string   // Directive parameter ("1..10", "string", ":", "~", …)
    InferredType  string   // Type inferred by INFER directive
    IsValueOf     bool     // Set by VALUE_OF or RETURN:
    IsAddressOf   bool     // Set by ADDRESS_OF or RETURN~
}
```

Pointer fields are `nil` when not set. Check `Meta.TypeRef != nil` before dereferencing.

---

## 7. Directives System

Directives are grammar-level annotations written inside the Squeeze source. They look like `DIRECTIVE_NAME<enclosed expression>`. The Directive Processor (Phase 3) walks the tree and validates or annotates each directive node.

### UNIQUE

Ensures all identifier leaves within the directive brackets have distinct values.

```
x, x : 42    →  error: UNIQUE violation (duplicate "x")
x, y : 42    →  OK
```

Use case: enforcing that LHS assignment targets are non-overlapping.

### RANGE `n..m`

The first numeric leaf inside the brackets must lie within `[n, m]` (inclusive).

```
RANGE 1..10<someVar>   →  error if someVar resolves to a number outside [1,10]
```

After processing, `child.Meta.DirectiveArg` holds the string `"1..10"`.

### TYPE_OF `typeName`

Annotates the enclosed node with a type reference.

```
TYPE_OF string<expr>
```

After processing, `child.Meta.TypeRef` points to the string `"string"`.

### VALUE_OF

Marks the enclosed node as a value dereference.

After processing, `child.Meta.IsValueOf = true`.

### ADDRESS_OF

Marks the enclosed node as an address-of operation.

After processing, `child.Meta.IsAddressOf = true`.

### RETURN `:` or `~`

`RETURN:<expr>` sets `child.Meta.IsValueOf = true`.  
`RETURN~<expr>` sets `child.Meta.IsAddressOf = true`.

### UNIFORM `tokenType`

Checks that every direct leaf token inside the brackets has the same token type. Errors if the collection is heterogeneous.

```
UNIFORM integer<1, 2, 3>   →  OK
UNIFORM integer<1, "two">  →  error
```

### INFER

Looks up the enclosed identifier in the symbol table (built from seen assignments). If found, sets `child.Meta.InferredType` to the resolved type. Falls back to the type of the first leaf token.

```
x : 42
INFER<x>    →  child.Meta.InferredType = "integer"
```

---

## 8. Traversing and Querying the Tree

### Walk

`Walk` does a depth-first pre-order traversal, calling `fn` on every node including the receiver. Return `false` from `fn` to stop early.

```go
root.Walk(func(n *parser.Node) bool {
    if n.Kind == parser.NodeAssignment {
        fmt.Println("assignment at", n.Pos)
    }
    return true  // continue
})
```

### FindAll

`FindAll` collects all descendant nodes (inclusive) matching any of the given kinds.

```go
assignments := root.FindAll(parser.NodeAssignment, parser.NodeScopeAssign)
for _, a := range assignments {
    fmt.Println(a.Kind, a.Pos)
}
```

### Append

`Append` adds children to a node (used internally when building the tree, but also useful in custom passes):

```go
parent.Append(child1, child2)
```

### Pretty

`Pretty` returns a multi-line indented string representation of the subtree. Useful for debugging:

```go
fmt.Println(node.Pretty())
```

### IsError

```go
if node.IsError() {
    fmt.Println("error node:", node.Tok.Value)
}
```

### Checking NodeKind as a string

```go
fmt.Println(node.Kind.String())  // e.g. "NodeAssignment"
```

---

## 9. Error Handling

### ParseResult

```go
type ParseResult struct {
    Root   *Node
    Errors []ParseError
}
```

`Root` is always non-nil after a successful call to `Parse()` — even when errors are present — because the parser recovers and continues after each error.

### ParseError

```go
type ParseError struct {
    Pos Pos    // Line and Col, 1-based
    Msg string
}
```

### Error Sources

| Phase | How errors arise |
|---|---|
| Phase 1 (Lexer) | Unrecognised characters, unterminated strings |
| Phase 2 (Parser) | Unexpected tokens, missing delimiters, depth limit (512) exceeded |
| Phase 3 (Directives) | UNIQUE duplicates, RANGE out-of-bounds, UNIFORM heterogeneity |

All errors from all three phases are aggregated into `ParseResult.Errors`.

### Error Recovery

The parser recovers from unexpected tokens by skipping to the next end-of-line marker (`\n`, `;`, or EOF). This means a single malformed statement does not abort the entire parse — subsequent valid statements are still parsed and included in the tree.

```go
result := parser.Parse("42\nx : 1")
// result.Errors contains an error about bare literal "42"
// result.Root.Children still contains the NodeAssignment for "x : 1"
```

---

## 10. Writing Custom Tree Passes

After calling `Parse()`, you can inspect or transform the tree using `Walk` or `FindAll`.

### Example: Collect all assigned names

```go
result := parser.Parse(src)

var names []string
result.Root.Walk(func(n *parser.Node) bool {
    if n.Kind == parser.NodeAssignLHS {
        n.Walk(func(child *parser.Node) bool {
            if child.Kind == parser.NodeIdentName {
                names = append(names, child.Tok.Value)
            }
            return true
        })
        return false // don't recurse further into this subtree
    }
    return true
})
```

### Example: Find all scope assignments

```go
scopes := result.Root.FindAll(parser.NodeScopeAssign)
for _, s := range scopes {
    // s.Children[0] is typically the LHS name node
    fmt.Println("scope:", s.Children[0].Tok.Value)
}
```

### Example: Run the Directive Processor as a standalone pass

In most cases you won't need this — `Parse()` runs Phase 3 automatically. If you build a subtree manually during testing, you can run the processor directly:

```go
dp := parser.NewDirectiveProcessor()
dp.Process(mySubtree)
if dp.HasErrors() {
    for _, e := range dp.Errors() {
        fmt.Println(e.Msg)
    }
}
```

---

## 11. Parser Internals

This section is for contributors or advanced users who need to extend the grammar.

### File Layout

| File | Responsibility |
|---|---|
| `parser_v3.go` | Phase 1: Lexer — `Lexer`, `Token`, `TokenType`, `Tokenize()` |
| `ast.go` | AST types — `Node`, `NodeKind`, `Metadata`, `Pos`, constructors, helpers |
| `parser.go` | Phase 2: Parser struct, navigation, match/consume, backtracking, error helpers |
| `rules_v1.go` | V1 grammar productions (43 rule functions) |
| `rules_v3.go` | V3 grammar productions (~45 rule functions) |
| `parse.go` | Public `Parse()` entry point, start rule `parseProgram()`, error recovery |
| `directive_processor.go` | Phase 3: Directive Processor, symbol table, 8 directive handlers |

### Backtracking / Try Pattern

Rule functions follow one of two conventions:

- **Return nil** on mismatch — the parser backtracks to the saved position.
- **Return an error node** on a committed mismatch (after consuming at least one token that commits to this alternative).

The `try` helper wraps a rule and discards both the result and any errors if the rule returns nil:

```go
if n := p.try(p.parseScopeAssign); n != nil {
    return n
}
if n := p.try(p.parseAssignment); n != nil {
    return n
}
```

`try` saves the cursor and restores it (including context flags) if the function returns nil. This supports LL(2) disambiguation for rules whose prefixes overlap.

### Depth Guard

The parser enforces a maximum recursion depth of 512 via `enter(ruleName) bool` / `leave()`. `enter` returns `false` when the limit is exceeded, causing the rule to return nil and preventing stack overflow on adversarial or deeply nested input.

### Context Flags

Two boolean flags control disambiguation inside the parser state:

- `inExpression` — set when parsing the RHS of an assignment; suppresses certain scope-level interpretations.
- `inScope` — set when parsing inside a `< … >` scope block; makes `<` tokens act as scope delimiters rather than less-than operators.

These are managed via `withExpression(fn)` and `withScope(fn)` combinators and are automatically restored by the backtracking machinery.

---

## 12. Grammar Quick Reference

### End-of-Line (EOL)

A statement ends at a newline (`\n` / `\r\n`), a semicolon `;`, or `EOF`. Blank lines and leading whitespace are ignored.

### Assignment Operators

| Operator | NodeKind | Meaning |
|---|---|---|
| `:` | `NodeAssignImmutable` | Bind immutably |
| `~` | `NodeAssignMutable` | Bind mutably |
| `<<<` | `NodeAssignReadOnlyRef` | Bind as read-only reference |
| `+:` | `NodeIncrAssignImmutable` | Add and bind immutably |
| `-:` | `NodeIncrAssignImmutable` | Subtract and bind immutably |
| `*:` | `NodeIncrAssignImmutable` | Multiply and bind immutably |
| `/:` | `NodeIncrAssignImmutable` | Divide and bind immutably |

### Identifier Rules

- Simple identifier: one or more words separated by spaces — `foo`, `my var`, `total count`.
- Dotted identifier: dot-separated path — `a.b.c`.
- Prefix-scoped: `scope::name`.
- Built-in `@`-identifiers: `@name`, `@type`, `@typeName`, `@data`, `@storeName`, `@ok`, `@error`, `@deps`.

### Expression Precedence (lowest to highest)

1. Logic expression (`&&`, `||`, `!`)
2. Comparison expression (`==`, `!=`, `<`, `>`, `<=`, `>=`)
3. String expression (concatenation)
4. Numeric expression (`+`, `-`, `*`, `/`, `%`)
5. Unary / inline increment
6. Primary (literal, identifier, grouped expression)

### Scope Syntax

```
scopeName : < statement1 \n statement2 \n … >
```

The `<` and `>` delimiters are context-sensitive: inside an assignment RHS they act as comparison operators; after a `:` at the top level they open a scope block.

### Empty Declarators

| Syntax | TokenType | NodeKind |
|---|---|---|
| `[]` | `TOK_EMPTY_ARR` | `NodeEmptyArrayDecl` |
| `{}` | `TOK_EMPTY_OBJ` | `NodeEmptyObjectDecl` |
| `>>` | `TOK_FUNC_STREAM_DECL` | `NodeFuncStreamDecl` |
| `//` | `TOK_REGEXP_DECL` | `NodeFuncRegexpDecl` |
| `''` | `TOK_FUNC_STR_SQ_DECL` | `NodeFuncStringDecl` |
| `""` | `TOK_FUNC_STR_DQ_DECL` | `NodeFuncStringDecl` |
| ` `` ` | `TOK_FUNC_STR_TQ_DECL` | `NodeFuncStringDecl` |

These declarators are used in function signatures to declare the shape of a parameter or return value without providing a concrete value.

### Range Literal

```
1..10     →  NodeRange  (from 1 to 10 inclusive)
```

Used both as an assignment RHS and as a directive argument in `RANGE n..m<…>`.
