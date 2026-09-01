package projector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func DigestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return HashAlgorithm + ":" + hex.EncodeToString(sum[:])
}

func DigestCanonical(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(raw), nil
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func canonicalEvidence(caseID string, cells []EvidenceCell, projection ProjectionRequest) EvidenceObject {
	ordered := append([]EvidenceCell(nil), cells...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CellID != ordered[j].CellID {
			return ordered[i].CellID < ordered[j].CellID
		}
		return ordered[i].ClaimID < ordered[j].ClaimID
	})
	claims := make([]string, 0, len(ordered))
	seen := map[string]bool{}
	for _, cell := range ordered {
		if cell.ClaimID != "" && !seen[cell.ClaimID] {
			claims = append(claims, cell.ClaimID)
			seen[cell.ClaimID] = true
		}
	}
	sort.Strings(claims)
	return EvidenceObject{Schema: "gooo/content-addressed-evidence-projector/evidence-object/v1", CaseID: caseID, ClaimIDs: claims, Cells: ordered, Projection: projection}
}

func canonicalProjection(input ProjectionRequestInput, caseID string) ProjectionRequest {
	depends := append([]string(nil), input.DependsOn...)
	sort.Strings(depends)
	return ProjectionRequest{Schema: ProjectionSchema, RequestID: input.RequestID, CaseID: caseID, RequestedCell: input.RequestedCell, RequestedClaim: input.RequestedClaim, ExpectedMode: input.ExpectedMode, DependsOn: depends}
}

func semanticRoot(manifestRoot, evidenceRoot, caseID string) string {
	return DigestBytes([]byte(strings.Join([]string{"gooo/content-addressed-evidence-projector/semantic-root/v1", caseID, manifestRoot, evidenceRoot}, "|")))
}

func leafDigest(ref ChunkRef) string {
	return DigestBytes([]byte(fmt.Sprintf("gooo/content-addressed-evidence-projector/leaf/v1|%d|%s|%d|%s", ref.Ordinal, ref.CellID, ref.ChunkIndex, ref.Digest)))
}

func nodeDigest(left, right string) string {
	return DigestBytes([]byte("gooo/content-addressed-evidence-projector/node/v1|" + left + "|" + right))
}

func merkleRoot(refs []ChunkRef) string {
	if len(refs) == 0 {
		return DigestBytes([]byte("gooo/content-addressed-evidence-projector/empty-tree/v1"))
	}
	level := make([]string, len(refs))
	for i, ref := range refs {
		level[i] = leafDigest(ref)
	}
	for len(level) > 1 {
		next := make([]string, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			right := level[i]
			if i+1 < len(level) {
				right = level[i+1]
			}
			next = append(next, nodeDigest(level[i], right))
		}
		level = next
	}
	return level[0]
}

func buildInclusionProof(manifest Manifest, refIndex int, request ProjectionRequest) InclusionProof {
	refs := manifest.Chunks
	level := make([]string, len(refs))
	for i, ref := range refs {
		level[i] = leafDigest(ref)
	}
	index := refIndex
	steps := []ProofStep{}
	for len(level) > 1 {
		paired := index ^ 1
		sibling := level[index]
		side := "right"
		if paired < len(level) {
			sibling = level[paired]
			if paired < index {
				side = "left"
			}
		}
		steps = append(steps, ProofStep{SiblingDigest: sibling, Side: side})
		next := make([]string, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			right := level[i]
			if i+1 < len(level) {
				right = level[i+1]
			}
			next = append(next, nodeDigest(level[i], right))
		}
		index /= 2
		level = next
	}
	ref := refs[refIndex]
	return InclusionProof{Schema: ProofSchema, CaseID: manifest.CaseID, RequestID: request.RequestID, CellID: ref.CellID, ChunkOrdinal: ref.Ordinal, ChunkIndex: ref.ChunkIndex, ChunkDigest: ref.Digest, ManifestRoot: manifest.Root, SemanticRoot: manifest.SemanticRoot, EvidenceRoot: manifest.EvidenceObjectRoot, LeafIndex: refIndex, LeafCount: len(refs), Steps: steps}
}

func VerifyInclusionProof(proof InclusionProof) bool {
	if proof.Schema != ProofSchema || proof.ChunkDigest == "" || proof.ManifestRoot == "" || proof.LeafIndex < 0 || proof.LeafIndex >= proof.LeafCount || proof.LeafCount < 1 {
		return false
	}
	ref := ChunkRef{Ordinal: proof.ChunkOrdinal, CellID: proof.CellID, ChunkIndex: proof.ChunkIndex, Digest: proof.ChunkDigest}
	current := leafDigest(ref)
	index := proof.LeafIndex
	width := proof.LeafCount
	for _, step := range proof.Steps {
		if step.Side == "left" {
			current = nodeDigest(step.SiblingDigest, current)
		} else if step.Side == "right" {
			current = nodeDigest(current, step.SiblingDigest)
		} else {
			return false
		}
		index /= 2
		width = (width + 1) / 2
	}
	return current == proof.ManifestRoot && width == 1 && index == 0
}
