package projector

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Generate(sourcePath, fixturePath, outputPath, repositoryRoot string) (ConformanceReport, error) {
	if err := EnsureOutputDirectory(outputPath, repositoryRoot); err != nil { return ConformanceReport{}, err }
	ir, err := LoadGraph(sourcePath)
	if err != nil { return ConformanceReport{}, err }
	fixture, err := LoadFixture(fixturePath, ir.Graph)
	if err != nil { return ConformanceReport{}, err }
	job := os.Getenv("GITHUB_JOB")
	if job == "" { job = "content-addressed-evidence-projector-conformance" }
	caseResults := make([]CaseEvaluation, 0, len(fixture.Cases))
	allChunks := map[string]Chunk{}
	for _, fixtureCase := range fixture.Cases {
		evaluation, chunks, err := evaluateCase(ir, fixtureCase, ir.SourceDigest, job)
		if err != nil { return ConformanceReport{}, err }
		caseResults = append(caseResults, evaluation)
		for digest, chunk := range chunks { if _, exists := allChunks[digest]; !exists { allChunks[digest] = chunk } }
	}
	normalDigest, err := canonicalCaseDigest(caseResults)
	if err != nil { return ConformanceReport{}, err }
	perturbed := append([]CaseEvaluation(nil), caseResults...)
	sort.SliceStable(perturbed, func(i, j int) bool { return perturbed[i].CaseID > perturbed[j].CaseID })
	perturbedDigest, err := canonicalCaseDigest(perturbed)
	if err != nil { return ConformanceReport{}, err }
	replay := ReplayReceipt{Schema: ReplaySchema, NormalCaseOrder: caseOrder(caseResults), PerturbedCaseOrder: caseOrder(perturbed), NormalSemanticDigest: normalDigest, PerturbedSemanticDigest: perturbedDigest, Match: normalDigest == perturbedDigest, State: DecisionClosed, Reason: "CANONICAL_REPLAY_MATCH"}
	if !replay.Match { replay.State = DecisionRefuted; replay.Reason = "CANONICAL_REPLAY_MISMATCH" }
	counts := stateCounts(caseResults)
	expected := expectedCounts(ir.Graph.Cases)
	decision := DecisionUnknown
	reason := "CONFORMANCE_REQUIRES_GITHUB_ACTIONS"
	if replay.Match && sameCounts(counts, expected) { decision = DecisionClosed; reason = "FIXED_9_CASES_AND_CANONICAL_PROJECTION" }
	if !replay.Match { decision = DecisionRefuted; reason = replay.Reason }
	inventory, err := inventoryForRoot(repositoryRoot)
	if err != nil { return ConformanceReport{}, err }
	report := ConformanceReport{Schema: ReportSchema, GraphID: ir.Graph.GraphID, SourceDigest: ir.SourceDigest, FixtureDigest: fileDigest(fixturePath), ScenarioDenominator: len(caseResults), StateCounts: counts, ExpectedStateCounts: expected, Cases: caseResults, Replay: replay, DedupUniqueChunks: len(allChunks), DedupReferences: countChunkReferences(caseResults), Runtime: RuntimeAuthority{RepositoryWrites: 0, LocalTestExecutions: 0, LocalValidationExactCount: 0, CrossProjectRequiredGates: ir.Graph.ExternalRequiredGates, OutputLocation: filepath.Clean(outputPath), OutputScope: OutputDescription, VerificationAuthority: "GITHUB_ACTIONS", GithubTokenSource: "github.token", RuntimeWriteCapability: "NONE", OperatorAuthoringAuthority: "SEPARATE", FailedHistoryPreserved: true}, Inventory: inventory, LiveObservation: LiveObservation{State: DecisionUnknown, Reason: "OPTIONAL_LIVE_PAIR_NOT_PROVIDED", Project: ir.Graph.LiveProject, ReleaseID: ir.Graph.LiveReleaseID, AssetID: ir.Graph.LiveAssetID, AssetDigest: ir.Graph.LiveAssetDigest, MatchedImmutable: false, RequiredGate: false}, Decision: decision, Reason: reason}
	if err := writeOutputs(outputPath, report, allChunks); err != nil { return ConformanceReport{}, err }
	return report, nil
}

func EnsureOutputDirectory(outputPath, repositoryRoot string) error {
	if !filepath.IsAbs(outputPath) { return errors.New("output directory must be an absolute caller-owned path") }
	root, err := filepath.Abs(repositoryRoot)
	if err != nil { return err }
	output, err := filepath.Abs(outputPath)
	if err != nil { return err }
	relative, err := filepath.Rel(root, output)
	if err != nil { return err }
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) { return errors.New("output directory must be outside the source repository") }
	info, statErr := os.Stat(output)
	if os.IsNotExist(statErr) { return nil }
	if statErr != nil { return statErr }
	if !info.IsDir() { return errors.New("output path must be a directory") }
	entries, err := os.ReadDir(output)
	if err != nil { return err }
	if len(entries) != 0 { return errors.New("caller-owned output directory must be empty") }
	return nil
}

