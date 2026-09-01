# Gooo content-addressed evidence projector

This repository projects canonical evidence into deterministic content-addressed chunks and a Merkle manifest. A caller can request one claim or cell, read only the required chunk(s), and verify an inclusion proof that connects the projection to the full semantic root.

The authoritative semantic input is `.gooo/content-addressed-evidence-projector.gooo`. It defines the evidence object, chunk digest, manifest root, projection request, inclusion proof, replay, state precedence, fixed indicator vector, and exact nine-case denominator. Go is the parser, generator, evaluator, verifier, and runtime only.

The fixed cases are exactly `CLOSED=3`, `UNKNOWN=3`, and `REFUTED=3`. Missing, stale, or ambiguous parent manifests remain `UNKNOWN` with `stage`, `step`, `reason`, `unknown_class`, `next_operation`, and `blocked_by`, and force `FULL_FETCH_FALLBACK`. Chunk digest mismatch, manifest-root contradiction, and authority/write escalation are `REFUTED`. Precedence is `REFUTED > UNKNOWN > CLOSED`.

The deterministic fixture demonstrates full replay equality, one-cell selective projection with a Merkle inclusion proof, and repeated-chunk deduplication. Baseline and candidate runs are measured in the same job with exact per-indicator pairs for `semantic_root`, `bytes_read`, `files_read`, `wall_ms`, and `peak_rss_kib`. No aggregate improvement, percentage, score, average, or inferred compression claim is produced.

The runtime accepts only an absolute, empty, caller-owned output directory outside the source repository:

```text
go run ./cmd/projector conformance \
  --source .gooo/content-addressed-evidence-projector.gooo \
  --fixture fixtures/deterministic-corpus-v1.json \
  --output /absolute/path/to/empty/caller-output \
  --root .
```

Generated output contains canonical evidence, the content-addressed manifest, projection events, inclusion proofs, replay evidence, exact pairs, a conformance report, and a human report. Chunk bytes are stored below `chunks/`. The source repository is never written by the runtime; ROOT `README.md` is excluded from inventory.

The v0.48 shared-ledger release and asset are optional observation inputs only. They are not a required cross-project gate. Without a live before/after pair in the same scenario, fixture, contract, toolchain, runner, and job, live improvement remains `UNKNOWN`.

Validation is intentionally executed by GitHub Actions only. Failed runs, pull requests, tags, and releases are retained. Release publication is draft-first and the platform's actual `immutable=true` field is audited before the release is reported complete.
