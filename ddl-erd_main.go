package main

// for `go generate -x`
//go:generate sh ddl-erd_cliGen.sh

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// define flags
	initVars()
	// popoulate flag variables from ENV
	initVals()
	// popoulate flag variables from cli
	flag.Parse()
	if Opts.Help {
		Usage(1)
	}

	// == Sanity check on stdin
	// Get file information about stdin
	info, err := os.Stdin.Stat()
	if err != nil {
		log.Fatalf("Error checking stdin:", err)
	}
	// Check if stdin is from a pipe
	if info.Mode()&os.ModeCharDevice != 0 {
		Usage(0)
		log.Fatalf("\nError: This program reads input from pipe.")
	}

	// Read all data from stdin
	input, err := readInput()
	if err != nil {
		log.Fatalf("Error reading input: %v", err)
	}

	relations := parseSQL(input)
	tablesDisplay := defineTableDisplay(relations)

	// Output & Dispatch
	if len(Opts.LPA) != 0 {
		generateLPADotFiles(Opts.LPA, relations, tablesDisplay)
		return
	} else {
		dotOutput := generateDot(relations, tablesDisplay)
		fmt.Println(dotOutput)
	}
}
