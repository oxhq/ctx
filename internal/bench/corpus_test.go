package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMorfxBenchmarkCorpusIsValidJSONL(t *testing.T) {
	path := filepath.Join("..", "..", "benchmarks", "morfx", "cases.jsonl")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 Morfx benchmark cases, got %d", len(lines))
	}
	for i, line := range lines {
		var benchCase Case
		if err := json.Unmarshal([]byte(line), &benchCase); err != nil {
			t.Fatalf("case %d is invalid JSON: %v", i+1, err)
		}
		if benchCase.Task == "" {
			t.Fatalf("case %d has empty task", i+1)
		}
		if benchCase.Budget <= 0 {
			t.Fatalf("case %d has invalid budget", i+1)
		}
		if len(benchCase.ExpectedTouchedAreas) == 0 {
			t.Fatalf("case %d has no expected touched areas", i+1)
		}
	}
}
