package projector

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func LoadGraph(path string) (SemanticIR, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SemanticIR{}, err
	}
	graph, err := parseGraph(path, raw)
	if err != nil {
		return SemanticIR{}, err
	}
	return SemanticIR{Schema: IRSchema, SourcePath: filepath.ToSlash(path), SourceDigest: DigestBytes(raw), Graph: graph}, nil
}

func parseGraph(path string, raw []byte) (Graph, error) {
	graph := Graph{Schema: GraphSchema, Rules: map[string]RuleDecl{}, Objects: []ObjectDecl{}, Cells: []CellDecl{}, Cases: []CaseContract{}}
	seen := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		parts := strings.Split(line, "|")
		kind := parts[0]
		if kind == "protocol" {
			if len(parts) != 2 || parts[1] != "gooo/content-addressed-evidence-projector/v1" {
				return Graph{}, fmt.Errorf("line %d: invalid protocol declaration", lineNumber)
			}
			continue
		}
		values := map[string]string{}
		for _, part := range parts[1:] {
			pair := strings.SplitN(part, "=", 2)
			if len(pair) != 2 || pair[0] == "" {
				return Graph{}, fmt.Errorf("line %d: invalid field %q", lineNumber, part)
			}
			values[pair[0]] = pair[1]
		}
		location := SourceLocation{Path: filepath.ToSlash(path), Line: lineNumber, Column: 1}
		switch kind {
		case "protocol":
			if len(parts) != 2 || parts[1] != "gooo/content-addressed-evidence-projector/v1" {
				return Graph{}, fmt.Errorf("line %d: invalid protocol declaration", lineNumber)
			}
		case "graph":
			if graph.GraphID != "" {
				return Graph{}, fmt.Errorf("line %d: duplicate graph declaration", lineNumber)
			}
			var err error
			graph.GraphID = values["id"]
			graph.Release = values["release"]
			graph.HashAlgorithm = values["hash"]
			graph.ChunkSize, err = integer(values, "chunk_size")
			if err != nil {
				return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			graph.Precedence = parseDecisions(values["precedence"])
			graph.Indicators = split(values["indicators"])
			graph.ExternalRequiredGates, err = integer(values, "cross_project_required_gates")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			graph.RepositoryWrites, err = integer(values, "repository_writes")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			graph.LocalTestExecutions, err = integer(values, "local_test_executions")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			graph.Toolchain = values["toolchain"]
			graph.Runner = values["runner"]
			graph.LiveProject = values["live_project"]
			graph.LiveReleaseID, err = integer64(values, "live_release_id")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			graph.LiveAssetID, err = integer64(values, "live_asset_id")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			graph.LiveAssetDigest = values["live_asset_digest"]
		case "object":
			ordinal, err := integer(values, "ordinal")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			object := ObjectDecl{Ordinal: ordinal, StableID: values["id"], Kind: values["kind"], CanonicalBy: split(values["canonical_by"]), Source: location}
			if object.StableID == "" || object.Kind == "" { return Graph{}, fmt.Errorf("line %d: incomplete object", lineNumber) }
			if err := register(seen, object.StableID, "object", lineNumber); err != nil { return Graph{}, err }
			graph.Objects = append(graph.Objects, object)
		case "cell":
			ordinal, err := integer(values, "ordinal")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			cell := CellDecl{Ordinal: ordinal, StableID: values["id"], Object: values["object"], Stage: values["stage"], Step: values["step"], Proof: values["proof"], Indicator: values["indicator"], Source: location}
			if cell.StableID == "" || cell.Object == "" || cell.Stage == "" || cell.Step == "" || cell.Proof == "" || cell.Indicator == "" { return Graph{}, fmt.Errorf("line %d: incomplete cell", lineNumber) }
			if err := register(seen, cell.StableID, "cell", lineNumber); err != nil { return Graph{}, err }
			graph.Cells = append(graph.Cells, cell)
		case "rule":
			cell := values["cell"]
			rule := RuleDecl{StableID: values["id"], Cell: cell, Decision: Decision(values["decision"]), Reason: values["reason"], UnknownClass: values["unknown_class"], NextOperation: values["next_operation"], Source: location}
			if rule.StableID == "" || rule.Cell == "" || rule.Reason == "" || rule.UnknownClass == "" || rule.NextOperation == "" { return Graph{}, fmt.Errorf("line %d: incomplete rule", lineNumber) }
			if rule.Decision != DecisionUnknown && rule.Decision != DecisionRefuted && rule.Decision != DecisionClosed { return Graph{}, fmt.Errorf("line %d: unsupported rule decision", lineNumber) }
			if _, exists := graph.Rules[rule.StableID]; exists { return Graph{}, fmt.Errorf("line %d: duplicate rule %s", lineNumber, rule.StableID) }
			graph.Rules[rule.StableID] = rule
		case "case":
			ordinal, err := integer(values, "ordinal")
			if err != nil { return Graph{}, fmt.Errorf("line %d: %w", lineNumber, err) }
			item := CaseContract{Ordinal: ordinal, StableID: values["id"], Expected: Decision(values["expected"]), Source: location}
			if item.StableID == "" || item.Expected == "" { return Graph{}, fmt.Errorf("line %d: incomplete case", lineNumber) }
			if item.Expected != DecisionClosed && item.Expected != DecisionUnknown && item.Expected != DecisionRefuted { return Graph{}, fmt.Errorf("line %d: unsupported case decision", lineNumber) }
			if err := register(seen, item.StableID, "case", lineNumber); err != nil { return Graph{}, err }
			graph.Cases = append(graph.Cases, item)
		default:
			return Graph{}, fmt.Errorf("line %d: unsupported declaration %q", lineNumber, kind)
		}
	}
	if err := scanner.Err(); err != nil { return Graph{}, err }
	if err := validateGraph(graph); err != nil { return Graph{}, err }
	return graph, nil
}

