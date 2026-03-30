package parser

import (
	"testing"
)

func TestLexerSmoke(t *testing.T) {
	src := "name : 42\nfoo ~ /hello.*/i\nresult : foo + bar\n"
	l := NewLexer(src)
	tokens, err := l.Tokenize()
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}

	// First token must be BOF, last must be EOF.
	if tokens[0].Type != TOK_BOF {
		t.Errorf("expected first token BOF, got %s", tokens[0].Type)
	}
	if tokens[len(tokens)-1].Type != TOK_EOF {
		t.Errorf("expected last token EOF, got %s", tokens[len(tokens)-1])
	}

	// Print all tokens for visual inspection when running with -v.
	for _, tok := range tokens {
		t.Log(tok)
	}
}

func TestLexerOperators(t *testing.T) {
	cases := []struct {
		src      string
		wantType TokenType
		wantVal  string
	}{
		{"..", TOK_DOTDOT, ".."},
		{"->", TOK_ARROW, "->"},
		{">>", TOK_STREAM, ">>"},
		{"=>", TOK_STORE, "=>"},
		{"<-", TOK_RETURN_STMT, "<-"},
		{"++", TOK_INC, "++"},
		{"--", TOK_DEC, "--"},
		{"**", TOK_POW, "**"},
		{"+:", TOK_IADD_IMM, "+:"},
		{"-:", TOK_ISUB_IMM, "-:"},
		{"*:", TOK_IMUL_IMM, "*:"},
		{"/:", TOK_IDIV_IMM, "/:"},
		{"+~", TOK_IADD_MUT, "+~"},
		{"-~", TOK_ISUB_MUT, "-~"},
		{"*~", TOK_IMUL_MUT, "*~"},
		{"/~", TOK_IDIV_MUT, "/~"},
		{":~", TOK_READONLY, ":~"},
		{">=", TOK_GEQ, ">="},
		{"<=", TOK_LEQ, "<="},
		{"!=", TOK_NEQ, "!="},
		{"=~", TOK_MATCH_OP, "=~"},
		{"[]", TOK_EMPTY_ARR, "[]"},
		{"{}", TOK_EMPTY_OBJ, "{}"},
		{`""`, TOK_EMPTY_STR_D, `""`},
		{"''", TOK_EMPTY_STR_S, "''"},
		{"``", TOK_EMPTY_STR_T, "``"},
		{"//", TOK_REGEXP_DECL, "//"},
	}

	for _, c := range cases {
		l := NewLexer(c.src)
		toks, err := l.Tokenize()
		if err != nil {
			t.Errorf("src=%q: unexpected error: %v", c.src, err)
			continue
		}
		// toks[0]=BOF, toks[1]=operator, toks[2]=EOF
		if len(toks) < 3 {
			t.Errorf("src=%q: expected at least 3 tokens, got %d", c.src, len(toks))
			continue
		}
		got := toks[1]
		if got.Type != c.wantType || got.Value != c.wantVal {
			t.Errorf("src=%q: want Token{%s %q}, got %s", c.src, c.wantType, c.wantVal, got)
		}
	}
}

func TestLexerRegexpVsDivision(t *testing.T) {
	// After an integer, / is division.
	l := NewLexer("4 / 2")
	toks, _ := l.Tokenize()
	// BOF  4  /  2  EOF
	if toks[2].Type != TOK_SLASH {
		t.Errorf("expected TOK_SLASH after integer, got %s", toks[2])
	}

	// After an assignment operator, / starts a regexp.
	l = NewLexer("x : /foo/i")
	toks, _ = l.Tokenize()
	// BOF  x  :  /foo/i  EOF
	if toks[3].Type != TOK_REGEXP {
		t.Errorf("expected TOK_REGEXP after ':', got %s", toks[3])
	}
}

func TestLexerIdentWithSpaces(t *testing.T) {
	l := NewLexer("my cool name : 1")
	toks, err := l.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// BOF  "my cool name"  :  1  EOF
	if toks[1].Type != TOK_IDENT || toks[1].Value != "my cool name" {
		t.Errorf("expected IDENT 'my cool name', got %s", toks[1])
	}
}

func TestLexerKeywords(t *testing.T) {
	keywords := map[string]TokenType{
		"true": TOK_TRUE, "false": TOK_FALSE,
		"UNIQUE": TOK_UNIQUE, "RANGE": TOK_RANGE_KW, "TYPE_OF": TOK_TYPE_OF,
		"VALUE_OF": TOK_VALUE_OF, "ADDRESS_OF": TOK_ADDRESS_OF,
		"RETURN": TOK_RETURN_DIR, "UNIFORM": TOK_UNIFORM, "INFER": TOK_INFER,
	}
	for kw, want := range keywords {
		l := NewLexer(kw)
		toks, _ := l.Tokenize()
		if toks[1].Type != want {
			t.Errorf("keyword %q: want %s, got %s", kw, want, toks[1])
		}
	}
}
