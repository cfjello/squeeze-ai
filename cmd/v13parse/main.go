package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// dispatchTable maps snake_case grammar rule names to a parse function.
// Each wrapper accepts a *parser.V13Parser and returns (any, error).
func dispatchTable(p *parser.V13Parser) map[string]func() (any, error) {
	return map[string]func() (any, error){
		// --- top-level ---
		"parser_root": func() (any, error) { return p.ParseParserRoot() },
		"scope_final": func() (any, error) { return p.ParseScopeFinal() },

		// --- assignment ---
		"assignment":       func() (any, error) { return p.ParseAssignment() },
		"assign_lhs":       func() (any, error) { return p.ParseAssignLHS() },
		"assign_rhs":       func() (any, error) { return p.ParseAssignRHS() },
		"assign_rhs_chain": func() (any, error) { return p.ParseAssignRhsChain() },
		"assign_rhs_item":  func() (any, error) { return p.ParseAssignRhsItem() },
		"assign_oper":      func() (any, error) { return p.ParseAssignOper() },
		"assign_version":   func() (any, error) { return p.ParseAssignVersion() },
		"assign_func_rhs":  func() (any, error) { return p.ParseAssignFuncRHS() },

		// --- scope ---
		"scope_assign":        func() (any, error) { return p.ParseScopeAssign() },
		"scope_body":          func() (any, error) { return p.ParseScopeBody() },
		"scope_inject":        func() (any, error) { return p.ParseScopeInject() },
		"scope_with_catch":    func() (any, error) { return p.ParseScopeWithCatch() },
		"scope_body_catch":    func() (any, error) { return p.ParseScopeBodyCatch() },
		"scope_merge_tail":    func() (any, error) { return p.ParseScopeMergeTail() },
		"import_assign":       func() (any, error) { return p.ParseImportAssign() },
		"other_inline_assign": func() (any, error) { return p.ParseOtherInlineAssign() },

		// --- functions ---
		"func_assign":           func() (any, error) { return p.ParseFuncAssign() },
		"func_assign_rhs_chain": func() (any, error) { return p.ParseFuncAssignRhsChain() },
		"func_unit":             func() (any, error) { return p.ParseFuncUnit() },
		"func_scope_assign":     func() (any, error) { return p.ParseFuncScopeAssign() },
		"func_body_stmt":        func() (any, error) { return p.ParseFuncBodyStmt() },
		"func_stmt":             func() (any, error) { return p.ParseFuncStmt() },
		"func_return_stmt":      func() (any, error) { return p.ParseFuncReturnStmt() },
		"func_store_stmt":       func() (any, error) { return p.ParseFuncStoreStmt() },
		"func_inject":           func() (any, error) { return p.ParseFuncInject() },
		"func_args":             func() (any, error) { return p.ParseFuncArgs() },
		"func_args_decl":        func() (any, error) { return p.ParseFuncArgsDecl() },
		"func_stream_args":      func() (any, error) { return p.ParseFuncStreamArgs() },
		"func_deps":             func() (any, error) { return p.ParseFuncDeps() },
		"func_range_args":       func() (any, error) { return p.ParseFuncRangeArgs() },
		"func_call":             func() (any, error) { return p.ParseFuncCall() },
		"func_call_chain":       func() (any, error) { return p.ParseFuncCallChain() },
		"func_call_final":       func() (any, error) { return p.ParseFuncCallFinal() },
		"func_stream_loop":      func() (any, error) { return p.ParseFuncStreamLoop() },
		"return_func_unit":      func() (any, error) { return p.ParseReturnFuncUnit() },
		"update_func_unit":      func() (any, error) { return p.ParseUpdateFuncUnit() },
		"array_idx_recursive":   func() (any, error) { return p.ParseArrayIdxRecursive() },
		"update_number":         func() (any, error) { return p.ParseUpdateNumber() },
		"string_update_oper":    func() (any, error) { return p.ParseStringUpdateOper() },
		"update_string":         func() (any, error) { return p.ParseUpdateString() },
		"ident_ref_update":      func() (any, error) { return p.ParseIdentRefUpdate() },
		"http_url":              func() (any, error) { return p.ParseHTTPURL() },
		"file_url":              func() (any, error) { return p.ParseFileURL() },
		"push_source":           func() (any, error) { return p.ParsePushSource() },
		"assign_push":           func() (any, error) { return p.ParseAssignPush() },
		"push_recv_decl":        func() (any, error) { return p.ParsePushRecvDecl() },
		"push_forward_stmt":     func() (any, error) { return p.ParsePushForwardStmt() },
		"push_stream_bind":      func() (any, error) { return p.ParsePushStreamBind() },
		"pipeline_decl":         func() (any, error) { return p.ParsePipelineDecl() },
		"pipeline_call":         func() (any, error) { return p.ParsePipelineCall() },
		"iterator_source":       func() (any, error) { return p.ParseIteratorSource() },
		"iterator_yield_stmt":   func() (any, error) { return p.ParseIteratorYieldStmt() },
		"assign_iterator":       func() (any, error) { return p.ParseAssignIterator() },
		"args_decl": func() (any, error) {
			nodes, err := p.ParseArgsDecl()
			return nodes, err
		},

		// --- receiver/inspect ---
		"receiver_clause":        func() (any, error) { return p.ParseReceiverClause() },
		"receiver_method_assign": func() (any, error) { return p.ParseReceiverMethodAssign() },
		"inspect_type":           func() (any, error) { return p.ParseInspectType() },
		"type_declare":           func() (any, error) { return p.ParseTypeDeclare() },

		// --- operators / expressions ---
		"ident_ref":         func() (any, error) { return p.ParseIdentRef() },
		"ident_dotted":      func() (any, error) { return p.ParseIdentDotted() },
		"ident_prefix":      func() (any, error) { return p.ParseIdentPrefix() },
		"type_ref":          func() (any, error) { return p.ParseTypeRef() },
		"single_num_expr":   func() (any, error) { return p.ParseSingleNumExpr() },
		"num_expr_list":     func() (any, error) { return p.ParseNumExprList() },
		"num_grouping":      func() (any, error) { return p.ParseNumGrouping() },
		"numeric_expr":      func() (any, error) { return p.ParseNumericExpr() },
		"string_expr_list":  func() (any, error) { return p.ParseStringExprList() },
		"string_grouping":   func() (any, error) { return p.ParseStringGrouping() },
		"string_expr":       func() (any, error) { return p.ParseStringExpr() },
		"compare_expr":      func() (any, error) { return p.ParseCompareExpr() },
		"single_logic_expr": func() (any, error) { return p.ParseSingleLogicExpr() },
		"logic_expr_list":   func() (any, error) { return p.ParseLogicExprList() },
		"logic_grouping":    func() (any, error) { return p.ParseLogicGrouping() },
		"logic_expr":        func() (any, error) { return p.ParseLogicExpr() },
		"calc_unit":         func() (any, error) { return p.ParseCalcUnit() },

		// --- objects / arrays ---
		"empty_decl":           func() (any, error) { return p.ParseEmptyDecl() },
		"array_uniform":        func() (any, error) { return p.ParseArrayUniform() },
		"empty_array_typed":    func() (any, error) { return p.ParseEmptyArrayTyped() },
		"array_list":           func() (any, error) { return p.ParseArrayList() },
		"plain_array":          func() (any, error) { return p.ParsePlainArray() },
		"array_append_tail":    func() (any, error) { return p.ParseArrayAppendTail() },
		"array_omit_tail":      func() (any, error) { return p.ParseArrayOmitTail() },
		"array_final":          func() (any, error) { return p.ParseArrayFinal() },
		"lookup_idx_expr":      func() (any, error) { return p.ParseLookupIdxExpr() },
		"array_lookup":         func() (any, error) { return p.ParseArrayLookup() },
		"object_entry":         func() (any, error) { return p.ParseObjectEntry() },
		"object_init":          func() (any, error) { return p.ParseObjectInit() },
		"object_merge_tail":    func() (any, error) { return p.ParseObjectMergeTail() },
		"object_merge_or_omit": func() (any, error) { return p.ParseObjectMergeOrOmit() },
		"object_final":         func() (any, error) { return p.ParseObjectFinal() },
		"object_lookup":        func() (any, error) { return p.ParseObjectLookup() },
		"lhs_caller":           func() (any, error) { return p.ParseLhsCaller() },
		"bootstrap_call":       func() (any, error) { return p.ParseBootstrapCall() },
		"array_value":          func() (any, error) { return p.ParseArrayValue() },
		"table_header":         func() (any, error) { return p.ParseTableHeader() },
		"table_objects":        func() (any, error) { return p.ParseTableObjects() },
		"table_init_simple":    func() (any, error) { return p.ParseTableInitSimple() },
		"table_ins_tail":       func() (any, error) { return p.ParseTableInsTail() },
		"table_final_simple":   func() (any, error) { return p.ParseTableFinalSimple() },
		"split_array":          func() (any, error) { return p.ParseSplitArray() },

		// --- ranges ---
		"num_range_valid":      func() (any, error) { return p.ParseNumRangeValid() },
		"date_range":           func() (any, error) { return p.ParseDateRange() },
		"date_range_valid":     func() (any, error) { return p.ParseDateRangeValid() },
		"time_range":           func() (any, error) { return p.ParseTimeRange() },
		"time_range_valid":     func() (any, error) { return p.ParseTimeRangeValid() },
		"regexp_assign":        func() (any, error) { return p.ParseRegexpAssign() },
		"array_default_range":  func() (any, error) { return p.ParseArrayDefaultRange() },
		"object_default_range": func() (any, error) { return p.ParseObjectDefaultRange() },

		// --- structures ---
		"unique_key":    func() (any, error) { return p.ParseUniqueKey() },
		"hashable":      func() (any, error) { return p.ParseHashable() },
		"sortable":      func() (any, error) { return p.ParseSortable() },
		"table_columns": func() (any, error) { return p.ParseTableColumns() },
		"key_columns":   func() (any, error) { return p.ParseKeyColumns() },
		"table_row":     func() (any, error) { return p.ParseTableRow() },
		"table_init":    func() (any, error) { return p.ParseTableInit() },
		"table_final":   func() (any, error) { return p.ParseTableFinal() },
		"tree_node":     func() (any, error) { return p.ParseTreeNode() },
		"tree_init":     func() (any, error) { return p.ParseTreeInit() },

		// --- JSON path ---
		"json_path":           func() (any, error) { return p.ParseJSONPath() },
		"ident_ref_with_path": func() (any, error) { return p.ParseIdentRefWithPath() },
		"jp_filter_expr":      func() (any, error) { return p.ParseJPFilterExpr() },

		// --- primitives ---
		"integer":         func() (any, error) { return p.ParseInteger() },
		"decimal":         func() (any, error) { return p.ParseDecimal() },
		"numeric_const":   func() (any, error) { return p.ParseNumericConst() },
		"nan":             func() (any, error) { return p.ParseNan() },
		"infinity":        func() (any, error) { return p.ParseInfinity() },
		"cast_directive":  func() (any, error) { return p.ParseCastDirective() },
		"string":          func() (any, error) { return p.ParseString() },
		"string_quoted":   func() (any, error) { return p.ParseStringQuoted() },
		"string_unquoted": func() (any, error) { return p.ParseStringUnquoted() },
		"any_type":        func() (any, error) { return p.ParseAnyType() },
		"date":            func() (any, error) { return p.ParseDate() },
		"time":            func() (any, error) { return p.ParseTime() },
		"duration":        func() (any, error) { return p.ParseDuration() },
		"regexp":          func() (any, error) { return p.ParseRegexp() },
		"uuid":            func() (any, error) { return p.ParseUUID() },
		"uuid_v7":         func() (any, error) { return p.ParseUUIDV7() },
		"ulid":            func() (any, error) { return p.ParseULID() },
		"nano_id":         func() (any, error) { return p.ParseNanoID() },
		"snowflake_id":    func() (any, error) { return p.ParseSnowflakeID() },
		"boolean":         func() (any, error) { return p.ParseBoolean() },
		"null":            func() (any, error) { return p.ParseNull() },
		"cardinality":     func() (any, error) { return p.ParseCardinality() },
		"range":           func() (any, error) { return p.ParseRange() },
		"constant":        func() (any, error) { return p.ParseConstant() },
		"sign_prefix": func() (any, error) {
			node := p.ParseSignPrefix()
			return node, nil
		},
	}
}

