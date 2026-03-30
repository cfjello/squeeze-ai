// directive_processor.go — Phase 3: post-parse directive processing.
//
// After the recursive-descent parser has produced a raw AST (Phase 2), the
// DirectiveProcessor walks the tree and acts on every NodeDirective* node:
//
//	UNIQUE      — collect all leaf ident_name values; error on duplicates
//	RANGE n..m  — check enclosed integer/decimal is within the bounds n..m
//	TYPE_OF t   — annotate the enclosed node with the expected type token t
//	VALUE_OF    — mark enclosed node with IsValueOf = true
//	ADDRESS_OF  — mark enclosed node with IsAddressOf = true
//	RETURN      — mark with value-of (":") or address-of ("~") semantics
//	UNIFORM     — annotate with the uniform type arg; homogeneity checked here
//	INFER       — attach inferred type from symbol table or first-element type
//
// The processor is purely additive: it annotates Meta fields and appends
// to the errors slice. It never restructures the tree.
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// =============================================================================
// Symbol table (lightweight, string-keyed type lookup)
// =============================================================================

// SymbolTable maps identifier names (as written in assign_lhs) to their
// inferred or declared type string.  It is populated by the processor as it
// encounters assignment nodes and queried when INFER is processed.
type SymbolTable map[string]string

// =============================================================================
// DirectiveProcessor
// =============================================================================

// DirectiveProcessor holds state for the post-parse tree walk.
type DirectiveProcessor struct {
	symbols SymbolTable
	errors  []ParseError
}

// NewDirectiveProcessor returns a ready-to-use processor with an empty
// symbol table.
func NewDirectiveProcessor() *DirectiveProcessor {
	return &DirectiveProcessor{
		symbols: make(SymbolTable),
	}
}

// Errors returns all errors accumulated during processing.
func (dp *DirectiveProcessor) Errors() []ParseError { return dp.errors }

// HasErrors reports whether any errors were accumulated.
func (dp *DirectiveProcessor) HasErrors() bool { return len(dp.errors) > 0 }

func (dp *DirectiveProcessor) errorf(pos Pos, format string, args ...any) {
	dp.errors = append(dp.errors, ParseError{
		Pos: pos,
		Msg: fmt.Sprintf(format, args...),
	})
}

// =============================================================================
// Process — main entry point
// =============================================================================

// Process walks the AST rooted at root and applies directive semantics to
// every NodeDirective* node found.  The tree is not restructured.
func (dp *DirectiveProcessor) Process(root *Node) {
	if root == nil {
		return
	}
	root.Walk(func(n *Node) bool {
		switch n.Kind {
		case NodeDirectiveUnique:
			dp.processUnique(n)
		case NodeDirectiveRange:
			dp.processRange(n)
		case NodeDirectiveTypeOf:
			dp.processTypeOf(n)
		case NodeDirectiveValueOf:
			dp.processValueOf(n)
		case NodeDirectiveAddressOf:
			dp.processAddressOf(n)
		case NodeDirectiveReturn:
			dp.processReturn(n)
		case NodeDirectiveUniform:
			dp.processUniform(n)
		case NodeDirectiveInfer:
			dp.processInfer(n)
		case NodeAssignment:
			// Populate symbol table from completed assignments.
			dp.recordAssignment(n)
		}
		return true
	})
}

// =============================================================================
// UNIQUE
// =============================================================================

// processUnique checks that every ident_name inside the directive wrapper
// appears at most once.  NodeIdentName nodes are interior nodes whose first
// child is the NodeToken leaf carrying the actual value.
func (dp *DirectiveProcessor) processUnique(n *Node) {
	seen := make(map[string]Pos)
	n.Walk(func(node *Node) bool {
		if node.Kind == NodeIdentName {
			v := dp.identNameValue(node)
			if v == "" {
				return true
			}
			if prev, exists := seen[v]; exists {
				dp.errorf(node.Pos,
					"UNIQUE violation: identifier %q already seen at %s", v, prev)
			} else {
				seen[v] = node.Pos
			}
		}
		return true
	})
}

