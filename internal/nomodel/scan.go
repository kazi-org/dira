// Package nomodel turns dec-0003 into something a build can check.
//
// dec-0003 says the binary contains no model client, and cst-0004 says dira
// never requires a network service, an account or a hosted tier. Both were
// prose. The change that breaks them will not arrive announcing itself — it
// arrives as "just add a fallback extractor so `--deep` works without an agent",
// which is a reasonable-sounding sentence that quietly imports a provider SDK
// and reads an API key out of the environment. This package is the check that
// makes that diff fail.
//
// Two independent questions, deliberately kept apart:
//
//  1. Does the command path LINK a model-provider SDK? Answered from
//     `go list -deps`, which is the linker's own answer rather than a grep's.
//  2. Does the source NAME an API key environment variable? Answered by
//     scanning the module's files, because a hand-rolled client over net/http
//     links no SDK at all and the key name is the thing it cannot avoid.
//
// # What this deliberately does not check, so nobody mistakes its scope
//
// **It does not denylist stdlib network packages, and must not.** `cmd/dira`
// legitimately links `net`, `net/http` and `crypto/tls` today, reached through
// `internal/ui`, which serves the read-only localhost UI. A check phrased as
// "no network packages linked" would be RED on a correct binary — docs/lore.md
// L-0001 rule 2 in its purest form, a gate that cannot pass the case it exists
// to certify. dec-0003 forbids a model client, not a socket. Whether the
// process actually opens an outbound connection is a runtime question and it is
// E1-L6-T3's, measured by observing the process rather than by reading its
// imports. TestStdlibNetworkingIsLinkedAndNotDenied pins that boundary so a
// later tightening has to argue with a failing test rather than with a comment.
//
// It is also a NAME-based heuristic on the linkage side. A determined
// hand-written client using only `net/http` and a literal URL links no
// denylisted module and would pass check 1. Check 2 catches the realistic
// version of that (it still needs a key from somewhere), and the honest
// statement is that this package raises the cost of the accidental regression
// rather than defeating a deliberate one.
//
// # Why nothing here touches the filesystem
//
// This file is pure: the denylist, the patterns, the exclusion lists and the
// predicates over them. Enumerating files and reading them happens in
// nomodel_test.go, which is this package's only caller. That split is not a
// testing affectation — internal/ledger/boundary_test.go enforces dec-0005 by
// refusing `os`, `io/fs`, `path` and `path/filepath` in any non-test package
// above the storage backend. dec-0005 is about the ledger and this package is
// not, but the rule is mechanical and it is right to be; complying with it by
// design costs nothing here and leaves the reviewable policy in one small file.
package nomodel

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// check 1 — linked modules
// ---------------------------------------------------------------------------

// A Vendor is one model provider, identified by a token that appears in the
// module paths of that provider's Go SDKs.
//
// Matching is on TOKENS, not raw substrings. A module path is split on `/`, `.`,
// `-` and `_`, and a token matches when it has the vendor's token as a prefix —
// so `anthropics` (the GitHub org) and `anthropic-sdk-go` (the repo) both match
// `anthropic`, while `github.com/google/uuid` matches nothing. A raw
// `strings.Contains` would be the tempting version and it is the one that
// eventually flags an innocent dependency and gets deleted for being noisy.
type Vendor struct {
	// Token is lowercase, and is matched as a prefix of a path token.
	Token string
	// Reason names the concrete module(s) this exists to keep out, so the
	// failure message tells the reader what they just imported.
	Reason string
}

