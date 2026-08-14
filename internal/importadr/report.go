package importadr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// report.go is E2-L7-T3: a pure function from T2's extraction output to the
// text a human reads and the routing decision that follows from it. No I/O,
// no confirmation prompt — cmd/dira/import.go (T6) is the only thing in this
// lane allowed to touch a real filesystem or read a real answer.

// Verdict is the outcome of measuring a corpus before touching the ledger:
// dec-0028's own words, "offers indexing when the yield is nothing" — read
// literally, the boundary is documents-with-a-reason equal to zero, not some
// higher floor.
type Verdict string

const (
	VerdictIndex  Verdict = "INDEX"
	VerdictImport Verdict = "IMPORT"
)

// ScannedDocument is one document a directory walk found, carrying its
// identity (Path, the natural key T5's idempotence check keys on alongside
// SHA256) and its title alongside what T2 extracted from it. Path and SHA256
// are opaque strings as far as this package is concerned — building one costs
// no filesystem import, because the bytes already arrived in the caller's
// hand (T6 read them to walk the directory in the first place).
type ScannedDocument struct {
	Path   string
	SHA256 string
	Title  string
	Document
}

// ScanDocument builds a ScannedDocument from one file's bytes and the path it
// came from. path is caller-supplied and never interpreted — this package
// does not know a filesystem exists.
func ScanDocument(path string, data []byte) ScannedDocument {
	sum := sha256.Sum256(data)
	text := string(data)
	return ScannedDocument{
		Path:     path,
		SHA256:   hex.EncodeToString(sum[:]),
		Title:    TitleOf(text),
		Document: Extract(text),
	}
}

// Report is what a dry run over a directory produces: the measured counts,
// the rendered text a human reads before confirming, the routing verdict that
// follows from the counts alone, and the documents themselves — T4 and T5
// read Documents to decide what to write; nothing here writes anything.
type Report struct {
	Documents           []ScannedDocument
	TotalDocuments      int
	DocumentsWithReason int
	TotalAlternatives   int
	Verdict             Verdict
	Text                string
}

// Summarize builds a Report from every document a directory walk scanned. The
// order of docs does not matter for the counts; Documents preserves whatever
// order the caller gave it.
func Summarize(docs []ScannedDocument) Report {
	r := Report{Documents: docs, TotalDocuments: len(docs)}
	for _, d := range docs {
		r.TotalAlternatives += len(d.Alternatives)
		if d.WithReason() {
			r.DocumentsWithReason++
		}
	}
	if r.DocumentsWithReason == 0 {
		r.Verdict = VerdictIndex
	} else {
		r.Verdict = VerdictImport
	}
	r.Text = renderReport(r)
	return r
}

// renderReport is dec-0028's own wording, byte-matched by T3's acceptance
// test rather than paraphrased. The three count lines are always present; the
// offer is only for the INDEX case, and the closing question for the IMPORT
// case is deliberately unpinned by the acc — this lane's own words, not a
// second literal contract.
func renderReport(r Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%d documents scanned\n", r.TotalDocuments)
	fmt.Fprintf(&b, "%d record a rejected option with a reason\n", r.DocumentsWithReason)
	fmt.Fprintf(&b, "%d reasons found\n", r.TotalAlternatives)

	switch r.Verdict {
	case VerdictIndex:
		b.WriteString("Importing these gives dira nothing it can enforce.\n")
		b.WriteString("Index them instead, so `dira why` can cite them without claiming them?\n")
	case VerdictImport:
		fmt.Fprintf(&b, "Import the %d entries that carry a reason?\n", r.DocumentsWithReason)
	}

	return b.String()
}
