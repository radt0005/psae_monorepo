package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored authentication credentials",
	Long:  `Removes the credentials stored by 'spade login' from ~/.spade/auth/.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogout()
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout() error {
	if err := ClearCredentials(); err != nil {
		return fmt.Errorf("removing credentials: %w", err)
	}
	fmt.Println("Logged out.")
	return nil
}
