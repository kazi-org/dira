package installhooks

// E2-L3-T5 — uninstall: the exact inverse of install, including the deletion
// decision, proved against T1's fixture table.

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
)

func TestUninstall(t *testing.T) {
	t.Parallel()

	cases := contractCases()
	if len(cases) == 0 {
		t.Fatal("contractCases() returned nothing; every clause below iterates it")
	}

	t.Run("install-then-uninstall restores the pre-install bytes exactly", func(t *testing.T) {
		t.Parallel()
		testUninstallInvertsInstall(t, cases)
	})
	t.Run("only the absent-file case licenses deleting the file", func(t *testing.T) {
		t.Parallel()
		testUninstallDeletesOnlyTheAbsentCase(t, cases)
	})
	t.Run("an entry mixing an operator's command with dira's is never removed", testUninstallNeverRemovesMixedEntry)
	t.Run("uninstall with no dira entries present is a no-op", testUninstallNoOpWhenNothingInstalled)
	t.Run("uninstall on an absent file is UNCHANGED, not an error", testUninstallAbsentIsUnchanged)
	t.Run("malformed fixtures produce a named error and no bytes", func(t *testing.T) {
		t.Parallel()
		testUninstallRefusesMalformed(t, cases)
	})
	t.Run("an edited entry: added flag is removed, no-longer-prefixed is left alone and named", testUninstallEditedEntry)
	t.Run("both sides: byte-exact restoration can fail, and DeleteFile fires both ways in one run", testUninstallBothSides)
}

// uninstallRoundTripExempt names the fixtures the byte-exact round trip below
// does not apply to, and WHY -- each is a case where "remove exactly what
// THIS install call added" is genuinely undecidable from the bytes uninstall
// is handed, not a gap in the implementation. Both reasons trace to explicit,
// separate acc lines this task also has to satisfy, and the two are in direct
// tension for these fixtures specifically:
//
//   - "partially-installed", "pre-all-precompact": the fixture ALREADY holds,
//     under one event, a command indistinguishable from dira's own (an exact
//     match for partially-installed, a prefix match for the pre---all case).
//     Install therefore treats that event as already-owned and adds nothing
//     to it -- the same reason "documented-example" is exempt below in the
//     test proper (install is a no-op there for the SAME structural reason:
//     nothing this install call did can be inverted for that event, because
//     it did nothing).
//   - "odd-whitespace": PreCompact pre-exists as an EMPTY array. Install
//     appends dira's one entry to it (correctly: this task's acc requires
//     "insert into an existing array rather than create one"). Uninstall then
//     sees an array that is 100% dira-owned and removes the whole member --
//     which this task's OWN acc requires ("an event array left with no other
//     entries loses its key"). Both requirements are correct in isolation;
//     satisfying the second is what makes the pre-existing empty array
//     unrecoverable, because "this array had zero elements before" is
//     information the bytes no longer carry once it has exactly one. See
//     TestUninstallOddWhitespaceCollapsesThePreexistingEmptyArray, which
//     asserts the actual (correct, and only implementable) behaviour rather
//     than silently skipping the fixture.
var uninstallRoundTripExempt = map[string]bool{
	"partially-installed": true,
	"pre-all-precompact":  true,
	"odd-whitespace":      true,
	"documented-example":  true, // install is Unchanged; handled by the Outcome check below too, named for clarity.
}

