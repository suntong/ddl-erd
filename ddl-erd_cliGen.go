// !!! !!!
// WARNING: Code automatically generated. Editing discouraged.
// !!! !!!

package main

import (
	"flag"
	"fmt"
	"os"
)

////////////////////////////////////////////////////////////////////////////
// Constant and data type/structure definitions

const progname = "ddl-erd" // os.Args[0]

// The Options struct defines the structure to hold the commandline values
type Options struct {
	LPA  string // use LPA (Label Propagation Algorithms) based community detection partitioning strategy
	Help bool   // show usage help
}

////////////////////////////////////////////////////////////////////////////
// Global variables definitions

// Opts holds the actual values from the command line parameters
var Opts Options

////////////////////////////////////////////////////////////////////////////
// Commandline definitions

func initVars() {

	// set default values for command line parameters
	flag.StringVar(&Opts.LPA, "lpa", "",
		"use LPA (Label Propagation Algorithms) based community detection partitioning strategy")
	flag.BoolVar(&Opts.Help, "h", false,
		"show usage help")
}

func initVals() {
	exists := false
	// Now override those default values from environment variables
	if len(Opts.LPA) == 0 &&
		len(os.Getenv("DDL_ERD_LPA")) != 0 {
		Opts.LPA = os.Getenv("DDL_ERD_LPA")
	}
	if _, exists = os.LookupEnv("DDL_ERD_H"); Opts.Help || exists {
		Opts.Help = true
	}

}

const usageSummary = "  -lpa\tuse LPA (Label Propagation Algorithms) based community detection partitioning strategy (DDL_ERD_LPA)\n  -h\tshow usage help (DDL_ERD_H)\n\nDetails:\n\n"

// Usage function shows help on commandline usage
func Usage(exit ...int) {
	fmt.Fprintf(os.Stderr,
		"\nUsage:\n %s [flags] \n\nFlags:\n\n",
		progname)
	fmt.Fprintf(os.Stderr, usageSummary)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr,
		``)
	if exit[0] != 0 {
		os.Exit(0)
	}
}
