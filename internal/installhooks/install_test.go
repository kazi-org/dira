package installhooks

// E2-L3-T4 — install: merge-never-clobber, idempotent, ownership by prefix,
// proved against T1's fixture table.

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
)

// TestInstall is T4's acceptance, iterating T1's table.
func TestInstall(t *testing.T) {
	t.Parallel()

	cases := contractCases()
	if len(cases) == 0 {
		t.Fatal("contractCases() returned nothing; every clause below iterates it")
	}

	t.Run("absent file gets a minimal document holding exactly the three registrations", testInstallIntoAbsent)
	t.Run("operator's own entry survives byte-identically alongside dira's", testInstallPreservesOperatorEntry)
	t.Run("unknown top-level keys survive sha256-identically", testInstallPreservesUnknownTopLevelKeys)
	t.Run("a second install over the first's output is a byte-level no-op", func(t *testing.T) {
		t.Parallel()
		testInstallIsIdempotent(t, cases)
	})
	t.Run("the orphan case: a pre---all PreCompact command is recognised and nothing is added beside it", testInstallRecognisesOrphan)
	t.Run("the other side: an operator's own dira-prefixed command is not recognised", testInstallDoesNotRecogniseLookalike)
	t.Run("a partially-installed file gains only the missing events", testInstallPartiallyInstalled)
	t.Run("malformed fixtures produce a named error, no bytes, no outcome", func(t *testing.T) {
		t.Parallel()
		testInstallRefusesMalformed(t, cases)
	})
	t.Run("no case ever removes or reorders a pre-existing array element", func(t *testing.T) {
		t.Parallel()
		testInstallNeverRemovesPreexisting(t, cases)
	})
	t.Run("both sides: the preservation checks can fail against a re-encoding emitter", testInstallPreservationCanFail)
}

func testInstallIntoAbsent(t *testing.T) {
	t.Parallel()

	result, err := Install(nil, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Outcome != Installed {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, Installed)
	}
	if !json.Valid(result.Data) {
		t.Fatalf("the fresh document is not valid JSON:\n%s", result.Data)
	}

	regs, err := Registrations()
	if err != nil {
		t.Fatalf("Registrations: %v", err)
	}
	root, err := Scan(result.Data)
	if err != nil {
		t.Fatalf("the fresh document does not scan: %v", err)
	}

	// Exactly the three registrations and nothing else: one top-level member.
	if len(root.Members) != 1 || root.Members[0].Key != "hooks" {
		t.Fatalf("the fresh document has top-level keys %v, want exactly [hooks]", memberKeys(root))
	}
	hooksObj := root.Members[0].Value
	if len(hooksObj.Members) != len(regs) {
		t.Fatalf("the fresh document's \"hooks\" has %d event(s), want exactly %d", len(hooksObj.Members), len(regs))
	}
	for _, r := range regs {
		event := hooksObj.Member(r.Event)
		if event == nil {
			t.Fatalf("the fresh document has no %q event", r.Event)
		}
		commands := entryCommands(result.Data, singleElement(t, event.Value))
		if len(commands) != 1 || commands[0] != r.Command {
			t.Fatalf("event %q commands = %v, want [%q]", r.Event, commands, r.Command)
		}
	}

	// Round-trips through T3: scanning and re-emitting with an empty edit set
	// changes nothing.
	if sha256.Sum256(Insert(result.Data, nil)) != sha256.Sum256(result.Data) {
		t.Error("the fresh document does not round-trip through Scan + Insert(data, nil)")
	}
	t.Logf("OBSERVED  fresh document: %s", result.Data)
}