func writeOutputs(outputPath string, report ConformanceReport, chunks map[string]Chunk) error {
	if err := os.MkdirAll(filepath.Join(outputPath, "chunks"), 0o755); err != nil { return err }
	for digest, chunk := range chunks {
		name := strings.TrimPrefix(digest, HashAlgorithm+":") + ".chunk"
		if err := os.WriteFile(filepath.Join(outputPath, "chunks", name), chunk.Content, 0o644); err != nil { return err }
	}
	evidence := make([]EvidenceObject, 0, len(report.Cases))
	manifests := make([]Manifest, 0, len(report.Cases))
	proofs := make([]InclusionProof, 0)
	comparisons := make([]CaseComparison, 0, len(report.Cases))
	for _, item := range report.Cases { evidence = append(evidence, item.Evidence); manifests = append(manifests, item.Manifest); proofs = append(proofs, item.Proofs...); comparisons = append(comparisons, item.Comparison) }
	if err := writeJSON(filepath.Join(outputPath, "canonical-evidence.json"), struct { Schema string `json:"schema"`; Cases []EvidenceObject `json:"cases"` }{Schema: "gooo/content-addressed-evidence-projector/canonical-evidence/v1", Cases: evidence}); err != nil { return err }
	if err := writeJSON(filepath.Join(outputPath, "content-addressed-manifest.json"), struct { Schema string `json:"schema"`; Manifests []Manifest `json:"manifests"`; UniqueChunks int `json:"unique_chunks"` }{Schema: ManifestSchema, Manifests: manifests, UniqueChunks: len(chunks)}); err != nil { return err }
	var projections bytes.Buffer
	for _, item := range report.Cases { raw, err := json.Marshal(struct { CaseID string `json:"case_id"`; Request ProjectionRequest `json:"request"`; Decision Decision `json:"decision"`; Mode string `json:"mode"`; Fallback bool `json:"full_fetch_fallback"` }{item.CaseID, item.Projection, item.Decision, item.Comparison.Candidate.ExecutionMode, item.Comparison.Candidate.Fallback}); if err != nil { return err }; projections.Write(raw); projections.WriteByte('\n') }
	if err := os.WriteFile(filepath.Join(outputPath, "projection-results.ndjson"), projections.Bytes(), 0o644); err != nil { return err }
	if err := writeJSON(filepath.Join(outputPath, "inclusion-proofs.json"), struct { Schema string `json:"schema"`; Proofs []InclusionProof `json:"proofs"` }{Schema: ProofSchema, Proofs: proofs}); err != nil { return err }
	if err := writeJSON(filepath.Join(outputPath, "replay-receipt.json"), report.Replay); err != nil { return err }
	if err := writeJSON(filepath.Join(outputPath, "exact-pair-comparison.json"), struct { Schema string `json:"schema"`; Cases []CaseComparison `json:"cases"` }{Schema: ComparisonSchema, Cases: comparisons}); err != nil { return err }
	if err := writeJSON(filepath.Join(outputPath, "conformance-report.json"), report); err != nil { return err }
	if err := os.WriteFile(filepath.Join(outputPath, "report.md"), []byte(renderReport(report)), 0o644); err != nil { return err }
	entries, err := os.ReadDir(outputPath)
	if err != nil { return err }
	for _, name := range RequiredOutputNames { found := false; for _, entry := range entries { if entry.Name() == name { found = true; break } }; if !found { return fmt.Errorf("missing generated artifact %s", name) } }
	return nil
}

func writeJSON(path string, value any) error { raw, err := json.MarshalIndent(value, "", "  "); if err != nil { return err }; raw = append(raw, '\n'); return os.WriteFile(path, raw, 0o644) }

