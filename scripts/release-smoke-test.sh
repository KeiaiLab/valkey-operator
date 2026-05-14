#!/usr/bin/env bash
# release-smoke-test.sh — Pre-release 5-stage validation.
#
# Stages: image (1) → SBOM (2) → trivy (3) → chart index (4) → smoke (5)
# Exit code: 0 = all PASS, !=0 = stage N failed.
#
# Usage: bash scripts/release-smoke-test.sh v1.0.13
#
# Refs: ROADMAP.md 'release-smoke-test.sh — port the mongodb-operator pattern'
#       (P-C.4.1)

set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "usage: $0 <version-tag>"
  exit 2
fi

IMG="ghcr.io/keiailab/valkey-operator:${VERSION}"
SBOM="dist/sbom-${VERSION}.spdx.json"
CHART_TGZ="dist/valkey-operator-${VERSION#v}.tgz"

echo "===== Stage 1/5: image existence ====="
docker buildx imagetools inspect "$IMG" >/dev/null 2>&1 || {
  echo "FAIL [stage 1]: $IMG not pullable"
  exit 1
}
echo "PASS"

echo "===== Stage 2/5: SBOM presence (SPDX) ====="
test -f "$SBOM" || {
  echo "FAIL [stage 2]: $SBOM missing"
  exit 2
}
spdx_ver=$(jq -r '.spdxVersion' "$SBOM" 2>/dev/null)
case "$spdx_ver" in
  SPDX-2.*) ;;
  *) echo "FAIL [stage 2]: SPDX version unknown ($spdx_ver)"; exit 2 ;;
esac
echo "PASS (SPDX $spdx_ver)"

echo "===== Stage 3/5: trivy HIGH/CRITICAL = 0 (fixed-only) ====="
trivy image --severity HIGH,CRITICAL --ignore-unfixed \
  --exit-code 1 --quiet "$IMG" || {
  echo "FAIL [stage 3]: HIGH/CRITICAL vulnerabilities found (fixed available)"
  exit 3
}
echo "PASS"

echo "===== Stage 4/5: helm chart index ====="
test -f "$CHART_TGZ" || {
  echo "FAIL [stage 4]: $CHART_TGZ missing"
  exit 4
}
helm lint "$CHART_TGZ" >/dev/null 2>&1 || {
  echo "FAIL [stage 4]: helm lint failed"
  exit 4
}
echo "PASS"

echo "===== Stage 5/5: smoke (kind ValkeyCluster Ready) ====="
KIND_CLUSTER="${KIND_CLUSTER:-valkey-smoke}"
if ! kind get clusters | grep -q "^${KIND_CLUSTER}$"; then
  kind create cluster --name "$KIND_CLUSTER" --wait 60s
fi
trap 'kind delete cluster --name "$KIND_CLUSTER" >/dev/null 2>&1 || true' EXIT

kind load docker-image "$IMG" --name "$KIND_CLUSTER"
helm install valkey-op "$CHART_TGZ" --wait --timeout 300s
kubectl apply -f - <<YAML
apiVersion: valkey.keiailab.com/v1alpha2
kind: Valkey
metadata:
  name: smoke
spec:
  replicas: 1
YAML
kubectl wait --for=condition=Ready valkey/smoke --timeout=180s || {
  echo "FAIL [stage 5]: Valkey Ready timeout"
  kubectl describe valkey/smoke || true
  exit 5
}
echo "PASS"

echo "===== ALL 5 STAGES PASS ====="
