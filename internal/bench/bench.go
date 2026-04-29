package bench

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/oxhq/ctx/internal/compiler"
	"github.com/oxhq/ctx/internal/jsonutil"
	"github.com/oxhq/ctx/internal/scanner"
	"github.com/oxhq/ctx/internal/store"
	"github.com/oxhq/ctx/internal/token"
)

type RunRequest struct {
	Repo      string
	CasesPath string
	Baseline  string
}

type Case struct {
	Task                 string   `json:"task"`
	Repo                 string   `json:"repo,omitempty"`
	ExpectedTouchedAreas []string `json:"expected_touched_areas"`
	ExpectedTerms        []string `json:"expected_terms,omitempty"`
	Budget               int      `json:"budget"`
	BaselineMode         string   `json:"baseline_mode,omitempty"`
}

type Result struct {
	Cases []CaseResult `json:"cases"`
}

type CaseResult struct {
	Task                  string   `json:"task"`
	TokensNaive           int      `json:"tokens_naive"`
	TokensCompiled        int      `json:"tokens_compiled"`
	TokenReductionPercent float64  `json:"token_reduction_percent"`
	ExpectedAreaHit       bool     `json:"expected_area_hit"`
	MissingExpectedAreas  []string `json:"missing_expected_areas"`
	ExpectedTermHit       bool     `json:"expected_term_hit"`
	MissingExpectedTerms  []string `json:"missing_expected_terms"`
	ContextQualityScore   float64  `json:"context_quality_score"`
	RuntimeMillis         int64    `json:"runtime_ms"`
}

func Run(req RunRequest) (Result, error) {
	cases, err := readCases(req.CasesPath)
	if err != nil {
		return Result{}, err
	}
	var result Result
	for _, benchCase := range cases {
		start := time.Now()
		repo := req.Repo
		if benchCase.Repo != "" {
			repo = benchCase.Repo
		}
		ctxDir := filepath.Join(repo, ".ctx")
		db, err := store.Open(ctxDir)
		if err != nil {
			return Result{}, err
		}
		facts, sources, err := scanner.New().Scan(repo)
		if err != nil {
			_ = db.Close()
			return Result{}, err
		}
		if err := db.UpsertFacts(facts, sources); err != nil {
			_ = db.Close()
			return Result{}, err
		}
		packet, _, err := compiler.New(db).Compile(compiler.CompileRequest{Task: benchCase.Task, Repo: repo, Budget: benchCase.Budget})
		_ = db.Close()
		if err != nil {
			return Result{}, err
		}
		baselineMode := req.Baseline
		if baselineMode == "" {
			baselineMode = benchCase.BaselineMode
		}
		naiveTokens, err := baselineTokens(repo, baselineMode)
		if err != nil {
			return Result{}, err
		}
		missing := missingAreas(packet, benchCase.ExpectedTouchedAreas)
		missingTerms := missingTerms(packet, benchCase.ExpectedTerms)
		reduction := 0.0
		if naiveTokens > 0 {
			reduction = float64(naiveTokens-packet.Meta.TokensUsed) * 100 / float64(naiveTokens)
		}
		qualityScore := contextQualityScore(len(missing) == 0, len(missingTerms) == 0, reduction)
		result.Cases = append(result.Cases, CaseResult{
			Task: benchCase.Task, TokensNaive: naiveTokens, TokensCompiled: packet.Meta.TokensUsed,
			TokenReductionPercent: reduction, ExpectedAreaHit: len(missing) == 0,
			MissingExpectedAreas: missing, ExpectedTermHit: len(missingTerms) == 0,
			MissingExpectedTerms: missingTerms, ContextQualityScore: qualityScore,
			RuntimeMillis: time.Since(start).Milliseconds(),
		})
	}
	return result, nil
}

func WriteResults(repo string, result Result) error {
	dir := filepath.Join(repo, ".ctx", "bench")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body, err := jsonutil.MarshalStable(result)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "results.json"), body, 0o644)
}

func readCases(path string) ([]Case, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var cases []Case
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var benchCase Case
		if err := json.Unmarshal([]byte(line), &benchCase); err != nil {
			return nil, err
		}
		cases = append(cases, benchCase)
	}
	return cases, scanner.Err()
}

func baselineTokens(repo, mode string) (int, error) {
	if mode == "repomix" {
		if tokens, ok := repomixTokens(repo); ok {
			return tokens, nil
		}
	}
	return naiveTokens(repo)
}

func repomixTokens(repo string) (int, bool) {
	if _, err := exec.LookPath("repomix"); err != nil {
		return 0, false
	}
	cmd := exec.Command("repomix", "--style", "plain", "--stdout")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	return token.EstimateTokenizer{CharsPerToken: 4}.Count(string(out)), true
}

func naiveTokens(repo string) (int, error) {
	files, err := trackedOrWalkedFiles(repo)
	if err != nil {
		return 0, err
	}
	var builder strings.Builder
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		builder.WriteString("\n# ")
		builder.WriteString(file)
		builder.WriteString("\n")
		builder.Write(body)
	}
	return token.EstimateTokenizer{CharsPerToken: 4}.Count(builder.String()), nil
}

func trackedOrWalkedFiles(repo string) ([]string, error) {
	cmd := exec.Command("git", "-C", repo, "ls-files")
	out, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for i := range lines {
			lines[i] = filepath.ToSlash(strings.TrimSpace(lines[i]))
		}
		return lines, nil
	}
	var files []string
	err = filepath.WalkDir(repo, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".ctx", "vendor", "node_modules", "dist", "build", "target":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

func missingAreas(packet compiler.ContextPacket, expected []string) []string {
	var missing []string
	for _, area := range expected {
		hit := false
		for _, item := range packet.Context {
			if strings.Contains(item.SourcePath, area) || strings.Contains(item.Value, area) || strings.Contains(item.Key, area) {
				hit = true
				break
			}
		}
		if !hit {
			missing = append(missing, area)
		}
	}
	return missing
}

func missingTerms(packet compiler.ContextPacket, expected []string) []string {
	var missing []string
	var haystack string
	for _, item := range packet.Context {
		haystack += "\n" + strings.ToLower(item.Key)
		haystack += "\n" + strings.ToLower(item.Value)
		haystack += "\n" + strings.ToLower(item.SourcePath)
	}
	for _, term := range expected {
		if !strings.Contains(haystack, strings.ToLower(term)) {
			missing = append(missing, term)
		}
	}
	return missing
}

func contextQualityScore(areaHit, termHit bool, reduction float64) float64 {
	score := 0.0
	if areaHit {
		score += 0.45
	}
	if termHit {
		score += 0.35
	}
	if reduction >= 30 {
		score += 0.20
	} else if reduction > 0 {
		score += 0.20 * (reduction / 30)
	}
	return math.Round(score*10000) / 10000
}
