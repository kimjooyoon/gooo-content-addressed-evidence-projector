#!/usr/bin/env bash
set -Eeuo pipefail

if test "$#" -ne 2; then
  echo "usage: conformance.sh BINARY OUTPUT" >&2
  exit 64
fi

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
binary=$1
output=$2
mkdir -p "$output"
"$binary" conformance \
  --source "$root/.gooo/content-addressed-evidence-projector.gooo" \
  --fixture "$root/fixtures/deterministic-corpus-v1.json" \
  --output "$output" \
  --root "$root"
