package projector

import "time"

type Decision string

const (
	DecisionClosed  Decision = "CLOSED"
	DecisionUnknown Decision = "UNKNOWN"
	DecisionRefuted Decision = "REFUTED"
)

var UnknownFieldNames = []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}

const (
	GraphSchema       = "gooo/content-addressed-evidence-projector/semantic-graph/v1"
	IRSchema          = "gooo/content-addressed-evidence-projector/semantic-ir/v1"
	FixtureSchema     = "gooo/content-addressed-evidence-projector/fixture/v1"
	ManifestSchema    = "gooo/content-addressed-evidence-projector/manifest/v1"
	ProjectionSchema  = "gooo/content-addressed-evidence-projector/projection/v1"
	ProofSchema       = "gooo/content-addressed-evidence-projector/inclusion-proof/v1"
	ReplaySchema      = "gooo/content-addressed-evidence-projector/replay/v1"
	ComparisonSchema  = "gooo/content-addressed-evidence-projector/exact-pair/v1"
	ReportSchema      = "gooo/content-addressed-evidence-projector/conformance/v1"
	ToolchainVersion  = "go1.27.x"
	RunnerIdentity    = "github-actions/ubuntu-latest"
	HashAlgorithm     = "sha256"
	OutputDescription = "caller-owned temporary output only"
)

var RequiredObjectKinds = []string{
	"EVIDENCE_OBJECT",
	"CHUNK_DIGEST",
	"MANIFEST_ROOT",
	"PROJECTION_REQUEST",
	"INCLUSION_PROOF",
	"REPLAY",
}

var RequiredOutputNames = []string{
	"canonical-evidence.json",
	"content-addressed-manifest.json",
	"projection-results.ndjson",
	"inclusion-proofs.json",
	"replay-receipt.json",
	"exact-pair-comparison.json",
	"conformance-report.json",
	"report.md",
}

type SourceLocation struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type ObjectDecl struct {
	Ordinal     int            `json:"ordinal"`
	StableID    string         `json:"stable_id"`
	Kind        string         `json:"kind"`
	CanonicalBy []string       `json:"canonical_by"`
	Source      SourceLocation `json:"source_location"`
}

type CellDecl struct {
	Ordinal   int            `json:"ordinal"`
	StableID  string         `json:"stable_id"`
	Object    string         `json:"object"`
	Stage     string         `json:"stage"`
	Step      string         `json:"step"`
	Proof     string         `json:"proof_choice"`
	Indicator string         `json:"indicator_class"`
	Source    SourceLocation `json:"source_location"`
}

type RuleDecl struct {
	StableID      string         `json:"stable_id"`
	Cell          string         `json:"cell"`
	Decision      Decision       `json:"decision"`
	Reason        string         `json:"reason"`
	UnknownClass  string         `json:"unknown_class"`
	NextOperation string         `json:"next_operation"`
	Source        SourceLocation `json:"source_location"`
}

type CaseContract struct {
	Ordinal int            `json:"ordinal"`
	StableID string         `json:"stable_id"`
	Expected Decision       `json:"expected_decision"`
	Source   SourceLocation `json:"source_location"`
}

type Graph struct {
	Schema                    string         `json:"schema"`
	GraphID                   string         `json:"graph_id"`
	Release                   string         `json:"release"`
	HashAlgorithm             string         `json:"hash_algorithm"`
	ChunkSize                 int            `json:"chunk_size"`
	Precedence                []Decision     `json:"precedence"`
	Indicators                []string       `json:"indicators"`
	ExternalRequiredGates     int            `json:"cross_project_required_gates"`
	RepositoryWrites          int            `json:"repository_writes"`
	LocalTestExecutions       int            `json:"local_test_executions"`
	Toolchain                 string         `json:"toolchain"`
	Runner                    string         `json:"runner"`
	Objects                   []ObjectDecl   `json:"objects"`
	Cells                     []CellDecl     `json:"cells"`
	Rules                     map[string]RuleDecl `json:"rules"`
	Cases                     []CaseContract `json:"cases"`
	LiveProject               string         `json:"optional_live_project"`
	LiveReleaseID             int64          `json:"optional_live_release_id"`
	LiveAssetID               int64          `json:"optional_live_asset_id"`
	LiveAssetDigest           string         `json:"optional_live_asset_digest"`
}

type SemanticIR struct {
	Schema       string `json:"schema"`
	SourcePath   string `json:"source_path"`
	SourceDigest string `json:"source_digest"`
	Graph        Graph  `json:"graph"`
}

type Fixture struct {
	Schema          string         `json:"schema"`
	FixtureID       string         `json:"fixture_id"`
	HashAlgorithm   string         `json:"hash_algorithm"`
	ContractVersion string         `json:"contract_version"`
	Cases           []FixtureCase  `json:"cases"`
}

type FixtureCase struct {
	Ordinal         int                    `json:"ordinal"`
	CaseID          string                 `json:"case_id"`
	Expected        Decision               `json:"expected_decision"`
	Description     string                 `json:"description"`
	Cells           []EvidenceCell         `json:"cells"`
	Projection      ProjectionRequestInput  `json:"projection"`
	ParentManifest  ParentManifestInput    `json:"parent_manifest"`
	Authority       AuthorityInput         `json:"authority"`
	FixturePath     string                 `json:"-"`
}

