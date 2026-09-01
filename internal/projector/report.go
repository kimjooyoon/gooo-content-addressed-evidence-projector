package projector

import (
	"fmt"
	"strings"
)

func renderReport(report ConformanceReport) string {
	var b strings.Builder
	b.WriteString("# Content-addressed evidence projector report\n\n")
	fmt.Fprintf(&b, "Graph: `%s`\n\nSource digest: `%s`\n\nFixture digest: `%s`\n\n", report.GraphID, report.SourceDigest, report.FixtureDigest)
	fmt.Fprintf(&b, "Fixed denominator: **%d** cases: `CLOSED=%d`, `UNKNOWN=%d`, `REFUTED=%d`. Precedence is `REFUTED > UNKNOWN > CLOSED`.\n\n", report.ScenarioDenominator, report.StateCounts[DecisionClosed], report.StateCounts[DecisionUnknown], report.StateCounts[DecisionRefuted])
	b.WriteString("The `.gooo` graph defines the evidence object, chunk digest, manifest root, projection request, inclusion proof, and replay objects. Go performs parsing, deterministic fixed-size chunking, canonical manifest generation, projection, verification, and report generation.\n\n")
	b.WriteString("## Exact per-indicator pairs\n\n| case | indicator | before | after | equal | pair state |\n|---|---|---:|---:|---|---|\n")
	for _, item := range report.Cases { for _, pair := range item.Comparison.Pairs { before := pair.BeforeString; after := pair.AfterString; if pair.BeforeInt != nil { before = fmt.Sprintf("%d", *pair.BeforeInt); after = fmt.Sprintf("%d", *pair.AfterInt) }; fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %t | `%s` |\n", item.CaseID, pair.Indicator, before, after, pair.Equal, pair.State) } }
	b.WriteString("\nNo aggregate improvement, percentage, score, average, or inferred compression claim is emitted. The pairs are only valid when scenario, fixture, contract, toolchain, runner, and job match exactly.\n\n")
	b.WriteString("## Canonical cases\n\n| ordinal | case | expected | observed | mode | bytes read (baseline → candidate) | files read (baseline → candidate) |\n|---:|---|---|---|---|---:|---:|\n")
	for _, item := range report.Cases { fmt.Fprintf(&b, "| %d | `%s` | `%s` | `%s` | `%s` | %d → %d | %d → %d |\n", item.Ordinal, item.CaseID, item.Expected, item.Decision, item.Comparison.Candidate.ExecutionMode, item.Comparison.Baseline.Metrics.BytesRead, item.Comparison.Candidate.Metrics.BytesRead, item.Comparison.Baseline.Metrics.FilesRead, item.Comparison.Candidate.Metrics.FilesRead) }
	b.WriteString("\nMissing, stale, and ambiguous parent manifests are `UNKNOWN`, preserve `stage`, `step`, `reason`, `unknown_class`, `next_operation`, and `blocked_by`, and force full-fetch fallback. Chunk digest mismatch, manifest root contradiction, and authority/write escalation are `REFUTED`.\n\n")
	fmt.Fprintf(&b, "Repeated content references: %d; unique content-addressed chunks: %d.\n\n", report.DedupReferences, report.DedupUniqueChunks)
	b.WriteString("## Runtime and inventory\n\n")
	fmt.Fprintf(&b, "Repository writes: `%d`; local test executions: `%d`; local validation exact count: `%d`; cross-project required gates: `%d`; runtime write capability: `%s`; operator authoring authority: `%s`; output: `%s`; verification authority: `%s`. ROOT `README.md` excluded from inventory: `%t`.\n\n", report.Runtime.RepositoryWrites, report.Runtime.LocalTestExecutions, report.Runtime.LocalValidationExactCount, report.Runtime.CrossProjectRequiredGates, report.Runtime.RuntimeWriteCapability, report.Runtime.OperatorAuthoringAuthority, report.Runtime.OutputScope, report.Runtime.VerificationAuthority, report.Inventory.RootREADMEExcluded)
	fmt.Fprintf(&b, "Optional v0.48 live observation: `%s` (%s); required gate: `%t`.\n\n", report.LiveObservation.State, report.LiveObservation.Reason, report.LiveObservation.RequiredGate)
	b.WriteString("Failed CI runs, pull requests, tags, and releases are retained. Release publication is draft-first and audits the platform's actual `immutable=true` field.\n")
	return b.String()
}