// =============================================================================
// RANGE n..m
// =============================================================================

// processRange checks that the enclosed numeric value is within [lo, hi].
// DirectiveArg is expected to be "n..m" where n and m are integer strings.
func (dp *DirectiveProcessor) processRange(n *Node) {
	arg := n.Meta.DirectiveArg
	parts := strings.SplitN(arg, "..", 2)
	if len(parts) != 2 {
		dp.errorf(n.Pos, "RANGE directive has malformed arg %q", arg)
		return
	}
	lo, err1 := strconv.ParseFloat(parts[0], 64)
	hi, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil {
		dp.errorf(n.Pos, "RANGE directive arg %q is not numeric", arg)
		return
	}
	// Find the first numeric leaf in the enclosed subtree.
	val, ok := dp.extractNumericValue(n)
	if !ok {
		// No concrete value to check yet — skip (value may be a reference).
		return
	}
	if val < lo || val > hi {
		dp.errorf(n.Pos, "RANGE violation: value %g is outside [%g,%g]", val, lo, hi)
	}
}

// extractNumericValue finds the first integer or decimal leaf in the subtree
// and returns its float64 value.
func (dp *DirectiveProcessor) extractNumericValue(n *Node) (float64, bool) {
	var found float64
	var ok bool
	n.Walk(func(node *Node) bool {
		if ok {
			return false
		}
		if node.IsLeaf() && (node.Tok.Type == TOK_INTEGER || node.Tok.Type == TOK_DECIMAL) {
			v, err := strconv.ParseFloat(node.Tok.Value, 64)
			if err == nil {
				found = v
				ok = true
			}
		}
		return !ok
	})
	return found, ok
}

// =============================================================================
// TYPE_OF
// =============================================================================

// processTypeOf annotates the enclosed child with the expected type string
// from DirectiveArg.  When the arg is "INFER" the actual type resolution is
// delegated to processInfer on the child node.
func (dp *DirectiveProcessor) processTypeOf(n *Node) {
	expectedType := n.Meta.DirectiveArg
	if len(n.Children) == 0 {
		return
	}
	child := n.Children[0]
	// Propagate the type expectation downward so the child can validate.
	if expectedType != "" && expectedType != "INFER" {
		if child.Meta.TypeRef == nil {
			child.Meta.TypeRef = &expectedType
		}
	}
}

// =============================================================================
// VALUE_OF
// =============================================================================

func (dp *DirectiveProcessor) processValueOf(n *Node) {
	if len(n.Children) == 0 {
		return
	}
	n.Children[0].Meta.IsValueOf = true
}

// =============================================================================
// ADDRESS_OF
// =============================================================================

func (dp *DirectiveProcessor) processAddressOf(n *Node) {
	if len(n.Children) == 0 {
		return
	}
	n.Children[0].Meta.IsAddressOf = true
}

// =============================================================================
// RETURN
// =============================================================================

// processReturn maps RETURN semantics onto the child: ":" means value-of,
// "~" means address-of.
func (dp *DirectiveProcessor) processReturn(n *Node) {
	if len(n.Children) == 0 {
		return
	}
	child := n.Children[0]
	switch n.Meta.DirectiveArg {
	case ":":
		child.Meta.IsValueOf = true
	case "~":
		child.Meta.IsAddressOf = true
	}
}

// =============================================================================
// UNIFORM
// =============================================================================

// processUniform verifies that all array-element children of the enclosed
// node share the same token type.  The expected type is either the
// DirectiveArg (e.g. "string") or inferred from the first element when the
// arg is "INFER" or empty.
func (dp *DirectiveProcessor) processUniform(n *Node) {
	if len(n.Children) == 0 {
		return
	}
	// Collect the immediate leaf types inside the enclosed subtree.
	// We walk only one level deep (the array element nodes).
	enclosed := n.Children[0]
	leafTypes := dp.collectLeafTokenTypes(enclosed)
	if len(leafTypes) == 0 {
		return
	}
	baseType := leafTypes[0]
	// When the directive arg names a concrete type, use that as the expectation.
	if n.Meta.DirectiveArg != "" && n.Meta.DirectiveArg != "INFER" {
		baseType = n.Meta.DirectiveArg
	}
	for _, lt := range leafTypes[1:] {
		if lt != baseType {
			dp.errorf(n.Pos,
				"UNIFORM violation: expected all elements to be %q, found %q",
				baseType, lt)
			return
		}
	}
}

