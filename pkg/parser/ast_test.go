package parser

import (
	"strings"
	"testing"
)

func TestNewTokenNode(t *testing.T) {
	tok := Token{Type: TOK_IDENT, Value: "foo", Line: 3, Col: 5}
	n := NewTokenNode(tok)

	if n.Kind != NodeToken {
		t.Errorf("expected NodeToken, got %s", n.Kind)
	}
	if !n.IsLeaf() {
		t.Error("expected IsLeaf() == true")
	}
	if n.TokenValue() != "foo" {
		t.Errorf("expected TokenValue 'foo', got %q", n.TokenValue())
	}
	if n.Pos.Line != 3 || n.Pos.Col != 5 {
		t.Errorf("wrong position: %s", n.Pos)
	}
}

func TestNewNode(t *testing.T) {
	pos := Pos{Line: 1, Col: 0}
	lhs := NewTokenNode(Token{Type: TOK_IDENT, Value: "x"})
	op := NewTokenNode(Token{Type: TOK_COLON, Value: ":"})
	rhs := NewTokenNode(Token{Type: TOK_INTEGER, Value: "42"})

	assign := NewNode(NodeAssignment, pos, lhs, op, rhs)

	if assign.Kind != NodeAssignment {
		t.Errorf("expected NodeAssignment, got %s", assign.Kind)
	}
	if len(assign.Children) != 3 {
		t.Errorf("expected 3 children, got %d", len(assign.Children))
	}
	if assign.IsLeaf() {
		t.Error("interior node should not be a leaf")
	}
}

func TestNewDirectiveNode(t *testing.T) {
	pos := Pos{Line: 2, Col: 0}
	inner := NewTokenNode(Token{Type: TOK_IDENT, Value: "myList"})
	d := NewDirectiveNode(NodeDirectiveUnique, "", pos, inner)

	if d.Kind != NodeDirectiveUnique {
		t.Errorf("expected NodeDirectiveUnique, got %s", d.Kind)
	}
	if len(d.Children) != 1 || d.Children[0] != inner {
		t.Error("directive node should wrap exactly the inner node")
	}
	if d.Meta.DirectiveKind != NodeDirectiveUnique {
		t.Error("Meta.DirectiveKind not set")
	}
}

func TestNewDirectiveNodeWithArg(t *testing.T) {
	pos := Pos{Line: 1, Col: 0}
	inner := NewTokenNode(Token{Type: TOK_IDENT, Value: "val"})
	d := NewDirectiveNode(NodeDirectiveRange, "1..128", pos, inner)

	if d.Meta.DirectiveArg != "1..128" {
		t.Errorf("expected DirectiveArg '1..128', got %q", d.Meta.DirectiveArg)
	}
}

func TestNewErrorNode(t *testing.T) {
	pos := Pos{Line: 5, Col: 10}
	e := NewErrorNode("unexpected token", pos)
	if !e.IsError() {
		t.Error("expected IsError() == true")
	}
	if e.Tok.Value != "unexpected token" {
		t.Errorf("wrong error message: %q", e.Tok.Value)
	}
}

func TestAppend(t *testing.T) {
	pos := Pos{Line: 1, Col: 0}
	parent := NewNode(NodeNumExprList, pos)
	c1 := NewTokenNode(Token{Type: TOK_INTEGER, Value: "1"})
	c2 := NewTokenNode(Token{Type: TOK_INTEGER, Value: "2"})

	parent.Append(c1).Append(c2)
	if len(parent.Children) != 2 {
		t.Errorf("expected 2 children after Append, got %d", len(parent.Children))
	}
}

func TestWalk(t *testing.T) {
	// Build a small tree: assignment → [ident, ":", integer]
	pos := Pos{}
	root := NewNode(NodeAssignment, pos,
		NewTokenNode(Token{Type: TOK_IDENT, Value: "x"}),
		NewTokenNode(Token{Type: TOK_COLON, Value: ":"}),
		NewTokenNode(Token{Type: TOK_INTEGER, Value: "7"}),
	)

	var visited []string
	root.Walk(func(n *Node) bool {
		visited = append(visited, n.Kind.String())
		return true
	})

	// Expect assignment, then the 3 Token children (pre-order).
	if len(visited) != 4 {
		t.Errorf("expected 4 visited nodes, got %d: %v", len(visited), visited)
	}
	if visited[0] != "assignment" {
		t.Errorf("expected first visited to be 'assignment', got %q", visited[0])
	}
}

func TestFindAll(t *testing.T) {
	pos := Pos{}
	root := NewNode(NodeLogicExpr, pos,
		NewNode(NodeAssignment, pos,
			NewTokenNode(Token{Type: TOK_IDENT, Value: "a"}),
		),
		NewNode(NodeAssignment, pos,
			NewTokenNode(Token{Type: TOK_IDENT, Value: "b"}),
		),
	)

	found := root.FindAll(NodeAssignment)
	if len(found) != 2 {
		t.Errorf("expected 2 NodeAssignment nodes, got %d", len(found))
	}
	foundTokens := root.FindAll(NodeToken)
	if len(foundTokens) != 2 {
		t.Errorf("expected 2 NodeToken leaves, got %d", len(foundTokens))
	}
}

func TestPretty(t *testing.T) {
	pos := Pos{Line: 1, Col: 0}
	root := NewNode(NodeAssignment, pos,
		NewTokenNode(Token{Type: TOK_IDENT, Value: "x", Line: 1, Col: 0}),
		NewTokenNode(Token{Type: TOK_COLON, Value: ":", Line: 1, Col: 2}),
		NewTokenNode(Token{Type: TOK_INTEGER, Value: "42", Line: 1, Col: 4}),
	)

	pretty := root.Pretty()
	if !strings.Contains(pretty, "assignment") {
		t.Errorf("Pretty() missing 'assignment': %s", pretty)
	}
	if !strings.Contains(pretty, `"x"`) {
		t.Errorf("Pretty() missing ident value: %s", pretty)
	}
	if !strings.Contains(pretty, `"42"`) {
		t.Errorf("Pretty() missing integer value: %s", pretty)
	}
}

func TestNodeKindString(t *testing.T) {
	cases := []struct {
		kind NodeKind
		want string
	}{
		{NodeToken, "Token"},
		{NodeAssignment, "assignment"},
		{NodeFuncUnit, "func_unit"},
		{NodeDirectiveUnique, "UNIQUE"},
		{NodeDirectiveTypeOf, "TYPE_OF"},
		{NodeScopeAssign, "scope_assign"},
		{NodeError, "ERROR"},
	}
	for _, c := range cases {
		if got := c.kind.String(); got != c.want {
			t.Errorf("NodeKind(%d).String() = %q, want %q", int(c.kind), got, c.want)
		}
	}
}