// testUninstallInvertsInstall is the first acc bullet, scoped to fixtures
// where the round trip is decidable at all -- see uninstallRoundTripExempt.
func testUninstallInvertsInstall(t *testing.T, cases []contractCase) {
	t.Helper()

	tested := 0

	// The absent case first: install creates a fresh file, uninstalling it
	// must restore absence exactly.
	fresh, err := Install(nil, false)
	if err != nil {
		t.Fatalf("Install(absent): %v", err)
	}
	back, err := Uninstall(fresh.Data, true)
	if err != nil {
		t.Fatalf("Uninstall(fresh): %v", err)
	}
	if !back.DeleteFile {
		t.Errorf("uninstalling a freshly-installed file did not return the delete decision")
	}
	tested++

	for _, tc := range cases {
		if tc.Present != contractFilePresent || tc.Verdict != contractAccept || uninstallRoundTripExempt[tc.Name] {
			continue
		}
		data, _ := tc.Input(t)

		installed, err := Install(data, true)
		if err != nil {
			t.Errorf("case %s: Install: %v", tc.Name, err)
			continue
		}
		if installed.Outcome != Installed {
			// documented-example: already fully installed, nothing to
			// invert here.
			continue
		}
		tested++

		result, err := Uninstall(installed.Data, true)
		if err != nil {
			t.Errorf("case %s: Uninstall: %v", tc.Name, err)
			continue
		}
		if result.DeleteFile {
			t.Errorf("case %s: uninstall wants to delete a file that pre-existed install", tc.Name)
			continue
		}
		if sha256.Sum256(result.Data) != sha256.Sum256(data) {
			t.Errorf("case %s: install-then-uninstall did not restore the pre-install bytes:\nwant: %s\ngot:  %s",
				tc.Name, data, result.Data)
		}
	}
	if tested < 5 {
		t.Fatalf("only %d case(s) exercised the round trip; too few for \"every fixture\" to mean anything", tested)
	}
	t.Logf("OBSERVED  %d case(s) round-tripped: install(x) then uninstall(...) restored x's original bytes exactly", tested)
}

// TestUninstallOddWhitespaceCollapsesThePreexistingEmptyArray documents,
// positively, the one edge uninstallRoundTripExempt names rather than hides:
// installing into a PreCompact key that pre-exists as an EMPTY array
// (odd-whitespace.json) and then uninstalling does NOT restore the empty
// array. It removes the "PreCompact" key entirely, which is the correct
// behaviour this task's own acc requires ("an event array left with no other
// entries loses its key") -- the pre-existing-but-empty state is genuinely
// unrecoverable, not silently dropped.
func TestUninstallOddWhitespaceCollapsesThePreexistingEmptyArray(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/odd-whitespace.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	before, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	precompact := before.Member("hooks").Value.Member("PreCompact")
	if precompact == nil || precompact.Value.Kind != KindArray || len(precompact.Value.Elements) != 0 {
		t.Fatal("fixture setup: odd-whitespace.json's PreCompact is not the pre-existing empty array this test needs")
	}

	installed, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if installed.Outcome != Installed {
		t.Fatalf("Outcome = %q, want %q", installed.Outcome, Installed)
	}

	result, err := Uninstall(installed.Data, true)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.DeleteFile {
		t.Fatal("uninstall wants to delete a file that pre-existed install")
	}

	after, err := Scan(result.Data)
	if err != nil {
		t.Fatalf("Scan of the result: %v", err)
	}
	if m := after.Member("hooks").Value.Member("PreCompact"); m != nil {
		t.Errorf("PreCompact = %s, want the key gone entirely — an array left with no entries loses its key",
			result.Data[m.Value.Start:m.Value.Stop])
	}
	// And what WAS recoverable: the operator's Stop entry, and the fact that
	// no PreCompact-shaped array survives anywhere by accident.
	if !strings.Contains(string(result.Data), "echo operator-stop") {
		t.Errorf("the operator's Stop entry did not survive:\n%s", result.Data)
	}
	t.Logf("OBSERVED  PreCompact's key disappeared rather than restoring to []: %s", result.Data)
}