func testInstallPreservesOperatorEntry(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/operator-hooks.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}

	before, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	beforeStop := before.Member("hooks").Value.Member("Stop")
	if beforeStop == nil {
		t.Fatal("fixture setup: operator-hooks.json has no Stop entry to preserve")
	}
	// The single ELEMENT's bytes, not the array's: the array's own closing
	// bracket moves when a second element is appended, so comparing the
	// array's own [Start,Stop) span would fail for a reason that has nothing
	// to do with whether the operator's own entry survived.
	operatorStopBytes := string(data[singleElement(t, beforeStop.Value).Start:singleElement(t, beforeStop.Value).Stop])
	if !strings.Contains(operatorStopBytes, "./notify.sh") {
		t.Fatalf("fixture setup: the Stop entry located is not the operator's own: %s", operatorStopBytes)
	}

	result, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Outcome != Installed {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, Installed)
	}

	after, err := Scan(result.Data)
	if err != nil {
		t.Fatalf("Scan of the result: %v", err)
	}
	afterStop := after.Member("hooks").Value.Member("Stop")
	if afterStop == nil {
		t.Fatal("Stop disappeared entirely")
	}
	// Located by SEARCHING for the operator's bytes, not by counting entries.
	if !strings.Contains(string(result.Data), operatorStopBytes) {
		t.Errorf("the operator's Stop entry did not survive byte-identically:\nwant substring: %s\ngot: %s",
			operatorStopBytes, result.Data)
	}
	// And dira's own Stop entry was added alongside it.
	regs, _ := Registrations()
	diraStop := regFor(t, regs, "Stop")
	if !strings.Contains(string(result.Data), diraStop.Command) {
		t.Errorf("dira's Stop command %q was not added:\n%s", diraStop.Command, result.Data)
	}
	if len(afterStop.Value.Elements) != len(beforeStop.Value.Elements)+1 {
		t.Errorf("Stop has %d element(s) after install, want %d (operator's + dira's)",
			len(afterStop.Value.Elements), len(beforeStop.Value.Elements)+1)
	}
	t.Logf("OBSERVED  operator's Stop entry preserved byte-identically, dira's Stop entry added alongside it")
}

func testInstallPreservesUnknownTopLevelKeys(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/unknown-top-level-keys.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	before, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if before.Member("hooks") != nil {
		t.Fatal("fixture setup: unknown-top-level-keys.json already has a \"hooks\" key")
	}
	if len(before.Members) == 0 {
		t.Fatal("fixture setup: unknown-top-level-keys.json has no top-level keys to preserve")
	}

	result, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	after, err := Scan(result.Data)
	if err != nil {
		t.Fatalf("Scan of the result: %v", err)
	}
	for _, m := range before.Members {
		want := string(data[m.Value.Start:m.Value.Stop])
		got := after.Member(m.Key)
		if got == nil {
			t.Errorf("key %q disappeared", m.Key)
			continue
		}
		if string(result.Data[got.Value.Start:got.Value.Stop]) != want {
			t.Errorf("key %q's value changed:\nwant: %s\ngot:  %s", m.Key, want, result.Data[got.Value.Start:got.Value.Stop])
		}
	}
	if after.Member("hooks") == nil {
		t.Error("\"hooks\" was not created")
	}
	t.Logf("OBSERVED  %d pre-existing unknown key(s) survived sha256-identically as spans", len(before.Members))
}

func testInstallIsIdempotent(t *testing.T, cases []contractCase) {
	t.Helper()

	tested := 0
	for _, tc := range cases {
		if tc.Verdict != contractAccept {
			continue
		}
		tested++

		data, exists := tc.Input(t)
		first, err := Install(data, exists)
		if err != nil {
			t.Errorf("case %s: first Install: %v", tc.Name, err)
			continue
		}

		// The first install must actually have changed something, or "no
		// edits on the second run" would be worthless -- it did nothing on
		// either run and this test would say nothing happened twice.
		var second InstallResult
		switch first.Outcome {
		case Installed:
			second, err = Install(first.Data, true)
		case Unchanged:
			// Already fully installed on the first run (possible for a
			// fixture whose events are already all dira's own, by prefix).
			// The idempotence property still has to hold from here.
			second, err = Install(data, exists)
		}
		if err != nil {
			t.Errorf("case %s: second Install: %v", tc.Name, err)
			continue
		}
		if second.Outcome != Unchanged {
			t.Errorf("case %s: second install reported %q, want %q", tc.Name, second.Outcome, Unchanged)
			continue
		}
		if second.Data != nil {
			t.Errorf("case %s: second install produced bytes to write despite reporting %q", tc.Name, Unchanged)
		}
	}
	if tested == 0 {
		t.Fatal("no accept-verdict case exercised idempotence; this clause would be vacuously true")
	}
	t.Logf("OBSERVED  %d case(s) idempotent: install(install(x)) == install(x) with no second write", tested)
}

