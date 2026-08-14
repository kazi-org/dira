// Command gen is tapformula's thin invocation surface: it takes the same
// five fields Generate needs as flags and prints the rendered formula bytes
// to stdout. scripts/tap-bump.sh runs it via `go run` rather than
// re-implementing formula rendering in shell (docs/plan/tasks/E0-L5.md's
// E0-L5-T2 note: "the generator package decides its own invocation
// surface").
//
// This package imports neither "os" nor any of the other packages
// internal/ledger/boundary_test.go polices (dec-0005) — it reads its
// arguments through the "flag" package (which reaches os.Args internally,
// but that import is flag's, not this package's) and reports failure
// through "log".Fatal (same reasoning: log.Fatal calls os.Exit, but from
// within the log package). So this command needs no entry in
// boundary_test.go's allowlist, matching the constraint every new package in
// this lane was written under.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kazi-org/dira/internal/tapformula"
)

func main() {
	version := flag.String("version", "", "formula version, no leading v")
	darwinURL := flag.String("darwin-arm64-url", "", "darwin/arm64 archive URL")
	darwinSHA256 := flag.String("darwin-arm64-sha256", "", "darwin/arm64 archive sha256")
	linuxURL := flag.String("linux-amd64-url", "", "linux/amd64 archive URL")
	linuxSHA256 := flag.String("linux-amd64-sha256", "", "linux/amd64 archive sha256")
	flag.Parse()

	out, err := tapformula.Generate(tapformula.Input{
		Version: *version,
		DarwinArm64: tapformula.Target{
			URL:    *darwinURL,
			SHA256: *darwinSHA256,
		},
		LinuxAmd64: tapformula.Target{
			URL:    *linuxURL,
			SHA256: *linuxSHA256,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(out))
}
