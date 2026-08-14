package installhooks

// E2-L3-T3 — the byte-span JSON scanner and splicer, proved against T1's
// fixture table.
//
// IMPORT NOTE (dec-0005): this file may import os and path/filepath freely —
// internal/ledger/boundary_test.go reads go list's non-test .Imports, so a
// *_test.go file touching the filesystem does not put installhooks on any
// allowlist. spans.go itself imports neither.

import (
	"crypto/sha256"
	"encoding/json"
	"testing"
)

// TestSpans is T3's acceptance, iterating T1's table.
func TestSpans(t *testing.T) {
	t.Parallel()

	cases := contractCases()
	if len(cases) == 0 {
		t.Fatal("contractCases() returned nothing; every clause below iterates it")
	}

	t.Run("valid fixtures round-trip through an empty edit set", func(t *testing.T) {
		t.Parallel()
		testValidFixturesRoundTrip(t, cases)
	})
	t.Run("malformed fixtures refuse with a named error and no tree", func(t *testing.T) {
		t.Parallel()
		testMalformedFixturesRefuse(t, cases)
	})
	t.Run("a single insertion changes exactly the intended range", testSingleInsertionExact)
	t.Run("delete then reinsert the same text at the same offset is the identity", testDeleteReinsertIdentity)
	t.Run("multiple insertions apply back-to-front", testMultipleInsertionsBackToFront)
	t.Run("the wrong emitter fails the round trip the span approach exists to defeat", testWrongEmitterFailsRoundTrip)
}

// testValidFixturesRoundTrip is the first acc bullet: for every fixture the
// table marks valid, scanning and re-emitting with an EMPTY edit set returns
// bytes with a sha256 equal to the input.
func testValidFixturesRoundTrip(t *testing.T, cases []contractCase) {
	t.Helper()

	validCount := 0
	for _, tc := range cases {
		if tc.Present != contractFilePresent || tc.Verdict != contractAccept {
			continue
		}
		validCount++

		data, _ := tc.Input(t)
		root, err := Scan(data)
		if err != nil {
			t.Errorf("case %s: Scan failed on a fixture the table marks valid: %v", tc.Name, err)
			continue
		}
		if root == nil {
			t.Errorf("case %s: Scan returned no tree for a fixture it did not refuse", tc.Name)
			continue
		}

		got := Insert(data, nil)
		if sha256.Sum256(got) != sha256.Sum256(data) {
			t.Errorf("case %s: re-emitting with an empty edit set changed the bytes", tc.Name)
		}
	}

	// The identity assertion above is trivially true of an empty set of valid
	// fixtures — L-0001's rule 1, restated here as T3's own acc line demands.
	if validCount < 5 {
		t.Fatalf("only %d valid fixture(s) exercised the round trip; at least 5 are required "+
			"or \"every fixture round-trips\" is trivially true of a near-empty set", validCount)
	}
	t.Logf("OBSERVED  %d valid fixtures round-tripped byte-identically through Scan + Insert(data, nil)", validCount)
}

// testMalformedFixturesRefuse is the second acc bullet: every fixture the
// table marks malformed — bad bytes, a non-object root, a non-object "hooks"
// value — returns a named error and no span tree.
func testMalformedFixturesRefuse(t *testing.T, cases []contractCase) {
	t.Helper()

	malformedCount := 0
	for _, tc := range cases {
		if tc.Verdict == contractAccept {
			continue
		}
		malformedCount++

		data, exists := tc.Input(t)
		if !exists {
			// The absent case's Verdict is contractAccept in T1's table
			// (install creates the file), so this branch is not expected
			// to be reached; guarded rather than assumed.
			t.Fatalf("case %s: declared malformed but has no bytes to scan", tc.Name)
		}

		root, err := Scan(data)
		if err == nil {
			t.Errorf("case %s: Scan accepted a fixture the table marks %q", tc.Name, tc.Verdict)
			continue
		}
		if root != nil {
			t.Errorf("case %s: Scan returned a non-nil tree alongside an error", tc.Name)
		}
		t.Logf("OBSERVED  case %s refused: %v", tc.Name, err)
	}

	if malformedCount == 0 {
		t.Fatal("no case in the table is marked malformed; this clause would be vacuously true")
	}
}

