// Package parser — V17 bootstrap-directive parse methods for spec/30_bootstrap.sqg.
//
// Grammar rules (SECTION 31 — 30_bootstrap.sqg):
//
//	__token          = any grammar rule name  (alternation over 29_grammar_tokens.sqg)
//	__ps_token_ref   = ps_prefix ( __token ) { "." __token }
//	                   where ps_prefix = "§"  (spec/02_operators.sqg)
//	__ps_obj_self    = __ps_token_ref ".$"
//	__ps_arg         = "LENGTH" | "LHS_CALLER" | "CAST_TO_STRING" | "INFER" | "REF_OF" | "LHS"
//	__ps_type_of     = __ps_token_ref !WS! ".@type"
//	__ps_cast        = "CAST" ( "string" | "integer" | "bit_array" | __ps_type_of )
//	__ps_call        = "__ps_call" [ __ps_arg | __ps_cast | __ps_type_of ] [ __ps_token_ref | ident_ref ]
//	__ps_proxy_range = "RANGE" range
//	__ps_proxy_args       = intrinsics | "UNIQUE" | "UNIFORM" | "VALUE_OF" | __ps_proxy_range | __ps_type_of
//	__ps_proxy_extra_args = [ reflect_prefix ] ident_ref { "," [ reflect_prefix ] ident_ref }
//	__ps_proxy            = "__ps_proxy" __ps_proxy_args [ __ps_proxy_extra_args ] [ func_call_args ] [ "<" func_call_args ">" ]
//	EXTEND<ident_ref>     = | __ps_token_ref
//	EXTEND<statement>     = | __ps_call | __ps_proxy
package parser

import (
	"fmt"
	"regexp"
)

// reGrammarTokenScan matches a grammar token name identifier.
// Token names are either plain identifiers ("any_type", "assign_version") or
// "__"-prefixed names ("__ps_call", "__ps_token_ref").
var reGrammarTokenScan = regexp.MustCompile(`^_{0,2}[\p{L}][\p{L}0-9_]*`)

// reIntrinsicScan matches an all-uppercase intrinsic name, e.g. "HMAP_PUT", "NOW_ISO8601".
var reIntrinsicScan = regexp.MustCompile(`^[A-Z][A-Z0-9_]*`)

// intrinsicExcludes lists uppercase keywords that appear as named alternatives in
// __ps_proxy_args / __ps_proxy_range and must NOT be consumed by ParseIntrinsics.
var intrinsicExcludes = map[string]bool{
	"UNIQUE":   true,
	"UNIFORM":  true,
	"VALUE_OF": true,
	"RANGE":    true,
}

// =============================================================================
// __token  — any grammar rule name (29_grammar_tokens.sqg alternation)
// =============================================================================

// V17GrammarTokenNode  __token = any grammar rule name
type V17GrammarTokenNode struct {
	V17BaseNode
	Name string
}

// ParseToken parses __token: any grammar rule name identifier (optionally "__"-prefixed).
func (p *V17Parser) ParseToken() (node *V17GrammarTokenNode, err error) {
	done := p.debugEnter("__token")
	defer func() { done(err == nil) }()
	tok, terr := p.matchRe(reGrammarTokenScan)
	if terr != nil {
		return nil, p.errAt("__token: expected grammar token name")
	}
	return &V17GrammarTokenNode{V17BaseNode{tok.Line, tok.Col}, tok.Value}, nil
}

// =============================================================================
// __ps_token_ref = ps_prefix ( __token ) { "." __token }
// ps_prefix = "§"  (spec/02_operators.sqg line 12)
// =============================================================================

// V17PsTokenRefNode  __ps_token_ref = "§" __token { "." __token }
//
// Segments holds the dot-separated token name path, e.g.:
//
//	"§assign_version.digits" → Segments = ["assign_version", "digits"]
//	"§array_final"           → Segments = ["array_final"]
type V17PsTokenRefNode struct {
	V17BaseNode
	Segments []string
}

