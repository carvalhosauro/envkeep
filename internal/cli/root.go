package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/carvalhosauro/envkeep/internal/buildinfo"
)

// Process exit codes: 0 success, 1 error (runtime or usage). The stdlib version
// distinguished 2 for flag-parse errors; cobra unifies that into 1.
const (
	exitOK    = 0
	exitError = 1
)

// Execute builds the root command, runs it, and returns the process exit code.
// Errors are printed once here with the "envkeep:" prefix (SilenceErrors), so
// the message format matches the pre-cobra CLI.
func Execute() int {
	root := newRootCmd()
	root.SilenceErrors = true
	root.SilenceUsage = true
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "envkeep:", err)
		return exitError
	}
	return exitOK
}

// newRootCmd assembles the root command and all subcommands. Kept unexported so
// tests can build a fresh tree per run (SetArgs/SetOut) without global state.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "envkeep",
		Short:         "keep .env in sync across the git worktrees of one repo",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the envkeep version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Preserve the exact pre-cobra output: "envkeep <version>".
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "envkeep", buildinfo.Version)
			return err
		},
	}
}

// processCwd returns the working directory the command was invoked in.
//
//nolint:unused // wired into the status/push/pull/check subcommands ported in A2-A5
func processCwd() (string, error) {
	return os.Getwd()
}
