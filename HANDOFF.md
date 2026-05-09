# HANDOFF — valkey-operator

> 본 문서는 *다음 세션이 컨버세이션 컨텍스트 없이 재개* 가능하도록 작성된다.
> SSOT 는 `TASKS.md` (목록·상태) + 본 파일 (컨텍스트·결정).
> token-budget.md §5 + workflow.md §2.

## 2026-05-10 PR-A2.2.5 controller-gen regenerate — storageversion regression fix

- **branch**: `feat/pr-a2.2.5-controller-gen-regenerate` (push 완료, PR 미생성)
- **commit**: `b138322 fix(api): v1alpha1 storageversion 마커 추가 + controller-gen regenerate (PR-A2.2.5)`
- **문제 진단**: 직전 세션의 미커밋 working tree (5 CRD diff 5001 lines) 가
  *모든 CRD 에서 storage:false* 상태였다. 이 상태로 `kubectl apply` 시 K8s
  validation error (정확히 1 storage:true 요구). controller-gen v0.18+ 는
  multi-version CRD 에 `+kubebuilder:storageversion` 명시 마커 강제 — v1alpha2
  type 도입 후 5 root type 모두 마커 부재로 본 regression 발생.
- **결정**: v1alpha1 에 마커 추가 (production storage 보존). conversion webhook
  ADR-0026 deferred 상태이므로 v1alpha2 storage 승격은 *별 PR (PR-A2.3 webhook
  본격 도입)* 시점으로 미룸. 5 type 모두 동일 패턴 적용.
- **검증 인용**:
  ```
  $ make manifests
  ✓ controller-gen regenerate / charts/valkey-operator/crds 자동 sync
  $ for f in config/crd/bases/cache.keiailab.io_valkey*.yaml; do
      grep -c "storage: true" $f; done
  1 1 1 1 1   (5 CRD 정확히 1 storage:true)
  $ make lint
  0 issues
  $ go test ./api/... ./internal/webhook/...
  ok api/v1alpha1, api/v1alpha2, internal/webhook/v1alpha1 (3 PASS)
  $ git push origin feat/pr-a2.2.5-controller-gen-regenerate
  pre-push hooks: helm-lint + helm-template + platforms-amd64-guard
                  + unit-test (20.74s) 모두 PASS
  ```
- **다음 단계 (사용자 invocation 시점)**:
  1. PR 생성: `gh pr create --base main --title "fix(api): v1alpha1 storageversion 마커 추가 (PR-A2.2.5)"` — 사용자 명시 승인 권장 (외부 effect).
  2. 머지 후 PR-A2.2.6 (controller import 변경 + ensureAuthSecret Required 분기) 진입.
  3. 또는 PR-A4 (cosign + SLSA L2) 독립 진행 가능.

## 2026-05-09 Sprint A 진입 (PR-A2 / A3 / A4 / A6) — Helm 차트 비교 plan

> Plan: `~/.claude/plans/1-https-artifacthub-io-packages-helm-clo-synthetic-gem.md`
>
> Sprint A 의 valkey-operator 측 4 PR 진입점. 본 HANDOFF 섹션은 *진입점
> 명시* 만 포함하며, valkey-operator 코드 변경은 미진행 (commons v0.6.0
> tag 머지 후 PR-A2 시작).

### PR-A2: v1alpha2 + conversion webhook + AuthSpec.Required toggle (T3)

- **의존**: 없음 (commons v0.6.0 무관 — v1alpha2 scaffold 자체는 commons 의 status/finalizer 비의존).
- **시작 절차**:
  1. `mkdir -p api/v1alpha2`
  2. `cp api/v1alpha1/{groupversion_info,common_types,valkey_types,valkeycluster_types,valkeybackup_types,valkeybackuptarget_types,valkeyrestore_types,zz_generated.deepcopy}.go api/v1alpha2/`
  3. `api/v1alpha2/groupversion_info.go` 의 `Version: "v1alpha2"` + 같은 group `cache.keiailab.io`.
  4. `AuthSpec.Required *bool` (`omitempty`, `kubebuilder:default:=true`) 신규.
  5. `api/v1alpha2/conversion.go` 신규 — v1alpha1 ↔ v1alpha2 양방향 (default true 로 v1alpha1 강제 동작 매핑).
  6. v1alpha2 Hub 마킹 (`api/v1alpha2/conversion.go` 의 `func (*Valkey) Hub() {}`).
  7. `make manifests` 후 chart CRD 동기 확인.
