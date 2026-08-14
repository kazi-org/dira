// Package ui serves dira's read-only surfaces — the ledger index and the
// decision page — as server-rendered HTML from the binary.
//
// # Why HTML from a template and not an SPA
//
// dec-0012. The rendered decision pages are dec-0010's long-tail acquisition
// surface, and a page a crawler cannot read is invisible to the one channel it
// exists to feed. On localhost the data is already in-process, so a JSON API
// exists only to serialise to yourself. Every page here renders complete with
// JavaScript disabled, and this package emits no <script> at all — asserted by
// TestNoScriptAnywhere rather than by intention.
//
// # One producer behind both renderers
//
// The decision page is rendered from a *why.Chain — the same structure
// `dira why` prints. docs/design/DESIGN.md makes that load-bearing by claiming
// the web chain is "the same output dira why prints in a terminal", and a claim
// like that survives exactly as long as there is one producer behind both
// renderers. This package therefore never walks the ledger to build a chain.
//
// # What it will not draw
//
// dec-0004 makes execution status a read-time join with kazi, and dira embeds no
// kazi client. Nothing here prints "converged", "in progress" or a predicate
// count. Where the join is unavailable the surfaces show dec-0004's ledger-side
// buckets and say the execution status is unavailable, rather than filling the
// gap with a guess.
//
// dec-0019 adds the general form of the same rule: the renderer derives
// presentation from recorded prose and never invents a field. There is no
// upheld-alternative card, because `alternatives` records the roads not taken
// and the chosen one is the ruling.
package ui

import (
	"embed"
	"fmt"
	"slices"
)

// Assets are the stylesheets and font faces served to the browser, embedded so
// `dira ui` works from any working directory and with the network unplugged
// (cst-0004, int-0002).
//
// They are copies. go:embed cannot reach outside its own package directory and
// docs/design/ is the single source of colour and spacing truth, so the binary
// needs its own copy of a file it must not be allowed to drift from.
// TestEmbeddedAssetsMatchTheDesignSource pins every one of them to its original
// in both directions — a copy nothing compares is drift with a delay on it.
//
// The fonts are here for the same reason and under the same pin. dec-0016 self-
// hosts TeX Gyre Pagella because the old system stack resolved to a different
// typeface on Linux than the one every measure in this design was tuned
// against; serving it from embed.FS rather than fetching it is what keeps
// dec-0012's offline guarantee intact.
//
//go:embed assets/*.css assets/fonts/*.woff2
var Assets embed.FS

// FontDir is where the embedded faces live, both inside Assets and on the
// wire. The two are the same string on purpose: the route is derived from the
// embedded tree rather than listed by hand, so a face cannot be added to the
// binary and left unrouted.
const FontDir = "assets/fonts"

// AssetSources maps each embedded asset to the file it is a copy of, relative
// to the repository root. The drift test walks this map, so adding an asset
// without adding its source here is caught by the test that asserts the map
// covers the whole directory.
var AssetSources = map[string]string{
	"assets/tokens.css":   "docs/design/tokens.css",
	"assets/decision.css": "docs/design/screens/decision.css",

	// index.css has no file of its own upstream: s2-index.html carries its
	// screen-specific rules in an inline <style> block, because it is one
	// screen and decision.css is shared by three. The drift test extracts
	// that block and compares it, so the served rules and the mockup's rules
	// are the same bytes either way.
	"assets/index.css": "docs/design/screens/s2-index.html#style",

	// distill.css is the same shape as index.css: s3-distill.html carries its
	// screen-specific rules in an inline <style> block rather than a file of
	// its own, because it is one screen (E6-L3-T2).
	"assets/distill.css": "docs/design/screens/s3-distill.html#style",

	// The fonts. Left of the arrow is a path inside this package; right of it
	// is a path from the repository root, and they read alike only because
	// the repository's font directory and the package's are named the same
	// thing. assets/fonts/ at the root is what NOTICE and its README describe
	// for the GUST Font Licence, so the licence text and the served bytes
	// cannot describe different files.
	"assets/fonts/pagella-regular.core.woff2": "assets/fonts/pagella-regular.core.woff2",
	"assets/fonts/pagella-italic.core.woff2":  "assets/fonts/pagella-italic.core.woff2",
	"assets/fonts/pagella-bold.core.woff2":    "assets/fonts/pagella-bold.core.woff2",
}

// Fonts lists the embedded faces by their name inside Assets, sorted. It reads
// the embedded tree rather than a hand-written list so that the routing table,
// the drift pin and the census check all count the same set.
func Fonts() ([]string, error) {
	entries, err := Assets.ReadDir(FontDir)
	if err != nil {
		return nil, fmt.Errorf("ui: listing embedded fonts: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, FontDir+"/"+e.Name())
	}
	slices.Sort(names)
	return names, nil
}

// asset reads one embedded file — a stylesheet or a font face.
//
// It calls embed.FS's own ReadFile rather than io/fs.ReadFile, and that is not a
// style choice: dec-0005 forbids anything above the storage backend from naming
// a path, enforced by TestNoFilesystemImportsAboveTheBackend, and `io/fs` is on
// that list. An embed.FS is a byte blob compiled into the binary rather than a
// filesystem, so the rule's reasoning does not reach it — but the mechanical
// check cannot tell the two apart, and buying an allowlist entry for a call that
// has a direct equivalent would spend the rule's credibility on nothing.
func asset(name string) ([]byte, error) {
	b, err := Assets.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("ui: reading embedded %s: %w", name, err)
	}
	return b, nil
}
