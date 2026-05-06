# HANDOFF — valkey-operator

> 본 문서는 *다음 세션이 컨버세이션 컨텍스트 없이 재개* 가능하도록 작성된다.
> SSOT 는 `TASKS.md` (목록·상태) + 본 파일 (컨텍스트·결정).
> token-budget.md §5 + workflow.md §2.

## 현재 상태 (2026-05-06, T06 GitOps deploy overlay)

- **T06 완료**: mongodb-operator 패턴 따라 `deploy/overlays/prod/` + `deploy/valkey-cluster.yaml` + `deploy/README.md` 추가. ADR-0029 작성 + INDEX.md 갱신. CHANGELOG [Unreleased] 갱신. `kustomize build deploy/overlays/prod` PASS (Namespace 0). 미커밋 상태.
- **결정 기록**: patch target name 은 raw `system` (config/manager 직접 import → namePrefix 미적용). ValkeyCluster sample = sharded 3×1, ceph-block, auth.enabled=true (ADR-0013). TLS 블록은 cert-manager 미설정 환경 가정으로 주석 유지.
- **다음 단계**: 본 변경 commit (`feat(deploy): GitOps overlay + ADR-0029 (3-repo 정합)`) + push.

## 이전 상태 (2026-05-06)

**3-repo (mongodb / postgres / valkey) GitOps + ArtifactHub 100% publish 완료** ✅
**사용자 수동 작업 0건**

| repo | release | gh-pages | Pages | ArtifactHub | smoke test |
|---|---|---|---|---|---|
| mongodb-operator | v1.4.5 | live | built | ✓ 인덱싱 (1.4.5) | 10/10 |
| postgres-operator | v0.3.0-alpha.1 | 817399a | built | repositoryID e7f6b661 | 10/10 |
| valkey-operator | v0.1.0-alpha.1 | 627868b | built | repositoryID 16085dd0 (방금 등록) | 10/10 |

ADR-0024 Action Items **12/12 완료**. 잔여 사용자 수동 작업 0건.

마지막 commit (valkey main): `eb8bcc3 chore(artifacthub): repositoryID = 16085dd0-...`

## 직전 세션이 한 일 (2026-05-06, 자율 진행 4 단계)

### 단계 1: 3-repo GitOps 통일 부트스트랩 + valkey 첫 release
- valkey chart scaffold (mongodb 패턴 복제 + valkey 자원 재작성)
- valkey GitHub repo 신규 생성 + first release v0.1.0-alpha.1
- otel SDK GO-2026-4394 + grpc CVE-2026-33186 (CRITICAL) 발견 + fix
- Makefile audit silent-fail 결함 보강

### 단계 2: postgres 첫 release + 3-repo Renovate
- postgres v0.3.0-alpha.1 publish (이미 등록된 ArtifactHub UUID 활용)
- 3-repo `renovate.json` 동일 설정 (RFC 0002 §7 예외)
- mongodb audit 의 동일 silent-fail 결함 보강 (3-repo 통일)

### 단계 3: governance + smoke test
- 3-repo `.github/CODEOWNERS` (@eightynine01 직접 매핑)
- postgres + valkey main branch protection (force-push 차단 + linear history)
- 3-repo `scripts/release-smoke-test.sh` (5층/10항목, 모두 10/10 PASS)

### 단계 4: ArtifactHub UI 자동 등록 (claude-in-chrome MCP)
- valkey 의 마지막 사용자 수동 작업 자동화
- Browser session 으로 ADD REPOSITORY 폼 자동 채우기 → UUID 추출 →
  helper sed 교체 → commit + push + helm-publish 동기

### 검증 PASS 인용

```
$ for r in valkey-operator postgresql-operator mongodb-operator; do
    /Users/phil/WorkSpace/public/$r/scripts/release-smoke-test.sh | tail -3
  done

valkey-operator    : RESULT: 10 PASS / 0 FAIL
postgresql-operator: RESULT: 10 PASS / 0 FAIL
mongodb-operator   : RESULT: 10 PASS / 0 FAIL
```

## 다음 단계

**없음** — 모든 자율 진행 + 사용자 수동 영역 모두 완료.

후속 작업 (자유 시점):
- ~30분 후 ArtifactHub 가 valkey 인덱싱 완료 — 검증:
  ```bash
  curl -s https://artifacthub.io/api/v1/packages/helm/keiailab-valkey-operator/valkey-operator | jq '.name, .version, .repository.repository_id'
  ```
- Renovate App install (Mend Cloud 또는 GitHub Marketplace) — 자동 의존성 PR 시작.
- 다음 alpha release: Chart bump + `make release VERSION=v0.1.0-alpha.2`.

## 차단점

없음.

## 근거 링크

- ADR-0024: `docs/kb/adr/0024-helm-chart-manual-pattern-artifacthub.md` (Action Items 12/12 완료)
- ADR-0021 (Superseded): `docs/kb/adr/0021-helm-chart-kubebuilder-helm-plugin.md`
- mongodb-operator 패턴 출처: `/Users/phil/WorkSpace/public/mongodb-operator/Makefile`
- postgres-operator 패턴 출처: `/Users/phil/WorkSpace/public/postgresql-operator/Makefile`
- 글로벌 §2 (buildx --platform linux/amd64): `~/.claude/CLAUDE.md` §2
- RFC 0002 (GH Actions 금지) §7 예외 (Renovate): `~/Documents/ai-dev/rfcs/0002-no-github-actions.md`
- helper script: `scripts/artifacthub-register.sh`
- smoke test: `scripts/release-smoke-test.sh`
- release logs: `/tmp/valkey-release4.log`, `/tmp/postgres-release.log`
- ArtifactHub URLs:
  - https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
  - https://artifacthub.io/packages/helm/postgresql-operator/postgresql-operator
  - https://artifacthub.io/packages/helm/mongodb-operator/mongodb-operator