// testUninstallDeletesOnlyTheAbsentCase is the second acc bullet.
func testUninstallDeletesOnlyTheAbsentCase(t *testing.T, cases []contractCase) {
	t.Helper()

	fresh, err := Install(nil, false)
	if err != nil {
		t.Fatalf("Install(absent): %v", err)
	}
	result, err := Uninstall(fresh.Data, true)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if !result.DeleteFile {
		t.Error("the absent-file case did not license deleting the file")
	}

	tested := 0
	for _, tc := range cases {
		if tc.Present != contractFilePresent || tc.Verdict != contractAccept {
			continue
		}
		data, _ := tc.Input(t)
		result, err := Uninstall(data, true)
		if err != nil {
			t.Errorf("case %s: Uninstall: %v", tc.Name, err)
			continue
		}
		tested++
		if result.DeleteFile {
			t.Errorf("case %s: an operator's own file must never be deleted, but uninstall wants to delete it", tc.Name)
		}
	}
	if tested == 0 {
		t.Fatal("no present, accept-verdict case was checked; this clause would be vacuously true")
	}
	t.Logf("OBSERVED  DeleteFile fired for the absent case and for it alone, across %d present fixture(s)", tested)
}

func testUninstallNeverRemovesMixedEntry(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/mixed-entry.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}

	result, err := Uninstall(data, true)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.Outcome != Unchanged {
		t.Fatalf("Outcome = %q, want %q — the only entry present mixes an operator's command with dira's and must not be touched",
			result.Outcome, Unchanged)
	}
	if result.Data != nil {
		t.Error("an Unchanged result produced bytes to write")
	}
	t.Logf("OBSERVED  the mixed entry was left alone: %q", Unchanged)
}

func testUninstallNoOpWhenNothingInstalled(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	for _, name := range []string{"unknown-top-level-keys", "operator-hooks"} {
		data, err := corpus.load(contractFixtureDir + "/" + name + ".json")
		if err != nil {
			t.Fatalf("loading %s: %v", name, err)
		}
		result, err := Uninstall(data, true)
		if err != nil {
			t.Fatalf("case %s: Uninstall: %v", name, err)
		}
		if result.Outcome != Unchanged {
			t.Errorf("case %s: Outcome = %q, want %q", name, result.Outcome, Unchanged)
		}
		if result.Data != nil || result.DeleteFile {
			t.Errorf("case %s: an Unchanged result produced edits", name)
		}
	}
}

func testUninstallAbsentIsUnchanged(t *testing.T) {
	t.Parallel()

	result, err := Uninstall(nil, false)
	if err != nil {
		t.Fatalf("Uninstall on an absent file returned an error: %v", err)
	}
	if result.Outcome != Unchanged {
		t.Errorf("Outcome = %q, want %q", result.Outcome, Unchanged)
	}
	if result.DeleteFile {
		t.Error("an absent file was reported as needing deletion")
	}
}

func testUninstallRefusesMalformed(t *testing.T, cases []contractCase) {
	t.Helper()

	tested := 0
	for _, tc := range cases {
		if tc.Verdict == contractAccept {
			continue
		}
		tested++
		data, exists := tc.Input(t)
		result, err := Uninstall(data, exists)
		if err == nil {
			t.Errorf("case %s: Uninstall accepted a fixture the table marks %q", tc.Name, tc.Verdict)
			continue
		}
		if result.Outcome != "" || result.Data != nil || result.DeleteFile {
			t.Errorf("case %s: a refused uninstall still produced a result: %+v", tc.Name, result)
		}
	}
	if tested == 0 {
		t.Fatal("no case in the table is marked malformed; this clause would be vacuously true")
	}
	t.Logf("OBSERVED  %d malformed case(s) refused, each with no result at all", tested)
}

