package bench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBenchReportsTokenReductionAndExpectedAreaHits(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "planner.go"), "package app\nfunc TransformPlanner() {}\n")
	writeFile(t, filepath.Join(root, "unrelated.go"), "package app\nvar Noise = `"+largeString(2000)+"`\n")
	cases := filepath.Join(root, "cases.jsonl")
	if err := os.WriteFile(cases, []byte(`{"task":"refactor transform planner","expected_touched_areas":["planner.go"],"expected_terms":["TransformPlanner"],"budget":300,"baseline_mode":"naive"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(RunRequest{Repo: root, CasesPath: cases, Baseline: "naive"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cases) != 1 {
		t.Fatalf("expected one case, got %d", len(result.Cases))
	}
	got := result.Cases[0]
	if got.TokensNaive <= got.TokensCompiled {
		t.Fatalf("expected compiled tokens below naive tokens: %#v", got)
	}
	if got.TokenReductionPercent <= 0 {
		t.Fatalf("expected positive reduction: %#v", got)
	}
	if !got.ExpectedAreaHit || len(got.MissingExpectedAreas) != 0 {
		t.Fatalf("expected planner.go hit: %#v", got)
	}
	if !got.ExpectedTermHit || len(got.MissingExpectedTerms) != 0 {
		t.Fatalf("expected TransformPlanner term hit: %#v", got)
	}
	if got.ContextQualityScore < 0.99 {
		t.Fatalf("expected high context quality score, got %#v", got)
	}
}
