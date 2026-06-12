package app

import (
	"fmt"
	"os"
)

// runMain is the implementation half of Main(); kept separate so tests
// can call into either the wrapper or the implementation directly.
//
// The full startup sequence (open store, dial broker, recover, start
// loops, serve HTTP) is wired in main_run.go; this stub allows the
// package to compile while the rest of the scheduler is being built.
func runMain() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "spade-scheduler:", err)
		os.Exit(1)
	}
}
