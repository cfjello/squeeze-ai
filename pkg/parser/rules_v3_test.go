// rules_v3_test.go — Tests for the V3 grammar parsing functions.
package parser

import (
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func tokensV3(src string) []Token {
	l := NewLexer(src)
	toks, err := l.Tokenize()
	if err != nil {
		panic("tokenize: " + err.Error())
	}
	return toks
}

// parserV3 creates a parser for src and skips the leading BOF token so that
// the cursor sits on the first real token — consistent with parseWith()/parserFor().
func parserV3(src string) *Parser {
	p := NewParser(tokensV3(src))
	p.advance() // skip BOF
	return p
}

// ---------------------------------------------------------------------------
// Empty declarators
// ---------------------------------------------------------------------------

func TestParseEmptyArrayDecl(t *testing.T) {
	p := parserV3("[]")
	n := p.parseEmptyArrayDecl()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeEmptyArrayDecl {
		t.Fatalf("expected NodeEmptyArrayDecl, got %s", n.Kind)
	}
}

func TestParseFuncStreamDecl(t *testing.T) {
	p := parserV3(">>")
	n := p.parseFuncStreamDecl()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncStreamDecl {
		t.Fatalf("expected NodeFuncStreamDecl, got %s", n.Kind)
	}
}

func TestParseFuncRegexpDecl(t *testing.T) {
	p := parserV3("//")
	n := p.parseFuncRegexpDecl()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncRegexpDecl {
		t.Fatalf("expected NodeFuncRegexpDecl, got %s", n.Kind)
	}
}

func TestParseEmptyObjectDecl(t *testing.T) {
	p := parserV3("{}")
	n := p.parseEmptyObjectDecl()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeEmptyObjectDecl {
		t.Fatalf("expected NodeEmptyObjectDecl, got %s", n.Kind)
	}
}

func TestParseFuncStringDecl_DoubleQuote(t *testing.T) {
	p := parserV3(`""`)
	n := p.parseFuncStringDecl()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncStringDecl {
		t.Fatalf("expected NodeFuncStringDecl, got %s", n.Kind)
	}
}

func TestParseFuncStringDecl_SingleQuote(t *testing.T) {
	p := parserV3("''")
	n := p.parseFuncStringDecl()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
}

func TestParseFuncStringDecl_BacktickQuote(t *testing.T) {
	p := parserV3("``")
	n := p.parseFuncStringDecl()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
}

func TestParseEmptyDecl_Array(t *testing.T) {
	p := parserV3("[]")
	n := p.parseEmptyDecl()
	if n == nil || n.Kind != NodeEmptyDecl {
		t.Fatalf("expected NodeEmptyDecl, got %v", n)
	}
}

func TestParseEmptyDecl_Stream(t *testing.T) {
	p := parserV3(">>")
	n := p.parseEmptyDecl()
	if n == nil || n.Kind != NodeEmptyDecl {
		t.Fatalf("expected NodeEmptyDecl, got %v", n)
	}
}

// ---------------------------------------------------------------------------
// calc_unit
// ---------------------------------------------------------------------------

func TestParseCalcUnit_NumericExpr(t *testing.T) {
	p := parserV3("1 + 2")
	n := p.parseCalcUnit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeCalcUnit {
		t.Fatalf("expected NodeCalcUnit, got %s", n.Kind)
	}
}

func TestParseCalcUnit_StringExpr(t *testing.T) {
	p := parserV3(`"hello" + "world"`)
	n := p.parseCalcUnit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeCalcUnit {
		t.Fatalf("expected NodeCalcUnit, got %s", n.Kind)
	}
}

// ---------------------------------------------------------------------------
// assign_lhs
// ---------------------------------------------------------------------------

func TestParseAssignLHS_SingleIdent(t *testing.T) {
	p := parserV3("x")
	n := p.parseAssignLHS()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	// Result is UNIQUE directive wrapper
	if n.Kind != NodeDirectiveUnique {
		t.Fatalf("expected NodeDirectiveUnique, got %s", n.Kind)
	}
}

func TestParseAssignLHS_MultipleIdents(t *testing.T) {
	p := parserV3("x, y, z")
	n := p.parseAssignLHS()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeDirectiveUnique {
		t.Fatalf("expected NodeDirectiveUnique, got %s", n.Kind)
	}
	inner := n.Children[0] // NodeAssignLHS
	if inner == nil || inner.Kind != NodeAssignLHS {
		t.Fatalf("expected NodeAssignLHS child, got %v", inner)
	}
	// NodeAssignLHS should have 3 children (x, y, z)
	if len(inner.Children) != 3 {
		t.Fatalf("expected 3 children in assign_lhs, got %d", len(inner.Children))
	}
}

// ---------------------------------------------------------------------------
// Arrays
// ---------------------------------------------------------------------------

func TestParseArrayInit_SingleElement(t *testing.T) {
	p := parserV3("[42]")
	n := p.parseArrayInit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeArrayInit {
		t.Fatalf("expected NodeArrayInit, got %s", n.Kind)
	}
}

func TestParseArrayInit_MultipleElements(t *testing.T) {
	p := parserV3(`[1, 2, 3]`)
	n := p.parseArrayInit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeArrayInit {
		t.Fatalf("expected NodeArrayInit, got %s", n.Kind)
	}
}

func TestParseArrayInit_TrailingComma(t *testing.T) {
	p := parserV3(`["a", "b",]`)
	n := p.parseArrayInit()
	if n == nil {
		t.Fatal("expected node for trailing comma array, got nil")
	}
}

func TestParseArrayList_Empty(t *testing.T) {
	p := parserV3("[]")
	n := p.parseArrayList()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeArrayList {
		t.Fatalf("expected NodeArrayList, got %s", n.Kind)
	}
}

func TestParseArrayList_WithElements(t *testing.T) {
	p := parserV3("[1, 2]")
	n := p.parseArrayList()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeArrayList {
		t.Fatalf("expected NodeArrayList, got %s", n.Kind)
	}
}

func TestParseArrayLookup(t *testing.T) {
	p := parserV3("arr[0]")
	n := p.parseArrayLookup()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeArrayLookup {
		t.Fatalf("expected NodeArrayLookup, got %s", n.Kind)
	}
}

func TestParseArrayLookup_NotArray(t *testing.T) {
	p := parserV3("arr")
	n := p.parseArrayLookup()
	if n != nil {
		t.Fatal("expected nil for non-lookup, got node")
	}
}

// ---------------------------------------------------------------------------
// Objects
// ---------------------------------------------------------------------------

func TestParseObjectInit_Simple(t *testing.T) {
	p := parserV3(`{ x: 1 }`)
	n := p.parseObjectInit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeObjectInit {
		t.Fatalf("expected NodeObjectInit, got %s", n.Kind)
	}
}

func TestParseObjectInit_MultipleFields(t *testing.T) {
	p := parserV3(`{ name: "alice", age: 30 }`)
	n := p.parseObjectInit()
	if n == nil {
		t.Fatal("expected node for multi-field object, got nil")
	}
}

func TestParseObjectList_Empty(t *testing.T) {
	p := parserV3("{}")
	n := p.parseObjectList()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeObjectList {
		t.Fatalf("expected NodeObjectList, got %s", n.Kind)
	}
}

func TestParseObjectLookup(t *testing.T) {
	p := parserV3("obj[key]")
	n := p.parseObjectLookup()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeObjectLookup {
		t.Fatalf("expected NodeObjectLookup, got %s", n.Kind)
	}
}

// ---------------------------------------------------------------------------
// Getters
// ---------------------------------------------------------------------------

func TestParseGetName(t *testing.T) {
	// Lexer must emit @name as AT_IDENT
	p := parserV3("foo.@name")
	n := p.parseGetName()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeGetName {
		t.Fatalf("expected NodeGetName, got %s", n.Kind)
	}
}

func TestParseGetData(t *testing.T) {
	p := parserV3("foo.@data")
	n := p.parseGetData()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeGetData {
		t.Fatalf("expected NodeGetData, got %s", n.Kind)
	}
}

func TestParseGetStoreName(t *testing.T) {
	p := parserV3("foo.@storeName")
	n := p.parseGetStoreName()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeGetStoreName {
		t.Fatalf("expected NodeGetStoreName, got %s", n.Kind)
	}
}

func TestParseGetType(t *testing.T) {
	p := parserV3("foo.@type")
	n := p.parseGetType()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeGetType {
		t.Fatalf("expected NodeGetType, got %s", n.Kind)
	}
}

func TestParseGetTypeName(t *testing.T) {
	p := parserV3("foo.@type.@name")
	n := p.parseGetTypeName()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeGetTypeName {
		t.Fatalf("expected NodeGetTypeName, got %s", n.Kind)
	}
}

// ---------------------------------------------------------------------------
// Built-in header params
// ---------------------------------------------------------------------------

func TestParseStoreNameAssign(t *testing.T) {
	p := parserV3(`@storeName ~ "myStore"`)
	n := p.parseStoreNameAssign()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeStoreNameAssign {
		t.Fatalf("expected NodeStoreNameAssign, got %s", n.Kind)
	}
}

func TestParseDataAssign_ObjectList(t *testing.T) {
	p := parserV3(`@data : {}`)
	n := p.parseDataAssign()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeDataAssign {
		t.Fatalf("expected NodeDataAssign, got %s", n.Kind)
	}
}

func TestParseFuncHeaderBuildinParams_StoreName(t *testing.T) {
	p := parserV3(`@storeName ~ "s"`)
	n := p.parseFuncHeaderBuildinParams()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncHeaderBuildinParams {
		t.Fatalf("expected NodeFuncHeaderBuildinParams, got %s", n.Kind)
	}
}

// ---------------------------------------------------------------------------
// assign_rhs
// ---------------------------------------------------------------------------

func TestParseAssignRHS_Constant(t *testing.T) {
	p := parserV3("42")
	n := p.parseAssignRHS()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeAssignRHS {
		t.Fatalf("expected NodeAssignRHS, got %s", n.Kind)
	}
}

func TestParseAssignRHS_EmptyArr(t *testing.T) {
	p := parserV3("[]")
	n := p.parseAssignRHS()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
}

func TestParseAssignRHS_FuncStringDecl(t *testing.T) {
	p := parserV3(`""`)
	n := p.parseAssignRHS()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
}

// ---------------------------------------------------------------------------
// func_header_assign / func_header_user_params
// ---------------------------------------------------------------------------

func TestParseFuncHeaderAssign_Simple(t *testing.T) {
	p := parserV3("x : 1")
	n := p.parseFuncHeaderAssign()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncHeaderAssign {
		t.Fatalf("expected NodeFuncHeaderAssign, got %s", n.Kind)
	}
}

func TestParseFuncHeaderUserParams_MultiLine(t *testing.T) {
	p := parserV3("x : 1\ny : 2")
	n := p.parseFuncHeaderUserParams()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncHeaderUserParams {
		t.Fatalf("expected NodeFuncHeaderUserParams, got %s", n.Kind)
	}
	if len(n.Children) < 2 {
		t.Fatalf("expected at least 2 children, got %d", len(n.Children))
	}
}

// ---------------------------------------------------------------------------
// ident_static_*
// ---------------------------------------------------------------------------

func TestParseIdentStaticBoolean(t *testing.T) {
	p := parserV3("ok")
	n := p.parseIdentStaticBoolean()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeIdentStaticBoolean {
		t.Fatalf("expected NodeIdentStaticBoolean, got %s", n.Kind)
	}
}

func TestParseIdentStaticError(t *testing.T) {
	p := parserV3("error")
	n := p.parseIdentStaticError()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeIdentStaticError {
		t.Fatalf("expected NodeIdentStaticError, got %s", n.Kind)
	}
}

func TestParseIdentStaticDeps(t *testing.T) {
	p := parserV3("deps")
	n := p.parseIdentStaticDeps()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeIdentStaticDeps {
		t.Fatalf("expected NodeIdentStaticDeps, got %s", n.Kind)
	}
}

func TestParseIdentStaticFunc_Next(t *testing.T) {
	p := parserV3("next")
	n := p.parseIdentStaticFunc()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeIdentStaticFunc {
		t.Fatalf("expected NodeIdentStaticFunc, got %s", n.Kind)
	}
}

func TestParseIdentStaticFunc_Value(t *testing.T) {
	p := parserV3("value")
	n := p.parseIdentStaticFunc()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
}

func TestParseIdentStaticStoreName(t *testing.T) {
	p := parserV3("@storeName")
	n := p.parseIdentStaticStoreName()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeIdentStaticStoreName {
		t.Fatalf("expected NodeIdentStaticStoreName, got %s", n.Kind)
	}
}

func TestParseIdentStaticStr_AtName(t *testing.T) {
	p := parserV3("@name")
	n := p.parseIdentStaticStr()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeIdentStaticStr {
		t.Fatalf("expected NodeIdentStaticStr, got %s", n.Kind)
	}
}

func TestParseIdentStaticStr_AtType(t *testing.T) {
	p := parserV3("@type")
	n := p.parseIdentStaticStr()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
}

// ---------------------------------------------------------------------------
// func_args / func_stream_args / func_deps
// ---------------------------------------------------------------------------

func TestParseFuncArgs_Arrow_NoArgs(t *testing.T) {
	p := parserV3("->")
	n := p.parseFuncArgs()
	if n == nil {
		t.Fatal("expected node for '->', got nil")
	}
	if n.Kind != NodeFuncArgs {
		t.Fatalf("expected NodeFuncArgs, got %s", n.Kind)
	}
}

func TestParseFuncArgs_Mismatch(t *testing.T) {
	p := parserV3("x")
	n := p.parseFuncArgs()
	if n != nil {
		t.Fatal("expected nil when not '->''")
	}
}

func TestParseFuncDeps(t *testing.T) {
	p := parserV3("=> <@storeName>")
	n := p.parseFuncDeps()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncDeps {
		t.Fatalf("expected NodeFuncDeps, got %s", n.Kind)
	}
}

// ---------------------------------------------------------------------------
// func_range_args
// ---------------------------------------------------------------------------

func TestParseFuncFixedNumRange(t *testing.T) {
	p := parserV3("1..10")
	n := p.parseFuncFixedNumRange()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncFixedNumRange {
		t.Fatalf("expected NodeFuncFixedNumRange, got %s", n.Kind)
	}
}

func TestParseFuncFixedNumRange_Mismatch(t *testing.T) {
	p := parserV3("x")
	n := p.parseFuncFixedNumRange()
	if n != nil {
		t.Fatal("expected nil")
	}
}

func TestParseFuncRangeArgs_NumRange(t *testing.T) {
	p := parserV3("1..10")
	n := p.parseFuncRangeArgs()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncRangeArgs {
		t.Fatalf("expected NodeFuncRangeArgs, got %s", n.Kind)
	}
}

func TestParseFuncRangeArgs_ListRange(t *testing.T) {
	p := parserV3("[1, 2, 3]")
	n := p.parseFuncRangeArgs()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncRangeArgs {
		t.Fatalf("expected NodeFuncRangeArgs, got %s", n.Kind)
	}
}

// ---------------------------------------------------------------------------
// func_call_args
// ---------------------------------------------------------------------------

func TestParseFuncCallArgs_Single(t *testing.T) {
	p := parserV3("42")
	n := p.parseFuncCallArgs()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncCallArgs {
		t.Fatalf("expected NodeFuncCallArgs, got %s", n.Kind)
	}
}

func TestParseFuncCallArgs_Multiple(t *testing.T) {
	p := parserV3(`42, "hello", true`)
	n := p.parseFuncCallArgs()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if len(n.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(n.Children))
	}
}

