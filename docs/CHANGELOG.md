# Changelog

> Historical record of all notable changes. Single-language (mixed
> English + Korean entries reflecting the commit-time tone). For
> localized release-notes summaries, see the per-release GitHub Release
> page.

본 프로젝트의 모든 주요 변경은 본 파일에 기록된다.
형식: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
버저닝: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

자동 생성: `git-cliff` (P1 §2.3 표준) — release tag 시점에 PR 자동 갱신.

## [Unreleased]

## [1.0.13] - 2026-05-13

### Added

- ADR-0045: Restore GitHub Actions workflows for OSS CI (scoped
  deviation from RFC-0002) (#89).
- ADR-0046: SLSA-3 provenance + cosign keyless signing for release
  artifacts — image + chart + SBOM (#92).
- `.github/FUNDING.yml` — GitHub Sponsors disclosure (#91).
- `.github/workflows/scorecard.yml` — weekly OpenSSF Scorecard
  analysis + SARIF upload (#94).
- `.github/workflows/dependency-review.yml` — block PRs that bring
  in dependencies with High+ CVEs or disallowed licenses (#94).
- `.github/workflows/codeql.yml` — CodeQL Go SAST with the
  `security-extended` query suite (#99).
- `.github/workflows/dco.yml` — server-side DCO sign-off check
  mirroring the local lefthook commit-msg hook (#99).
- `.github/ISSUE_TEMPLATE/config.yml` — disable blank issues and
  expose Security Advisory / Discussions / Runbook contact links
  (#96).
- English README, CONTRIBUTING, SECURITY, GOVERNANCE, MAINTAINERS,
  ADOPTERS as the canonical docs; Korean originals preserved as
  `.ko.md` siblings (#93, #97, #98).
- README "Known limitations" section to keep the SECURITY.md
  cross-link valid (#93 follow-up).
- `.editorconfig` covering Go (tabs), YAML / JSON / Markdown
  (2-space, trim policies), Makefile (mandatory tabs), and shell
  scripts.

### Changed

- Pin every GitHub Actions reference across all eight workflows to a
  commit SHA with a trailing version comment, satisfying the OpenSSF
  Scorecard `Pinned-Dependencies` check (#95).
- `setup-go check-latest: true` and `toolchain go1.26.3` in `go.mod`
  so the stdlib CVE fixes (16 advisories in 1.26.2 / 1.26.3) are
  applied to every CI run (#92).
- `security-scan.yml` now runs on every PR rather than only when
  the diff touches go.mod / Dockerfile — security scans must never
  be skipped because the diff is elsewhere (#92).
- Branch protection on `main` enforces seven required status checks
  (golangci-lint, unit + envtest, build, govulncheck, trivy-fs,
  trivy-image, Review dependencies), strict mode, linear history,
  conversation resolution, force-push and deletion blocked,
  enforce-admins on.
- Repository security toggles: Dependency graph, Automated security
  fixes, Secret scanning, Secret-scanning push protection all
  enabled.
- README Go badge bumped from 1.25+ to 1.26+ (#90).
- CONTRIBUTING.md Go prerequisite bumped to 1.26 to keep the
  `TestGoVersionDockerfileVsGoMod` guard test green (#84
  follow-up).
- `release.yml` `sbom` job now declares `contents: read` and
  `packages: read` alongside `id-token: write` so syft retains GHCR
  manifest access while still keeping its OIDC token (#92 review
  follow-up).
- `cosign --certificate-identity-regexp` tightened to match the
  exact `release.yml` workflow rather than any workflow in this
  repository (#92 review follow-up).

### Security

- Sigstore Rekor transparency-log entries are now produced for every
  signed image, chart, and SBOM (from v1.0.13).
- SLSA-3 provenance attestation produced via
  `slsa-framework/slsa-github-generator` for the container image
  (from v1.0.13).
- SECURITY.md documents the exact `cosign verify` and
  `slsa-verifier verify-image` commands and the certificate
  identity regex required to authenticate artifacts.

### Dependencies

- `actions/setup-go check-latest: true` ensures the CI Go runtime
  receives stdlib CVE fixes automatically.
- 7 dependabot updates merged into main:
  - Docker base: `golang 1.26.3`, `distroless/static@e3f9456` (#80, #81)
  - Go modules: k8s 0.36 + controller-runtime 0.24 + utils + + otel group + ginkgo 2.28.3 + gomega
    1.40.0 (#84–#88).

## [1.0.12] - 2026-05-12

### Changed

- Publish a clean Artifact Hub refresh release after the pre-existing
  `v1.0.11` tag was found to point at an incomplete release commit.
- Refresh the published chart metadata to advertise the Alpine 3.23 Valkey
  runtime image and operator image tag `1.0.12`.
- Keep the chart README and Artifact Hub visible metadata in English.

### Fixed

- Remove the prohibited GitHub Actions workflow files that were introduced
  before the final release commit.

## [1.0.11] - 2026-05-12

### Changed

- Refresh Artifact Hub chart metadata so the published package advertises the
  Alpine 3.23 Valkey runtime image.
- Publish chart/app version `1.0.11` for the current release surface.
- Keep Artifact Hub trust-badge documentation in English.

### Fixed

- Correct the stale Helm repository package that still advertised
  `docker.io/valkey/valkey:9.0.4`.

## [1.0.10] - 2026-05-10

### Added

- OperatorHub.io bundle scaffold + ADR-0037 (PR-B9 first cut, #21).
- `alm-examples` 5 sample inline JSON 추가 (PR-B9.2, #22).
- CITATION.cff 추가 (OSS 메타데이터, #20).

### Changed

- Chart bump v1.0.10 + amd64-only build (CLAUDE.md §2 정합, #27).
- `v1alpha2 zz_generated.deepcopy.go` regenerate (controller-gen 동기, cfd0398).
- bundle: `generate-kustomize-manifests` 단계 제거 (PR-B9.4, mongodb ADR-0023 정합, #23).

### Fixed

- `ValkeyCluster` post-init self-heal (INC-0001 영구 fix, ADR-0039, #25).
- v1alpha1 `storageversion` 마커 추가 + controller-gen regenerate (PR-A2.2.5, #19).
- `ReadOnlyRootFilesystem=true` 활성 (modern security baseline 마지막 layer, 3aa5480).

### Docs

- INC-0001 ValkeyCluster bootstrap skip — production 운영 cluster 19h fail recovery (#24).
- INC-0001 cluster_state=fail 회복 runbook + ADR-0039 self-heal 명시 (AI-0004, #26).
- ADR-0026 부분 회복 진행 명시 (PR-A2.2.* 누적, #18).
- HANDOFF PR-A2.2.5 머지 결과 + 다음 진입점 갱신 (1818031).

## [1.0.9] - 2026-05-10

### Added

- v1alpha2 type definition module + `AuthSpec.Required` toggle (PR-A2.1, #6).
- v1alpha2 Hub marker 5 type 추가 (PR-A2.2.1, #15).
- v1alpha1 `ConvertTo`/`ConvertFrom` 5 type 본문 (PR-A2.2.2, #16).
- `cmd`: v1alpha2 SchemeBuilder 등록 (PR-A2.2.3.a, #17).
- Valkey Custom Modules type 추가 (v1alpha2, PR-C6.1, ADR-0032, #14).
- `AuthSpec.RotationPolicy` enum 추가 (v1alpha2, PR-B7.1, ADR-0031, #12).
- `PodSecurity` Restricted optional toggle (v1alpha2, PR-A3.2, ADR-0036, #10).
- `NetworkPolicy.AutoCreate` optional toggle (v1alpha2, PR-A3.1, ADR-0035, #9).
- RFC-0018 `pkg/finalizer` migration (controller, PR-A6 first cut, ADR-0038, #8).
- release: cosign sign + SLSA L2 in-toto attestation + ADR-0033 (PR-A4, #5).

### Changed

- (RFC-0018 `SetAvailable` + `SetReadyFalse` 사용 가능, #7).

### Docs

- ADR-0018 정식 작성 — Cluster Auto-Resharding (PR-B8.1, #13).
- Sentinel migration runbook (PR-C7, ADR-0017 거절 보강, #11).

## [1.0.8] - 2026-05-09

### Fixed

- `monitoring.exporter.resources` 가 metrics sidecar 까지 reconcile 안 됨 (운영 통합 시점, 1eb6faf):
  - `STSParams.ExporterResources corev1.ResourceRequirements` 신규 필드 (internal/resources/statefulset.go).
  - `BuildStatefulSet` 의 metrics container 가 `p.ExporterResources` 적용.
  - Valkey + ValkeyCluster controller 가 `exporterResources(spec.Monitoring)` helper 로 전달.
  - 빈 ResourceRequirements (default) → K8s Burstable QoS, 이전 동작 동등 호환.

### Changed

- Chart bump to 1.0.7 (8408005).

## [1.0.7] - 2026-05-09

### Changed

- audit (4-repo cross-cut, 2026-05-09): RFC-0017 채택 — `.golangci.yml` + `.custom-gcl.yml` 신규 (postgres 표준 cp + depguard 정리), Makefile `validate` 타겟 추가 (kustomize + helm lint + helm template). ADR-0030 등재. 본 repo `.lefthook.yml` 은 RFC-0017 §3.1 표준 원본으로 승격됨 (변경 없음, 0aea740).
- (4833f13).
- `.codecov.yml` 신규 (4-repo target 70% 절대 floor 통일, d381587).

### Fixed

- `.golangci.yml` depguard 비활성 (golangci-lint v2.8 schema 가 빈 deny list 거부) — valkey internal boundary 도입 후 ADR 와 함께 재활성. 17 linter 활성 (logcheck plugin 포함, 9dae535).
- lint: valkey 잔여 37 lint 0 issues 달성 (goconst 17 + unparam 17 + gocyclo 3, 8ba60a7).
- lint: lll/prealloc/revive 5건 fix (안전 cleanup, 5d16c94).
- lint: modernize 20 + copyloopvar 2 자동 fix (slices.Contains 등, 8820460).

### Docs

- CHANGELOG entry + deps log (audit 마무리, bd667ad).

## [1.0.6] - 2026-05-08

### Added

- TLS `clientAuth` field — required/optional/disabled mTLS toggle (0c804c9).
- renovate: auto-update PR 진입점 (Go modules + image tag, ba3c9af).

## [1.0.5] - 2026-05-08

### Fixed

- Artifact Hub 가 `1.0.4` chart icon `https://valkey.io/img/Valkey-Logo-RGB-Color.svg` 를 가져오다 404 로 tracking warning 을 내던 문제를 수정했다. 현재 Valkey 사이트에서 200 응답하는 `https://valkey.io/img/valkey-horizontal.svg` 로 교체했다.

## [1.0.4] - 2026-05-08

### Added

- Service builder: TLS 활성 시 client-tls (6380) port expose. BuildClientService + BuildHeadlessService 의 tlsEnabled 인자 추가. 외부 client 가 rediss:// scheme 으로 connect 가능 (tls-auth-clients=yes 시 client cert 별도 발급 필요, 본 patch 는 server-side TLS 외부 노출 인프라).

## [1.0.1] - 2026-05-07

### Fixed

- `ValkeyBackup` 이 `ValkeyCluster` 대상에서 첫 번째 pod 의 `dump.rdb` 만
  저장하던 문제를 수정했다. 이제 샤드별 primary pod 를 기준으로
  `shard-N/dump.rdb` 구조를 생성해 cluster restore 기본 shard layout 과
  직접 호환된다.
- `ValkeyRestore` 가 `ValkeyCluster` 대상 복구 중 pause/unpause annotation 을
  `Valkey` CR 에만 적용하던 문제를 수정했다.
- multi-pod restore 의 기존 source PVC 검증에서 `ReadOnlyMany` 뿐 아니라
  read-only mount 가능한 `ReadWriteMany` PVC 도 허용한다.

## [1.0.0] - 2026-05-07

### Added

- 첫 stable 릴리스. Valkey `9.0.4` 기본값, `8.0.9`/`8.1.7` milestone
  호환성, `ValkeyCluster` sharded HA, 자동 failover, `ValkeyBackup`,
  `ValkeyRestore`, `ValkeyBackupTarget`, restricted PodSecurity 기본값,
  `linux/amd64`/`linux/arm64` multi-arch operator image를 포함한다.

## [0.1.0-alpha.5] - 2026-05-07

### Fixed

- **Runtime P0 — restricted PodSecurity 네임스페이스에서 Valkey Pod 생성 실패** (`internal/resources/statefulset.go`):
  Valkey StatefulSet 컨테이너가 `allowPrivilegeEscalation=false`,
  `capabilities.drop=[ALL]`, `seccompProfile.type=RuntimeDefault` 기본값을 갖지 않아
  `data-staging` 네임스페이스에서 Pod 생성이 금지됐다. 기본 Valkey 컨테이너와
  metrics sidecar에 restricted SecurityContext를 주입했다.

## [0.1.0-alpha.4] - 2026-05-07

### Fixed

- **Release P0 — operator image build metadata 누락** (`Makefile`):
  `make release`가 Dockerfile의 `VERSION`/`COMMIT`/`BUILD_DATE` build args를 전달하지 않아
  실제 배포 이미지의 `/manager --version`과 `valkey_cluster_build_info`가
  `dev/none/unknown`으로 노출됐다. release target에서 tag, git commit, UTC build date를
  주입하도록 수정했다.
- **Release P0 — chart affinity와 image platform 불일치** (`Makefile`):
  chart 기본 affinity는 `linux/amd64`와 `linux/arm64` 노드를 허용하지만 release image는
  `linux/amd64`만 push했다. release build를 `linux/amd64,linux/arm64` multi-arch로 변경했다.

### Added

- release target이 build metadata와 multi-arch platform을 강제하는 회귀 테스트 추가.

## [0.1.0-alpha.3] - 2026-05-07

### Added

- Valkey latest default 정렬: API default, CRD default, Helm values,
  ArtifactHub examples/images, samples, GitOps workload CR 을 `9.0.4` 로 갱신.
- `SupportedValkeyVersions` whitelist 를 `8.0.9`, `8.1.6`, `8.1.7`, `9.0.4`
  로 명시하여 최신 + 8.0/8.1 milestone patch 호환 기준을 문서화.
- ValkeyCluster 9.0.4 sharded 3x1 Kind smoke: 6 pod Ready, `cluster_state=ok`,
  16384 slots, SET/GET 검증.

### Fixed

- Redis 8.2.x RDB 를 Valkey 9.0.4 로 직접 restore 할 때 RDB format 불일치로
  CrashLoopBackOff 되는 경로를 `ValkeyRestore.status.phase=Failed` 로 fail-fast 처리.

## [0.1.0-alpha.2] - 2026-05-07

ADR-0057 Phase A1 (운영 클러스터 사전 배포) 진행 중 발견된 chart RBAC 결함 fix.

### Fixed
- **chart RBAC P0 — `features.{cluster,backup}.enabled=false` 시 informer startup 실패** (`charts/valkey-operator/templates/clusterrole.yaml`):
  이전 chart 가 `features.cluster.enabled` / `features.backup.enabled` 조건부로 `valkeyclusters` / `valkeybackups` / `valkeybackuptargets` / `valkeyrestores` RBAC 부여 — 그러나 operator manager (`cmd/main.go`) 는 *항상* 모든 controller 등록 → flag=false 시 informer 가 `forbidden` 으로 startup 실패. RBAC 와 코드 mismatch 가 production-grade 차단 요인. RBAC 를 *항상 모든 CRD 권한 부여* 로 단순화, feature flag 는 controller 코드 측에서만 처리.

### Verified (운영 클러스터 Phase A1 + A2)
- valkey-operator pod 1/1 Running, Certificate/Issuer/ValidatingWebhookConfiguration Ready
- Valkey CR `valkey-test` (Standalone, valkey 8.1.6, 1Gi ceph-rbd) 1/1 Running
- SET/GET smoke: `SET phase-a2-smoke "OK-2026-05-07"` → `OK`, `GET` → 정상 round-trip
- `INFO server`: valkey_version=8.1.6, tcp_port=6379

### Refs
- ADR-0057 (인프라 부트스트랩 43fd542): self-hosted valkey-operator 채택 로드맵
### Added (GitOps deploy 정합)

- `deploy/overlays/prod/` GitOps 진입점 — config/{crd,rbac,manager} 를 prod ns 로
  정렬 + 자동 생성 Namespace 제거. ArgoCD 단방향 동기 전제.
- `deploy/valkey-cluster.yaml` — production ValkeyCluster sample (db ns,
  shards=3, replicasPerShard=1, ceph-block, auth.enabled=true).
- `deploy/README.md` — 운영 런북.
### Added (cycles 20-90 — Quality systems + production-grade UX)

**Quality 시스템 (39 SSOT 게이트)**:
- ADR governance (4 게이트): file/INDEX/Status/Superseded/Nygard 3-section.
- Alert rules (4): schema/fields/metric/runbook anchor 동기.
- RBAC (2 양방향): kubebuilder:rbac ↔ role.yaml.
- Sample CR (3): strict unmarshal + dir-mapping + metadata.
- ClusterRef.Kind (2 — 3-way): enum ↔ switch case.
- LICENSE + Chart annotation (2).
- Chart artifacts (6): images/CRDExamples/CRD sync/values/NOTES/README YAML.
- Markdown links + anchors (2).
- Webhook + Reconciler 등록 (2).
- dist/install.yaml (2): structure + OPERATOR_IMAGE env.
- Release-checklist self-sync (1, 양방향 cycle 60).
- Kustomize ↔ chart sync family (3): resources/probes/securityContext.
- Cross-feature interaction family (3): NP+webhook/tracing/backup.
- features.* RBAC + reconciler 동기 (1).
- value↔template binding (1).
- chart args ↔ operator flags (1).

**자동화 (실수 발생 자체 차단)**:
- `make manifests` chart CRD 자동 sync.
- pre-push lefthook 6-hook (full-lint + gitleaks + go-mod-tidy + helm-lint +
  helm-template + unit-test).
- `make sbom` (syft SPDX) + trivy post-scan release pipeline 자동 첨부.

**Production-grade UX**:
- ldflags chain (cycles 53-57): cmd/main.go → Dockerfile → docker-build →
  docker-buildx → release.sh → Prometheus build_info gauge.
- chart features 5 (cycles 65/72/73/74/82): tracing + NetworkPolicy + webhook +
  watch.namespaces + autoscaling 정직 표시.
- 6-layer documentation: README + chart README + NOTES.txt + CONTRIBUTING +
  release-checklist + HANDOFF (모든 사용자 역할별 entry point).
- runbook §7.1 환경변수 진단 가이드.
- 3-layer DX: lefthook auto + `make ssot-check` (1.4s) + `make gate` (30s).

**구현된 기능 (cycles 72-74 — chart 4 unused values 중 3 해결)**:
- charts/valkey-operator/templates/networkpolicy.yaml — operator pod default-deny.
- charts/valkey-operator/templates/webhook.yaml — cert-manager 의존 admission webhook.
- WATCH_NAMESPACES env — namespace-scoped watch (cache.DefaultNamespaces).

**구현된 기능 (cycles 99-106 — kubebuilder boilerplate completion + Helm parity)**:
- cycle 100 — runbook §7.0 production TLS 강화 가이드 (insecureSkipVerify → cert-manager).
- cycle 101 — config/manager + chart values nodeAffinity (amd64+arm64+linux) — mixed-arch ImagePullBackOff 차단.
- cycle 102 — config/default/kustomization.yaml `- ../prometheus` 활성 — kustomize 사용자 도 ServiceMonitor + PrometheusRule 자동 설치.
- cycle 103 — charts/.../prometheusrule.yaml — Helm 사용자 의 10 alerts silent loss 차단.
- cycle 104 — charts/.../metrics-auth-rbac.yaml — secure metrics 의 Prometheus 401 silent fail 차단.
- cycle 106 — charts/.../deployment.yaml webhook serving config (--webhook-cert-path + 9443 + cert mount) — webhook 활성화 시 operator 9443 listen 정확히 작동.

**production gap 발견·수정 (27건)** + **내부 부채 cleanup (3건)** + **5 hot-path benchmark** + **8 결함 family progressive completion**.

### Added (iter 7+ — 부트스트랩·검증 사이클)
- README quickstart (kind 기반): 5 단계 부트스트랩 + 데이터 plane smoke + 운영 시나리오 매트릭스. [iter 6]
- ADR-0011: Required 필드 (omitempty 부재) 의 mutating webhook defaulting 패턴. [iter 4]
- ADR-0012: CLUSTER MEET hostname 미지원 → DNS 해석 후 IP 사용. [iter 4]
- ADR-0013: Auth.Enabled 강제 true (옵션 A 채택). [iter 5]
- `internal/valkey/cluster.go::resolveAddrIP`: hostname → IP 정규화 (IPv4 prefer).
- `internal/webhook/v1alpha1/valkey_webhook.go`: Version + Auth.Enabled 정규화.
- `internal/webhook/v1alpha1/valkeycluster_webhook.go`: Shards/ReplicasPerShard/Version/Auth defaulting.
- `api/v1alpha1/common_types.go`: `DefaultValkeyVersion` / `DefaultValkeyImage` 상수.
- `internal/controller/valkeycluster_controller.go`: pods RBAC 추가 (status reconciliation).
- `config/samples/cache_v1alpha1_valkeybackup.yaml`: 의미있는 ClusterRef 채움.
- `.dockerignore`: `*.tmpl`, `*.lua`, `*.sh` 패턴 — embed 자산 보존.
- lefthook 활성화 (pre-commit + pre-push + commit-msg) + Conventional Commits 패턴.

### Fixed (iter 7+)
- ValkeyBackup controller 테스트 fixture 의 ClusterRef 누락 (webhook validation 통과 못함).
- ValkeyCluster bootstrap 무한 retry: CLUSTER MEET 가 hostname 거부 → DNS 해석.
- defaulting webhook 이 required 필드 (Version/Shards/ReplicasPerShard) 채우지 않아 무한 reconcile 루프.
- pods RBAC 누락으로 ValkeyCluster status 갱신 불가.
- lefthook commit-msg 가 `$1` 대신 `{1}` 사용.
- lefthook golangci-lint cross-directory staged files 오류.

### Verified (iter 7+ 실측)
- e2e suite: 5/5 PASS (manager 시작, metrics endpoint, cert-manager, mutating/validating webhook CA injection).
- integration test: 14 케이스 PASS (실 valkey:8 컨테이너 + 6노드 클러스터 부트스트랩).
- unit test: 4 패키지 PASS (`internal/{controller,resources,valkey,webhook}`).
- 회복성: primary pod force kill → STS 재생성 → operator 재 promote → 데이터 보존 (canary `preserved`).
- scale up/down: 3→5→2, master_link_status:up, 데이터 보존.
- 클러스터 모드: 3 shards × 2 instances, cluster_state:ok, slots:16384/16384 OK.
- TLS+mTLS 클러스터 (cert-manager + selfsigned ClusterIssuer): Phase=Running, slots=16384/16384 OK, 데이터 plane SET/GET 성공 (cluster mode -c, 다중 shard 분산).
- NetworkPolicy 리소스 정합성: deny-by-default + selfPeer ingress (6379) + ownerReferences (Standalone). cluster mode 시 16379 추가. 강제 동작 검증은 Calico/Cilium CNI 필요 (kindnet 미지원).
- operator metrics endpoint (HTTPS:8443, ServiceAccount 토큰 인증): controller_runtime_* 메트릭 정상 노출. 커스텀 valkey_cluster_* 메트릭은 ValkeyCluster reconcile 시 emit.

### Added (iter 1-6 — 이전 사이클)
- ValkeyCluster Reconcile 14 단계 구현 (cluster mode CRD bootstrap → CLUSTER MEET / ADDSLOTS / REPLICATE → status polling). [iter 1]
- `internal/valkey/cluster.go`: `CreateCluster` 단계별 멱등 분리 (`ensureMeet` / `ensureSlots` / `ensureReplicas`). partial-state 회복 가능. [iter 2]
- `internal/valkey/nodes.go`: `CLUSTER NODES` 응답 파서 (`NodeView`, `SlotRange`). [iter 2]
- 통합 테스트 (`//go:build integration`): 실 valkey:8 컨테이너 6노드 클러스터 — 4 시나리오 PASS. [iter 2-4]
- Finalizer graceful cleanup: `gracefulClusterTeardown` (best-effort `CLUSTER FORGET`, 30s timeout). [iter 2]
- Prometheus metrics: 7 시계열 (state_ok, assigned_slots, shards, ready_replicas, reconcile_total, reconcile_errors_total, phase). [iter 3]
- ScalePolicy.Deliberate 가드: 미동의 시 `Status.PendingScale` 기록 + STS replicas 보존. [iter 3]
- ServiceMonitor (`monitoring.coreos.com/v1` unstructured) 자동 생성 + metrics Service 분리. [iter 3]
- AutoFailover ConfigMap 디렉티브 통합 (`cluster-replica-no-failover yes`). [iter 3]
- `make integration-test` Makefile target. [iter 2]
- `buildShardStatusFromNodes`: CLUSTER NODES 기반 ShardStatus (failover 정확 반영, ADR-0007). [iter 4]
- TLS RootCAs 로드: `Spec.TLS.CustomCert.SecretName.ca.crt` → x509 CertPool (ADR-0008). [iter 4]
- Validating + Mutating Webhook (양 CRD): 8 조합 검증 + immutable 가드 (Mode, Storage, TLS toggle). [iter 5]
- ShardStatus pod ordinal 매핑: `buildPodAddrMap` (K8s Pod list → "vk-N"). [iter 5]
- cert-manager Certificate 자동 생성: `Spec.TLS.CertManager.IssuerRef` 명시 시 Certificate CR 자동 + secretName 자동 발견 (ADR-0010). [iter 6]
- Version upgrade detection: `decidePhase` 가 Spec.Version != Status.Version 감지 시 Phase=Upgrading. [iter 6]
- ADR 0001-0010 (10건, 2건 supersede): defaulting → webhook (0001→0009), ShardStatus spec → NODES (0004→0007), TLS 단계적 통합 (0003 → 0008 → 0010). [iter 1-6]
- lefthook 설정 (.lefthook.yml). [iter 3]

### Changed
- `valkey/replication.go`: `SlaveOf` → `ReplicaOf` (Redis 5.0+ deprecated API). 모던 Valkey `role:replica` 인식 추가. [iter 1]
- `valkeycluster_controller.go`: `pollClusterState` 가 모든 노드 fallback (`queryAnyNode`) — pod-0 SPOF 제거. [iter 1]
- `dialPod` 가 Spec.TLS.Enabled 통과 (이전: 무시됨). [iter 1]
- SetupWithManager: `Owns(PDB, NetworkPolicy)` 추가 — drift 감지. [iter 1]

### Fixed
- 컴파일 오류: `&appsv1StatefulSet{}.s` → `(&appsv1StatefulSet{}).Inner()` (Go struct literal addressability). [iter 1]
- ensureReplicas 에 gossip 수렴 retry — `replicateWithRetry` (10회 backoff, "Unknown node" 흡수). [iter 2]
- `parseReplicationInfo` 가 modern Valkey `role:replica` 인식 — 이전엔 매 reconcile 마다 ReplicaOf 재호출 (멱등성 결함). [iter 1]

### Documentation
- ADR 인덱스 (`docs/kb/adr/INDEX.md`).
- 본 CHANGELOG. [iter 3, iter 6 갱신]

### Test Coverage Snapshot (iter 6 끝)
- internal/controller: 50.5%
- internal/resources: 33%+
- internal/valkey: 33.7%
- **internal/webhook/v1alpha1: 80.7%** (신규 패키지)
- 단위테스트: 60+건
- 통합테스트: 4 시나리오 (실 Valkey 6노드)