// ParsePsTokenRef parses __ps_token_ref = "§" __token { "." __token }.
func (p *V17Parser) ParsePsTokenRef() (node *V17PsTokenRefNode, err error) {
	done := p.debugEnter("__ps_token_ref")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.matchLit("§"); err != nil {
		return nil, err
	}
	first, ferr := p.ParseToken()
	if ferr != nil {
		return nil, fmt.Errorf("__ps_token_ref: %w", ferr)
	}
	segments := []string{first.Name}
	// Optional dotted continuation: { "." __token }
	for p.peekAfterWS() == '.' {
		saved := p.savePos()
		if _, merr := p.matchLit("."); merr != nil {
			p.restorePos(saved)
			break
		}
		seg, serr := p.ParseToken()
		if serr != nil {
			p.restorePos(saved)
			break
		}
		segments = append(segments, seg.Name)
	}
	return &V17PsTokenRefNode{V17BaseNode{line, col}, segments}, nil
}

// =============================================================================
// __ps_obj_self = __ps_token_ref ".$"
// =============================================================================

// V17PsObjSelfNode  __ps_obj_self = __ps_token_ref ".$"
type V17PsObjSelfNode struct {
	V17BaseNode
	TokenRef *V17PsTokenRefNode
}

// ParsePsObjSelf parses __ps_obj_self = __ps_token_ref ".$".
func (p *V17Parser) ParsePsObjSelf() (node *V17PsObjSelfNode, err error) {
	done := p.debugEnter("__ps_obj_self")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	ref, rerr := p.ParsePsTokenRef()
	if rerr != nil {
		return nil, rerr
	}
	if _, err = p.matchLit(".$"); err != nil {
		return nil, fmt.Errorf("__ps_obj_self: expected '.$': %w", err)
	}
	return &V17PsObjSelfNode{V17BaseNode{line, col}, ref}, nil
}

// =============================================================================
// __ps_arg = "LENGTH" | "LHS_CALLER" | "CAST_TO_STRING" | "INFER" | "REF_OF" | "LHS"
// =============================================================================

// V17PsArgNode  __ps_arg = "LENGTH" | "LHS_CALLER" | "CAST_TO_STRING" | "INFER" | "REF_OF" | "LHS"
type V17PsArgNode struct {
	V17BaseNode
	Keyword string
}

// ParsePsArg parses __ps_arg.
// Alternatives are tried longest-first to avoid "LHS" prematurely matching "LHS_CALLER".
func (p *V17Parser) ParsePsArg() (node *V17PsArgNode, err error) {
	done := p.debugEnter("__ps_arg")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	for _, kw := range []string{"CAST_TO_STRING", "LHS_CALLER", "REF_OF", "LENGTH", "INFER", "LHS"} {
		if p.peekLit(kw) {
			if tok, kerr := p.matchKeyword(kw); kerr == nil {
				return &V17PsArgNode{V17BaseNode{line, col}, tok.Value}, nil
			}
		}
	}
	return nil, p.errAt("__ps_arg: expected LENGTH, LHS_CALLER, CAST_TO_STRING, INFER, REF_OF or LHS")
}

// =============================================================================
// __ps_type_of = __ps_token_ref !WS! "." !WS! reflect_prefix !WS! "type"
// reflect_prefix = "@"
// =============================================================================

// V17PsTypeOfNode  __ps_type_of = __ps_token_ref !WS! ".@type"
type V17PsTypeOfNode struct {
	V17BaseNode
	TokenRef *V17PsTokenRefNode
}

