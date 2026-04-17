// parser_v13_template.go — Semantic helpers for Squeeze V13 template strings.
//
// Covered spec section: 14 (spec/14_templates.sqg)
//
// Two exported functions are provided:
//
//	V13CheckTmplScope    — mode-1 / mode-2 scope validation
//	V13ValidateTmplCall  — mode-3 call-site argument validation
//
// Neither function depends on a runtime symbol table; callers supply the
// relevant information as plain Go values.  This keeps the helpers usable
// from both the parser pipeline and any future semantic-analysis pass.
package parser

import "fmt"

// --------------------------------------------------------------------------
// Step 6 — Scope checker (mode 1 / mode 2)
// --------------------------------------------------------------------------

// V13TmplScopeError is the error type returned by V13CheckTmplScope.
type V13TmplScopeError struct {
	// MissingNames lists every §(expr) slot whose top-level name was not
	// found in the provided scope set.
	MissingNames []string
}

func (e *V13TmplScopeError) Error() string {
	return fmt.Sprintf("template scope error: identifiers not in scope: %v", e.MissingNames)
}

// V13CheckTmplScope validates a mode-1 or mode-2 template node against a set
// of names that are known to be in scope at the point of use.
//
// For each IsExpr part, the "top-level name" is the first dot-separated
// component of the expression text (e.g. "user" from "user.first_name").
// This matches the minimal parse-time check described in spec 14.7.
//
// Returns nil if all names are in scope, or *V13TmplScopeError otherwise.
func V13CheckTmplScope(node *V13StringNode, inScope map[string]bool) error {
	if node == nil {
		return nil
	}
	var missing []string
	for _, part := range node.Parts {
		if !part.IsExpr {
			continue
		}
		// Extract top-level name: take the text up to the first '(', '.', ' ', or end.
		name := v13tmplTopLevelName(part.Text)
		if name != "" && !inScope[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return &V13TmplScopeError{MissingNames: missing}
	}
	return nil
}

// v13tmplTopLevelName extracts the leading identifier from an expression text.
// "user.first_name" → "user"
// "count * 2"       → "count"
// "greet(\"hi\")"   → "greet"
func v13tmplTopLevelName(expr string) string {
	for i, ch := range expr {
		if ch == '.' || ch == '(' || ch == ' ' || ch == '\t' || ch == '*' ||
			ch == '+' || ch == '-' || ch == '/' || ch == '>' || ch == '<' {
			return expr[:i]
		}
	}
	return expr
}

// --------------------------------------------------------------------------
// Step 7 — Call-site validator (mode 3)
// --------------------------------------------------------------------------

// V13TmplCallError is the error type returned by V13ValidateTmplCall.
type V13TmplCallError struct {
	Message string
}

func (e *V13TmplCallError) Error() string { return e.Message }

// V13ValidateTmplCall checks that a mode-3 template call is well-formed.
//
// argTypes is the list of type strings supplied by the caller, in order.
// paramTypes is the expected type for each positional slot; an empty string
// means "any / unconstrained" (the parser has not inferred a type yet).
//
// Rules (spec 14.5):
//   - argument count must equal len(node.Params)
//   - for each slot where paramTypes[i] != "" and argTypes[i] != "",
//     the types must match (case-insensitive)
//
// Returns nil on success, or *V13TmplCallError describing the first violation.
func V13ValidateTmplCall(node *V13TmplDeferredNode, argTypes []string, paramTypes []string) error {
	if node == nil {
		return nil
	}
	want := len(node.Params)
	got := len(argTypes)
	if got != want {
		return &V13TmplCallError{
			Message: fmt.Sprintf("template call: expected %d argument(s), got %d", want, got),
		}
	}
	for i, param := range node.Params {
		if i >= len(paramTypes) || paramTypes[i] == "" {
			continue
		}
		if i >= len(argTypes) || argTypes[i] == "" {
			continue
		}
		want := paramTypes[i]
		got := argTypes[i]
		if !v13typeCompat(want, got) {
			return &V13TmplCallError{
				Message: fmt.Sprintf(
					"template call: argument %d (%q) type mismatch: expected %s, got %s",
					i, param.Name, want, got,
				),
			}
		}
	}
	return nil
}

// v13typeCompat returns true when gotType is compatible with wantType.
// The comparison is case-insensitive and treats "string" as the only
// required output type (coercible sources are listed explicitly).
func v13typeCompat(want, got string) bool {
	if want == got {
		return true
	}
	// Case-insensitive match.
	if len(want) == len(got) {
		// manual toLower comparison to avoid importing "strings" at pkg init
		match := true
		for i := 0; i < len(want); i++ {
			wc, gc := want[i], got[i]
			if wc >= 'A' && wc <= 'Z' {
				wc += 32
			}
			if gc >= 'A' && gc <= 'Z' {
				gc += 32
			}
			if wc != gc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	// Numeric types are coercible to string for template output.
	if want == "string" {
		switch got {
		case "int", "integer", "float", "decimal", "bool", "boolean":
			return true
		}
	}
	return false
}
