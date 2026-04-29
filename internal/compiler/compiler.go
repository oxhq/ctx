package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oxhq/ctx/internal/jsonutil"
	"github.com/oxhq/ctx/internal/model"
	"github.com/oxhq/ctx/internal/retrieval"
	"github.com/oxhq/ctx/internal/store"
	"github.com/oxhq/ctx/internal/token"
)

type ContextPacket = model.ContextPacket
type ContextItem = model.ContextItem
type Task = model.Task
type PacketMeta = model.PacketMeta
type Explanation = model.Explanation
type ExplainEntry = model.ExplainEntry
type BudgetReport = model.BudgetReport

type CompileRequest struct {
	Task   string
	Repo   string
	Budget int
}

type Compiler struct {
	store     *store.DB
	retriever retrieval.Retriever
	tokenizer token.Tokenizer
}

func New(db *store.DB) Compiler {
	return Compiler{
		store:     db,
		retriever: retrieval.New(db),
		tokenizer: token.EstimateTokenizer{CharsPerToken: 4},
	}
}

func (c Compiler) Compile(req CompileRequest) (ContextPacket, Explanation, error) {
	if req.Budget <= 0 {
		req.Budget = 12000
	}
	candidates, err := c.retriever.Search(req.Task, 100)
	if err != nil {
		return ContextPacket{}, Explanation{}, err
	}
	facts, err := c.store.Facts(false)
	if err != nil {
		return ContextPacket{}, Explanation{}, err
	}
	candidates = preferProjectFacts(candidates, facts)

	intent := classifyIntent(req.Task)
	packet := ContextPacket{
		Task: model.Task{Intent: intent, Query: req.Task},
		Meta: model.PacketMeta{
			Budget:       req.Budget,
			RulesApplied: []string{"PreferProjectFacts", "IncludeRelevantCode", "BudgetGuard"},
		},
	}
	explain := Explanation{Budget: BudgetReport{Max: req.Budget}}
	seen := map[string]bool{}
	usedSources := map[string]bool{}

	for _, candidate := range candidates {
		if seen[candidate.ID] {
			continue
		}
		seen[candidate.ID] = true
		item := itemFromCandidate(candidate)
		item.Tokens = c.tokenizer.Count(item.Key + item.Value + item.SourcePath)
		if item.Tokens > req.Budget/2 && candidate.Kind == "code" {
			item.Value = "@doc(" + candidate.SourcePath + ")"
			item.Collapsed = true
			item.Tokens = c.tokenizer.Count(item.Key + item.Value + item.SourcePath)
			explain.Collapsed = append(explain.Collapsed, ExplainEntry{
				ID: candidate.ID, SourcePath: candidate.SourcePath, Reason: "large file collapsed to symbolic ref", Score: candidate.Score, Tokens: item.Tokens,
			})
		}
		if packet.Meta.TokensUsed+item.Tokens > req.Budget {
			explain.Excluded = append(explain.Excluded, ExplainEntry{
				ID: candidate.ID, SourcePath: candidate.SourcePath, Reason: "budget guard", Score: candidate.Score, Tokens: item.Tokens,
			})
			continue
		}
		packet.Context = append(packet.Context, item)
		packet.Meta.TokensUsed += item.Tokens
		if candidate.SourcePath != "" {
			usedSources[candidate.SourcePath] = true
		}
		explain.Included = append(explain.Included, ExplainEntry{
			ID: candidate.ID, SourcePath: candidate.SourcePath, Reason: strings.Join(candidate.Reasons, ","), Score: candidate.Score, Tokens: item.Tokens,
		})
	}
	sources, err := c.store.Sources(false)
	if err != nil {
		return ContextPacket{}, Explanation{}, err
	}
	for _, source := range sources {
		if usedSources[source.Path] {
			continue
		}
		explain.Excluded = append(explain.Excluded, ExplainEntry{
			ID: source.ID, SourcePath: source.Path, Reason: "low score or no query match",
		})
	}
	explain.Budget.Used = packet.Meta.TokensUsed
	return packet, explain, nil
}