// ParsePsTypeOf parses __ps_type_of = __ps_token_ref !WS! ".@type".
func (p *V17Parser) ParsePsTypeOf() (node *V17PsTypeOfNode, err error) {
	done := p.debugEnter("__ps_type_of")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	saved := p.savePos()
	ref, rerr := p.ParsePsTokenRef()
	if rerr != nil {
		p.restorePos(saved)
		return nil, fmt.Errorf("__ps_type_of: %w", rerr)
	}
	// !WS! — "." must be immediately adjacent (no whitespace)
	if p.runePos >= len(p.input) || p.input[p.runePos] != '.' {
		p.restorePos(saved)
		return nil, p.errAt("__ps_type_of: expected '.@type' immediately after token_ref")
	}
	if _, err = p.matchLit("."); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if p.runePos >= len(p.input) || p.input[p.runePos] != '@' {
		p.restorePos(saved)
		return nil, p.errAt("__ps_type_of: expected '@' immediately after '.'")
	}
	if _, err = p.matchLit("@"); err != nil {
		p.restorePos(saved)
		return nil, err
	}
	if _, err = p.matchKeyword("type"); err != nil {
		p.restorePos(saved)
		return nil, fmt.Errorf("__ps_type_of: expected 'type' after '.@': %w", err)
	}
	return &V17PsTypeOfNode{V17BaseNode{line, col}, ref}, nil
}

// =============================================================================
// __ps_cast = "CAST" ( "string" | "integer" | "bit_array" | __ps_type_of )
// =============================================================================

// V17PsCastNode  __ps_cast = "CAST" ( "string" | "integer" | "bit_array" | __ps_type_of )
type V17PsCastNode struct {
	V17BaseNode
	// TargetType is non-empty when one of the primitive type keywords is matched.
	TargetType string
	// TypeOf is non-nil when the __ps_type_of alternative is matched.
	TypeOf *V17PsTypeOfNode
}

// ParsePsCast parses __ps_cast = "CAST" ( "string" | "integer" | "bit_array" | __ps_type_of ).
func (p *V17Parser) ParsePsCast() (node *V17PsCastNode, err error) {
	done := p.debugEnter("__ps_cast")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.matchKeyword("CAST"); err != nil {
		return nil, err
	}
	// Primitive type keywords — tried longest-first to prevent sub-matches.
	for _, kw := range []string{"bit_array", "integer", "string"} {
		if p.peekLit(kw) {
			if _, kerr := p.matchKeyword(kw); kerr == nil {
				return &V17PsCastNode{V17BaseNode{line, col}, kw, nil}, nil
			}
		}
	}
	// __ps_type_of starts with "@".
	typeOf, tErr := p.ParsePsTypeOf()
	if tErr != nil {
		return nil, fmt.Errorf("__ps_cast: expected string, integer, bit_array or __ps_type_of: %w", tErr)
	}
	return &V17PsCastNode{V17BaseNode{line, col}, "", typeOf}, nil
}

// =============================================================================
// ps_call = "ps_call" [ ps_arg | ps_cast | ps_type_of ] [ ident_ref | ps_token_ref ]
// =============================================================================

// V17PsCallNode  __ps_call = "__ps_call" [ __ps_arg | __ps_cast | __ps_type_of ] [ __ps_token_ref | ident_ref ]
type V17PsCallNode struct {
	V17BaseNode
	// Arg is one of: nil | *V17PsArgNode | *V17PsCastNode | *V17PsTypeOfNode
	Arg interface{}
	// Ref is one of: nil | *V17PsTokenRefNode | *V17IdentRefNode
	Ref interface{}
}

