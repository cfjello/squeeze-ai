//go:build ignore

// squeeze_v13_push_test.go — Specification-driven tests for push-model syntax.
// Covers spec/13_push_pull.sqg: push_source, push_recv_decl, push_forward_stmt,
// push_stream_bind, and assign_push (all three roles of the ~> operator).
package squeeze_v1_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parsePushSource(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParsePushSource()
	return err
}

func parsePushRecvDecl(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParsePushRecvDecl()
	return err
}

func parsePushForwardStmt(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParsePushForwardStmt()
	return err
}

func parsePushStreamBind(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParsePushStreamBind()
	return err
}

func parseAssignPush(src string) error {
	lex := parser.NewV13Lexer(src)
	toks, err := lex.V13Tokenize()
	if err != nil {
		return err
	}
	p := parser.NewV13Parser(toks)
	_, err = p.ParseAssignPush()
	return err
}

// ---------------------------------------------------------------------------
// Section 13.1 — "~>" token disambiguation
// ---------------------------------------------------------------------------

func TestV13_Push_LexPush(t *testing.T) {
	// "~>" lexes as V13_PUSH; bare "~" still lexes as V13_TILDE
	cases := []struct {
		src      string
		wantType parser.V13TokenType
	}{
		{"~>", parser.V13_PUSH},
		{"~", parser.V13_TILDE},
	}
	for _, tc := range cases {
		lex := parser.NewV13Lexer(tc.src)
		toks, err := lex.V13Tokenize()
		if err != nil {
			t.Errorf("lex %q: unexpected error: %v", tc.src, err)
			continue
		}
		// toks[0] is always V13_BOF; the operator token is at index 1.
		if len(toks) < 2 {
			t.Errorf("lex %q: expected at least 2 tokens, got %d", tc.src, len(toks))
			continue
		}
		if toks[1].Type != tc.wantType {
			t.Errorf("lex %q: want token type %v, got %v", tc.src, tc.wantType, toks[1].Type)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 13.2 — push_source
// ---------------------------------------------------------------------------

func TestV13_Push_PushSource_BooleanTrue(t *testing.T) {
	if err := parsePushSource("true"); err != nil {
		t.Errorf("push_source(true): unexpected error: %v", err)
	}
}

func TestV13_Push_PushSource_IdentRef(t *testing.T) {
	cases := []string{"my_events", "sensor_feed", "results"}
	for _, src := range cases {
		if err := parsePushSource(src); err != nil {
			t.Errorf("push_source(%q): unexpected error: %v", src, err)
		}
	}
}

func TestV13_Push_PushSource_FuncRangeArgs(t *testing.T) {
	cases := []string{"1..10", "0..99"}
	for _, src := range cases {
		if err := parsePushSource(src); err != nil {
			t.Errorf("push_source(%q): unexpected error: %v", src, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Section 13.7.1 — Role A: push_recv_decl  (~> ident: @T in header)
// ---------------------------------------------------------------------------

func TestV13_Push_PushRecvDecl_Simple(t *testing.T) {
	// ~> item: @my_type
	if err := parsePushRecvDecl("~> item: @my_type"); err != nil {
		t.Errorf("push_recv_decl(~> item: @my_type): unexpected error: %v", err)
	}
}

func TestV13_Push_PushRecvDecl_AnyType(t *testing.T) {
	// ~> event: @?
	if err := parsePushRecvDecl("~> event: @?"); err != nil {
		t.Errorf("push_recv_decl(~> event: @?): unexpected error: %v", err)
	}
}

func TestV13_Push_PushRecvDecl_RejectsMissingColon(t *testing.T) {
	// ~> item @my_type  — missing ":"
	if err := parsePushRecvDecl("~> item @my_type"); err == nil {
		t.Errorf("push_recv_decl: expected parse failure for missing ':'")
	}
}

// ---------------------------------------------------------------------------
// Section 13.7.2 — Role B: push_forward_stmt  (postfix ~> + NL)
// ---------------------------------------------------------------------------

func TestV13_Push_ForwardStmt_SimpleIdent(t *testing.T) {
	// enriched ~>\n  — postfix emit
	if err := parsePushForwardStmt("enriched ~>\n"); err != nil {
		t.Errorf("push_forward_stmt(enriched ~>): unexpected error: %v", err)
	}
}

func TestV13_Push_ForwardStmt_DottedRef(t *testing.T) {
	// rec.data ~>\n
	if err := parsePushForwardStmt("rec.data ~>\n"); err != nil {
		t.Errorf("push_forward_stmt(rec.data ~>): unexpected error: %v", err)
	}
}

func TestV13_Push_ForwardStmt_RejectsMissingNL(t *testing.T) {
	// "result ~> handler" — no NL after ~> means this is a stream bind, not a forward stmt
	if err := parsePushForwardStmt("result ~> handler"); err == nil {
		t.Errorf("push_forward_stmt: expected parse failure when '~>' not followed by NL")
	}
}

func TestV13_Push_ForwardStmt_RejectsNoOperator(t *testing.T) {
	// plain ident with no ~> at all
	if err := parsePushForwardStmt("result"); err == nil {
		t.Errorf("push_forward_stmt: expected parse failure with no '~>'")
	}
}

// ---------------------------------------------------------------------------
// Section 13.7.3 — Role C: push_stream_bind  (source ~> stage { ~> stage })
// ---------------------------------------------------------------------------

func TestV13_Push_StreamBind_SingleStage(t *testing.T) {
	// sensor_feed ~> { ~> item: @my_type\n<- item }
	src := "sensor_feed ~> {\n~> item: @my_type\n<- item\n}"
	if err := parsePushStreamBind(src); err != nil {
		t.Errorf("push_stream_bind single: unexpected error: %v", err)
	}
}

func TestV13_Push_StreamBind_ChainedStages(t *testing.T) {
	// events ~> { ~> item: @my_type\n<- item } ~> { ~> val: @?\n<- val }
	src := "events ~> {\n~> item: @my_type\n<- item\n} ~> {\n~> val: @?\n<- val\n}"
	if err := parsePushStreamBind(src); err != nil {
		t.Errorf("push_stream_bind chained: unexpected error: %v", err)
	}
}

func TestV13_Push_StreamBind_FuncUnitStage(t *testing.T) {
	// source ~> { ~> item: @my_type\n<- item.value }
	src := "sensor_feed ~> {\n~> item: @my_type\n<- item.value\n}"
	if err := parsePushStreamBind(src); err != nil {
		t.Errorf("push_stream_bind func_unit stage: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Section 13.7.4 — assign_push  (cold binding: source ~> EOL)
// ---------------------------------------------------------------------------

func TestV13_Push_AssignPush_IdentRef(t *testing.T) {
	cases := []string{
		"sensor_feed ~>\n",
		"event_bus ~>\n",
	}
	for _, src := range cases {
		if err := parseAssignPush(src); err != nil {
			t.Errorf("assign_push %q: unexpected error: %v", src, err)
		}
	}
}

func TestV13_Push_AssignPush_RangeSource(t *testing.T) {
	if err := parseAssignPush("1..100 ~>\n"); err != nil {
		t.Errorf("assign_push(range): unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Section 13.7.2 — ForwardStmt inside a func_stream_loop body (regression)
// ---------------------------------------------------------------------------

func TestV13_Push_ForwardInsideLoopBody(t *testing.T) {
	// rows >> ( >> rec: @row_type \n rec.data ~> \n )
	// Stream loop body with push_forward inside — both >> (yield) and ~> (push fwd) coexist.
	src := "rows >> (\n>> rec: @row_type\nrec.data ~>\n)"
	if err := parseFuncStreamLoop(src); err != nil {
		t.Errorf("push_forward inside stream loop: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AST node type assertions
// ---------------------------------------------------------------------------

func TestV13_Push_NodeTypes_PushSource(t *testing.T) {
	lex := parser.NewV13Lexer("my_feed")
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParsePushSource()
	if err != nil {
		t.Fatalf("ParsePushSource: %v", err)
	}
	if node == nil {
		t.Fatal("ParsePushSource returned nil node")
	}
	if node.Value == nil {
		t.Fatal("V13PushSourceNode.Value is nil")
	}
}

func TestV13_Push_NodeTypes_PushRecvDecl(t *testing.T) {
	lex := parser.NewV13Lexer("~> item: @event_type")
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParsePushRecvDecl()
	if err != nil {
		t.Fatalf("ParsePushRecvDecl: %v", err)
	}
	if node == nil {
		t.Fatal("ParsePushRecvDecl returned nil node")
	}
	if node.Name != "item" {
		t.Errorf("V13PushRecvDeclNode.Name: want %q, got %q", "item", node.Name)
	}
	if node.Type == nil {
		t.Fatal("V13PushRecvDeclNode.Type is nil")
	}
}

func TestV13_Push_NodeTypes_PushForwardStmt(t *testing.T) {
	lex := parser.NewV13Lexer("result ~>\n")
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParsePushForwardStmt()
	if err != nil {
		t.Fatalf("ParsePushForwardStmt: %v", err)
	}
	if node == nil {
		t.Fatal("ParsePushForwardStmt returned nil node")
	}
	if node.Stmt == nil {
		t.Fatal("V13PushForwardStmtNode.Stmt is nil")
	}
}

func TestV13_Push_NodeTypes_PushStreamBind(t *testing.T) {
	src := "events ~> {\n~> item: @my_type\n<- item\n}"
	lex := parser.NewV13Lexer(src)
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParsePushStreamBind()
	if err != nil {
		t.Fatalf("ParsePushStreamBind: %v", err)
	}
	if node == nil {
		t.Fatal("ParsePushStreamBind returned nil node")
	}
	if node.Source == nil {
		t.Fatal("V13PushStreamBindNode.Source is nil")
	}
	if len(node.Stages) == 0 {
		t.Fatal("V13PushStreamBindNode.Stages is empty")
	}
}

func TestV13_Push_NodeTypes_AssignPush(t *testing.T) {
	lex := parser.NewV13Lexer("sensor_feed ~>\n")
	toks, _ := lex.V13Tokenize()
	p := parser.NewV13Parser(toks)
	node, err := p.ParseAssignPush()
	if err != nil {
		t.Fatalf("ParseAssignPush: %v", err)
	}
	if node == nil {
		t.Fatal("ParseAssignPush returned nil node")
	}
	if node.Source == nil {
		t.Fatal("V13AssignPushNode.Source is nil")
	}
}