// ModelVendors is the denylist. It covers the providers dec-0003's rejected
// alternative names — "embed an LLM client" — in the form that alternative would
// actually arrive: as a `go get`.
//
// This list is MODULE-scoped on purpose. See the package comment for why a
// stdlib-scoped version would be red on a correct binary.
var ModelVendors = []Vendor{
	{Token: "anthropic", Reason: "Anthropic's Go SDK (github.com/anthropics/anthropic-sdk-go)"},
	{Token: "openai", Reason: "OpenAI clients (github.com/openai/openai-go, github.com/sashabaranov/go-openai)"},
	{Token: "genai", Reason: "Google's GenAI SDK (google.golang.org/genai)"},
	{Token: "generative", Reason: "Google's predecessor SDK (github.com/google/generative-ai-go)"},
	{Token: "cohere", Reason: "Cohere's Go SDK (github.com/cohere-ai/cohere-go)"},
	{Token: "mistral", Reason: "Mistral clients (github.com/mistralai/..., github.com/gage-technologies/mistral-go)"},
	{Token: "ollama", Reason: "Ollama's Go client (github.com/ollama/ollama/api)"},
	{Token: "bedrock", Reason: "AWS Bedrock runtime clients, which are model clients wearing a cloud SDK's name"},
	{Token: "vertexai", Reason: "Google Vertex AI clients"},
}

// sdkMarkers are the naming conventions a vendored cloud SDK uses. A module is
// denied when it carries one of these AND a token naming a provider host, which
// is how `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` gets caught even
// though the interesting word is buried three path elements deep.
var sdkMarkers = []string{"sdk-go", "sdk-for-go"}

// sdkHosts are the vendors whose broad cloud SDKs ship a model client inside
// them. Deliberately narrow: this rule only fires together with an sdkMarker,
// so `github.com/google/uuid` is not in scope no matter how `google` is spelled.
var sdkHosts = []string{"aws", "amazonaws", "azure", "microsoft", "googleapis", "google", "vertexai", "bedrock"}

// A ModuleViolation is one denied module and the packages that dragged it in.
type ModuleViolation struct {
	Module   string
	Reason   string
	Packages []string
}

func (v ModuleViolation) String() string {
	s := fmt.Sprintf("%s — %s", v.Module, v.Reason)
	if len(v.Packages) > 0 {
		s += "\n\treached through:\n\t\t" + strings.Join(v.Packages, "\n\t\t")
	}
	return s
}

// ErrNothingScanned is returned when a check was handed an empty input set.
//
// This is the whole of docs/lore.md L-0001 rule 1 in one sentinel: a filter over
// zero modules reports zero violations, and a scan over zero files reports zero
// findings, and both look exactly like a clean repository. Callers must treat it
// as a failure of the check, never as a pass.
var ErrNothingScanned = errors.New("nomodel: nothing was scanned, so the check measured nothing")

// ErrSkipFile is what ScanFiles' read function returns for a path that is listed
// but is not there — a file deleted and not yet staged, which an enumerator over
// git's index will still name. Such a file is not counted as scanned, because a
// file that was never read must not inflate the number that proves the check
// measured something.
var ErrSkipFile = errors.New("nomodel: file is listed but absent")

// DeniedModules reports every module in modules that names a model provider.
//
// modules is keyed by module path, valued by the packages that reach it — the
// shape `go list -deps` is easiest to reduce to, and the shape whose failure
// message can say how the dependency arrived.
//
// It returns ErrNothingScanned on an empty map rather than an empty result.
func DeniedModules(modules map[string][]string) ([]ModuleViolation, error) {
	if len(modules) == 0 {
		return nil, fmt.Errorf("%w: no modules were listed", ErrNothingScanned)
	}

	var out []ModuleViolation
	for module, packages := range modules {
		if reason, denied := DenyModule(module); denied {
			pkgs := append([]string(nil), packages...)
			sort.Strings(pkgs)
			out = append(out, ModuleViolation{Module: module, Reason: reason, Packages: pkgs})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Module < out[j].Module })
	return out, nil
}

