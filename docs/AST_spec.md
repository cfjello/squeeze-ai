# Squeeze V10 Parser — AST Node Specification

This document lists every AST node type produced by the V10 recursive-descent
parser (`pkg/parser/parser_v10*.go`).  Nodes are grouped by the grammar spec
section in which they are defined.

All concrete node types implement the `V10Node` interface:

```go
type V10Node interface {
    v10NodePos() (line int, col int)
}
```

---

## Contents

1. [Base node](#1-base-node)
2. [§01 Primitives & Constants](#2-01-primitives--constants)
3. [§02 Identifiers & Expressions](#3-02-identifiers--expressions)
4. [§03 Assignment](#4-03-assignment)
5. [§04 Objects & Arrays](#5-04-objects--arrays)
6. [§05 JSON Path](#6-05-json-path)
7. [§06 Functions](#7-06-functions)
8. [§07 Scope & Root](#8-07-scope--root)
9. [Helper structs](#9-helper-structs)
10. [Enum types](#10-enum-types)
11. [V10Node variant reference](#11-v10node-variant-reference)

---

## 1. Base node

Every node struct embeds `v10BaseNode`, which records the **source position**
of the first token that belongs to the node.

```go
type v10BaseNode struct {
    Line int
    Col  int
}
```

---

## 2. §01 Primitives & Constants

*Source: `parser_v10.go`, spec `01_definitions.sqg`*

### Numeric

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10IntegerNode` | `Sign string` · `Digits string` | `integer` |
| `V10DecimalNode` | `Sign string` · `Integral string` · `Frac string` | `decimal` |
| `V10NumericConstNode` | `Value V10Node` ← `*V10IntegerNode \| *V10DecimalNode` | `numeric_const` |
| `V10IntTypeNode` | `Kind string` (e.g. `"int8"`) · `Value *V10IntegerNode` | `int_type` |
| `V10NanNode` | *(no fields)* | `nan` |
| `V10InfinityNode` | `Sign string` (`""` or `"-"`) | `infinity` |
| `V10FloatNode` | `Kind string` · `Value V10Node` | `float_type` |
| `V10DecimalTypeNode` | `Kind string` · `Integral *V10IntegerNode` · `Frac string` | `decimal_type` |
| `V10DecimalNumNode` | `Value *V10DecimalTypeNode` | `decimal_num` |

### Strings

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10StringNode` | `Kind V10StringKind` · `Raw string` · `Parts []V10TmplPart` | `string` |
| `V10CharNode` | `Value rune` | `char` |

`V10StringNode.Parts` is populated only for template strings; for plain strings
`Raw` holds the decoded value and `Parts` is nil.

### Date / Time

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10DateNode` | `Year string` · `Month string` · `Day string` | `date` |
| `V10TimeNode` | `Hour string` · `Minute string` · `Second string` · `Millis string` | `time` |
| `V10DateTimeNode` | `Date *V10DateNode` · `Time *V10TimeNode` | `date_time` |
| `V10TimeStampNode` | `Date *V10DateNode` · `Time *V10TimeNode` | `timestamp` |
| `V10DurationNode` | `Segments []V10DurationSegment` | `duration` |

### Regexp

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10RegexpNode` | `Pattern string` · `Flags string` | `regexp` |

### Boolean / Null

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10BoolNode` | `Value bool` | `boolean` |
| `V10NullNode` | *(no fields)* | `null` |

### Structural primitives

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10CardinalityNode` | `Lo string` · `Hi string` | `cardinality` |
| `V10RangeNode` | `Lo *V10IntegerNode` · `Hi *V10IntegerNode` | `range` |

### URIs & identifiers

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10UUIDNode` | `Value string` | `uuid` |
| `V10UUIDV7Node` | `Value string` | `uuid_v7` |
| `V10URLNode` | `Value string` | `http_url` |
| `V10FilePathNode` | `Value string` | `file_path` |

### Constant wrapper

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10ConstantNode` | `Value V10Node` | `constant` |

`Value` holds any of the leaf types above.

---

## 3. §02 Identifiers & Expressions

*Source: `parser_v10_operators.go`, spec `02_operators.sqg`*

### Identifiers

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10IdentDottedNode` | `Parts []string` | `ident_dotted` |
| `V10IdentPrefixNode` | `Value string` | `ident_prefix` |
| `V10IdentRefNode` | `Prefix *V10IdentPrefixNode` · `Dotted *V10IdentDottedNode` | `ident_ref` |

`Prefix` is nil when there is no sigil prefix.

### Numeric expressions

| Type | Fields | Notes |
|------|--------|-------|
| `V10SingleNumExprNode` | `Literal *V10NumericConstNode` · `InlineIncr string` · `IdentRef *V10IdentRefNode` | exactly one of `Literal`/`IdentRef` is non-nil |
| `V10NumExprListNode` | `Terms []V10NumExprTerm` | flat `+`/`-` chain |
| `V10NumGroupingNode` | `Terms []V10NumGroupTerm` | parenthesised group |
| `V10NumericExprNode` | `Value V10Node` ← `*V10NumExprListNode \| *V10NumGroupingNode` | `numeric_expr` |

### String expressions

| Type | Fields | Notes |
|------|--------|-------|
| `V10StringExprListNode` | `Terms []V10StringExprTerm` | flat `+` chain |
| `V10StringGroupingNode` | `Terms []V10StringGroupTerm` | parenthesised group |
| `V10StringExprNode` | `Value V10Node` ← `*V10StringExprListNode \| *V10StringGroupingNode` | `string_expr` |

### Comparison expressions

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10NumCompareNode` | `Left *V10NumericExprNode` · `Oper string` · `Right *V10NumericExprNode` | `num_compare` |
| `V10StringCompareNode` | `Left *V10StringExprNode` · `Oper string` · `Right *V10StringExprNode` | `string_compare` |
| `V10StringRegexpCompNode` | `Left *V10StringExprNode` · `Regexp *V10RegexpNode` | `string_regexp_comp` |
| `V10CompareExprNode` | `Value V10Node` ← any compare above | `compare_expr` |

### Logic expressions

| Type | Fields | Notes |
|------|--------|-------|
| `V10SingleLogicExprNode` | `Negated bool` · `IdentRef *V10IdentRefNode` · `Compare *V10CompareExprNode` · `Numeric *V10NumericExprNode` · `StringVal *V10StringExprNode` | exactly one of the last four is non-nil |
| `V10LogicExprListNode` | `Terms []V10LogicExprTerm` | flat `&&`/`\|\|` chain |
| `V10LogicGroupingNode` | `Terms []V10LogicGroupTerm` | parenthesised group |
| `V10LogicExprNode` | `Value V10Node` ← `*V10LogicExprListNode \| *V10LogicGroupingNode` | `logic_expr` |

### Top-level expression

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10CalcUnitNode` | `Value V10Node` ← `*V10NumericExprNode \| *V10StringExprNode \| *V10LogicExprNode` | `calc_unit` |

---

## 4. §03 Assignment

*Source: `parser_v10_assignment.go`, spec `03_assignment.sqg`*

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10AssignVersionNode` | `Raw string` (e.g. `"v1.0.2"`) | `assign_version` |
| `V10AssignOperNode` | `Kind V10AssignOperKind` · `Value string` | `assign_oper` |
| `V10AssignLHSNode` | `Name string` · `Annotations []V10AssignAnnotation` | `assign_lhs` |
| `V10AssignRHSNode` | `Value V10Node` ← `*V10ConstantNode \| *V10CalcUnitNode` | `assign_rhs` |
| `V10AssignmentNode` | `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `RHS *V10AssignRHSNode` | `assignment` |

`V10AssignLHSNode.Annotations` holds 0–3 entries; each `V10AssignAnnotation`
has exactly one of `IdentName string`, `Cardin *V10CardinalityNode`, or
`Version *V10AssignVersionNode` set.

---

## 5. §04 Objects & Arrays

*Source: `parser_v10_objects.go`, spec `04_objects.sqg`*

### Empty declarations

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10EmptyDeclNode` | `Kind V10EmptyDeclKind` | `empty_decl` |

### TYPE_OF reference

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10TypeOfRefNode` | `TypeName string` · `Ref *V10IdentRefNode` | `TYPE_OF typeName<ident_ref>` |

`TypeName` is the grammar-rule name used as the type annotation (e.g.
`"array_list"`, `"object_final"`, `"func_unit"`).

### Arrays

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10ArrayValueNode` | `Value V10Node` | `array_value` |
| `V10ArrayUniformNode` | `Items []V10Node` | `array_uniform` / `array_list` |
| `V10ArrayAppendTailNode` | `Arrays []*V10ArrayUniformNode` | `array_append_tail` |
| `V10ArrayOmitTailNode` | `Indices []*V10IntegerNode` | `array_omit_tail` |
| `V10ArrayFinalNode` | `TypeRef *V10TypeOfRefNode` · `List V10Node` · `Tails []V10Node` | `array_final` |
| `V10LookupIdxExprNode` | `Value V10Node` | `lookup_idx_expr` |
| `V10ArrayLookupNode` | `Array *V10TypeOfRefNode` · `Index *V10LookupIdxExprNode` | `array_lookup` |

`V10ArrayFinalNode`: `TypeRef` is non-nil only for `TYPE_OF array_list<…>` form;
`Tails` items are `*V10ArrayAppendTailNode` or `*V10ArrayOmitTailNode`.

### Objects

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10ObjectEntryNode` | `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `Value *V10ArrayValueNode` | key-value pair inside `object_init` |
| `V10ObjectInitNode` | `Entries []V10ObjectEntryNode` | `object_init` |
| `V10ObjectMergeTailNode` | `Items []V10Node` ← `*V10TypeOfRefNode \| *V10ObjectInitNode` | `object_merge_tail` |
| `V10ObjectOmitTailNode` | `Items []V10Node` ← `*V10IdentDottedNode \| *V10LookupIdxExprNode` | `object_omit_tail` |
| `V10ObjectMergeOrOmitNode` | `Base V10Node` · `Modifier V10Node` | `object_merge_or_omit` |
| `V10ObjectFinalNode` | `Value V10Node` ← `*V10ObjectInitNode \| *V10ObjectMergeOrOmitNode \| *V10EmptyDeclNode` | `object_final` |
| `V10ObjectLookupNode` | `Object *V10TypeOfRefNode` · `Index *V10LookupIdxExprNode` | `object_lookup` |

### Tables

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10TableHeaderNode` | `Value V10Node` ← `*V10ArrayUniformNode \| *V10TypeOfRefNode` | `table_header` |
| `V10TableObjectsNode` | `Rows []V10Node` ← `*V10ArrayFinalNode \| *V10ObjectFinalNode` | `table_objects` |
| `V10TableInitNode` | `Header *V10TableHeaderNode` · `Objects *V10TableObjectsNode` · `AltObj *V10ObjectFinalNode` | `table_init` |
| `V10TableInsTailNode` | `Items []V10Node` ← `*V10TypeOfRefNode \| *V10ArrayFinalNode \| *V10ObjectFinalNode` | `table_ins_tail` |
| `V10TableFinalNode` | `Base V10Node` · `Tails []*V10TableInsTailNode` | `table_final` |

`V10TableInitNode`: either `Header`+`Objects` are both non-nil (header+rows
form), or `AltObj` is non-nil (object-only form).

---

## 6. §05 JSON Path

*Source: `parser_v10_json_path.go`, spec `05_json_path.sqg`*

### Leaf selectors

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10JPNameNode` | `Value string` | `jp_name` |
| `V10JPWildcardNode` | *(no fields)* | `jp_wildcard` (`*`) |
| `V10JPIndexNode` | `Value string` (signed integer string) | `jp_index` |
| `V10JPSliceNode` | `Start *string` · `End *string` · `Step *string` | `jp_slice` (`nil` = omitted) |

### Filter expressions

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10JPCurrentPathNode` | `Segments []V10JPSegmentNode` | `jp_current_path` (`@{…}`) |
| `V10JPFilterValueNode` | `Value V10Node` ← `*V10ConstantNode \| *V10JPCurrentPathNode \| *V10IdentRefNode` | `jp_filter_value` |
| `V10JPFilterCmpNode` | `Left *V10JPFilterValueNode` · `Oper V10JPFilterOperKind` · `Right *V10JPFilterValueNode` | `jp_filter_cmp` |
| `V10JPFilterAtomNode` | `Value V10Node` ← `*V10JPFilterCmpNode \| *V10JPCurrentPathNode \| *V10JPFilterExprNode` | `jp_filter_atom` |
| `V10JPFilterUnaryNode` | `Not bool` · `Atom *V10JPFilterAtomNode` | `jp_filter_unary` |
| `V10JPFilterExprNode` | `Head *V10JPFilterUnaryNode` · `Pairs []V10JPFilterExprPair` | `jp_filter_expr` |
| `V10JPFilterNode` | `Expr *V10JPFilterExprNode` | `jp_filter` (`?(…)`) |

### Selectors & segments

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10JPSelectorNode` | `Filter *V10JPFilterNode` · `Slice *V10JPSliceNode` · `Index *V10JPIndexNode` · `Name *V10JPNameNode` · `Wildcard *V10JPWildcardNode` | `jp_selector` (exactly one non-nil) |
| `V10JPSelectorListNode` | `Items []*V10JPSelectorNode` | `jp_selector_list` |
| `V10JPBracketSegNode` | `Selectors *V10JPSelectorListNode` | `jp_bracket_seg` (`[…]`) |
| `V10JPDotSegNode` | `Name *V10JPNameNode` · `Wildcard *V10JPWildcardNode` | `jp_dot_seg` (`.name` / `.*`) |
| `V10JPDescSegNode` | `Name *V10JPNameNode` · `Wildcard *V10JPWildcardNode` · `Bracket *V10JPBracketSegNode` | `jp_desc_seg` (`..`) |
| `V10JPSegmentNode` | `Desc *V10JPDescSegNode` · `Dot *V10JPDotSegNode` · `Bracket *V10JPBracketSegNode` | `jp_segment` (exactly one non-nil) |

### Top-level path nodes

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10JSONPathNode` | `Segments []V10JPSegmentNode` | `json_path` (`.$…`) |
| `V10IdentRefWithPathNode` | `Base *V10IdentRefNode` · `Path *V10JSONPathNode` | `EXTEND<ident_ref>` with path suffix |

`V10IdentRefWithPathNode.Path` is `nil` when no JSON Path suffix is present.

---

## 7. §06 Functions

*Source: `parser_v10_functions.go`, spec `06_functions.sqg`*

### Inspect access

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10InspectAccessNode` | `Ref *V10IdentRefNode` · `PropName string` | `inspect_*` |

`PropName` is one of: `"@storeName"`, `"@typeName"`, `"@data"`, `"@type"`.

### Built-in assignments

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10DataAssignNode` | `Target *V10InspectAccessNode` · `Value V10Node` | `data_assign` |
| `V10StoreNameAssignNode` | `Target *V10InspectAccessNode` · `Value *V10StringNode` | `store_name_assign` |
| `V10FuncHeaderBuildinNode` | `Value V10Node` ← `*V10StoreNameAssignNode \| *V10DataAssignNode` | `func_header_buildin_params` |

`V10DataAssignNode.Value` is one of `*V10ObjectFinalNode`, `*V10ArrayFinalNode`, `*V10ConstantNode`.

### Function header

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10AssignFuncRHSNode` | `Value V10Node` | `assign_func_rhs` |
| `V10FuncHeaderAssignNode` | `Buildin *V10FuncHeaderBuildinNode` · `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `RHS *V10AssignFuncRHSNode` | `func_header_assign` |
| `V10FuncEnclosureParamsNode` | `Items []V10Node` ← `*V10FuncHeaderBuildinNode \| *V10FuncHeaderAssignNode` | `func_enclosure_params` |
| `V10FuncHeaderUserParamsNode` | `Items []*V10FuncHeaderAssignNode` | `func_header_user_params` |

`V10FuncHeaderAssignNode`: `Buildin` is non-nil for the buildin form; `LHS`/`Oper`/`RHS` are set for the normal assign form.

### Static fn reference

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10IdentStaticFnNode` | `Ref *V10IdentRefNode` · `Static string` | `ident_static_fn` |

`Static` is one of: `"@name"`, `"@type"`, `"@storeName"`, `"ok"`, `"error"`, `"deps"`, `"next"`, `"value"`.

### Argument declarations (V11)

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10ArgsDeclNode` | `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `Type *V10InspectAccessNode` | `args_decl` |
| `V10FuncArgsNode` | `Entries []*V10ArgsDeclNode` | `func_args` (`->`) |
| `V10FuncStreamArgsNode` | `Entries []*V10ArgsDeclNode` | `func_stream_args` (`>>`) |
| `V10FuncDepsNode` | `StoreNames []string` | `func_deps` (`=>`) |
| `V10FuncArgsDeclNode` | `Args *V10FuncArgsNode` · `StreamArgs *V10FuncStreamArgsNode` · `Deps *V10FuncDepsNode` | `func_args_decl` |

`V10ArgsDeclNode.Oper` is `nil` when the operator is absent (positional arg).
All three fields in `V10FuncArgsDeclNode` are individually optional (may be nil).

> **Legacy (V10 only):** `V10FuncArgEntryNode` — `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `Value V10Node` — superseded by `V10ArgsDeclNode` in V11.

### Range arguments

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10FuncFixedNumRangeNode` | `Lo V10Node` · `Hi V10Node` | numeric range (`..`) |
| `V10FuncRangeArgsNode` | `Value V10Node` | `func_range_args` |

`V10FuncFixedNumRangeNode` Lo/Hi are `*V10NumericConstNode` or `*V10TypeOfRefNode`.
`V10FuncRangeArgsNode.Value` is one of `*V10FuncFixedNumRangeNode`, `*V10StringNode`, `*V10TypeOfRefNode`, `*V10ArrayFinalNode`, `*V10ObjectFinalNode`.

### Function call

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10FuncCallNode` | `Ref *V10TypeOfRefNode` · `Args []V10Node` | `func_call` |
| `V10FuncCallChainNode` | `Head *V10FuncCallNode` · `Steps []V10FuncCallChainStepNode` | `func_call_chain` |
| `V10FuncStreamLoopNode` | `Source V10Node` · `Body V10Node` | `func_stream_loop` |
| `V10FuncCallFinalNode` | `Value V10Node` ← `*V10FuncCallChainNode \| *V10FuncStreamLoopNode` | `func_call_final` |

`V10FuncCallNode.Args` items are `*V10AssignFuncRHSNode`.
`V10FuncStreamLoopNode.Source` is `*V10FuncRangeArgsNode`, `*V10IdentRefNode`, or `*V10BoolNode` (for `true`); `Body` is `*V10FuncUnitNode` or `*V10FuncCallNode`.

### Function injection (V11)

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10FuncInjectHeadInspectNode` | `LHS *V10AssignLHSNode` · `Inspect *V10InspectAccessNode` · `HasArray bool` | head with `inspect_type` |
| `V10FuncInjectBindNode` | `Bind V10FuncInjectBind` | head bind wrapper |
| `V10FuncInjectNode` | `Head V10Node` · `Binds []V10FuncInjectBind` | `func_inject` (`(…)`) |

`V10FuncInjectNode.Head` is nil (empty parens), `*V10FuncInjectHeadInspectNode`, or `*V10FuncInjectBindNode`.

### Function body statements

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10SelfRefNode` | *(no fields)* | `self_ref` (`$`) |
| `V10FuncStmtNode` | `Value V10Node` | `func_stmt` |
| `V10FuncAssignNode` | `Inject *V10FuncInjectNode` · `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `Stmt *V10FuncStmtNode` | `func_assign` |
| `V10FuncReturnStmtNode` | `Stmt *V10FuncStmtNode` | `func_return_stmt` (`<-`) |
| `V10FuncStoreStmtNode` | `Items []V10Node` | `func_store_stmt` (`=>`) |
| `V10FuncBodyStmtNode` | `Value V10Node` | `func_body_stmt` |

`V10FuncStmtNode.Value` is one of: `*V10RegexpNode`, `*V10IdentRefNode`, `*V10ObjectFinalNode`, `*V10ArrayFinalNode`, `*V10FuncCallChainNode`, `*V10FuncUnitNode`, `*V10CalcUnitNode`, `*V10SelfRefNode`, `*V10ReturnFuncUnitNode`.
`V10FuncBodyStmtNode.Value` is one of: `*V10FuncAssignNode`, `*V10FuncReturnStmtNode`, `*V10FuncStoreStmtNode`, `*V10FuncStreamLoopNode`, `*V10FuncCallFinalNode`.

### Function unit

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10FuncUnitHeaderNode` | `Enclosure *V10FuncEnclosureParamsNode` · `ArgsDecl *V10FuncArgsDeclNode` | `func_unit_header` |
| `V10FuncUnitNode` | `Header *V10FuncUnitHeaderNode` · `Body []*V10FuncBodyStmtNode` | `func_unit` (`{…}`) |
| `V10ReturnFuncUnitNode` | `Unit *V10FuncUnitNode` | `return_func_unit` (`<- {…}`) |
| `V10UpdateFuncUnitNode` | `Ref *V10TypeOfRefNode` · `Assign *V10AssignOperNode` · `NewUnit *V10ReturnFuncUnitNode` | `update_func_unit` |

### Update statements

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10ArrayIdxRecursiveNode` | `Indices []V10Node` | `array_idx_recursive` |
| `V10NumericStmtNode` | `Ref V10TypeOfRefNode` | `numeric_stmt` |
| `V10UpdateNumberNode` | `Target *V10TypeOfRefNode` · `Oper *V10AssignOperNode` · `RHS V10Node` | `update_number` |
| `V10StringStmtNode` | `Ref V10TypeOfRefNode` | `string_stmt` |
| `V10StringUpdateOperNode` | `Kind V10StringUpdateOperKind` · `Token string` | `string_update_oper` |
| `V10UpdateStringNode` | `Target *V10TypeOfRefNode` · `Oper *V10StringUpdateOperNode` · `RHS V10Node` | `update_string` |
| `V10IdentRefUpdateNode` | `Ref *V10IdentRefNode` · `Oper *V10AssignOperNode` · `RHS *V10AssignRHSNode` | `ident_ref_update` |

`V10UpdateNumberNode.RHS` is `*V10NumericConstNode` or `*V10NumericStmtNode`.
`V10UpdateStringNode.RHS` is `*V10StringNode` or `*V10StringStmtNode`.

---

## 8. §07 Scope & Root

*Source: `parser_v10_scope.go`, spec `07_scope.sqg`*

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V10ScopeInjectNode` | `Binds []V10ScopeInjectBind` | `scope_inject` (`(…)`) |
| `V10ImportAssignNode` | `LHS *V10AssignLHSNode` · `Target V10Node` | `import_assign` |
| `V10ScopeBodyItemNode` | `Value V10Node` | one item in a scope body |
| `V10ScopeAssignNode` | `Inject *V10ScopeInjectNode` · `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `Body []*V10ScopeBodyItemNode` | `scope_assign` |
| `V10ParserRootNode` | `Value V10Node` ← `*V10ImportAssignNode \| *V10ScopeAssignNode` | `parser_root` |

`V10ImportAssignNode.Target` is `*V10URLNode` or `*V10FilePathNode`.
`V10ScopeBodyItemNode.Value` is one of: `*V10ImportAssignNode`, `*V10AssignmentNode`, `*V10ScopeAssignNode`.

---

## 9. Helper structs

These plain structs are **not** `V10Node` implementors.  They appear as field
types in the node types above.

| Struct | Fields | Used by |
|--------|--------|---------|
| `V10TmplPart` | `IsExpr bool` · `Text string` | `V10StringNode.Parts` |
| `V10DurationSegment` | `Digits string` · `Unit string` | `V10DurationNode.Segments` |
| `V10AssignAnnotation` | `IdentName string` · `Cardin *V10CardinalityNode` · `Version *V10AssignVersionNode` | `V10AssignLHSNode.Annotations` |
| `V10NumExprTerm` | `Oper string` · `Expr *V10SingleNumExprNode` | `V10NumExprListNode.Terms` |
| `V10NumGroupTerm` | `Oper string` · `Expr V10Node` | `V10NumGroupingNode.Terms` |
| `V10StringExprTerm` | `Oper string` · `Str *V10StringNode` | `V10StringExprListNode.Terms` |
| `V10StringGroupTerm` | `Oper string` · `Expr V10Node` | `V10StringGroupingNode.Terms` |
| `V10LogicExprTerm` | `Oper string` · `Expr *V10SingleLogicExprNode` | `V10LogicExprListNode.Terms` |
| `V10LogicGroupTerm` | `Oper string` · `Expr V10Node` | `V10LogicGroupingNode.Terms` |
| `V10FuncCallChainStepNode` | `Op string` · `Ref *V10TypeOfRefNode` | `V10FuncCallChainNode.Steps` |
| `V10JPFilterExprPair` | `IsAnd bool` · `RHS *V10JPFilterUnaryNode` | `V10JPFilterExprNode.Pairs` |
| `V10FuncInjectBind` | `LHS *V10AssignLHSNode` · `Oper *V10AssignOperNode` · `Ref *V10IdentRefNode` | `V10FuncInjectNode.Binds` · `V10FuncInjectBindNode.Bind` |
| `V10ScopeInjectBind` | `LHS *V10AssignLHSNode` · `Ref *V10IdentRefNode` | `V10ScopeInjectNode.Binds` |

---

## 10. Enum types

### `V10AssignOperKind`

| Constant | Operator |
|----------|----------|
| `AssignMutable` | `=` |
| `AssignImmutable` | `:` |
| `AssignReadOnlyRef` | `:~` |
| `IncrAddImmutable` | `+:` |
| `IncrSubImmutable` | `-:` |
| `IncrMulImmutable` | `*:` |
| `IncrDivImmutable` | `/:` |
| `IncrAddMutable` | `+=` |
| `IncrSubMutable` | `-=` |
| `IncrMulMutable` | `*=` |
| `IncrDivMutable` | `/=` |

### `V10EmptyDeclKind`

| Constant | Token |
|----------|-------|
| `EmptyArray` | `[]` |
| `EmptyStream` | `>>` |
| `EmptyRegexp` | `//` |
| `EmptyScope` | `{}` |
| `EmptyStringD` | `""` |
| `EmptyStringS` | `''` |
| `EmptyStringT` | ` `` ` |

### `V10JPFilterOperKind`

| Constant | Operator |
|----------|----------|
| `JPFilterEqEq` | `==` |
| `JPFilterNeq` | `!=` |
| `JPFilterGEq` | `>=` |
| `JPFilterLEq` | `<=` |
| `JPFilterGt` | `>` |
| `JPFilterLt` | `<` |
| `JPFilterMatch` | `=~` |

### `V10StringUpdateOperKind`

| Constant | Operator(s) |
|----------|-------------|
| `StringAppendImmutable` | `+:` |
| `StringAppendMutable` | `+~` |
| `StringEqualAssign` | `=` / `:` / `:~` |

---

## 11. V10Node variant reference

This table summarises which concrete types can appear in polymorphic `V10Node`
fields, grouped by the discriminating field name pattern.

| Field / context | Possible concrete types |
|-----------------|-------------------------|
| `V10NumericConstNode.Value` | `*V10IntegerNode`, `*V10DecimalNode` |
| `V10ConstantNode.Value` | any leaf in §01 except helper structs |
| `V10CalcUnitNode.Value` | `*V10NumericExprNode`, `*V10StringExprNode`, `*V10LogicExprNode` |
| `V10NumericExprNode.Value` | `*V10NumExprListNode`, `*V10NumGroupingNode` |
| `V10StringExprNode.Value` | `*V10StringExprListNode`, `*V10StringGroupingNode` |
| `V10LogicExprNode.Value` | `*V10LogicExprListNode`, `*V10LogicGroupingNode` |
| `V10CompareExprNode.Value` | `*V10NumCompareNode`, `*V10StringCompareNode`, `*V10StringRegexpCompNode` |
| `V10AssignRHSNode.Value` | `*V10ConstantNode`, `*V10CalcUnitNode` |
| `V10AssignFuncRHSNode.Value` | `*V10ConstantNode`, `*V10EmptyDeclNode`, `*V10RegexpNode`, `*V10IdentRefNode`, `*V10CalcUnitNode`, `*V10ObjectFinalNode`, `*V10ArrayFinalNode` |
| `V10ArrayValueNode.Value` | `*V10ConstantNode`, `*V10RangeNode`, `*V10IdentRefNode`, `*V10CalcUnitNode`, `*V10ArrayFinalNode`, `*V10ObjectFinalNode` |
| `V10ArrayFinalNode.Tails[i]` | `*V10ArrayAppendTailNode`, `*V10ArrayOmitTailNode` |
| `V10ObjectFinalNode.Value` | `*V10ObjectInitNode`, `*V10ObjectMergeOrOmitNode`, `*V10EmptyDeclNode` |
| `V10ObjectMergeOrOmitNode.Base` | `*V10TypeOfRefNode`, `*V10ObjectInitNode` |
| `V10ObjectMergeOrOmitNode.Modifier` | `*V10ObjectMergeTailNode`, `*V10ObjectOmitTailNode` |
| `V10TableFinalNode.Base` | `*V10TypeOfRefNode`, `*V10TableInitNode` |
| `V10TableObjectsNode.Rows[i]` | `*V10ArrayFinalNode`, `*V10ObjectFinalNode` |
| `V10JPFilterValueNode.Value` | `*V10ConstantNode`, `*V10JPCurrentPathNode`, `*V10IdentRefNode` |
| `V10JPFilterAtomNode.Value` | `*V10JPFilterCmpNode`, `*V10JPCurrentPathNode`, `*V10JPFilterExprNode` |
| `V10FuncInjectNode.Head` | `nil`, `*V10FuncInjectHeadInspectNode`, `*V10FuncInjectBindNode` |
| `V10FuncStmtNode.Value` | `*V10RegexpNode`, `*V10IdentRefNode`, `*V10ObjectFinalNode`, `*V10ArrayFinalNode`, `*V10FuncCallChainNode`, `*V10FuncUnitNode`, `*V10CalcUnitNode`, `*V10SelfRefNode`, `*V10ReturnFuncUnitNode` |
| `V10FuncBodyStmtNode.Value` | `*V10FuncAssignNode`, `*V10FuncReturnStmtNode`, `*V10FuncStoreStmtNode`, `*V10FuncStreamLoopNode`, `*V10FuncCallFinalNode` |
| `V10FuncCallFinalNode.Value` | `*V10FuncCallChainNode`, `*V10FuncStreamLoopNode` |
| `V10FuncStreamLoopNode.Source` | `*V10FuncRangeArgsNode`, `*V10IdentRefNode`, `*V10BoolNode` |
| `V10FuncStreamLoopNode.Body` | `*V10FuncUnitNode`, `*V10FuncCallNode` |
| `V10FuncRangeArgsNode.Value` | `*V10FuncFixedNumRangeNode`, `*V10StringNode`, `*V10TypeOfRefNode`, `*V10ArrayFinalNode`, `*V10ObjectFinalNode` |
| `V10UpdateNumberNode.RHS` | `*V10NumericConstNode`, `*V10NumericStmtNode` |
| `V10UpdateStringNode.RHS` | `*V10StringNode`, `*V10StringStmtNode` |
| `V10DataAssignNode.Value` | `*V10ObjectFinalNode`, `*V10ArrayFinalNode`, `*V10ConstantNode` |
| `V10ScopeBodyItemNode.Value` | `*V10ImportAssignNode`, `*V10AssignmentNode`, `*V10ScopeAssignNode` |
| `V10ImportAssignNode.Target` | `*V10URLNode`, `*V10FilePathNode` |
| `V10ParserRootNode.Value` | `*V10ImportAssignNode`, `*V10ScopeAssignNode` |
