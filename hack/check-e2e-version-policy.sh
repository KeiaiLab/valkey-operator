#!/usr/bin/env bash
# e2e 가 **정책상 금지된 major 버전 승급**을 시도하지 않는지 정적 검사.
#
# webhook 은 major 승급을 거부한다("manual major version upgrade is prohibited;
# AutoUpdate automates patch/minor only"). 그런데 e2e 에 8.x → 9.x patch 를 시도하는
# spec 이 **세 곳**에 흩어져 있었고(version_upgrade_test / e2e_test / backup_restore_test),
# 한 곳씩 고치다 두 번 놓쳤다. 사람이 grep 으로 훑을 일이 아니라 CI 가 막을 일이다.
#
# 검사 대상 = `kubectl patch ... {"spec":{"version":{"version":"<X>"}}}` 형태의 리터럴.
# 승급 *출발* 버전은 spec 마다 다르므로, major 가 다른 두 버전이 같은 파일에 patch
# 대상으로 등장하는지가 아니라 **patch 리터럴의 major** 가 8 이 아닌 경우를 본다
# (현재 전 e2e 승급 시나리오의 baseline 이 8.x).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
violations=0

while IFS= read -r hit; do
	printf '  %s\n' "$hit"
	violations=$((violations + 1))
done < <(grep -rnE '"version":[[:space:]]*\{[^}]*"version":[[:space:]]*"9\.' "$root/test/e2e" --include='*.go' || true)

if [ "$violations" -gt 0 ]; then
	cat >&2 <<'MSG'
::error::e2e 가 major 버전 승급(8.x → 9.x)을 patch 로 시도한다 — webhook 이 정당하게
거부하므로 해당 spec 은 통과할 수 없다. AutoUpdate 는 patch/minor 만 자동화하고
major 는 명시적 마이그레이션을 요구한다. 승급 시나리오는 patch 승급(예: 8.1.6 → 8.1.7)
으로 작성하고, major 마이그레이션을 검증하려면 별도 마이그레이션 절차 spec 을 쓴다.
MSG
	exit 1
fi

echo "e2e version policy OK: major 승급 patch 시도 0건"
