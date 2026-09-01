package projector

import (
	"fmt"
	"sort"
)

func buildManifest(graph Graph, evidence EvidenceObject) (Manifest, map[string]Chunk, error) {
	evidenceRoot, err := DigestCanonical(evidence)
	if err != nil { return Manifest{}, nil, err }
	refs := []ChunkRef{}
	chunks := map[string]Chunk{}
	ordinal := 1
	for _, cell := range evidence.Cells {
		content := []byte(cell.Content)
		if len(content) == 0 { content = []byte{} }
		chunkIndex := 0
		for offset := 0; offset < len(content) || (len(content) == 0 && chunkIndex == 0); {
			end := offset + graph.ChunkSize
			if end > len(content) { end = len(content) }
			payload := append([]byte(nil), content[offset:end]...)
			digest := DigestBytes(payload)
			if _, exists := chunks[digest]; !exists { chunks[digest] = Chunk{Digest: digest, Content: payload} }
			refs = append(refs, ChunkRef{Ordinal: ordinal, CellID: cell.CellID, ChunkIndex: chunkIndex, Digest: digest, Size: len(payload)})
			ordinal++
			chunkIndex++
			offset = end
			if len(content) == 0 { break }
		}
	}
	root := merkleRoot(refs)
	manifest := Manifest{Schema: ManifestSchema, ManifestID: "manifest-" + evidence.CaseID, CaseID: evidence.CaseID, HashAlgorithm: graph.HashAlgorithm, ChunkSize: graph.ChunkSize, EvidenceObjectRoot: evidenceRoot, Chunks: refs, Root: root}
	manifest.SemanticRoot = semanticRoot(root, evidenceRoot, evidence.CaseID)
	return manifest, chunks, nil
}

func parentManifest(current Manifest, input ParentManifestInput) (Manifest, bool) {
	if input.State != "valid" { return Manifest{}, false }
	parent := current
	parent.Chunks = append([]ChunkRef(nil), current.Chunks...)
	if input.ManifestID != "" { parent.ManifestID = input.ManifestID }
	if input.ChunkDigestOverride != "" && len(parent.Chunks) > 0 {
		parent.Chunks[0].Digest = input.ChunkDigestOverride
	}
	if input.RootOverride != "" { parent.Root = input.RootOverride }
	return parent, true
}

func parentChunkMismatch(current, parent Manifest) bool {
	if len(current.Chunks) != len(parent.Chunks) { return true }
	for i := range current.Chunks {
		if current.Chunks[i] != parent.Chunks[i] { return true }
	}
	return false
}

func uniqueChunkDigests(refs []ChunkRef) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, ref := range refs {
		if !seen[ref.Digest] { result = append(result, ref.Digest); seen[ref.Digest] = true }
	}
	return result
}

func refsForCell(manifest Manifest, cellID string) []int {
	result := []int{}
	for i, ref := range manifest.Chunks { if ref.CellID == cellID { result = append(result, i) } }
	return result
}

func readMetrics(refs []ChunkRef, chunks map[string]Chunk, deduplicate bool, startedAt int64) (RunMetrics, []string, error) {
	selected := refs
	if deduplicate {
		seen := map[string]bool{}
		selected = []ChunkRef{}
		for _, ref := range refs { if !seen[ref.Digest] { selected = append(selected, ref); seen[ref.Digest] = true } }
	}
	read := []string{}
	var bytesRead int64
	for _, ref := range selected {
		chunk, ok := chunks[ref.Digest]
		if !ok { return RunMetrics{}, nil, fmt.Errorf("chunk %s is not present in content-addressed store", ref.Digest) }
		if DigestBytes(chunk.Content) != ref.Digest { return RunMetrics{}, nil, fmt.Errorf("chunk digest mismatch for %s", ref.Digest) }
		bytesRead += int64(len(chunk.Content))
		read = append(read, ref.Digest)
	}
	return RunMetrics{BytesRead: bytesRead, FilesRead: int64(len(selected)), ChunksRead: int64(len(selected)), ChunksReused: int64(len(refs) - len(selected)), WallMS: startedAt}, read, nil
}

func sortedRefs(refs []ChunkRef) []ChunkRef {
	result := append([]ChunkRef(nil), refs...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].Ordinal < result[j].Ordinal })
	return result
}
