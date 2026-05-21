# S1+ Design: valkey-operator PR Cleanup + GHA Workflow 정합 + 통합 ADR

| 메타 | 값 |
|---|---|
| 날짜 | 2026-05-21 |
| 상태 | **Accepted** (2026-05-21 사용자 결정 반영 v2.0 재작성) |
| 작성자 | keiailab — auto-cycle |
| 범위 | valkey-operator 만 (postgres / mongodb / commons / forgewise 는 별 spec) |
| 후속 | `docs/plans/pr-cleanup-and-gha-retention/INDEX.md` (writing-plans 산출) |
| 선행 ADR | ADR-0045 (GHA 복원), ADR-0024 (helm-publish), ADR-0033 (cosign+SLSA), ADR-0047 (community-operators sync) |
| 신규 ADR | ADR-0048 (GHA + 로컬 4계층 이중 운영 통합 결정) |

## 변경 이력

- **2026-05-21 v1.0**: 초기 작성 (GHA 제거 노선), Status=Proposed.
- **2026-05-21 v1.1**: 사용자 결정 4건 반영, "14개 workflow 모두 제거" 결정.
- **2026-05-21 v2.0**: **노선 전면 전환** — ADR-0045 등 이미 채택된 GHA 유지 결정과의 일관성 회복. 본 spec 은 *GHA 유지 + 통합 ADR 작성 + workflow 정합 + 사람 PR statusCheckRollup=[] 문제 해소* 로 재정의. 파일명도 `gha-removal` → `gha-retention` 으로 rename.

## 1. 배경 (Background)

### 1.0 노선 전환 사유 (v2.0)

v1.x 의 "GHA 14개 모두 제거" 노선은 RFC-0002 정신에는 부합하나, 본 저장소에 *이미 존재하는* 다음 4개 ADR 결정과 직접 충돌:

| ADR | 결정 | 충돌 지점 |
|---|---|---|
| ADR-0045 (`0045-restore-github-actions-for-oss-ci.md`) | OSS CI 를 위해 GHA 복원 (RFC-0002 명시적 일탈) | "14개 모두 제거" 와 정면 충돌 |
| ADR-0024 (helm-publish) | gh-pages 배포 채널을 GHA 자동화로 운영 | helm-publish.yml 제거 시 배포 채널 단절 |
| ADR-0033 (cosign + SLSA) | release.yml 의 SLSA3 / cosign 서명 파이프라인 | release.yml 제거 시 공급망 신뢰성 손상 |
| ADR-0047 (community-operators sync) | scorecard / release 와 연계된 자동 동기화 | 의존 워크플로 제거 시 동기화 차단 |

v1.x 는 "RFC-0002 정신 철저" 라는 가치만 보고 *이미 합의된 일탈* 을 인지 못 한 결정이었다. v2.0 은 ADR 들 간 일관성 회복이 우선:

- **GHA 14개 모두 유지** (제거 0, 추가 0, 정합 작업만).
- **로컬 4계층 유지** (lefthook + Makefile). GHA 와 *이중 운영* — 외부 신뢰 + 내부 fast feedback.
- **통합 ADR-0048 작성** — ADR-0045 + 0024 + 0033 + 0047 의 부분 결정을 묶어 *valkey-operator 의 CI/CD 통합 정책* 으로 격상.
- **사람 PR statusCheckRollup=[] 문제** — 본 spec 의 핵심 blocker. 별 Phase 로 추적·해소.

### 1.1 현재 상태 — `gh pr/issue` + `git branch -r` + `gh api ...protection` 측정 (2026-05-21)

