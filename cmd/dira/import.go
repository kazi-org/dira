package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/importadr"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/local"
)

// `dira import <dir>` is E2-L7-T6: the first command in this repository to
// read a directory the user names rather than one dira owns (dec-0005's
// boundary — see internal/ledger/boundary_test.go's widened cmd/dira entry).
// Everything about what to do with what it reads lives in internal/importadr,
// which is why this file is short: walk the directory with os.ReadDir, hand
// the bytes to importadr.ScanDocument, print importadr.Summarize's report,
// read one confirmation, and perform whichever write the routed policy
// returned. internal/importadr itself imports neither os nor path/filepath —
// every function in it still takes bytes or another function's structured
// output.

const importName = "import"

const importSummary = "measure a directory of ADRs and offer to import or index them"

// importCommand is this verb's registry entry, mirroring installSkillCommand
// and runInstallSkill's own comment on why it exists as a function: a test
// can register it against its own app without reaching into package-level
// state, and the integrator's line in newApp's slice is the only thing that
// makes `dira import` reachable from the built binary (docs/lore.md L-0027).
//
//	{name: importName, summary: importSummary, run: runImport, usage: writeImportUsage},
func importCommand() *command {
	return &command{name: importName, summary: importSummary, run: runImport, usage: writeImportUsage}
}

func runImport(a *app, args []string) error {
	var target string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target, args = args[0], args[1:]
	}

	f := &importFlags{}
	fs := f.flagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeImportUsage(a.stdout)
			return nil
		}
		return &usageError{err: err, usage: writeImportUsage}
	}
	if rest := fs.Arg(0); rest != "" {
		return &usageError{
			err:   fmt.Errorf("import takes one directory argument, but got %q after the flags", rest),
			usage: writeImportUsage,
		}
	}
	if target == "" {
		return &usageError{
			err:   errors.New("import needs the directory to scan, e.g. `dira import ./adr`"),
			usage: writeImportUsage,
		}
	}

	docs, err := scanDirectory(target)
	if err != nil {
		return fmt.Errorf("reading %s: %w", target, err)
	}

	report := importadr.Summarize(docs)
	if _, err := io.WriteString(a.stdout, report.Text); err != nil {
		return err
	}

	confirmed := f.yes
	if !confirmed {
		confirmed, err = readConfirmation(a.stdin)
		if err != nil {
			return err
		}
	}

	store, diraDir, err := openLedger(f.dir)
	if err != nil {
		return err
	}
	ctx := context.Background()

	switch report.Verdict {
	case importadr.VerdictIndex:
		artifact, err := importadr.BuildIndexArtifact(report, confirmed)
		if err != nil {
			return err
		}
		if artifact == nil {
			return nil
		}
		return writeIndexArtifact(diraDir, a.now(), artifact)

	case importadr.VerdictImport:
		already, err := loadAlreadyImported(ctx, store)
		if err != nil {
			return err
		}
		drafts, err := importadr.BuildImportDrafts(report, confirmed, already)
		if err != nil {
			return err
		}
		now := a.now().UTC().Format(time.RFC3339)
		for _, draft := range drafts {
			draft.Created = now
			if err := ledger.Add(ctx, store, draft); err != nil {
				return fmt.Errorf("writing %s: %w", draft.Title, err)
			}
		}
	}
	return nil
}

// scanDirectory reads every ".md" file directly inside dir — no recursion,
// dec-0028's own "49 documents scanned" is a flat corpus and os.ReadDir is
// what a flat corpus needs — and scans each one. Sorted by name, so a repeat
// run over an unchanged directory produces entries in the same order.
func scanDirectory(dir string) ([]importadr.ScannedDocument, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	docs := make([]importadr.ScannedDocument, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		docs = append(docs, importadr.ScanDocument(name, data))
	}
	return docs, nil
}

// readConfirmation reads one line from stdin and reports whether it is a
// case-insensitive "y" or "yes". EOF (a closed or empty stdin) reads as
// declined, not as an error — a script that forgot to answer gets a no-op,
// not a hang or a panic.
func readConfirmation(stdin io.Reader) (bool, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading the confirmation from stdin: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// writeIndexArtifact is the one write "index instead" performs: one JSON
// file under .dira/cache/imports/, regenerable by re-running `dira import`
// over the same directory and never confused with a ledger entry.
func writeIndexArtifact(diraDir string, now time.Time, artifact *importadr.IndexArtifact) error {
	dir := filepath.Join(local.CacheDir(diraDir), "imports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the index artifact: %w", err)
	}
	name := fmt.Sprintf("index-%s.json", now.UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

// loadAlreadyImported reconstructs T5's idempotence exclusion set from the
// ledger itself, rather than from a second, parallel record of what has been
// imported: every existing entry whose source.hook is import carries its
// source document's path and sha256 in its excerpt
// (importadr.ParseImportExcerpt), and that is the whole of what a document
// key needs.
func loadAlreadyImported(ctx context.Context, store ledger.Store) (map[importadr.DocumentKey]bool, error) {
	infos, err := store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the ledger: %w", err)
	}
	already := make(map[importadr.DocumentKey]bool, len(infos))
	for _, info := range infos {
		entry, err := store.Get(ctx, info.ID)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", info.ID, err)
		}
		if entry.Source == nil || entry.Source.Hook != ledger.HookImport {
			continue
		}
		if key, ok := importadr.ParseImportExcerpt(entry.Source.Excerpt); ok {
			already[key] = true
		}
	}
	return already, nil
}

type importFlags struct {
	dir string
	yes bool
}

func (f *importFlags) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet(importName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.StringVar(&f.dir, "C", "", "run as if started in this directory (the ledger, not the scanned directory)")
	fs.BoolVar(&f.yes, "yes", false, "skip the confirmation prompt and confirm unconditionally")
	return fs
}

// writeImportUsage renders `dira import -h`.
func writeImportUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira " + importName + " - " + importSummary + "\n\n")
	b.WriteString("usage:\n\n")
	b.WriteString("\tdira import DIR              measure DIR, print a report, ask before writing\n")
	b.WriteString("\tdira import DIR --yes        skip the prompt and confirm unconditionally\n\n")

	b.WriteString("DIR is walked one level deep for *.md files (dec-0028). Each is measured for\n")
	b.WriteString("whether it records a rejected alternative with a reason. If none do, the report\n")
	b.WriteString("offers to index them instead — a manifest under .dira/cache/imports/, never a\n")
	b.WriteString("ledger entry. If at least one does, confirming imports one staged decision per\n")
	b.WriteString("reasoned document into .dira/entries/, for `dira distill` to dispose of\n")
	b.WriteString("individually. A directory already imported is skipped on a repeat run.\n\n")

	b.WriteString("flags:\n\n")
	for _, line := range [][2]string{
		{"--yes", "skip the confirmation prompt and confirm unconditionally"},
		{"-C DIR", "run as if started in this directory (the ledger, not the scanned directory)"},
	} {
		fmt.Fprintf(&b, "\t%-8s  %s\n", line[0], line[1])
	}

	_, _ = io.WriteString(w, b.String())
}
