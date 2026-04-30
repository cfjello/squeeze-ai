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
func dispatchTable(p *parser.V17Parser) map[string]func() (any, error) {
	return map[string]func() (any, error){
		// --- comments ---
		"comment_begin":    func() (any, error) { return p.ParseCommentBegin() },
		"comment_end":      func() (any, error) { return p.ParseCommentEnd() },
		"comment_txt":      func() (any, error) { return p.ParseCommentTxt() },
		"comment":          func() (any, error) { return p.ParseComment() },
		"comment_tbd_stub": func() (any, error) { return p.ParseCommentTbdStub() },

		// --- whitespace ---
		"nl":  func() (any, error) { return p.ParseNl() },
		"eol": func() (any, error) { return p.ParseEol() },

		// --- digits / sign ---
		"digits":      func() (any, error) { return p.ParseDigits() },
		"digits2":     func() (any, error) { return p.ParseDigits2() },
		"digits3":     func() (any, error) { return p.ParseDigits3() },
		"digits4":     func() (any, error) { return p.ParseDigits4() },
		"sign_prefix": func() (any, error) { return p.ParseSignPrefix() },

		// --- numerics ---
		"integer":       func() (any, error) { return p.ParseInteger() },
		"decimal":       func() (any, error) { return p.ParseDecimal() },
		"numeric_const": func() (any, error) { return p.ParseNumericConst() },
		"nan":           func() (any, error) { return p.ParseNan() },
		"infinity":      func() (any, error) { return p.ParseInfinity() },

		// --- uint types ---
		"byte":    func() (any, error) { return p.ParseByte() },
		"uint8":   func() (any, error) { return p.ParseUint8() },
		"uint16":  func() (any, error) { return p.ParseUint16() },
		"uint32":  func() (any, error) { return p.ParseUint32() },
		"uint64":  func() (any, error) { return p.ParseUint64() },
		"uint128": func() (any, error) { return p.ParseUint128() },

		// --- float types ---
		"float32": func() (any, error) { return p.ParseFloat32() },
		"float64": func() (any, error) { return p.ParseFloat64() },

		// --- decimal types ---
		"decimal8":    func() (any, error) { return p.ParseDecimal8() },
		"decimal16":   func() (any, error) { return p.ParseDecimal16() },
		"decimal32":   func() (any, error) { return p.ParseDecimal32() },
		"decimal64":   func() (any, error) { return p.ParseDecimal64() },
		"decimal128":  func() (any, error) { return p.ParseDecimal128() },
		"decimal_num": func() (any, error) { return p.ParseDecimalNum() },

		// --- date/time components ---
		"date_year":     func() (any, error) { return p.ParseDateYear() },
		"date_month":    func() (any, error) { return p.ParseDateMonth() },
		"date_day":      func() (any, error) { return p.ParseDateDay() },
		"time_hour":     func() (any, error) { return p.ParseTimeHour() },
		"time_minute":   func() (any, error) { return p.ParseTimeMinute() },
		"time_second":   func() (any, error) { return p.ParseTimeSecond() },
		"time_millis":   func() (any, error) { return p.ParseTimeMillis() },
		"date":          func() (any, error) { return p.ParseDate() },
		"time":          func() (any, error) { return p.ParseTime() },
		"date_time":     func() (any, error) { return p.ParseDateTime() },
		"time_stamp":    func() (any, error) { return p.ParseTimeStamp() },
		"duration_unit": func() (any, error) { return p.ParseDurationUnit() },
		"duration":      func() (any, error) { return p.ParseDuration() },

		// --- strings ---
		"single_quoted": func() (any, error) { return p.ParseSingleQuoted() },
		"double_quoted": func() (any, error) { return p.ParseDoubleQuoted() },
		"string_quoted": func() (any, error) { return p.ParseStringQuoted() },
		"tmpl_quoted":   func() (any, error) { return p.ParseTmplQuoted() },
		"string":        func() (any, error) { return p.ParseString() },

		// --- regexp ---
		"regexp_flags": func() (any, error) { return p.ParseRegexpFlags() },
		"regexp_expr":  func() (any, error) { return p.ParseRegexpExpr() },

		// --- boolean / null / any ---
		"boolean_true":  func() (any, error) { return p.ParseBooleanTrue() },
		"boolean_false": func() (any, error) { return p.ParseBooleanFalse() },
		"boolean":       func() (any, error) { return p.ParseBoolean() },
		"null":          func() (any, error) { return p.ParseNull() },
		"any_type":      func() (any, error) { return p.ParseAnyType() },

		// --- cardinality / range ---
		"cardinality": func() (any, error) { return p.ParseCardinality() },
		"range":       func() (any, error) { return p.ParseRange() },

		// --- hex segments ---
		"hex_seg2":   func() (any, error) { return p.ParseHexSeg2() },
		"hex_seg4":   func() (any, error) { return p.ParseHexSeg4() },
		"hex_seg8":   func() (any, error) { return p.ParseHexSeg8() },
		"hex_seg12":  func() (any, error) { return p.ParseHexSeg12() },
		"hex_seg32":  func() (any, error) { return p.ParseHexSeg32() },
		"hex_seg40":  func() (any, error) { return p.ParseHexSeg40() },
		"hex_seg64":  func() (any, error) { return p.ParseHexSeg64() },
		"hex_seg128": func() (any, error) { return p.ParseHexSeg128() },

		// --- UUID ---
		"uuid":        func() (any, error) { return p.ParseUuid() },
		"uuid_v7_ver": func() (any, error) { return p.ParseUuidV7Ver() },
		"uuid_v7_var": func() (any, error) { return p.ParseUuidV7Var() },
		"uuid_v7":     func() (any, error) { return p.ParseUuidV7() },

		// --- hashes ---
		"hash_md5":    func() (any, error) { return p.ParseHashMd5() },
		"hash_sha1":   func() (any, error) { return p.ParseHashSha1() },
		"hash_sha256": func() (any, error) { return p.ParseHashSha256() },
		"hash_sha512": func() (any, error) { return p.ParseHashSha512() },
		"hash_key":    func() (any, error) { return p.ParseHashKey() },

		// --- IDs ---
		"ulid":         func() (any, error) { return p.ParseUlid() },
		"nano_id":      func() (any, error) { return p.ParseNanoId() },
		"snowflake_id": func() (any, error) { return p.ParseSnowflakeId() },
		"seq_id16":     func() (any, error) { return p.ParseSeqId16() },
		"seq_id32":     func() (any, error) { return p.ParseSeqId32() },
		"seq_id64":     func() (any, error) { return p.ParseSeqId64() },
		"seq_id":       func() (any, error) { return p.ParseSeqId() },

		// --- keys / URLs ---
		"unique_key": func() (any, error) { return p.ParseUniqueKey() },
		"http_url":   func() (any, error) { return p.ParseHttpUrl() },
		"file_url":   func() (any, error) { return p.ParseFileUrl() },

		// --- top-level ---
		"constant": func() (any, error) { return p.ParseConstant() },

		// --- 02_operators: identifiers ---
		"ident_name":   func() (any, error) { return p.ParseIdentName() },
		"group_begin":  func() (any, error) { return p.ParseGroupBegin() },
		"group_end":    func() (any, error) { return p.ParseGroupEnd() },
		"ident_dotted": func() (any, error) { return p.ParseIdentDotted() },
		"ident_prefix": func() (any, error) { return p.ParseIdentPrefix() },
		"ident_ref":    func() (any, error) { return p.ParseIdentRef() },

		// --- 02_operators: numeric ---
		"numeric_oper":    func() (any, error) { return p.ParseNumericOper() },
		"inline_incr":     func() (any, error) { return p.ParseInlineIncr() },
		"single_num_expr": func() (any, error) { return p.ParseSingleNumExpr() },
		"num_expr_chain":  func() (any, error) { return p.ParseNumExprChain() },
		"num_grouping":    func() (any, error) { return p.ParseNumGrouping() },
		"numeric_calc":    func() (any, error) { return p.ParseNumericCalc() },

		// --- 02_operators: string ---
		"string_oper":       func() (any, error) { return p.ParseStringOper() },
		"string_expr_chain": func() (any, error) { return p.ParseStringExprChain() },
		"string_grouping":   func() (any, error) { return p.ParseStringGrouping() },
		"string_concat":     func() (any, error) { return p.ParseStringConcat() },

		// --- 02_operators: compare ---
		"compare_oper":   func() (any, error) { return p.ParseCompareOper() },
		"num_compare":    func() (any, error) { return p.ParseNumCompare() },
		"string_compare": func() (any, error) { return p.ParseStringCompare() },
		"condition":      func() (any, error) { return p.ParseCondition() },

		// --- 02_operators: logic ---
		"not_oper":           func() (any, error) { return p.ParseNotOper() },
		"logic_and":          func() (any, error) { return p.ParseLogicAnd() },
		"logic_or":           func() (any, error) { return p.ParseLogicOr() },
		"logic_exclusive_or": func() (any, error) { return p.ParseLogicExclusiveOr() },
		"logic_oper":         func() (any, error) { return p.ParseLogicOper() },
		"single_logic_expr":  func() (any, error) { return p.ParseSingleLogicExpr() },
		"logic_expr_chain":   func() (any, error) { return p.ParseLogicExprChain() },
		"logic_grouping":     func() (any, error) { return p.ParseLogicGrouping() },
		"logic_expr":         func() (any, error) { return p.ParseLogicExpr() },

		// --- 02_operators: statement ---
		"statement": func() (any, error) { return p.ParseStatement() },

		// --- 03_assignment ---
		"update_mutable_oper":     func() (any, error) { return p.ParseUpdateMutableOper() },
		"assign_mutable":          func() (any, error) { return p.ParseAssignMutable() },
		"assign_immutable":        func() (any, error) { return p.ParseAssignImmutable() },
		"assign_read_only_ref":    func() (any, error) { return p.ParseAssignReadOnlyRef() },
		"assign_version":          func() (any, error) { return p.ParseAssignVersion() },
		"assign_lhs":              func() (any, error) { return p.ParseAssignLhs() },
		"assign_cond_rhs":         func() (any, error) { return p.ParseAssignCondRhs() },
		"assign_single":           func() (any, error) { return p.ParseAssignSingle() },
		"private_modifier":        func() (any, error) { return p.ParsePrivateModifier() },
		"assign_private_single":   func() (any, error) { return p.ParseAssignPrivateSingle() },
		"assign_private_grouping": func() (any, error) { return p.ParseAssignPrivateGrouping() },
		"assign_rhs":              func() (any, error) { return p.ParseAssignRhs() },
		"assign_new_var":          func() (any, error) { return p.ParseAssignNewVar() },
		"update_mutable":          func() (any, error) { return p.ParseUpdateMutable() },
		"assignment":              func() (any, error) { return p.ParseAssignment() },
		"self_ref":                func() (any, error) { return p.ParseSelfRef() },
	}
}