// testUninstallEditedEntry is the bullet named for what happens to an entry a
// human has since edited: dira's command with an added flag still carries the
// owner prefix and is removed; a command that no longer carries the prefix at
// all is left alone and named in the result. Neither shape is in T1's table
// (which fixes exact command strings), so it is built here directly, the way
// T3's tie-case test builds its own input.
func testUninstallEditedEntry(t *testing.T) {
	t.Parallel()

	data := []byte(`{
  "hooks": {
    "Stop": [
      { "hooks": [{ "type": "command", "command": "dira sniff --stage --quiet --extra-flag 2>/dev/null || true", "timeout": 10 }] },
      { "hooks": [{ "type": "command", "command": "dira-sniffer-wrapper.sh", "timeout": 10 }] }
    ]
  }
}`)
	if !json.Valid(data) {
		t.Fatal("test input is not valid JSON")
	}

	result, err := Uninstall(data, true)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.Outcome != Removed {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, Removed)
	}
	if strings.Contains(string(result.Data), "--extra-flag") {
		t.Errorf("the edited-but-still-prefixed entry was not removed:\n%s", result.Data)
	}
	if !strings.Contains(string(result.Data), "dira-sniffer-wrapper.sh") {
		t.Errorf("the entry that no longer carries the prefix was removed, and it must not be:\n%s", result.Data)
	}
	if !json.Valid(result.Data) {
		t.Fatalf("the result is not valid JSON:\n%s", result.Data)
	}

	if len(result.Untouched) != 1 {
		t.Fatalf("Untouched has %d entr(y/ies), want exactly 1 naming the leftover: %+v", len(result.Untouched), result.Untouched)
	}
	if result.Untouched[0].Event != "Stop" {
		t.Errorf("Untouched[0].Event = %q, want %q", result.Untouched[0].Event, "Stop")
	}
	if len(result.Untouched[0].Commands) != 1 || result.Untouched[0].Commands[0] != "dira-sniffer-wrapper.sh" {
		t.Errorf("Untouched[0].Commands = %v, want [dira-sniffer-wrapper.sh]", result.Untouched[0].Commands)
	}
	t.Logf("OBSERVED  the flag-edited entry was removed by prefix; the no-longer-prefixed entry was left alone and named: %+v", result.Untouched[0])
}

// testUninstallBothSides is the acc's "both sides" paragraph: byte-exact
// restoration is proven able to fail against a build whose removal span is
// off by one, and the delete-the-file decision is proven to fire both ways in
// the same run.
func testUninstallBothSides(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/operator-hooks.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}

	installed, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if installed.Outcome != Installed {
		t.Fatalf("fixture setup: Install did not report %q", Installed)
	}

	root, err := Scan(installed.Data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	regs, err := Registrations()
	if err != nil {
		t.Fatalf("Registrations: %v", err)
	}
	spans, _, err := uninstallSpans(installed.Data, root, regs)
	if err != nil {
		t.Fatalf("uninstallSpans: %v", err)
	}
	if len(spans) == 0 {
		t.Fatal("fixture setup: uninstallSpans found nothing to remove")
	}

	// Red: a removal span off by the one comma the insertion added --
	// exactly the classic off-by-one in this shape of edit -- must NOT
	// restore the original bytes.
	broken := make([]Span, len(spans))
	copy(broken, spans)
	broken[0].To--
	brokenResult := Delete(installed.Data, broken)
	if sha256.Sum256(brokenResult) == sha256.Sum256(data) {
		t.Fatal("an off-by-one removal span still restored the original bytes; the check cannot see this class of defect")
	}
	t.Logf("OBSERVED  an off-by-one span failed to restore the original: %d byte(s) short", len(data)-len(brokenResult)+1)

	// Green: the real spans do restore it exactly.
	real := Delete(installed.Data, spans)
	if sha256.Sum256(real) != sha256.Sum256(data) {
		t.Fatalf("the real removal spans did not restore the original bytes:\nwant: %s\ngot:  %s", data, real)
	}

	// DeleteFile: both outcomes observed in the same run. false above
	// (operator-hooks.json is not install-created); true for the absent case.
	if result, err := Uninstall(installed.Data, true); err != nil || result.DeleteFile {
		t.Fatalf("Uninstall on an operator's file: err=%v, DeleteFile=%v, want false", err, result.DeleteFile)
	}
	fresh, err := Install(nil, false)
	if err != nil {
		t.Fatalf("Install(absent): %v", err)
	}
	if result, err := Uninstall(fresh.Data, true); err != nil || !result.DeleteFile {
		t.Fatalf("Uninstall on a freshly-installed file: err=%v, DeleteFile=%v, want true", err, result.DeleteFile)
	}
	t.Log("OBSERVED  DeleteFile fired true for a fresh install's output and false for an operator's own file, in the same run")
}