func LoadFixture(path string, graph Graph) (Fixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil { return Fixture{}, err }
	var fixture Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil { return Fixture{}, err }
	if fixture.Schema != FixtureSchema || fixture.FixtureID == "" || fixture.HashAlgorithm != HashAlgorithm { return Fixture{}, errors.New("fixture schema or hash algorithm is invalid") }
	if len(fixture.Cases) != len(graph.Cases) { return Fixture{}, fmt.Errorf("fixture has %d cases, graph requires %d", len(fixture.Cases), len(graph.Cases)) }
	counts := map[Decision]int{}
	seen := map[string]bool{}
	for i := range fixture.Cases {
		item := &fixture.Cases[i]
		item.FixturePath = filepath.ToSlash(path)
		if item.Ordinal != i+1 || item.CaseID == "" || seen[item.CaseID] { return Fixture{}, fmt.Errorf("fixture case %d is not unique and ordered", i+1) }
		seen[item.CaseID] = true
		contract := graphCase(graph, item.CaseID)
		if contract.StableID == "" || contract.Expected != item.Expected { return Fixture{}, fmt.Errorf("fixture case %s does not match graph contract", item.CaseID) }
		counts[item.Expected]++
	}
	if counts[DecisionClosed] != 3 || counts[DecisionUnknown] != 3 || counts[DecisionRefuted] != 3 { return Fixture{}, fmt.Errorf("fixture denominator must be CLOSED=3 UNKNOWN=3 REFUTED=3") }
	return fixture, nil
}

func validateGraph(graph Graph) error {
	if graph.Schema != GraphSchema || graph.GraphID == "" || graph.Release == "" || graph.HashAlgorithm != HashAlgorithm || graph.ChunkSize < 1 || graph.Toolchain != ToolchainVersion || graph.Runner != RunnerIdentity || graph.ExternalRequiredGates != 0 || graph.RepositoryWrites != 0 || graph.LocalTestExecutions != 0 {
		return errors.New("graph metadata violates the read-only or deterministic contract")
	}
	if len(graph.Precedence) != 3 || graph.Precedence[0] != DecisionRefuted || graph.Precedence[1] != DecisionUnknown || graph.Precedence[2] != DecisionClosed { return errors.New("precedence must be REFUTED,UNKNOWN,CLOSED") }
	wantIndicators := []string{"semantic_root", "bytes_read", "files_read", "wall_ms", "peak_rss_kib"}
	if !sameStrings(graph.Indicators, wantIndicators) { return errors.New("indicator vector is not the fixed per-indicator vector") }
	if len(graph.Objects) != len(RequiredObjectKinds) || len(graph.Cells) != len(RequiredObjectKinds) || len(graph.Cases) != 9 { return errors.New("graph must declare six semantic objects, six cells, and nine cases") }
	for i, object := range graph.Objects { if object.Ordinal != i+1 || object.Kind != RequiredObjectKinds[i] { return fmt.Errorf("object %d is not the required semantic object", i+1) } }
	for i, cell := range graph.Cells { if cell.Ordinal != i+1 { return fmt.Errorf("cell %d is not contiguous", i+1) }; if !objectExists(graph.Objects, cell.Object) { return fmt.Errorf("cell %s refers to missing object", cell.StableID) } }
	for i, item := range graph.Cases { if item.Ordinal != i+1 { return fmt.Errorf("case %d is not contiguous", i+1) } }
	for _, id := range []string{"missing_parent_manifest", "stale_parent_manifest", "ambiguous_parent_manifest", "chunk_digest_mismatch", "manifest_root_contradiction", "authority_write_escalation"} { if _, ok := graph.Rules[id]; !ok { return fmt.Errorf("missing rule %s", id) } }
	return nil
}

func register(seen map[string]string, id, kind string, line int) error {
	if previous, ok := seen[id]; ok { return fmt.Errorf("line %d: stable ID %s duplicates %s", line, id, previous) }
	seen[id] = kind
	return nil
}

func integer(values map[string]string, key string) (int, error) { value, ok := values[key]; if !ok { return 0, fmt.Errorf("missing integer field %s", key) }; result, err := strconv.Atoi(value); if err != nil { return 0, fmt.Errorf("invalid integer field %s", key) }; return result, nil }
func integer64(values map[string]string, key string) (int64, error) { value, ok := values[key]; if !ok || value == "" { return 0, nil }; result, err := strconv.ParseInt(value, 10, 64); if err != nil { return 0, fmt.Errorf("invalid integer field %s", key) }; return result, nil }
func split(value string) []string { if value == "" { return []string{} }; return strings.Split(value, ",") }
func parseDecisions(value string) []Decision { values := split(value); result := make([]Decision, len(values)); for i, item := range values { result[i] = Decision(item) }; return result }
func sameStrings(left, right []string) bool { if len(left) != len(right) { return false }; for i := range left { if left[i] != right[i] { return false } }; return true }
func objectExists(objects []ObjectDecl, id string) bool { for _, object := range objects { if object.StableID == id { return true } }; return false }
func graphCase(graph Graph, id string) CaseContract { for _, item := range graph.Cases { if item.StableID == id { return item } }; return CaseContract{} }
func graphCell(graph Graph, id string) CellDecl { for _, item := range graph.Cells { if item.StableID == id { return item } }; return CellDecl{} }
