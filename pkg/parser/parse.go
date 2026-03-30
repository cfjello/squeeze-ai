// parse.go — Top-level entry point for the Squeeze language parser.
//
// Parse ties together all three phases described in specifications/parser_v3.md:
//
//	Phase 1 — Lexer (parser_v3.go)
//	Phase 2 — Recursive descent (parser.go + rules_v1.go + rules_v3.go)
//	Phase 3 — Directive processor (directive_processor.go)
package parser

// =============================================================================
// Public API
// =============================================================================

// ParseResult holds the decorated AST and all accumulated errors from all
// three phases.
type ParseResult struct {
	Root   *Node
	Errors []ParseError
}

// Parse is the single entry point for callers.  It:
//  1. Tokenizes src via the Lexer.
//  2. Runs the recursive-descent parser to produce a raw AST.
//  3. Runs the DirectiveProcessor to validate and annotate the AST.
//
// All errors from every phase are collected and returned in ParseResult.Errors.
// Root is always non-nil (it may be a NodeProgram wrapping zero children if
// parsing fails completely).
func Parse(src string) ParseResult {
	// ── Phase 1: Lex ──────────────────────────────────────────────────────────
	l := NewLexer(src)
	tokens, lexErr := l.Tokenize()
	if lexErr != nil {
		errNode := NewErrorNode(lexErr.Error(), Pos{Line: 1, Col: 1})
		return ParseResult{
			Root: NewNode(NodeProgram, Pos{}, errNode),
			Errors: []ParseError{{
				Pos: Pos{Line: 1, Col: 1},
				Msg: lexErr.Error(),
			}},
		}
	}

	// ── Phase 2: Parse ────────────────────────────────────────────────────────
	p := NewParser(tokens)
	p.advance() // skip BOF
	root := p.parseProgram()
	parseErrors := p.Errors()

	// ── Phase 3: Directive processing ────────────────────────────────────────
	dp := NewDirectiveProcessor()
	dp.Process(root)
	directiveErrors := dp.Errors()

	allErrors := append(parseErrors, directiveErrors...)

	return ParseResult{
		Root:   root,
		Errors: allErrors,
	}
}

// =============================================================================
// parseProgram — the start rule
// =============================================================================

// parseProgram is the start rule.  A Squeeze source file is a sequence of
// top-level statements separated by optional blank lines / semicolons.
//
//	program = { EOL | scope_assign | table_init | assignment } EOF;
func (p *Parser) parseProgram() *Node {
	root := NewNode(NodeProgram, p.curPos())

	for !p.match(TOK_EOF) {
		// Skip blank lines / semicolons between statements.
		if p.matchAny(TOK_NL, TOK_SEMICOLON) {
			p.skipEOL()
			continue
		}

		stmt := p.parseTopLevelStmt()
		if stmt == nil {
			// Unrecognised token — record error and skip to next EOL to recover.
			p.errorf("unexpected token %q at top level", p.peek().Value)
			p.skipToEOL()
			continue
		}
		root.Append(stmt)
	}
	return root
}

// parseTopLevelStmt attempts to parse one top-level statement, trying each
// alternative in priority order.  Returns nil if nothing matches.
func (p *Parser) parseTopLevelStmt() *Node {
	// table_init starts with UNIFORM keyword.
	if p.match(TOK_UNIFORM) {
		if n := p.try(p.parseTableInit); n != nil {
			return n
		}
	}

	// scope_assign and assignment both start with an ident_name.
	if p.match(TOK_IDENT) {
		// Try scope_assign first (LHS oper "<" body ">") before plain assignment.
		if n := p.try(p.parseScopeAssign); n != nil {
			return n
		}
		if n := p.try(p.parseAssignment); n != nil {
			return n
		}
	}

	return nil
}

// skipToEOL advances past all tokens until the next NL, semicolon, or EOF,
// used for error recovery at the top level.
func (p *Parser) skipToEOL() {
	for !p.match(TOK_EOF) && !p.matchAny(TOK_NL, TOK_SEMICOLON) {
		p.advance()
	}
	p.skipEOL()
}
