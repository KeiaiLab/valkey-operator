#!/usr/bin/env bash
# release-smoke-test.sh — 첫 publish 또는 release 직후 8층 smoke 검증.
#
# 검증 항목 (8 층):
#   1. GH Release tag 존재 + asset 첨부 (.tgz)
#   2. GHCR image manifest 가져오기 (digest 일치)
#   3. GitHub Pages built status
#   4. Helm repo index.yaml fetch + version entry
#   5. Helm provenance(.tgz.prov) fetch + public key 검증
#   6. helm pull + helm template (default + all-features)
#   7. trivy image post-publish vulnerability scan (HIGH+CRITICAL)
#   8. cosign verify image + verify-attestation (SLSA L2 provenance) — ADR-0033
#
# 사용법:
#   scripts/release-smoke-test.sh                        # Chart.yaml 의 현재 version
#   scripts/release-smoke-test.sh v0.1.0-alpha.1         # 특정 version
#
# 환경: gh CLI 인증 + helm + curl 필요. trivy/cosign optional.
# COSIGN_PUBLIC_KEY env (cosign.pub 경로) 설정 시 cosign image 검증 활성.
#
# Exit code: 0 = 모두 PASS, 1 = 1건 이상 fail.

set -uo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CHART_NAME="$(awk '/^name:/ { print $2; exit }' "$REPO_DIR"/charts/*/Chart.yaml)"
# GH repo name 은 git remote 에서 추출 (chart name 과 다를 수 있음 — postgresql-operator
# chart 가 postgres-operator GH repo 에서 publish 되는 패턴 등).
REMOTE_URL="$(cd "$REPO_DIR" && git remote get-url origin 2>/dev/null)"
GH_OWNER="$(echo "$REMOTE_URL" | sed -E 's|.*[:/]([^/]+)/[^/]+\.git$|\1|; s|.*[:/]([^/]+)/[^/]+$|\1|')"
GH_REPO="$(echo "$REMOTE_URL" | sed -E 's|.*[:/][^/]+/([^/]+)\.git$|\1|; s|.*[:/][^/]+/([^/]+)$|\1|')"
HELM_REPO_URL="https://${GH_OWNER}.github.io/${GH_REPO}"

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  CHART_VER="$(awk '/^version:/ { print $2; exit }' "$REPO_DIR"/charts/*/Chart.yaml)"
  VERSION="v${CHART_VER}"
fi
TAG_VER="${VERSION#v}"

PASS=0
FAIL=0

# gh-pages CDN 인덱싱 / Pages build 큐잉 등 *비결정적 지연* 을 흡수하기 위한 retry.
# 정상 publish 면 첫 시도 즉시 통과(fast-path), 지연되어도 ~3 분 내 수렴.
SMOKE_RETRY_ATTEMPTS="${SMOKE_RETRY_ATTEMPTS:-12}"
SMOKE_RETRY_SLEEP="${SMOKE_RETRY_SLEEP:-15}"