- **ADR**: ADR-0034 신규 (`docs/kb/adr/0034-auth-optional-v1alpha2.md`) — ADR-0013 supersede, ADR-0026 deferred 회복.
- **controller 변경**: `internal/controller/valkey_controller.go` 의 `ensureAuthSecret` 분기:
  ```go
  if !ptr.Deref(spec.Auth.Required, true) {
      // skip auth provisioning — user opted out
      return nil
  }
  ```
- **검증**: `kubectl apply v1alpha1 sample` → v1alpha2 hub 저장 + 전체 spec 필드 보존, `kubectl apply v1alpha2-auth-disabled.yaml` → requirepass 미설정.

### PR-A3: NetworkPolicy.AutoCreate + Security.PodSecurityRestricted toggle (T2)

- **의존**: PR-A2 (v1alpha2 base).
- **신규 필드** (api/v1alpha2):
  - `NetworkPolicySpec.AutoCreate *bool` (default true) — 기존 `Enabled` 와 의미 분리: `Enabled` 는 *정책 사용 여부*, `AutoCreate` 는 *operator 가 NP 리소스 생성 책임을 가질지*.
  - `SecuritySpec.PodSecurityRestricted *bool` (default true) + `PodSecurityContextOverride *corev1.PodSecurityContext`.
- **ADR**: ADR-0035 (NetworkPolicy.AutoCreate optional, ADR-0057 supersede) + ADR-0036 (PSS Restricted optional).
- **builder 분기**:
  - `internal/resources/networkpolicy.go`: `AutoCreate=false` 시 빌드 스킵.
  - `internal/resources/statefulset.go`: `PodSecurityRestricted=false` 시 `PodSecurityContextOverride` 적용 허용.

### PR-A4: cosign + SLSA L2 in-toto attestation (T2, *독립적*)

- **의존**: 없음 — PR-A2 와 병렬 가능.
- **scripts/release.sh 변경 (Step 6 docker-buildx 후 신규 Step 6.5)**:
  ```bash
  # Step 6.5: cosign sign + SLSA L2 in-toto attestation
  if command -v cosign >/dev/null && [[ -n "${COSIGN_KEY:-}" ]]; then
    cosign sign --key "$COSIGN_KEY" --yes "$IMAGE"
    cosign attest --predicate provenance.json --type slsaprovenance --key "$COSIGN_KEY" "$IMAGE"
  fi
  ```
- **Makefile 신규 타겟**: `sign-image` + `attest-provenance` (기존 `release-notes` 타겟 패턴 사용).
- **release-smoke-test.sh** 확장: `cosign verify` + `cosign verify-attestation --type slsaprovenance` 단계.
- **ADR**: ADR-0033 (`docs/kb/adr/0033-supply-chain-cosign-slsa.md`).
- **결정 (ADR-0033 본문)**: cosign keyless OIDC 는 GHA OIDC 의존 → RFC-0002 (GHA 영구 금지) 충돌 → **keyfile + GitHub Secret + 수동 release sign** 채택. Sigstore Rekor public ledger 보조.
- **외부 비교 차용**: Cloudpirates redis chart 가 cosign signed (Phase 1 조사) — 동일 보증 수준.

### PR-A6: pkg/finalizer + pkg/status migration (T3)

- **의존**: operator-commons v0.6.0 (PR-A1 commit 완료 — `~/.claude/plans/1-...md` HANDOFF 의 PR-A1 결정) tag 머지.
- **변경 범위**: 4 controller (Valkey, ValkeyCluster, ValkeyBackup, ValkeyRestore, ValkeyBackupTarget):
  - `controllerutil.AddFinalizer/RemoveFinalizer` → `finalizer.Add/Remove` (`pkg/finalizer.Prefix` + repo 별 suffix).
  - `setCondition` 호출 → `status.SetReady/SetReadyFalse/SetAvailable` 위임 (도메인 ConditionType `ShardReady` 등 보존).
- **ADR**: ADR-0038 신규 (`docs/kb/adr/0038-rfc-0018-pkg-status-finalizer-adoption.md`).
- **회귀**: 기존 envtest 전부 통과 + e2e (kind cluster).

### 차단점

- PR-A2/A3/A6 는 commons v0.6.0 tag 머지 후 진입. PR-A4 는 독립 — *지금 즉시 진입 가능*.
- 본 세션은 commons PR-A1 변경만 완료, valkey 측 코드 변경 없음.