// ParsePsCall parses __ps_call = "__ps_call" [ __ps_arg | __ps_cast | __ps_type_of ] [ __ps_token_ref | ident_ref ].
func (p *V17Parser) ParsePsCall() (node *V17PsCallNode, err error) {
	done := p.debugEnter("__ps_call")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.matchKeyword("__ps_call"); err != nil {
		return nil, err
	}

	// Optional: [ __ps_arg | __ps_cast | __ps_type_of ]
	// Discriminated by leading literal: "CAST" → __ps_cast; "@" → __ps_type_of; else → __ps_arg.
	var arg interface{}
	if p.peekLit("CAST") {
		if saved := p.savePos(); true {
			if ca, caerr := p.ParsePsCast(); caerr == nil {
				arg = ca
			} else {
				p.restorePos(saved)
			}
		}
	} else if p.peekLit("@") {
		if saved := p.savePos(); true {
			if to, toerr := p.ParsePsTypeOf(); toerr == nil {
				arg = to
			} else {
				p.restorePos(saved)
			}
		}
	} else {
		if saved := p.savePos(); true {
			if pa, paerr := p.ParsePsArg(); paerr == nil {
				arg = pa
			} else {
				p.restorePos(saved)
			}
		}
	}

	// Optional: [ __ps_token_ref | ident_ref ]
	// __ps_token_ref starts with "§"; ident_ref starts with a letter or prefix.
	var ref interface{}
	if p.peekLit("§") {
		if saved := p.savePos(); true {
			if tr, trerr := p.ParsePsTokenRef(); trerr == nil {
				ref = tr
			} else {
				p.restorePos(saved)
			}
		}
	} else {
		if saved := p.savePos(); true {
			if ir, irerr := p.ParseIdentRef(); irerr == nil {
				ref = ir
			} else {
				p.restorePos(saved)
			}
		}
	}

	return &V17PsCallNode{V17BaseNode{line, col}, arg, ref}, nil
}

// =============================================================================
// =============================================================================
// intrinsics  (28_intrinsics.sqg)
// Any all-uppercase identifier that is not one of the reserved keywords
// (UNIQUE, UNIFORM, VALUE_OF, RANGE).  Validation of the exact name against
// the canonical list is deferred to the checker.
// =============================================================================

// V17IntrinsicsNode  intrinsics = any uppercase intrinsic name
type V17IntrinsicsNode struct {
	V17BaseNode
	Name string
}

// ParseIntrinsics parses an intrinsic keyword: an all-uppercase identifier
// (A-Z, 0-9, underscore) that is not one of the reserved proxy-arg keywords.
func (p *V17Parser) ParseIntrinsics() (node *V17IntrinsicsNode, err error) {
	done := p.debugEnter("intrinsics")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	saved := p.savePos()
	tok, terr := p.matchRe(reIntrinsicScan)
	if terr != nil {
		p.restorePos(saved)
		return nil, p.errAt("intrinsics: expected uppercase intrinsic name")
	}
	if intrinsicExcludes[tok.Value] {
		p.restorePos(saved)
		return nil, p.errAt("intrinsics: " + tok.Value + " is a reserved keyword, not an intrinsic")
	}
	return &V17IntrinsicsNode{V17BaseNode{line, col}, tok.Value}, nil
}

// =============================================================================
// __ps_proxy_range = "RANGE" range
// =============================================================================

// V17PsProxyRangeNode  __ps_proxy_range = "RANGE" range
type V17PsProxyRangeNode struct {
	V17BaseNode
	Range *V17RangeNode
}

// ParsePsProxyRange parses __ps_proxy_range = "RANGE" range.
func (p *V17Parser) ParsePsProxyRange() (node *V17PsProxyRangeNode, err error) {
	done := p.debugEnter("__ps_proxy_range")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.matchKeyword("RANGE"); err != nil {
		return nil, err
	}
	r, rerr := p.ParseRange()
	if rerr != nil {
		return nil, fmt.Errorf("__ps_proxy_range: %w", rerr)
	}
	return &V17PsProxyRangeNode{V17BaseNode{line, col}, r}, nil
}

// =============================================================================
// __ps_proxy_args = intrinsics | "UNIQUE" | "UNIFORM" | "VALUE_OF" | __ps_proxy_range | __ps_type_of
// =============================================================================

