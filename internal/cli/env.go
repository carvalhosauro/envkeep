package cli

import (
	"io"

	"github.com/carvalhosauro/envkeep/internal/vault"
)

// Envs lists the repo's environments, marking the default with "*".
func Envs(w io.Writer, cwd string) error {
	ctx, err := Resolve(cwd, "", "")
	if err != nil {
		return err
	}
	p := &printer{w: w}
	envs, err := vault.Environments(ctx.CommonDir)
	if err != nil {
		return err
	}
	if len(envs) == 0 {
		p.printf("no environments yet (run 'envkeep use -c <env>' or 'envkeep push --env <env> --create')\n")
		return p.err
	}
	for _, e := range envs {
		marker := " "
		if e == ctx.DefaultEnv {
			marker = "*"
		}
		p.printf("%s %s\n", marker, e.String())
	}
	return p.err
}
