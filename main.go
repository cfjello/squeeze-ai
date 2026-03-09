package main

import (
	"fmt"

	"github.com/cfjello/squeeze-ai/squeeze"
)

func main() {
	interp := squeeze.NewInterpreter()
	result := interp.Eval("Hello, Squeeze!")
	fmt.Println(result)
}