func WriteLast(repo string, packet ContextPacket, explain Explanation) error {
	ctxDir := filepath.Join(repo, ".ctx")
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		return err
	}
	packetJSON, err := MarshalStable(packet)
	if err != nil {
		return err
	}
	explainJSON, err := MarshalStable(explain)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "last_packet.json"), packetJSON, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ctxDir, "last_explain.json"), explainJSON, 0o644)
}

func MarshalStable(v any) ([]byte, error) {
	return jsonutil.MarshalStable(v)
}

func preferProjectFacts(candidates []model.Candidate, facts []model.Fact) []model.Candidate {
	seen := map[string]bool{}
	for _, candidate := range candidates {
		seen[candidate.ID] = true
	}
	for _, fact := range facts {
		if fact.Confidence < 0.75 || seen[fact.ID] {
			continue
		}
		value := strings.Trim(string(fact.Value), `"`)
		candidates = append(candidates, model.Candidate{
			ID: fact.ID, Kind: "fact", Key: fact.Key, Value: value, SourcePath: fact.SourcePath,
			SourceHash: fact.SourceHash, Text: fact.Key + " " + value, Score: 1.0,
			Reasons: []string{"project_fact"},
		})
	}
	return candidates
}

func itemFromCandidate(candidate model.Candidate) model.ContextItem {
	value := candidate.Value
	key := candidate.Key
	if candidate.Kind == "code" {
		key = "code." + candidate.SourcePath
		value = snippet(candidate.Text)
	}
	return model.ContextItem{
		ID: candidate.ID, Key: key, Value: value, SourcePath: candidate.SourcePath,
		StartLine: candidate.StartLine, EndLine: candidate.EndLine,
	}
}

func MarshalMarkdown(packet ContextPacket, explain Explanation) string {
	var builder strings.Builder
	builder.WriteString("# ctx context packet\n\n")
	builder.WriteString(fmt.Sprintf("- intent: %s\n", packet.Task.Intent))
	builder.WriteString(fmt.Sprintf("- task: %s\n", packet.Task.Query))
	builder.WriteString(fmt.Sprintf("- budget: %d\n", packet.Meta.Budget))
	builder.WriteString(fmt.Sprintf("- tokens_used: %d\n\n", packet.Meta.TokensUsed))
	builder.WriteString("## Context\n\n")
	for _, item := range packet.Context {
		location := item.SourcePath
		if item.StartLine > 0 {
			location = fmt.Sprintf("%s:%d-%d", item.SourcePath, item.StartLine, item.EndLine)
		}
		builder.WriteString(fmt.Sprintf("### %s\n\n", location))
		builder.WriteString("```text\n")
		builder.WriteString(item.Value)
		builder.WriteString("\n```\n\n")
	}
	builder.WriteString("## Explain\n\n")
	builder.WriteString(fmt.Sprintf("- budget_used: %d/%d\n", explain.Budget.Used, explain.Budget.Max))
	builder.WriteString(fmt.Sprintf("- included: %d\n", len(explain.Included)))
	builder.WriteString(fmt.Sprintf("- collapsed: %d\n", len(explain.Collapsed)))
	builder.WriteString(fmt.Sprintf("- excluded: %d\n", len(explain.Excluded)))
	return builder.String()
}

func snippet(text string) string {
	const max = 1200
	if len(text) <= max {
		return text
	}
	return text[:max]
}

func classifyIntent(task string) string {
	lower := strings.ToLower(task)
	switch {
	case strings.Contains(lower, "refactor"):
		return "refactor"
	case strings.Contains(lower, "test"):
		return "test"
	case strings.Contains(lower, "debug") || strings.Contains(lower, "fix"):
		return "debug"
	default:
		return "general"
	}
}
