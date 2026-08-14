package cli

import (
	"encoding/json"
	"io"
)

// The --json shape. Documented in docs/design/schemas/map.md, which a test
// checks against this file's own key set so the two cannot drift apart
// silently (docs/lore.md's dominant-finding pattern).
//
// Every bucket value is one of dira's six (status.Bucket's own string
// values) or omitted entirely — never one of kazi's own RepoBucket/RowBucket
// strings, which this file never encodes.

// jsonBlockingQuestion mirrors status.BlockingQuestion.
type jsonBlockingQuestion struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// jsonEvidence mirrors status.KaziEvidence.
type jsonEvidence struct {
	RunID      string `json:"run_id,omitempty"`
	ReleaseRef string `json:"release_ref,omitempty"`
}

// jsonUnresolved mirrors status.UnresolvedDetail.
type jsonUnresolved struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// jsonEntry is one Node, encoded.
type jsonEntry struct {
	ID           string                `json:"id"`
	Kind         string                `json:"kind"`
	Title        string                `json:"title"`
	Bucket       string                `json:"bucket,omitempty"`
	BlockedBy    *jsonBlockingQuestion `json:"blocked_by,omitempty"`
	Evidence     *jsonEvidence         `json:"evidence,omitempty"`
	Ambiguous    []string              `json:"ambiguous_statuses,omitempty"`
	Unresolved   *jsonUnresolved       `json:"unresolved,omitempty"`
	BlocksTarget string                `json:"blocks,omitempty"`
}

// jsonGroup is one Group, encoded.
type jsonGroup struct {
	Parent   jsonEntry      `json:"parent"`
	Children []jsonEntry    `json:"children"`
	Rollup   map[string]int `json:"rollup"`
}

// jsonDegraded, present only when kazi could not be asked, mirrors the
// text renderer's own degradation line — both surfaces read the same
// reason, never two independently-worded copies.
type jsonDegraded struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// jsonDoc is the whole --json document.
type jsonDoc struct {
	ObservedAt string        `json:"observed_at"`
	Groups     []jsonGroup   `json:"groups"`
	Unparented []jsonEntry   `json:"unparented"`
	Degraded   *jsonDegraded `json:"degraded,omitempty"`
}

// RenderJSON writes tree as the documented --json shape.
func RenderJSON(w io.Writer, tree *Tree, snapErr error, observedAt string) error {
	doc := jsonDoc{
		ObservedAt: observedAt,
		Groups:     make([]jsonGroup, 0, len(tree.Groups)),
		Unparented: make([]jsonEntry, 0, len(tree.Unparented)),
	}
	for _, g := range tree.Groups {
		children := make([]jsonEntry, 0, len(g.Children))
		for _, c := range g.Children {
			children = append(children, encodeEntry(c))
		}
		rollup := make(map[string]int, len(g.Rollup))
		for b, n := range g.Rollup {
			rollup[string(b)] = n
		}
		doc.Groups = append(doc.Groups, jsonGroup{
			Parent:   encodeEntry(g.Parent),
			Children: children,
			Rollup:   rollup,
		})
	}
	for _, n := range tree.Unparented {
		doc.Unparented = append(doc.Unparented, encodeEntry(n))
	}
	if snapErr != nil {
		doc.Degraded = &jsonDegraded{
			Reason:  degradationReasonFor(snapErr),
			Message: degradationLineFor(snapErr),
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// encodeEntry converts one Node into its wire shape. bucket is dira's own
// status.Bucket value or "" — never a kazi vocabulary string.
func encodeEntry(n *Node) jsonEntry {
	e := jsonEntry{
		ID:           n.ID,
		Kind:         string(n.Kind),
		Title:        n.Title,
		Bucket:       string(n.Bucket),
		BlocksTarget: n.BlocksTarget,
	}
	if n.BlockedBy != nil {
		e.BlockedBy = &jsonBlockingQuestion{ID: n.BlockedBy.ID, Title: n.BlockedBy.Title}
	}
	if n.Evidence != nil {
		e.Evidence = &jsonEvidence{RunID: n.Evidence.RunID, ReleaseRef: n.Evidence.ReleaseRef}
	}
	if n.Ambiguous != nil {
		e.Ambiguous = n.Ambiguous.Statuses
	}
	if n.Unresolved != nil {
		e.Unresolved = &jsonUnresolved{Ref: n.Unresolved.Ref, Reason: n.Unresolved.Reason}
	}
	return e
}