type EvidenceCell struct {
	CellID      string `json:"cell_id"`
	ClaimID     string `json:"claim_id"`
	Content     string `json:"content"`
	StableValue string `json:"stable_value"`
}

type ProjectionRequestInput struct {
	RequestID     string   `json:"request_id"`
	RequestedCell string   `json:"requested_cell"`
	RequestedClaim string  `json:"requested_claim"`
	ExpectedMode  string   `json:"expected_mode"`
	DependsOn     []string `json:"depends_on"`
}

type ParentManifestInput struct {
	State               string `json:"state"`
	ManifestID          string `json:"manifest_id"`
	RootOverride        string `json:"root_override,omitempty"`
	ChunkDigestOverride string `json:"chunk_digest_override,omitempty"`
	StatusReason        string `json:"status_reason,omitempty"`
}

type AuthorityInput struct {
	RepositoryWrites int    `json:"repository_writes"`
	Authority        string `json:"authority"`
}

type EvidenceObject struct {
	Schema    string         `json:"schema"`
	CaseID    string         `json:"case_id"`
	ClaimIDs  []string       `json:"claim_ids"`
	Cells     []EvidenceCell `json:"cells"`
	Projection ProjectionRequest `json:"projection"`
}

type ProjectionRequest struct {
	Schema         string   `json:"schema"`
	RequestID      string   `json:"request_id"`
	CaseID         string   `json:"case_id"`
	RequestedCell  string   `json:"requested_cell"`
	RequestedClaim string   `json:"requested_claim"`
	ExpectedMode   string   `json:"expected_mode"`
	DependsOn      []string `json:"depends_on"`
}

