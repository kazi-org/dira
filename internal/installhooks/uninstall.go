package installhooks

// Uninstall: the exact inverse of Install, including the deletion decision.
//
// Removes exactly the spans an install added; everything else is untouched.
// If the file's bytes equal what a fresh install creates, the file IS what
// install created and nothing has written to it since, so removing the file
// restores the pre-install state -- absence -- exactly (install_hooks.ex:152-160).
// An uninstall with nothing installed is Unchanged, not an error.
//
// Only entries WHOLLY owned by dira are removed -- kazi's
// wholly_kazi_entry?/2: an entry mixing an operator's command with dira's is
// never removed, and is reported back in Untouched so a caller can name it
// rather than silently doing nothing.
//
// The package deletes nothing. The deletion decision is data on the result;
// cmd/dira acts on it.

import (
	"bytes"
	"slices"
)

// An UntouchedEntry is one array element Uninstall found but did not remove,
// because it mixes an operator's own command with a command carrying dira's
// owner prefix for that event. Reported so a caller can name exactly what it
// left alone rather than reporting a bare Unchanged over content that does,
// in fact, mention dira.
type UntouchedEntry struct {
	Event    string
	Commands []string
}

// An UninstallResult is what Uninstall decided.
//
// Data is meaningful only when Outcome is Removed and DeleteFile is false. A
// caller must delete the file entirely when DeleteFile is true, and must
// write Data otherwise -- the two are mutually exclusive by construction.
type UninstallResult struct {
	Outcome    Outcome
	Data       []byte
	DeleteFile bool
	Untouched  []UntouchedEntry
}

// Uninstall computes what `dira install-hooks --uninstall` should do to a
// Claude Code settings file.
//
// exists tells Uninstall whether there is a file at all. An absent file is
// Unchanged, not an error -- there is nothing to remove and nothing was ever
// installed.
func Uninstall(data []byte, exists bool) (UninstallResult, error) {
	if !exists {
		return UninstallResult{Outcome: Unchanged}, nil
	}

	regs, err := Registrations()
	if err != nil {
		return UninstallResult{}, err
	}

	if bytes.Equal(data, freshSettings(regs)) {
		// The bytes are exactly what a fresh install produces, so install
		// created this file and nothing else has written to it since:
		// deleting it restores the pre-install state (absence) exactly.
		return UninstallResult{Outcome: Removed, DeleteFile: true}, nil
	}

	root, err := Scan(data)
	if err != nil {
		return UninstallResult{}, err
	}

	spans, untouched, err := uninstallSpans(data, root, regs)
	if err != nil {
		return UninstallResult{}, err
	}
	if len(spans) == 0 {
		return UninstallResult{Outcome: Unchanged, Untouched: untouched}, nil
	}
	return UninstallResult{Outcome: Removed, Data: Delete(data, spans), Untouched: untouched}, nil
}

