#!/usr/bin/env bash
# artifacthub-register.sh — ArtifactHub repo 등록 후 repositoryID placeholder 교체.
#
# ADR-0024 T01 자동화. ArtifactHub UI 등록 자체는 web 만 지원하므로 본 스크립트는:
#  1. 등록 절차를 명확히 echo
#  2. 사용자가 UUID 를 입력하면
#  3. charts/artifacthub-repo.yml 의 placeholder 를 교체
#  4. helm-publish 재실행 가이드
#
# 사용법:
#   scripts/artifacthub-register.sh                # 대화형
#   scripts/artifacthub-register.sh <uuid>         # 비대화형
#
# 검증: 등록 완료 후 https://artifacthub.io/api/v1/repositories/helm/valkey-operator
#      에서 repository_id == 입력 UUID 인지 curl 로 확인.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
META="$REPO_DIR/charts/artifacthub-repo.yml"
PLACEHOLDER="00000000-0000-0000-0000-000000000000"

if [ ! -f "$META" ]; then
  echo "ERROR: $META 가 없습니다." >&2
  exit 1
fi

CURRENT_ID="$(awk '/^repositoryID:/ { print $2; exit }' "$META")"

if [ "$CURRENT_ID" != "$PLACEHOLDER" ]; then
  echo "INFO: repositoryID 가 이미 교체되었습니다 ($CURRENT_ID). 추가 작업 없음."
  exit 0
fi

cat <<'EOF'
================================================================================
 ArtifactHub repo 등록 절차 (수동 — UI 만 지원)
================================================================================

 1. 브라우저로 접속:
      https://artifacthub.io/control-panel/repositories

 2. ADD REPOSITORY 클릭. 다음 입력:
      Kind:           Helm charts
      Name:           valkey-operator
      Display name:   Valkey Operator
      URL:            https://keiailab.github.io/valkey-operator
      Branch:         (gh-pages — auto-detect)

 3. SAVE 후 생성된 Repository ID (UUID 형식) 를 복사.

 4. 본 스크립트에 UUID 입력 (또는 인자로 재실행):
      scripts/artifacthub-register.sh <uuid>

================================================================================
EOF

if [ $# -eq 1 ]; then
  UUID="$1"
else
  read -r -p "Repository ID (UUID): " UUID
fi

if ! echo "$UUID" | grep -Eq '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'; then
  echo "ERROR: '$UUID' 는 valid UUID 형식이 아닙니다." >&2
  exit 1
fi

# macOS sed 호환 (BSD sed -i '')
sed -i.bak "s/$PLACEHOLDER/$UUID/" "$META"
rm -f "$META.bak"

echo ""
echo "✓ $META 의 repositoryID 를 $UUID 로 교체했습니다."
echo ""
echo "다음 단계:"
echo "  git add charts/artifacthub-repo.yml"
echo "  git commit -m 'chore(artifacthub): repositoryID = $UUID (등록 완료)'"
echo "  git push origin main"
echo "  make helm-publish    # gh-pages 의 artifacthub-repo.yml 도 갱신"
echo ""
echo "검증 (등록 후 ~30분, ArtifactHub polling 완료 시):"
echo "  curl -s https://artifacthub.io/api/v1/repositories/helm/valkey-operator | jq '.repository_id, .verified_publisher'"