### 근거 링크

- Plan §2 D1/D2/D3 (사용자 결정 1: Auth + NetworkPolicy + PSS optional, v1alpha2 + conversion webhook).
- Plan §2 D5 (cosign + SLSA — P0 supply chain).
- Plan §2 D10/D11 (RFC-0018 pkg/status + pkg/finalizer adoption).
- Plan §3 (v1alpha2 CRD 설계 — `api/v1alpha2/common_types.go` 신규).
- ADR-0013 (Auth always enabled, supersede 대상), ADR-0017 (failover 보존), ADR-0026 (conversion webhook deferred, 회복 대상), ADR-0057 (NetworkPolicy 자동 생성, supersede 대상).

---

## 현재 상태 (2026-05-07, Valkey latest default 정렬 완료)

- **이번 세션 추가 구현**:
  - API default / CRD default / Helm `values.yaml` / ArtifactHub examples+images / samples / GitOps workload CR 기본값을 Valkey `9.0.4` 로 정렬했다.
  - `SupportedValkeyVersions` 는 `8.0.9`, `8.1.6`, `8.1.7`, `9.0.4` 를 허용한다. 최신은 9.0.4, 8.0/8.1 milestone patch 는 호환 whitelist 로 보존한다.
- **검증 인용**:
  ```
  $ go test ./internal/observability ./internal/webhook/v1alpha1 ./internal/controller \
      -run 'TestChartValuesValkeyVersionMatchesAPIDefault|TestChartImagesAnnotationMatchesAppVersion|TestChartCRDExamplesStrictUnmarshal|TestValidate|TestValkey|TestBuild|Test.*Version' -count=1
  ok ./internal/observability ./internal/webhook/v1alpha1 ./internal/controller

  $ make test
  PASS (non-e2e all packages)

  $ make lint
  0 issues

  $ make helm-template
  default/all-features/OTEL/debug/webhook+NetworkPolicy/full-production stack PASS

  $ kustomize build deploy/overlays/prod
  namespace_count=0, line_count=5407
  ```
- **남은 리스크**:
  - Redis 8.2.x RDB 직접 restore 는 여전히 호환 불가다. 현재 구현은 fail-fast 로 멈추며, Bitnami redis-cluster 대체 마이그레이션은 온라인 key copy/dual-write/cutover 또는 Valkey 호환 source dump 경로가 필요하다.

## 이전 현재 상태 (2026-05-07, Phase B RDB 호환성 blocker + fail-fast 완료)

- **이번 세션 추가 발견**:
  - Bitnami redis-cluster chart 13.0.4 의 appVersion/image 값은 Redis 8.2.1 계열이었다.
  - chart 가 가리키는 `docker.io/bitnami/redis-cluster:8.2.1-debian-12-r0` 는 현재 manifest 확인 실패. 그래서 동일 appVersion RDB format 검증은 pull 가능한 `docker.io/redis:8.2.1` 로 실측했다.
  - Redis 8.2.1 이 생성한 RDB 는 Valkey 9.0.4 가 직접 읽지 못한다. 로컬 컨테이너 검증과 Kind E2E 모두 pod log 에 `Can't handle RDB format version 12` 를 남겼다.
- **구현**:
  - `ValkeyRestore` Restoring 단계에서 대상 pod/init container 상태를 읽고 `CrashLoopBackOff`, `RunContainerError`, image pull/config error, non-zero terminated 상태를 감지하면 `status.phase=Failed`, Condition reason=`RestorePodFailed` 로 fail-fast 처리.
  - Kind E2E 추가: `test-redis-rdb-restore-20260507` namespace 에 Redis 8.2.1 Job 으로 `dump.rdb` 생성 → Valkey 9.0.4 대상에 `ValkeyRestore` 적용 → `status.phase=Failed` 확인 → pod log 의 RDB format version 12 확인 → 테스트 데이터 삭제.
  - Kind E2E 추가: `test-valkey-cluster-20260507` namespace 에 `ValkeyCluster` 9.0.4, 3 shards × 1 replica 생성 → 6개 pod Ready → `cluster_state=ok` + `assignedSlots=16384` + `status.version=9.0.4` → `valkey-cli -c SET/GET` 확인 → 테스트 데이터 삭제.
