package retrieval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oxhq/ctx/internal/model"
	"github.com/oxhq/ctx/internal/store"
	"github.com/oxhq/ctx/internal/token"
)

type Retriever interface {
	Search(query string, limit int) ([]model.Candidate, error)
}

type LocalRetriever struct {
	store     *store.DB
	tokenizer token.Tokenizer
}

func New(db *store.DB) LocalRetriever {
	return LocalRetriever{store: db, tokenizer: token.EstimateTokenizer{CharsPerToken: 4}}
}

func (r LocalRetriever) Search(query string, limit int) ([]model.Candidate, error) {
	facts, err := r.store.Facts(false)
	if err != nil {
		return nil, err
	}
	sources, err := r.store.Sources(false)
	if err != nil {
		return nil, err
	}
	queryTerms := terms(query)
	docs := make([]searchDoc, 0, len(facts)+len(sources))
	for _, fact := range facts {
		var value any
		_ = json.Unmarshal(fact.Value, &value)
		text := fact.Key + " " + stringify(value) + " " + fact.SourcePath
		docs = append(docs, searchDoc{
			id: fact.ID, kind: "fact", key: fact.Key, value: stringify(value),
			sourcePath: fact.SourcePath, sourceHash: fact.SourceHash, text: text,
		})
	}
	for _, source := range sources {
		body, _ := os.ReadFile(source.AbsPath)
		fullText := source.Path + "\n" + string(body)
		snippet, startLine, endLine := snippetForQuery(string(body), queryTerms)
		docs = append(docs, searchDoc{
			id: source.ID, kind: "code", sourcePath: source.Path, sourceHash: source.Hash,
			text: fullText, snippet: snippet, startLine: startLine, endLine: endLine,
		})
	}
	idf := inverseDocumentFrequency(docs)
	var candidates []model.Candidate
	for _, doc := range docs {
		score, reasons := scoreDoc(doc, queryTerms, idf)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, model.Candidate{
			ID: doc.id, Kind: doc.kind, Key: doc.key, Value: doc.value, SourcePath: doc.sourcePath,
			SourceHash: doc.sourceHash, StartLine: doc.startLine, EndLine: doc.endLine,
			Text: candidateText(doc), Score: round(score), Reasons: reasons,
			Tokens: r.tokenizer.Count(candidateText(doc)),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].SourcePath == candidates[j].SourcePath {
				return candidates[i].ID < candidates[j].ID
			}
			return candidates[i].SourcePath < candidates[j].SourcePath
		}
		return candidates[i].Score > candidates[j].Score
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

type searchDoc struct {
	id, kind, key, value, sourcePath, sourceHash, text string
	snippet                                            string
	startLine, endLine                                 int
}

func scoreDoc(doc searchDoc, queryTerms []string, idf map[string]float64) (float64, []string) {
	docTerms := terms(doc.text)
	tf := map[string]int{}
	for _, term := range docTerms {
		tf[term]++
	}
	var score float64
	var reasons []string
	for _, term := range queryTerms {
		count := tf[term]
		if count == 0 {
			continue
		}
		k1 := 1.2
		b := 0.75
		avgLen := 80.0
		docLen := float64(max(1, len(docTerms)))
		score += idf[term] * (float64(count) * (k1 + 1)) / (float64(count) + k1*(1-b+b*docLen/avgLen))
		reasons = append(reasons, "bm25:"+term)
	}
	lowerPath := strings.ToLower(doc.sourcePath)
	lowerKey := strings.ToLower(doc.key)
	for _, term := range queryTerms {
		if strings.Contains(lowerPath, term) {
			score += 2.0
			reasons = append(reasons, "path_match:"+term)
		}
		if lowerKey != "" && strings.Contains(lowerKey, term) {
			score += 1.5
			reasons = append(reasons, "symbol_match:"+term)
		}
	}
	hasQuerySignal := score > 0
	if strings.Contains(doc.text, "git.recent_file") && hasQuerySignal {
		score += 0.75
		reasons = append(reasons, "recent_file")
	}
	if doc.kind == "code" && strings.HasSuffix(lowerPath, ".go") && hasQuerySignal {
		score += 1.0
		reasons = append(reasons, "go_source")
	}
	if doc.kind == "code" && doc.startLine > 0 && hasQuerySignal {
		score += 0.5
		reasons = append(reasons, "line_window")
	}
	if generatedFile(lowerPath) {
		score -= 3.0
		reasons = append(reasons, "generated_file_penalty")
	}
	return score, reasons
}

func candidateText(doc searchDoc) string {
	if doc.kind == "code" && doc.snippet != "" {
		return doc.snippet
	}
	return doc.text
}

func snippetForQuery(body string, queryTerms []string) (string, int, int) {
	lines := strings.Split(body, "\n")
	match := 0
	for i, line := range lines {
		lineTerms := terms(line)
		for _, queryTerm := range queryTerms {
			for _, lineTerm := range lineTerms {
				if queryTerm == lineTerm || strings.Contains(lineTerm, queryTerm) {
					match = i + 1
					break
				}
			}
			if match > 0 {
				break
			}
		}
		if match > 0 {
			break
		}
	}
	if match == 0 {
		if len(lines) == 0 {
			return "", 0, 0
		}
		match = 1
	}
	start := max(1, match-3)
	end := min(len(lines), match+6)
	var builder strings.Builder
	for lineNo := start; lineNo <= end; lineNo++ {
		builder.WriteString(fmt.Sprintf("%4d | %s\n", lineNo, lines[lineNo-1]))
	}
	return strings.TrimRight(builder.String(), "\n"), start, end
}

func inverseDocumentFrequency(docs []searchDoc) map[string]float64 {
	df := map[string]int{}
	for _, doc := range docs {
		seen := map[string]bool{}
		for _, term := range terms(doc.text) {
			seen[term] = true
		}
		for term := range seen {
			df[term]++
		}
	}
	idf := map[string]float64{}
	n := float64(len(docs))
	for term, freq := range df {
		idf[term] = math.Log(1 + (n-float64(freq)+0.5)/(float64(freq)+0.5))
	}
	return idf
}

var splitter = regexp.MustCompile(`[^\pL\pN_]+`)

func terms(text string) []string {
	text = splitCamel(text)
	parts := splitter.Split(strings.ToLower(text), -1)
	var out []string
	for _, part := range parts {
		part = strings.TrimFunc(part, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' })
		if len(part) > 1 {
			out = append(out, part)
		}
	}
	return out
}

func splitCamel(text string) string {
	var builder strings.Builder
	var prev rune
	for i, r := range text {
		if i > 0 && utf8.ValidRune(prev) && unicode.IsLower(prev) && unicode.IsUpper(r) {
			builder.WriteRune(' ')
		}
		builder.WriteRune(r)
		prev = r
	}
	return builder.String()
}

func stringify(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	default:
		raw, _ := json.Marshal(typed)
		return string(raw)
	}
}

func generatedFile(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, "generated") || strings.HasSuffix(base, ".pb.go") || strings.HasSuffix(base, "_gen.go")
}

func round(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
