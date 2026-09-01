package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kimjooyoon/gooo-content-addressed-evidence-projector/internal/projector"
)

type options struct {
	source  string
	fixture string
	output  string
	root    string
	report  string
}

func main() {
	if len(os.Args) < 2 { fatal("command is required: check, generate, project, conformance, or verify") }
	switch os.Args[1] {
	case "check": check(os.Args[2:])
	case "generate": run(os.Args[2:], false)
	case "project": run(os.Args[2:], false)
	case "conformance": run(os.Args[2:], true)
	case "verify": verify(os.Args[2:])
	default: fatal("unknown command %q", os.Args[1])
	}
}

func parse(command string, args []string, outputRequired bool) options {
	set := flag.NewFlagSet(command, flag.ExitOnError)
	values := options{}
	set.StringVar(&values.source, "source", ".gooo/content-addressed-evidence-projector.gooo", "authoritative .gooo graph")
	set.StringVar(&values.fixture, "fixture", "fixtures/deterministic-corpus-v1.json", "own deterministic fixture")
	set.StringVar(&values.output, "output", "", "absolute empty caller-owned output directory")
	set.StringVar(&values.root, "root", ".", "source repository root")
	set.StringVar(&values.report, "report", "", "generated conformance report")
	set.Parse(args)
	if outputRequired && values.output == "" { fatal("%s requires --output", command) }
	if (command == "generate" || command == "project") && values.output == "" { fatal("%s requires --output", command) }
	return values
}

func check(args []string) {
	values := parse("check", args, false)
	ir, err := projector.LoadGraph(values.source)
	if err != nil { fatal(err.Error()) }
	fixture, err := projector.LoadFixture(values.fixture, ir.Graph)
	if err != nil { fatal(err.Error()) }
	printJSON(struct { Schema string `json:"schema"`; GraphID string `json:"graph_id"`; Objects int `json:"objects"`; Cells int `json:"cells"`; Cases int `json:"cases"`; ChunkSize int `json:"chunk_size"` }{ir.Schema, ir.Graph.GraphID, len(ir.Graph.Objects), len(ir.Graph.Cells), len(fixture.Cases), ir.Graph.ChunkSize})
}

func run(args []string, conformance bool) {
	values := parse("generate", args, conformance)
	report, err := projector.Generate(values.source, values.fixture, values.output, values.root)
	if err != nil { fatal(err.Error()) }
	if conformance {
		if err := projector.VerifyConformance(report); err != nil { fatal(err.Error()) }
	}
	printJSON(struct { Decision projector.Decision `json:"decision"`; ScenarioDenominator int `json:"scenario_denominator"`; Closed int `json:"closed"`; Unknown int `json:"unknown"`; Refuted int `json:"refuted"`; ReplayMatch bool `json:"replay_match"`; OutputDirectory string `json:"output_directory"` }{report.Decision, report.ScenarioDenominator, report.StateCounts[projector.DecisionClosed], report.StateCounts[projector.DecisionUnknown], report.StateCounts[projector.DecisionRefuted], report.Replay.Match, filepath.Clean(values.output)})
}

func verify(args []string) {
	values := parse("verify", args, false)
	if values.report == "" { fatal("verify requires --report") }
	raw, err := os.ReadFile(values.report)
	if err != nil { fatal(err.Error()) }
	var report projector.ConformanceReport
	if err := json.Unmarshal(raw, &report); err != nil { fatal(err.Error()) }
	if err := projector.VerifyConformance(report); err != nil { fatal(err.Error()) }
	printJSON(struct { Decision projector.Decision `json:"decision"`; ScenarioDenominator int `json:"scenario_denominator"` }{projector.DecisionClosed, report.ScenarioDenominator})
}

func printJSON(value any) { raw, err := json.Marshal(value); if err != nil { fatal(err.Error()) }; fmt.Println(string(raw)) }
func fatal(format string, values ...any) { fmt.Fprintf(os.Stderr, format+"\n", values...); os.Exit(1) }