// ---------------------------------------------------------------------------
// func_body_stmt / assignment / return / store
// ---------------------------------------------------------------------------

func TestParseAssignment_Simple(t *testing.T) {
	p := parserV3("x : 42")
	n := p.parseAssignment()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeAssignment {
		t.Fatalf("expected NodeAssignment, got %s", n.Kind)
	}
}

func TestParseFuncReturnStmt(t *testing.T) {
	p := parserV3("<- 42")
	n := p.parseFuncReturnStmt()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncReturnStmt {
		t.Fatalf("expected NodeFuncReturnStmt, got %s", n.Kind)
	}
}

func TestParseFuncBodyStmt_Assignment(t *testing.T) {
	p := parserV3("x : 1")
	n := p.parseFuncBodyStmt()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncBodyStmt {
		t.Fatalf("expected NodeFuncBodyStmt, got %s", n.Kind)
	}
}

func TestParseFuncBodyStmt_Return(t *testing.T) {
	p := parserV3("<- 0")
	n := p.parseFuncBodyStmt()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
}

// ---------------------------------------------------------------------------
// func_unit
// ---------------------------------------------------------------------------

func TestParseFuncUnit_Empty(t *testing.T) {
	p := parserV3("()")
	n := p.parseFuncUnit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncUnit {
		t.Fatalf("expected NodeFuncUnit, got %s", n.Kind)
	}
}