// testSingleInsertionExact is the third bullet: a single insertion at a
// computed offset changes exactly the intended byte range, asserted by
// comparing the prefix and suffix around the offset rather than by
// eyeballing the output.
func testSingleInsertionExact(t *testing.T) {
	t.Parallel()

	data, err := realContractCorpus(t).load(contractFixtureDir + "/operator-hooks.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	root, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Insert a new top-level member right after the last one, mirroring the
	// shape install.go uses: at the last member's value's Stop.
	last := root.Members[len(root.Members)-1]
	at := last.Value.Stop
	text := []byte(`,"e2-l3-t3":true`)

	got := Insert(data, []Insertion{{At: at, Text: text}})

	if string(got[:at]) != string(data[:at]) {
		t.Errorf("the prefix before the insertion point changed")
	}
	if string(got[at:at+len(text)]) != string(text) {
		t.Errorf("the inserted range does not hold exactly the inserted text: %q", got[at:at+len(text)])
	}
	if string(got[at+len(text):]) != string(data[at:]) {
		t.Errorf("the suffix after the insertion point changed")
	}
	if len(got) != len(data)+len(text) {
		t.Errorf("len(got) = %d, want %d", len(got), len(data)+len(text))
	}

	// And the result is itself still valid, editable JSON.
	if !json.Valid(got) {
		t.Fatalf("the spliced result is not valid JSON:\n%s", got)
	}
	t.Logf("OBSERVED  inserted %d bytes at offset %d; prefix and suffix both preserved exactly", len(text), at)
}

// testDeleteReinsertIdentity is the fourth bullet: a span deletion followed by
// re-insertion of the same text at the same offset is the identity.
func testDeleteReinsertIdentity(t *testing.T) {
	t.Parallel()

	data, err := realContractCorpus(t).load(contractFixtureDir + "/operator-hooks.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	root, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// The last member's value span: delete it, then put the exact same bytes
	// back at the exact same offset.
	last := root.Members[len(root.Members)-1]
	span := Span{From: last.Value.Start, To: last.Value.Stop}
	removed := append([]byte(nil), data[span.From:span.To]...)

	deleted := Delete(data, []Span{span})
	if len(deleted) != len(data)-(span.To-span.From) {
		t.Fatalf("Delete did not remove the span: len(deleted) = %d, want %d", len(deleted), len(data)-(span.To-span.From))
	}

	restored := Insert(deleted, []Insertion{{At: span.From, Text: removed}})
	if sha256.Sum256(restored) != sha256.Sum256(data) {
		t.Errorf("delete-then-reinsert of the same text at the same offset did not restore the original bytes")
	}
	t.Logf("OBSERVED  deleted %d bytes at [%d,%d) and reinserted them: sha256 matches the original", len(removed), span.From, span.To)
}

// testMultipleInsertionsBackToFront is the fifth bullet: multiple insertions
// apply back-to-front so earlier offsets stay valid. The construction needing
// all three events inserted AT ONCE, at the SAME computed offset, is an
// object with an already-present but EMPTY "hooks" object — none of T1's
// fixtures happen to hold that shape, so it is built here, directly, the way
// install.go's own edit set would compute it: every insertion position
// computed against the ORIGINAL (empty) node before any edit is applied.
func testMultipleInsertionsBackToFront(t *testing.T) {
	t.Parallel()

	data := []byte(`{"hooks": {}}`)
	root, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	hooks := root.Member("hooks")
	if hooks == nil || hooks.Value.Kind != KindObject {
		t.Fatalf("fixture does not have the expected empty \"hooks\" object")
	}
	// Every insertion computed against the SAME pristine (empty) hooks
	// object, exactly as install.go computes a whole event's worth of edits
	// before any of them are applied — this is the tie case Insert's doc
	// comment describes.
	at := hooks.Value.Start + 1
	edits := []Insertion{
		{At: at, Text: []byte(`"SessionStart":[1]`)},
		{At: at, Text: []byte(`,"Stop":[2]`)},
		{At: at, Text: []byte(`,"PreCompact":[3]`)},
	}

	got := Insert(data, edits)
	if !json.Valid(got) {
		t.Fatalf("the spliced result is not valid JSON:\n%s", got)
	}

	var decoded struct {
		Hooks map[string][]int `json:"hooks"`
	}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decoding the result: %v", err)
	}
	want := map[string][]int{"SessionStart": {1}, "Stop": {2}, "PreCompact": {3}}
	for event, wantVal := range want {
		if got := decoded.Hooks[event]; len(got) != 1 || got[0] != wantVal[0] {
			t.Errorf("event %s = %v, want %v — a tied insertion overwrote or reordered a sibling", event, got, wantVal)
		}
	}

	// And the caller-declared order survives in the actual bytes, not merely
	// in the decoded (unordered) map — the whole point of a span splicer.
	root2, err := Scan(got)
	if err != nil {
		t.Fatalf("Scan of the spliced result: %v", err)
	}
	gotOrder := make([]string, 0, 3)
	for _, m := range root2.Member("hooks").Value.Members {
		gotOrder = append(gotOrder, m.Key)
	}
	wantOrder := []string{"SessionStart", "Stop", "PreCompact"}
	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("hooks has %d members, want %d: %v", len(gotOrder), len(wantOrder), gotOrder)
	}
	for i, k := range wantOrder {
		if gotOrder[i] != k {
			t.Errorf("member %d = %q, want %q (order: %v)", i, gotOrder[i], k, gotOrder)
		}
	}
	t.Logf("OBSERVED  %s", got)
}

