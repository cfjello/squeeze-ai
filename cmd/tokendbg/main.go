package main

import (
	"fmt"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

func tok(src string) {
	l := parser.NewLexer(src)
	toks, _ := l.Tokenize()
	fmt.Printf("%-20s => ", src)
	for _, t := range toks {
		fmt.Printf("[%s:%q] ", t.Type, t.Value)
	}
	fmt.Println()
}

func main() {
	tok("[]")
	tok("{}")
	tok(">>")
	tok("//")
	tok(`""`)
	tok("''")
	tok("``")
	tok("{ x: 1 }")
	tok("{ x : 1 }")
	tok("[42]")
}