// knownRules returns the sorted list of all dispatchable grammar rule names.
// Must be kept in sync with dispatchTable keys.
func knownRules() []string {
	rules := []string{
		"any_type", "args_decl", "array_append_tail", "array_default_range",
		"array_final", "array_idx_recursive", "array_list", "array_lookup",
		"array_omit_tail", "array_uniform", "array_value", "assign_func_rhs",
		"assign_iterator", "assign_lhs", "assign_oper", "assign_push",
		"assign_rhs", "assign_rhs_chain", "assign_rhs_item", "assign_version", "assignment", "boolean",
		"bootstrap_call", "calc_unit", "cardinality", "cast_directive",
		"compare_expr", "constant", "date", "date_range", "date_range_valid",
		"decimal", "duration", "empty_array_typed", "empty_decl",
		"file_url", "func_args", "func_args_decl", "func_assign", "func_assign_rhs_chain",
		"func_body_stmt", "func_call", "func_call_chain", "func_call_final",
		"func_deps", "func_inject", "func_range_args", "func_return_stmt",
		"func_scope_assign", "func_stmt", "func_store_stmt", "func_stream_args",
		"func_stream_loop", "func_unit", "hashable", "http_url",
		"ident_dotted", "ident_prefix", "ident_ref", "ident_ref_update",
		"ident_ref_with_path", "import_assign", "infinity", "inspect_type",
		"integer", "iterator_source", "iterator_yield_stmt", "jp_filter_expr",
		"json_path", "key_columns", "lhs_caller", "logic_expr",
		"logic_expr_list", "logic_grouping", "lookup_idx_expr",
		"nan", "null", "num_expr_list", "num_grouping", "num_range_valid",
		"numeric_const", "numeric_expr", "object_entry", "object_final",
		"object_init", "object_lookup", "object_merge_or_omit",
		"object_merge_tail", "other_inline_assign", "parser_root",
		"pipeline_call", "pipeline_decl", "plain_array", "push_forward_stmt",
		"push_recv_decl", "push_source", "push_stream_bind", "range",
		"receiver_clause", "receiver_method_assign", "regexp", "regexp_assign",
		"return_func_unit", "scope_assign", "scope_body", "scope_body_catch",
		"scope_final", "scope_inject", "scope_merge_tail", "scope_with_catch",
		"sign_prefix", "single_logic_expr", "single_num_expr", "snowflake_id",
		"sortable", "split_array", "string", "string_expr", "string_expr_list",
		"string_grouping", "string_quoted", "string_unquoted",
		"string_update_oper", "table_columns", "table_final", "table_final_simple",
		"table_header", "table_init", "table_init_simple", "table_ins_tail",
		"table_objects", "table_row", "time", "time_range", "time_range_valid",
		"tree_init", "tree_node", "type_declare", "type_ref",
		"ulid", "unique_key", "update_func_unit", "update_number",
		"update_string", "uuid", "uuid_v7",
	}
	sort.Strings(rules)
	return rules
}

