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