// V17PsProxyArgsNode  __ps_proxy_args = intrinsics | "UNIQUE" | "UNIFORM" | "VALUE_OF" | __ps_proxy_range | __ps_type_of
type V17PsProxyArgsNode struct {
	V17BaseNode
	// Value is one of: *V17IntrinsicsNode | string ("UNIQUE"|"UNIFORM"|"VALUE_OF") | *V17PsProxyRangeNode | *V17PsTypeOfNode
	Value interface{}
}

// ParsePsProxyArgs parses __ps_proxy_args.
func (p *V17Parser) ParsePsProxyArgs() (node *V17PsProxyArgsNode, err error) {
	done := p.debugEnter("__ps_proxy_args")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	// intrinsics: all-uppercase identifier (first alternative per grammar)
	saved := p.savePos()
	if intr, intrerr := p.ParseIntrinsics(); intrerr == nil {
		return &V17PsProxyArgsNode{V17BaseNode{line, col}, intr}, nil
	}
	p.restorePos(saved)

	// "UNIQUE" | "UNIFORM" | "VALUE_OF"
	for _, kw := range []string{"UNIFORM", "UNIQUE", "VALUE_OF"} {
		if p.peekLit(kw) {
			if tok, kerr := p.matchKeyword(kw); kerr == nil {
				return &V17PsProxyArgsNode{V17BaseNode{line, col}, tok.Value}, nil
			}
		}
	}
	if p.peekLit("RANGE") {
		if saved2 := p.savePos(); true {
			if pr, prerr := p.ParsePsProxyRange(); prerr == nil {
				return &V17PsProxyArgsNode{V17BaseNode{line, col}, pr}, nil
			}
			p.restorePos(saved2)
		}
	}
	if p.peekLit("@") {
		if saved2 := p.savePos(); true {
			if to, toerr := p.ParsePsTypeOf(); toerr == nil {
				return &V17PsProxyArgsNode{V17BaseNode{line, col}, to}, nil
			}
			p.restorePos(saved2)
		}
	}
	return nil, p.errAt("__ps_proxy_args: expected intrinsic name, UNIQUE, UNIFORM, VALUE_OF, __ps_proxy_range or __ps_type_of")
}

// =============================================================================
// __ps_proxy_extra_args = [ type_prefix ] ident_ref { "," [ type_prefix ] ident_ref }
// type_prefix = "@"
// =============================================================================

// V17PsProxyExtraArgItem is one element of __ps_proxy_extra_args.
// Expr is one of: *V17PsTypeOfNode | *V17TypeRefNode | *V17IdentRefNode
type V17PsProxyExtraArgItem struct {
	Expr interface{}
}

// V17PsProxyExtraArgsNode  __ps_proxy_extra_args = ( __ps_type_of | ident_type_of | ident_ref ) { "," [ reflect_prefix ] ( __ps_type_of | ident_type_of | ident_ref ) }
type V17PsProxyExtraArgsNode struct {
	V17BaseNode
	Items []V17PsProxyExtraArgItem
}

// parsePsProxyExtraArgOne parses one extra-arg item:
// __ps_type_of | ident_type_of (via ParseTypeRef) | ident_ref
func (p *V17Parser) parsePsProxyExtraArgOne() (interface{}, error) {
	// Try __ps_type_of first (§token.@type form)
	if saved := p.savePos(); true {
		if pto, ptoerr := p.ParsePsTypeOf(); ptoerr == nil {
			return pto, nil
		}
		p.restorePos(saved)
	}
	// Try ident_type_of = ident_ref.@type (handled by ParseTypeRef)
	if saved := p.savePos(); true {
		if tr, trerr := p.ParseTypeRef(); trerr == nil {
			return tr, nil
		}
		p.restorePos(saved)
	}
	// Fall back to plain ident_ref
	ir, irerr := p.ParseIdentRef()
	if irerr != nil {
		return nil, p.errAt("__ps_proxy_extra_args: expected __ps_type_of, ident_type_of, or ident_ref")
	}
	return ir, nil
}

