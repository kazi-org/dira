package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kazi-org/dira/internal/config"
)

// TestItReadsTheRealConfig is the case that matters most: this repository's own
// .dira/config.toml, parsed by the reader that will be run against it in
// production. A hand-rolled reader tested only against inputs its author wrote
// is a reader tested against its own assumptions.
func TestItReadsTheRealConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", ".dira", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("parsing this repository's own config: %v", err)
	}
	if cfg.Name != "dira" {
		t.Errorf("ledger.name = %q, want %q", cfg.Name, "dira")
	}
	if cfg.Tier != "repo" {
		t.Errorf("ledger.tier = %q, want %q", cfg.Tier, "repo")
	}
	if cfg.MaxTokens != 1500 {
		t.Errorf("brief.max_tokens = %d, want 1500 — the number cst-0001 fixes", cfg.MaxTokens)
	}
	// Every [parents] line in that file is commented out today. A commented
	// example is not a declaration, which is the rule scripts/privacy-lint.py
	// applies to the same file for the same reason: a reader that counted
	// them would report parent ledgers nobody configured.
	if len(cfg.Parents) != 0 {
		t.Errorf("parents = %v, want none: every declaration in that file is commented out", cfg.Parents)
	}
	if len(cfg.ParentDecls) != 0 {
		t.Errorf("parent declarations = %+v, want none: every declaration in that file is commented out", cfg.ParentDecls)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    config.Config
		wantErr string
	}{
		{
			name:  "an empty file yields no ceiling of its own",
			input: "",
			want:  config.Config{},
		},
		{
			name:  "a lowered ceiling",
			input: "[brief]\nmax_tokens = 400\n",
			want:  config.Config{MaxTokens: 400},
		},
		{
			name:  "a trailing comment is not part of the value",
			input: "[brief]\nmax_tokens = 400 # lowered while the ledger is small\n",
			want:  config.Config{MaxTokens: 400},
		},
		{
			name:  "a hash inside a quoted value is not a comment",
			input: "[ledger]\nname = \"dira # 2\"\n",
			want:  config.Config{Name: "dira # 2"},
		},
		{
			name:  "sections dira does not read are ignored rather than rejected",
			input: "[kazi]\nenabled = true\n\n[mirror]\nadr = true\nadr_dir = \"docs/adr\"\n\n[brief]\nmax_tokens = 900\n",
			want:  config.Config{MaxTokens: 900},
		},
		{
			name:  "parents are declared in file order",
			input: "[parents]\nsire = { path = \"../sire\" }\nme = { visibility = \"private\" }\n",
			want:  config.Config{Parents: []string{"sire", "me"}},
		},
		{
			name:  "a commented parent is not a parent",
			input: "[parents]\n# sire = { path = \"../sire\" }\n",
			want:  config.Config{},
		},
		{
			name:    "a ceiling that is not a number is reported, not guessed at",
			input:   "[brief]\nmax_tokens = \"fifteen hundred\"\n",
			want:    config.Config{},
			wantErr: "not a number",
		},
		{
			name:    "a ceiling of zero is reported rather than read as no ceiling",
			input:   "[brief]\nmax_tokens = 0\n",
			want:    config.Config{},
			wantErr: "has to be positive",
		},
		{
			name:  "a key outside any section is not attributed to one",
			input: "max_tokens = 10\n[brief]\nmax_tokens = 800\n",
			want:  config.Config{MaxTokens: 800},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := config.Parse([]byte(tc.input))

			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("Parse: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("Parse returned no error, want one mentioning %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("Parse error = %q, want one mentioning %q", err, tc.wantErr)
			}

			if got.Name != tc.want.Name || got.Tier != tc.want.Tier || got.MaxTokens != tc.want.MaxTokens {
				t.Errorf("Parse = %+v, want %+v", got, tc.want)
			}
			if !slices.Equal(got.Parents, tc.want.Parents) {
				t.Errorf("parents = %v, want %v", got.Parents, tc.want.Parents)
			}
		})
	}
}

