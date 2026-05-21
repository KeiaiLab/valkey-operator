#!/usr/bin/env bash
# release-smoke-test.sh — Pre-release 6-stage validation.
#
# Stages: image (1) → SBOM (2) → trivy (3) → chart index (4) → smoke (5)
#         → helm chart provenance (6, Artifact Hub Signed badge gate)
# Exit code: 0 = all PASS, !=0 = stage N failed.
#
# Usage: bash scripts/release-smoke-test.sh v1.0.13
#
# Env overrides:
#   HELM_REPO_URL   default https://keiailab.github.io/valkey-operator
#   CHART_NAME      default valkey-operator
#   PUB_RING        default /tmp/valkey-operator-pubring.gpg
#
# Refs: docs/ROADMAP.md 'release-smoke-test.sh — port the mongodb-operator pattern'
#       (P-C.4.1) + ADR-0044 (Artifact Hub Signed badge) + issue #187

set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "usage: $0 <version-tag>"
  exit 2
fi

IMG="ghcr.io/keiailab/valkey-operator:${VERSION}"
SBOM="dist/sbom-${VERSION}.spdx.json"
CHART_TGZ="dist/valkey-operator-${VERSION#v}.tgz"

echo "===== Stage 1/6: image existence ====="
docker buildx imagetools inspect "$IMG" >/dev/null 2>&1 || {
  echo "FAIL [stage 1]: $IMG not pullable"
  exit 1
}
echo "PASS"

echo "===== Stage 2/6: SBOM presence (SPDX) ====="
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

echo "===== Stage 3/6: trivy HIGH/CRITICAL = 0 (fixed-only) ====="
trivy image --severity HIGH,CRITICAL --ignore-unfixed \
  --exit-code 1 --quiet "$IMG" || {
  echo "FAIL [stage 3]: HIGH/CRITICAL vulnerabilities found (fixed available)"
  exit 3
}
echo "PASS"

echo "===== Stage 4/6: helm chart index ====="
test -f "$CHART_TGZ" || {
  echo "FAIL [stage 4]: $CHART_TGZ missing"
  exit 4
}
helm lint "$CHART_TGZ" >/dev/null 2>&1 || {
  echo "FAIL [stage 4]: helm lint failed"
  exit 4
}
echo "PASS"

echo "===== Stage 5/6: smoke (kind ValkeyCluster Ready) ====="
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

echo "===== Stage 6/6: helm chart provenance (Artifact Hub Signed badge) ====="
# ADR-0044: Helm chart provenance 검증 → Artifact Hub Signed badge 활성 조건.
# 실패 시 chart 위변조 또는 PGP key drift 가능성 — release block.
HELM_REPO_URL="${HELM_REPO_URL:-https://keiailab.github.io/valkey-operator}"
CHART_NAME="${CHART_NAME:-valkey-operator}"
TAG_VER="${VERSION#v}"
TMP_REPO="valkey-operator-smoke-tmp"
PUB_RING="${PUB_RING:-/tmp/valkey-operator-pubring.gpg}"

# 6.1. artifacthub-repo.yml → signKey.url 로 PGP public key fetch.
curl -fsSL "${HELM_REPO_URL}/artifacthub-repo.yml" > /tmp/artifacthub-repo.yml || {
  echo "FAIL [stage 6]: ${HELM_REPO_URL}/artifacthub-repo.yml 미접근 — Artifact Hub Signed badge 불가"
  exit 6
}
SIGN_KEY_URL=$(grep -E '^\s*url:' /tmp/artifacthub-repo.yml | head -1 | awk '{print $2}' | tr -d '"')
if [ -z "$SIGN_KEY_URL" ]; then
  echo "FAIL [stage 6]: artifacthub-repo.yml 에 signKey.url 부재 — Artifact Hub Signed badge 불가"
  exit 6
fi
curl -fsSL "$SIGN_KEY_URL" > /tmp/valkey-operator-pubkey.asc || {
  echo "FAIL [stage 6]: signKey.url fetch 실패 ($SIGN_KEY_URL)"
  exit 6
}
grep -q "BEGIN PGP PUBLIC KEY BLOCK" /tmp/valkey-operator-pubkey.asc || {
  echo "FAIL [stage 6]: BEGIN PGP PUBLIC KEY BLOCK 부재 in pubkey.asc"
  exit 6
}

# 6.2. helm pull --prov: chart .tgz + .tgz.prov 동시 다운로드.
helm repo add "$TMP_REPO" "$HELM_REPO_URL" >/dev/null 2>&1 || true
helm repo update "$TMP_REPO" >/dev/null 2>&1
helm pull "${TMP_REPO}/${CHART_NAME}" --version "${TAG_VER}" --destination /tmp --prov || {
  echo "FAIL [stage 6]: chart .tgz.prov asset 첨부 안 됨 (helm pull --prov 실패) — Artifact Hub Signed badge 불가"
  helm repo remove "$TMP_REPO" >/dev/null 2>&1 || true
  exit 6
}
helm repo remove "$TMP_REPO" >/dev/null 2>&1 || true

# 6.3. helm verify: PGP keyring 으로 .tgz.prov 무결성 검증.
gpg --dearmor < /tmp/valkey-operator-pubkey.asc > "$PUB_RING" 2>/dev/null || {
  echo "FAIL [stage 6]: pubring dearmor 실패 ($PUB_RING)"
  exit 6
}
CHART_FILE="/tmp/${CHART_NAME}-${TAG_VER}.tgz"
helm verify "$CHART_FILE" --keyring "$PUB_RING" || {
  echo "FAIL [stage 6]: chart .tgz.prov 위변조 또는 PGP key 불일치 — Artifact Hub Signed badge 불가"
  exit 6
}
echo "PASS"

echo "===== ALL 6 STAGES PASS ====="
