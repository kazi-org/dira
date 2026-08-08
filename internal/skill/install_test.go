package skill

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kazi-org/dira/skills"
)

// E2-L2-T8's acceptance, and the two things it is really asking.
//
// The first is that the installer is idempotent in the way an installer has to
// be: run it twice and the second run must say so rather than rewrite the file,
// and run it over something an operator edited and it must leave that alone.
// Those two look the same from outside — both "did not write" — which is why
// they are asserted as different reported outcomes and not as an absence of
// writes. An installer that refused on every second run would protect edits and
// would also be broken, and nobody would notice for months.
//
// The second is that none of this can happen anywhere except the directory it
// was handed. That is structural in install.go — the package imports no
// filesystem package, so it has no way to name a path — and it is measured here
// anyway, from the outside, with the real $HOME redirected under a temp dir and
// then walked. The walk is proven able to see a file before its emptiness is
// read as evidence.

// fixture is a document with the shape of a skill and none of its content. The
// installer compares bytes and never parses, so the real 6 KB document would
// make every assertion below harder to read and none of them stronger. The
// shipped document is installed for real in
// TestInstallSkillInstallsTheShippedDocument.
var fixture = []byte("---\nname: dira\ndescription: a fixture\n---\n\nthe body.\n")

func TestInstallSkill(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := newTempRoot(t, dir)
	installed := filepath.Join(dir, "skills", "dira", "SKILL.md")

	t.Run("a first install writes the file", func(t *testing.T) {
		got, err := Install(root, fixture, false)
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		if got != Installed {
			t.Fatalf("outcome = %q, want %q", got, Installed)
		}
		onDisk, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("the install reported %q but wrote no %s: %v", got, Path, err)
		}
		if !bytes.Equal(onDisk, fixture) {
			t.Errorf("%s does not carry the document that was installed", Path)
		}
		t.Logf("OBSERVED  %s wrote %d bytes to <root>/%s", got, len(onDisk), Path)
	})

	t.Run("a second install is a byte-level no-op reporting UNCHANGED", func(t *testing.T) {
		before := root.writes

		got, err := Install(root, fixture, false)
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		if got != Unchanged {
			t.Fatalf("outcome = %q, want %q", got, Unchanged)
		}
		// Byte-level no-op means no write happened, not that the bytes
		// happen to match afterwards — rewriting identical bytes would
		// satisfy a content comparison and still churn the operator's
		// file, its mtime and anything watching it.
		if root.writes != before {
			t.Errorf("the second install performed %d write(s); a no-op writes nothing", root.writes-before)
		}
		t.Logf("OBSERVED  %s, and %d writes to the root", got, root.writes-before)
	})

	t.Run("a locally modified file is left alone and reported", func(t *testing.T) {
		local := append(bytes.Clone(fixture), []byte("\nsomething the operator added.\n")...)
		if err := os.WriteFile(installed, local, 0o644); err != nil {
			t.Fatalf("editing the installed file: %v", err)
		}
		before := root.writes

		got, err := Install(root, fixture, false)
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		if got != Refused {
			t.Fatalf("outcome = %q, want %q", got, Refused)
		}
		if root.writes != before {
			t.Errorf("the refused install performed %d write(s)", root.writes-before)
		}
		onDisk, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("reading %s: %v", installed, err)
		}
		if !bytes.Equal(onDisk, local) {
			t.Errorf("the operator's edit did not survive the refusal")
		}
		// The outcome names the file the caller has to report, and Path
		// is the only file this package ever touches.
		if Path == "" || !strings.HasSuffix(installed, Path) {
			t.Errorf("Path = %q, which is not the file that was left alone (%s)", Path, installed)
		}
		t.Logf("OBSERVED  %s, %s left at %d bytes", got, Path, len(onDisk))
	})

	t.Run("force replaces it", func(t *testing.T) {
		got, err := Install(root, fixture, true)
		if err != nil {
			t.Fatalf("Install: %v", err)
		}
		if got != Installed {
			t.Fatalf("outcome = %q, want %q", got, Installed)
		}
		onDisk, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("reading %s: %v", installed, err)
		}
		if !bytes.Equal(onDisk, fixture) {
			t.Errorf("--force did not replace the modified file")
		}
		t.Logf("OBSERVED  %s under force, %d bytes", got, len(onDisk))
	})

	t.Run("an empty document is refused rather than installed", func(t *testing.T) {
		before := root.writes

		got, err := Install(root, []byte("   \n\t\n"), false)
		if !errors.Is(err, ErrEmpty) {
			t.Fatalf("Install of an empty document: outcome %q, err %v; want %v", got, err, ErrEmpty)
		}
		if root.writes != before {
			t.Errorf("the refusal still wrote %d time(s)", root.writes-before)
		}
		t.Logf("OBSERVED  refused: %v", err)
	})

	t.Run("no root is an error, never a write somewhere else", func(t *testing.T) {
		got, err := Install(nil, fixture, false)
		if !errors.Is(err, ErrNoRoot) {
			t.Fatalf("Install with no root: outcome %q, err %v; want %v", got, err, ErrNoRoot)
		}
		t.Logf("OBSERVED  refused: %v", err)
	})
}