// testWrongEmitterFailsRoundTrip is the deliberately wrong emitter this task's
// acceptance names explicitly: json.Unmarshal followed by json.Marshal MUST
// fail the round trip on the "//"-comment-key fixture and the
// unusual-whitespace fixture. That is the class of defect the span approach
// exists to defeat, so the suite must be able to show it happening — a
// round-trip test that passes for both implementations would be evidence for
// neither.
func testWrongEmitterFailsRoundTrip(t *testing.T) {
	t.Parallel()

	wrongEmit := func(data []byte) []byte {
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decoding: %v", err)
		}
		out, err := json.Marshal(decoded)
		if err != nil {
			t.Fatalf("re-encoding: %v", err)
		}
		return out
	}

	corpus := realContractCorpus(t)
	for _, name := range []string{
		contractFixtureDir + "/odd-whitespace.json",
		contractExampleFile, // the project's own "//"-comment-key file
	} {
		data, err := corpus.load(name)
		if err != nil {
			t.Fatalf("loading %s: %v", name, err)
		}

		// Red control: the wrong emitter must NOT round-trip this fixture.
		if sha256.Sum256(wrongEmit(data)) == sha256.Sum256(data) {
			t.Fatalf("the deliberately wrong emitter (json.Unmarshal + json.Marshal) round-tripped %s byte-for-byte; "+
				"it should rewrite formatting and drop nothing, but a round trip passing for both implementations "+
				"would be evidence for neither", name)
		}

		// Green: the real (span-based) approach DOES round-trip it.
		if sha256.Sum256(Insert(data, nil)) != sha256.Sum256(data) {
			t.Errorf("the span approach itself failed to round-trip %s", name)
		}
		t.Logf("OBSERVED  %s: wrong emitter changes the bytes, span approach preserves them", name)
	}
}
