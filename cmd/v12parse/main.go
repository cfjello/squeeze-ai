package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cfjello/squeeze-ai/pkg/parser"
)

func main() {
	filePath := "pkg/lib/integer.sqz"
	if len(os.Args) > 1 {
		filePath = os.Args[1]
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Tokenising %s ===\n", filePath)
	lex := parser.NewV12Lexer(string(src))
	tokens, err := lex.V12Tokenize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lexer error: %v\n", err)
		os.Exit(1)
	}
	for i, t := range tokens {
		fmt.Printf("  [%3d] %-18s %q  (line %d, col %d)\n", i, t.Type, t.Value, t.Line, t.Col)
	}

	fmt.Printf("\n=== Parsing %s ===\n", filePath)
	p := parser.NewV12Parser(tokens)
	root, err := p.ParseParserRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nPARSE ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Parse OK")
	out, _ := json.MarshalIndent(root, "", "  ")
	fmt.Println(string(out))
}
