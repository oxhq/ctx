package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oxhq/ctx/internal/store"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanExtractsGoModuleLayoutAndDocsFacts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\nrequire github.com/acme/lib v1.2.3\n")
	writeFile(t, filepath.Join(root, "cmd", "app", "main.go"), "package main\nfunc main() {}\n")
	writeFile(t, filepath.Join(root, "README.md"), "# App\n")
	writeFile(t, filepath.Join(root, "vendor", "ignored.go"), "package vendor\n")

	facts, sources, err := New().Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	assertFact := func(key string) {
		t.Helper()
		for _, fact := range facts {
			if fact.Key == key {
				return
			}
		}
		t.Fatalf("missing fact %q in %#v", key, facts)
	}

	assertFact("project.language")
	assertFact("project.go.module")
	assertFact("project.go.dependency")
	assertFact("project.layout.package")
	assertFact("project.doc")

	for _, source := range sources {
		if source.Path == "vendor/ignored.go" {
			t.Fatalf("vendor source should be skipped")
		}
		if source.Hash == "" {
			t.Fatalf("source hash should be populated for %#v", source)
		}
	}
}

func TestStoreUpsertDoesNotDuplicateAndMarksMissingSourcesStale(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")

	db, err := store.Open(filepath.Join(root, ".ctx"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	scanner := New()
	facts, sources, err := scanner.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertFacts(facts, sources); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertFacts(facts, sources); err != nil {
		t.Fatal(err)
	}
	count, err := db.CountFacts()
	if err != nil {
		t.Fatal(err)
	}
	if count != len(facts) {
		t.Fatalf("expected %d facts after duplicate upsert, got %d", len(facts), count)
	}

	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	facts, sources, err = scanner.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertFacts(facts, sources); err != nil {
		t.Fatal(err)
	}
	stale, err := db.StaleSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) == 0 {
		t.Fatalf("expected removed source to be marked stale")
	}
}
