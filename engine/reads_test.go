package engine_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kazi-org/dira/engine"
	"github.com/kazi-org/dira/internal/brief"
	"github.com/kazi-org/dira/internal/why"
	"github.com/kazi-org/dira/ledger"
)

// Public signatures prove a consumer can implement both source interfaces
// without internal types. This is an in-memory fixture, not a production backend.
type readSource struct {
	entries []*ledger.Entry
	ids     []string
	err     error
}

var _ engine.BriefSource = readSource{}
var _ engine.WhySource = readSource{}

func (s readSource) Select(_ context.Context, sel engine.Selector) ([]engine.EntryRef, error) {
	if s.err != nil {
		return nil, s.err
	}
	var refs []engine.EntryRef
	for _, e := range s.entries {
		if len(sel.Kinds) != 0 && !contains(sel.Kinds, e.Kind) || len(sel.States) != 0 && !contains(sel.States, e.State) {
			continue
		}
		if sel.WithEdge != "" {
			found := false
			for _, edge := range e.Edges {
				found = found || edge.Type == sel.WithEdge
			}
			if !found {
				continue
			}
		}
		refs = append(refs, engine.EntryRef{ID: e.ID, Kind: e.Kind, State: e.State, Title: e.Title, Created: e.Created, Updated: e.Updated, Private: e.Private})
		if sel.Limit > 0 && len(refs) == sel.Limit {
			break
		}
	}
	return refs, nil
}

func contains[T comparable](values []T, wanted T) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (s readSource) Entry(_ context.Context, id string) (*ledger.Entry, error) {
	if s.err != nil {
		return nil, s.err
	}
	for _, e := range s.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, ledger.ErrNotFound
}

func (s readSource) Resolve(context.Context, string) ([]string, error) { return s.ids, s.err }

func (s readSource) In(_ context.Context, id string) ([]engine.Backlink, error) {
	if s.err != nil {
		return nil, s.err
	}
	var links []engine.Backlink
	for _, e := range s.entries {
		for _, edge := range e.Edges {
			if edge.To == id {
				links = append(links, engine.Backlink{From: e.ID, Type: edge.Type, Note: edge.Note})
			}
		}
	}
	return links, nil
}

func TestBriefKeepsCanonicalTextAccountingAndChainCallback(t *testing.T) {
	src := readSource{entries: fixture(t).entries}
	for _, chain := range []bool{false, true} {
		calls := 0
		opts := engine.BriefOptions{Now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Context: true, MaxTokens: 200, Chain: chain,
			ChainSource: func(context.Context) ([]engine.BriefAncestor, error) { calls++; return nil, nil }}
		var actual, canonical bytes.Buffer
		got, err := engine.Brief(context.Background(), &actual, src, opts)
		if err != nil {
			t.Fatal(err)
		}
		facadeCalls := calls
		want, err := brief.Render(context.Background(), &canonical, src, opts)
		if err != nil {
			t.Fatal(err)
		}
		if actual.String() != canonical.String() || !reflect.DeepEqual(got, want) {
			t.Fatal("brief differs from canonical producer")
		}
		// The producer may omit a chain when the local brief fills its budget.
		// Preserve that lazy decision rather than assuming a walk always ran.
		if calls != 2*facadeCalls || !chain && calls != 0 {
			t.Fatal("unexpected parent callback execution")
		}
	}
}

func TestWhyKeepsCanonicalChainAndDisambiguation(t *testing.T) {
	src := readSource{entries: fixture(t).entries, ids: []string{"dec-0060", "dec-0075"}}
	nodes, err := engine.ResolveWhy(context.Background(), src, "daemon")
	if err != nil {
		t.Fatal(err)
	}
	wantNodes, err := why.Resolve(context.Background(), src, "daemon")
	if err != nil || !reflect.DeepEqual(nodes, wantNodes) || len(nodes) != 2 {
		t.Fatal("candidate resolution changed")
	}
	var actualList, wantList bytes.Buffer
	if engine.RenderWhyCandidates(&actualList, "daemon", nodes, 80) != nil || why.RenderCandidates(&wantList, "daemon", wantNodes, 80) != nil || actualList.String() != wantList.String() {
		t.Fatal("candidate rendering changed")
	}
	got, err := engine.BuildWhy(context.Background(), src, "daemon", "dec-0060")
	if err != nil {
		t.Fatal(err)
	}
	want, err := why.Build(context.Background(), src, "daemon", "dec-0060")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("chain producer changed")
	}
	var actual, canonical bytes.Buffer
	if engine.RenderWhy(&actual, got, 80) != nil || why.RenderText(&canonical, want, 80) != nil || actual.String() != canonical.String() {
		t.Fatal("chain rendering changed")
	}
}

func TestReadSourceErrorsRemainErrors(t *testing.T) {
	sentinel := errors.New("source unavailable")
	src := readSource{err: sentinel}
	var out bytes.Buffer
	if result, err := engine.Brief(context.Background(), &out, src, engine.BriefOptions{}); result != nil || !errors.Is(err, sentinel) {
		t.Fatal("brief failure became success")
	}
	if _, err := engine.ResolveWhy(context.Background(), src, "daemon"); !errors.Is(err, sentinel) {
		t.Fatal("resolution lost source error")
	}
}
