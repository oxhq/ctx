package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIScanCompileExplainAndBench(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "planner.go"), "package app\nfunc TransformPlanner() {}\n")
	cases := filepath.Join(root, "cases.jsonl")
	if err := os.WriteFile(cases, []byte(`{"task":"refactor transform planner","expected_touched_areas":["planner.go"],"budget":300}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Execute([]string{"scan", root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ctx", "facts.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ctx", "sources.db")); err != nil {
		t.Fatal(err)
	}

	if err := Execute([]string{"compile", "refactor transform planner", "--repo", root, "--budget", "300", "--format", "json", "--explain"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ctx", "last_packet.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ctx", "last_explain.json")); err != nil {
		t.Fatal(err)
	}
	if err := Execute([]string{"explain", "--repo", root, "--last"}); err != nil {
		t.Fatal(err)
	}
	if err := Execute([]string{"bench", "--repo", root, "--cases", cases, "--baseline", "naive"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ctx", "bench", "results.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCLIVersionPrintsConfiguredVersion(t *testing.T) {
	original := Version
	Version = "v9.9.9-test"
	t.Cleanup(func() { Version = original })

	var out bytes.Buffer
	if err := ExecuteWithOutput([]string{"version"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "v9.9.9-test\n" {
		t.Fatalf("unexpected version output %q", out.String())
	}
}