func VerifyConformance(report ConformanceReport) error {
	if report.ScenarioDenominator != 9 || report.StateCounts[DecisionClosed] != 3 || report.StateCounts[DecisionUnknown] != 3 || report.StateCounts[DecisionRefuted] != 3 { return errors.New("denominator is not exactly CLOSED=3 UNKNOWN=3 REFUTED=3") }
	if !sameCounts(report.ExpectedStateCounts, report.StateCounts) { return errors.New("projected state counts do not equal expected state counts") }
	if !report.Replay.Match { return errors.New("canonical replay did not match") }
	for _, item := range report.Cases {
		if item.Decision != item.Expected { return fmt.Errorf("case %s projected %s, expected %s", item.CaseID, item.Expected, item.Decision) }
		if item.Decision == DecisionUnknown && (!item.Unknown.Valid() || !item.Comparison.Candidate.Fallback || item.Comparison.Candidate.ExecutionMode != "FULL_FETCH_FALLBACK") { return fmt.Errorf("UNKNOWN case %s does not preserve six fields and full-fetch fallback", item.CaseID) }
		if item.Decision == DecisionRefuted && item.Unknown != nil { return fmt.Errorf("REFUTED case %s contains UNKNOWN claim", item.CaseID) }
		if err := validateProofs(item); err != nil { return err }
		if !item.Comparison.CanonicalEqual || !item.Comparison.SemanticRootEqual { return fmt.Errorf("baseline/candidate semantic equality failed for %s", item.CaseID) }
		if len(item.Comparison.Pairs) != 5 { return fmt.Errorf("case %s does not have five per-indicator pairs", item.CaseID) }
		for _, pair := range item.Comparison.Pairs { if !pair.ExactPair || pair.State != DecisionClosed || pair.Scenario != item.CaseID || pair.Fixture == "" || pair.Contract == "" || pair.Toolchain != ToolchainVersion || pair.Runner != RunnerIdentity || pair.Job == "" { return fmt.Errorf("invalid exact pair for %s/%s", item.CaseID, pair.Indicator) } }
		if item.CaseID == "closed-one-cell-selective-projection" && (item.Comparison.Candidate.ExecutionMode != "SELECTIVE_PROJECTION" || item.Comparison.Candidate.Metrics.FilesRead >= item.Comparison.Baseline.Metrics.FilesRead || len(item.Proofs) == 0) { return errors.New("one-cell selective projection did not read selectively with proof") }
		if item.CaseID == "closed-repeated-chunk-dedup-reuse" && item.Comparison.Candidate.Metrics.ChunksReused < 1 { return errors.New("repeated chunk fixture did not demonstrate reuse") }
	}
	if report.Runtime.RepositoryWrites != 0 || report.Runtime.LocalTestExecutions != 0 || report.Runtime.LocalValidationExactCount != 0 || report.Runtime.CrossProjectRequiredGates != 0 || report.Runtime.OutputScope != OutputDescription || report.Runtime.VerificationAuthority != "GITHUB_ACTIONS" || report.Runtime.GithubTokenSource != "github.token" || report.Runtime.RuntimeWriteCapability != "NONE" || report.Runtime.OperatorAuthoringAuthority != "SEPARATE" || report.Runtime.OperationalRefuted != "" || !report.Runtime.FailedHistoryPreserved { return errors.New("runtime authority violates read-only contract") }
	if report.LiveObservation.State != DecisionUnknown || report.LiveObservation.RequiredGate { return errors.New("live observation must remain optional UNKNOWN") }
	return nil
}

func stateCounts(cases []CaseEvaluation) map[Decision]int { result := map[Decision]int{DecisionClosed: 0, DecisionUnknown: 0, DecisionRefuted: 0}; for _, item := range cases { result[item.Decision]++ }; return result }
func expectedCounts(cases []CaseContract) map[Decision]int { result := map[Decision]int{DecisionClosed: 0, DecisionUnknown: 0, DecisionRefuted: 0}; for _, item := range cases { result[item.Expected]++ }; return result }
func sameCounts(left, right map[Decision]int) bool { return left[DecisionClosed] == right[DecisionClosed] && left[DecisionUnknown] == right[DecisionUnknown] && left[DecisionRefuted] == right[DecisionRefuted] }
func countChunkReferences(cases []CaseEvaluation) int { total := 0; for _, item := range cases { total += len(item.Manifest.Chunks) }; return total }
func fileDigest(path string) string { raw, err := os.ReadFile(path); if err != nil { return "" }; return DigestBytes(raw) }

func inventoryForRoot(root string) (Inventory, error) {
	var inventory Inventory
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil { return walkErr }
		if entry.IsDir() { if entry.Name() == ".git" { return fs.SkipDir }; if path != root { inventory.DescendantDirs++ }; return nil }
		relative, err := filepath.Rel(root, path); if err != nil { return err }
		if relative == "README.md" { inventory.RootREADMEExcluded = true; return nil }
		if !entry.Type().IsRegular() { return nil }
		raw, err := os.ReadFile(path); if err != nil { return err }
		inventory.RegularFiles++
		inventory.Bytes += int64(len(raw))
		switch filepath.Ext(path) { case ".go": inventory.GoFiles++; inventory.GoPhysicalLines += int64(physicalLines(raw)); case ".gooo": inventory.GoooFiles++; inventory.GoooPhysicalLines += int64(physicalLines(raw)) }
		return nil
	})
	return inventory, err
}

func physicalLines(raw []byte) int { if len(raw) == 0 { return 0 }; count := bytes.Count(raw, []byte{'\n'}); if raw[len(raw)-1] != '\n' { count++ }; return count }
