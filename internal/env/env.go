// Package env defines Name, the identifier for a named environment
// (local / homo / prod, …). It is a leaf domain type — it imports nothing — so
// vault, state, config, and cli can all speak in Name without coupling to
// each other. The empty name is the unnamed (legacy) environment, kept so repos
// that never adopt environments behave exactly as before (see docs/DECISIONS.md
// D23–D27).
package env

import (
	"errors"
	"fmt"
	"regexp"
)

// Name is the name of an environment. The zero value is Unnamed.
type Name string

// Unnamed is the legacy environment with no name: its vault is the flat
// pre-environments file and its marker carries no env field (D27).
const Unnamed Name = ""

// String returns the plain name ("" for Unnamed).
func (e Name) String() string { return string(e) }

// IsUnnamed reports whether e is the unnamed (legacy) environment.
func (e Name) IsUnnamed() bool { return e == Unnamed }

// nameRe constrains an environment name to a filesystem-safe charset, since a
// name becomes a directory under the vault (D26).
var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Validate reports why e is not a legal name to create, or nil if it is legal.
// Rejected (D26): the unnamed environment, path-unsafe names, and the reserved
// names held for a future shared layer (Model B, D24). Targeting an existing
// environment does not go through Validate — only creation does.
func (e Name) Validate() error {
	switch {
	case e.IsUnnamed():
		return errors.New("environment name is empty")
	case e == "." || e == "..":
		return fmt.Errorf("invalid environment name %q", string(e))
	case e == "shared" || e == "_base":
		return fmt.Errorf("environment name %q is reserved", string(e))
	case !nameRe.MatchString(string(e)):
		return fmt.Errorf("environment name %q must match [A-Za-z0-9._-]", string(e))
	}
	return nil
}