// ParsePsProxyExtraArgs parses __ps_proxy_extra_args.
// Only called when the next visible token is NOT "<", so angle-args are never consumed here.
func (p *V17Parser) ParsePsProxyExtraArgs() (node *V17PsProxyExtraArgsNode, err error) {
	done := p.debugEnter("__ps_proxy_extra_args")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	saved0 := p.savePos()
	first, ferr := p.parsePsProxyExtraArgOne()
	if ferr != nil {
		p.restorePos(saved0)
		return nil, ferr
	}
	items := []V17PsProxyExtraArgItem{{first}}

	// Optional continuation: { "," [ reflect_prefix ] ( __ps_type_of | ident_type_of | ident_ref ) }
	for {
		saved := p.savePos()
		if _, cerr := p.matchLit(","); cerr != nil {
			p.restorePos(saved)
			break
		}
		// consume optional bare "@" prefix (grammar allows [ reflect_prefix ] in repetition)
		if p.peekLit("@") {
			p.matchLit("@") //nolint — optional, ignore error
		}
		next, nerr := p.parsePsProxyExtraArgOne()
		if nerr != nil {
			p.restorePos(saved)
			break
		}
		items = append(items, V17PsProxyExtraArgItem{next})
	}
	return &V17PsProxyExtraArgsNode{V17BaseNode{line, col}, items}, nil
}

// =============================================================================
// __ps_proxy = "__ps_proxy" __ps_proxy_args [ __ps_proxy_extra_args ] [ func_call_args ] [ "<" func_call_args ">" ]
// =============================================================================

// V17PsProxyNode  __ps_proxy = "__ps_proxy" __ps_proxy_args [ __ps_proxy_extra_args ] [ "<" func_call_args ">" ]
type V17PsProxyNode struct {
	V17BaseNode
	Verb      *V17PsProxyArgsNode      // required: the intrinsic / special keyword
	ExtraArgs *V17PsProxyExtraArgsNode // nil when omitted (e.g. "string" in CAST string <x>)
	AngleArgs *V17FuncCallArgsNode     // nil when omitted (args inside "<" ">")
}

// ParsePsProxy parses __ps_proxy = "__ps_proxy" __ps_proxy_args [ __ps_proxy_extra_args ] [ "<" func_call_args ">" ].
func (p *V17Parser) ParsePsProxy() (node *V17PsProxyNode, err error) {
	done := p.debugEnter("__ps_proxy")
	defer func() { done(err == nil) }()
	line, col := p.runeLine, p.runeCol

	if _, err = p.matchKeyword("__ps_proxy"); err != nil {
		return nil, err
	}

	// __ps_proxy_args (required)
	verb, verberr := p.ParsePsProxyArgs()
	if verberr != nil {
		return nil, fmt.Errorf("__ps_proxy: %w", verberr)
	}

	// Optional: [ __ps_proxy_extra_args ]
	// Only attempted when next visible token is NOT "<" (angle-arg opener).
	var extraArgs *V17PsProxyExtraArgsNode
	if !p.peekLit("<") {
		saved := p.savePos()
		if ea, eaerr := p.ParsePsProxyExtraArgs(); eaerr == nil {
			extraArgs = ea
		} else {
			p.restorePos(saved)
		}
	}

	// Optional: [ "<" func_call_args ">" ]
	var angleArgs *V17FuncCallArgsNode
	if p.peekLit("<") {
		saved := p.savePos()
		if _, lerr := p.matchLit("<"); lerr == nil {
			if aa, aaerr := p.ParseFuncCallArgs(); aaerr == nil {
				if _, rerr := p.matchLit(">"); rerr == nil {
					angleArgs = aa
				} else {
					p.restorePos(saved)
				}
			} else {
				p.restorePos(saved)
			}
		} else {
			p.restorePos(saved)
		}
	}

	return &V17PsProxyNode{V17BaseNode{line, col}, verb, extraArgs, angleArgs}, nil
}
