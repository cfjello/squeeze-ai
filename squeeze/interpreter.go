// Package squeeze provides the core interpreter for the Squeeze language.
package squeeze

// Interpreter evaluates Squeeze language expressions.
type Interpreter struct{}

// NewInterpreter creates a new Squeeze interpreter.
func NewInterpreter() *Interpreter {
	return &Interpreter{}
}

// Eval evaluates the given Squeeze expression and returns the result as a string.
func (i *Interpreter) Eval(expr string) string {
	if expr == "" {
		return ""
	}
	return expr
}
