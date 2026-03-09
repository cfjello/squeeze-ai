package squeeze_test

import (
	"testing"

	"github.com/cfjello/squeeze-ai/squeeze"
)

func TestNewInterpreter(t *testing.T) {
	interp := squeeze.NewInterpreter()
	if interp == nil {
		t.Fatal("NewInterpreter() returned nil")
	}
}

func TestEval(t *testing.T) {
	interp := squeeze.NewInterpreter()

	tests := []struct {
		input string
		want  string
	}{
		{"Hello, Squeeze!", "Hello, Squeeze!"},
		{"", ""},
		{"42", "42"},
	}

	for _, tt := range tests {
		got := interp.Eval(tt.input)
		if got != tt.want {
			t.Errorf("Eval(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
