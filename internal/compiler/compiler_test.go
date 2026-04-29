package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/oxhq/ctx/internal/scanner"
	"github.com/oxhq/ctx/internal/store"
)

func TestCompileEnforcesBudgetAndExplainsDecisions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "planner.go"), "package app\nfunc TransformPlanner() {}\n")
	writeFile(t, filepath.Join(root, "large.go"), "package app\nvar Large = `"+largeString(1600)+"`\n")
	writeFile(t, filepath.Join(root, "notes.txt"), "unrelated notes\n")

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
	foundLineRange := false
	for _, item := range packet.Context {
		if item.SourcePath == "planner.go" && item.StartLine > 0 && item.EndLine >= item.StartLine {
			foundLineRange = true
		}
	}
	if !foundLineRange {
		t.Fatalf("expected line-ranged planner.go context item: %#v", packet.Context)
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

func TestMarshalMarkdownRendersContextForAgentPaste(t *testing.T) {
	packet := ContextPacket{
		Task: Task{Intent: "refactor", Query: "refactor planner"},
		Context: []ContextItem{
			{ID: "src_1", Key: "code.planner.go", Value: "func TransformPlanner() {}", SourcePath: "planner.go", StartLine: 10, EndLine: 10},
		},
		Meta: PacketMeta{TokensUsed: 10, Budget: 1200, RulesApplied: []string{"IncludeRelevantCode"}},
	}
	explain := Explanation{Budget: BudgetReport{Max: 1200, Used: 10}}

	got := MarshalMarkdown(packet, explain)
	if !strings.Contains(got, "# ctx context packet") {
		t.Fatalf("missing markdown header:\n%s", got)
	}
	if !strings.Contains(got, "planner.go:10-10") {
		t.Fatalf("missing line range:\n%s", got)
	}
	if !strings.Contains(got, "func TransformPlanner()") {
		t.Fatalf("missing context body:\n%s", got)
	}
}

func largeString(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'x'
	}
	return string(out)
}