// testInstallRecognisesOrphan is the case named because it already happened
// once: a settings file holding `dira sniff --deep --stage` (the pre---all
// command) under PreCompact must be recognised as dira's own, and install
// adds nothing beside it.
func testInstallRecognisesOrphan(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/pre-all-precompact.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	before, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	precompactBefore := before.Member("hooks").Value.Member("PreCompact")
	if precompactBefore == nil || len(precompactBefore.Value.Elements) != 1 {
		t.Fatal("fixture setup: pre-all-precompact.json does not hold exactly one PreCompact entry")
	}

	result, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Outcome != Installed {
		t.Fatalf("Outcome = %q, want %q (SessionStart and Stop are both still missing)", result.Outcome, Installed)
	}

	after, err := Scan(result.Data)
	if err != nil {
		t.Fatalf("Scan of the result: %v", err)
	}
	precompactAfter := after.Member("hooks").Value.Member("PreCompact")
	if precompactAfter == nil {
		t.Fatal("PreCompact disappeared")
	}
	if len(precompactAfter.Value.Elements) != 1 {
		t.Errorf("PreCompact has %d element(s) after install, want 1 — a whole-string matcher would have "+
			"added a second entry and the session would run sniff twice per compaction", len(precompactAfter.Value.Elements))
	}
	// The bytes of that one element must be byte-identical to before: dira's
	// own edits never touch an already-owned entry, even to "fix" it to the
	// current command.
	wantBytes := string(data[precompactBefore.Value.Elements[0].Start:precompactBefore.Value.Elements[0].Stop])
	gotBytes := string(result.Data[precompactAfter.Value.Elements[0].Start:precompactAfter.Value.Elements[0].Stop])
	if gotBytes != wantBytes {
		t.Errorf("the pre-existing PreCompact entry changed:\nwant: %s\ngot:  %s", wantBytes, gotBytes)
	}
	if !strings.Contains(gotBytes, "dira sniff --deep --stage") || strings.Contains(gotBytes, "--all") {
		t.Errorf("the preserved entry is not the pre---all command verbatim: %s", gotBytes)
	}

	// And SessionStart, Stop were both added.
	for _, event := range []string{"SessionStart", "Stop"} {
		if after.Member("hooks").Value.Member(event) == nil {
			t.Errorf("%s was not added even though it was entirely missing", event)
		}
	}
	t.Logf("OBSERVED  PreCompact recognised by prefix, untouched; SessionStart and Stop added")
}

// testInstallDoesNotRecogniseLookalike is the other side of prefix
// recognition: an operator's own `dira why dec-0003` hook under Stop starts
// with "dira " but must not be recognised as dira's own Stop entry.
func testInstallDoesNotRecogniseLookalike(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/operator-dira-lookalike.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}

	result, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Outcome != Installed {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, Installed)
	}

	after, err := Scan(result.Data)
	if err != nil {
		t.Fatalf("Scan of the result: %v", err)
	}
	stop := after.Member("hooks").Value.Member("Stop")
	if stop == nil {
		t.Fatal("Stop disappeared")
	}
	if len(stop.Value.Elements) != 2 {
		t.Fatalf("Stop has %d element(s), want 2 (the operator's lookalike + dira's own) — a prefix broad enough "+
			"to swallow any \"dira \" command would leave the session with no capture at all", len(stop.Value.Elements))
	}
	if !strings.Contains(string(result.Data), "dira why dec-0003") {
		t.Error("the operator's lookalike entry did not survive")
	}
	regs, _ := Registrations()
	if !strings.Contains(string(result.Data), regFor(t, regs, "Stop").Command) {
		t.Error("dira's own Stop entry was not added alongside the lookalike")
	}
	t.Logf("OBSERVED  operator's `dira why dec-0003` under Stop not recognised as dira's; dira's own entry added alongside it")
}

