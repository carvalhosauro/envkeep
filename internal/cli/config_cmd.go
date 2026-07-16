package cli

import (
	"github.com/spf13/cobra"

	"github.com/carvalhosauro/envkeep/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "get/set repo configuration"}
	cmd.AddCommand(newConfigGetCmd(), newConfigSetCmd(), newConfigListCmd(), newConfigUnsetCmd())
	return cmd
}

func configCommonDir() (string, error) {
	cwd, err := processCwd()
	if err != nil {
		return "", err
	}
	ctx, err := Resolve(cwd, "", "")
	if err != nil {
		return "", err
	}
	return ctx.CommonDir, nil
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <key>", Args: cobra.ExactArgs(1),
		ValidArgs: config.Keys(),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := configCommonDir()
			if err != nil {
				return err
			}
			v, _, err := config.Get(dir, args[0])
			if err != nil {
				return err
			}
			cmd.Println(v)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "set <key> <value>", Args: cobra.ExactArgs(2),
		ValidArgs: config.Keys(),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := configCommonDir()
			if err != nil {
				return err
			}
			return config.Set(dir, args[0], args[1])
		},
	}
}

func newConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "unset <key>", Args: cobra.ExactArgs(1),
		ValidArgs: config.Keys(),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := configCommonDir()
			if err != nil {
				return err
			}
			return config.Unset(dir, args[0])
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := configCommonDir()
			if err != nil {
				return err
			}
			for _, k := range config.Keys() {
				v, _, err := config.Get(dir, k)
				if err != nil {
					return err
				}
				cmd.Printf("%s=%s\n", k, v)
			}
			return nil
		},
	}
}
