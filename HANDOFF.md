# HANDOFF — valkey-operator

> 본 문서는 *다음 세션이 컨버세이션 컨텍스트 없이 재개* 가능하도록 작성된다.
> SSOT 는 `TASKS.md` (목록·상태) + 본 파일 (컨텍스트·결정).
> token-budget.md §5 + workflow.md §2.

## 현재 상태 (2026-05-06)

- 마지막 commit: `8a54d3d feat(helm): GitOps publish 파이프라인 — chart scaffold + ArtifactHub 통일 (ADR-0024)`
- branch: `main` → `origin/main` (GitHub repo 신규 생성됨: https://github.com/keiailab/valkey-operator)
- 미커밋 변경: 0건 (`.claude/ralph-loop.local.md` 외 — gitignore 등재)
- ADR: 0024 (Accepted, supersedes 0021)
- 검증 PASS: `helm lint`, `helm template` (default + all-features-on),
  `go vet`, `make manifests generate`, lefthook pre-commit (helm-lint),
  pre-push (gitleaks/helm-lint/helm-template/unit-test/full-lint)

## 직전 세션이 한 일

3-repo (mongodb-operator / postgres-operator / valkey-operator) GitOps 파이프라인을
*동일 패턴*으로 통일. valkey 에 mongodb-operator 와 동일한 *수기 chart +
ArtifactHub publish* 패턴을 부트스트랩.

산출물:
- `charts/valkey-operator/` — Helm chart (Chart.yaml + values.yaml +
  values.schema.json + README.md + LICENSE + crds/ + templates/)
- `charts/artifacthub-repo.yml` — ArtifactHub 메타 (PGP signing key 재사용,
  repositoryID 는 placeholder)
- `Makefile` — release / helm-publish / gate / audit / setup-hooks /
  release-preflight / helm-lint / helm-template 타겟 추가
- `.lefthook.yml` — helm-lint (pre-commit) + helm-template + gitleaks (pre-push)
- `docs/kb/adr/0024-helm-chart-manual-pattern-artifacthub.md` — 결정 ADR
- `docs/kb/adr/0021-helm-chart-kubebuilder-helm-plugin.md` — Status: Superseded

연관 commit (다른 repo): postgres-operator `314af15 fix(release): docker buildx
--platform linux/amd64 강제` (글로벌 §2 정합).

## 다음 단계

### 1. ArtifactHub repo 신규 등록 (사용자 수동, 외부 UI)

- URL: https://artifacthub.io/control-panel/repositories
- ADD REPOSITORY → Helm Chart →
  Name: `valkey-operator`, URL: `https://keiailab.github.io/valkey-operator`
- 생성된 UUID 를 `charts/artifacthub-repo.yml` 의 `repositoryID` placeholder
  (`00000000-0000-0000-0000-000000000000`) 에 교체 → follow-up commit.
- 검증 명령 (등록 후):
  ```bash
  curl -s https://artifacthub.io/api/v1/repositories/helm/valkey-operator \
    | jq '.repository_id, .verified_publisher'
  ```

### 2. 첫 release 트리거 (사용자 결정)

게이트 준비 완료. 실행:
```bash
cd /Users/phil/WorkSpace/public/valkey-operator
make setup-hooks    # 첫 1회 (pre-commit + pre-push 설치)
make gate            # lint + test + helm + audit 전부 PASS 확인
make release VERSION=v0.1.0-alpha.1
```

6단계 자동 실행: gate → preflight → image build/push (linux/amd64, GHCR)
→ git tag → GitHub Release (--prerelease) → helm-publish (gh-pages auto-orphan).

### 3. GitHub Pages 활성화 (gh-pages 첫 publish 후 자동, 또는 수동)

helm-publish 가 `gh-pages` 브랜치를 push 한 *후* GitHub Pages 가 자동
빌드되거나, 수동 활성화 필요:
```bash
gh api repos/keiailab/valkey-operator/pages -X POST \
  -f 'source[branch]=gh-pages' -f 'source[path]=/'
```

### 4. annotations drift 감시

`charts/valkey-operator/Chart.yaml` 의 `artifacthub.io/changes` 는 release 마다
CHANGELOG 에서 manual sync 필요. 자동화는 별도 작업.

## 차단점

- ArtifactHub UI 등록 = 사용자 수동 (CLI 자동화 불가)
- 첫 release 트리거 = 사용자 결정 (GHCR push + GitHub Release 외부 영향)

## 근거 링크

- ADR-0024: `docs/kb/adr/0024-helm-chart-manual-pattern-artifacthub.md`
- ADR-0021 (Superseded): `docs/kb/adr/0021-helm-chart-kubebuilder-helm-plugin.md`
- mongodb-operator 패턴 출처: `/Users/phil/WorkSpace/public/mongodb-operator/Makefile` line 75-148
- postgres-operator 패턴 출처: `/Users/phil/WorkSpace/public/postgresql-operator/Makefile` line 195-243
- 글로벌 §2 (buildx --platform linux/amd64): `~/.claude/CLAUDE.md` §2
- RFC 0002 (GH Actions 금지): `~/Documents/ai-dev/rfcs/0002-no-github-actions.md`
