package projector

import "testing"

func TestMerkleInclusionProofRoundTrip(t *testing.T) {
	refs := []ChunkRef{
		{Ordinal: 1, CellID: "a", ChunkIndex: 0, Digest: DigestBytes([]byte("a")), Size: 1},
		{Ordinal: 2, CellID: "b", ChunkIndex: 0, Digest: DigestBytes([]byte("b")), Size: 1},
		{Ordinal: 3, CellID: "c", ChunkIndex: 1, Digest: DigestBytes([]byte("c")), Size: 1},
	}
	manifest := Manifest{Schema: ManifestSchema, CaseID: "proof-case", Root: merkleRoot(refs), EvidenceObjectRoot: "sha256:evidence", SemanticRoot: "sha256:semantic", Chunks: refs}
	proof := buildInclusionProof(manifest, 2, ProjectionRequest{Schema: ProjectionSchema, RequestID: "request", CaseID: "proof-case"})
	if !VerifyInclusionProof(proof) { t.Fatal("expected inclusion proof to verify") }
	proof.ChunkDigest = DigestBytes([]byte("tampered"))
	if VerifyInclusionProof(proof) { t.Fatal("tampered inclusion proof verified") }
}

func TestRepeatedContentUsesOneChunk(t *testing.T) {
	graph := Graph{HashAlgorithm: HashAlgorithm, ChunkSize: 64}
	evidence := canonicalEvidence("dedup-case", []EvidenceCell{
		{CellID: "a", ClaimID: "claim-a", Content: "shared"},
		{CellID: "b", ClaimID: "claim-b", Content: "shared"},
	}, ProjectionRequest{Schema: ProjectionSchema, CaseID: "dedup-case"})
	manifest, chunks, err := buildManifest(graph, evidence)
	if err != nil { t.Fatal(err) }
	if len(manifest.Chunks) != 2 || len(chunks) != 1 { t.Fatalf("got %d references and %d unique chunks", len(manifest.Chunks), len(chunks)) }
}

func TestUnknownRecordRequiresSixFields(t *testing.T) {
	if !(&UnknownRecord{Stage: "stage", Step: "step", Reason: "reason", UnknownClass: "class", NextOperation: "next", BlockedBy: []string{"blocked"}}).Valid() { t.Fatal("complete UNKNOWN record rejected") }
	if (&UnknownRecord{Stage: "stage", Step: "step", Reason: "reason", UnknownClass: "class", NextOperation: "next"}).Valid() { t.Fatal("UNKNOWN record without blocked_by accepted") }
}