func printRules() {
	fmt.Println("Available grammar rules:")
	for _, n := range knownRules() {
		fmt.Printf("  %s\n", n)
	}
}

func main() {
	tokenFlag := flag.String("token", "", "Grammar rule to use as parse entry point (default: parser_root)")
	codeFlag := flag.String("code", "", "Squeeze source code to parse (overrides file argument)")
	listFlag := flag.Bool("list", false, "List all available grammar rules and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: v13parse [flags] [file]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  v13parse pkg/lib/array.sqz\n")
		fmt.Fprintf(os.Stderr, "  v13parse --token assignment --code 'x: 42'\n")
		fmt.Fprintf(os.Stderr, "  v13parse --token numeric_expr --code '1 + 2 * 3'\n")
		fmt.Fprintf(os.Stderr, "  v13parse --list\n")
	}
	flag.Parse()

	if *listFlag {
		printRules()
		return
	}

	// Determine source text
	var sourceLabel string
	var sourceText string

	if *codeFlag != "" {
		sourceLabel = "<inline>"
		sourceText = *codeFlag
	} else {
		filePath := "pkg/lib/base_library.sqz"
		if flag.NArg() > 0 {
			filePath = flag.Arg(0)
		}
		src, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			os.Exit(1)
		}
		sourceLabel = filePath
		sourceText = string(src)
	}

	// Lex
	fmt.Printf("=== Tokenising %s ===\n", sourceLabel)
	lex := parser.NewV13Lexer(sourceText)
	tokens, err := lex.V13Tokenize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lexer error: %v\n", err)
		os.Exit(1)
	}
	for i, t := range tokens {
		fmt.Printf("  [%3d] %-18s %q  (line %d, col %d)\n", i, t.Type, t.Value, t.Line, t.Col)
	}

	// Build parser and dispatch table
	p := parser.NewV13Parser(tokens)
	table := dispatchTable(p)

	// Determine entry point
	rule := strings.TrimSpace(*tokenFlag)
	if rule == "" {
		rule = "parser_root"
	}

	fn, ok := table[rule]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown grammar rule %q\n\nRun with --list to see available rules.\n", rule)
		os.Exit(1)
	}

	fmt.Printf("\n=== Parsing %s (rule: %s) ===\n", sourceLabel, rule)
	result, err := fn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nPARSE ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Parse OK")
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}
