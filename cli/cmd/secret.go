package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"spade/internal/secretstore"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage local secrets for pipeline runs",
	Long: `Store secrets in the operating-system keychain for use by local pipeline
runs (spade run). A pipeline block references a secret by name in its 'secrets'
mapping; the value is read from the keychain and injected into the block at run
time — it never appears in the pipeline file.

Local secrets are independent from cloud secrets, which live in the Spade KMS
(see spec/secrets.md). A block that references a secret must find it in the
local keychain to run locally.`,
}

var secretSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Store a secret value in the OS keychain",
	Long: `Store a secret value under a name. The value is read from the terminal
without echoing, or from standard input when piped (e.g. echo "$DSN" | spade
secret set db).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		value, err := readSecretValue()
		if err != nil {
			return fmt.Errorf("reading secret value: %w", err)
		}
		if value == "" {
			return errors.New("empty secret value")
		}
		if err := secretstore.Set(name, value); err != nil {
			return fmt.Errorf("storing secret %q: %w", name, err)
		}
		fmt.Printf("Stored secret %q in the OS keychain.\n", name)
		return nil
	},
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the names of stored secrets",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := secretstore.List()
		if err != nil {
			return fmt.Errorf("listing secrets: %w", err)
		}
		if len(names) == 0 {
			fmt.Println("No secrets stored.")
			return nil
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return nil
	},
}

var secretRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a stored secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := secretstore.Delete(name); err != nil {
			return fmt.Errorf("removing secret %q: %w", name, err)
		}
		fmt.Printf("Removed secret %q.\n", name)
		return nil
	},
}

func init() {
	secretCmd.AddCommand(secretSetCmd, secretListCmd, secretRmCmd)
	rootCmd.AddCommand(secretCmd)
}

// readSecretValue reads a secret value from the terminal without echoing, or
// from standard input when it is not a terminal (piped input).
func readSecretValue() (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Enter secret value: ")
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
