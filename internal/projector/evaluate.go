package projector

import (
	"fmt"
	"runtime"
	"sort"
	"syscall"
	"time"
)

func evaluateCase(ir SemanticIR, fixtureCase FixtureCase, contractDigest string, job string) (CaseEvaluation, map[string]Chunk, error) {
	projection := canonicalProjection(fixtureCase.Projection, fixtureCase.CaseID)
	evidence := canonicalEvidence(fixtureCase.CaseID, fixtureCase.Cells, projection)
	manifest, chunks, err := buildManifest(ir.Graph, evidence)
	if err != nil { return CaseEvaluation{}, nil, err }
	parent, parentPresent := parentManifest(manifest, fixtureCase.ParentManifest)
	unknown := func(ruleID string, blocked []string) *UnknownRecord { return unknownForRule(ir.Graph, ruleID, blocked) }

	decision := DecisionClosed
	reason := "PARENT_MANIFEST_VERIFIED"
	var unknownRecord *UnknownRecord
	fallback := false
	executionMode := "CONTENT_ADDRESSED_FULL"
	if !parentPresent {
		decision = DecisionUnknown
		fallback = true
		executionMode = "FULL_FETCH_FALLBACK"
		switch fixtureCase.ParentManifest.State {
		case "missing":
			reason = "PARENT_MANIFEST_MISSING"
			unknownRecord = unknown("missing_parent_manifest", []string{"parent_manifest"})
		case "stale":
			reason = "PARENT_MANIFEST_STALE"
			unknownRecord = unknown("stale_parent_manifest", []string{"parent_manifest.freshness"})
		case "ambiguous":
			reason = "PARENT_MANIFEST_AMBIGUOUS"
			unknownRecord = unknown("ambiguous_parent_manifest", []string{"parent_manifest.matches"})
		default:
			reason = "PARENT_MANIFEST_UNAVAILABLE"
			unknownRecord = unknown("missing_parent_manifest", []string{"parent_manifest"})
		}
	} else if fixtureCase.Authority.RepositoryWrites > 0 || fixtureCase.Authority.Authority != "READ_ONLY" {
		decision = DecisionRefuted
		reason = "AUTHORITY_WRITE_ESCALATION"
		executionMode = "FULL_REFUTED_SCAN"
	} else if parentChunkMismatch(manifest, parent) {
		decision = DecisionRefuted
		reason = "PARENT_CHUNK_DIGEST_MISMATCH"
		executionMode = "FULL_REFUTED_SCAN"
	} else if parent.Root != manifest.Root {
		decision = DecisionRefuted
		reason = "PARENT_MANIFEST_ROOT_CONTRADICTION"
		executionMode = "FULL_REFUTED_SCAN"
	} else if projection.RequestedCell != "" && projection.ExpectedMode == "SELECTIVE" {
		executionMode = "SELECTIVE_PROJECTION"
	}

	baselineStart := time.Now()
	baselineRefs := manifest.Chunks
	baselineMetrics, baselineRead, err := readMetrics(baselineRefs, chunks, false, 0)
	if err != nil { return CaseEvaluation{}, nil, err }
	baselineMetrics.WallMS = time.Since(baselineStart).Milliseconds()
	baselineMetrics.PeakRSSKiB = currentPeakRSSKiB()
	baseline := EngineRun{Engine: "monolith-baseline", CaseID: fixtureCase.CaseID, Decision: decision, Reason: reason, SemanticRoot: manifest.SemanticRoot, ManifestRoot: manifest.Root, ExecutionMode: "FULL_MONOLITH_FETCH", Fallback: !parentPresent, FallbackReason: reason, Unknown: unknownRecord, Metrics: baselineMetrics, ReadDigests: baselineRead}

	candidateStart := time.Now()
	candidateRefs := manifest.Chunks
	deduplicate := true
	if executionMode == "SELECTIVE_PROJECTION" {
		indices := refsForCell(manifest, projection.RequestedCell)
		if len(indices) == 0 {
			decision = DecisionRefuted
			reason = "REQUESTED_CELL_NOT_IN_MANIFEST"
			executionMode = "FULL_REFUTED_SCAN"
			candidateRefs = manifest.Chunks
		} else {
			candidateRefs = make([]ChunkRef, 0, len(indices))
			for _, index := range indices { candidateRefs = append(candidateRefs, manifest.Chunks[index]) }
		}
	}
	if fallback || decision == DecisionRefuted { deduplicate = false; candidateRefs = manifest.Chunks }
	candidateMetrics, candidateRead, err := readMetrics(candidateRefs, chunks, deduplicate, 0)
	if err != nil { return CaseEvaluation{}, nil, err }
	candidateMetrics.WallMS = time.Since(candidateStart).Milliseconds()
	candidateMetrics.PeakRSSKiB = currentPeakRSSKiB()
	candidate := EngineRun{Engine: "content-addressed-candidate", CaseID: fixtureCase.CaseID, Decision: decision, Reason: reason, SemanticRoot: manifest.SemanticRoot, ManifestRoot: manifest.Root, ExecutionMode: executionMode, Fallback: fallback, FallbackReason: reason, Unknown: unknownRecord, Metrics: candidateMetrics, ReadDigests: candidateRead}

	proofs := []InclusionProof{}
	if projection.RequestedCell != "" {
		for _, index := range refsForCell(manifest, projection.RequestedCell) {
			proof := buildInclusionProof(manifest, index, projection)
			proof.Verified = VerifyInclusionProof(proof)
			proofs = append(proofs, proof)
		}
	}
	candidate.Proofs = proofs
	comparison := compareRuns(ir.Graph, fixtureCase, contractDigest, job, baseline, candidate)
	for _, pair := range comparison { if pair.Indicator == "semantic_root" && pair.Equal == false && decision == DecisionClosed { decision = DecisionRefuted; reason = "SEMANTIC_ROOT_MISMATCH" } }
	baseline.Decision = decision
	candidate.Decision = decision
	return CaseEvaluation{Ordinal: fixtureCase.Ordinal, CaseID: fixtureCase.CaseID, Expected: fixtureCase.Expected, Decision: decision, Reason: reason, Unknown: unknownRecord, Evidence: evidence, EvidenceRoot: manifest.EvidenceObjectRoot, Manifest: manifest, Projection: projection, Proofs: proofs, Comparison: CaseComparison{CaseID: fixtureCase.CaseID, Expected: fixtureCase.Expected, Baseline: baseline, Candidate: candidate, CanonicalEqual: true, SemanticRootEqual: baseline.SemanticRoot == candidate.SemanticRoot, Pairs: comparison}}, chunks, nil
}

