# S1+ Design: valkey-operator PR Cleanup + GHA Removal

| 메타 | 값 |
|---|---|
| 날짜 | 2026-05-21 |
| 상태 | **Accepted** (2026-05-21 사용자 결정 반영 amendment) |
| 작성자 | keiailab — auto-cycle |
| 범위 | valkey-operator 만 (postgres / mongodb / commons / forgewise 는 별 spec) |
| 후속 | `docs/plans/pr-cleanup-and-gha-removal/INDEX.md` (writing-plans 산출) |

## 변경 이력

- **2026-05-21 v1.0**: 초기 작성, Status=Proposed.
- **2026-05-21 v1.1**: 사용자 결정 4건 반영, Status=Accepted.
  - RFC-0002 예외: **모두 제거** (helm-publish + release + scorecard + 나머지 11개 = 14개 전체)
  - PR #157 (multi-arch 거대): **별 cycle 로 분리** (Phase 3 종료 시 close + 후속 spec 분해)
  - 본 design 의 (1)(2) 결정 사항 해소

## 1. 배경 (Background)

### 1.1 현재 상태 — `gh pr/issue` + `git branch -r` 측정 (2026-05-21)

| 항목 | 수치 | 비고 |
|---|---|---|
| open PR | **21** | 사람 5 + dependabot 16 |
| open Issue | 1 | "Action Required: Fix Renovate Configuration" (#4) |
| stale (no PR) origin 브랜치 | 1 | `fix/multi-arch-cleanup-2026-05-21` |
| `.github/workflows/` 파일 | 14 (1,068 LOC) | RFC-0002 위반 상태 |
| `main` `required_status_checks` | **11개** | 모두 GHA job 이름 |
| 기타 보호 설정 | enforce_admins=ON, linear_history=ON, force_push=OFF | rebase 머지 강제 |

### 1.2 PR 분류

**사람 PR 5개** (`eightynine01` 작성):

| # | branch | 크기 | 영역 | 우선순위 |
|---|---|---|---|---|
| 138 | feat/p-c-batch-3-webhook-otel-9x | +183/-4 | docs (OTel + 9.x flags) | 작음, 안전 |
| 157 | feat/multi-arch-olm-prep | **+12,572/-9,178** | bundle/deploy/olm + ADR-0043 | 거대, 별도 검증 |
| 158 | feat/valkey-op-ready-msg | +491/-19 | webhook + controller (TLS immutable + ready msg) | 중간, 핵심 |
| 159 | feat/cdex-m1-pdb-delete-path-2026-05-21 | +41/-1 | controller (PDB delete) | 작음, 핵심 |
| 161 | feat/keiailab-branding-2026-05-21 | +653/-20 | docs (BRANDING + family) | MERGEABLE 이나 BLOCKED |

**dependabot PR 16개** (분류):

| 종류 | 개수 | PR # | workflow 제거 영향 |
|---|---|---|---|
| GHA actions | 8 | 156, 152, 151, 150, 149, 148, 147, 144 | **자동 close** (의존 워크플로 사라짐) |
| Go modules | 6 | 146, 145, 143, 142, 141, 139 | 영향 없음 → 수동 처리 |
| Docker base image | 2 | 154, 153 | 영향 없음 → 수동 처리 |

### 1.3 BLOCKED 의 근본 원인

PR #161 (브랜딩, `MERGEABLE`) 이 `BLOCKED` 인 이유:
- `required_status_checks` 11개가 모두 GHA job 이름 (`golangci-lint`, `unit + envtest`, `build manager binary`, `govulncheck`, `trivy-fs`, `trivy-image`, `Review dependencies`, `Analyze (go)`, `Verify Signed-off-by`, `kube-linter (helm chart render)`, `go-licenses scan`)
- *워크플로 자체* 가 머지를 막는 게이트키퍼. GHA 가 실행되지 않으면 영원히 BLOCKED.
- 다른 사람 PR 4개의 `mergeable=UNKNOWN`, `statusCheckRollup=[]` 도 동일 원인.

### 1.4 로컬 4계층 현황 (lefthook + Makefile)

`.lefthook.yml` 와 `Makefile` 이 GHA 의 거의 모든 check 를 *이미* 커버:

| GHA check | 로컬 대체 | 상태 |
|---|---|---|
| golangci-lint | lefthook pre-commit `golangci-lint` + pre-push `full-lint` | ✅ |
| unit + envtest | lefthook pre-push `unit-test` (`go test -count=1 ./...`) | ✅ |
| build manager binary | Makefile `build` | ✅ (Makefile only, hook 없음) |
| govulncheck | lefthook pre-push `govulncheck` | ✅ |
| trivy-fs / trivy-image | Makefile `audit` (govulncheck + gosec + trivy fs) | ✅ |
| Review dependencies | 로컬: `go mod tidy` 강제 (lefthook `go-mod-tidy`) + govulncheck | ✅ (CVE call-graph 기준) |
| Analyze (go) — CodeQL | Makefile `audit` 의 gosec | ✅ (간이) |
| Verify Signed-off-by | lefthook commit-msg `dco-signoff` | ✅ |
| kube-linter | **부재** → 추가 필요 | ❌ |
| go-licenses scan | Makefile `go-licenses` target 없음 (workflow 만) | ❌ |
| helm-lint / helm-template | lefthook pre-push `helm-lint` + `helm-template` | ✅ |
| markdown-link-check | **부재** | ❌ |
| scorecard | (조직 메타데이터, 머지 게이트 아님) | — |
| stale | (issue 관리, 머지 게이트 아님) | — |
| release / helm-publish | (배포, 머지 게이트 아님) | — |
| dependency-review | (수동 PR 게이트, 로컬 govulncheck 가 더 강함) | ✅ (대체) |

→ **부족: kube-linter, go-licenses, markdown-link-check 3종**. 보강 필요.

## 2. 목표 (Goals) + 비목표 (Non-Goals)

### 2.1 Goals

| ID | 목표 | 검증 |
|---|---|---|
| G1 | open PR `21 → ≤2` | `gh pr list --state open` (남는 건 PR #157 또는 별 cycle 권장 항목) |
| G2 | stale branch `6 → 0` | `git branch -r` (origin/feat/* + origin/fix/* 0개) |
| G3 | `.github/workflows/` 디렉토리 **제거** + 로컬 4계층 보강 | `ls .github/workflows/` → No such file |
| G4 | `main` branch protection `required_checks 11 → 0` | `gh api ...protection` |
| G5 | 로컬 4계층 = GHA 가 했던 모든 머지 게이트 1:1 대체 (3종 보강) | `lefthook run pre-push` 통과 |
| G6 | e2e 통과 — kind cluster 에서 install → CR 생성 → reconcile → delete | `make integration-test` 또는 별도 e2e |
| G7 | Issue #4 (Renovate) 해소 | gh issue close + ADR |
| G8 | helm-publish + release 대체책 동시 구축 (로컬 스크립트 + Makefile target) | `make release` + `scripts/helm-publish.sh` 통과 |

### 2.2 Non-Goals

- postgres / mongodb / commons / forgewise 의 GHA 제거 (별 spec)
- 다국어 (S4) — 본 spec 은 영어/한국어 혼합만 보장
- operator-commons 공통화 (S5)
- PR #157 (거대) 의 *실 머지* — 별도 cycle 권장 (Goal 은 close 또는 squash 결정만)

## 3. 아키텍처 (단계 흐름)

```
[Phase 0] pre-flight
   ↓
[Phase 1] 로컬 4계층 보강 (kube-linter + go-licenses + md-link)
   ↓
[Phase 2] workflow 제거 + branch protection 갱신 (atomic)
   ↓
   ├─ dependabot/github_actions/* 8개 PR 자동 close (관찰)
   └─ 사람 PR + go_modules dependabot 의 BLOCKED 해제 확인
   ↓
[Phase 3] 사람 PR 머지 (작은 것부터: 159 → 138 → 161 → 158 → [157 별도])
   ↓
[Phase 4] dependabot 처리 (Go modules 6 + Docker 2)
   ↓
[Phase 5] 브랜치 cleanup (origin push --delete)
   ↓
[Phase 6] e2e + Issue #4 해소 + ADR + main tag
```

**원자성 (Atomic) 핵심**:
- **Phase 1 ↔ Phase 2** 는 묶어서 1 commit + 1 PR + 1 머지로. 로컬 게이트가 보강되지 않은 채로 워크플로를 제거하면 게이트 공백 발생.
- **Phase 2** 자체도 `workflow 제거 commit` 과 `branch protection 갱신` 이 같은 시점이어야 dependabot/github_actions PR 들이 깔끔히 close.

## 4. 단계별 상세 (Detailed Phases)

### Phase 0 — pre-flight

- `git fetch --all --prune` (모든 origin 상태 sync)
- 모든 사람 PR 의 `mergeable` 재확인
- valkey-operator owner / admin 권한 확인 (branch protection 갱신 권한 필요)
- 작업 spec branch (현 worktree) PR 생성 → 본 design 공유

### Phase 1 — 로컬 4계층 보강 (3종)

| 항목 | 어디에 | 무엇 |
|---|---|---|
| kube-linter | lefthook pre-push + Makefile `kube-lint` target | `kube-linter lint deploy/ charts/valkey-operator/templates/` |
| go-licenses | Makefile `go-licenses` target + lefthook pre-push | `go-licenses check ./... --disallowed_types=forbidden,restricted` |
| markdown-link-check | lefthook pre-push (변경 *.md only) | `markdown-link-check --quiet` |

산출:
- `.lefthook.yml` 수정
- `Makefile` 에 `kube-lint`, `go-licenses`, `md-link-check` target 추가
- `docs/kb/adr/0048-gha-to-local-4-layer.md` 신규 (RFC-0002 적용 결정 기록)

### Phase 2 — workflow 제거 + protection 갱신

```bash
# 단일 commit (atomic)
git rm -r .github/workflows/
git commit -m "feat(ci): RFC-0002 — remove .github/workflows + 로컬 4계층 이관"

# protection 갱신 (gh api PATCH)
gh api -X PUT repos/keiailab/valkey-operator/branches/main/protection \
  --input <(현재 protection json 에서 required_status_checks.contexts = [] 적용)
```

산출:
- `.github/workflows/` 디렉토리 git rm (14 파일)
- branch protection 갱신: `required_status_checks.contexts = []`
- `docs/kb/adr/0048-gha-to-local-4-layer.md` 본문에 *예외 결정* 기록 (helm-publish, release, scorecard 등 각각의 운명)

**RFC-0002 결정 (v1.1 사용자 승인)**: **14개 workflow 모두 제거**. 예외 ①(Pages)·③(Release) 도 적용 안 함 — 대신 *로컬 대체책 동시 구축* (Goal G8). RFC-0002 정신 철저 + 단일 외부 SaaS 의존 제거.

**제거 대상 14개**: ci, codeql, dco, dependency-review, go-licenses, helm-install-test, helm-lint, **helm-publish**, kube-linter, markdown-link-check, **release**, scorecard, security-scan, stale.

**관찰 단계**: PR list 재조회 → dependabot/github_actions PR 8개 자동 close 또는 conflict 상태 → 일괄 close.

### Phase 2.5 — helm-publish + release 대체책 (Goal G8)

`helm-publish.yml` + `release.yml` 의 *기능* 을 로컬로 이관:

| GHA 기능 | 로컬 대체 | 산출 |
|---|---|---|
| helm chart → gh-pages 자동 push | `scripts/helm-publish.sh` — helm package + gh-pages branch clone + index.yaml 갱신 + push | Makefile `make helm-publish` |
| GoReleaser (release.yml 의 가장 큰 부분) | `scripts/release.sh` — git tag + goreleaser local + gh release create | Makefile `make release` |
| GH Release 본문 자동 생성 (git-cliff) | `cliff.toml` (이미 있음) + `scripts/release.sh` 안에서 cliff → release body | Makefile `make release` |
| 멀티아키 docker build (release.yml 의 일부) | `docker buildx` (default builder, CLAUDE.md §2 단일 amd64 강제, 멀티아키 금지) | Makefile `docker-build` |

대체책은 *수동 trigger* (개발자가 release 시 `make release` 실행). 자동화는 *별 cycle* 에서 cron / local pre-tag hook 으로 후속 검토 가능.

### Phase 3 — 사람 PR 머지

| 순서 | PR | 전략 | 검증 |
|---|---|---|---|
| 1 | #159 (PDB fix +41/-1) | rebase main + merge | local `make verify` 통과 |
| 2 | #138 (OTel docs +183/-4) | rebase + merge | docs lint |
| 3 | #161 (브랜딩 +653/-20) | rebase + merge | `markdown-link-check` 통과 |
| 4 | #158 (webhook +491/-19) | rebase + merge | `make integration-test` |
| — | #157 (거대 +12K/-9K) | **별도 cycle** — close 또는 squash 결정 후 후속 spec | — |

**PR #157 결정 (v1.1 사용자 승인)**: **별 cycle 로 분리** (옵션 b). Phase 3 종료 시:
- PR #157 자체 close (comment: "별 spec 으로 분해 후 단계 머지 예정 — 본 PR 닫고 후속 spec 작성")
- 브랜치 `feat/multi-arch-olm-prep` 은 *임시 보존* (`git push origin --delete` 안 함) — 후속 spec 의 작업 기반
- 후속 spec slug: `multi-arch-olm-prep-decomposition` — bundle / deploy / docs / Makefile 영역별 분해 후 각각 작은 PR

### Phase 4 — dependabot 처리

**Go modules 6개**: 의존성 차근차근. 그룹 머지 권장 시 단일 PR.
```bash
# dependabot grouped 가 이미 일부 활성. 그룹 PR 1개로 통합:
gh pr comment 146 145 143 142 141 139 --body "@dependabot recreate"
# 또는 개별 rebase + merge.
```

**Docker base image 2개** (#154 distroless, #153 golang):
- base image 변경 → docker build 통과 확인 후 머지.

### Phase 5 — 브랜치 cleanup

```bash
# 머지된 사람 PR 브랜치
for br in feat/cdex-m1-pdb-delete-path-2026-05-21 \
          feat/p-c-batch-3-webhook-otel-9x \
          feat/keiailab-branding-2026-05-21 \
          feat/valkey-op-ready-msg ; do
  git push origin --delete "$br"
done

# stale (PR 없는) 브랜치
git push origin --delete fix/multi-arch-cleanup-2026-05-21

# 모든 dependabot/* 자동 close 됐는지 확인 → 안 됐으면 close + 브랜치 삭제
```

산출: `git branch -r` 결과 = `main`, `gh-pages` 만.

### Phase 6 — e2e + Issue #4 + ADR + main tag

- e2e: `make integration-test` 또는 `kind create cluster && helm install valkey-operator charts/valkey-operator && kubectl apply -f config/samples/*` 후 CR reconcile 검증
- Issue #4 (Renovate): renovate.json 점검 → 적정 설정 또는 close
- ADR 최종: `docs/kb/adr/0048-gha-to-local-4-layer.md` Status `Accepted` 로 승격
- main tag: `git tag v0.x.y` 후 push (옵션)

## 5. 리스크 & 완화

| 리스크 | 영향 | 완화 |
|---|---|---|
| workflow 제거 후 dependabot PR 이 자동 close 안 됨 | 노이즈 잔존 | Phase 2 직후 수동 close: `gh pr close 156 152 ...` |
| branch protection 갱신 권한 없음 | Phase 2 차단 | owner / admin 권한 확인 (`gh api user`) + 미보유 시 사용자 권한 위임 요청 |
| helm-publish / release workflow 제거 → 배포 영향 | helm chart gh-pages 배포 정지 | RFC-0002 예외 ① (Pages) ③ (Release) 검토 — 본 spec 의 default = 모두 제거. 사용자 결정 필요 |
| PR #157 거대 변경의 merge conflict | rebase 시 충돌 폭발 | Phase 3 마지막 + 별도 cycle 분리 |
| 로컬 4계층의 *발견되지 않은* gap (e.g. CodeQL 의 deep static analysis) | 보안 회귀 | Phase 1 의 `audit` (gosec + govulncheck) 가 1차 방어, 사후 *연 1회* security review 권장 |
| valkey 만 GHA 제거 → 3개 operator 정합성 깨짐 | 가족성 손상 | S7 (postgres + mongodb) 후속 cycle 즉시 실행 |
| 사용자 검토 없는 owner 권한 작업 (force push, protection) | 신뢰 손상 | force push **금지** (linear_history=ON). protection 갱신은 사용자 명시 승인 후 |

## 6. 성공 기준 (Success Criteria)

본 spec 은 다음 모두 충족 시 *완료*:

```bash
# 1. open PR ≤ 2 (PR #157 잔존 허용)
test $(gh pr list --repo keiailab/valkey-operator --state open --json number | jq length) -le 2

# 2. stale origin 브랜치 0
test $(git branch -r | grep -v -E 'main|gh-pages|HEAD' | wc -l) -eq 0

# 3. .github/workflows 디렉토리 없음
test ! -d .github/workflows

# 4. required_status_checks 0
test $(gh api repos/keiailab/valkey-operator/branches/main/protection | jq '.required_status_checks.contexts | length') -eq 0

# 5. lefthook pre-push 통과
lefthook run pre-push

# 6. e2e 통과 — kind + helm install + CR reconcile
make integration-test  # 또는 별도 e2e script

# 7. Issue #4 close 또는 ADR
gh issue view 4 --repo keiailab/valkey-operator --json state | jq -r .state  # CLOSED

# 8. ADR 0048 Accepted
grep -A1 'Status:' docs/kb/adr/0048-gha-to-local-4-layer.md | grep -q Accepted
```

증거 보존:
- 각 phase 종료 시 `verification.md` 에 명령어 + 출력 인용 (CLAUDE.md §2 "통과 로그·핵심 출력을 인용해 입증")
- main commit log + ADR 0048 = 영구 흔적

## 7. 본 spec 의 out-of-scope (별 sub-project)

| 항목 | 분류 | 비고 |
|---|---|---|
| postgres / mongodb GHA 제거 | S7 잔여 | 별도 spec, 병렬 가능 (required_check 가 이미 0) |
| 5개 저장소 다국어 (ja + zh 신규) | S4 | 별도 spec, S3 이후 |
| operator-commons 공통화 (helper 추출) | S5 | 별도 spec, 본 spec 완료 후 |
| forgewise 정합화 (README/LICENSE/SECURITY) | S6 | 별도 spec, 독립 진행 가능 |
| commons stale 브랜치 정리 | S2 | 별도 spec, 독립 진행 가능 |
| PR #157 (multi-arch + bundle 거대) | 후속 cycle | 본 spec 의 Phase 3 결정 시점에 *별 spec 으로 분리* |
| Wave 3 브랜딩의 valkey 적용 | S3 | PR #161 머지로 부분 처리, 나머지 S3 |

## 8. 다음 단계

1. 사용자 본 design 검토 + 승인 (PR comment 또는 직접 답변)
2. 승인 후 `superpowers:writing-plans` skill 호출 → `docs/plans/pr-cleanup-and-gha-removal/INDEX.md` + `research/*.md` 작성
3. plan 의 각 phase atomic 실행 (CLAUDE.md §8: task 1개 = 1 commit + 1 ship)
4. 각 phase 종료 시 verification.md 갱신
5. 완료 후 본 spec 의 Status `Proposed → Implemented` 갱신 + ADR 0048 Accepted

## 부록 A. 정의 (Definitions)

- **로컬 4계층** (RFC-0002): pre-commit hook · pre-push hook · Makefile target · PR 리뷰어 증거 확인
- **atomic** (CLAUDE.md §8): task 1개 = 1 commit + 1 ship + 1 deploy
- **stale branch**: origin 에 존재하나 해당 PR 이 MERGED 또는 NO_PR 상태인 브랜치
- **dependabot grouped PR**: dependabot 의 `groups:` 설정으로 묶은 단일 PR