- **검증 인용**:
  ```
  $ go test ./internal/controller -run TestRestore_restoring_valkeyCrashLoop_marksFailed -count=1
  ok github.com/keiailab/valkey-operator/internal/controller 0.909s

  $ KIND_CLUSTER=valkey-redis-rdb-e2e-20260507 IMG=ghcr.io/keiailab/valkey-operator:e2e-redis-rdb-20260507 \
    go test -tags=e2e ./test/e2e/ -v -ginkgo.v -ginkgo.focus 'Redis 8.2.1 RDB'
  SUCCESS! 1 Passed / 0 Failed / 19 Skipped

  $ KIND_CLUSTER=valkey-cluster904-e2e-20260507 IMG=ghcr.io/keiailab/valkey-operator:e2e-cluster904-20260507 \
    go test -tags=e2e ./test/e2e/ -v -ginkgo.v -ginkgo.focus 'sharded ValkeyCluster 9.0.4'
  SUCCESS! 1 Passed / 0 Failed / 20 Skipped
  ```
- **해석**:
  - Phase B 의 "Bitnami redis-cluster RDB 직접 restore" 성공 조건은 현재 만족 불가다. 이것은 operator reconcile 버그가 아니라 Redis 8.2.x RDB format 과 Valkey 9.0.4 reader 호환성 차단이다.
  - 이번 변경은 성공 마이그레이션 구현이 아니라 운영자가 호환 불가 restore 를 무한 대기하지 않게 만드는 안전장치다.
  - Valkey 자체의 sharded 3×1 bootstrap 은 9.0.4 에서 Kind 기준 정상 동작한다. 즉 Bitnami 대체 운영 형태 자체는 가능하지만, Redis 8.2.x RDB 직접 이관 경로는 별도 설계가 필요하다.
- **다음 단계**:
  1. Bitnami 대체 마이그레이션 경로 재설계: RDB 직접 restore 대신 온라인 key copy/dual-write/cutover 또는 Valkey 호환 source dump 경로 중 하나를 선택.
  2. `ValkeyRestore` 실패 후 사용자 복구 절차 문서화: 원본 PVC 보존, 대상 PVC 교체/재생성, 재시도 기준.

## 이전 현재 상태 (2026-05-07, Phase B version-upgrade gate 재검증 완료)

- **이번 세션**:
  - `spec.version.version` 8.1.6 → 9.0.4 변경이 StatefulSet pod template image 로 반영되는지 `Valkey` / `ValkeyCluster` 양쪽 envtest 회귀 가드 추가.
  - Kind E2E 추가: 전용 namespace `test-valkey-upgrade-20260507` 에 Standalone `Valkey` 8.1.6 생성 → `kubectl patch ... version=9.0.4` → STS template image `docker.io/valkey/valkey:9.0.4` → pod UID 변경 → Ready=True → `status.version=9.0.4` 확인.
  - E2E harness 보강: `config/default` 가 ServiceMonitor/PrometheusRule 을 기본 렌더하므로 BeforeSuite 에 Prometheus Operator CRD 2개 bootstrap/cleanup 추가. focused spec 반복 실행을 위해 manager namespace 생성 idempotent 처리.
- **검증 인용**:
  ```
  $ go test ./internal/controller -run TestControllers -count=1
  ok github.com/keiailab/valkey-operator/internal/controller 16.606s

  $ KIND_CLUSTER=valkey-upgrade-e2e-20260507 IMG=ghcr.io/keiailab/valkey-operator:e2e-upgrade-20260507 \
    go test -tags=e2e ./test/e2e/ -v -ginkgo.v -ginkgo.focus 'spec.version.version changes'
  SUCCESS! 1 Passed / 0 Failed / 18 Skipped

  $ make test
  PASS (non-e2e all packages)

  $ make lint
  0 issues
  ```
- **해석**:
  - 이전 handoff 의 "version upgrade reconcile 결함"은 재현되지 않았고, operator reconcile + 실제 Kind StatefulSet rolling 은 현재 코드에서 정상 동작함이 확인됐다.
  - 남은 Phase B 위험은 RDB 호환/마이그레이션 흐름 자체와 실제 bitnami → 자체 operator restore/cutover 검증이다.
- **다음 단계**:
  1. Phase B RDB 마이그레이션 재시도: bitnami/valkey 9.x dump → 자체 Valkey 9.0.4 restore init container → SET/GET 데이터 검증.
  2. ValkeyCluster sharded mode 에도 9.0.4 cluster bootstrap/slot 16384 smoke 추가.

