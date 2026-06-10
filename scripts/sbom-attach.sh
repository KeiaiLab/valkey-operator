#!/usr/bin/env bash
# sbom-attach.sh — Generate SPDX SBOM + trivy scan + attach to release.
#
# 3-repo (mongodb/valkey/postgres) 공유 패턴. keiailab-commons/scripts 도
# 동일 stub 제공 가능.
#
# Usage: bash scripts/sbom-attach.sh <image-tag> <release-tag>
#
# Stages:
#   1. Generate SPDX SBOM (syft)
#   2. trivy HIGH/CRITICAL scan (--ignore-unfixed)
#   3. cosign sign SBOM + attach to release
#
# Refs: docs/ROADMAP.md L164-166 'Image SBOM (SPDX) + trivy ... shared script'
#       (P-C.7.1 + C.7.2 + C.7.3)

set -euo pipefail

IMG="${1:-}"
TAG="${2:-}"

if [ -z "$IMG" ] || [ -z "$TAG" ]; then
  echo "usage: $0 <image-tag> <release-tag>"
  echo "  example: $0 ghcr.io/keiailab/valkey-operator:v1.0.13 v1.0.13"
  exit 2
fi

mkdir -p dist

echo "===== Stage 1/3: SPDX SBOM generation (syft) ====="
SBOM="dist/sbom-${TAG}.spdx.json"
syft "$IMG" -o spdx-json="$SBOM"
test -s "$SBOM" || { echo "FAIL: SBOM empty"; exit 1; }
spdx_ver=$(jq -r '.spdxVersion' "$SBOM")
echo "PASS (SPDX $spdx_ver, $(stat -f%z "$SBOM") bytes)"

echo "===== Stage 2/3: trivy HIGH/CRITICAL = 0 (fixed-only) ====="
trivy image --severity HIGH,CRITICAL --ignore-unfixed \
    --exit-code 1 --quiet "$IMG" || {
  echo "FAIL: HIGH/CRITICAL vulnerabilities found"
  trivy image --severity HIGH,CRITICAL --ignore-unfixed "$IMG"
  exit 2
}
echo "PASS"

echo "===== Stage 3/3: cosign sign SBOM + attach to release ====="
cosign attest --type spdxjson --predicate "$SBOM" "$IMG"
gh release upload "$TAG" "$SBOM" --clobber
echo "PASS"

echo "===== SBOM attach complete: $TAG ====="
