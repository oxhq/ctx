package compiler

import (
	"path/filepath"
	"testing"

	"github.com/oxhq/ctx/internal/scanner"
	"github.com/oxhq/ctx/internal/store"
)

func TestCompileEnforcesBudgetAndExplainsDecisions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "planner.go"), "package app\nfunc TransformPlanner() {}\n")
	writeFile(t, filepath.Join(root, "large.go"), "package app\nvar Large = `"+largeString(1600)+"`\n")

	db, err := store.Open(filepath.Join(root, ".ctx"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	facts, sources, err := scanner.New().Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertFacts(facts, sources); err != nil {
		t.Fatal(err)
	}

	packet, explain, err := New(db).Compile(CompileRequest{
		Task:   "refactor transform planner",
		Repo:   root,
		Budget: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Meta.TokensUsed > 120 {
		t.Fatalf("packet exceeded budget: %#v", packet.Meta)
	}
	if len(explain.Included) == 0 {
		t.Fatalf("expected included explanation entries")
	}
	if len(explain.Excluded)+len(explain.Collapsed) == 0 {
		t.Fatalf("expected at least one excluded or collapsed entry")
	}
	if packet.Task.Intent == "" {
		t.Fatalf("expected task intent")
	}
}

func TestPacketJSONIsStable(t *testing.T) {
	packet := ContextPacket{
		Task: Task{Intent: "refactor", Query: "refactor planner"},
		Context: []ContextItem{
			{ID: "b", Key: "project.language", Value: "go", SourcePath: "go.mod"},
		},
		Meta: PacketMeta{TokensUsed: 10, RulesApplied: []string{"PreferProjectFacts"}},
	}

	first, err := MarshalStable(packet)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalStable(packet)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("stable JSON changed:\n%s\n%s", first, second)
	}
}

func largeString(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'x'
	}
	return string(out)
}
