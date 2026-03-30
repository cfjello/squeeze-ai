// rules_v1.go — Recursive-descent parsing functions for the V1 bootstrap grammar.
//
// Rule names map 1-to-1 to the EBNF in spec/squeeze_v1.ebnf.txt:
//
//	every "parseXxx" method corresponds to the grammar rule "xxx".
//
// Convention:
//   - A parse function returns *Node on success, nil when this rule simply
//     does not apply at the current position (caller tries next alternative).
//   - The parser position is NEVER advanced on a nil return (the save/restore
//     mechanism in helpers methods guarantees this).
//   - When a rule is required (the caller already committed to this branch),
//     parse errors are recorded via p.errorf and an error node is returned.
//
// Grammar directives (TYPE_OF, UNIQUE, RANGE …) are grammar-spec annotations,
// not user-visible syntax.  Where a rule is annotated with a directive the
// parser wraps the resulting sub-tree in the matching NewDirectiveNode so that
// Phase-3 (the directive processor) can act on it.
package parser

// =============================================================================
// NL / EOL
// =============================================================================

// parseNL matches one NL token (newline sequence) or BOF.
//   NL = /([ \t]*[\r\n]+)+/ | BOF;
func (p *Parser) parseNL() *Node {
	if !p.matchAny(TOK_NL, TOK_BOF) {
		return nil
	}
	tok := p.advance()
	return NewTokenNode(tok)
}

// parseEOL matches one end-of-line marker.
//   EOL = NL | ";" | EOF;
func (p *Parser) parseEOL() *Node {
	if !p.matchAny(TOK_NL, TOK_BOF, TOK_SEMICOLON, TOK_EOF) {
		return nil
	}
	tok := p.advance()
	return NewTokenNode(tok)
}

// =============================================================================
// Numeric primitives
// =============================================================================

