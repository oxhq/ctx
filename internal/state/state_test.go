package state

import (
	"path/filepath"
	"testing"

	"github.com/oxhq/ctx/internal/compiler"
	"github.com/oxhq/ctx/internal/store"
)

func TestStateStoreStoresRefsAndAppliesDeltasDeterministically(t *testing.T) {
	root := t.TempDir()
	db, err := store.Open(filepath.Join(root, ".ctx"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := New(db)
	ref, err := store.Put(compiler.ContextPacket{
		Task: compiler.Task{Intent: "refactor", Query: "refactor planner"},
		Context: []compiler.ContextItem{
			{ID: "doc_1", Key: "file", Value: "one", SourcePath: "one.go"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.ApplyDelta(ref, Delta{
		Add:    []compiler.ContextItem{{ID: "doc_2", Key: "file", Value: "two", SourcePath: "two.go"}},
		Remove: []string{"doc_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := store.Get(next)
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Context) != 1 || packet.Context[0].ID != "doc_2" {
		t.Fatalf("unexpected effective context: %#v", packet.Context)
	}
}
