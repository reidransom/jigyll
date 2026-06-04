package main

import (
	"fmt"
	"os"

	"github.com/reidransom/gojekyll/commands"
)

func main() {
	err := commands.ParseAndRun(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