// parseDigits parses a bare (unsigned) integer digit sequence.
//   digits = /[0-9]+/;
// The lexer emits unsigned digit sequences as TOK_INTEGER.
func (p *Parser) parseDigits() *Node {
	if !p.match(TOK_INTEGER) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeDigits, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseInteger parses an optionally-signed integer.
//   integer = [ "+" | "-" ] digits;
func (p *Parser) parseInteger() *Node {
	// Signs are only valid if immediately followed by digits, so we need a
	// 1-token look-ahead when a sign is present.
	if p.matchAny(TOK_PLUS, TOK_MINUS) {
		if p.peek2().Type != TOK_INTEGER {
			return nil // lone + or – is not an integer literal
		}
	} else if !p.match(TOK_INTEGER) {
		return nil
	}
	pos := p.curPos()
	n := NewNode(NodeInteger, pos)
	if p.matchAny(TOK_PLUS, TOK_MINUS) {
		n.Append(NewTokenNode(p.advance()))
	}
	n.Append(p.parseDigits())
	return n
}

// parseDecimal parses an optionally-signed decimal number.
//   decimal = [ "+" | "-" ] digits "." digits;
// The lexer emits the unsigned decimal (both digit groups and the dot) as a
// single TOK_DECIMAL token; the optional sign is a separate TOK_PLUS/TOK_MINUS.
func (p *Parser) parseDecimal() *Node {
	if p.matchAny(TOK_PLUS, TOK_MINUS) {
		if p.peek2().Type != TOK_DECIMAL {
			return nil
		}
	} else if !p.match(TOK_DECIMAL) {
		return nil
	}
	pos := p.curPos()
	n := NewNode(NodeDecimal, pos)
	if p.matchAny(TOK_PLUS, TOK_MINUS) {
		n.Append(NewTokenNode(p.advance()))
	}
	tok := p.advance() // TOK_DECIMAL ("digits.digits")
	n.Append(NewTokenNode(tok))
	return n
}

// parseNumericConst parses either a decimal or integer numeric constant.
//   numeric_const = integer | decimal;
// Decimal is tried first because both start with an optional sign followed by
// digits, but only decimal has a fractional part (the lexer emits TOK_DECIMAL).
func (p *Parser) parseNumericConst() *Node {
	// Determine whether we're looking at a decimal or integer by checking the
	// concrete numeric token (possibly after skipping over an optional sign).
	var target TokenType
	if p.matchAny(TOK_PLUS, TOK_MINUS) {
		target = p.peek2().Type
	} else {
		target = p.peek().Type
	}

	pos := p.curPos()
	var child *Node
	switch target {
	case TOK_DECIMAL:
		child = p.parseDecimal()
	case TOK_INTEGER:
		child = p.parseInteger()
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeNumericConst, pos, child)
}

// =============================================================================
// String primitives
// =============================================================================

// parseSingleQuoted parses a single-quoted string token.
//   single_quoted = "'" /…/ "'";
// The lexer emits the full quoted string (delimiters included) as one
// TOK_STRING whose value starts with "'".
func (p *Parser) parseSingleQuoted() *Node {
	if !p.match(TOK_STRING) || len(p.peek().Value) == 0 || p.peek().Value[0] != '\'' {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeSingleQuoted, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseDoubleQuoted parses a double-quoted string token.
//   double_quoted = "\"" /…/ "\"";
func (p *Parser) parseDoubleQuoted() *Node {
	if !p.match(TOK_STRING) || len(p.peek().Value) == 0 || p.peek().Value[0] != '"' {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeDoubleQuoted, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseTmplQuoted parses a template-quoted (backtick) string token.
//   tmpl_quoted = "`" /…/ "`";
func (p *Parser) parseTmplQuoted() *Node {
	if !p.match(TOK_STRING) || len(p.peek().Value) == 0 || p.peek().Value[0] != '`' {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeTmplQuoted, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseString parses any string literal (single, double, or template-quoted).
//   string = single_quoted | double_quoted | tmpl_quoted;
func (p *Parser) parseString() *Node {
	if !p.match(TOK_STRING) {
		return nil
	}
	pos := p.curPos()
	var child *Node
	switch p.peek().Value[0] {
	case '\'':
		child = p.parseSingleQuoted()
	case '"':
		child = p.parseDoubleQuoted()
	case '`':
		child = p.parseTmplQuoted()
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeString, pos, child)
}

// =============================================================================
// Regexp
// =============================================================================

// parseRegexpFlags parses a single regexp flag identifier ("g" or "i").
// NOTE: the Squeeze lexer embeds flags directly into the TOK_REGEXP token
// (e.g. "/pattern/gi"), so this function is only useful if flags are split
// off separately by a future lexer revision.
//   regexp_flags = "g" | "i";
func (p *Parser) parseRegexpFlags() *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	v := p.peek().Value
	if v != "g" && v != "i" {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeRegexpFlags, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseRegexp parses a regexp literal.
//   regexp = "/" TYPE_OF XRegExp</.*/> "/" [ regexp_flags { regexp_flags } ];
// The lexer emits the complete regexp (including flags) as a single TOK_REGEXP
// token (e.g. "/pattern/gi"), so no separate flag parsing is needed.
// The TYPE_OF XRegExp directive is a grammar annotation: the lexer guarantees
// the content is a valid XRegExp pattern, so we don't need to re-validate here.
func (p *Parser) parseRegexp() *Node {
	if !p.match(TOK_REGEXP) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeRegexp, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// =============================================================================
// Boolean
// =============================================================================

// parseBooleanTrue parses the "true" keyword.
//   boolean_true = "true";
func (p *Parser) parseBooleanTrue() *Node {
	if !p.match(TOK_TRUE) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeBooleanTrue, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseBooleanFalse parses the "false" keyword.
//   boolean_false = "false";
func (p *Parser) parseBooleanFalse() *Node {
	if !p.match(TOK_FALSE) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeBooleanFalse, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseBoolean parses a boolean literal.
//   boolean = "true" | "false";
func (p *Parser) parseBoolean() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_TRUE:
		child = p.parseBooleanTrue()
	case TOK_FALSE:
		child = p.parseBooleanFalse()
	default:
		return nil
	}
	return NewNode(NodeBoolean, pos, child)
}

// =============================================================================
// Constant
// =============================================================================

// parseConstant parses any constant literal.
//   constant = numeric_const | string | regexp | boolean;
func (p *Parser) parseConstant() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_INTEGER, TOK_DECIMAL:
		child = p.parseNumericConst()
	case TOK_PLUS, TOK_MINUS:
		// Signed numeric constant — peek2 determines decimal vs integer.
		child = p.parseNumericConst()
	case TOK_STRING:
		child = p.parseString()
	case TOK_REGEXP:
		child = p.parseRegexp()
	case TOK_TRUE, TOK_FALSE:
		child = p.parseBoolean()
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeConstant, pos, child)
}

// =============================================================================
// Identifiers
// =============================================================================

// parseIdentName parses a single identifier name (may contain embedded spaces).
//   ident_name = /(?<value>[\p{L}][\p{L}0-9_ ]*)/;
func (p *Parser) parseIdentName() *Node {
	if !p.match(TOK_IDENT) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIdentName, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIdentDotted parses a dot-separated sequence of identifier names.
//   ident_dotted = ident_name { "." ident_name };
func (p *Parser) parseIdentDotted() *Node {
	if !p.enter("ident_dotted") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	first := p.parseIdentName()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeIdentDotted, pos, first)
	for p.match(TOK_DOT) {
		// Only consume the dot if a valid ident_name follows.
		c := p.save()
		p.advance() // consume "."
		name := p.parseIdentName()
		if name == nil {
			p.restore(c)
			break
		}
		n.Append(NewTokenNode(Token{Type: TOK_DOT, Value: "."}), name)
	}
	return n
}

// parseIdentPrefix parses a relative-path prefix used in ident_ref.
//   ident_prefix = ( "../" { "../" } ) | ( "./" );
//
// NOTE: The Squeeze lexer currently emits '.' as TOK_DOT and '..' as TOK_DOTDOT,
// then treats '/' as a regexp start (not TOK_SLASH) when not preceded by a
// value token.  Full support for '../' and './' path prefixes therefore
// requires a future lexer update.  Until then this function always returns nil.
func (p *Parser) parseIdentPrefix() *Node {
	// "./" case: TOK_DOT TOK_SLASH
	if p.match(TOK_DOT) && p.peek2().Type == TOK_SLASH {
		pos := p.curPos()
		dotTok := p.advance()   // "."
		slashTok := p.advance() // "/"
		return NewNode(NodeIdentPrefix, pos,
			NewTokenNode(dotTok), NewTokenNode(slashTok))
	}
	// "../" { "../" } case: TOK_DOTDOT TOK_SLASH { TOK_DOTDOT TOK_SLASH }
	if p.match(TOK_DOTDOT) && p.peek2().Type == TOK_SLASH {
		pos := p.curPos()
		n := NewNode(NodeIdentPrefix, pos)
		for p.match(TOK_DOTDOT) && p.peek2().Type == TOK_SLASH {
			n.Append(NewTokenNode(p.advance())) // ".."
			n.Append(NewTokenNode(p.advance())) // "/"
		}
		return n
	}
	return nil
}

// parseIdentRef parses a (possibly prefixed) dotted identifier reference.
//   ident_ref = [ ident_prefix ] ident_dotted;
func (p *Parser) parseIdentRef() *Node {
	pos := p.curPos()
	n := NewNode(NodeIdentRef, pos)
	if prefix := p.parseIdentPrefix(); prefix != nil {
		n.Append(prefix)
	}
	dotted := p.parseIdentDotted()
	if dotted == nil {
		if len(n.Children) > 0 {
			p.errorf("expected identifier after path prefix")
		}
		return nil
	}
	n.Append(dotted)
	return n
}

// =============================================================================
// Numeric operators and expressions
// =============================================================================

// parseNumericOper parses a binary numeric operator.
//   numeric_oper = "+" | "-" | "*" | "**" | "/" | "%" ;
func (p *Parser) parseNumericOper() *Node {
	var tok Token
	switch p.peek().Type {
	case TOK_POW: // "**" — must be matched before TOK_STAR
		tok = p.advance()
	case TOK_PLUS, TOK_MINUS, TOK_STAR, TOK_SLASH, TOK_PERCENT:
		tok = p.advance()
	default:
		return nil
	}
	return NewNode(NodeNumericOper, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// isNumericOperFirst reports whether the current token can start a numeric_oper.
func (p *Parser) isNumericOperFirst() bool {
	return p.matchAny(TOK_PLUS, TOK_MINUS, TOK_STAR, TOK_POW, TOK_SLASH, TOK_PERCENT)
}

// parseInlineIncr parses "++" or "--".
//   inline_incr = "++" | "--";
func (p *Parser) parseInlineIncr() *Node {
	if !p.matchAny(TOK_INC, TOK_DEC) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeInlineIncr, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseSingleNumExpr parses a single operand inside a numeric expression.
//
//	single_num_expr = [ inline_incr ] numeric_const
//	                | TYPE_OF numeric_const<ident_ref>;
//
// The second alternative is a grammar-directive form: user writes an ident_ref;
// the parser wraps the result in NodeDirectiveTypeOf("numeric_const") for
// Phase-3 type validation.
func (p *Parser) parseSingleNumExpr() *Node {
	if !p.enter("single_num_expr") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()

	// TYPE_OF numeric_const<ident_ref> — variable reference in numeric context.
	if p.match(TOK_IDENT) {
		ref := p.parseIdentRef()
		if ref != nil {
			inner := NewNode(NodeSingleNumExpr, pos, ref)
			return NewDirectiveNode(NodeDirectiveTypeOf, "numeric_const", pos, inner)
		}
	}

	// [ inline_incr ] numeric_const
	n := NewNode(NodeSingleNumExpr, pos)
	if incr := p.parseInlineIncr(); incr != nil {
		n.Append(incr)
	}
	nc := p.parseNumericConst()
	if nc == nil {
		return nil
	}
	n.Append(nc)
	return n
}

// parseNumExprList parses a flat list of numeric operands joined by operators.
//   num_expr_list = single_num_expr { numeric_oper single_num_expr };
func (p *Parser) parseNumExprList() *Node {
	if !p.enter("num_expr_list") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	first := p.parseSingleNumExpr()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeNumExprList, pos, first)
	for p.isNumericOperFirst() {
		// Use backtracking in case the operator is followed by something that
		// is NOT a valid single_num_expr (e.g. end of grouping).
		c := p.save()
		oper := p.parseNumericOper()
		next := p.parseSingleNumExpr()
		if next == nil {
			p.restore(c)
			break
		}
		n.Append(oper, next)
	}
	return n
}

// parseNumGroupingItem parses the inner alternatives of a numeric grouping.
func (p *Parser) parseNumGroupingItem() *Node {
	if p.match(TOK_LPAREN) {
		return p.parseNumGrouping()
	}
	return p.parseNumExprList()
}

// parseNumGrouping parses a parenthesised numeric expression.
//   num_grouping = "(" ( num_expr_list | num_grouping )
//                     { numeric_oper ( num_expr_list | num_grouping ) } ")";
func (p *Parser) parseNumGrouping() *Node {
	if !p.enter("num_grouping") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	if !p.match(TOK_LPAREN) {
		return nil
	}
	pos := p.curPos()
	p.advance() // consume "("

	first := p.parseNumGroupingItem()
	if first == nil {
		p.errorf("expected numeric expression inside '('")
		return nil
	}
	n := NewNode(NodeNumGrouping, pos, first)
	for p.isNumericOperFirst() {
		c := p.save()
		oper := p.parseNumericOper()
		item := p.parseNumGroupingItem()
		if item == nil {
			p.restore(c)
			break
		}
		n.Append(oper, item)
	}
	p.consume(TOK_RPAREN)
	return n
}

// parseNumericExpr parses a complete numeric expression.
//   numeric_expr = num_expr_list | num_grouping;
func (p *Parser) parseNumericExpr() *Node {
	if !p.enter("numeric_expr") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	var child *Node
	if p.match(TOK_LPAREN) {
		child = p.parseNumGrouping()
	} else {
		child = p.parseNumExprList()
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeNumericExpr, pos, child)
}

// =============================================================================
// String expressions
// =============================================================================

// parseStringOper parses the string concatenation operator "+".
//   string_oper = "+";
func (p *Parser) parseStringOper() *Node {
	if !p.match(TOK_PLUS) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeStringOper, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseStringExprList parses a string concatenation expression.
//   string_expr_list = string string_oper string { string_oper string };
// Requires at least two string literals joined by "+".
func (p *Parser) parseStringExprList() *Node {
	c := p.save()
	first := p.parseString()
	if first == nil {
		return nil
	}
	if !p.match(TOK_PLUS) {
		p.restore(c) // single string alone is NOT a string_expr_list
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeStringExprList, pos, first)
	for p.match(TOK_PLUS) {
		oper := p.parseStringOper()
		next := p.parseString()
		if next == nil {
			p.errorf("expected string after '+' in string expression")
			break
		}
		n.Append(oper, next)
	}
	return n
}

// parseStringGroupingItem parses the inner alternatives of a string grouping.
func (p *Parser) parseStringGroupingItem() *Node {
	if p.match(TOK_LPAREN) {
		return p.parseStringGrouping()
	}
	return p.parseStringExprList()
}

// parseStringGrouping parses a parenthesised string-concat expression.
//   string_grouping = "(" ( string_expr_list | string_grouping )
//                        { string_oper ( string_expr_list | string_grouping ) } ")";
func (p *Parser) parseStringGrouping() *Node {
	if !p.enter("string_grouping") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	if !p.match(TOK_LPAREN) {
		return nil
	}
	pos := p.curPos()
	p.advance() // consume "("

	first := p.parseStringGroupingItem()
	if first == nil {
		p.errorf("expected string expression inside '('")
		return nil
	}
	n := NewNode(NodeStringGrouping, pos, first)
	for p.match(TOK_PLUS) {
		oper := p.parseStringOper()
		item := p.parseStringGroupingItem()
		if item == nil {
			p.errorf("expected string expression after '+' in grouping")
			break
		}
		n.Append(oper, item)
	}
	p.consume(TOK_RPAREN)
	return n
}

// parseStringExpr parses a complete string expression.
//   string_expr = string_expr_list | string_grouping;
func (p *Parser) parseStringExpr() *Node {
	if !p.enter("string_expr") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	var child *Node
	if p.match(TOK_LPAREN) {
		child = p.parseStringGrouping()
	} else {
		child = p.parseStringExprList()
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeStringExpr, pos, child)
}

// =============================================================================
// Compare expressions
// =============================================================================

// parseCompareOper parses a comparison operator.
//   compare_oper = "!=" | "=" | ">" | ">=" | "<" | "<=";
// '<' is valid here because compare expressions are always parsed with the
// inExpression context flag set (via withExpression), preventing '<' from
// being mistaken for a scope or directive bracket.
func (p *Parser) parseCompareOper() *Node {
	var tok Token
	switch p.peek().Type {
	case TOK_NEQ, TOK_EQ, TOK_GT, TOK_GEQ, TOK_LEQ:
		tok = p.advance()
	case TOK_LT:
		// '<' as compare operator is only valid inside an expression context.
		if p.ltIsCompare() {
			tok = p.advance()
		} else {
			return nil
		}
	default:
		return nil
	}
	return NewNode(NodeCompareOper, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// isCompareOperFirst reports whether the current token can start a compare_oper.
func (p *Parser) isCompareOperFirst() bool {
	switch p.peek().Type {
	case TOK_NEQ, TOK_EQ, TOK_GT, TOK_GEQ, TOK_LEQ:
		return true
	case TOK_LT:
		return p.ltIsCompare()
	}
	return false
}

// parseNumCompare parses a numeric comparison expression.
//   num_compare = TYPE_OF boolean<numeric_expr compare_oper numeric_expr>;
// In user source the user writes "3 > 2" or "x > y"; the TYPE_OF boolean<…>
// is a grammar directive so the parser wraps the result in a directive node.
func (p *Parser) parseNumCompare() *Node {
	if !p.enter("num_compare") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	c := p.save()

	left := p.withExpression(p.parseNumericExpr)
	if left == nil {
		return nil
	}
	if !p.isCompareOperFirst() {
		p.restore(c)
		return nil
	}
	oper := p.withExpression(p.parseCompareOper)
	right := p.withExpression(p.parseNumericExpr)
	if right == nil {
		p.restore(c)
		return nil
	}
	inner := NewNode(NodeNumCompare, pos, left, oper, right)
	return NewDirectiveNode(NodeDirectiveTypeOf, "boolean", pos, inner)
}

// parseStringCompare parses a string comparison expression.
//   string_compare = TYPE_OF boolean<string_expr compare_oper string_expr>;
func (p *Parser) parseStringCompare() *Node {
	if !p.enter("string_compare") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	c := p.save()

	left := p.withExpression(p.parseStringExpr)
	if left == nil {
		return nil
	}
	if !p.isCompareOperFirst() {
		p.restore(c)
		return nil
	}
	oper := p.withExpression(p.parseCompareOper)
	right := p.withExpression(p.parseStringExpr)
	if right == nil {
		p.restore(c)
		return nil
	}
	inner := NewNode(NodeStringCompare, pos, left, oper, right)
	return NewDirectiveNode(NodeDirectiveTypeOf, "boolean", pos, inner)
}

// parseStringRegexpComp parses a string-regexp match expression.
//   string_regexp_comp = TYPE_OF boolean<string "=~" regexp>;
func (p *Parser) parseStringRegexpComp() *Node {
	if !p.enter("string_regexp_comp") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	c := p.save()

	str := p.parseString()
	if str == nil {
		return nil
	}
	if !p.match(TOK_MATCH_OP) {
		p.restore(c)
		return nil
	}
	matchTok := p.advance() // "=~"
	re := p.parseRegexp()
	if re == nil {
		p.restore(c)
		return nil
	}
	inner := NewNode(NodeStringRegexpComp, pos, str, NewTokenNode(matchTok), re)
	return NewDirectiveNode(NodeDirectiveTypeOf, "boolean", pos, inner)
}

// parseCompareExpr parses any compare expression.
//   compare_expr = num_compare | string_compare | string_regexp_comp;
// The most specific alternative is tried first.
func (p *Parser) parseCompareExpr() *Node {
	pos := p.curPos()
	// string_regexp_comp is most specific (requires "=~")
	if n := p.try(p.parseStringRegexpComp); n != nil {
		return NewNode(NodeCompareExpr, pos, n)
	}
	// string_compare requires string expressions on both sides
	if n := p.try(p.parseStringCompare); n != nil {
		return NewNode(NodeCompareExpr, pos, n)
	}
	// numeric comparison is the most general
	if n := p.try(p.parseNumCompare); n != nil {
		return NewNode(NodeCompareExpr, pos, n)
	}
	return nil
}

// =============================================================================
// Logic expressions
// =============================================================================

// parseNotOper parses the logical-not operator "!".
//   not_oper = "!";
func (p *Parser) parseNotOper() *Node {
	if !p.match(TOK_BANG) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeNotOper, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseLogicOper parses a binary logic operator.
//   logic_oper = "&" | "|" | "^";
func (p *Parser) parseLogicOper() *Node {
	if !p.matchAny(TOK_AMP, TOK_PIPE, TOK_CARET) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeLogicOper, Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseSingleLogicExpr parses a single atom of a logic expression.
//
//	single_logic_expr =
//	    [ not_oper ] ( ident_dotted | TYPE_OF boolean<ident_ref> )
//	  | ( numeric_expr | string_expr | compare_expr );
//
// Disambiguation:
//   - If '!' is present → must be ident-based alternative (negated ref).
//   - If the identifier is followed by a compare_oper → compare_expr wins.
//   - Otherwise a lone identifier is treated as ident_dotted (boolean ref).
//   - Literals, strings, and grouped expressions use the value alternatives.
func (p *Parser) parseSingleLogicExpr() *Node {
	if !p.enter("single_logic_expr") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	n := NewNode(NodeSingleLogicExpr, pos)

	// --- Optional not_oper: '!' can only prefix an ident_dotted reference ---
	if p.match(TOK_BANG) {
		notOper := p.parseNotOper()
		n.Append(notOper)
		// Must be followed by ident_dotted or ident_ref.
		if ref := p.try(p.parseIdentDotted); ref != nil {
			n.Append(ref)
			return n
		}
		if ref := p.parseIdentRef(); ref != nil {
			wrapped := NewDirectiveNode(NodeDirectiveTypeOf, "boolean", pos, ref)
			n.Append(wrapped)
			return n
		}
		p.errorf("expected identifier after '!' in logic expression")
		return nil
	}

	// --- Identifier at current position: disambiguate by what follows ---
	if p.match(TOK_IDENT) {
		// Peek past the ident (possibly dotted) to see if a compare_oper follows;
		// if so parse it as compare_expr.  This avoids consuming the ident into
		// ident_dotted when it's actually the LHS of a numeric comparison.
		if expr := p.try(p.parseCompareExpr); expr != nil {
			n.Append(expr)
			return n
		}
		// No compare_oper: treat as a boolean identifier reference.
		if ref := p.parseIdentDotted(); ref != nil {
			n.Append(ref)
			return n
		}
		return nil
	}

	// --- Parenthesised groupings and literal values ---
	if p.match(TOK_LPAREN) {
		if expr := p.try(p.parseCompareExpr); expr != nil {
			n.Append(expr)
			return n
		}
		if expr := p.try(p.parseNumericExpr); expr != nil {
			n.Append(expr)
			return n
		}
		if expr := p.try(p.parseStringExpr); expr != nil {
			n.Append(expr)
			return n
		}
		return nil
	}

	// --- String literals: regexp or string_expr or string compare ---
	if p.match(TOK_STRING) {
		if expr := p.try(p.parseStringRegexpComp); expr != nil {
			n.Append(expr)
			return n
		}
		if expr := p.try(p.parseStringCompare); expr != nil {
			n.Append(expr)
			return n
		}
		if expr := p.parseStringExpr(); expr != nil {
			n.Append(expr)
			return n
		}
		return nil
	}

	// --- Numeric literals: num_compare or numeric_expr ---
	if isConstantFirst(p.peek().Type) {
		if expr := p.try(p.parseNumCompare); expr != nil {
			n.Append(expr)
			return n
		}
		if expr := p.parseNumericExpr(); expr != nil {
			n.Append(expr)
			return n
		}
		return nil
	}

	return nil
}

// parseLogicExprList parses a logic expression list.
//   logic_expr_list = single_logic_expr { logic_oper single_logic_expr };
func (p *Parser) parseLogicExprList() *Node {
	if !p.enter("logic_expr_list") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	first := p.parseSingleLogicExpr()
	if first == nil {
		return nil
	}
	pos := first.Pos
	n := NewNode(NodeLogicExprList, pos, first)
	for p.matchAny(TOK_AMP, TOK_PIPE, TOK_CARET) {
		c := p.save()
		oper := p.parseLogicOper()
		next := p.parseSingleLogicExpr()
		if next == nil {
			p.restore(c)
			break
		}
		n.Append(oper, next)
	}
	return n
}

// parseLogicGroupingItem parses the inner alternatives of a logic grouping.
func (p *Parser) parseLogicGroupingItem() *Node {
	if p.match(TOK_LPAREN) {
		return p.parseLogicGrouping()
	}
	return p.parseLogicExprList()
}

// parseLogicGrouping parses a parenthesised logic expression.
//   logic_grouping = "(" ( logic_expr_list | logic_grouping )
//                       { logic_oper ( logic_expr_list | logic_grouping ) } ")";
func (p *Parser) parseLogicGrouping() *Node {
	if !p.enter("logic_grouping") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()

	if !p.match(TOK_LPAREN) {
		return nil
	}
	pos := p.curPos()
	p.advance() // consume "("

	first := p.parseLogicGroupingItem()
	if first == nil {
		p.errorf("expected logic expression inside '('")
		return nil
	}
	n := NewNode(NodeLogicGrouping, pos, first)
	for p.matchAny(TOK_AMP, TOK_PIPE, TOK_CARET) {
		c := p.save()
		oper := p.parseLogicOper()
		item := p.parseLogicGroupingItem()
		if item == nil {
			p.restore(c)
			break
		}
		n.Append(oper, item)
	}
	p.consume(TOK_RPAREN)
	return n
}

// parseLogicExpr parses a complete logic expression.
//   logic_expr = logic_expr_list | logic_grouping;
func (p *Parser) parseLogicExpr() *Node {
	if !p.enter("logic_expr") {
		return NewErrorNode("max depth", p.curPos())
	}
	defer p.leave()
	pos := p.curPos()
	var child *Node
	if p.match(TOK_LPAREN) {
		child = p.parseLogicGrouping()
	} else {
		child = p.parseLogicExprList()
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeLogicExpr, pos, child)
}

// =============================================================================
// Assignment operators
// =============================================================================

// parseIncrAssignImmutable parses an incremental-immutable assignment operator.
//   incr_assign_immutable = "+:" | "-:" | "*:" | "/:";
func (p *Parser) parseIncrAssignImmutable() *Node {
	if !p.matchAny(TOK_IADD_IMM, TOK_ISUB_IMM, TOK_IMUL_IMM, TOK_IDIV_IMM) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIncrAssignImmutable,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseIncrAssignMutable parses an incremental-mutable assignment operator.
//   incr_assign_mutable = "+~" | "-~" | "*~" | "/~";
func (p *Parser) parseIncrAssignMutable() *Node {
	if !p.matchAny(TOK_IADD_MUT, TOK_ISUB_MUT, TOK_IMUL_MUT, TOK_IDIV_MUT) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeIncrAssignMutable,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseAssignMutable parses the mutable assignment operator "~".
//   assign_mutable = "~";
func (p *Parser) parseAssignMutable() *Node {
	if !p.match(TOK_TILDE) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeAssignMutable,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseAssignImmutable parses the immutable assignment operator ":".
//   assign_immutable = ":";
func (p *Parser) parseAssignImmutable() *Node {
	if !p.match(TOK_COLON) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeAssignImmutable,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseAssignReadOnlyRef parses the read-only reference operator ":~".
//   assign_read_only_ref = ":~";
func (p *Parser) parseAssignReadOnlyRef() *Node {
	if !p.match(TOK_READONLY) {
		return nil
	}
	tok := p.advance()
	return NewNode(NodeAssignReadOnlyRef,
		Pos{Line: tok.Line, Col: tok.Col}, NewTokenNode(tok))
}

// parseEqualAssign parses any of the "equals-style" assignment operators.
//   equal_assign = assign_mutable | assign_immutable | assign_read_only_ref;
// TOK_READONLY (":~") must be tried before TOK_COLON (":") so that max-munch
// is respected at the grammar level (the lexer already handles this).
func (p *Parser) parseEqualAssign() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_READONLY:
		child = p.parseAssignReadOnlyRef()
	case TOK_TILDE:
		child = p.parseAssignMutable()
	case TOK_COLON:
		child = p.parseAssignImmutable()
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeEqualAssign, pos, child)
}

// parseAssignOper parses any assignment operator.
//   assign_oper = incr_assign_immutable | incr_assign_mutable | equal_assign;
func (p *Parser) parseAssignOper() *Node {
	pos := p.curPos()
	var child *Node
	switch p.peek().Type {
	case TOK_IADD_IMM, TOK_ISUB_IMM, TOK_IMUL_IMM, TOK_IDIV_IMM:
		child = p.parseIncrAssignImmutable()
	case TOK_IADD_MUT, TOK_ISUB_MUT, TOK_IMUL_MUT, TOK_IDIV_MUT:
		child = p.parseIncrAssignMutable()
	case TOK_READONLY, TOK_TILDE, TOK_COLON:
		child = p.parseEqualAssign()
	default:
		return nil
	}
	if child == nil {
		return nil
	}
	return NewNode(NodeAssignOper, pos, child)
}

// =============================================================================
// Range
// =============================================================================

// parseRange parses a range expression.
//   range = digits ".." ( digits | "m" | "M" | "many" | "Many" );
func (p *Parser) parseRange() *Node {
	// Pre-check: we need INTEGER ".." before committing to consume anything.
	if !p.match(TOK_INTEGER) || p.peek2().Type != TOK_DOTDOT {
		return nil
	}
	pos := p.curPos()
	lo := p.parseDigits()
	if !p.match(TOK_DOTDOT) {
		return nil
	}
	dotdotTok := p.advance() // ".."

	var hi *Node
	switch p.peek().Type {
	case TOK_INTEGER:
		hi = p.parseDigits()
	case TOK_MANY:
		hi = NewTokenNode(p.advance())
	case TOK_IDENT:
		v := p.peek().Value
		if v != "m" && v != "M" {
			p.errorf("expected upper bound in range (digits, 'm', 'M', 'many', 'Many'), got %q", v)
			return nil
		}
		hi = NewTokenNode(p.advance())
	default:
		p.errorf("expected upper bound in range, got %s", p.peek().Type)
		return nil
	}
	return NewNode(NodeRange, pos, lo, NewTokenNode(dotdotTok), hi)
}
