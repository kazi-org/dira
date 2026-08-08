package ledger_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/ledger/ledgertest"
)

// memStore is a ledger.Store held in memory, so the id allocator can be tested
// against a backend that is not the filesystem.
//
// That is the point of it rather than a convenience. Add has to work over E7's
// github backend, where there is no lock, no rename and no directory — so a
// concurrency test that only ever runs against the local backend would be
// evidence about os.Link rather than about the allocator. This store has none of
// those mechanisms: a plain map under a mutex, with Create failing when the key
// is taken, which is the only thing the interface promises.
//
// It is held honest by TestTheMemoryStoreObeysTheStoreContract, which runs
// ledgertest's suite against it. A fake that quietly disagrees with the real
// backend would make every test in this file a test of the fake.
type memStore struct {
	mu    sync.Mutex
	files map[string][]byte

	// beforeCreate is an injection point: it runs inside Create, before the
	// entry lands, so a test can make a write fail at the moment a real one
	// would be committing.
	beforeCreate func(id string) error

	// creates counts every Create call, including the ones that lost a race.
	creates int
}

func newMemStore() *memStore {
	return &memStore{files: map[string][]byte{}}
}

func (m *memStore) Get(ctx context.Context, id string) (*ledger.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.files[id]
	if !ok {
		return nil, fmt.Errorf("%s: %w", id, ledger.ErrNotFound)
	}
	return ledger.DecodeStored(data, memVersion(data))
}

func (m *memStore) List(ctx context.Context) ([]ledger.EntryInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]ledger.EntryInfo, 0, len(m.files))
	for id, data := range m.files {
		out = append(out, ledger.EntryInfo{ID: id, Version: memVersion(data)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memStore) Create(ctx context.Context, e *ledger.Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Encoding validates, which is what a real backend does and what makes
	// "an invalid entry is not written" true here for the same reason.
	data, err := ledger.Encode(e)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.creates++
	if m.beforeCreate != nil {
		if err := m.beforeCreate(e.ID); err != nil {
			return err
		}
	}
	if _, taken := m.files[e.ID]; taken {
		return fmt.Errorf("%s: %w", e.ID, ledger.ErrExists)
	}
	m.files[e.ID] = data
	return nil
}

func (m *memStore) Put(ctx context.Context, e *ledger.Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := ledger.Encode(e)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[e.ID] = data
	return nil
}

func (m *memStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[id]; !ok {
		return fmt.Errorf("%s: %w", id, ledger.ErrNotFound)
	}
	delete(m.files, id)
	return nil
}

// ids returns every id in the store, sorted.
func (m *memStore) ids() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]string, 0, len(m.files))
	for id := range m.files {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// createCalls returns how many times Create was called, which is how a test
// tells one clean allocation from a retry storm.
func (m *memStore) createCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.creates
}

func memVersion(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestTheMemoryStoreObeysTheStoreContract(t *testing.T) {
	t.Parallel()

	ledgertest.RunStoreContract(t, func(*testing.T) ledger.Store { return newMemStore() })
}