// TestInstallSkillWritesNothingUnderHome is the acceptance clause that no code
// path in this package can write outside the injected directory.
//
// It runs every entry point the package has, in every branch it has, with $HOME
// pointed at a temp directory and the injected root deliberately somewhere else
// — so a stray os.UserHomeDir() anywhere below would land inside the walked
// tree rather than in the operator's real ~/.claude.
//
// The emptiness of that tree is only evidence once the walk has been seen to
// report something, so the second half writes one file into it and asserts the
// same walk finds it. A walk that reported nothing whatever happened would pass
// the first half and mean nothing, which is docs/lore.md L-0001 rule 2 in the
// form it actually shows up in.
func TestInstallSkillWritesNothingUnderHome(t *testing.T) {
	// Not parallel: it sets HOME for the process.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := t.TempDir()
	if strings.HasPrefix(dir, home) {
		t.Fatalf("the injected root %s is inside $HOME %s; this test would prove nothing", dir, home)
	}
	root := newTempRoot(t, dir)
	installed := filepath.Join(dir, "skills", "dira", "SKILL.md")

	// Fresh install, no-op re-install, refusal, forced replacement, and both
	// error paths.
	for _, step := range []func(){
		func() { _, _ = Install(root, fixture, false) },
		func() { _, _ = Install(root, fixture, false) },
		func() {
			if err := os.WriteFile(installed, append(bytes.Clone(fixture), 'x'), 0o644); err != nil {
				t.Fatalf("editing the installed file: %v", err)
			}
		},
		func() { _, _ = Install(root, fixture, false) },
		func() { _, _ = Install(root, fixture, true) },
		func() { _, _ = Install(root, nil, false) },
		func() { _, _ = Install(nil, fixture, false) },
	} {
		step()
	}
	if root.writes == 0 {
		t.Fatal("the installer wrote nothing at all, so an empty $HOME below would say nothing about where it writes")
	}

	if found := filesUnder(t, home); len(found) != 0 {
		t.Fatalf("%d file(s) appeared under $HOME after %d write(s) to the injected root:\n\t%s",
			len(found), root.writes, strings.Join(found, "\n\t"))
	}
	t.Logf("OBSERVED  %d write(s) to the injected root, 0 files under $HOME", root.writes)

	// The other half: the walk can see a file when there is one.
	control := filepath.Join(home, ".claude", "skills", "dira", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(control), 0o755); err != nil {
		t.Fatalf("creating the control directory: %v", err)
	}
	if err := os.WriteFile(control, fixture, 0o644); err != nil {
		t.Fatalf("writing the control file: %v", err)
	}
	found := filesUnder(t, home)
	if len(found) == 0 {
		t.Fatal("the walk reports an empty $HOME with a file in it; the emptiness asserted above measured nothing")
	}
	t.Logf("OBSERVED  the same walk finds %d file(s) once one is written there: %s", len(found), strings.Join(found, ", "))
}

// TestInstallSkillInstallsTheShippedDocument closes the loop between the bytes
// compiled into the binary and the file every other check in this repository
// reads. They are the same file, so this cannot drift — it asserts that, rather
// than trusting it, because a go:embed pattern that stopped matching would
// compile to an empty variable and install an empty skill.
func TestInstallSkillInstallsTheShippedDocument(t *testing.T) {
	t.Parallel()

	onDisk, err := os.ReadFile(SkillPath)
	if err != nil {
		t.Fatalf("reading %s: %v", SkillPath, err)
	}
	if len(skills.Dira) == 0 {
		t.Fatal("the embedded skill is empty; go:embed matched nothing")
	}
	if !bytes.Equal(skills.Dira, onDisk) {
		t.Fatalf("the embedded skill is %d bytes and %s is %d; they are meant to be the same file",
			len(skills.Dira), SkillPath, len(onDisk))
	}

	dir := t.TempDir()
	root := newTempRoot(t, dir)
	got, err := Install(root, skills.Dira, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got != Installed {
		t.Fatalf("outcome = %q, want %q", got, Installed)
	}
	written, err := os.ReadFile(filepath.Join(dir, "skills", "dira", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading the installed skill: %v", err)
	}
	if !bytes.Equal(written, onDisk) {
		t.Error("the installed file is not the document this repository ships")
	}
	t.Logf("OBSERVED  installed %d bytes, byte-identical to %s", len(written), SkillPath)
}

// ---- the test's own Root ----------------------------------------------------

// tempRoot is a Root over a real directory, and it counts its writes so a
// no-op can be told from a rewrite of identical bytes.
//
// It is a second implementation of the same two methods cmd/dira implements,
// and deliberately so: cmd/dira's lives in package main and cannot be imported,
// and a fixture that shared code with the thing it measures would stop being a
// fixture. Both are backed by *os.Root, which is what makes the confinement
// real rather than a matter of how carefully the names are built.
type tempRoot struct {
	root   *os.Root
	writes int
}

func newTempRoot(t *testing.T, dir string) *tempRoot {
	t.Helper()

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("opening %s as a root: %v", dir, err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return &tempRoot{root: root}
}

func (r *tempRoot) ReadFile(name string) ([]byte, bool, error) {
	data, err := r.root.ReadFile(name)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return data, true, nil
}

func (r *tempRoot) WriteFile(name string, data []byte) error {
	r.writes++
	if at := strings.LastIndex(name, "/"); at > 0 {
		if err := r.root.MkdirAll(name[:at], 0o755); err != nil {
			return err
		}
	}
	return r.root.WriteFile(name, data, 0o644)
}

// filesUnder returns every file below dir, directories excluded, as paths
// relative to dir. A missing dir is no files rather than an error: the question
// asked is what was created, and nothing created is the answer either way.
func filesUnder(t *testing.T, dir string) []string {
	t.Helper()

	var found []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		found = append(found, rel)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return found
}