// DenyModule answers the denylist question for one module path, and says why.
func DenyModule(module string) (reason string, denied bool) {
	tokens := pathTokens(module)

	for _, vendor := range ModelVendors {
		for _, token := range tokens {
			if strings.HasPrefix(token, vendor.Token) {
				return vendor.Reason, true
			}
		}
	}

	lower := strings.ToLower(module)
	for _, marker := range sdkMarkers {
		if !strings.Contains(lower, marker) {
			continue
		}
		for _, host := range sdkHosts {
			for _, token := range tokens {
				if strings.HasPrefix(token, host) {
					return fmt.Sprintf("a %q SDK under the provider host %q, which is how a model client arrives disguised as cloud plumbing", marker, host), true
				}
			}
		}
	}
	return "", false
}

// pathTokens splits a module path into lowercase word tokens.
func pathTokens(module string) []string {
	return strings.FieldsFunc(strings.ToLower(module), func(r rune) bool {
		return r == '/' || r == '.' || r == '-' || r == '_'
	})
}

// ---------------------------------------------------------------------------
// check 2 — API key names in the source
// ---------------------------------------------------------------------------

// apiKeyName matches an API-key environment variable name: `ANTHROPIC_API_KEY`,
// `OPENAI_API_KEY`, a bare `API_KEY`, and anything else of that shape.
//
// The pattern is on the NAME, not on a key's value. A key value is a random
// string and any pattern for one is either leaky or noisy; the variable name is
// the part a client cannot avoid writing down, and it is the part that survives
// into a diff.
var apiKeyName = regexp.MustCompile(`\b[A-Z0-9]*(?:_[A-Z0-9]+)*_?API_KEY\b`)

// apiKeyMarker is the substring every match of apiKeyName necessarily contains.
//
// It is a prefilter, not a second rule: a byte scan for it is a necessary
// condition for a match, so skipping the regex where it is absent cannot change
// any answer. It is here because the scan covers every file in the module and
// this repository carries ~76MB of tracked PNG renders under docs/design/; the
// prefilter takes that pass from ~10s to well under one, without narrowing what
// is looked at. Deliberately NOT an extension or content-type filter — "skip the
// binaries" is how a key in an unexpected file stops being found.
var apiKeyMarker = []byte("API_KEY")

// An Exclusion is one file that is allowed to contain an API-key name, with the
// reason it is allowed to.
//
// This is a NAMED LIST OF PATHS, deliberately, and not a cleverer scheme.
// E8-L5-T7 hit exactly this problem — a scanner that flags its own source — and
// tried a string-splitting trick first ("API" + "_KEY") so the literal would not
// appear. It failed the same run: the trick makes the scanner's own source
// unsearchable for the thing it looks for, so the next person cannot grep for
// the rule, and it silently stops working the moment the pattern is edited. A
// path list is boring, it is greppable, and TestExclusionsAreNotStale keeps it
// from rotting into a list of what was once convenient.
type Exclusion struct {
	Path   string
	Reason string
}

// APIKeyExclusions is the complete set of files permitted to name an API key.
//
// Note what is NOT here and does not need to be: `_test.go` files are excluded
// structurally by ScanFiles, because a test that proves redaction works has to
// name the thing it redacts.
var APIKeyExclusions = []Exclusion{
	{
		Path: "internal/nomodel/scan.go",
		Reason: "the scanner's own source. It has to contain the pattern it looks for, " +
			"and hiding that behind string concatenation is the trick E8-L5-T7 already proved does not work.",
	},
	{
		Path: "internal/sniff/testdata/transcripts/with-credentials.jsonl",
		Reason: "the redaction fixture. It carries a leaked key on purpose so internal/sniff " +
			"can prove the excerpt path strips it (see internal/sniff/redact.go); a fixture " +
			"with nothing to redact would certify nothing.",
	},
	{
		Path:   "docs/plan/tasks/E2-L2.md",
		Reason: "this check's own specification, which quotes the variable names it denies.",
	},
	{
		Path:   "docs/plan/prompts/L2-E2-L2.md",
		Reason: "the lane prompt the specification was decomposed from, for the same reason.",
	},
}

// A FileViolation is one file naming one or more API-key variables.
type FileViolation struct {
	// Path is relative to the module root, slash-separated.
	Path  string
	Names []string
}

