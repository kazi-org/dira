package local_test

import (
	"path/filepath"
	"testing"

	"github.com/kazi-org/dira/internal/ledger/local"
)

// TestName covers the display name a rendered surface puts in its breadcrumb.
//
// It lives in the backend because dec-0005 puts every path concept there:
// taking the last segment of a path is naming one, and a caller above
// ledger.Store that did it itself would be the leak
// TestNoFilesystemImportsAboveTheBackend exists to catch.
func TestName(t *testing.T) {
	t.Parallel()

	sep := string(filepath.Separator)
	cases := map[string]string{
		sep + filepath.Join("home", "me", "code", "dira", ".dira"): "dira",
		sep + filepath.Join("srv", "kazi", ".dira"):                "kazi",
		filepath.Join("relative", "path", "repo", ".dira"):         "repo",
	}
	for in, want := range cases {
		if got := local.Name(in); got != want {
			t.Errorf("Name(%q) = %q, want %q", in, got, want)
		}
	}

	// A ledger with nothing above it to name must produce a label rather
	// than an empty crumb, which would render as a stray separator.
	for _, in := range []string{".dira", sep + ".dira"} {
		if got := local.Name(in); got != "this ledger" {
			t.Errorf("Name(%q) = %q, want the neutral fallback", in, got)
		}
	}
}