type ChunkRef struct {
	Ordinal   int    `json:"ordinal"`
	CellID    string `json:"cell_id"`
	ChunkIndex int    `json:"chunk_index"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

type Manifest struct {
	Schema             string     `json:"schema"`
	ManifestID         string     `json:"manifest_id"`
	CaseID             string     `json:"case_id"`
	HashAlgorithm      string     `json:"hash_algorithm"`
	ChunkSize          int        `json:"chunk_size"`
	EvidenceObjectRoot string     `json:"evidence_object_root"`
	Chunks             []ChunkRef `json:"chunks"`
	Root               string     `json:"manifest_root"`
	SemanticRoot       string     `json:"semantic_root"`
}

type Chunk struct {
	Digest  string `json:"digest"`
	Content []byte `json:"content"`
}

type ProofStep struct {
	SiblingDigest string `json:"sibling_digest"`
	Side         string `json:"side"`
}

type InclusionProof struct {
	Schema         string      `json:"schema"`
	CaseID         string      `json:"case_id"`
	RequestID      string      `json:"request_id"`
	CellID         string      `json:"cell_id"`
	ChunkOrdinal   int         `json:"chunk_ordinal"`
	ChunkIndex     int         `json:"chunk_index"`
	ChunkDigest    string      `json:"chunk_digest"`
	ManifestRoot   string      `json:"manifest_root"`
	SemanticRoot   string      `json:"semantic_root"`
	EvidenceRoot   string      `json:"evidence_object_root"`
	LeafIndex      int         `json:"leaf_index"`
	LeafCount      int         `json:"leaf_count"`
	Steps          []ProofStep `json:"steps"`
	Verified       bool        `json:"verified"`
}

type UnknownRecord struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

func (u *UnknownRecord) Valid() bool {
	return u != nil && u.Stage != "" && u.Step != "" && u.Reason != "" &&
		u.UnknownClass != "" && u.NextOperation != "" && u.BlockedBy != nil
}

type RunMetrics struct {
	WallMS       int64 `json:"wall_ms"`
	PeakRSSKiB   int64 `json:"peak_rss_kib"`
	BytesRead    int64 `json:"bytes_read"`
	FilesRead    int64 `json:"files_read"`
	ChunksRead   int64 `json:"chunks_read"`
	ChunksReused int64 `json:"chunks_reused"`
}

type EngineRun struct {
	Engine          string      `json:"engine"`
	CaseID          string      `json:"case_id"`
	Decision        Decision    `json:"decision"`
	Reason          string      `json:"reason"`
	SemanticRoot    string      `json:"semantic_root"`
	ManifestRoot    string      `json:"manifest_root"`
	ExecutionMode   string      `json:"execution_mode"`
	Fallback        bool        `json:"full_fetch_fallback"`
	FallbackReason  string      `json:"fallback_reason,omitempty"`
	Unknown         *UnknownRecord `json:"unknown,omitempty"`
	Metrics         RunMetrics  `json:"metrics"`
	ReadDigests     []string    `json:"read_digests"`
	Proofs          []InclusionProof `json:"proofs"`
}

type IndicatorPair struct {
	Indicator       string        `json:"indicator"`
	Scenario        string        `json:"scenario"`
	Fixture         string        `json:"fixture"`
	Contract        string        `json:"contract"`
	Toolchain       string        `json:"toolchain"`
	Runner          string        `json:"runner"`
	Job             string        `json:"job"`
	BeforeInt       *int64        `json:"before_int,omitempty"`
	AfterInt        *int64        `json:"after_int,omitempty"`
	BeforeString    string        `json:"before_string,omitempty"`
	AfterString     string        `json:"after_string,omitempty"`
	ExactPair       bool          `json:"exact_pair"`
	Equal           bool          `json:"equal"`
	Improved       *bool         `json:"improved,omitempty"`
	State           Decision      `json:"state"`
	Unknown         *UnknownRecord `json:"unknown,omitempty"`
}

type CaseComparison struct {
	CaseID          string          `json:"case_id"`
	Expected        Decision        `json:"expected_decision"`
	Baseline        EngineRun       `json:"baseline"`
	Candidate       EngineRun       `json:"candidate"`
	CanonicalEqual  bool            `json:"canonical_evidence_equal"`
	SemanticRootEqual bool          `json:"semantic_root_equal"`
	Pairs           []IndicatorPair `json:"per_indicator_pairs"`
}

type CaseEvaluation struct {
	Ordinal       int             `json:"ordinal"`
	CaseID        string          `json:"case_id"`
	Expected      Decision        `json:"expected_decision"`
	Decision      Decision        `json:"decision"`
	Reason        string          `json:"reason"`
	Unknown       *UnknownRecord  `json:"unknown,omitempty"`
	Evidence      EvidenceObject  `json:"evidence"`
	EvidenceRoot  string          `json:"evidence_object_root"`
	Manifest      Manifest         `json:"manifest"`
	Projection    ProjectionRequest `json:"projection"`
	Proofs        []InclusionProof `json:"inclusion_proofs"`
	Comparison    CaseComparison   `json:"comparison"`
}

type ReplayReceipt struct {
	Schema                 string `json:"schema"`
	NormalCaseOrder        []string `json:"normal_case_order"`
	PerturbedCaseOrder     []string `json:"perturbed_case_order"`
	NormalSemanticDigest   string `json:"normal_semantic_digest"`
	PerturbedSemanticDigest string `json:"perturbed_semantic_digest"`
	Match                  bool   `json:"match"`
	State                  Decision `json:"state"`
	Reason                 string `json:"reason"`
}

type RuntimeAuthority struct {
	RepositoryWrites          int    `json:"repository_writes"`
	LocalTestExecutions       int    `json:"local_test_executions"`
	LocalValidationExactCount int    `json:"local_validation_exact_count"`
	CrossProjectRequiredGates int    `json:"cross_project_required_gates"`
	OutputLocation            string `json:"output_location"`
	OutputScope               string `json:"output_scope"`
	VerificationAuthority     string `json:"verification_authority"`
	GithubTokenSource         string `json:"github_token_source"`
	RuntimeWriteCapability    string `json:"runtime_write_capability"`
	OperatorAuthoringAuthority string `json:"operator_authoring_authority"`
	OperationalRefuted        string `json:"operational_refuted,omitempty"`
	FailedHistoryPreserved    bool   `json:"failed_history_preserved"`
}

type LiveObservation struct {
	State              Decision `json:"state"`
	Reason             string   `json:"reason"`
	Project            string   `json:"project"`
	ReleaseID          int64    `json:"release_id"`
	AssetID            int64    `json:"asset_id"`
	AssetDigest        string   `json:"asset_digest"`
	MatchedImmutable   bool     `json:"matched_immutable"`
	RequiredGate       bool     `json:"required_gate"`
}

type Inventory struct {
	DescendantDirs     int   `json:"descendant_dirs"`
	RegularFiles       int   `json:"regular_files"`
	Bytes              int64 `json:"bytes"`
	GoFiles            int   `json:"go_files"`
	GoPhysicalLines    int64 `json:"go_physical_lines"`
	GoooFiles          int   `json:"gooo_files"`
	GoooPhysicalLines  int64 `json:"gooo_physical_lines"`
	RootREADMEExcluded bool  `json:"root_readme_excluded"`
}

type ConformanceReport struct {
	Schema             string             `json:"schema"`
	GraphID            string             `json:"graph_id"`
	SourceDigest       string             `json:"source_digest"`
	FixtureDigest      string             `json:"fixture_digest"`
	ScenarioDenominator int               `json:"scenario_denominator"`
	StateCounts        map[Decision]int   `json:"state_counts"`
	ExpectedStateCounts map[Decision]int  `json:"expected_state_counts"`
	Cases              []CaseEvaluation   `json:"cases"`
	Replay             ReplayReceipt      `json:"replay"`
	DedupUniqueChunks  int                `json:"dedup_unique_chunks"`
	DedupReferences    int                `json:"dedup_references"`
	Runtime            RuntimeAuthority  `json:"runtime"`
	Inventory          Inventory          `json:"inventory"`
	LiveObservation    LiveObservation   `json:"live_observation"`
	Decision           Decision           `json:"decision"`
	Reason             string             `json:"reason"`
	GeneratedAt        time.Time          `json:"generated_at"`
}