func (v FileViolation) String() string {
	return fmt.Sprintf("%s names %s", v.Path, strings.Join(v.Names, ", "))
}

// APIKeyNames returns every API-key variable name appearing in content, in
// first-appearance order and without duplicates.
func APIKeyNames(content string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range apiKeyName.FindAllString(content, -1) {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// ScanFiles applies the API-key rule to paths, reading each one through read.
//
// The I/O is injected rather than performed here, and that is not a testing
// affectation: internal/ledger/boundary_test.go enforces dec-0005 by refusing
// any non-test package above the storage backend the imports `os`, `io/fs`,
// `path` or `path/filepath`. dec-0005 is about the ledger and this package is
// not, but the rule is mechanical and it is right to be — so the policy lives
// here, where it is small enough to review in a diff, and the enumerator and the
// file reads live in nomodel_test.go, which is where this package's only caller
// is anyway.
//
// paths are slash-separated and relative to the module root. Two kinds are
// skipped: `_test.go` files, because a test that proves redaction works has to
// name the thing it redacts; and APIKeyExclusions, by exact path.
//
// It returns the violations and the number of files actually read, and
// ErrNothingScanned when that number is zero. A caller that ignores the count
// has written a check that passes on an empty tree, which is the failure mode
// this whole package exists to prevent.
func ScanFiles(paths []string, read func(path string) ([]byte, error)) (violations []FileViolation, scanned int, err error) {
	excluded := map[string]bool{}
	for _, e := range APIKeyExclusions {
		excluded[e.Path] = true
	}

	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || excluded[path] {
			continue
		}
		content, err := read(path)
		if errors.Is(err, ErrSkipFile) {
			continue
		}
		if err != nil {
			return nil, scanned, fmt.Errorf("reading %s: %w", path, err)
		}
		scanned++
		if !bytes.Contains(content, apiKeyMarker) {
			continue
		}
		if names := APIKeyNames(string(content)); len(names) > 0 {
			violations = append(violations, FileViolation{Path: path, Names: names})
		}
	}

	if scanned == 0 {
		return nil, 0, fmt.Errorf("%w: no files were read", ErrNothingScanned)
	}
	sort.Slice(violations, func(i, j int) bool { return violations[i].Path < violations[j].Path })
	return violations, scanned, nil
}

// SkippedDirs are the directory names the enumerator never descends into, with
// the reason each one is not this module's source.
//
// A directory carrying its own `go.mod` is skipped too, by rule rather than by
// name — see InModule.
var SkippedDirs = []Exclusion{
	{
		Path:   ".git",
		Reason: "object storage, not source. Every blob in history would be scanned, including ones already deleted.",
	},
	{
		Path: ".claude",
		Reason: "the agent harness's own directory. Never tracked, never shipped, and in the primary checkout " +
			"it holds `.claude/worktrees/<id>/` — a full checkout of this repository per running session, " +
			"which a naive walk would report as this module's files.",
	},
	{
		Path:   "node_modules",
		Reason: "third-party JavaScript for the mockup render harness; gitignored, and not Go source at all.",
	},
}

// InModule reports whether the slash-separated path rel belongs to the module,
// asking hasGoMod whether a given ancestor directory carries its own go.mod.
//
// The nested-module rule matters here for a concrete reason rather than
// tidiness: this repository's agent worktrees live at `.claude/worktrees/<id>/`,
// each a full untracked checkout of the repo. Without it, a scan run in the
// primary checkout reports every other session's files as this module's.
func InModule(rel string, hasGoMod func(dir string) bool) bool {
	parts := strings.Split(rel, "/")
	for i := 0; i < len(parts)-1; i++ {
		for _, skip := range SkippedDirs {
			if parts[i] == skip.Path {
				return false
			}
		}
		if hasGoMod(strings.Join(parts[:i+1], "/")) {
			return false
		}
	}
	return true
}
