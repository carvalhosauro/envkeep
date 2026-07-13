// Package envfile parses, mutates, and renders .env files while preserving key
// order and comments (see docs/DECISIONS.md D11), and provides the pure merge /
// diff / 3-way classification logic the sync commands build on (D8, D5).
//
// The parser is deliberately single-line: KEY=VALUE with an optional "export "
// prefix, single- or double-quoted values, and space-preceded inline
// "# comments". Multiline values are not supported and produce a parse error
// (an unterminated quote), which is preferable to silently misreading a file.
package envfile

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

type lineKind int

const (
	kindBlank lineKind = iota
	kindComment
	kindKV
)

// line is one physical line of the file. For blank/comment lines only raw is
// used. For KV lines raw is reused verbatim on render unless dirty is set, so an
// untouched file round-trips byte-for-byte (modulo a normalized trailing
// newline).
type line struct {
	kind    lineKind
	raw     string
	export  bool
	key     string
	value   string
	quote   byte   // original quote style: 0 (bare), '\'' or '"'
	comment string // inline trailing comment incl. leading whitespace, e.g. "  # note"
	dirty   bool   // value changed since parse -> must re-render instead of using raw
}

// File is a parsed .env file preserving layout. It is not safe for concurrent
// use.
type File struct {
	lines []*line
	index map[string]*line // key -> its (last) KV line
}

// Parse reads an .env file. It returns an error with a 1-based line number on
// malformed input (bad key, unterminated quote, missing '=').
func Parse(data []byte) (*File, error) {
	f := &File{index: map[string]*line{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		l, err := parseLine(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		f.lines = append(f.lines, l)
		if l.kind == kindKV {
			f.index[l.key] = l
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read env: %w", err)
	}
	return f, nil
}

func parseLine(raw string) (*line, error) {
	trimmed := strings.TrimLeft(raw, " \t")
	switch {
	case trimmed == "":
		return &line{kind: kindBlank, raw: raw}, nil
	case strings.HasPrefix(trimmed, "#"):
		return &line{kind: kindComment, raw: raw}, nil
	}

	work := trimmed
	export := false
	if rest, ok := cutPrefixSpace(work, "export"); ok {
		export = true
		work = rest
	}

	rawKey, rawVal, ok := strings.Cut(work, "=")
	if !ok {
		return nil, fmt.Errorf("missing '=' in %q", raw)
	}
	key := strings.TrimRight(rawKey, " \t")
	if !validKey(key) {
		return nil, fmt.Errorf("invalid key %q", key)
	}

	value, quote, comment, err := parseValue(rawVal)
	if err != nil {
		return nil, err
	}
	return &line{
		kind: kindKV, raw: raw, export: export,
		key: key, value: value, quote: quote, comment: comment,
	}, nil
}

// cutPrefixSpace reports whether s starts with prefix followed by whitespace,
// returning the remainder with that whitespace trimmed.
func cutPrefixSpace(s, prefix string) (string, bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	rest := s[len(prefix):]
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return s, false
	}
	return strings.TrimLeft(rest, " \t"), true
}

func parseValue(s string) (value string, quote byte, comment string, err error) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return "", 0, "", nil
	}
	switch s[0] {
	case '"':
		return parseQuoted(s, '"')
	case '\'':
		return parseQuoted(s, '\'')
	default:
		return parseBare(s)
	}
}

// parseQuoted parses a single- or double-quoted value. Double quotes honor
// \n \t \r \" \\ escapes; single quotes are literal.
func parseQuoted(s string, q byte) (string, byte, string, error) {
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if q == '"' && c == '\\' && i+1 < len(s) {
			switch n := s[i+1]; n {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte('\\')
				b.WriteByte(n)
			}
			i += 2
			continue
		}
		if c == q {
			comment, err := trailingComment(s[i+1:])
			return b.String(), q, comment, err
		}
		b.WriteByte(c)
		i++
	}
	return "", 0, "", fmt.Errorf("unterminated %c-quoted value", q)
}

// trailingComment validates the text after a closing quote: it must be empty or
// a space-separated "# comment".
func trailingComment(s string) (string, error) {
	if strings.TrimLeft(s, " \t") == "" {
		return "", nil
	}
	t := strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(t, "#") {
		return "", fmt.Errorf("unexpected text after quoted value: %q", strings.TrimSpace(s))
	}
	return s, nil
}

func parseBare(s string) (string, byte, string, error) {
	if i := inlineCommentStart(s); i >= 0 {
		return strings.TrimRight(s[:i], " \t"), 0, s[i:], nil
	}
	return strings.TrimRight(s, " \t"), 0, "", nil
}

// inlineCommentStart returns the index of the whitespace run that precedes an
// inline '#' comment, or -1. A '#' not preceded by whitespace is part of the
// value (e.g. a URL fragment).
func inlineCommentStart(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			continue
		}
		j := i
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j < len(s) && s[j] == '#' {
			return i
		}
	}
	return -1
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case i > 0 && c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return true
}

// Map returns the logical key/value set, ignoring layout. Later definitions of
// a duplicated key win.
func (f *File) Map() Env {
	m := make(Env, len(f.index))
	for k, l := range f.index {
		m[k] = l.value
	}
	return m
}

// Keys returns keys in first-appearance order.
func (f *File) Keys() []string {
	seen := make(map[string]bool, len(f.index))
	keys := make([]string, 0, len(f.index))
	for _, l := range f.lines {
		if l.kind == kindKV && !seen[l.key] {
			seen[l.key] = true
			keys = append(keys, l.key)
		}
	}
	return keys
}

// Get returns the value for key and whether it is present.
func (f *File) Get(key string) (string, bool) {
	l, ok := f.index[key]
	if !ok {
		return "", false
	}
	return l.value, true
}

// Set updates key in place (preserving its line position and surrounding
// comments) or appends a new KEY=value line if it does not exist.
func (f *File) Set(key, value string) {
	if l, ok := f.index[key]; ok {
		if l.value != value {
			l.value = value
			l.dirty = true
		}
		return
	}
	l := &line{kind: kindKV, key: key, value: value, dirty: true}
	f.lines = append(f.lines, l)
	f.index[key] = l
}

// Delete removes every line defining key. It is a no-op if key is absent.
func (f *File) Delete(key string) {
	if _, ok := f.index[key]; !ok {
		return
	}
	delete(f.index, key)
	kept := f.lines[:0]
	for _, l := range f.lines {
		if l.kind == kindKV && l.key == key {
			continue
		}
		kept = append(kept, l)
	}
	f.lines = kept
}

// Render serializes the file. Untouched lines are emitted verbatim; changed or
// newly added KV lines are re-rendered with safe quoting. Output always ends
// with a trailing newline.
func (f *File) Render() []byte {
	var b bytes.Buffer
	for _, l := range f.lines {
		if l.kind == kindKV && (l.dirty || l.raw == "") {
			b.WriteString(renderKV(l))
		} else {
			b.WriteString(l.raw)
		}
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func renderKV(l *line) string {
	var b strings.Builder
	if l.export {
		b.WriteString("export ")
	}
	b.WriteString(l.key)
	b.WriteByte('=')
	b.WriteString(renderValue(l.value, l.quote))
	b.WriteString(l.comment)
	return b.String()
}

func renderValue(v string, quote byte) string {
	if v == "" {
		return ""
	}
	if quote == 0 && !strings.ContainsAny(v, " \t#\"'\n\r=") {
		return v
	}
	return `"` + doubleQuoteEscaper.Replace(v) + `"`
}

var doubleQuoteEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\t", `\t`,
	"\r", `\r`,
)
