package cli

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This file is the load-test lane. The heavy scale test (TestLoadScale) is
// gated behind ENVKEEP_LOADTEST so it never slows `go test` / CI; the
// Benchmark* functions only run under `-bench`. Both build REAL git worktrees
// (D18) and a synthetic env whose shape mirrors a large real .env (fake
// values), to measure the sync commands under extreme key counts and file
// sizes and to guard the D22 agreement path at scale.
//
//   make bench                      # benchmark the read hot paths
//   make loadtest                   # scale correctness + timing
//   ENVKEEP_LOADTEST_KEYS=20000 ENVKEEP_LOADTEST_WT=12 ENVKEEP_LOADTEST_BIG=1 \
//     make loadtest                 # crank the knobs

const (
	upperAlpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerAlpha = "abcdefghijklmnopqrstuvwxyz"
	digits     = "0123456789"
	alnum      = upperAlpha + lowerAlpha + digits
	lowerDig   = lowerAlpha + digits
	hexChars   = "0123456789abcdef"
	b64Chars   = alnum + "+/="
)

// gitCmd runs an isolated git command (own identity, no ambient GIT_* env) so
// the fixture is self-contained regardless of who invokes the test.
func gitCmd(tb testing.TB, dir string, args ...string) {
	tb.Helper()
	base := []string{
		"-c", "user.email=loadtest@test.local",
		"-c", "user.name=loadtest",
		"-c", "commit.gpgsign=false",
		"-c", "init.defaultBranch=main",
	}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	cmd.Env = gitCleanEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		tb.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitCleanEnv() []string {
	drop := []string{"GIT_DIR=", "GIT_WORK_TREE=", "GIT_COMMON_DIR=", "GIT_INDEX_FILE=", "GIT_PREFIX=", "GIT_NAMESPACE="}
	out := os.Environ()[:0:0]
	for _, e := range os.Environ() {
		skip := false
		for _, p := range drop {
			if strings.HasPrefix(e, p) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, e)
		}
	}
	return out
}

// buildLoadRepo creates a repo with nWorktrees linked worktrees (wt-00..) plus
// the primary "main", returning the worktree paths. Skips when git is absent.
func buildLoadRepo(tb testing.TB, nWorktrees int) []string {
	tb.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		tb.Skip("git not installed")
	}
	root := tb.TempDir()
	main := filepath.Join(root, "main")
	gitCmd(tb, root, "init", "-q", "-b", "main", main)
	if err := os.WriteFile(filepath.Join(main, "README"), []byte("seed\n"), 0o644); err != nil {
		tb.Fatal(err)
	}
	gitCmd(tb, main, "add", "README")
	gitCmd(tb, main, "commit", "-q", "-m", "seed")

	wts := make([]string, nWorktrees)
	for i := range wts {
		name := fmt.Sprintf("wt-%02d", i)
		wt := filepath.Join(root, name)
		gitCmd(tb, main, "worktree", "add", "-q", wt, "-b", name)
		wts[i] = wt
	}
	return wts
}

