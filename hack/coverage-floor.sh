#!/usr/bin/env bash
# 커버리지 하한 래칫.
#
# .codecov.yml 은 target 70% / informational:false 로 "차단 게이트"처럼 보이지만,
# codecov 상태는 PR 체크 목록에 **나타나지 않는다** (업로드가 조용히 무시됨 —
# 업로드 스텝이 fail_ci_if_error:false 라 실패해도 초록). 즉 설정만 있고 아무것도
# 막지 않는 상태였다. 외부 서비스·토큰 의존 없이 CI 안에서 직접 판정한다.
#
# 하한은 "현재 실측치"에서 출발한다 — 도달 불가능한 목표를 걸면 전 PR 이 즉시
# 막혀 결국 게이트를 꺼버리게 된다. 목적은 **회귀 차단(래칫)** 이고, 하한 상향은
# 테스트를 실제로 보강한 PR 이 같이 올린다.
set -euo pipefail

profile="${1:-cover.out}"
floor="${COVERAGE_FLOOR:?COVERAGE_FLOOR must be set}"

[ -s "$profile" ] || { echo "::error::coverage profile not found: $profile"; exit 1; }

total="$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/,"",$3); print $3}')"
[ -n "$total" ] || { echo "::error::could not parse total coverage from $profile"; exit 1; }

echo "total coverage: ${total}% (floor ${floor}%)"

if awk -v t="$total" -v f="$floor" 'BEGIN { exit !(t < f) }'; then
	echo "::error::coverage ${total}% dropped below floor ${floor}% — add tests, or lower the floor in the same PR with a stated reason"
	exit 1
fi

# 하한을 크게 웃돌면 래칫을 올리라고 알린다 (차단은 아님).
if awk -v t="$total" -v f="$floor" 'BEGIN { exit !(t > f + 3) }'; then
	echo "::notice::coverage ${total}% is well above the floor ${floor}% — consider raising COVERAGE_FLOOR"
fi