func TestParseFuncUnit_WithBody(t *testing.T) {
	p := parserV3("( x : 1\n<- x )")
	n := p.parseFuncUnit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeFuncUnit {
		t.Fatalf("expected NodeFuncUnit, got %s", n.Kind)
	}
}

func TestParseFuncUnit_Mismatch(t *testing.T) {
	p := parserV3("x")
	n := p.parseFuncUnit()
	if n != nil {
		t.Fatal("expected nil for non-func_unit")
	}
}

// ---------------------------------------------------------------------------
// scope_assign / scope_unit
// ---------------------------------------------------------------------------

func TestParseScopeUnit_Simple(t *testing.T) {
	p := parserV3("x : 1")
	n := p.parseScopeUnit()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeScopeUnit {
		t.Fatalf("expected NodeScopeUnit, got %s", n.Kind)
	}
}

func TestParseScopeAssign(t *testing.T) {
	// scope_assign = assign_lhs equal_assign "<" scope_unit ">" EOL
	// equal_assign = "~" | ":" | "<<<" (assign_mutable | assign_immutable | assign_read_only_ref)
	p := parserV3("myScope : < x : 1 >\n")
	n := p.parseScopeAssign()
	if n == nil {
		t.Fatal("expected node, got nil")
	}
	if n.Kind != NodeScopeAssign {
		t.Fatalf("expected NodeScopeAssign, got %s", n.Kind)
	}
}