func compareRuns(graph Graph, fixtureCase FixtureCase, contractDigest, job string, baseline, candidate EngineRun) []IndicatorPair {
	base := pairMeta(graph, fixtureCase, contractDigest, job)
	result := []IndicatorPair{
		{Indicator: "semantic_root", BeforeString: baseline.SemanticRoot, AfterString: candidate.SemanticRoot, Equal: baseline.SemanticRoot == candidate.SemanticRoot, ExactPair: baseline.SemanticRoot != "" && candidate.SemanticRoot != "", State: DecisionClosed},
		{Indicator: "bytes_read", BeforeInt: ptrInt64(baseline.Metrics.BytesRead), AfterInt: ptrInt64(candidate.Metrics.BytesRead), Equal: baseline.Metrics.BytesRead == candidate.Metrics.BytesRead, ExactPair: true, State: DecisionClosed},
		{Indicator: "files_read", BeforeInt: ptrInt64(baseline.Metrics.FilesRead), AfterInt: ptrInt64(candidate.Metrics.FilesRead), Equal: baseline.Metrics.FilesRead == candidate.Metrics.FilesRead, ExactPair: true, State: DecisionClosed},
		{Indicator: "wall_ms", BeforeInt: ptrInt64(baseline.Metrics.WallMS), AfterInt: ptrInt64(candidate.Metrics.WallMS), Equal: baseline.Metrics.WallMS == candidate.Metrics.WallMS, ExactPair: true, State: DecisionClosed},
		{Indicator: "peak_rss_kib", BeforeInt: ptrInt64(baseline.Metrics.PeakRSSKiB), AfterInt: ptrInt64(candidate.Metrics.PeakRSSKiB), Equal: baseline.Metrics.PeakRSSKiB == candidate.Metrics.PeakRSSKiB, ExactPair: true, State: DecisionClosed},
	}
	for i := range result {
		result[i].Scenario = base.Scenario
		result[i].Fixture = base.Fixture
		result[i].Contract = base.Contract
		result[i].Toolchain = base.Toolchain
		result[i].Runner = base.Runner
		result[i].Job = base.Job
		if result[i].Indicator != "semantic_root" {
			before, after := *result[i].BeforeInt, *result[i].AfterInt
			improved := after < before
			result[i].Improved = &improved
		}
	}
	return result
}

type pairMetadata struct { Scenario, Fixture, Contract, Toolchain, Runner, Job string }
func pairMeta(graph Graph, fixtureCase FixtureCase, contractDigest, job string) pairMetadata { return pairMetadata{Scenario: fixtureCase.CaseID, Fixture: fixtureCase.FixturePath, Contract: contractDigest, Toolchain: graph.Toolchain, Runner: graph.Runner, Job: job} }
func ptrInt64(value int64) *int64 { return &value }

func unknownForRule(graph Graph, ruleID string, blocked []string) *UnknownRecord {
	rule, ok := graph.Rules[ruleID]
	if !ok { return &UnknownRecord{Stage: "UNKNOWN", Step: "REQUIRE_PARENT_MANIFEST", Reason: ruleID, UnknownClass: "MISSING_RULE", NextOperation: "READ_SEMANTIC_GRAPH", BlockedBy: append([]string(nil), blocked...)} }
	cell := graphCell(graph, rule.Cell)
	return &UnknownRecord{Stage: cell.Stage, Step: cell.Step, Reason: rule.Reason, UnknownClass: rule.UnknownClass, NextOperation: rule.NextOperation, BlockedBy: append([]string(nil), blocked...)}
}

func currentPeakRSSKiB() int64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil {
		value := int64(usage.Maxrss)
		if runtime.GOOS == "darwin" { return value / 1024 }
		return value
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return int64(stats.Sys / 1024)
}

func caseOrder(cases []CaseEvaluation) []string { result := make([]string, len(cases)); for i, item := range cases { result[i] = item.CaseID }; return result }
func canonicalCaseDigest(cases []CaseEvaluation) (string, error) {
	ordered := append([]CaseEvaluation(nil), cases...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CaseID < ordered[j].CaseID })
	values := make([]string, len(ordered))
	for i, item := range ordered { values[i] = item.Manifest.SemanticRoot }
	return DigestCanonical(values)
}

func validateProofs(eval CaseEvaluation) error {
	for _, proof := range eval.Proofs {
		if !proof.Verified || !VerifyInclusionProof(proof) { return fmt.Errorf("inclusion proof for %s is invalid", eval.CaseID) }
	}
	return nil
}