pass() { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

# retry_check <pass_msg> <fail_msg> <cmd...>
# cmd 가 exit 0 이면 pass, attempts 모두 실패하면 fail. 첫 시도 통과 시 sleep 0.
retry_check() {
  local pass_msg="$1" fail_msg="$2"
  shift 2
  local i=1
  while [ "$i" -le "$SMOKE_RETRY_ATTEMPTS" ]; do
    if "$@" >/dev/null 2>&1; then
      if [ "$i" -eq 1 ]; then
        pass "$pass_msg"
      else
        pass "$pass_msg (attempt ${i}/${SMOKE_RETRY_ATTEMPTS})"
      fi
      return 0
    fi
    [ "$i" -lt "$SMOKE_RETRY_ATTEMPTS" ] && sleep "$SMOKE_RETRY_SLEEP"
    i=$((i+1))
  done
  fail "$fail_msg (after ${SMOKE_RETRY_ATTEMPTS} attempts × ${SMOKE_RETRY_SLEEP}s)"
  return 1
}

echo "════════════════════════════════════════════════════════════════"
echo " release-smoke-test  ${GH_OWNER}/${GH_REPO}  ${VERSION}"
echo "════════════════════════════════════════════════════════════════"

# 1. GH Release + assets (chart .tgz + SBOM)
echo ""
echo "▸ [1/8] GH Release tag + assets"
if gh release view "$VERSION" -R "${GH_OWNER}/${GH_REPO}" >/dev/null 2>&1; then
  pass "release ${VERSION} 존재"
  ASSETS="$(gh release view "$VERSION" -R "${GH_OWNER}/${GH_REPO}" --json assets --jq '.assets[].name')"
  if echo "$ASSETS" | grep -q "${CHART_NAME}-${TAG_VER}.tgz"; then pass "chart .tgz asset 첨부"; else fail "chart .tgz asset 누락"; fi
  if echo "$ASSETS" | grep -q "${CHART_NAME}-${TAG_VER}.tgz.prov"; then
    pass "chart .tgz.prov asset 첨부 — Helm provenance"
  else
    fail "chart .tgz.prov asset 누락 — Artifact Hub Signed badge 불가"
  fi
  if echo "$ASSETS" | grep -Eq "${CHART_NAME}-${VERSION}\.spdx\.json"; then
    pass "SBOM (SPDX) asset 첨부 — supply chain 표준"
  else
    fail "SBOM asset 누락 (${CHART_NAME}-${VERSION}.spdx.json) — make sbom 후 gh release upload 필요"
  fi
else
  fail "release ${VERSION} 없음"
fi

# 2. GHCR image
echo ""
echo "▸ [2/8] GHCR image manifest"
# Image name 은 GH repo name 을 따름 (chart name 과 다를 수 있음 — postgresql-operator
# chart 가 ghcr.io/keiailab/postgres-operator 로 push 되는 패턴 등).
IMAGE_REF="ghcr.io/${GH_OWNER}/${GH_REPO}:${VERSION}"
if docker manifest inspect "$IMAGE_REF" >/dev/null 2>&1; then
  DIGEST="$(docker manifest inspect "$IMAGE_REF" 2>/dev/null | jq -r '.config.digest // .digest // .manifests[0].digest' 2>/dev/null | head -1)"
  pass "image ${IMAGE_REF} (digest: ${DIGEST:0:19}...)"
else
  fail "image ${IMAGE_REF} manifest fetch 실패"
fi

# 3. GitHub Pages — build 가 queued/building 상태에서 시작될 수 있어 retry.
echo ""
echo "▸ [3/8] GitHub Pages status"
_check_pages_built() {
  local status
  status="$(gh api "repos/${GH_OWNER}/${GH_REPO}/pages/builds" --jq '.[0].status' 2>/dev/null || echo missing)"
  [ "$status" = "built" ]
}
retry_check \
  "Pages status=built" \
  "Pages status≠built" \
  _check_pages_built

# 4. Helm repo index.yaml — gh-pages CDN 반영 지연 흡수를 위해 fetch+grep retry.
echo ""
echo "▸ [4/8] Helm repo index.yaml fetch"
INDEX_FILE="/tmp/release-smoke-index-$$.yaml"
_fetch_index_only() { curl -sfo "$INDEX_FILE" "${HELM_REPO_URL}/index.yaml"; }
_fetch_index_with_version() {
  curl -sfo "$INDEX_FILE" "${HELM_REPO_URL}/index.yaml" \
    && grep -Fq "version: ${TAG_VER}" "$INDEX_FILE"
}
if retry_check \
     "index.yaml fetch" \
     "index.yaml fetch 실패 (${HELM_REPO_URL}/index.yaml)" \
     _fetch_index_only; then
  SIZE=$(wc -c < "$INDEX_FILE" | tr -d ' ')
  echo "    (${SIZE} bytes)"
  retry_check \
    "index.yaml 에 version: ${TAG_VER} 존재" \
    "index.yaml 에 version: ${TAG_VER} 누락" \
    _fetch_index_with_version
fi
rm -f "$INDEX_FILE"

# 5. Helm provenance file + signature verification
echo ""
echo "▸ [5/8] Helm provenance(.tgz.prov) + PGP signature"
PROV_URL="${HELM_REPO_URL}/${CHART_NAME}-${TAG_VER}.tgz.prov"
CHART_URL="${HELM_REPO_URL}/${CHART_NAME}-${TAG_VER}.tgz"
CHART_FILE="/tmp/${CHART_NAME}-${TAG_VER}.tgz"
PROV_FILE="/tmp/${CHART_NAME}-${TAG_VER}-smoke.tgz.prov"
_fetch_provenance() { curl -sfo "$PROV_FILE" "$PROV_URL"; }
_fetch_chart_archive() { curl -sfo "$CHART_FILE" "$CHART_URL"; }
if retry_check \
     "provenance fetch ${CHART_NAME}-${TAG_VER}.tgz.prov" \
     "provenance fetch 실패 (${PROV_URL})" \
     _fetch_provenance; then
  SIZE=$(wc -c < "$PROV_FILE" | tr -d ' ')
  echo "    (${SIZE} bytes)"
  cp "$PROV_FILE" "${CHART_FILE}.prov"
  retry_check \
    "chart archive fetch ${CHART_NAME}-${TAG_VER}.tgz" \
    "chart archive fetch 실패 (${CHART_URL})" \
    _fetch_chart_archive
  if command -v gpg >/dev/null 2>&1; then
    META_FILE="/tmp/${CHART_NAME}-${TAG_VER}-artifacthub-repo.yml"
    PUB_ASC="/tmp/${CHART_NAME}-${TAG_VER}-artifacthub-signing.asc"
    PUB_RING="/tmp/${CHART_NAME}-${TAG_VER}-artifacthub-pubring.gpg"
    GPG_HOME="/tmp/${CHART_NAME}-${TAG_VER}-gnupg"
    rm -rf "$GPG_HOME"
    mkdir -p "$GPG_HOME"
    chmod 700 "$GPG_HOME"
    if curl -sfo "$META_FILE" "${HELM_REPO_URL}/artifacthub-repo.yml" \
        && awk '
          /-----BEGIN PGP PUBLIC KEY BLOCK-----/ { in_key=1 }
          in_key {
            sub(/^    /, "")
            print
          }
          /-----END PGP PUBLIC KEY BLOCK-----/ { in_key=0 }
        ' "$META_FILE" > "$PUB_ASC" \
        && grep -q "BEGIN PGP PUBLIC KEY BLOCK" "$PUB_ASC" \
        && GNUPGHOME="$GPG_HOME" gpg --batch --import "$PUB_ASC" >/dev/null 2>&1 \
        && GNUPGHOME="$GPG_HOME" gpg --batch --export > "$PUB_RING" \
        && helm verify "$CHART_FILE" --keyring "$PUB_RING" >/dev/null 2>&1; then
      pass "helm verify: Artifact Hub signingKey 로 provenance 검증"
    else
      fail "helm verify 실패 — artifacthub-repo.yml signingKey 또는 .tgz.prov 불일치"
    fi
    rm -rf "$GPG_HOME" "$META_FILE" "$PUB_ASC" "$PUB_RING"
  else
    fail "gpg 미설치 — Helm provenance 검증 불가"
  fi
fi

# 6. helm pull + template
echo ""
echo "▸ [6/8] helm pull + template (default + all-features)"
TMP_REPO="smoke-test-$$"
# helm pull 자체는 인덱스 캐시 의존 — 매 시도마다 repo update 강제하여
# gh-pages 신규 entry 가 client cache 에 반영될 시간을 준다.
_helm_update_and_pull() {
  helm repo update "$TMP_REPO" >/dev/null 2>&1 \
    && helm pull "${TMP_REPO}/${CHART_NAME}" --version "${TAG_VER}" --destination /tmp --prov >/dev/null 2>&1
}
if helm repo add "$TMP_REPO" "${HELM_REPO_URL}" >/dev/null 2>&1; then
  TMP_TGZ="/tmp/${CHART_NAME}-${TAG_VER}-smoke.tgz"
  if retry_check \
       "helm pull ${TMP_REPO}/${CHART_NAME} --version ${TAG_VER}" \
       "helm pull ${TMP_REPO}/${CHART_NAME} 실패" \
       _helm_update_and_pull; then
    PULLED_TGZ="/tmp/${CHART_NAME}-${TAG_VER}.tgz"
    if [ -f "$PULLED_TGZ" ]; then
      SIZE=$(stat -f%z "$PULLED_TGZ" 2>/dev/null || stat -c%s "$PULLED_TGZ")
      pass "chart .tgz ${SIZE} bytes"
      if [ -f "${PULLED_TGZ}.prov" ]; then
        pass "helm pull --prov: provenance 동시 다운로드"
      else
        fail "helm pull --prov: provenance 파일 누락"
      fi
      # default values render
      if helm template smoke "$PULLED_TGZ" --namespace "${CHART_NAME}-system" >/dev/null 2>&1; then
        pass "helm template (default values)"
      else
        fail "helm template (default values)"
      fi
      ICON_URL="$(helm show chart "$PULLED_TGZ" 2>/dev/null | awk '/^icon:/ { print $2; exit }')"
      if [ -n "$ICON_URL" ] && curl -fsSL -o /dev/null "$ICON_URL"; then
        pass "chart icon URL fetch"
      else
        fail "chart icon URL fetch 실패 (${ICON_URL:-empty})"
      fi
      # all features ON (valkey 한정 — 다른 repo 는 silent skip)
      if helm template smoke "$PULLED_TGZ" --namespace "${CHART_NAME}-system" \
          --set features.cluster.enabled=true \
          --set features.backup.enabled=true \
          --set features.autoscaling.enabled=true >/dev/null 2>&1; then
        pass "helm template (features.cluster/backup/autoscaling=true)"
      else
        echo "  ○ helm template (features.* 가드 부재 — chart 별로 다름, skip)"
      fi
      rm -f "$PULLED_TGZ"
    fi
  fi
  helm repo remove "$TMP_REPO" >/dev/null 2>&1
else
  fail "helm repo add ${HELM_REPO_URL} 실패"
fi

# 7. trivy image post-publish vulnerability scan (exit-code 기반)
echo ""
echo "▸ [7/8] trivy image post-publish scan (HIGH+CRITICAL, fixed only)"
if command -v trivy >/dev/null 2>&1; then
  TRIVY_OUT="/tmp/release-smoke-trivy-$$.txt"
  # --exit-code 1 → CVE 검출 시 exit 1 (정직한 fail). --ignore-unfixed → fix 가능한 것만.
  if trivy image --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1 \
       --quiet --no-progress --skip-version-check "$IMAGE_REF" > "$TRIVY_OUT" 2>&1; then
    pass "trivy image: 0 HIGH+CRITICAL (fixed CVE 없음)"
  else
    fail "trivy image: HIGH/CRITICAL CVE 검출 — $TRIVY_OUT 참조"
    head -20 "$TRIVY_OUT" | sed 's/^/    /'
  fi
  rm -f "$TRIVY_OUT"
else
  echo "  ○ trivy 미설치 — skip (brew install trivy 권장)"
fi

# 8. cosign verify image + attestation (SLSA L2 provenance) — ADR-0033
echo ""
echo "▸ [8/8] cosign verify image + verify-attestation (ADR-0033)"
if command -v cosign >/dev/null 2>&1; then
  if [[ -z "${COSIGN_PUBLIC_KEY:-}" ]]; then
    echo "  ○ COSIGN_PUBLIC_KEY 미설정 — skip (export COSIGN_PUBLIC_KEY=/path/to/cosign.pub)"
  else
    # 7a. image 서명 검증.
    if cosign verify --key "$COSIGN_PUBLIC_KEY" "$IMAGE_REF" >/dev/null 2>&1; then
      pass "cosign verify image: 서명 통과"
    else
      fail "cosign verify image: 서명 부재 또는 무효 — make sign-image 후 재시도"
    fi
    # 7b. SLSA L2 provenance 검증.
    if cosign verify-attestation --type slsaprovenance --key "$COSIGN_PUBLIC_KEY" "$IMAGE_REF" >/dev/null 2>&1; then
      pass "cosign verify-attestation: SLSA L2 provenance 통과"
    else
      fail "cosign verify-attestation: provenance 부재 또는 무효 — make attest-provenance 후 재시도"
    fi
  fi
else
  echo "  ○ cosign 미설치 — skip (brew install cosign 권장)"
fi

# Summary
echo ""
echo "════════════════════════════════════════════════════════════════"
echo " RESULT: ${PASS} PASS / ${FAIL} FAIL"
echo "════════════════════════════════════════════════════════════════"

[ "$FAIL" -eq 0 ]
