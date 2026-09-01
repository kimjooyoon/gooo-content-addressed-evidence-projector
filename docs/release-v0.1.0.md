# v0.1.0 release record

The first release is published from an annotated tag after the implementation pull request is merged and main CI succeeds. The release workflow creates or preserves a draft, attaches the source archive, checksums, and release manifest, then publishes it and verifies the GitHub API's actual `immutable=true` field.

The release manifest is generated in runner temporary storage and binds the tag object, commit, archive bytes, archive digest, and asset size. Existing releases and failed historical runs are preserved.
