#!/usr/bin/env bash
#
# valkey-operator 수동 release 스크립트.
#
# 사용:
#   bash scripts/release.sh v0.1.0 [<registry>]
#
# 동작:
#   1. tag 형식 검증 (vMAJOR.MINOR.PATCH).
#   2. working tree clean 검증.
#   3. lefthook pre-push 동등 (lint + test) 통과 확인.
#   4. docs/CHANGELOG.md 갱신 (git-cliff 가 설치되어 있으면).
#   5. version commit + tag.
#   6. 멀티아키 이미지 빌드 + push (docker-buildx, $PLATFORMS).
#   7. install.yaml 생성 (dist/install.yaml).
#   8. GitHub Release 본문 (.release_notes.md) 생성 — 사용자가 수동 publish.
#
# 사전조건:
#   - git remote 'origin' 설정
#   - docker buildx 활성화
#   - 컨테이너 레지스트리 로그인 (docker login)
#   - (선택) git-cliff: brew install git-cliff
#
# CLAUDE.md §2 (GHA 영구 금지, RFC 0002) 준수 — 본 스크립트는 *수동* 실행.
# release tag → GitHub Release 본문 *자동* 생성 1-step workflow 는 별개
# ADR-0019 + 사용자 승인 후 추가 가능.

set -euo pipefail

usage() {
  echo "Usage: $0 <version> [registry]"
  echo "  version: vMAJOR.MINOR.PATCH (e.g. v0.1.0)"
  echo "  registry: image registry prefix (default: ghcr.io/keiailab)"
  exit 1
}

[[ $# -lt 1 ]] && usage

VERSION="$1"
REGISTRY="${2:-ghcr.io/keiailab}"
IMAGE="${REGISTRY}/valkey-operator:${VERSION}"

# 1. tag 형식 검증.
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "ERROR: version must be vMAJOR.MINOR.PATCH (got: $VERSION)" >&2
  exit 1
fi

# 2. working tree clean.
if ! git diff-index --quiet HEAD --; then
  echo "ERROR: working tree dirty. Commit or stash first." >&2
  git status --short
  exit 1
fi

# 3. lint + test 통과 확인 (lefthook pre-push 동등).
echo "==> Running lint + test (lefthook pre-push 동등)"
make lint
make test
echo "✓ lint + test PASS"

# 4. CHANGELOG (git-cliff 가 있을 때만).
if command -v git-cliff >/dev/null 2>&1; then
  echo "==> Updating docs/CHANGELOG.md (git-cliff)"
  git-cliff --tag "$VERSION" --output docs/CHANGELOG.md
  git add docs/CHANGELOG.md
  git commit -m "chore(release): CHANGELOG for $VERSION" || true
else
  echo "WARN: git-cliff not installed — skip CHANGELOG. brew install git-cliff" >&2
fi

# 5. version 정보 (PROJECT 또는 별도 version.go) 갱신은 본 스크립트 범위 외.
#    git tag 만으로 version 식별.

# 6. 멀티아키 이미지 빌드 + push.
# VERSION 명시 전달 — cycle 56 발견: 본 변수 미전달 시 Makefile fallback "dev"
# → production GHCR image 가 `--version` 명령에서 "dev" 출력 (cycle 53/54/55
# 의 ldflags chain 마지막 link).
echo "==> Building multi-arch image: $IMAGE (VERSION=$VERSION)"
make docker-buildx IMG="$IMAGE" VERSION="$VERSION"
echo "✓ image pushed: $IMAGE"

# 6.5. Supply Chain — cosign sign + SLSA L2 in-toto attestation (ADR-0033).
# Bitnami in-toto + Cloudpirates cosign 차용 — 동등 보증 수준.
# COSIGN_KEY 미설정 시 warning 후 skip (개발자 로컬 release 테스트 허용).
# RFC-0002 (GHA 영구 금지) 와의 충돌 회피: keyless OIDC 대신 keyfile 사용.
if [[ -n "${COSIGN_KEY:-}" ]] && command -v cosign >/dev/null 2>&1; then
  echo "==> Supply chain: cosign sign + SLSA L2 attest"
  make sign-image VERSION="$VERSION" COSIGN_KEY="$COSIGN_KEY"
  # SBOM 먼저 생성 — provenance subject digest 와 정합 보존.
  make sbom VERSION="$VERSION" || echo "WARN: SBOM 생성 실패 — provenance 만 진행" >&2
  make attest-provenance VERSION="$VERSION" COSIGN_KEY="$COSIGN_KEY"
  echo "✓ image signed + provenance attested"
else
  echo "WARN: COSIGN_KEY unset 또는 cosign 미설치 — supply chain attest skip" >&2
  echo "  활성 절차:"
  echo "    1. brew install cosign jq"
  echo "    2. cosign generate-key-pair  (cosign.key + cosign.pub 생성)"
  echo "    3. export COSIGN_KEY=/path/to/cosign.key COSIGN_PASSWORD=<pwd>"
  echo "    4. release.sh 재실행"
fi

# 7. install.yaml 생성.
echo "==> Generating dist/install.yaml"
make build-installer IMG="$IMAGE"
echo "✓ dist/install.yaml ready"

# 8. tag + push (이 시점 에서 release 본문 생성).
echo "==> Creating tag $VERSION"
git tag -a "$VERSION" -m "Release $VERSION"

# 9. GitHub Release 본문 stub.
{
  echo "# valkey-operator $VERSION"
  echo
  echo "## Installation"
  echo
  echo "\`\`\`sh"
  echo "kubectl apply -f https://github.com/keiailab/valkey-operator/releases/download/$VERSION/install.yaml"
  echo "\`\`\`"
  echo
  echo "## Container Image"
  echo "\`$IMAGE\`"
  echo
  echo "## Changes"
  echo
  if [[ -f docs/CHANGELOG.md ]]; then
    # 첫 # ${VERSION} section 추출
    awk "/^## /${VERSION}/{flag=1; next} /^## /{flag=0} flag" docs/CHANGELOG.md || echo "(see docs/CHANGELOG.md)"
  else
    echo "(see git log)"
  fi
  echo
  echo "## Verification"
  echo "\`\`\`sh"
  echo "kubectl -n valkey-operator-system rollout status deploy/valkey-operator-controller-manager"
  echo "\`\`\`"
} > .release_notes.md

echo
echo "==================================================="
echo "✓ Release $VERSION 준비 완료"
echo "==================================================="
echo "다음 단계 (수동):"
echo "  1. git push origin main"
echo "  2. git push origin $VERSION"
echo "  3. dist/install.yaml + .release_notes.md 를 GitHub Release 페이지에 업로드"
echo "     gh release create $VERSION --title \"$VERSION\" --notes-file .release_notes.md dist/install.yaml"
echo "  4. (옵션) registry 에서 image 검증: docker pull $IMAGE"
echo
echo "image: $IMAGE"
echo "installer: dist/install.yaml"
echo "release notes: .release_notes.md"
