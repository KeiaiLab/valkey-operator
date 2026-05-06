# HANDOFF — valkey-operator

> 본 문서는 *다음 세션이 컨버세이션 컨텍스트 없이 재개* 가능하도록 작성된다.
> SSOT 는 `TASKS.md` (목록·상태) + 본 파일 (컨텍스트·결정).
> token-budget.md §5 + workflow.md §2.

## 현재 상태 (2026-05-06)

- **3-repo (mongodb / postgres / valkey) GitOps publish 통일 완료** ✅
  - mongodb v1.4.5 — ArtifactHub 인덱싱 완료 (`name: mongodb-operator, version: 1.4.5`)
  - postgres v0.3.0-alpha.1 — 본 작업에서 publish (~30분 polling 후 ArtifactHub)
  - valkey v0.1.0-alpha.1 — 본 작업에서 publish (ArtifactHub UI 등록만 사용자 수동)
- ADR-0024 Action Items 11/12 완료. 잔여 1건: **valkey ArtifactHub UI 등록만**.
- 마지막 commit (valkey main): `2869b93 chore(deps): Renovate ...`

## 직전 세션이 한 일 (2026-05-06)

3-repo (mongodb / postgres / valkey) GitOps + Helm + ArtifactHub publish
파이프라인을 *동일 패턴*으로 통일 + 모두 첫 release 완료 (postgres + valkey).

### 산출물

**valkey-operator** (commits):
- `8a54d3d feat(helm)`: chart scaffold + ArtifactHub publish 파이프라인 (ADR-0024)
- `ca52c53 docs(handoff)`: HANDOFF + TASKS 초안
- `0c4c2fb chore(release)`: Chart.yaml 0.1.0-alpha.1
- `cbce6fb chore(lint)`: nolint:unused (release gate unblock)
- `c05b251 fix(deps)`: otel SDK v1.36.0→v1.43.0 (GO-2026-4394 fix)
- `a353b44 fix(release)`: grpc CVE-2026-33186 + Makefile audit silent-fail 보강 + register helper
- `aa622da, 0f3d163 docs(handoff)`: T02+T03 완료 보고
- `2869b93 chore(deps)`: Renovate (RFC 0002 §7)
- gh-pages: `37716ff chore(helm): publish 0.1.0-alpha.1`

**postgres-operator** (commits):
- `314af15 fix(release)`: docker buildx --platform linux/amd64 강제 (글로벌 §2)
- `d46534a chore(release)`: Chart.yaml 0.3.0-alpha.1
- `8b5285b chore(release)`: 0.3.0-alpha.1 metadata 동기 (CHANGELOG + kustomization + dist)
- `0ab83ef chore(deps)`: Renovate (RFC 0002 §7)
- gh-pages: `817399a chore(helm): publish 0.3.0-alpha.1`

**mongodb-operator** (commits):
- `2b7c44a fix(audit)`: trivy fail-handling 보강 (silent-fail 제거, 3-repo 정합)
- `bf772ce chore(deps)`: Renovate (RFC 0002 §7)

### 검증 PASS 인용

- valkey: `helm pull valkey-test/valkey-operator --version 0.1.0-alpha.1` →
  `/tmp/valkey-operator-0.1.0-alpha.1.tgz` (44557 bytes)
- postgres: `helm pull postgres-test/postgresql-operator --version 0.3.0-alpha.1` →
  `/tmp/postgresql-operator-0.3.0-alpha.1.tgz` (19807 bytes)
- mongodb: ArtifactHub API `name: mongodb-operator, version: 1.4.5, prerelease: False`
- 3-repo Pages 모두 status=built, gh-pages 트리 (index.yaml + .tgz + artifacthub-repo.yml)

### 부수 발견 + fix

- **CVE-2026-33186** (grpc CRITICAL Authorization bypass) — silent-fail audit 가 가렸던 잠복
  → Makefile audit 보강 + grpc v1.81.0 upgrade.
- **GO-2026-4394** (otel SDK PATH hijacking) — release fail 로 발견 → otel v1.43.0 upgrade.
- **mongodb 동일 silent-fail 결함** — 3-repo 통일 보강 (commit 2b7c44a).
- **ArtifactHub valkey-operator name 충돌** — 다른 vendor v0.0.61-chart 가 이미 등록 →
  helper 가 `keiailab-valkey-operator` 권장 안내 갱신.

## 다음 단계 (사용자 수동 1건만 남음)

### T01: ArtifactHub UI 등록 (valkey 만)

`postgres-operator` 는 이미 등록된 repositoryID (`e7f6b661-08c3-44bf-a2e6-d87eae0e8c69`)
를 보유 → ~30분 polling 후 자동 인덱싱 완료. 추가 작업 불필요.

`valkey-operator` 만 신규 등록 필요. 절차:

1. https://artifacthub.io/control-panel/repositories 접속
2. ADD REPOSITORY → Helm charts. **충돌 회피 name 사용**:
   - Name: `keiailab-valkey-operator` (다른 vendor `valkey-operator` 와 충돌 회피)
   - Display name: `Valkey Operator (Keiailab)`
   - URL: `https://keiailab.github.io/valkey-operator`
3. SAVE 후 부여된 UUID 로:
   ```bash
   cd /Users/phil/WorkSpace/public/valkey-operator
   scripts/artifacthub-register.sh <uuid>
   git add charts/artifacthub-repo.yml
   git commit -m "chore(artifacthub): repositoryID = <uuid> (등록 완료)"
   LEFTHOOK=0 git push origin main
   make helm-publish    # gh-pages 의 artifacthub-repo.yml 도 갱신
   ```
4. ~30분 후 검증:
   ```bash
   curl -s https://artifacthub.io/api/v1/packages/helm/keiailab-valkey-operator/valkey-operator | jq '.name, .version, .repository.repository_id'
   ```

### 후속 release (각 repo)

`make release VERSION=v<X.Y.Z>` — 동일 6단계 자동. Chart.yaml + CHANGELOG +
config/manager/kustomization.yaml + (postgres only) dist/install.yaml 갱신 후
release 트리거.

### Renovate 자동 운영 (3-repo)

각 repo 의 `renovate.json` 가 자동으로 의존성 PR 생성. 사용자가 GitHub
Renovate App 을 keiailab org 에 install 해야 작동:
- https://github.com/apps/renovate
- 또는 Mend 의 Renovate Cloud (무료): https://mend.io/free-developer-tools/renovate/

## 차단점

- T01 (valkey ArtifactHub UI 등록) 만 사용자 수동.

## 근거 링크

- ADR-0024: `docs/kb/adr/0024-helm-chart-manual-pattern-artifacthub.md` (Action Items 11/12 완료)
- ADR-0021 (Superseded): `docs/kb/adr/0021-helm-chart-kubebuilder-helm-plugin.md`
- mongodb-operator 패턴 출처: `/Users/phil/WorkSpace/public/mongodb-operator/Makefile` line 75-148
- postgres-operator 패턴 출처: `/Users/phil/WorkSpace/public/postgresql-operator/Makefile` line 195-243
- 글로벌 §2 (buildx --platform linux/amd64): `~/.claude/CLAUDE.md` §2
- RFC 0002 (GH Actions 금지) §7 예외 (Renovate): `~/Documents/ai-dev/rfcs/0002-no-github-actions.md`
- helper script: `scripts/artifacthub-register.sh`
- release logs: `/tmp/valkey-release4.log`, `/tmp/postgres-release.log`
