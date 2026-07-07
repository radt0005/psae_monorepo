package spade

import (
	"fmt"
	"os"
)

// Run executes a handler function as a Spade block. It loads inputs and params,
// calls the handler, and writes outputs. On any error it prints to stderr and exits.
func Run[O IntoOutput](handler func(*Args) (O, error)) {
	loadSecrets() // scrub SPADE_SECRETS from the environment early
	if err := RunAt(".", handler); err != nil {
		fmt.Fprintf(os.Stderr, "spade: %s\n", err)
		os.Exit(1)
	}
}

// RunAt executes a handler at a specific base path. Used for testing.
func RunAt[O IntoOutput](base string, handler func(*Args) (O, error)) error {
	args, err := BuildArgs(base)
	if err != nil {
		return err
	}

	result, err := handler(args)
	if err != nil {
		return err
	}

	if result.DefaultOutputName() == "__none__" {
		return nil
	}

	manifest := ReadBlockManifest(base)
	return WriteOutputs(result, base, manifest)
}

// RunNoOutput executes a handler that produces no output.
func RunNoOutput(handler func(*Args) error) {
	loadSecrets() // scrub SPADE_SECRETS from the environment early
	if err := RunNoOutputAt(".", handler); err != nil {
		fmt.Fprintf(os.Stderr, "spade: %s\n", err)
		os.Exit(1)
	}
}

// RunNoOutputAt executes a no-output handler at a specific base path. Used for testing.
func RunNoOutputAt(base string, handler func(*Args) error) error {
	args, err := BuildArgs(base)
	if err != nil {
		return err
	}
	return handler(args)
}