## 현재 상태 (2026-05-07, governance 표준 정합 — P0+P1 baseline 도달)

- **이번 세션 (상용 제품 수준 trajectory)**:
  - **ADR INDEX 정렬**: ID 오름차순 + 0018-0020 Reserved 슬롯 명시 (재사용 금지 정책 가시화).
  - **작성 가이드 + Reserved 슬롯 정책** 신규 추가.
  - **deps log seed**: `docs/kb/deps/2026-05.md` (P1 추적성, enforcement.md §2.4) — ADR-0022 (minio-go v7) 의존성 결정 + 이전 발견 (otel GO-2026-4394, grpc CVE-2026-33186 CRITICAL fix) 참조.
- **검증 인용**:
  ```
  $ make lint                      # ./bin/golangci-lint run → 0 issues ✓
  ```
- **3-repo governance 정합**: postgres 의 ADR 경로 표준화 (`docs/adr/` → `docs/kb/adr/`) + ADR-0007 신규 (pre-commit 분기 정당화). mongodb 도 ADR-0011 동일 패턴. valkey 는 lefthook 으로 표준 정합 ✓.
- **다음 단계 (열린 트랙)**: T06 이후 GitOps overlay 실 클러스터 검증 (RFC-0004 클러스터 라이브 게이트 발동 영역), 0018 Cluster Resharding ADR 작성.

## Quality baseline (2026-05-07 실측)

`enforcement.md §3.4 (Coverage 합산)` 의 P2 측정 — 본 세션 baseline.

```
$ make test    # exit 0 / FAIL: 0
internal/webhook/v1alpha1   coverage: 93.1% of statements
internal/resources          coverage: 91.3% of statements
internal/observability      coverage: 76.2% of statements
internal/storage            coverage: 68.2% of statements
internal/cli                coverage: 62.5% of statements
internal/controller         coverage: 48.7% of statements
internal/valkey             coverage: 41.6% of statements
cmd                         coverage:  0.0% of statements
```

**80% 목표** 대비:
- ✓ webhook 93.1% / resources 91.3% — 안정 영역 (3-repo 중 최고)
- △ observability 76.2% — 근접
- ✗ controller 48.7% / valkey 41.6% / cmd 0% — envtest 기반 reconcile 강화 권장

`enforcement` 의 "절대치보다 *변경된 코드의 커버 여부*가 우선" 원칙 적용 — 본 baseline 은 회귀 비교 기준점.

## 이전 상태 (2026-05-06, release pipeline 정합 + image ref 버그 수정)

- **HEAD `9a93f4c`**: `chore(deploy): image controller tag v0.1.0 → 0.1.0-alpha.1 (실재 GHCR tag 정합)`
- **HEAD~1 `96f4139`**: `fix(scripts): smoke-test step 번호 [N/5] → [N/6] 정합 + .gitignore (dist/)`
- **이번 세션 핵심 발견**: 이전 image ref 변경 (`v0.1.0`) 은 GHCR 미존재 tag 였음 — ImagePullBackOff 보장 → 0.1.0-alpha.1 (실재 published tag) 로 정정. postgres 패턴 (no v prefix) 정합.
- **3-repo smoke-test 강화 정합**: SBOM (SPDX) asset 검증 + trivy image post-publish HIGH/CRITICAL fixed-only scan 추가됐음. valkey 는 step 번호 누락 버그만 본 세션에서 정정 (SBOM+trivy 본체는 이전 commit 에 있음).
- **다음 단계 (열린 트랙)**: T06 이후 GitOps overlay (`deploy/overlays/prod/` 등) 의 *실 클러스터 적용 검증* 미실시 — `kubectl apply -k deploy/overlays/prod` + ValkeyCluster CR 생성 → Pod Ready / readiness probe smoke 1회 권장. RFC-0004 클러스터 라이브 게이트 발동 영역.

## 이전 상태 (2026-05-06, T06 GitOps deploy overlay — *완료*)

- **T06 머지됨** (어느 PR 인지 git log 로 확인 권장 — `ad9a241` 이전 history).
- **결정 기록**: patch target name 은 raw `system` (config/manager 직접 import → namePrefix 미적용). ValkeyCluster sample = sharded 3×1, ceph-block, auth.enabled=true (ADR-0013). TLS 블록은 cert-manager 미설정 환경 가정으로 주석 유지.

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
