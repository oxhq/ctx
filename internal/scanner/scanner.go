package scanner

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oxhq/ctx/internal/model"
)

type Scanner interface {
	Scan(root string) ([]model.Fact, []model.Source, error)
}

type LocalScanner struct{}

func New() LocalScanner {
	return LocalScanner{}
}

func (LocalScanner) Scan(root string) ([]model.Fact, []model.Source, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	var sources []model.Source
	err = filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeType != 0 || shouldSkipFile(name) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		hash := sha256.Sum256(body)
		sources = append(sources, model.Source{
			ID:       sourceID(absRoot, rel),
			Root:     absRoot,
			Path:     rel,
			AbsPath:  path,
			Hash:     hex.EncodeToString(hash[:]),
			Kind:     sourceKind(rel),
			Size:     info.Size(),
			Modified: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Path < sources[j].Path })

	recency := gitRecency(absRoot)
	now := time.Now().UTC()
	var facts []model.Fact
	for _, source := range sources {
		facts = append(facts, factsForSource(source, now, recency[source.Path])...)
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].Key == facts[j].Key {
			if facts[i].SourcePath == facts[j].SourcePath {
				return facts[i].ID < facts[j].ID
			}
			return facts[i].SourcePath < facts[j].SourcePath
		}
		return facts[i].Key < facts[j].Key
	})
	return facts, sources, nil
}

func factsForSource(source model.Source, now time.Time, recent bool) []model.Fact {
	var facts []model.Fact
	add := func(key string, value any, confidence float64) {
		raw, _ := json.Marshal(value)
		id := factID(key, source.Path, string(raw), source.Hash)
		facts = append(facts, model.Fact{
			ID:         id,
			Key:        key,
			Value:      raw,
			Source:     source.ID,
			SourcePath: source.Path,
			SourceHash: source.Hash,
			Confidence: confidence,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	switch {
	case source.Path == "go.mod":
		add("project.language", "go", 0.95)
		module, deps := parseGoMod(source.AbsPath)
		if module != "" {
			add("project.go.module", module, 0.95)
		}
		for _, dep := range deps {
			add("project.go.dependency", dep, 0.85)
		}
	case strings.HasSuffix(source.Path, ".go"):
		add("project.layout.package", filepath.ToSlash(filepath.Dir(source.Path)), 0.8)
		for _, symbol := range parseGoSymbols(source.AbsPath) {
			add("project.go.symbol", symbol, 0.85)
		}
	case strings.EqualFold(filepath.Base(source.Path), "README.md") || strings.HasPrefix(source.Path, "docs/"):
		add("project.doc", source.Path, 0.8)
	case isConfig(source.Path):
		add("project.config", source.Path, 0.8)
	}
	if recent {
		add("git.recent_file", source.Path, 0.75)
	}
	return facts
}

type goSymbol struct {
	Kind string `json:"kind"`
	Line int    `json:"line"`
	Name string `json:"name"`
}

func parseGoSymbols(path string) []goSymbol {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var symbols []goSymbol
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			symbols = append(symbols, goSymbol{Kind: "func", Line: fset.Position(typed.Pos()).Line, Name: typed.Name.Name})
		case *ast.GenDecl:
			kind := typed.Tok.String()
			for _, spec := range typed.Specs {
				switch specTyped := spec.(type) {
				case *ast.TypeSpec:
					symbols = append(symbols, goSymbol{Kind: kind, Line: fset.Position(specTyped.Pos()).Line, Name: specTyped.Name.Name})
				case *ast.ValueSpec:
					for _, name := range specTyped.Names {
						symbols = append(symbols, goSymbol{Kind: kind, Line: fset.Position(name.Pos()).Line, Name: name.Name})
					}
				}
			}
		}
	}
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Line == symbols[j].Line {
			return symbols[i].Name < symbols[j].Name
		}
		return symbols[i].Line < symbols[j].Line
	})
	return symbols
}

func parseGoMod(path string) (string, []string) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer file.Close()
	var module string
	var deps []string
	inRequireBlock := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "module ") {
			module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			continue
		}
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			fields := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(fields) >= 2 {
				deps = append(deps, fields[0]+" "+fields[1])
			}
			continue
		}
		if inRequireBlock {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				deps = append(deps, fields[0]+" "+fields[1])
			}
		}
	}
	sort.Strings(deps)
	return module, deps
}

func gitRecency(root string) map[string]bool {
	cmd := exec.Command("git", "-C", root, "log", "--name-only", "--pretty=format:", "-n", "20")
	out, err := cmd.Output()
	if err != nil {
		return map[string]bool{}
	}
	recent := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(filepath.ToSlash(line))
		if line != "" {
			recent[line] = true
		}
	}
	return recent
}

func sourceID(root, rel string) string {
	sum := sha256.Sum256([]byte(root + "\x00" + rel))
	return "src_" + hex.EncodeToString(sum[:])[:16]
}

func factID(key, path, value, hash string) string {
	sum := sha256.Sum256([]byte(key + "\x00" + path + "\x00" + value + "\x00" + hash))
	return "fact_" + hex.EncodeToString(sum[:])[:16]
}

func sourceKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".md":
		return "doc"
	case ".json", ".yaml", ".yml", ".toml", ".mod", ".sum":
		return "config"
	default:
		return "file"
	}
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".ctx", "vendor", "node_modules", "dist", "build", "target", ".cache":
		return true
	default:
		return false
	}
}

func shouldSkipFile(name string) bool {
	return strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".dll") || strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg")
}

func isConfig(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "package.json", "composer.json", "cargo.toml", "pyproject.toml", "go.mod", "go.sum":
		return true
	default:
		return strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".json")
	}
}
