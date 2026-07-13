package envfile

import "maps"

// Env is a logical set of environment key/value pairs, independent of file
// layout. It is the core data structure the sync logic operates on: File.Map
// produces one, the vault is one on disk, and merge/diff/classify all work in
// terms of it.
type Env map[string]string

// Equal reports whether two sets have identical keys and values.
func (e Env) Equal(other Env) bool {
	return maps.Equal(e, other)
}