// genLoadEnv renders a synthetic .env with nKeys pairs across realistic
// categories (URL/SECRET/DSN/…) with fake values. big scales secret/token/key
// values into the multi-KiB range to stress large-value handling. Output is
// deterministic (fixed seed) so runs are comparable.
func genLoadEnv(nKeys int, big bool) string {
	r := rand.New(rand.NewSource(1234))
	cats := []string{"ID", "URL", "URI", "SECRET", "TOKEN", "KEY", "DSN", "PASS", "USER", "MODEL", "REGION", "NAME", "LOG", "HOST", "PORT"}
	var b strings.Builder
	b.WriteString("# synthetic load-test env — fake values\n")
	for i := range nKeys {
		cat := cats[i%len(cats)]
		key := fmt.Sprintf("SVC%05d_%s_%s", i, randStr(r, 3+r.Intn(18), upperAlpha), cat)
		var v string
		switch cat {
		case "URL", "URI":
			v = "https://svc-" + randStr(r, 8, lowerDig) + ".example.internal:8443/v1/" + randStr(r, 12, lowerDig) + "?tok=" + randStr(r, 24, alnum)
		case "DSN":
			v = "postgres://user_" + randStr(r, 6, lowerAlpha) + ":" + randStr(r, 18, alnum) + "@db-" + randStr(r, 6, lowerDig) + ".example:5432/app_" + randStr(r, 5, lowerAlpha)
		case "SECRET", "TOKEN", "KEY":
			sizes := []int{40, 64, 88, 128}
			if big {
				sizes = []int{256, 1024, 4096, 8192}
			}
			v = randStr(r, sizes[r.Intn(len(sizes))], b64Chars)
		case "ID":
			v = randStr(r, 24, hexChars)
		case "PORT":
			v = strconv.Itoa(1024 + r.Intn(64000))
		default:
			v = randStr(r, 4+r.Intn(56), alnum)
		}
		if i%7 == 0 {
			v += " extra note" // internal spaces -> forces quoting on render
		}
		line := key + "=" + v
		if i%9 == 0 {
			line += "  # tuned" // inline comment
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if i%40 == 39 {
			fmt.Fprintf(&b, "# --- section %d ---\n\n", i/40)
		}
	}
	return b.String()
}

func randStr(r *rand.Rand, n int, set string) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = set[r.Intn(len(set))]
	}
	return string(buf)
}

// setupLoaded builds a repo, writes the synthetic env to wt-00, pushes it, and
// pulls it into every other worktree — leaving all worktrees synced+clean.
func setupLoaded(tb testing.TB, keys, nwt int, big bool) []string {
	tb.Helper()
	wts := buildLoadRepo(tb, nwt)
	env := genLoadEnv(keys, big)
	if err := os.WriteFile(filepath.Join(wts[0], ".env"), []byte(env), 0o600); err != nil {
		tb.Fatal(err)
	}
	mustCmd(tb, "push", func(b *bytes.Buffer) error { return Push(b, wts[0], "", "", false, false) })
	for _, wt := range wts[1:] {
		w := wt
		mustCmd(tb, "pull", func(b *bytes.Buffer) error { return Pull(b, w, "", "", false, false) })
	}
	return wts
}

func mustCmd(tb testing.TB, name string, fn func(*bytes.Buffer) error) {
	tb.Helper()
	var b bytes.Buffer
	if err := fn(&b); err != nil {
		tb.Fatalf("%s: %v\n%s", name, err, b.String())
	}
}

func vaultFile(tb testing.TB, cwd string) string {
	tb.Helper()
	ctx, err := Resolve(cwd, "", "")
	if err != nil {
		tb.Fatal(err)
	}
	return ctx.vaultPath("")
}

