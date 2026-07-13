package envfile

import (
	"reflect"
	"testing"
)

func mustParse(t *testing.T, s string) *File {
	t.Helper()
	f, err := Parse([]byte(s))
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", s, err)
	}
	return f
}

func TestParseValues(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		key   string
		value string
	}{
		{"bare", "A=1", "A", "1"},
		{"export prefix", "export A=1", "A", "1"},
		{"spaces around eq trimmed", "A =  1", "A", "1"},
		{"double quoted", `A="hello world"`, "A", "hello world"},
		{"single quoted literal", `A='no\nescape'`, "A", `no\nescape`},
		{"double quote escapes", `A="a\nb\t\"c\""`, "A", "a\nb\t\"c\""},
		{"equals in value", "A=k=v", "A", "k=v"},
		{"hash not comment without space", "A=http://x#frag", "A", "http://x#frag"},
		{"inline comment bare", "A=1 # note", "A", "1"},
		{"inline comment quoted", `A="1" # note`, "A", "1"},
		{"empty value", "A=", "A", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := mustParse(t, tt.line)
			got, ok := f.Get(tt.key)
			if !ok {
				t.Fatalf("key %q missing", tt.key)
			}
			if got != tt.value {
				t.Errorf("value = %q, want %q", got, tt.value)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{
		"missing eq",
		"1BAD=x",
		"BAD KEY=x",
		`A="unterminated`,
		`A="v"garbage`,
	} {
		if _, err := Parse([]byte(s)); err == nil {
			t.Errorf("Parse(%q): expected error, got nil", s)
		}
	}
}

func TestRoundTripPreservesLayout(t *testing.T) {
	src := "# header comment\n" +
		"\n" +
		"export A=1        # inline\n" +
		"B='literal value'\n" +
		"C=\"quoted\"\n" +
		"\n" +
		"# trailing note\n"
	f := mustParse(t, src)
	if got := string(f.Render()); got != src {
		t.Errorf("round-trip changed file:\n got=%q\nwant=%q", got, src)
	}
}

func TestKeysOrderAndMap(t *testing.T) {
	f := mustParse(t, "B=2\n# c\nA=1\nC=3\n")
	if got := f.Keys(); !reflect.DeepEqual(got, []string{"B", "A", "C"}) {
		t.Errorf("Keys() = %v, want [B A C]", got)
	}
	want := Env{"A": "1", "B": "2", "C": "3"}
	if got := f.Map(); !reflect.DeepEqual(got, want) {
		t.Errorf("Map() = %v, want %v", got, want)
	}
}

func TestSetInPlacePreservesCommentAndPosition(t *testing.T) {
	f := mustParse(t, "# top\nA=old # keep me\nB=2\n")
	f.Set("A", "new")
	want := "# top\nA=new # keep me\nB=2\n"
	if got := string(f.Render()); got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestSetAppendsNewKey(t *testing.T) {
	f := mustParse(t, "A=1\n")
	f.Set("PORT", "3000")
	f.Set("MSG", "a b") // needs quoting
	f.Set("EMPTY", "")  // bare empty
	f.Set("EQ", "k=v")  // '=' forces quoting
	want := "A=1\nPORT=3000\nMSG=\"a b\"\nEMPTY=\nEQ=\"k=v\"\n"
	if got := string(f.Render()); got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestSetSameValueDoesNotRewrite(t *testing.T) {
	// A bare value re-set to the same thing must keep its exact original text.
	f := mustParse(t, "A=1 # c\n")
	f.Set("A", "1")
	if got := string(f.Render()); got != "A=1 # c\n" {
		t.Errorf("Render() = %q, want unchanged", got)
	}
}

func TestDelete(t *testing.T) {
	f := mustParse(t, "A=1\nB=2\nA=3\n")
	f.Delete("A") // removes all A lines
	if _, ok := f.Get("A"); ok {
		t.Error("A still present after Delete")
	}
	if got := string(f.Render()); got != "B=2\n" {
		t.Errorf("Render() = %q, want %q", got, "B=2\n")
	}
	f.Delete("missing") // no-op
}
