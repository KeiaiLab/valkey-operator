# ADR-0043: GitLab → GitHub Mirror Auto-Sync (RFC-0002 §2.3 예외 4번째 카테고리)

- Date: 2026-05-20
- Status: Proposed
- Authors: @phil

## Context

RFC-0040 (2026-05-16) 채택 후 keiailab self-host GitLab 가 *모든 source repo 의 SSOT 진본*. valkey-operator 의 GitHub `origin` (`keiailab/valkey-operator`) 은 OSS 마케팅 + community contribution path. 2026-05-20 audit 시점 `origin/main` 이 `gitlab-upstream/main` 보다 **68 commits behind, 2일 stale** — manual mirror sync 의 *주기적 누락* 의 라이브 evidence.

RFC-0002 (2026-04-28) 가 GitHub Actions 영구 금지 — 예외 3 카테고리 (Pages / Dependabot / release-tag-only). 본 mirror sync 는 *4번째 예외 카테고리* 신설 후보 — 단 GitLab CI 가 push 하는 *서버사이드 패턴* 이므로 *GitHub Actions 자체 사용 X* (GitHub workflow 부재).

본 ADR 의 *조직 정책 변경* = RFC-0002 §2.3 예외 카테고리 확장 + RFC-0040 §10 진본 정합 자동화. 후속 cross-project 적용 가능성 (12+ repo).

Codex stage 3 adversarial review (RFC-0045 §2.5, 2026-05-20) 의 *CDEX-C1 critical* — `git push --mirror` 사용 시 dependabot 16 + gh-pages + feature branch *force delete + main force update + 로컬 remote-tracking ref 까지 GitHub 에 새 ref* 로 push. mirror sync 가 아닌 *GitHub repo rewrite 위험*. 본 ADR 의 *explicit refspec 정책 codify 의무*.

## Decision

GitLab CI `.gitlab-ci.yml` 의 별 stage `mirror_to_github` 가 *main branch 머지 후* GitHub `origin` 으로 push:

- **방식**: `git push github refs/heads/main:refs/heads/main` + `git push github --tags` — **explicit refspec only** (Codex CDEX-C1 처리)
- **인증**: GitHub Deploy Key (PAT 대신, single-repo write 권한)
- **trigger**: `$CI_COMMIT_BRANCH == "main"` + `when: on_success`
- **scope**: main branch + tag 만. *gh-pages / dependabot / feature branch 모두 GitHub 측 무손실*.

`--mirror` flag **영구 금지** — Codex CDEX-C1 의 진본 위험 영구 차단.

## Consequences

**긍정**:
- drift 0 자동화 — `local_clone_drift_count` rule (§3.1.7) 적색 회피
- OSS 마케팅 가치 유지 (GitHub star + clone metric)
- RFC-0040 진본 정합 자동화 — manual sync 책임 제거
- 사용자 결정 #7 (Contabo VPS 강제) sister 패턴 — *진본 = self-host, mirror = external*
- Codex CDEX-C1 의 *force-delete 위험* 영구 차단

**부정**:
- GitLab CI infrastructure 의존성 (runner 가용 시점에만 sync)
- GitHub Deploy Key rotation 책임 (6개월 + audit log)
- compromise 시 mirror force push risk — git history rewrite 의무
- 4번째 RFC-0002 §2.3 예외 카테고리 = 추가 ADR governance overhead

**트레이드오프**:
- GitHub repo 의 *read-only mirror 화* — GitHub side 의 PR/issue 가 *진본 아님*. community contributor 는 GitLab 진본으로 redirect 필요 (README + CONTRIBUTING 안내).
- *dependabot 16 orphan branch* 는 GitHub side 에 *그대로 잔존* (mirror 가 main + tag 만 push). 별 PR 로 cleanup 의무 (T0 plan T1.4 후속).

### SR-C4 Secret 관리 4 항목 (MUST)

1. **(a) GitLab CI variable masked + protected branch only** — `GITHUB_DEPLOY_KEY` 가 GitLab project Settings → CI/CD → Variables 에 `masked: true` + `protected: true` 등록. `main` branch pipeline 만 접근 가능.

2. **(b) GitHub Deploy Key scope = single repo write** — PAT 대신 *Deploy Key* 사용 — GitHub Settings → repository → Deploy keys 에 *single repo* 한정 write 권한. compromise 시 *전체 GitHub 계정 영향 X*.

3. **(c) rotation 6개월 + audit log** — Deploy Key 6개월 마다 rotation 의무. GitHub audit log 의 `repo.update_deploy_key` 모니터링.

4. **(d) compromise 시 immediate revoke + git history rewrite** — Deploy Key 유출 의심 시 (a) GitHub side immediate revoke (b) git history rewrite if secret committed (filter-branch / BFG repo-cleaner) (c) force push main + tag mirror (recovery).

## Alternatives Considered

1. **방식 B**: 로컬 lefthook post-merge hook — 거절 (사용자 워크스테이션 의존, 비결정적, 다중 사용자 환경 동기화 곤란)
2. **방식 C**: 외부 cron + sync 스크립트 — 거절 (별 인프라 의무, GitLab CI 와 중복)
3. **No mirror — GitHub 폐기**: GitLab SSOT only — 거절 (OSS 마케팅 가치 + 사용자 결정 D1 정합 안 함)
4. **GH Action 사용 (RFC-0002 정신 위배)**: GitHub side workflow 가 GitLab pull — 거절 (RFC-0002 §1 핵심 정책)
5. **`git push --mirror`** (Codex CDEX-C1 reject reason): main + tag 외 모든 ref force push — 거절 (dependabot 16 + gh-pages + feature branch *force delete* 위험)

## Refs

- RFC-0002 (GitHub Actions 영구 금지, §2.3 예외 3 카테고리 — 본 ADR = 4번째 예외 신설)
- RFC-0040 (GitLab MCP First, §10 SSOT 진본 정합)
- RFC-0043 (GitLab CI L5 라이브 게이트, mirror stage 도입 path)
- RFC-0045 (Plan Adversarial Review, §2.5 stage 3 Codex CDEX-C1 발견)
- T0 fix plan: `~/.claude/plans/valkey-operator-t0-fix-chart-ha-mirror-gitlab-ci.md`
- 라이브 evidence (2026-05-20): `git log gitlab-upstream/main..origin/main | wc -l` = 68, 2일 stale
- 관련 memory: `gitlab-mailstory-auth-gateway.md` + `gitlab-sudo-root-header.md` + `multi-llm-single-source.md`
- 트리거: 2026-05-20 Codex stage 3 adversarial review CDEX-C1 + 사용자 결정 D1
