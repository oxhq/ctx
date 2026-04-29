package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oxhq/ctx/internal/bench"
	"github.com/oxhq/ctx/internal/compiler"
	"github.com/oxhq/ctx/internal/scanner"
	"github.com/oxhq/ctx/internal/store"
)

var Version = "dev"

func Execute(args []string) error {
	return ExecuteWithOutput(args, io.Discard)
}

func ExecuteWithOutput(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing command")
	}
	switch args[0] {
	case "scan":
		return scanCmd(args[1:], out)
	case "compile":
		return compileCmd(args[1:], out)
	case "explain":
		return explainCmd(args[1:], out)
	case "bench":
		return benchCmd(args[1:], out)
	case "version":
		_, _ = fmt.Fprintln(out, Version)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func scanCmd(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: ctx scan <path>")
	}
	repo, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(repo, ".ctx"))
	if err != nil {
		return err
	}
	defer db.Close()
	facts, sources, err := scanner.New().Scan(repo)
	if err != nil {
		return err
	}
	if err := db.UpsertFacts(facts, sources); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "scanned %d facts from %d sources\n", len(facts), len(sources))
	return nil
}

func compileCmd(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: ctx compile <task> --repo <path> --budget <tokens> --format json --explain")
	}
	task := args[0]
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", ".", "repository path")
	budget := fs.Int("budget", 12000, "token budget")
	format := fs.String("format", "json", "output format")
	explainFlag := fs.Bool("explain", false, "write explain output")
	stateRef := fs.String("state-ref", "", "prior local context ref")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *format != "json" {
		return errors.New("only --format json is supported in v0")
	}
	absRepo, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(absRepo, ".ctx"))
	if err != nil {
		return err
	}
	defer db.Close()
	facts, sources, err := scanner.New().Scan(absRepo)
	if err != nil {
		return err
	}
	if err := db.UpsertFacts(facts, sources); err != nil {
		return err
	}
	packet, explanation, err := compiler.New(db).Compile(compiler.CompileRequest{Task: task, Repo: absRepo, Budget: *budget})
	if err != nil {
		return err
	}
	if *stateRef != "" {
		packet.Meta.ContextRef = *stateRef
	}
	if err := compiler.WriteLast(absRepo, packet, explanation); err != nil {
		return err
	}
	body, err := compiler.MarshalStable(packet)
	if err != nil {
		return err
	}
	_, _ = out.Write(body)
	_, _ = out.Write([]byte("\n"))
	if *explainFlag {
		_, _ = fmt.Fprintf(out, "explain: %s\n", filepath.Join(absRepo, ".ctx", "last_explain.json"))
	}
	return nil
}

func explainCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", ".", "repository path")
	last := fs.Bool("last", false, "show last explanation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*last {
		return errors.New("only ctx explain --last is supported in v0")
	}
	absRepo, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(absRepo, ".ctx", "last_explain.json"))
	if err != nil {
		return err
	}
	_, _ = out.Write(body)
	_, _ = out.Write([]byte("\n"))
	return nil
}

func benchCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", ".", "repository path")
	cases := fs.String("cases", "", "cases jsonl")
	baseline := fs.String("baseline", "naive", "baseline mode")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cases == "" {
		return errors.New("--cases is required")
	}
	absRepo, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	result, err := bench.Run(bench.RunRequest{Repo: absRepo, CasesPath: *cases, Baseline: *baseline})
	if err != nil {
		return err
	}
	if err := bench.WriteResults(absRepo, result); err != nil {
		return err
	}
	body, err := compiler.MarshalStable(result)
	if err != nil {
		return err
	}
	_, _ = out.Write(body)
	_, _ = out.Write([]byte("\n"))
	return nil
}