// TestLoadScale drives the sync commands over a large synthetic env across many
// worktrees, asserts correctness at scale (no false diverged, the #1/#4
// stale-base-agrees path settles to clean), and logs per-command wall time.
// Gated behind ENVKEEP_LOADTEST so ordinary test runs skip it.
func TestLoadScale(t *testing.T) {
	if os.Getenv("ENVKEEP_LOADTEST") == "" {
		t.Skip("set ENVKEEP_LOADTEST=1 to run (heavy: builds real git worktrees)")
	}
	keys := envInt("ENVKEEP_LOADTEST_KEYS", 5000)
	nwt := envInt("ENVKEEP_LOADTEST_WT", 8)
	big := os.Getenv("ENVKEEP_LOADTEST_BIG") != ""

	wts := buildLoadRepo(t, nwt)
	env := genLoadEnv(keys, big)
	t.Logf("env: %d keys, %.1f KiB, %d worktrees, big=%v", keys, float64(len(env))/1024, nwt, big)
	writeFile(t, filepath.Join(wts[0], ".env"), env)

	timed(t, "push", func() {
		mustCmd(t, "push", func(b *bytes.Buffer) error { return Push(b, wts[0], "", "", false, false) })
	})
	timed(t, "pull all", func() {
		for _, wt := range wts[1:] {
			mustCmd(t, "pull", func(b *bytes.Buffer) error { return Pull(b, wt, "", "", false, false) })
		}
	})
	timed(t, "status fast", func() { runStatus(t, wts[0]) })
	timed(t, "check fast", func() { checkPorcelain(t, wts[0]) })

	// Force the slow path for every worktree by bumping the vault mtime (status
	// is read-only, so no marker refresh rescues it) and time a full reparse.
	vp := vaultFile(t, wts[0])
	if st, err := os.Stat(vp); err == nil {
		bump := st.ModTime().Add(5 * time.Second)
		_ = os.Chtimes(vp, bump, bump)
	}
	timed(t, "status reparse", func() { runStatus(t, wts[0]) })

	// Correctness at scale: every worktree clean, zero false diverged.
	out := runStatus(t, wts[0])
	if strings.Contains(out, "diverged") {
		t.Errorf("false 'diverged' at scale:\n%s", out)
	}
	if n := strings.Count(out, " clean"); n != nwt {
		t.Errorf("clean worktrees = %d, want %d\n%s", n, nwt, out)
	}

	// #1 at scale: drive wt-02 to a stale base whose content now agrees with the
	// vault, then confirm it settles to clean (the D22 agreement path).
	parts := strings.SplitN(env, "\n", 3)
	firstKV := parts[1]
	firstKey := strings.SplitN(firstKV, "=", 2)[0]
	changed := strings.Replace(env, firstKV, firstKey+"=CHANGED_LOADTEST_VALUE", 1)
	writeFile(t, filepath.Join(wts[0], ".env"), changed)
	mustCmd(t, "push", func(b *bytes.Buffer) error { return Push(b, wts[0], "", "", false, false) })
	writeFile(t, filepath.Join(wts[2], ".env"), changed) // matches vault, base stale
	if tok := checkPorcelain(t, wts[2]); tok != "" {
		t.Errorf("#1 stale-base-agrees should be clean, got %q", tok)
	}
}

func runStatus(t *testing.T, cwd string) string {
	t.Helper()
	var b bytes.Buffer
	if err := Status(&b, cwd, "", ""); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func timed(t *testing.T, label string, fn func()) {
	t.Helper()
	start := time.Now()
	fn()
	t.Logf("  %-16s %v", label, time.Since(start).Round(time.Millisecond))
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func benchKeys() int { return envInt("ENVKEEP_LOADTEST_KEYS", 2000) }
func benchWT() int   { return envInt("ENVKEEP_LOADTEST_WT", 6) }

// BenchmarkStatusFastPath measures status when nothing moved (pure mtime cache).
func BenchmarkStatusFastPath(b *testing.B) {
	wts := setupLoaded(b, benchKeys(), benchWT(), false)
	b.ReportAllocs()
	for b.Loop() {
		var buf bytes.Buffer
		if err := Status(&buf, wts[0], "", ""); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStatusReparseAll measures the worst case: every worktree reparsed and
// reclassified (vault mtime bumped so no marker matches; status never refreshes).
func BenchmarkStatusReparseAll(b *testing.B) {
	wts := setupLoaded(b, benchKeys(), benchWT(), false)
	vp := vaultFile(b, wts[0])
	if st, err := os.Stat(vp); err == nil {
		bump := st.ModTime().Add(5 * time.Second)
		_ = os.Chtimes(vp, bump, bump)
	}
	b.ReportAllocs()
	for b.Loop() {
		var buf bytes.Buffer
		if err := Status(&buf, wts[0], "", ""); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCheckFastPath measures the per-prompt hook on a clean worktree.
func BenchmarkCheckFastPath(b *testing.B) {
	wts := setupLoaded(b, benchKeys(), benchWT(), false)
	b.ReportAllocs()
	for b.Loop() {
		var buf bytes.Buffer
		if err := Check(&buf, wts[0], true); err != nil {
			b.Fatal(err)
		}
	}
}