| 항목 | 수치 | 비고 |
|---|---|---|
| open PR | **22** | 사람 6 + dependabot 16 (spec PR 포함 시 23) |
| open Issue | 1 | "Action Required: Fix Renovate Configuration" (#4) |
| stale (no PR) origin 브랜치 | 1 | `fix/multi-arch-cleanup-2026-05-21` |
| `.github/workflows/` 파일 | **14 (유지)** | 1,068 LOC, ADR-0045 근거 보존 |
| `main` `required_status_checks` | **11개** | 모두 GHA job 이름 (정합 OK) |
| 사람 PR `statusCheckRollup=[]` | **8건** | #138, #157, #158, #159, #161, #163~166 |
| dependabot PR `statusCheckRollup` | **13~15 checks** | 정상 (workflow 실행됨) |
| 기타 보호 설정 | enforce_admins=ON, linear_history=ON, force_push=OFF | rebase 머지 강제 |

### 1.2 PR 분류

**사람 PR 8개** (`eightynine01`):

| # | branch | 크기 | 영역 | 우선순위 | statusCheckRollup |
|---|---|---|---|---|---|
| 138 | feat/p-c-batch-3-webhook-otel-9x | +183/-4 | docs (OTel + 9.x flags) | 작음, 안전 | **[]** |
| 157 | feat/multi-arch-olm-prep | **+12,572/-9,178** | bundle/deploy/olm + ADR-0043 | 거대, 별도 검증 | **[]** |
| 158 | feat/valkey-op-ready-msg | +491/-19 | webhook + controller (TLS immutable + ready msg) | 중간, 핵심 | **[]** |
| 159 | feat/cdex-m1-pdb-delete-path-2026-05-21 | +41/-1 | controller (PDB delete) | 작음, 핵심 | **[]** |
| 161 | feat/keiailab-branding-2026-05-21 | +653/-20 | docs (BRANDING + family) | MERGEABLE 이나 BLOCKED | **[]** |
| 163~166 | 추가 작업 PR (이번 cycle 신규) | 변동 | spec / docs / cleanup | 본 cycle 작업 | **[]** |

**dependabot PR 16개** (분류):

| 종류 | 개수 | PR # | 본 spec 영향 |
|---|---|---|---|
| GHA actions | 8 | 156, 152, 151, 150, 149, 148, 147, 144 | **유지 + rebase merge** (workflow 보존) |
| Go modules | 6 | 146, 145, 143, 142, 141, 139 | 영향 없음 → 머지 |
| Docker base image | 2 | 154, 153 | 영향 없음 → 머지 |

### 1.3 BLOCKED 의 근본 원인 (v2.0 재해석)

v1.x 는 "workflow 자체가 게이트키퍼" 라고 판단했으나, dependabot PR 은 workflow 가 정상 실행되어 13~15 check 받음. 따라서 *워크플로 부재가 아니라 사람 PR 의 trigger 실패* 가 진짜 원인:

PR #161 (브랜딩, `MERGEABLE`) 이 `BLOCKED` 인 이유:
- `required_status_checks` 11개가 모두 GHA job 이름 → **정합 OK**
- *문제*: 사람 PR 에서 워크플로가 *실행조차 안 됨* (`statusCheckRollup=[]`)
- 후보 원인 (Phase 2.5 에서 검증):
    1. **Actions permissions setting**: Settings → Actions → General → "Approval for first-time contributors" 가 *Require approval for all outside collaborators* 또는 *Require for first PR* 설정
    2. **fork policy**: fork PR 의 workflow 자동 승인 안 됨 (`pull_request_target` 필요 또는 maintainer manual approve)
    3. **ci.yml branch filter**: `pull_request: branches: [main]` → 정상이라 가설 약함, 단 확인 필요
    4. **first-time-contributor approval 큐**: workflow 가 *Waiting* 상태로 머무름 → Actions tab 에서 maintainer 가 "Approve and run" 누르면 해제

→ Phase 2.5 에서 4 후보를 순서대로 검증·해소.

### 1.4 로컬 4계층 현황 (lefthook + Makefile) — *v2.0: 유지 + GHA 와 이중 운영*

`.lefthook.yml` 와 `Makefile` 이 GHA 의 거의 모든 check 를 이미 커버:

| GHA check | 로컬 대체 | 상태 | v2.0 운영 |
|---|---|---|---|
| golangci-lint | lefthook pre-commit + pre-push `full-lint` | ✅ | 이중 운영 |
| unit + envtest | lefthook pre-push `unit-test` | ✅ | 이중 운영 |
| build manager binary | Makefile `build` | ✅ | 이중 운영 |
| govulncheck | lefthook pre-push `govulncheck` | ✅ | 이중 운영 |
| trivy-fs / trivy-image | Makefile `audit` | ✅ | 이중 운영 |
| Review dependencies | govulncheck (로컬, CVE call-graph) | ✅ | GHA 보조 |
| Analyze (go) — CodeQL | gosec (간이) | ✅ | GHA 가 deep 분석 |
| Verify Signed-off-by | lefthook commit-msg `dco-signoff` | ✅ | 이중 운영 |
| kube-linter | lefthook pre-push (신규 추가) | ❌→✅ | Phase 1 보강 |
| go-licenses scan | Makefile `go-licenses` (신규 추가) | ❌→✅ | Phase 1 보강 |
| helm-lint / helm-template | lefthook pre-push | ✅ | 이중 운영 |
| markdown-link-check | lefthook pre-push (신규 추가) | ❌→✅ | Phase 1 보강 |
| scorecard / stale | 조직 메타데이터 / issue 관리 | — | GHA 전용 |
| release / helm-publish | 배포 자동화 | — | GHA 전용 (ADR-0024/0033) |
| dependency-review | govulncheck | ✅ | 이중 운영 |

→ **3종 보강 (kube-linter, go-licenses, markdown-link-check)** 은 v1.x 와 동일. 단 *GHA 대체용이 아니라 fast-feedback 용*.

## 2. 목표 (Goals) + 비목표 (Non-Goals)

### 2.1 Goals

| ID | 목표 | 검증 |
|---|---|---|
| G1 | open PR `22 → ≤2` | `gh pr list --state open` (남는 건 PR #157 또는 별 cycle 권장 항목) |
| G2 | stale branch `1 → 0` | `git branch -r` 결과 = `main`, `gh-pages` 만 |
| G3 | **`.github/workflows/` 14개 유지 + 통합 ADR-0048 작성** | `ls .github/workflows/` = 14 파일, `docs/kb/adr/0048-*.md` Accepted |
| G4 | **`required_status_checks` 11개 가 실제 workflow job 과 1:1 정합** (제거 0, 변경 0) | 각 check 이름 ↔ workflow job 매핑 표 검증 |
| G5 | **로컬 4계층 유지** + GHA 와 *이중 운영* (3종 보강 포함) | `lefthook run pre-push` 통과 |
| G6 | **사람 PR `statusCheckRollup` 정상화** (8건 → 모두 13~15 checks) | `gh pr view <N> --json statusCheckRollup` |
| G7 | e2e 통과 — kind cluster 에서 install → CR 생성 → reconcile → delete | `make integration-test` |
| G8 | Issue #4 (Renovate) 해소 | gh issue close + ADR |

### 2.2 Non-Goals

- ~~`.github/workflows/` 디렉토리 제거~~ (v1.x 목표 — **v2.0 에서 폐기**, ADR-0045 우선)
- ~~helm-publish / release 의 로컬 대체책 구축~~ (v1.x Phase 2.5 Goal G8 — **v2.0 에서 폐기**, ADR-0024/0033 우선)
- postgres / mongodb / commons / forgewise 의 CI/CD 변경 (별 spec)
- 다국어 (S4)
- operator-commons 공통화 (S5)
- PR #157 (거대) 의 *실 머지* — 별도 cycle 권장

## 3. 아키텍처 (단계 흐름)

```
[Phase 0] pre-flight
   ↓
[Phase 1] 로컬 4계층 보강 (kube-linter + go-licenses + md-link) — fast-feedback 용
   ↓
[Phase 2] Workflow 정합 + 통합 ADR-0048 작성
   ├─ 14 workflow 분류 (외부 신뢰 7 / 자동 배포 2 / 로컬 백업 4 / 운영 1)
   ├─ 각 workflow 의 유지 사유 명문화
   └─ ADR-0048 = ADR-0045+0024+0033+0047 통합 + 본 spec 결정 흡수
   ↓
[Phase 2.5] Workflow trigger 정상화 (사람 PR statusCheckRollup=[] 문제 해소)
   ├─ 원인 추적 (Actions permissions → fork policy → ci.yml filter → first-time approval)
   ├─ 필요 시 ci.yml 의 event 조건 수정 (pull_request_target 검토)
   └─ PR 재트리거 (close+reopen 또는 빈 commit)
   ↓
[Phase 3] 사람 PR 머지 (작은 것부터: 159 → 138 → 161 → 158 → [157 별도])
   ↓
[Phase 4] dependabot 처리 (GHA actions 8 + Go modules 6 + Docker 2 = 16 모두 머지)
   ↓
[Phase 5] 브랜치 cleanup (origin push --delete)
   ↓
[Phase 6] e2e + Issue #4 해소 + ADR-0048 Accepted 승격 + main tag
```

**원자성 (Atomic) 핵심**:
- **Phase 1 ↔ Phase 2** 는 묶지 않아도 OK. v1.x 와 달리 *제거* 가 없어 게이트 공백 위험 0.
- **Phase 2.5** 가 *Phase 3 의 선결 조건*. trigger 정상화 없이는 사람 PR 머지 불가 (BLOCKED 유지).

## 4. 단계별 상세 (Detailed Phases)

### Phase 0 — pre-flight

- `git fetch --all --prune` (모든 origin 상태 sync)
- 모든 사람 PR 의 `mergeable` 재확인
- valkey-operator owner / admin 권한 확인 (Settings → Actions, branch protection 모두 필요)
- 작업 spec branch PR 생성 → 본 design 공유 (이 작업)

### Phase 1 — 로컬 4계층 보강 (3종, fast-feedback 용)

| 항목 | 어디에 | 무엇 |
|---|---|---|
| kube-linter | lefthook pre-push + Makefile `kube-lint` target | `kube-linter lint deploy/ charts/valkey-operator/templates/` |
| go-licenses | Makefile `go-licenses` target + lefthook pre-push | `go-licenses check ./... --disallowed_types=forbidden,restricted` |
| markdown-link-check | lefthook pre-push (변경 *.md only) | `markdown-link-check --quiet` |

산출:
- `.lefthook.yml` 수정
- `Makefile` 에 `kube-lint`, `go-licenses`, `md-link-check` target 추가
- 본 작업은 **GHA 대체 아님** — 개발자가 push 전 GHA 통과 여부를 *예측* 하기 위한 fast feedback.

### Phase 2 — Workflow 정합 + 통합 ADR-0048 작성

#### 2.1 14 Workflow 분류 + 유지 사유

| 분류 | Workflow | LOC | 유지 사유 | 근거 ADR |
|---|---|---|---|---|
| **외부 신뢰 게이트 (7)** | ci.yml | 70 | lint+unit+build — 외부 contributor PR 의 1차 게이트 | ADR-0045 |
| | codeql.yml | 50 | CodeQL static analysis — OpenSSF Scorecard 점수 | ADR-0045, ADR-0033 |
| | dco.yml | 80 | Signed-off-by 검증 — 외부 PR 강제 | ADR-0045 |
| | dependency-review.yml | 45 | GitHub Dependency Review API — PR 단위 CVE | ADR-0045 |
| | go-licenses.yml | 90 | 라이선스 호환성 검사 | ADR-0045 |
| | kube-linter.yml | 80 | helm chart kube-linter | ADR-0045 |
| | security-scan.yml | 100 | trivy fs + image | ADR-0045 |
| **자동 배포 (2)** | helm-publish.yml | 65 | helm chart → gh-pages 자동 push (tag trigger) | ADR-0024 |
| | release.yml | 430 | GoReleaser + cosign + SLSA3 provenance + Hub sync | ADR-0033, ADR-0047 |
| **로컬 백업 (4)** | helm-install-test.yml | 135 | helm install in kind (smoke test) | ADR-0045 |
| | helm-lint.yml | 55 | helm lint matrix | ADR-0045 |
| | markdown-link-check.yml | 40 | 변경 *.md link 검증 | ADR-0045 |
| | scorecard.yml | 55 | OpenSSF Scorecard weekly | ADR-0045 |
| **운영 도구 (1)** | stale.yml | 60 | 30일 inactive issue/PR auto-close | ADR-0045 |

→ **합계: 14 workflow / 14 유지** (제거 0, 추가 0).

#### 2.2 통합 ADR-0048 작성

`docs/kb/adr/0048-gha-and-local-dual-track-ci.md`:

- **제목**: "GHA + 로컬 4계층 이중 운영 — valkey-operator CI/CD 통합 정책"
- **상태**: Accepted
- **선행 ADR 통합**: ADR-0045 (GHA 복원), ADR-0024 (helm-publish), ADR-0033 (cosign+SLSA), ADR-0047 (community-operators sync)
- **결정**:
    1. 14개 workflow 모두 유지. RFC-0002 영구 일탈 (이미 ADR-0045 로 정당화됨).
    2. 로컬 4계층 (lefthook + Makefile) 도 유지. 개발자 push 전 fast feedback 용.
    3. *동일 게이트의 이중 운영* 은 비용 (실행 시간 + 결과 불일치 가능성) 보다 *외부 contributor 호환성 + 개발자 속도* 이득이 큼.
    4. `required_status_checks` 11개는 GHA job 과 1:1 정합 유지. 변경 시 본 ADR 갱신.
- **결과**: RFC-0002 의 "단일 외부 SaaS SPOF" 리스크는 인지하되, 외부 신뢰가 우선. GHA billing 사고 재발 시 *임시* 로 로컬 4계층만으로 운영 가능 (degraded mode).

산출:
- `docs/kb/adr/0048-gha-and-local-dual-track-ci.md` 신규 (Accepted)
- `docs/kb/adr/INDEX.md` 갱신

#### 2.3 workflow 정합 검증 (변경 없음 확인)

```bash
# required_status_checks 11개가 모두 workflow job 에 존재하는지 검증
gh api repos/keiailab/valkey-operator/branches/main/protection | jq -r '.required_status_checks.contexts[]' | sort > /tmp/required.txt
grep -rh "name:" .github/workflows/*.yml | grep -E "^\s*name:" | sed 's/.*name:\s*//' | tr -d '"' | sort -u > /tmp/jobs.txt
comm -23 /tmp/required.txt /tmp/jobs.txt  # required 에 있으나 job 에 없는 것 = 0 이어야 함
```

### Phase 2.5 — Workflow trigger 정상화 (사람 PR statusCheckRollup=[] 해소)

**핵심 blocker**. 본 phase 통과 없이는 Phase 3 진입 불가.

#### 2.5.1 원인 추적 (순서대로)

| 단계 | 명령 | 가설 | 해소 |
|---|---|---|---|
| 1 | Settings → Actions → General 페이지 확인 (`gh api`) | "Require approval for first-time contributors" 가 ON | maintainer 가 PR 별 "Approve and run" 또는 setting 변경 |
| 2 | `gh api /repos/keiailab/valkey-operator/actions/permissions` | Actions 자체가 disabled / restricted | 권한 조정 (사용자 확인 필요) |
| 3 | `gh run list --branch <PR-branch>` | run 자체가 queued/waiting 상태 | maintainer approve |
| 4 | `gh pr view 161 --json headRepository` | fork PR 인가? (원작자가 maintainer 자신이라면 fork 아님) | `pull_request_target` 검토 |
| 5 | ci.yml 의 `on:` filter 검증 | `pull_request: branches: [main]` — base 가 main 인지 | base 확인 |

#### 2.5.2 발견된 원인에 따른 조치

- **case A (first-time-contributor approval)**: Settings → Actions → "Require approval for first-time contributors" 를 *Disabled* 또는 *Selected users* 로 변경. ADR-0048 의 운영 정책에 명문화.
- **case B (Actions disabled)**: `gh api -X PUT /repos/keiailab/valkey-operator/actions/permissions -f enabled=true -f allowed_actions=all`
- **case C (fork PR)**: 본 저장소 사람 PR 은 *직접 push* 인지 fork 인지 확인. 직접 push 면 fork 아님. 그럼에도 statusCheckRollup=[] 면 위 case A/B 우선.
- **case D (ci.yml 의 event 조건 문제)**: `on:` 에 `workflow_dispatch:` 추가 + maintainer manual trigger. 단 본 spec 의 default 는 *event 조건 미변경* (ADR-0045 의 의도 보존).

#### 2.5.3 PR 재트리거

원인 해소 후에도 기존 사람 PR 의 statusCheckRollup 이 비어 있을 수 있음 — 재트리거 필요:

```bash
# 옵션 a: close + reopen (가장 깔끔, status reset)
for n in 138 157 158 159 161 163 164 165 166 ; do
  gh pr close $n --repo keiailab/valkey-operator
  gh pr reopen $n --repo keiailab/valkey-operator
done

# 옵션 b: 빈 commit push (history 오염)
git commit --allow-empty -m "ci: trigger workflows" && git push
```

본 spec 의 default = **옵션 a** (close+reopen).

산출:
- Settings → Actions 변경 (또는 변경 불요 확인)
- 8건 사람 PR 모두 statusCheckRollup ≥ 11 checks 확보
- 원인·조치 내역을 `docs/plans/.../verification.md` 에 기록 + ADR-0048 부록에 운영 정책 명문화

### Phase 3 — 사람 PR 머지 (Phase 2.5 BLOCKED 해소 후)

| 순서 | PR | 전략 | 검증 |
|---|---|---|---|
| 1 | #159 (PDB fix +41/-1) | rebase main + merge | local `make verify` + GHA 11 checks PASS |
| 2 | #138 (OTel docs +183/-4) | rebase + merge | docs lint + GHA |
| 3 | #161 (브랜딩 +653/-20) | rebase + merge | `markdown-link-check` + GHA |
| 4 | #158 (webhook +491/-19) | rebase + merge | `make integration-test` + GHA |
| 5 | #163~166 | 본 cycle 내 작업 PR | 자체 검증 + GHA |
| — | #157 (거대 +12K/-9K) | **별도 cycle** — close 또는 squash 결정 후 후속 spec | — |

**PR #157 결정 (v1.1 사용자 결정 그대로)**: 별 cycle 로 분리. 본 spec Phase 3 종료 시:
- PR #157 자체 close (comment: "별 spec 으로 분해 후 단계 머지 예정")
- 브랜치 `feat/multi-arch-olm-prep` 은 임시 보존
- 후속 spec slug: `multi-arch-olm-prep-decomposition`

### Phase 4 — dependabot 처리 (16건 모두 머지)

**v1.x 와 차이**: GHA actions 8건도 *유지* 결정 → 모두 머지 (close 아님).

```bash
# GHA actions 8 (workflow 자체 의존성 업데이트 — actions/checkout 등)
for n in 156 152 151 150 149 148 147 144 ; do
  gh pr review $n --approve --repo keiailab/valkey-operator
  gh pr merge $n --rebase --auto --repo keiailab/valkey-operator
done

# Go modules 6
gh pr comment 146 145 143 142 141 139 --body "@dependabot recreate"  # grouped 권장
# 또는 개별 rebase + merge

# Docker base image 2 (#154 distroless, #153 golang)
gh pr merge 154 --rebase --repo keiailab/valkey-operator
gh pr merge 153 --rebase --repo keiailab/valkey-operator
```

### Phase 5 — 브랜치 cleanup

```bash
for br in feat/cdex-m1-pdb-delete-path-2026-05-21 \
          feat/p-c-batch-3-webhook-otel-9x \
          feat/keiailab-branding-2026-05-21 \
          feat/valkey-op-ready-msg ; do
  git push origin --delete "$br"
done

git push origin --delete fix/multi-arch-cleanup-2026-05-21

# dependabot/* 16개 자동 정리 확인. 안 됐으면 close + 브랜치 삭제.
```

산출: `git branch -r` 결과 = `main`, `gh-pages` (+ 임시 보존 `feat/multi-arch-olm-prep`).

### Phase 6 — e2e + Issue #4 + ADR + main tag

- e2e: `make integration-test` 또는 kind cluster 수동 검증
- Issue #4 (Renovate): renovate.json 점검 → 적정 설정 또는 close
- ADR 최종: `docs/kb/adr/0048-gha-and-local-dual-track-ci.md` Status `Accepted` (이미 Phase 2 에서 작성)
- main tag: `git tag v0.x.y` 후 push (옵션, GHA release.yml 자동 trigger)

## 5. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|---|---|---|
| Phase 2.5 의 원인 추적 실패 → 사람 PR 영원히 BLOCKED | spec 전체 차단 | case A~D 5단계 진단 + admin override (`gh api -X PUT branch protection` 으로 임시 contexts=[]) 후 머지 + 사후 복원 — 최후 수단 |
| GHA billing 사고 재발 (RFC-0002 트리거) | 전 PR merge 차단 | ADR-0048 의 "degraded mode" 운영 — 임시 contexts=[] + 로컬 4계층만으로 운영, 사고 해소 후 복원 |
| 동일 게이트 이중 운영의 결과 불일치 | 개발자 혼란 (로컬 PASS / GHA FAIL) | ADR-0048 에 "GHA 가 SSOT, 로컬은 prediction" 명문화 + 불일치 발견 시 로컬 게이트 강화 |
| 11개 required_check 가 실제 job 명과 미정합 | merge 영원히 BLOCKED | Phase 2.3 의 `comm -23` 검증으로 사전 차단 |
| PR #157 거대 변경의 merge conflict | rebase 시 충돌 폭발 | 별도 cycle 분리 (v1.1 결정 보존) |
| 로컬 4계층 의 *발견되지 않은* gap | 보안 회귀 | GHA 가 백업 게이트로 작동 — 본 spec 의 핵심 안전망 |
| valkey 만 GHA 유지 → 3개 operator 정합성 깨짐 | 가족성 손상 | postgres / mongodb 도 GHA 유지로 정렬 가능성 — 별 spec 의 정책 결정 |
| 사용자 검토 없는 owner 권한 작업 (Actions setting, branch protection) | 신뢰 손상 | Settings → Actions 변경은 사용자 명시 승인 후 |

## 6. 성공 기준 (Success Criteria)

본 spec 은 다음 모두 충족 시 *완료*:

```bash
# 1. open PR ≤ 2 (PR #157 잔존 허용)
test $(gh pr list --repo keiailab/valkey-operator --state open --json number | jq length) -le 2

# 2. stale origin 브랜치 0 (feat/multi-arch-olm-prep 임시 허용)
test $(git branch -r | grep -vE 'main|gh-pages|HEAD|feat/multi-arch-olm-prep' | wc -l) -eq 0

# 3. .github/workflows 디렉토리 = 14 파일 유지
test $(ls .github/workflows/*.yml | wc -l) -eq 14

# 4. required_status_checks 11개 모두 실제 job 과 정합
gh api repos/keiailab/valkey-operator/branches/main/protection | jq '.required_status_checks.contexts | length' | grep -q 11
# + Phase 2.3 의 comm -23 결과 = 빈 줄

# 5. 사람 PR statusCheckRollup 정상화 (예: PR #161 기준)
test $(gh pr view 161 --json statusCheckRollup --jq '.statusCheckRollup | length') -ge 11

# 6. lefthook pre-push 통과 (로컬 4계층, 3종 보강 포함)
lefthook run pre-push

# 7. e2e 통과
make integration-test

# 8. Issue #4 close 또는 ADR
gh issue view 4 --repo keiailab/valkey-operator --json state | jq -r .state  # CLOSED

# 9. ADR-0048 Accepted
grep -A1 'status:' docs/kb/adr/0048-gha-and-local-dual-track-ci.md | grep -q Accepted
```

증거 보존:
- 각 phase 종료 시 `verification.md` 에 명령어 + 출력 인용
- main commit log + ADR-0048 = 영구 흔적

## 7. 본 spec 의 out-of-scope (별 sub-project)

| 항목 | 분류 | 비고 |
|---|---|---|
| postgres / mongodb 의 GHA 정책 | S7 잔여 | 별도 spec — valkey 와 동일하게 GHA 유지로 정렬 가능 |
| 5개 저장소 다국어 (ja + zh 신규) | S4 | 별도 spec, S3 이후 |
| operator-commons 공통화 (helper 추출) | S5 | 별도 spec, 본 spec 완료 후 |
| forgewise 정합화 (README/LICENSE/SECURITY) | S6 | 별도 spec, 독립 진행 가능 |
| commons stale 브랜치 정리 | S2 | 별도 spec, 독립 진행 가능 |
| PR #157 (multi-arch + bundle 거대) | 후속 cycle | 본 spec Phase 3 결정 시점에 별 spec 으로 분리 |
| Wave 3 브랜딩의 valkey 적용 | S3 | PR #161 머지로 부분 처리, 나머지 S3 |
| RFC-0002 본문 갱신 (valkey-operator OSS 예외 명문화) | RFC | 별도 RFC 후속 — ADR-0048 이 부분 해소 |

## 8. 다음 단계

1. 사용자 본 v2.0 design 검토 + 승인 (PR comment 또는 직접 답변)
2. 승인 후 `superpowers:writing-plans` skill 호출 → `docs/plans/pr-cleanup-and-gha-retention/INDEX.md` + `research/*.md` 작성
3. plan 의 각 phase atomic 실행 (CLAUDE.md §8: task 1개 = 1 commit + 1 ship)
4. 각 phase 종료 시 verification.md 갱신
5. 완료 후 본 spec 의 Status `Accepted → Implemented` 갱신 + ADR-0048 Accepted

## 부록 A. 정의 (Definitions)

- **이중 운영** (v2.0): GHA + 로컬 4계층 동시 운영. GHA = SSOT (외부 신뢰 + merge 게이트), 로컬 = prediction (개발자 fast feedback).
- **로컬 4계층** (RFC-0002): pre-commit hook · pre-push hook · Makefile target · PR 리뷰어 증거 확인
- **atomic** (CLAUDE.md §8): task 1개 = 1 commit + 1 ship + 1 deploy
- **stale branch**: origin 에 존재하나 해당 PR 이 MERGED 또는 NO_PR 상태인 브랜치
- **degraded mode** (ADR-0048): GHA 장애 시 임시로 contexts=[] + 로컬 4계층만으로 운영하는 운영 모드
- **first-time-contributor approval**: GitHub Actions 설정의 외부 PR workflow 수동 승인 게이트