// collectLeafTokenTypes returns the token-type string of every direct-child
// leaf node within n (depth 1 only, to avoid descending into sub-structures).
func (dp *DirectiveProcessor) collectLeafTokenTypes(n *Node) []string {
	var types []string
	for _, child := range n.Children {
		if child.IsLeaf() {
			types = append(types, child.Tok.Type.String())
		}
	}
	return types
}

// =============================================================================
// INFER
// =============================================================================

// processInfer resolves the type of the enclosed node.  It first checks the
// symbol table (for ident_ref nodes), then falls back to the first leaf
// token type.
func (dp *DirectiveProcessor) processInfer(n *Node) {
	if len(n.Children) == 0 {
		return
	}
	inferred := dp.inferType(n.Children[0])
	if inferred != "" {
		n.Meta.InferredType = inferred
		n.Children[0].Meta.InferredType = inferred
	}
}

// inferType returns a best-guess type string for node n by looking at:
//  1. An existing InferredType annotation (from a prior INFER pass)
//  2. A symbol-table lookup for ident_ref nodes
//  3. The first leaf token's type string
func (dp *DirectiveProcessor) inferType(n *Node) string {
	if n.Meta.InferredType != "" {
		return n.Meta.InferredType
	}
	// For ident_ref: look up in symbol table.
	if n.Kind == NodeIdentRef || n.Kind == NodeIdentName {
		name := dp.identName(n)
		if t, ok := dp.symbols[name]; ok {
			return t
		}
	}
	// Fallback: first leaf token type.
	var found string
	n.Walk(func(node *Node) bool {
		if found != "" {
			return false
		}
		if node.IsLeaf() && node.Tok.Type != TOK_BOF && node.Tok.Type != TOK_EOF {
			found = node.Tok.Type.String()
		}
		return found == ""
	})
	return found
}

// identName extracts the string name from a node tree by finding the first
// NodeIdentName node and reading the value of its leaf child.
func (dp *DirectiveProcessor) identName(n *Node) string {
	// Direct ident_name value from leaf child.
	if n.Kind == NodeIdentName {
		if v := dp.identNameValue(n); v != "" {
			return v
		}
	}
	// Walk the subtree to find the first NodeIdentName.
	var name string
	n.Walk(func(node *Node) bool {
		if name != "" {
			return false
		}
		if node.Kind == NodeIdentName {
			name = dp.identNameValue(node)
		}
		return name == ""
	})
	return name
}

// identNameValue returns the token value from a NodeIdentName node.
// NodeIdentName is an interior node whose first child is the NodeToken leaf.
func (dp *DirectiveProcessor) identNameValue(n *Node) string {
	if n.IsLeaf() && n.Tok != nil {
		// Defensive: handle directly-stored leaf (tests may construct this way).
		return n.Tok.Value
	}
	for _, child := range n.Children {
		if child.IsLeaf() && child.Tok != nil {
			return child.Tok.Value
		}
	}
	return ""
}

// =============================================================================
// Symbol table population
// =============================================================================

// recordAssignment extracts the LHS identifier name and RHS type from a
// completed NodeAssignment and stores it in the symbol table.
func (dp *DirectiveProcessor) recordAssignment(n *Node) {
	// NodeAssignment children: lhs (NodeDirectiveUnique→NodeAssignLHS), oper, rhs
	if len(n.Children) < 3 {
		return
	}
	lhsNode := n.Children[0]
	rhsNode := n.Children[2]

	name := dp.identName(lhsNode)
	if name == "" {
		return
	}
	t := dp.inferType(rhsNode)
	if t != "" {
		dp.symbols[name] = t
	}
}