// uninstallSpans is the removal set an uninstall needs, mirroring
// install_hooks.ex:305-354 (uninstall_spans/classify_event): only entries
// WHOLLY owned by dira are ever removed; an event array left with no other
// entries loses its whole key; a "hooks" object left with no other keys is
// removed entirely.
func uninstallSpans(data []byte, root *Node, regs []Registration) (spans []Span, untouched []UntouchedEntry, err error) {
	hooksMember := root.Member("hooks")
	if hooksMember == nil {
		return nil, nil, nil
	}
	// Scan already refused a non-object "hooks" value.
	hooksObj := hooksMember.Value

	var fullMemberIndices []int
	var partialElementSpans []Span

	for _, r := range regs {
		eventMember := hooksObj.Member(r.Event)
		if eventMember == nil || eventMember.Value.Kind != KindArray {
			continue
		}
		arr := eventMember.Value

		var ownedIdx, leftoverIdx []int
		for i, el := range arr.Elements {
			if entryWhollyOwnedByPrefix(data, el, r.OwnerPrefix) {
				ownedIdx = append(ownedIdx, i)
			} else {
				leftoverIdx = append(leftoverIdx, i)
			}
		}
		if len(ownedIdx) == 0 {
			// Nothing in this event's array is dira's to remove -- whether
			// it holds an unrelated operator entry, an entry MIXING an
			// operator's command with dira's (kazi's wholly_kazi_entry?/2:
			// only a WHOLLY owned entry is ever removed), or nothing at all.
			continue
		}
		if len(ownedIdx) == len(arr.Elements) {
			fullMemberIndices = append(fullMemberIndices, hooksObj.MemberIndex(eventMember))
			continue
		}
		// A partial removal: something in this array IS being removed, so
		// what is left behind is worth naming rather than leaving the caller
		// to notice a bare Unchanged over content that does mention dira --
		// an entry a human has since edited to no longer carry the owner
		// prefix at all is exactly this case.
		partialElementSpans = append(partialElementSpans, elementRemovalSpans(arr, ownedIdx)...)
		for _, i := range leftoverIdx {
			untouched = append(untouched, UntouchedEntry{Event: r.Event, Commands: entryCommands(data, arr.Elements[i])})
		}
	}

	switch {
	case len(fullMemberIndices) == 0 && len(partialElementSpans) == 0:
		return nil, untouched, nil

	case len(fullMemberIndices) == len(hooksObj.Members):
		// Every member of "hooks" -- dira's and, since none is left over,
		// nobody else's -- is wholly dira-owned: remove the whole "hooks"
		// member from the root object.
		return memberRemovalSpans(root, []int{root.MemberIndex(hooksMember)}), untouched, nil

	default:
		spans = memberRemovalSpans(hooksObj, fullMemberIndices)
		spans = append(spans, partialElementSpans...)
		return spans, untouched, nil
	}
}

// memberRemovalSpans returns the removal spans for the members of obj at
// indices, the exact inverse of insertMember.
func memberRemovalSpans(obj *Node, indices []int) []Span {
	return removalSpans(len(obj.Members), obj.Start, indices,
		func(i int) int { return obj.Members[i].KeyStart },
		func(i int) int { return obj.Members[i].Value.Stop })
}

// elementRemovalSpans returns the removal spans for the elements of arr at
// indices, the exact inverse of appendElement.
func elementRemovalSpans(arr *Node, indices []int) []Span {
	return removalSpans(len(arr.Elements), arr.Start, indices,
		func(i int) int { return arr.Elements[i].Start },
		func(i int) int { return arr.Elements[i].Stop })
}

// removalSpans computes the byte ranges to delete for removing the items at
// indices from a container of n items -- object members or array elements
// alike -- given each item's own [start,stop) span. Mirrors
// install_hooks.ex:443-474 (member_spans/element_spans/removal_spans): the
// span shapes are the exact inverses of insertMember/appendElement --
//
//   - a run ending at the last item, not starting at 0: from the previous
//     item's end through the run's last item (eats the comma + whitespace the
//     append inserted);
//   - a run starting at 0 with items after it: from the first item's start
//     through the next surviving item's start (eats the trailing comma + ws);
//   - the whole container: everything after the opening bracket through the
//     last item's end.
func removalSpans(n, containerStart int, indices []int, itemStart, itemStop func(i int) int) []Span {
	if len(indices) == 0 {
		return nil
	}
	lastIndex := n - 1

	var out []Span
	for _, run := range consecutiveRuns(indices) {
		a, b := run[0], run[1]
		switch {
		case a == 0 && b == lastIndex:
			out = append(out, Span{From: containerStart + 1, To: itemStop(b)})
		case b == lastIndex:
			out = append(out, Span{From: itemStop(a - 1), To: itemStop(b)})
		case a == 0:
			out = append(out, Span{From: itemStart(0), To: itemStart(b + 1)})
		default:
			out = append(out, Span{From: itemStop(a - 1), To: itemStop(b)})
		}
	}
	return out
}

// consecutiveRuns groups sorted, de-duplicated indices into inclusive
// [first, last] runs of consecutive integers, mirroring install_hooks.ex's
// runs/1.
func consecutiveRuns(indices []int) [][2]int {
	sorted := slices.Clone(indices)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	var runs [][2]int
	for _, i := range sorted {
		if n := len(runs); n > 0 && runs[n-1][1]+1 == i {
			runs[n-1][1] = i
		} else {
			runs = append(runs, [2]int{i, i})
		}
	}
	return runs
}
