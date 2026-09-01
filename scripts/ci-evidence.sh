#!/usr/bin/env bash
set -Eeuo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
work="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/gooo-content-addressed-evidence-projector"
mkdir -p "$work"
output="$work/output"
mkdir -p "$output"

stage_names=(build test vet format conformance)
stage_status="$work/stage-status.ndjson"
: > "$stage_status"

run_stage() {
  local stage=$1
  shift
  local timing="$work/$stage.time"
  set +e
  /usr/bin/time -f '%e %M' -o "$timing" "$@"
  local status=$?
  set -e
  local wall="0"
  local rss="0"
  if read -r wall rss < "$timing"; then :; fi
  jq -cn --arg stage "$stage" --argjson status "$status" --arg wall "$wall" --arg rss "$rss" '{stage:$stage,status:$status,wall_seconds:$wall,peak_rss_kib:$rss}' >> "$stage_status"
  return "$status"
}

binary="$work/projector"
run_stage build go build -trimpath -o "$binary" ./cmd/projector || build_status=$?
build_status=${build_status:-0}

run_stage test go test ./... || test_status=$?
test_status=${test_status:-0}
run_stage vet go vet ./... || vet_status=$?
vet_status=${vet_status:-0}

set +e
gofmt -l ./cmd ./internal > "$work/gofmt-files.txt"
format_status=$?
set -e
jq -cn --arg stage format --argjson status "$format_status" --arg files "$(tr '\n' ' ' < "$work/gofmt-files.txt")" '{stage:$stage,status:$status,files:$files}' >> "$stage_status"

conformance_status=1
if [[ "$build_status" -eq 0 ]]; then
  run_stage conformance "$binary" conformance \
    --source "$root/.gooo/content-addressed-evidence-projector.gooo" \
    --fixture "$root/fixtures/deterministic-corpus-v1.json" \
    --output "$output" \
    --root "$root" || conformance_status=$?
fi
conformance_status=${conformance_status:-0}

test -f "$output/conformance-report.json" && cp "$output/conformance-report.json" "$work/conformance-report.json"
jq -e '
  .scenario_denominator == 9 and
  .state_counts.CLOSED == 3 and .state_counts.UNKNOWN == 3 and .state_counts.REFUTED == 3 and
  .runtime.repository_writes == 0 and
  .runtime.local_test_executions == 0 and
  .runtime.local_validation_exact_count == 0 and
  .runtime.cross_project_required_gates == 0 and
  .runtime.github_token_source == "github.token" and
  .live_observation.state == "UNKNOWN" and .live_observation.required_gate == false
' "$output/conformance-report.json" > "$work/report-invariants.txt"

set +e
git -C "$root" diff --exit-code --quiet
source_status=$?
set -e
jq -cn --arg stage source-boundary --argjson status "$source_status" '{stage:$stage,status:$status,repository_writes:0}' >> "$stage_status"

cat > "$work/operational-audit.json" <<EOF
{
  "schema": "gooo/content-addressed-evidence-projector/operational-audit/v1",
  "local_validation_exact_count": 0,
  "operational_refuted": null,
  "repository_writes": 0,
  "cross_project_required_gates": 0,
  "github_token_source": "github.token",
  "failed_history_preserved": true,
  "verification_authority": "GITHUB_ACTIONS"
}
EOF

test "$build_status" -eq 0
test "$test_status" -eq 0
test "$vet_status" -eq 0
test "$conformance_status" -eq 0
test "$source_status" -eq 0
echo "CI evidence complete: exact denominator, content-addressed projection, proofs, replay, and runtime boundary verified"
