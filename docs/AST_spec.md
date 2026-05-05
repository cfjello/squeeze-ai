# Squeeze V13 Parser — AST Node Specification

*Source: `pkg/parser/parser_v13*.go` · Spec: `spec/*.sqg`*  
*Updated to reflect all V13 additions (structures, ranges, key types, dependencies).*

---

## V13Node interface

Every AST node implements:

```go
type V13Node interface {
    V13NodePos() (line, col int)
}
```

`V13BaseNode` is embedded in every concrete node to carry source position:

```go
type V13BaseNode struct { Line, Col int }
func (n V13BaseNode) V13NodePos() (int, int) { return n.Line, n.Col }
```

---

## Table of contents

1. [§01 Literals & primitives](#1-01-literals--primitives)
2. [§02 Identifiers & operators](#2-02-identifiers--operators)
3. [§03 Assignment](#3-03-assignment)
4. [§04 Objects & arrays](#4-04-objects--arrays)
5. [§05 JSON Path](#5-05-json-path)
6. [§06 Functions](#6-06-functions)
7. [§07 Scope & root](#7-07-scope--root)
8. [§08 Ranges & validation](#8-08-ranges--validation)
9. [§09 Structures](#9-09-structures)
10. [§10 Helper structs](#10-10-helper-structs)
11. [§11 Enum types](#11-11-enum-types)
12. [§12 V13Node variant reference](#12-12-v13node-variant-reference)

---

## 1. §01 Literals & primitives

*Source: `parser_v13.go`, spec `01_definitions.sqg`*

### Numeric

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13SignPrefixNode` | `Sign string` (`""`, `"+"`, `"-"`) | `sign_prefix` |
| `V13IntegerNode` | `SignPrefix *V13SignPrefixNode` · `Digits string` | `integer` |
| `V13DecimalNode` | `SignPrefix *V13SignPrefixNode` · `Integral string` · `Frac string` | `decimal` |
| `V13NumericConstNode` | `Value V13Node` | `numeric_const` |
| `V13NanNode` | *(no extra fields)* | `nan` (`NaN`) |
| `V13InfinityNode` | `Sign string` | `infinity` (`+Infinity`, `-Infinity`, `Infinity`) |
| `V13FloatNode` | `Kind string` · `Value V13Node` | `float32` \| `float64` |
| `V13DecimalTypeNode` | `Kind string` · `Integral *V13IntegerNode` · `Frac string` | `decimalN` |
| `V13DecimalNumNode` | `Value *V13DecimalTypeNode` | `decimal_num` (`decimal8`…`decimal128`) |

`V13NumericConstNode.Value` is `*V13IntegerNode` or `*V13DecimalNode`.  
`V13FloatNode.Value` is `*V13DecimalNode`, `*V13NanNode`, or `*V13InfinityNode`.

### String

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13StringNode` | `Kind V13StringKind` · `Raw string` · `Parts []V13TmplPart` | `single_quoted` \| `double_quoted` \| `tmpl_quoted` |
| `V13StringQuotedNode` | `Value *V13StringNode` | `string_quoted` |
| `V13StringUnquotedNode` | `Inner string` | `string_unquoted` (UNQUOTE directive) |
| `V13TmplUnquotedNode` | `Inner string` · `Parts []V13TmplPart` | `tmpl_unquoted` (UNQUOTE directive) |
| `V13CharNode` | `Value rune` | `char` |

### Date / Time

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13DateNode` | `Year string` · `Month string` · `Day string` | `date` |
| `V13TimeNode` | `Hour string` · `Minute string` · `Second string` · `Millis string` | `time` |
| `V13DateTimeNode` | `Date *V13DateNode` · `Time *V13TimeNode` | `date_time` |
| `V13TimeStampNode` | `Date *V13DateNode` · `Time *V13TimeNode` | `time_stamp` |
| `V13DurationNode` | `Segments []V13DurationSegment` | `duration` |

### Regexp / Bool / Null / Any

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13RegexpNode` | `Pattern string` · `Flags string` | `regexp` |
| `V13BoolNode` | `Value bool` | `boolean` (`true` \| `false`) |
| `V13NullNode` | *(no extra fields)* | `null` |
| `V13AnyTypeNode` | *(no extra fields)* | `any_type` (`@?`) |

### Structural primitives

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13CardinalityNode` | `Lo string` · `Hi string` | `cardinality` |
| `V13RangeNode` | `Lo *V13IntegerNode` · `Hi *V13IntegerNode` | `range` |

### Key / identity types *(V13 new)*

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13UUIDNode` | `Value string` | `uuid` (8-4-4-4-12) |
| `V13UUIDV7Node` | `Value string` | `uuid_v7` (RFC 9562, time-sortable) |
| `V13ULIDNode` | `Value string` | `ulid` (26-char Crockford base32) |
| `V13NanoIDNode` | `Value string` | `nano_id` (21-char URL-safe random) |
| `V13SnowflakeIDNode` | `Value string` | `snowflake_id` (uint64 distributed sortable) |
| `V13SeqIDNode` | `Kind string` · `Value string` | `seq_id16` \| `seq_id32` \| `seq_id64` |
| `V13HashKeyNode` | `Kind string` · `Value string` | `hash_md5` \| `hash_sha1` \| `hash_sha256` \| `hash_sha512` |
| `V13CompositeKeyNode` | `Parts []V13Node` (≥2) | `composite_key` |
| `V13UniqueKeyNode` | `Value V13Node` | `unique_key` |

`V13SeqIDNode.Kind` is `"seq_id16"`, `"seq_id32"`, or `"seq_id64"`.  
`V13HashKeyNode.Kind` is `"hash_md5"`, `"hash_sha1"`, `"hash_sha256"`, or `"hash_sha512"`.

### URL / file path

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13URLNode` | `Value string` | `http_url` |
| `V13FileURLNode` | `Value string` | `file_url` |

### Top-level constant wrapper

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13ConstantNode` | `Value V13Node` | `constant` |

`V13ConstantNode.Value` is one of: `*V13NumericConstNode`, `*V13StringNode`, `*V13CharNode`, `*V13RegexpNode`, `*V13BoolNode`, `*V13NullNode`, `*V13DateNode`, `*V13TimeNode`, `*V13DateTimeNode`, `*V13TimeStampNode`, `*V13DurationNode`, `*V13UUIDNode`, `*V13UUIDV7Node`, `*V13ULIDNode`, `*V13NanoIDNode`, `*V13URLNode`, `*V13FileURLNode`.

---

## 2. §02 Identifiers & operators

*Source: `parser_v15_operators.go`, spec `02_operators.sqg`*

### Identifier nodes

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13IdentDottedNode` | `Parts []string` | `ident_name` or `AT_IDENT` with dot segments |
| `V13IdentPrefixNode` | `Prefix string` · `Name string` | `@`-prefixed identifier |
| `V13IdentRefNode` | `Name string` · `Dotted *V13IdentDottedNode` | `ident_ref` |
| `V13TypeRefNode` | `Ref *V13IdentRefNode` · `TypeName string` | `type_ref` (`ident_ref "." "@type"`) |

### Numeric expression

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13SingleNumExprNode` | `Value V13Node` | `single_num_expr` |
| `V13NumExprListNode` | `Head V13Node` · `Terms []V13NumExprTerm` | `num_expr_list` |
| `V13NumGroupingNode` | `Terms []V13NumGroupTerm` | `num_grouping` |
| `V13NumericExprNode` | `Value V13Node` | `numeric_expr` |

`V13NumericExprNode.Value` is `*V13NumExprListNode` or `*V13NumGroupingNode`.

### String expression

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13StringExprListNode` | `Head *V13StringNode` · `Terms []V13StringExprTerm` | `string_expr_list` |
| `V13StringGroupingNode` | `Terms []V13StringGroupTerm` | `string_grouping` |
| `V13StringExprNode` | `Value V13Node` | `string_expr` |

`V13StringExprNode.Value` is `*V13StringExprListNode` or `*V13StringGroupingNode`.

### Compare expression

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13NumCompareNode` | `LHS *V13NumericExprNode` · `Oper string` · `RHS *V13NumericExprNode` | `num_compare` |
| `V13StringCompareNode` | `LHS *V13StringExprNode` · `Oper string` · `RHS *V13StringExprNode` | `string_compare` |
| `V13StringRegexpCompNode` | `LHS *V13StringExprNode` · `Oper string` · `RHS *V13RegexpNode` | `string_regexp_comp` |
| `V13CompareExprNode` | `Value V13Node` | `compare_expr` |

`V13CompareExprNode.Value` is `*V13NumCompareNode`, `*V13StringCompareNode`, or `*V13StringRegexpCompNode`.

### Logic expression

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13SingleLogicExprNode` | `Negate bool` · `Value V13Node` | `single_logic_expr` |
| `V13LogicExprListNode` | `Head *V13SingleLogicExprNode` · `Terms []V13LogicExprTerm` | `logic_expr_list` |
| `V13LogicGroupingNode` | `Terms []V13LogicGroupTerm` | `logic_grouping` |
| `V13LogicExprNode` | `Value V13Node` | `logic_expr` |

`V13LogicExprNode.Value` is `*V13LogicExprListNode` or `*V13LogicGroupingNode`.

Logic operators supported: `&&` (logic_and), `||` (logic_or), `^` (logic_exclusive_or — *V13 formalised*).

### Calculation unit

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13SelfRefNode` | *(no extra fields)* | `self_ref` |
| `V13CalcUnitNode` | `Value V13Node` | `calc_unit` |

`V13CalcUnitNode.Value` is `*V13NumericExprNode`, `*V13StringExprNode`, or `*V13LogicExprNode`.

---

## 3. §03 Assignment

*Source: `parser_v15_assignment.go`, spec `03_assignment.sqg`*

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13AssignOperNode` | `Kind V13AssignOperKind` · `Value string` | `assign_oper` |
| `V13AssignVersionNode` | `Raw string` | `assign_version` (`"v"` prefix optional in V13) |
| `V13AssignLHSNode` | `Name string` · `Annotations []V13AssignAnnotation` | `assign_lhs` |
| `V13AssignRhsItemNode` | `Value V13Node` | `assign_rhs_item` |
| `V13AssignRhsChainNode` | `Terms []V13AssignRhsChainTerm` | `assign_rhs_chain` |
| `V13AssignRHSNode` | `Value V13Node` | `assign_rhs` |
| `V13AssignmentNode` | `LHS *V13AssignLHSNode` · `Oper *V13AssignOperNode` · `RHS *V13AssignRHSNode` | `assignment` |

`V13AssignRHSNode.Value` is `*V13AssignRhsItemNode` or `*V13AssignRhsChainNode`.
`V13AssignRhsItemNode.Value` is `*V13ConstantNode`, `*V13CalcUnitNode`, `*V13CardinalityNode`, or a structural form.
`V13AssignRhsChainTerm.Item` is `*V13AssignRhsItemNode`; `Oper` is `""` for the first term, `"&"`, `"|"`, or `"^"` for the rest.

---

## 4. §04 Objects & arrays

*Source: `parser_v15_objects.go`, spec `04_objects.sqg`*

### Empty declarations

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13EmptyDeclNode` | `Kind V13EmptyDeclKind` | `empty_decl` |

### Array nodes

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13TypeOfRefNode` | `TypeName string` · `Ref *V13IdentRefNode` | `TYPE_OF typeName<ident_ref>` |
| `V13ArrayValueNode` | `Value V13Node` | `array_value` |
| `V13ArrayUniformNode` | `Items []V13Node` | `array_uniform` |
| `V13ArrayAppendTailNode` | `Arrays []*V13ArrayUniformNode` | `array_append_tail` |
| `V13ArrayOmitTailNode` | `Indices []*V13IntegerNode` | `array_omit_tail` |
| `V13EmptyArrayTypedNode` | `Ref *V13IdentRefNode` | `empty_array_typed` |
| `V13PlainArrayNode` | `Items []*V13ArrayValueNode` | `plain_array` — bracket list with no UNIFORM INFER wrapper |
| `V13ArrayFinalNode` | `TypeRef *V13TypeOfRefNode` · `List V13Node` · `Tails []V13Node` | `array_final` |
| `V13LookupIdxExprNode` | `Value V13Node` | `lookup_idx_expr` |
| `V13ArrayLookupNode` | `Array *V13TypeOfRefNode` · `Index *V13LookupIdxExprNode` | `array_lookup` |
| `V13SplitArrayNode` | `Separator *V13ConstantNode` | `split_array` (`...` expr — *V13 new*) |

`V13ArrayFinalNode.Tails[i]` is `*V13ArrayAppendTailNode` or `*V13ArrayOmitTailNode`.
`V13ArrayFinalNode.List` is `*V13ArrayUniformNode`, `*V13EmptyArrayTypedNode`, or `*V13PlainArrayNode`.

### Object nodes

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13ObjectEntryNode` | `LHS *V13AssignLHSNode` · `Oper *V13AssignOperNode` · `Value *V13ArrayValueNode` | one key-value pair |
| `V13ObjectInitNode` | `Entries []V13ObjectEntryNode` | `object_init` |
| `V13ObjectMergeTailNode` | `Items []V13Node` | `object_merge_tail` |
| `V13ObjectOmitTailNode` | `Items []V13Node` | `object_omit_tail` |
| `V13ObjectMergeOrOmitNode` | `Base V13Node` · `Modifier V13Node` | `object_merge_or_omit` |
| `V13ObjectFinalNode` | `Value V13Node` | `object_final` |
| `V13ObjectLookupNode` | `Object *V13TypeOfRefNode` · `Index *V13LookupIdxExprNode` | `object_lookup` |

`V13ObjectMergeOrOmitNode.Base` is `*V13TypeOfRefNode` or `*V13ObjectInitNode`.  
`V13ObjectMergeOrOmitNode.Modifier` is `*V13ObjectMergeTailNode` or `*V13ObjectOmitTailNode`.  
`V13ObjectFinalNode.Value` is `*V13ObjectInitNode`, `*V13ObjectMergeOrOmitNode`, or `*V13EmptyDeclNode`.

### Simple table nodes (V12-style, still supported)

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13TableHeaderNode` | `Value V13Node` | `table_header` |
| `V13TableObjectsNode` | `Rows []V13Node` | `table_objects` |
| `V13TableInitSimpleNode` | `Header *V13TableHeaderNode` · `Objects *V13TableObjectsNode` · `AltObj *V13ObjectFinalNode` | `table_init` (V12-style) |
| `V13TableInsTailNode` | `Items []V13Node` | `table_ins_tail` |
| `V13TableFinalSimpleNode` | `Base V13Node` · `Tails []*V13TableInsTailNode` | `table_final` (V12-style) |

The structured table form (using `columns`/`rows`/`key_columns`) is defined in §09 Structures.

---

## 5. §05 JSON Path

*Source: `parser_v15_json_path.go`, spec `05_json_path.sqg`*

### Selector nodes

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13JPNameNode` | `Name string` | `jp_name` |
| `V13JPWildcardNode` | *(no extra fields)* | `jp_wildcard` (`*`) |
| `V13JPIndexNode` | `Index int` | `jp_index` |
| `V13JPSliceNode` | `Start *int` · `End *int` · `Step *int` | `jp_slice` |

### Filter nodes

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13JPCurrentPathNode` | `Segments []V13Node` | `jp_current_path` (`@.…`) |
| `V13JPFilterValueNode` | `Value V13Node` | `jp_filter_value` |
| `V13JPFilterCmpNode` | `LHS *V13JPFilterValueNode` · `Oper V13JPFilterOperKind` · `RHS *V13JPFilterValueNode` | `jp_filter_cmp` |
| `V13JPFilterAtomNode` | `Value V13Node` | `jp_filter_atom` |
| `V13JPFilterUnaryNode` | `Negate bool` · `Atom *V13JPFilterAtomNode` | `jp_filter_unary` |
| `V13JPFilterExprNode` | `Head *V13JPFilterUnaryNode` · `Pairs []V13JPFilterExprPair` | `jp_filter_expr` |
| `V13JPFilterNode` | `Expr *V13JPFilterExprNode` | `jp_filter` |

### Segment / path nodes

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13JPSelectorNode` | `Value V13Node` | `jp_selector` |
| `V13JPSelectorListNode` | `Items []V13Node` | `jp_selector_list` |
| `V13JPBracketSegNode` | `List *V13JPSelectorListNode` | `jp_bracket_seg` (`[…]`) |
| `V13JPDotSegNode` | `Value V13Node` | `jp_dot_seg` (`.name` or `.*`) |
| `V13JPDescSegNode` | `Value V13Node` | `jp_desc_seg` (`..name`) |
| `V13JPSegmentNode` | `Value V13Node` | `jp_segment` |
| `V13JSONPathNode` | `Segments []V13Node` | `json_path` |
| `V13IdentRefWithPathNode` | `Ref *V13IdentRefNode` · `Path *V13JSONPathNode` | `ident_ref_with_path` |

---

## 6. §06 Functions

*Source: `parser_v15_functions.go`, spec `06_functions.sqg`*

### Inspect / type declaration

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13InspectTypeNode` | `Name string` | `inspect_type` (`@MyType` or `@?`) |
| `V13InspectTypeNameNode` | `Ref *V13IdentRefNode` · `Prop string` | `inspect_type_name` (`@ref.name`) |
| `V13TypeDeclareNode` | `Name string` · `Inspect *V13InspectTypeNode` | `type_declare` |

### Function header

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13AssignFuncRHSNode` | `Value V13Node` | `assign_func_rhs` |
| `V13FuncHeaderAssignNode` | `LHS *V13AssignLHSNode` · `Oper *V13AssignOperNode` · `RHS *V13AssignFuncRHSNode` | `func_header_assign` |
| `V13FuncHeaderUserParamsNode` | `Items []*V13FuncHeaderAssignNode` | `func_header_user_params` |

`V13AssignFuncRHSNode.Value` is one of: `*V13ConstantNode`, `*V13EmptyDeclNode`, `*V13RegexpNode`, `*V13IdentRefNode`, `*V13CalcUnitNode`, `*V13ObjectFinalNode`, `*V13ArrayFinalNode`.

### Arguments declaration

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13ArgsDeclNode` | `LHS *V13AssignLHSNode` · `Oper *V13AssignOperNode` · `Type *V13InspectTypeNode` | `args_decl` |
| `V13FuncArgsNode` | `Entries []*V13ArgsDeclNode` | `func_args` (`->`) |
| `V13FuncStreamArgsNode` | `Entries []*V13ArgsDeclNode` | `func_stream_args` (`iterator_oper args_decl`; `iterator_oper` = `>>`) |
| `V13FuncDepsNode` | `StoreNames []string` | `func_deps` (`=>` dependency_oper — *V13 new*) |
| `V13FuncArgsDeclNode` | `Args *V13FuncArgsNode` · `StreamArgs *V13FuncStreamArgsNode` · `Deps *V13FuncDepsNode` | `func_args_decl` |

### Range arguments

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13FuncFixedNumRangeNode` | `Lo V13Node` · `Hi V13Node` | `func_fixed_num_range` |
| `V13FuncRangeArgsNode` | `Value V13Node` | `func_range_args` |

`V13FuncRangeArgsNode.Value` is `*V13FuncFixedNumRangeNode`, `*V13StringNode`, `*V13TypeOfRefNode`, `*V13ArrayFinalNode`, or `*V13ObjectFinalNode`.

### Function call

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13FuncCallNode` | `Ref *V13TypeOfRefNode` · `Args []V13Node` | `func_call` |
| `V13FuncCallChainNode` | `Head *V13FuncCallNode` · `Steps []V13FuncCallChainStepNode` | `func_call_chain` (step ops: `->`, `iterator_oper` (`>>`), `logic_oper`) |
| `V13IteratorSourceNode` | `Value V13Node` | `iterator_source` |
| `V13FuncStreamLoopNode` | `Source V13Node` · `Body V13Node` | `func_stream_loop` (`iterator_source iterator_oper body`; `iterator_oper` = `>>`) |
| `V13FuncCallFinalNode` | `Value V13Node` | `func_call_final` |

`V13FuncCallFinalNode.Value` is `*V13FuncCallChainNode` or `*V13FuncStreamLoopNode`.  
`V13IteratorSourceNode.Value` is `*V13ArrayFinalNode`, `*V13FuncRangeArgsNode`, `*V13BoolNode`, `*V13IdentRefNode`, or `*V13FuncCallFinalNode`.  
`V13FuncStreamLoopNode.Source` is `*V13IteratorSourceNode`.  
`V13FuncStreamLoopNode.Body` is `*V13FuncUnitNode` or `*V13FuncCallNode`.

### Function injection

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13FuncInjectBindNode` | `Bind V13FuncInjectBind` | `func_inject_bind` |
| `V13FuncInjectHeadInspectNode` | `LHS *V13AssignLHSNode` · `Inspect *V13InspectTypeNode` · `HasArray bool` | `func_inject_head_inspect` |
| `V13FuncInjectNode` | `Head V13Node` · `Binds []V13FuncInjectBind` | `func_inject` |

`V13FuncInjectNode.Head` is `nil`, `*V13FuncInjectHeadInspectNode`, or `*V13FuncInjectBindNode`.

### Function body statements

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13FuncStmtNode` | `Value V13Node` | `func_stmt` |
| `V13FuncAssignNode` | `Inject *V13FuncInjectNode` · `LHS *V13AssignLHSNode` · `Oper *V13AssignOperNode` · `Stmt *V13FuncStmtNode` | `func_assign` |
| `V13FuncReturnStmtNode` | `Stmt *V13FuncStmtNode` | `func_return_stmt` (`<-`) |
| `V13CondReturnStmtNode` | `Cond V13Node` · `Oper string` · `Return *V13FuncReturnStmtNode` | `cond_return_stmt` (*V13 new*) |
| `V13FuncStoreStmtNode` | `Items []V13Node` | `func_store_stmt` (`=>` — publishes UUIDv7-stamped data version) |
| `V13IteratorYieldStmtNode` | `Stmt *V13FuncStmtNode` | `iterator_yield_stmt` (`result >>` — postfix yield from iterator body) |
| `V13AssignIteratorNode` | `Source *V13IteratorSourceNode` | `assign_iterator` (lazy binding alias; `iterator_source >>` EOL) |
| `V13PushSourceNode` | `Value V13Node` | `push_source` (source value for a push pipeline) |
| `V13PushRecvDeclNode` | `Name string` · `Type *V13InspectTypeNode` | `push_recv_decl` (`~> item: @T` — Role A; header receive declaration) |
| `V13PushForwardStmtNode` | `Stmt *V13FuncStmtNode` | `push_forward_stmt` (`result ~>` — Role B; postfix emit in push body) |
| `V13PushStreamBindNode` | `Source *V13PushSourceNode` · `Stages []V13Node` | `push_stream_bind` (`source ~> handler { ~> handler }` — Role C; pipeline) |
| `V13AssignPushNode` | `Source *V13PushSourceNode` | `assign_push` (cold push binding; `push_source ~>` EOL) |
| `V13FuncBodyStmtNode` | `Value V13Node` | `func_body_stmt` |

`V13FuncStmtNode.Value` is one of: `*V13RegexpNode`, `*V13IdentRefNode`, `*V13ObjectFinalNode`, `*V13ArrayFinalNode`, `*V13FuncCallChainNode`, `*V13FuncUnitNode`, `*V13CalcUnitNode`, `*V13SelfRefNode`, `*V13ReturnFuncUnitNode`.  
`V13FuncBodyStmtNode.Value` is one of: `*V13FuncAssignNode`, `*V13FuncReturnStmtNode`, `*V13CondReturnStmtNode`, `*V13FuncStoreStmtNode`, `*V13IteratorYieldStmtNode`, `*V13PushForwardStmtNode`, `*V13PushStreamBindNode`, `*V13FuncStreamLoopNode`, `*V13FuncCallFinalNode`.

### Function unit

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13FuncUnitHeaderNode` | `UserParams *V13FuncHeaderUserParamsNode` · `ArgsDecl *V13FuncArgsDeclNode` | `func_unit_header` |
| `V13FuncUnitNode` | `Header *V13FuncUnitHeaderNode` · `Body []*V13FuncBodyStmtNode` · `UseGroupDelim bool` | `func_unit` (`{…}` or `(…)`) |
| `V13ReturnFuncUnitNode` | `Unit *V13FuncUnitNode` | `return_func_unit` (`<- {…}`) |
| `V13UpdateFuncUnitNode` | `Ref *V13TypeOfRefNode` · `Assign *V13AssignOperNode` · `NewUnit *V13ReturnFuncUnitNode` | `update_func_unit` |

### Update statements

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13ArrayIdxRecursiveNode` | `Indices []V13Node` | `array_idx_recursive` |
| `V13NumericStmtNode` | `Ref V13TypeOfRefNode` | `numeric_stmt` |
| `V13UpdateNumberNode` | `Target *V13TypeOfRefNode` · `Oper *V13AssignOperNode` · `RHS V13Node` | `update_number` |
| `V13StringStmtNode` | `Ref V13TypeOfRefNode` | `string_stmt` |
| `V13StringUpdateOperNode` | `Kind V13StringUpdateOperKind` · `Token string` | `string_update_oper` |
| `V13UpdateStringNode` | `Target *V13TypeOfRefNode` · `Oper *V13StringUpdateOperNode` · `RHS V13Node` | `update_string` |
| `V13IdentRefUpdateNode` | `Ref *V13IdentRefNode` · `Oper *V13AssignOperNode` · `RHS *V13AssignRHSNode` | `ident_ref_update` |

`V13UpdateNumberNode.RHS` is `*V13NumericConstNode` or `*V13NumericStmtNode`.  
`V13UpdateStringNode.RHS` is `*V13StringNode` or `*V13StringStmtNode`.

---

## 7. §07 Scope & root

*Source: `parser_v17_scope.go`, spec `07_types_scope.sqg`*

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V17AssignOperNode` | `Op interface{}` | `assign_oper` (`:` / `:~` / `=`) |
| `V17ExtendScopeOperNode` | *(no fields)* | `extend_scope_oper` (`+=`) |
| `V17ScopeAssignInlineNode` | *(no fields)* | `scope_assign_inline` (`_`) |
| `V17ScopeInjectItem` | `Lhs *V17AssignLhsNode` · `TypeExpr interface{}` | one binding in `scope_inject` |
| `V17ScopeInjectNode` | `Items []V17ScopeInjectItem` | `scope_inject` (`(…)`) |
| `V17ImportAssignNode` | `Modifier *V17PrivateModifierNode` · `LHS interface{}` · `URLs []*V17ConstantNode` | `import_assign` |
| `V17OtherInlineAssignNode` | `Modifier *V17PrivateModifierNode` · `LHS *V17ScopeAssignInlineNode` · `Oper *V17AssignOperNode` · `Rhs *V17AssignRhsNode` | `other_inline_assign` |
| `V17ScopeBodyItemNode` | `Value interface{}` | one item in a scope body (EXTEND combined) |
| `V17PrivateBlockNode` | `Modifier *V17PrivateModifierNode` · `Items []*V17ScopeBodyItemNode` · `UseParens bool` | `private_block` (`-( )` or `-{ }`) |
| `V17ScopeBodyNode` | `Items []interface{}` | `scope_body` (`{ … }`) |
| `V17EqualAssignNode` | *(no fields)* | `equal_assign` (`=` in scope context) |
| `V17ScopeAssignNode` | `Inject *V17ScopeInjectNode` · `LHS *V17AssignLhsNode` · `Oper interface{}` · `Body interface{}` | `scope_assign` |
| `V17ScopeMergeTailSegment` | `Body interface{}` | one `+` segment in `scope_merge_tail` |
| `V17ScopeMergeTailNode` | `Base interface{}` · `Segments []V17ScopeMergeTailSegment` | `scope_merge_tail` |
| `V17ScopeFinalNode` | `Value interface{}` | `scope_final` |
| `V17ParserRootNode` | `Value interface{}` | `parser_root` (grammar entry point) |
| `V17FuncInjectItem` | `LHS *V17AssignLhsNode` · `Value interface{}` · `EmptyArrayDecl *V17EmptyArrayDeclNode` | one binding in `func_inject` |
| `V17FuncInjectNode` | `Items []V17FuncInjectItem` | `func_inject` (`(…)`) |
| `V17FuncScopeAssignNode` | `Inject *V17FuncInjectNode` · `LHS *V17AssignLhsNode` · `FUnit *V17FuncUnitNode` | `func_scope_assign` |

`V17AssignOperNode.Op` is `*V17AssignImmutableNode`, `*V17AssignReadOnlyRefNode`, or `*V17AssignMutableNode`.  
`V17ScopeInjectItem.TypeExpr` is `*V17IdentRefNode` (bare or `@`-prefixed) or `*V17AnyTypeNode` (`@?`).  
`V17ImportAssignNode.LHS` is `*V17AssignLhsNode` (named import) or `*V17ScopeAssignInlineNode` (wildcard `_` import).  
`V17ImportAssignNode.URLs` is a slice of `*V17ConstantNode` (http_url or file_url string literals).  
`V17ScopeBodyItemNode.Value` is one of: `*V17ImportAssignNode`, `*V17OtherInlineAssignNode`, `*V17ScopeFinalNode`, `*V17FuncScopeAssignNode`, or `*V17AssignmentNode`.  
`V17PrivateBlockNode.Items[i]` is a `*V17ScopeBodyItemNode`; no nested private blocks.  
`V17ScopeBodyNode.Items[i]` is `*V17ScopeBodyItemNode` or `*V17PrivateBlockNode`.  
`V17ScopeAssignNode.Oper` is `*V17EqualAssignNode` (`=`) or `*V17ExtendScopeOperNode` (`+=`).  
`V17ScopeAssignNode.Body` is `*V17ScopeBodyNode` or `*V17EmptyScopeDeclNode`.  
`V17ScopeMergeTailNode.Base` is `*V17ScopeAssignNode` or `*V17IdentRefNode` (TYPE_OF scope_assign\<ident_ref\>).  
`V17ScopeMergeTailSegment.Body` is `*V17ScopeBodyNode` or `*V17IdentRefNode` (TYPE_OF scope_body\<ident_ref\>).  
`V17ScopeFinalNode.Value` is `*V17ScopeAssignNode` or `*V17ScopeMergeTailNode`.  
`V17ParserRootNode.Value` is `*V17ImportAssignNode` or `*V17ScopeFinalNode`.  
`V17FuncInjectItem.Value` is `*V17InspectTypeNode` (when `EmptyArrayDecl` may be non-nil) or `*V17IdentRefNode`.  
`V17FuncScopeAssignNode.Inject` is nil when no `func_inject` clause is present.

---

## 8. §08 Ranges & validation

*Source: `parser_v17_range.go`, spec `08_range.sqg`*

All nodes in this section are **V17** — implemented in `parser_v17_range.go`.

### Terminals

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V17RangeOperNode` | *(none)* | `range_oper` (`".."`) |
| `V17ValidateOperNode` | *(none)* | `validate_oper` (`"><"` — two tokens `V17_GT` + `V17_LT`) |

### Numeric range validation

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V17NumRangeValidNode` | `Expr *V17NumericCalcNode` · `RangeRef *V17IdentRefNode` | `num_range_valid` (`TYPE_OF boolean< numeric_calc >< func_fixed_num_range >`) |

`func_fixed_num_range` is not defined as a grammar rule; it is parsed as `ident_ref`.

In source text: `TYPE_OF boolean<numeric_expr >< rangeRef>`.  
`TYPE_OF` is not a keyword token in V17; it is a `V17_IDENT` with value `"TYPE_OF"`.

### Date range

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V17DateRangeNode` | `Lo V17DateRangeSide` · `Hi V17DateRangeSide` | `date_range` |
| `V17DateRangeValidNode` | `Date *V17DateNode` · `Range *V17DateRangeNode` | `date_range_valid` (`TYPE_OF boolean< date >< date_range >`) |

`V17DateRangeSide` is a plain (non-pointer) struct with two mutually exclusive fields:

| Field | Type | When non-nil |
|-------|------|-------------|
| `Date` | `*V17DateNode` | literal date used |
| `TypedOf` | `*V17IdentRefNode` | `TYPE_OF date<ident_ref>` form |

### Time range

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V17TimeRangeNode` | `Lo V17TimeRangeSide` · `Hi V17TimeRangeSide` | `time_range` |
| `V17TimeRangeValidNode` | `Time *V17TimeNode` · `Range *V17TimeRangeNode` | `time_range_valid` (`TYPE_OF boolean< time >< time_range >`) |

`V17TimeRangeSide` is a plain (non-pointer) struct with two mutually exclusive fields:

| Field | Type | When non-nil |
|-------|------|-------------|
| `Time` | `*V17TimeNode` | literal time used |
| `TypedOf` | `*V17IdentRefNode` | `TYPE_OF time<ident_ref>` form |

### EXTEND rules

| Target | Added alternatives |
|--------|--------------------|
| `collection` (`parser_v17_objects.go`) | `\| date_range \| time_range` |
| `condition` (`parser_v17_operators.go`) | `\| num_range_valid \| date_range_valid \| time_range_valid` |
| `statement` (`parser_v17_operators.go`) | `\| date_range \| time_range` |

`date_range` and `time_range` are added **before** `numeric_calc` in `ParseStatement` to prevent
greedy arithmetic consumption of date strings like `2024-01-01..2024-12-31`.

---

## 9. §09 Structures

*Source: `parser_v15_structures.go`, spec `09_stuctures.sqg`*

All nodes in this section are **V13 new**.

### Type contracts

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13HashableNode` | `TypeName string` | `hashable` (type union contract) |
| `V13SortableNode` | `TypeName string` | `sortable` (type union contract) |

### Structured table *(V13 new, distinct from V12 simple table)*

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13TableColNode` | `Name string` · `Type *V13InspectTypeNode` · `Nullable bool` | `table_col` |
| `V13TableColumnsNode` | `Cols []*V13TableColNode` | `table_columns` |
| `V13TableColFromObjNode` | `Ref *V13IdentRefNode` | `table_col_from_obj` |
| `V13KeyColNode` | `Name string` · `Type *V13InspectTypeNode` | `key_col` |
| `V13KeyColumnsNode` | `Cols []*V13KeyColNode` | `key_columns` |
| `V13TableRowNode` | `Values []V13Node` | `table_row` |
| `V13TableRowsListNode` | `Rows []*V13TableRowNode` | `table_rows_list` |
| `V13TableInitNode` | `Columns *V13TableColumnsNode` · `Keys *V13KeyColumnsNode` · `Rows *V13TableRowsListNode` | `table_init` (structured) |
| `V13TableInsTailStructNode` | `Rows *V13TableRowsListNode` | `table_ins_tail` (structured) |
| `V13TableFinalNode` | `Base V13Node` · `Tails []*V13TableInsTailStructNode` | `table_final` (structured) |

### Tree

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13TreeValueNode` | `Value V13Node` | `tree_value` |
| `V13TreeNodeNode` | `Value *V13TreeValueNode` · `Children []*V13TreeNodeNode` | `tree_node` |
| `V13TreeInitNode` | `Root *V13TreeNodeNode` | `tree_init` |
| `V13TreeInsTailNode` | `Nodes []*V13TreeNodeNode` | `tree_ins_tail` |
| `V13TreeFinalNode` | `Base V13Node` · `Tails []*V13TreeInsTailNode` | `tree_final` |

### String tree

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13StringTreeValueNode` | `Value *V13StringNode` | `string_tree_value` |
| `V13StringTreeNodeNode` | `Value *V13StringTreeValueNode` · `Children []*V13StringTreeNodeNode` | `string_tree_node` |
| `V13StringTreeInitNode` | `Root *V13StringTreeNodeNode` | `string_tree_init` |
| `V13StringTreeInsTailNode` | `Nodes []*V13StringTreeNodeNode` | `string_tree_ins_tail` |
| `V13StringTreeFinalNode` | `Base V13Node` · `Tails []*V13StringTreeInsTailNode` | `string_tree_final` |

### Keyed tree

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13KeyedTreeNodeNode` | `Key V13Node` · `Value *V13TreeValueNode` · `Children []*V13KeyedTreeNodeNode` | `keyed_tree_node` |
| `V13KeyedTreeInitNode` | `Root *V13KeyedTreeNodeNode` | `keyed_tree_init` |
| `V13KeyedTreeInsTailNode` | `Nodes []*V13KeyedTreeNodeNode` | `keyed_tree_ins_tail` |
| `V13KeyedTreeFinalNode` | `Base V13Node` · `Tails []*V13KeyedTreeInsTailNode` | `keyed_tree_final` |

### Sorted tree

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13SortedTreeNodeNode` | `Key V13Node` · `Value *V13TreeValueNode` · `Children []*V13SortedTreeNodeNode` | `sorted_tree_node` |
| `V13SortedTreeInitNode` | `Root *V13SortedTreeNodeNode` | `sorted_tree_init` |
| `V13SortedTreeInsTailNode` | `Nodes []*V13SortedTreeNodeNode` | `sorted_tree_ins_tail` |
| `V13SortedTreeFinalNode` | `Base V13Node` · `Tails []*V13SortedTreeInsTailNode` | `sorted_tree_final` |

### Set

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13SetInitNode` | `Values []V13Node` | `set_init` |
| `V13SetAddTailNode` | `Values []V13Node` | `set_add_tail` |
| `V13SetOmitTailNode` | `Values []V13Node` | `set_omit_tail` |
| `V13SetFinalNode` | `Base V13Node` · `Tails []V13Node` | `set_final` |

`V13SetFinalNode.Tails[i]` is `*V13SetAddTailNode` or `*V13SetOmitTailNode`.

### Enum

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13EnumMembersNode` | `Members []string` | `enum_members` |
| `V13EnumDeclNode` | `Name string` · `Members *V13EnumMembersNode` | `enum_decl` |
| `V13EnumExtendNode` | `Base *V13IdentRefNode` · `Members *V13EnumMembersNode` | `enum_extend` |
| `V13EnumFinalNode` | `Value V13Node` | `enum_final` |

`V13EnumFinalNode.Value` is `*V13EnumDeclNode` or `*V13EnumExtendNode`.

### Graph

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13GraphNodeNode` | `ID V13Node` · `Value V13Node` | `graph_node` |
| `V13GraphNodesNode` | `Nodes []*V13GraphNodeNode` | `graph_nodes` |
| `V13GraphEdgeNode` | `From V13Node` · `To V13Node` · `Label V13Node` | `graph_edge` |
| `V13GraphEdgesNode` | `Edges []*V13GraphEdgeNode` | `graph_edges` |
| `V13GraphInitNode` | `Nodes *V13GraphNodesNode` · `Edges *V13GraphEdgesNode` | `graph_init` |
| `V13GraphAddTailNode` | `Nodes *V13GraphNodesNode` · `Edges *V13GraphEdgesNode` | `graph_add_tail` |
| `V13GraphFinalNode` | `Base V13Node` · `Tails []*V13GraphAddTailNode` | `graph_final` |

### Bitfield

| Type | Fields | Grammar rule |
|------|--------|--------------|
| `V13BitfieldFlagNode` | `Name string` · `Bit *V13IntegerNode` | `bitfield_flag` |
| `V13BitfieldFlagsNode` | `Flags []*V13BitfieldFlagNode` | `bitfield_flags` |
| `V13BitfieldDeclNode` | `Name string` · `Base string` · `Flags *V13BitfieldFlagsNode` | `bitfield_decl` |
| `V13BitfieldFinalNode` | `Value *V13BitfieldDeclNode` | `bitfield_final` |

---

## 10. §10 Helper structs

These plain structs are **not** `V13Node` implementors. They appear as field
types in the node types above.

| Struct | Fields | Used by |
|--------|--------|---------|
| `V13TmplPart` | `IsExpr bool` · `Text string` | `V13StringNode.Parts`, `V13TmplUnquotedNode.Parts` |
| `V13DurationSegment` | `Digits string` · `Unit string` | `V13DurationNode.Segments` |
| `V13CastDirective` | `Source string` · `Targets []string` | `V13Parser.CastChains` |
| `V13AssignAnnotation` | `IdentName string` · `Cardin *V13CardinalityNode` · `Version *V13AssignVersionNode` | `V13AssignLHSNode.Annotations` |
| `V13NumExprTerm` | `Oper string` · `Expr *V13SingleNumExprNode` | `V13NumExprListNode.Terms` |
| `V13NumGroupTerm` | `Oper string` · `Expr V13Node` | `V13NumGroupingNode.Terms` |
| `V13StringExprTerm` | `Oper string` · `Str *V13StringNode` | `V13StringExprListNode.Terms` |
| `V13StringGroupTerm` | `Oper string` · `Expr V13Node` | `V13StringGroupingNode.Terms` |
| `V13LogicExprTerm` | `Oper string` · `Expr *V13SingleLogicExprNode` | `V13LogicExprListNode.Terms` |
| `V13LogicGroupTerm` | `Oper string` · `Expr V13Node` | `V13LogicGroupingNode.Terms` |
| `V13FuncCallChainStepNode` | `Op string` · `Ref *V13TypeOfRefNode` | `V13FuncCallChainNode.Steps` |
| `V13FuncInjectBind` | `LHS *V13AssignLHSNode` · `Oper *V13AssignOperNode` · `Ref *V13IdentRefNode` | `V13FuncInjectNode.Binds` · `V13FuncInjectBindNode.Bind` |
| `V13ScopeInjectBind` | `LHS *V13AssignLHSNode` · `Ref *V13IdentRefNode` | `V13ScopeInjectNode.Binds` |
| `V13DateRangeSide` | `Date *V13DateNode` · `TypedOf *V13IdentRefNode` | `V13DateRangeNode.Lo` / `.Hi` |
| `V13TimeRangeSide` | `Time *V13TimeNode` · `TypedOf *V13IdentRefNode` | `V13TimeRangeNode.Lo` / `.Hi` |
| `V13JPFilterExprPair` | `IsAnd bool` · `RHS *V13JPFilterUnaryNode` | `V13JPFilterExprNode.Pairs` |

---

## 11. §11 Enum types

### `V13AssignOperKind`

| Constant | Operator |
|----------|----------|
| `V13AssignMutable` | `=` |
| `V13AssignImmutable` | `:` |
| `V13AssignReadOnlyRef` | `:~` |
| `V13IncrAddImmutable` | `+:` |
| `V13IncrSubImmutable` | `-:` |
| `V13IncrMulImmutable` | `*:` |
| `V13IncrDivImmutable` | `/:` |
| `V13IncrAddMutable` | `+=` |
| `V13IncrSubMutable` | `-=` |
| `V13IncrMulMutable` | `*=` |
| `V13IncrDivMutable` | `/=` |

### `V13EmptyDeclKind`

| Constant | Token |
|----------|-------|
| `V13EmptyArray` | `[]` |
| `V13EmptyStream` | `>>` |
| `V13EmptyRegexp` | `//` |
| `V13EmptyScope` | `{}` |
| `V13EmptyStringD` | `""` |
| `V13EmptyStringS` | `''` |
| `V13EmptyStringT` | ` `` ` |

### `V13JPFilterOperKind`

| Constant | Operator |
|----------|----------|
| `JPFilterEqEq` | `==` |
| `JPFilterNeq` | `!=` |
| `JPFilterGEq` | `>=` |
| `JPFilterLEq` | `<=` |
| `JPFilterGt` | `>` |
| `JPFilterLt` | `<` |
| `JPFilterMatch` | `=~` |

### `V13StringUpdateOperKind`

| Constant | Operator(s) |
|----------|-------------|
| `StringAppendImmutable` | `+:` |
| `StringAppendMutable` | `+~` |
| `StringEqualAssign` | `=` / `:` / `:~` |

### `V13StringKind`

| Constant | Delimiter |
|----------|-----------|
| `V13StringSingle` | `'…'` |
| `V13StringDouble` | `"…"` |
| `V13StringTemplate` | `` `…` `` |

---

## 12. §12 V13Node variant reference

This table summarises which concrete types can appear in polymorphic `V13Node`
fields, grouped by the discriminating field name pattern.

| Field / context | Possible concrete types |
|-----------------|-------------------------|
| `V13NumericConstNode.Value` | `*V13IntegerNode`, `*V13DecimalNode` |
| `V13FloatNode.Value` | `*V13DecimalNode`, `*V13NanNode`, `*V13InfinityNode` |
| `V13ConstantNode.Value` | any leaf in §01 except helper structs |
| `V13CalcUnitNode.Value` | `*V13NumericExprNode`, `*V13StringExprNode`, `*V13LogicExprNode` |
| `V13NumericExprNode.Value` | `*V13NumExprListNode`, `*V13NumGroupingNode` |
| `V13StringExprNode.Value` | `*V13StringExprListNode`, `*V13StringGroupingNode` |
| `V13LogicExprNode.Value` | `*V13LogicExprListNode`, `*V13LogicGroupingNode` |
| `V13CompareExprNode.Value` | `*V13NumCompareNode`, `*V13StringCompareNode`, `*V13StringRegexpCompNode` |
| `V13AssignRHSNode.Value` | `*V13AssignRhsItemNode`, `*V13AssignRhsChainNode` |
| `V13AssignFuncRHSNode.Value` | `*V13ConstantNode`, `*V13EmptyDeclNode`, `*V13RegexpNode`, `*V13IdentRefNode`, `*V13CalcUnitNode`, `*V13ObjectFinalNode`, `*V13ArrayFinalNode` |
| `V13ArrayValueNode.Value` | `*V13ConstantNode`, `*V13RangeNode`, `*V13IdentRefNode`, `*V13CalcUnitNode`, `*V13ArrayFinalNode`, `*V13ObjectFinalNode` |
| `V13ArrayFinalNode.Tails[i]` | `*V13ArrayAppendTailNode`, `*V13ArrayOmitTailNode` |
| `V13ObjectFinalNode.Value` | `*V13ObjectInitNode`, `*V13ObjectMergeOrOmitNode`, `*V13EmptyDeclNode` |
| `V13ObjectMergeOrOmitNode.Base` | `*V13TypeOfRefNode`, `*V13ObjectInitNode` |
| `V13ObjectMergeOrOmitNode.Modifier` | `*V13ObjectMergeTailNode`, `*V13ObjectOmitTailNode` |
| `V13TableFinalNode.Base` | `*V13TypeOfRefNode`, `*V13TableInitNode` |
| `V13TableFinalSimpleNode.Base` | `*V13TypeOfRefNode`, `*V13TableInitSimpleNode` |
| `V13TableObjectsNode.Rows[i]` | `*V13ArrayFinalNode`, `*V13ObjectFinalNode` |
| `V13JPFilterValueNode.Value` | `*V13ConstantNode`, `*V13JPCurrentPathNode`, `*V13IdentRefNode` |
| `V13JPFilterAtomNode.Value` | `*V13JPFilterCmpNode`, `*V13JPCurrentPathNode`, `*V13JPFilterExprNode` |
| `V13FuncInjectNode.Head` | `nil`, `*V13FuncInjectHeadInspectNode`, `*V13FuncInjectBindNode` |
| `V13FuncStmtNode.Value` | `*V13RegexpNode`, `*V13IdentRefNode`, `*V13ObjectFinalNode`, `*V13ArrayFinalNode`, `*V13FuncCallChainNode`, `*V13FuncUnitNode`, `*V13CalcUnitNode`, `*V13SelfRefNode`, `*V13ReturnFuncUnitNode` |
| `V13FuncBodyStmtNode.Value` | `*V13FuncAssignNode`, `*V13FuncReturnStmtNode`, `*V13CondReturnStmtNode`, `*V13FuncStoreStmtNode`, `*V13IteratorYieldStmtNode`, `*V13PushForwardStmtNode`, `*V13PushStreamBindNode`, `*V13FuncStreamLoopNode`, `*V13FuncCallFinalNode` |
| `V13FuncCallFinalNode.Value` | `*V13FuncCallChainNode`, `*V13FuncStreamLoopNode` |
| `V13IteratorSourceNode.Value` | `*V13ArrayFinalNode`, `*V13FuncRangeArgsNode`, `*V13BoolNode`, `*V13IdentRefNode`, `*V13FuncCallFinalNode` |
| `V13PushSourceNode.Value` | `*V13ArrayFinalNode`, `*V13FuncRangeArgsNode`, `*V13BoolNode`, `*V13IdentRefNode`, `*V13FuncCallFinalNode` |
| `V13PushStreamBindNode.Stages[i]` | `*V13FuncUnitNode`, `*V13FuncCallNode` |
| `V13FuncStreamLoopNode.Source` | `*V13IteratorSourceNode` |
| `V13FuncStreamLoopNode.Body` | `*V13FuncUnitNode`, `*V13FuncCallNode` |
| `V13FuncRangeArgsNode.Value` | `*V13FuncFixedNumRangeNode`, `*V13StringNode`, `*V13TypeOfRefNode`, `*V13ArrayFinalNode`, `*V13ObjectFinalNode` |
| `V13UpdateNumberNode.RHS` | `*V13NumericConstNode`, `*V13NumericStmtNode` |
| `V13UpdateStringNode.RHS` | `*V13StringNode`, `*V13StringStmtNode` |
| `V13ScopeBodyItemNode.Value` | `*V13ImportAssignNode`, `*V13AssignmentNode`, `*V13ScopeAssignNode`, `*V13ScopeWithCatchNode`, `*V13ReceiverMethodAssignNode`, `*V13PrivateItemNode`, `*V13PrivateBlockNode` |
| `V13ImportAssignNode.Targets[i]` | `*V13URLNode`, `*V13FileURLNode` |
| `V13ParserRootNode.Value` | `*V13ImportAssignNode`, `*V13ScopeAssignNode`, `*V13ScopeWithCatchNode` |
| `V13UniqueKeyNode.Value` | `*V13UUIDNode`, `*V13UUIDV7Node`, `*V13ULIDNode`, `*V13SnowflakeIDNode`, `*V13NanoIDNode`, `*V13HashKeyNode`, `*V13SeqIDNode`, `*V13CompositeKeyNode` |
| `V13EnumFinalNode.Value` | `*V13EnumDeclNode`, `*V13EnumExtendNode` |
| `V13SetFinalNode.Tails[i]` | `*V13SetAddTailNode`, `*V13SetOmitTailNode` |
| `V13RegexpAssignNode.Str` | `*V13StringQuotedNode`, `*V13StringUnquotedNode` |