// TestParentsAreDeclarationsNotNames is the case this reader was extended for.
// A namespace on its own cannot tell a caller where the parent is, or whether it
// is one the caller may read — and that second distinction is the whole boundary
// (dec-0011).
//
// The commented-out example is in the fixture on purpose: it is the shape
// .dira/config.toml actually ships, and a reader that counted it would report a
// parent ledger nobody configured.
func TestParentsAreDeclarationsNotNames(t *testing.T) {
	t.Parallel()

	const input = `[parents]
# tirith = { path = "../tirith", ref = "main" }
sire = { path = "../x", ref = "main" }
me   = { visibility = "private", label = "the maintainer's own ledger" }
`
	cfg, err := config.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []config.Parent{
		{Name: "sire", Path: "../x", Ref: "main"},
		{Name: "me", Visibility: "private", Label: "the maintainer's own ledger"},
	}
	if len(cfg.ParentDecls) != len(want) {
		t.Fatalf("got %d declarations %+v, want %d — in file order, and a commented line is not one of them",
			len(cfg.ParentDecls), cfg.ParentDecls, len(want))
	}
	for i, w := range want {
		if got := cfg.ParentDecls[i]; got != w {
			t.Errorf("declaration %d = %+v, want %+v", i, got, w)
		}
	}

	// Stated separately from the struct comparison above, because these two
	// are what a caller acts on and an absence is easy to satisfy by accident.
	if cfg.ParentDecls[1].Path != "" {
		t.Errorf("me declares no path, but got %q", cfg.ParentDecls[1].Path)
	}
	if !cfg.ParentDecls[1].Private() {
		t.Error(`me is declared visibility = "private" and must read as private`)
	}
	if cfg.ParentDecls[0].Private() {
		t.Error("sire declares no visibility and must not read as private")
	}

	// The name-only view is derived from the declarations, so it cannot drift
	// from them: same names, same order.
	if !slices.Equal(cfg.Parents, []string{"sire", "me"}) {
		t.Errorf("parents = %v, want [sire me]", cfg.Parents)
	}
}

// TestACommentedParentIsNeverADeclaration is the negative half of the case
// above, asserted on its own so it cannot pass by the file having been read
// wrongly in some other way.
func TestACommentedParentIsNeverADeclaration(t *testing.T) {
	t.Parallel()

	const input = `[parents]
# sire = { path = "../sire", ref = "main" }
   #me = { visibility = "private" }
`
	cfg, err := config.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.ParentDecls) != 0 || len(cfg.Parents) != 0 {
		t.Errorf("declarations = %+v, names = %v; want neither — every line is commented out",
			cfg.ParentDecls, cfg.Parents)
	}
}

// TestParentFieldsSurviveWhatTheyContain. A label is prose a human wrote, so it
// can hold the two characters this reader has to be careful about.
func TestParentFieldsSurviveWhatTheyContain(t *testing.T) {
	t.Parallel()

	const input = `[parents]
sire = { path = "../x", label = "sire, the workspace # 2", flavour = "vanilla" } # a trailing note
empty = { }
`
	cfg, err := config.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []config.Parent{
		{Name: "sire", Path: "../x", Label: "sire, the workspace # 2"},
		{Name: "empty"},
	}
	if !slices.Equal(cfg.ParentDecls, want) {
		t.Errorf("declarations = %+v, want %+v — a comma and a hash inside quotes are content, and a key dira does not read is skipped rather than rejected",
			cfg.ParentDecls, want)
	}
}

// TestAParentDiraCannotReadIsReportedRatherThanGuessedAt holds the package's
// existing error contract over the new shape: Parse still returns a usable
// Config, and still says what it could not make sense of.
//
// The namespace survives every one of these. A declaration dropped because its
// value was malformed would turn every ref through it into an
// undeclared-namespace error (dec-0011), which is a typo in the config file
// punishing the entries.
func TestAParentDiraCannotReadIsReportedRatherThanGuessedAt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr string
		want    config.Parent
	}{
		{
			name:    "a bare string is not a declaration",
			input:   "[parents]\nsire = \"../sire\"\n",
			wantErr: "parents.sire is not written as a { … } declaration",
			want:    config.Parent{Name: "sire"},
		},
		{
			name:    "an unclosed table is not guessed at",
			input:   "[parents]\nsire = { path = \"../sire\"\n",
			wantErr: "parents.sire is not written as a { … } declaration",
			want:    config.Parent{Name: "sire"},
		},
		{
			name:    "a field that is not a pair is reported, and the rest of the line is still read",
			input:   "[parents]\nsire = { path = \"../sire\", main }\n",
			wantErr: "parents.sire has a field that is not a key = value pair",
			want:    config.Parent{Name: "sire", Path: "../sire"},
		},
		{
			name:    "a visibility dira does not read falls closed to private",
			input:   "[parents]\nme = { visibility = \"privat\" }\n",
			wantErr: "parents.me has a visibility dira does not read",
			want:    config.Parent{Name: "me", Visibility: "private"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Parse([]byte(tc.input))
			if err == nil {
				t.Fatalf("Parse returned no error, want one mentioning %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Parse error = %q, want one mentioning %q", err, tc.wantErr)
			}
			if !slices.Equal(cfg.ParentDecls, []config.Parent{tc.want}) {
				t.Errorf("declarations = %+v, want %+v — the namespace is declared even when the rest of the line is not readable",
					cfg.ParentDecls, []config.Parent{tc.want})
			}
		})
	}
}

