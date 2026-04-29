package retrieval

import (
	"path/filepath"
	"testing"

	"github.com/oxhq/ctx/internal/scanner"
	"github.com/oxhq/ctx/internal/store"
)

func TestRetrieverRanksDeterministicallyWithBoostsAndGeneratedPenalty(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "planner.go"), "package app\nfunc TransformPlanner() {}\n")
	writeFile(t, filepath.Join(root, "generated.pb.go"), "package app\nfunc TransformPlannerGenerated() {}\n")
	writeFile(t, filepath.Join(root, "README.md"), "transform planner docs\n")

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

	retriever := New(db)
	first, err := retriever.Search("refactor transform planner", 5)
	if err != nil {
		t.Fatal(err)
	}
	second, err := retriever.Search("refactor transform planner", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatalf("expected candidates")
	}
	if first[0].SourcePath != "planner.go" {
		t.Fatalf("expected planner.go first, got %#v", first[0])
	}
	if len(first) != len(second) {
		t.Fatalf("candidate count changed between runs")
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].Score != second[i].Score {
			t.Fatalf("non-deterministic result at %d: %#v != %#v", i, first[i], second[i])
		}
	}
}
