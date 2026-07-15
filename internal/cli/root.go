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
	// Setting Version makes cobra auto-bind --version (shorthand -v, since v is
	// otherwise free), restoring the pre-cobra `--version`/`-v` aliases. The
	// template reproduces the exact old output: "envkeep <version>\n". The
	// `version` subcommand below is kept alongside it (plan-specified).
	root.Version = buildinfo.Version
	root.SetVersionTemplate("envkeep {{.Version}}\n")
	root.AddCommand(newVersionCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newPushCmd(), newPullCmd())
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
// Wired into the status/push/pull/check subcommands ported in A2-A5.
func processCwd() (string, error) {
	return os.Getwd()
}

func newStatusCmd() *cobra.Command {
	var file, envName string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "show each worktree's active env and sync state vs the vault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := processCwd()
			if err != nil {
				return err
			}
			return Status(cmd.OutOrStdout(), cwd, file, envName)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "tracked env filename (overrides repo config)")
	cmd.Flags().StringVar(&envName, "env", "", "environment to compare against (default: each worktree's active env)")
	return cmd
}

func newPushCmd() *cobra.Command {
	var file, envName string
	var create, dry bool
	cmd := &cobra.Command{
		Use:   "push",
		Short: "merge this worktree's env into the environment's vault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := processCwd()
			if err != nil {
				return err
			}
			return Push(cmd.OutOrStdout(), cwd, file, envName, create, dry)
		},
	}
	addSyncFlags(cmd, &file, &envName, &create, &dry)
	return cmd
}

func newPullCmd() *cobra.Command {
	var file, envName string
	var create, dry bool
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "write the environment's vault into this worktree's env",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := processCwd()
			if err != nil {
				return err
			}
			return Pull(cmd.OutOrStdout(), cwd, file, envName, create, dry)
		},
	}
	addSyncFlags(cmd, &file, &envName, &create, &dry)
	return cmd
}

// addSyncFlags registers the flags shared by push and pull.
func addSyncFlags(cmd *cobra.Command, file, envName *string, create, dry *bool) {
	cmd.Flags().StringVar(file, "file", "", "tracked env filename (overrides repo config)")
	cmd.Flags().StringVar(envName, "env", "", "target environment (default: this worktree's active env)")
	cmd.Flags().BoolVarP(create, "create", "c", false, "create the environment if it does not exist")
	cmd.Flags().BoolVar(dry, "dry-run", false, "show what would change without writing")
}