func testInstallPartiallyInstalled(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/partially-installed.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	before, err := Scan(data)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	sessionStartBefore := before.Member("hooks").Value.Member("SessionStart")
	if sessionStartBefore == nil {
		t.Fatal("fixture setup: partially-installed.json has no SessionStart entry")
	}
	wantBytes := string(data[sessionStartBefore.Value.Start:sessionStartBefore.Value.Stop])

	result, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Outcome != Installed {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, Installed)
	}

	after, err := Scan(result.Data)
	if err != nil {
		t.Fatalf("Scan of the result: %v", err)
	}
	sessionStartAfter := after.Member("hooks").Value.Member("SessionStart")
	if sessionStartAfter == nil {
		t.Fatal("SessionStart disappeared")
	}
	if got := string(result.Data[sessionStartAfter.Value.Start:sessionStartAfter.Value.Stop]); got != wantBytes {
		t.Errorf("the present SessionStart event changed:\nwant: %s\ngot:  %s", wantBytes, got)
	}
	for _, event := range []string{"Stop", "PreCompact"} {
		if after.Member("hooks").Value.Member(event) == nil {
			t.Errorf("%s was not added", event)
		}
	}
	t.Logf("OBSERVED  SessionStart untouched, Stop and PreCompact both added")
}

func testInstallRefusesMalformed(t *testing.T, cases []contractCase) {
	t.Helper()

	tested := 0
	for _, tc := range cases {
		if tc.Verdict == contractAccept {
			continue
		}
		tested++

		data, exists := tc.Input(t)
		result, err := Install(data, exists)
		if err == nil {
			t.Errorf("case %s: Install accepted a fixture the table marks %q", tc.Name, tc.Verdict)
			continue
		}
		if result.Outcome != "" || result.Data != nil {
			t.Errorf("case %s: a refused install still produced Outcome=%q, %d byte(s)", tc.Name, result.Outcome, len(result.Data))
		}
	}
	if tested == 0 {
		t.Fatal("no case in the table is marked malformed; this clause would be vacuously true")
	}
	t.Logf("OBSERVED  %d malformed case(s) refused, each with no Outcome and no bytes", tested)
}

// testInstallNeverRemovesPreexisting is the acc line's own generality check:
// across the WHOLE table, no case ever removes or reorders a pre-existing
// array element or top-level member. Compares every pre-existing member's
// bytes before and after, for every present, accept-verdict case.
func testInstallNeverRemovesPreexisting(t *testing.T, cases []contractCase) {
	t.Helper()

	tested := 0
	for _, tc := range cases {
		if tc.Present != contractFilePresent || tc.Verdict != contractAccept {
			continue
		}
		data, _ := tc.Input(t)
		result, err := Install(data, true)
		if err != nil {
			t.Errorf("case %s: Install: %v", tc.Name, err)
			continue
		}
		if result.Outcome == Unchanged {
			// Nothing was written; the untouched input trivially preserves
			// itself, and there is nothing new to compare it against.
			tested++
			continue
		}
		if problems := preexistingBytesPreserved(t, data, result.Data); len(problems) != 0 {
			for _, p := range problems {
				t.Errorf("case %s: %s", tc.Name, p)
			}
			continue
		}
		tested++
	}
	if tested == 0 {
		t.Fatal("no present, accept-verdict case was checked; this clause would be vacuously true")
	}
	t.Logf("OBSERVED  %d case(s) preserved every pre-existing member and array element byte-for-byte, in order", tested)
}