// TestAnUnreadableVisibilityFallsClosed states the direction of the previous
// case as its own assertion, because it is a privacy property rather than a
// parsing one. Reading "privat" as public would let one typo publish a label
// dec-0011 says must never ship — scripts/privacy-lint.py P2's failure arriving
// at run time instead.
func TestAnUnreadableVisibilityFallsClosed(t *testing.T) {
	t.Parallel()

	cfg, _ := config.Parse([]byte("[parents]\nme = { visibility = \"PRIVATE\" }\n"))
	if len(cfg.ParentDecls) != 1 {
		t.Fatalf("declarations = %+v, want one", cfg.ParentDecls)
	}
	if !cfg.ParentDecls[0].Private() {
		t.Errorf("visibility = %q reads as not private; an unreadable visibility must fall closed",
			cfg.ParentDecls[0].Visibility)
	}

	// The other side of the same claim: a parent that really is public must
	// not be dragged private by a rule this blunt.
	cfg, err := config.Parse([]byte("[parents]\nsire = { visibility = \"public\" }\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.ParentDecls[0].Private() {
		t.Error(`visibility = "public" must read as public`)
	}
}

// TestNoParentValueEverReachesAnError. An error message is an output like any
// other: a caller prints it. The values on a [parents] line include `label`,
// which dec-0011 says must not ship for a private parent, so this reader names
// the key and the line and never the value — the same zero-occurrence shape the
// runtime leak checks use, applied to the one output this package produces.
func TestNoParentValueEverReachesAnError(t *testing.T) {
	t.Parallel()

	const sentinel = "SENTINEL-PRIVATE-LABEL"
	input := "[parents]\n" +
		"one = { visibility = \"" + sentinel + "-A\" }\n" +
		"two = \"" + sentinel + "-B\"\n" +
		"three = { label = \"" + sentinel + "-C\", " + sentinel + "-D }\n" +
		"three = { label = \"" + sentinel + "-E\" }\n"

	cfg, err := config.Parse([]byte(input))
	if err == nil {
		t.Fatal("Parse returned no error; this fixture is malformed four ways")
	}

	// Assert the reports are present before asserting the sentinel is absent,
	// or an empty error would satisfy the absence for free (L-0014's shape).
	for _, want := range []string{"parents.one", "parents.two", "parents.three"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %s; the absence below would then prove nothing", err, want)
		}
	}
	if n := strings.Count(err.Error(), sentinel); n != 0 {
		t.Errorf("a [parents] value reached an error message %d times: %v", n, err)
	}

	// It reaches the parsed declaration, which is where a caller reads it
	// from; the label is only forbidden from being printed.
	if got := cfg.ParentDecls[2].Label; got != sentinel+"-C" {
		t.Errorf("label = %q, want %q — the first declaration of a duplicated namespace is the one dira reads", got, sentinel+"-C")
	}
}

// TestADuplicateParentIsReportedAndTheFirstWins. Two declarations of one
// namespace and no rule for which wins; merging them would be a guess, and
// last-one-wins is the guess that can quietly drop a visibility.
func TestADuplicateParentIsReportedAndTheFirstWins(t *testing.T) {
	t.Parallel()

	const input = `[parents]
me = { path = "../me", visibility = "private" }
me = { path = "../elsewhere" }
`
	cfg, err := config.Parse([]byte(input))
	if err == nil || !strings.Contains(err.Error(), "declared twice") {
		t.Fatalf("Parse error = %v, want one reporting parents.me declared twice", err)
	}
	want := []config.Parent{{Name: "me", Path: "../me", Visibility: "private"}}
	if !slices.Equal(cfg.ParentDecls, want) {
		t.Errorf("declarations = %+v, want %+v", cfg.ParentDecls, want)
	}
	if !slices.Equal(cfg.Parents, []string{"me"}) {
		t.Errorf("parents = %v, want [me] — one namespace is one parent however many times it is written", cfg.Parents)
	}
}

// TestAValueDiraCannotReadIsNotSilentlyAccepted. The error is the whole reason
// Parse returns one: a caller degrades to the constitutional default and says
// so, and a reader who typed a ceiling that did nothing finds out.
func TestAValueDiraCannotReadIsNotSilentlyAccepted(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte("[brief]\nmax_tokens = 1_500\n"))
	if err == nil {
		t.Fatal("a max_tokens dira cannot read was accepted in silence")
	}
	if cfg.MaxTokens != 0 {
		t.Errorf("max_tokens = %d after a value that did not parse; the caller must apply its own default", cfg.MaxTokens)
	}
	if !strings.Contains(err.Error(), "config.toml") {
		t.Errorf("the error does not name the file it is about: %v", err)
	}
}
