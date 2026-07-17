package cli

// SyncOpts groups the flags shared by push and pull (--create, --dry-run).
type SyncOpts struct {
	Create bool
	DryRun bool
}

// PushOpts is SyncOpts plus push-only flags.
type PushOpts struct {
	SyncOpts
	Force bool
}