// knownRules returns the sorted list of all dispatchable grammar rule names.
func knownRules() []string {
	rules := []string{
		"any_type", "boolean", "boolean_false", "boolean_true",
		"byte", "cardinality", "comment", "comment_begin", "comment_end",
		"comment_tbd_stub", "comment_txt", "constant",
		"date", "date_day", "date_month", "date_time", "date_year",
		"decimal", "decimal8", "decimal16", "decimal32", "decimal64", "decimal128", "decimal_num",
		"digits", "digits2", "digits3", "digits4", "double_quoted", "duration", "duration_unit",
		"eol", "file_url", "float32", "float64",
		"hash_key", "hash_md5", "hash_sha1", "hash_sha256", "hash_sha512",
		"hex_seg2", "hex_seg4", "hex_seg8", "hex_seg12", "hex_seg32", "hex_seg40", "hex_seg64", "hex_seg128",
		"http_url", "infinity", "integer",
		"nan", "nano_id", "nl", "null", "numeric_const",
		"range", "regexp_expr", "regexp_flags",
		"seq_id", "seq_id16", "seq_id32", "seq_id64",
		"sign_prefix", "single_quoted", "snowflake_id", "string", "string_quoted",
		"time", "time_hour", "time_millis", "time_minute", "time_second", "time_stamp",
		"tmpl_quoted", "uint8", "uint16", "uint32", "uint64", "uint128",
		"ulid", "unique_key", "uuid", "uuid_v7", "uuid_v7_var", "uuid_v7_ver",
		// 02_operators
		"compare_oper", "condition",
		"group_begin", "group_end",
		"ident_dotted", "ident_name", "ident_prefix", "ident_ref",
		"inline_incr",
		"logic_and", "logic_exclusive_or", "logic_expr", "logic_expr_chain",
		"logic_grouping", "logic_oper", "logic_or",
		"not_oper", "num_compare", "num_expr_chain", "num_grouping",
		"numeric_calc", "numeric_oper",
		"single_logic_expr", "single_num_expr", "statement",
		"string_compare", "string_concat", "string_expr_chain", "string_grouping", "string_oper",
		// 03_assignment
		"assign_cond_rhs", "assign_immutable", "assign_lhs", "assign_mutable",
		"assign_new_var", "assign_private_grouping", "assign_private_single",
		"assign_read_only_ref", "assign_rhs", "assign_single", "assign_version",
		"assignment", "private_modifier", "update_mutable", "update_mutable_oper",
		"self_ref",
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
	tokenFlag := flag.String("token", "", "Grammar rule to use as parse entry point (default: constant)")
	codeFlag := flag.String("code", "", "Squeeze source to parse (overrides file argument)")
	listFlag := flag.Bool("list", false, "List all available grammar rules and exit")
	debugFlag := flag.Bool("debug", false, "Enable parse trace output to stderr")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: v17parse [flags] [file]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  v17parse --token constant --code 'true'\n")
		fmt.Fprintf(os.Stderr, "  v17parse --token uuid --code '550e8400-e29b-41d4-a716-446655440000'\n")
		fmt.Fprintf(os.Stderr, "  v17parse --token duration --code '1h30m' --debug\n")
		fmt.Fprintf(os.Stderr, "  v17parse --list\n")
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
		if flag.NArg() == 0 {
			flag.Usage()
			os.Exit(1)
		}
		filePath := flag.Arg(0)
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
	lex := parser.NewV17Lexer(sourceText)
	tokens, err := lex.V17Tokenize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lexer error: %v\n", err)
		os.Exit(1)
	}
	for i, t := range tokens {
		fmt.Printf("  [%3d] %-22s %q  (line %d, col %d)\n", i, t.Type, t.Value, t.Line, t.Col)
	}

	// Build parser
	p := parser.NewV17Parser(tokens, sourceText)

	if *debugFlag {
		p.EnableDebug()
	}

	table := dispatchTable(p)

	// Determine entry point
	rule := strings.TrimSpace(*tokenFlag)
	if rule == "" {
		rule = "constant"
	}

	fn, ok := table[rule]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown grammar rule %q\n\nRun with --list to see available rules.\n", rule)
		os.Exit(1)
	}

	fmt.Printf("\n=== Parsing %s (rule: %s) ===\n", sourceLabel, rule)
	result, err := fn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s\n", p.FormatParseError(err))
		os.Exit(1)
	}

	fmt.Println("Parse OK")
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}
