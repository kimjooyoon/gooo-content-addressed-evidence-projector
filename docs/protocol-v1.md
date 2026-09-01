# Content-addressed evidence protocol v1

## Projection model

1. A canonical evidence object is built from the case's claim and cell content. Cells are ordered by `cell_id` and claims are ordered by stable ID.
2. Each cell's bytes are split into the fixed size declared by the `.gooo` graph. Each unique byte sequence is stored once under its SHA-256 digest. Manifest references retain cell identity and chunk position, so equal bytes can be reused without losing semantic location.
3. The manifest contains the evidence-object digest and canonical chunk references. A domain-separated Merkle tree over the references produces `manifest_root`; the semantic root binds case ID, manifest root, and evidence-object root.
4. A projection request selects a cell. The runtime reads the selected content-addressed chunks and emits one Merkle inclusion proof per selected chunk. Proof verification requires no full bundle fetch.

## Boundary states

Parent manifest absence, staleness, and ambiguity are evidence gaps, not success. They retain the six-field UNKNOWN record and use full-fetch fallback. A known chunk digest mismatch, root contradiction, or write-capable authority is a counterexample and is REFUTED.

## Measurement contract

The monolith baseline and content-addressed candidate run on the same fixture, contract digest, toolchain, runner, and CI job. The output records exact pairs for semantic-root equality, bytes read, files read, wall milliseconds, and peak RSS KiB. Missing metrics are null plus UNKNOWN; no zero is substituted. There is no aggregate improvement verdict.

The live v0.48 reference is read-only and optional. Its immutable release ID, asset ID, and digest are pinned in the graph for observation, but `cross_project_required_gates=0` keeps the own-fixture acceptance independent of another repository.