// testInstallPreservationCanFail is the "both sides" proof this task's own
// acc line asks for: the preservation checks above are proven able to fail
// against a build whose emitter re-encodes rather than splices, reusing T3's
// wrong emitter rather than writing a second one.
func testInstallPreservationCanFail(t *testing.T) {
	t.Parallel()

	corpus := realContractCorpus(t)
	data, err := corpus.load(contractFixtureDir + "/unknown-top-level-keys.json")
	if err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}

	wrongInstall := func(data []byte) []byte {
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decoding: %v", err)
		}
		regs, err := Registrations()
		if err != nil {
			t.Fatalf("Registrations: %v", err)
		}
		hooks := map[string]any{}
		for _, r := range regs {
			hooks[r.Event] = []any{map[string]any{"hooks": []any{
				map[string]any{"type": "command", "command": r.Command, "timeout": r.Timeout},
			}}}
		}
		decoded["hooks"] = hooks
		out, err := json.Marshal(decoded)
		if err != nil {
			t.Fatalf("re-encoding: %v", err)
		}
		return out
	}

	wrongOutput := wrongInstall(data)
	if problems := preexistingBytesPreserved(t, data, wrongOutput); len(problems) == 0 {
		t.Fatal("the wrong (decode/re-encode) installer round-tripped every pre-existing key byte-for-byte; " +
			"that is the defect the span approach exists to defeat, so the check must be able to catch it")
	} else {
		t.Logf("OBSERVED  the wrong emitter tripped the preservation check: %s", problems[0])
	}

	// And the real installer passes the same check on the same input.
	realResult, err := Install(data, true)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if problems := preexistingBytesPreserved(t, data, realResult.Data); len(problems) != 0 {
		t.Fatalf("the real installer failed its own preservation check:\n%s", strings.Join(problems, "\n"))
	}
}

// ---- shared test helpers ----------------------------------------------------

// preexistingBytesPreserved returns every way output does not preserve every
// member and array element that existed in before -- byte-identically, in
// order, allowing only new members and new trailing array elements.
func preexistingBytesPreserved(t *testing.T, before, output []byte) []string {
	t.Helper()

	oldRoot, err := Scan(before)
	if err != nil {
		t.Fatalf("Scan(before): %v", err)
	}
	newRoot, err := Scan(output)
	if err != nil {
		t.Fatalf("Scan(output): %v", err)
	}

	var problems []string
	for _, m := range oldRoot.Members {
		newMember := newRoot.Member(m.Key)
		if newMember == nil {
			problems = append(problems, "top-level key "+m.Key+" disappeared")
			continue
		}
		if m.Key != "hooks" {
			want := string(before[m.Value.Start:m.Value.Stop])
			got := string(output[newMember.Value.Start:newMember.Value.Stop])
			if got != want {
				problems = append(problems, "top-level key "+m.Key+" changed:\nwant: "+want+"\ngot:  "+got)
			}
			continue
		}
		if m.Value.Kind != KindObject || newMember.Value.Kind != KindObject {
			continue
		}
		for _, event := range m.Value.Members {
			newEvent := newMember.Value.Member(event.Key)
			if newEvent == nil {
				problems = append(problems, `event "`+event.Key+`" disappeared`)
				continue
			}
			if event.Value.Kind != KindArray || newEvent.Value.Kind != KindArray {
				continue
			}
			if len(newEvent.Value.Elements) < len(event.Value.Elements) {
				problems = append(problems, `event "`+event.Key+`" lost element(s)`)
				continue
			}
			for i, el := range event.Value.Elements {
				want := string(before[el.Start:el.Stop])
				got := string(output[newEvent.Value.Elements[i].Start:newEvent.Value.Elements[i].Stop])
				if got != want {
					problems = append(problems, `event "`+event.Key+`" element `+itoa(i)+` changed:\nwant: `+want+"\ngot:  "+got)
				}
			}
		}
	}
	return problems
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

func memberKeys(n *Node) []string {
	out := make([]string, 0, len(n.Members))
	for _, m := range n.Members {
		out = append(out, m.Key)
	}
	return out
}

func singleElement(t *testing.T, arr *Node) *Node {
	t.Helper()
	if len(arr.Elements) != 1 {
		t.Fatalf("array has %d element(s), want exactly 1", len(arr.Elements))
	}
	return arr.Elements[0]
}

func regFor(t *testing.T, regs []Registration, event string) Registration {
	t.Helper()
	for _, r := range regs {
		if r.Event == event {
			return r
		}
	}
	t.Fatalf("no registration for event %q", event)
	return Registration{}
}
