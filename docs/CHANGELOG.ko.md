# Changelog (한국어)

> English: [CHANGELOG.md](CHANGELOG.md) — canonical / 정본

> 주목할 만한 모든 변경의 역사 기록. 영문 정본은 단일 언어이지만
> 본 사본은 한국어 운영 톤으로 재번역한다. 릴리스별 요약은 GitHub
> Release 페이지를 참조.

본 프로젝트의 모든 주요 변경은 본 파일에 기록된다.
형식: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
버저닝: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

자동 생성: `git-cliff` (P1 §2.3 표준) — release tag 시점에 PR 로
자동 갱신.

## [Unreleased]

## [1.0.13] - 2026-05-13

### Added

- ADR-0045: OSS CI 용 GitHub Actions workflow 부분 복귀 (RFC-0002 의
  범위 한정 일탈) (#89).
- ADR-0046: release artifact (image + chart + SBOM) 에 SLSA-3
  provenance + cosign keyless 서명 적용 (#92).
- `.github/FUNDING.yml` — GitHub Sponsors 공시 추가 (#91).
- `.github/workflows/scorecard.yml` — 매주 OpenSSF Scorecard 분석
  + SARIF 업로드 (#94).
- `.github/workflows/dependency-review.yml` — High+ CVE 또는
  비허용 라이선스 의존성을 PR 단계에서 차단 (#94).
- `.github/workflows/codeql.yml` — `security-extended` 룰셋 기반
  Go SAST (CodeQL) (#99).
- `.github/workflows/dco.yml` — 서버 측 DCO sign-off 검사. 로컬
  lefthook commit-msg hook 과 동일 정책 미러링 (#99).
- `.github/ISSUE_TEMPLATE/config.yml` — 빈 이슈 차단 + Security
  Advisory / Discussions / Runbook contact 링크 노출 (#96).
- 영문 README, CONTRIBUTING, SECURITY, GOVERNANCE, MAINTAINERS,
  ADOPTERS 를 정본으로 승격. 기존 한국어 원문은 `.ko.md` 형제로
  보존 (#93, #97, #98).
- README "Known limitations" 절 신설 — SECURITY.md 의 cross-link
  무결성을 유지하기 위함 (#93 후속).
- `.editorconfig` 신규 — Go (tab), YAML / JSON / Markdown
  (2-space, trim 정책), Makefile (tab 필수), shell script 까지 일괄
  적용.

### Changed

- 모든 GitHub Actions reference 8 개 workflow 전체를 commit SHA +
  꼬리에 버전 주석으로 pin. OpenSSF Scorecard 의
  `Pinned-Dependencies` 검사 통과 (#95).
- `setup-go check-latest: true` + `go.mod` 의
  `toolchain go1.26.3` 채택 — stdlib CVE 수정 (1.26.2 / 1.26.3
  합산 16 건) 이 모든 CI run 에 자동 반영 (#92).
- `security-scan.yml` 이 이제 *모든 PR* 에서 동작. 이전엔 diff 가
  go.mod / Dockerfile 을 건드릴 때만 돌았음 — 보안 스캔이 다른 영역
  변경으로 건너뛰는 일을 차단 (#92).
- `main` 의 branch protection 강화. 필수 status check 7 종
  (golangci-lint, unit + envtest, build, govulncheck, trivy-fs,
  trivy-image, Review dependencies) + strict mode + linear history
  + conversation resolution + force-push / delete 차단 +
  enforce-admins on.
- 저장소 security toggle 일괄 활성화: Dependency graph, Automated
  security fixes, Secret scanning, Secret-scanning push protection.
- README Go 배지 1.25+ → 1.26+ (#90).
- CONTRIBUTING.md Go 사전요건을 1.26 으로 상향 —
  `TestGoVersionDockerfileVsGoMod` 회귀 가드 통과를 유지하기 위함
  (#84 후속).
- `release.yml` 의 `sbom` job 이 이제 `contents: read` +
  `packages: read` 까지 함께 선언. syft 가 GHCR manifest 접근을
  유지하면서도 OIDC token 을 잃지 않음 (#92 리뷰 후속).
- `cosign --certificate-identity-regexp` 를 정확히 `release.yml`
  workflow 한정으로 좁힘 — 이 저장소의 임의 workflow 가 아닌 (#92
  리뷰 후속).

### Security

- 서명된 모든 image / chart / SBOM 에 Sigstore Rekor 투명성 로그
  엔트리가 v1.0.13 부터 기록된다.
- 컨테이너 이미지에 대해 `slsa-framework/slsa-github-generator`
  로 SLSA-3 provenance attestation 을 발급 (v1.0.13 부터).
- SECURITY.md 가 artifact 검증에 필요한 정확한 `cosign verify` /
  `slsa-verifier verify-image` 명령 + 인증서 identity regex 를
  명문화.

### Dependencies

- `actions/setup-go check-latest: true` — CI Go runtime 이
  stdlib CVE 수정을 자동 수신.
- dependabot 7 건 main 머지:
  - Docker base: `golang 1.26.3`, `distroless/static@e3f9456`
    (#80, #81).
  - Go modules: k8s 0.36 + controller-runtime 0.24 + utils + + otel 그룹 + ginkgo 2.28.3 + gomega
    1.40.0 (#84–#88).

## [1.0.12] - 2026-05-12

### Changed

- 기존 `v1.0.11` 태그가 미완성 release commit 을 가리키던 것을 정리
  하기 위한 Artifact Hub 클린 재배포.
- 게시된 chart 메타데이터를 새로고침 — Alpine 3.23 기반 Valkey
  runtime image + operator image tag `1.0.12` 노출.
- chart README 와 Artifact Hub 공개 메타데이터는 영문 표기를 유지.

### Fixed

- 최종 release commit 직전에 유입된 금지 대상 GitHub Actions
  workflow 파일을 제거.

## [1.0.11] - 2026-05-12

### Changed

- Artifact Hub chart 메타데이터를 새로고침 — 게시된 패키지가
  Alpine 3.23 기반 Valkey runtime image 를 노출하도록 수정.
- 현재 release 면을 위해 chart / app 버전 `1.0.11` 게시.
- Artifact Hub trust-badge 문서는 영문 표기를 유지.

### Fixed

- 여전히 `docker.io/valkey/valkey:9.0.4` 를 광고하던 stale 한 Helm
  repository 패키지를 정정.

## [1.0.10] - 2026-05-10

### Added

- OperatorHub.io bundle 골격 + ADR-0037 (PR-B9 첫 컷, #21).
- `alm-examples` 인라인 JSON 5 종 추가 (PR-B9.2, #22).
- CITATION.cff 추가 (OSS 메타데이터, #20).

### Changed

- Chart 버전 v1.0.10 + amd64-only build (CLAUDE.md §2 정합, #27).
- `v1alpha2 zz_generated.deepcopy.go` 재생성 (controller-gen
  동기, cfd0398).
- bundle: `generate-kustomize-manifests` 단계 제거 (PR-B9.4,
  mongodb ADR-0023 정합, #23).

### Fixed

- `ValkeyCluster` post-init self-heal — INC-0001 영구 fix
  (ADR-0039, #25).
- v1alpha1 `storageversion` 마커 추가 + controller-gen 재생성
  (PR-A2.2.5, #19).
- `ReadOnlyRootFilesystem=true` 활성 — modern security baseline
  의 마지막 layer (3aa5480).

### Docs

- INC-0001 ValkeyCluster bootstrap skip — 운영 cluster 19h fail
  복구 회고 (#24).
- INC-0001 cluster_state=fail 회복 runbook + ADR-0039 self-heal
  명시 (AI-0004, #26).
- ADR-0026 부분 회복 진행 명시 (PR-A2.2.* 누적, #18).
- HANDOFF PR-A2.2.5 머지 결과 + 다음 진입점 갱신 (1818031).

## [1.0.9] - 2026-05-10

### Added

- v1alpha2 type 정의 모듈 + `AuthSpec.Required` toggle (PR-A2.1,
  #6).
- v1alpha2 Hub marker 5 type 추가 (PR-A2.2.1, #15).
- v1alpha1 의 `ConvertTo` / `ConvertFrom` 5 type 본문 구현
  (PR-A2.2.2, #16).
- `cmd`: v1alpha2 SchemeBuilder 등록 (PR-A2.2.3.a, #17).
- Valkey Custom Modules type 추가 (v1alpha2, PR-C6.1, ADR-0032,
  #14).
- `AuthSpec.RotationPolicy` enum 추가 (v1alpha2, PR-B7.1,
  ADR-0031, #12).
- `PodSecurity` Restricted optional toggle (v1alpha2, PR-A3.2,
  ADR-0036, #10).
- `NetworkPolicy.AutoCreate` optional toggle (v1alpha2,
  PR-A3.1, ADR-0035, #9).
- RFC-0018 `pkg/finalizer` 마이그레이션 (controller, PR-A6 첫 컷,
  ADR-0038, #8).
- release: cosign 서명 + SLSA L2 in-toto attestation + ADR-0033
  (PR-A4, #5).

### Changed

- — RFC-0018 의 `SetAvailable` +
  `SetReadyFalse` 사용 가능해짐 (#7).

### Docs

- ADR-0018 정식 작성 — Cluster Auto-Resharding (PR-B8.1, #13).
- Sentinel migration runbook 신설 — PR-C7, ADR-0017 거절 보강
  (#11).

## [1.0.8] - 2026-05-09

### Fixed

- `monitoring.exporter.resources` 가 metrics sidecar 까지 reconcile
  되지 않던 운영 통합 결함 fix (1eb6faf):
  - `STSParams.ExporterResources corev1.ResourceRequirements`
    신규 필드 (internal/resources/statefulset.go).
  - `BuildStatefulSet` 의 metrics container 에 `p.ExporterResources`
    적용.
  - Valkey + ValkeyCluster controller 가
    `exporterResources(spec.Monitoring)` helper 로 전달.
  - 빈 ResourceRequirements (default) → K8s Burstable QoS, 이전
    동작과 동등 호환.

### Changed

- Chart 버전 1.0.7 로 bump (8408005).

## [1.0.7] - 2026-05-09

### Changed

- audit (4-repo cross-cut, 2026-05-09): RFC-0017 채택 —
  `.golangci.yml` + `.custom-gcl.yml` 신규 (postgres 표준 cp +
  depguard 정리), Makefile `validate` 타겟 추가 (kustomize + helm
  lint + helm template). ADR-0030 등재. 본 repo 의 `.lefthook.yml`
  은 RFC-0017 §3.1 표준 원본으로 승격 (변경 없음, 0aea740).
- (4833f13).
- `.codecov.yml` 신규 — 4-repo target 70% 절대 floor 통일
  (d381587).

### Fixed

- `.golangci.yml` depguard 일시 비활성 (golangci-lint v2.8 schema
  가 빈 deny list 거부) — valkey internal boundary 도입 이후 ADR 와
  함께 재활성 예정. 17 linter 활성, logcheck plugin 포함
  (9dae535).
- lint: valkey 잔여 37 건 lint 0 issue 달성 (goconst 17 + unparam
  17 + gocyclo 3, 8ba60a7).
- lint: lll / prealloc / revive 5 건 fix (안전 cleanup, 5d16c94).
- lint: modernize 20 + copyloopvar 2 건 자동 fix (slices.Contains
  등, 8820460).

### Docs

- CHANGELOG entry + deps log (audit 마무리, bd667ad).

## [1.0.6] - 2026-05-08

### Added

- TLS `clientAuth` 필드 신설 — required / optional / disabled mTLS
  toggle (0c804c9).
- renovate: auto-update PR 진입점 (Go modules + image tag,
  ba3c9af).

## [1.0.5] - 2026-05-08

### Fixed

- Artifact Hub 가 `1.0.4` chart icon
  `https://valkey.io/img/Valkey-Logo-RGB-Color.svg` 를 가져오다 404
  로 tracking warning 을 내던 문제를 수정. 현재 Valkey 사이트에서
  200 응답하는 `https://valkey.io/img/valkey-horizontal.svg` 로
  교체.

## [1.0.4] - 2026-05-08

### Added

- Service builder: TLS 활성 시 client-tls (6380) port expose 한다.
  BuildClientService + BuildHeadlessService 의 tlsEnabled 인자
  추가. 외부 client 가 `rediss://` scheme 으로 connect 가능
  (`tls-auth-clients=yes` 인 경우 client cert 는 별도 발급 필요 —
  본 patch 는 server-side TLS 외부 노출 인프라만 다룬다).

## [1.0.1] - 2026-05-07

### Fixed

- `ValkeyBackup` 이 `ValkeyCluster` 대상에서 첫 번째 pod 의
  `dump.rdb` 만 저장하던 문제 수정. 이제 shard 별 primary pod 기준
  으로 `shard-N/dump.rdb` 구조를 만들어 cluster restore 기본 shard
  layout 과 직접 호환된다.
- `ValkeyRestore` 가 `ValkeyCluster` 대상 복구 중 pause / unpause
  annotation 을 `Valkey` CR 에만 적용하던 문제 수정.
- multi-pod restore 의 기존 source PVC 검증에서 `ReadOnlyMany`
  뿐 아니라 read-only mount 가능한 `ReadWriteMany` PVC 도 허용.

## [1.0.0] - 2026-05-07

### Added

- 첫 stable 릴리스. Valkey `9.0.4` 기본값, `8.0.9` / `8.1.7`
  milestone 호환성, `ValkeyCluster` sharded HA, 자동 failover,
  `ValkeyBackup`, `ValkeyRestore`, `ValkeyBackupTarget`, restricted
  PodSecurity 기본값, `linux/amd64` / `linux/arm64` multi-arch
  operator image 를 포함한다.

## [0.1.0-alpha.5] - 2026-05-07

### Fixed

- **Runtime P0 — restricted PodSecurity 네임스페이스에서 Valkey
  Pod 생성 실패** (`internal/resources/statefulset.go`):
  Valkey StatefulSet 컨테이너가 `allowPrivilegeEscalation=false`,
  `capabilities.drop=[ALL]`, `seccompProfile.type=RuntimeDefault`
  기본값을 갖지 않아 `data-staging` 네임스페이스에서 Pod 생성이
  금지됐다. 기본 Valkey 컨테이너와 metrics sidecar 에 restricted
  SecurityContext 를 주입.

## [0.1.0-alpha.4] - 2026-05-07

### Fixed

- **Release P0 — operator image build metadata 누락** (`Makefile`):
  `make release` 가 Dockerfile 의 `VERSION` / `COMMIT` /
  `BUILD_DATE` build args 를 전달하지 않아 실제 배포 이미지의
  `/manager --version` 과 `valkey_cluster_build_info` 가
  `dev/none/unknown` 으로 노출됐다. release target 에서 tag, git
  commit, UTC build date 를 주입하도록 수정.
- **Release P0 — chart affinity 와 image platform 불일치**
  (`Makefile`): chart 기본 affinity 는 `linux/amd64` 와
  `linux/arm64` 노드를 허용하지만 release image 는 `linux/amd64`
  만 push 했다. release build 를 `linux/amd64,linux/arm64`
  multi-arch 로 변경.

### Added

- release target 이 build metadata 와 multi-arch platform 을 강제
  하도록 회귀 테스트 추가.

## [0.1.0-alpha.3] - 2026-05-07

### Added

- Valkey latest default 정렬: API default, CRD default, Helm
  values, ArtifactHub examples / images, samples, GitOps workload
  CR 을 `9.0.4` 로 갱신.
- `SupportedValkeyVersions` whitelist 를 `8.0.9`, `8.1.6`, `8.1.7`,
  `9.0.4` 로 명시 — 최신 + 8.0 / 8.1 milestone patch 호환 기준을
  문서화.
- ValkeyCluster 9.0.4 sharded 3x1 Kind smoke: 6 pod Ready,
  `cluster_state=ok`, 16384 slots, SET / GET 검증.

### Fixed

- Redis 8.2.x RDB 를 Valkey 9.0.4 로 직접 restore 할 때 RDB
  format 불일치로 CrashLoopBackOff 되는 경로를
  `ValkeyRestore.status.phase=Failed` 로 fail-fast 처리.

## [0.1.0-alpha.2] - 2026-05-07

ADR-0057 Phase A1 (운영 클러스터 사전 배포) 진행 중 발견된 chart
RBAC 결함 fix.

### Fixed
- **chart RBAC P0 — `features.{cluster,backup}.enabled=false` 시
  informer startup 실패**
  (`charts/valkey-operator/templates/clusterrole.yaml`):
  이전 chart 가 `features.cluster.enabled` /
  `features.backup.enabled` 조건부로 `valkeyclusters` /
  `valkeybackups` / `valkeybackuptargets` / `valkeyrestores` RBAC
  부여 — 그러나 operator manager (`cmd/main.go`) 는 *항상* 모든
  controller 등록 → flag=false 시 informer 가 `forbidden` 으로
  startup 실패. RBAC 와 코드 mismatch 가 production-grade 차단
  요인. RBAC 를 *항상 모든 CRD 권한 부여* 로 단순화, feature flag
  는 controller 코드 측에서만 처리.

### Verified (운영 클러스터 Phase A1 + A2)
- valkey-operator pod 1/1 Running, Certificate / Issuer /
  ValidatingWebhookConfiguration Ready.
- Valkey CR `valkey-test` (Standalone, valkey 8.1.6, 1Gi ceph-rbd)
  1/1 Running.
- SET / GET smoke:
  `SET phase-a2-smoke "OK-2026-05-07"` → `OK`, `GET` → 정상
  round-trip.
- `INFO server`: valkey_version=8.1.6, tcp_port=6379.

### Refs
- ADR-0057 (인프라 부트스트랩 43fd542): self-hosted
  valkey-operator 채택 로드맵.
  HANDOFF.md (2026-05-07).

### Added (GitOps deploy 정합)

- `deploy/overlays/prod/` GitOps 진입점 — `config/{crd,rbac,manager}`
  를 prod ns 로 정렬 + 자동 생성 Namespace 제거. ArgoCD 단방향
  동기를 전제로 한다.
- `deploy/valkey-cluster.yaml` — production ValkeyCluster sample
  (db ns, shards=3, replicasPerShard=1, ceph-block,
  auth.enabled=true).
- `deploy/README.md` — 운영 런북.
- ADR-0029 — GitOps deploy 오버레이 도입 (3-repo 정합).

### Added (cycles 20-90 — Quality 시스템 + production-grade UX)

**Quality 시스템 (39 SSOT 게이트)**:
- ADR governance (4 게이트): file / INDEX / Status / Superseded /
  Nygard 3-section.
- Alert rules (4): schema / fields / metric / runbook anchor 동기.
- RBAC (2 양방향): `kubebuilder:rbac` ↔ `role.yaml`.
- Sample CR (3): strict unmarshal + dir-mapping + metadata.
- ClusterRef.Kind (2 — 3-way): enum ↔ switch case.
- LICENSE + Chart annotation (2).
- Chart artifacts (6): images / CRDExamples / CRD sync / values /
  NOTES / README YAML.
- Markdown links + anchor (2).
- Webhook + Reconciler 등록 (2).
- `dist/install.yaml` (2): 구조 + `OPERATOR_IMAGE` env.
- Release-checklist self-sync (1, 양방향 cycle 60).
- Kustomize ↔ chart sync family (3): resources / probes /
  securityContext.
- Cross-feature interaction family (3): NP + webhook / tracing /
  backup.
- `features.*` RBAC + reconciler 동기 (1).
- value ↔ template binding (1).
- chart args ↔ operator flag (1).

**자동화 (실수 발생 자체 차단)**:
- `make manifests` 가 chart CRD 자동 sync.
- pre-push lefthook 6-hook (full-lint + gitleaks + go-mod-tidy +
  helm-lint + helm-template + unit-test).
- `make sbom` (syft SPDX) + trivy post-scan 이 release pipeline 에
  자동 첨부.

**Production-grade UX**:
- ldflags chain (cycles 53-57): `cmd/main.go` → Dockerfile →
  `docker-build` → `docker-buildx` → `release.sh` → Prometheus
  `build_info` gauge.
- chart features 5 종 (cycles 65 / 72 / 73 / 74 / 82): tracing +
  NetworkPolicy + webhook + watch.namespaces + autoscaling 까지
  정직 표시.
- 6-layer documentation: README + chart README + NOTES.txt +
  CONTRIBUTING + release-checklist + HANDOFF (모든 사용자 역할별
  entry point).
- runbook §7.1 — 환경 변수 진단 가이드.
- 3-layer DX: lefthook auto + `make ssot-check` (1.4s) +
  `make gate` (30s).

**구현된 기능 (cycles 72-74 — chart 4 종 미사용 value 중 3 종 해결)**:
- `charts/valkey-operator/templates/networkpolicy.yaml` —
  operator pod default-deny.
- `charts/valkey-operator/templates/webhook.yaml` — cert-manager
  의존 admission webhook.
- `WATCH_NAMESPACES` env — namespace-scoped watch
  (`cache.DefaultNamespaces`).

**구현된 기능 (cycles 99-106 — kubebuilder boilerplate completion
+ Helm parity)**:
- cycle 100 — runbook §7.0 production TLS 강화 가이드
  (`insecureSkipVerify` → cert-manager).
- cycle 101 — `config/manager` + chart values 의 nodeAffinity
  (amd64 + arm64 + linux) — mixed-arch ImagePullBackOff 차단.
- cycle 102 — `config/default/kustomization.yaml` 의
  `- ../prometheus` 활성 — kustomize 사용자도 ServiceMonitor +
  PrometheusRule 자동 설치.
- cycle 103 — `charts/.../prometheusrule.yaml` — Helm 사용자의
  10 alerts silent loss 차단.
- cycle 104 — `charts/.../metrics-auth-rbac.yaml` — secure
  metrics 의 Prometheus 401 silent fail 차단.
- cycle 106 — `charts/.../deployment.yaml` 의 webhook serving
  config (`--webhook-cert-path` + 9443 + cert mount) — webhook
  활성화 시 operator 가 9443 으로 정확히 listen.

**production gap 발견 · 수정 (27 건)** + **내부 부채 cleanup (3
건)** + **5 종 hot-path benchmark** + **8 결함 family 의 progressive
completion**.

### Added (iter 7+ — 부트스트랩 · 검증 사이클)
- README quickstart (kind 기반): 5 단계 부트스트랩 + 데이터 plane
  smoke + 운영 시나리오 매트릭스. [iter 6]
- ADR-0011: Required 필드 (omitempty 부재) 의 mutating webhook
  defaulting 패턴. [iter 4]
- ADR-0012: CLUSTER MEET hostname 미지원 → DNS 해석 후 IP 사용.
  [iter 4]
- ADR-0013: `Auth.Enabled` 강제 true (옵션 A 채택). [iter 5]
- `internal/valkey/cluster.go::resolveAddrIP`: hostname → IP
  정규화 (IPv4 prefer).
- `internal/webhook/v1alpha1/valkey_webhook.go`: Version +
  `Auth.Enabled` 정규화.
- `internal/webhook/v1alpha1/valkeycluster_webhook.go`: Shards /
  ReplicasPerShard / Version / Auth defaulting.
- `api/v1alpha1/common_types.go`: `DefaultValkeyVersion` /
  `DefaultValkeyImage` 상수.
- `internal/controller/valkeycluster_controller.go`: pods RBAC
  추가 (status reconciliation).
- `config/samples/cache_v1alpha1_valkeybackup.yaml`: 의미 있는
  ClusterRef 채움.
- `.dockerignore`: `*.tmpl`, `*.lua`, `*.sh` 패턴 — embed 자산
  보존.
- lefthook 활성화 (pre-commit + pre-push + commit-msg) +
  Conventional Commits 패턴.

### Fixed (iter 7+)
- ValkeyBackup controller 테스트 fixture 의 ClusterRef 누락
  (webhook validation 통과 못함).
- ValkeyCluster bootstrap 무한 retry: CLUSTER MEET 가 hostname
  거부 → DNS 해석.
- defaulting webhook 이 required 필드 (Version / Shards /
  ReplicasPerShard) 를 채우지 않아 무한 reconcile 루프.
- pods RBAC 누락으로 ValkeyCluster status 갱신 불가.
- lefthook commit-msg 가 `$1` 대신 `{1}` 사용.
- lefthook golangci-lint cross-directory staged files 오류.

### Verified (iter 7+ 실측)
- e2e suite: 5/5 PASS (manager 시작, metrics endpoint,
  cert-manager, mutating / validating webhook CA injection).
- integration test: 14 케이스 PASS (실 valkey:8 컨테이너 + 6 노드
  클러스터 부트스트랩).
- unit test: 4 패키지 PASS
  (`internal/{controller,resources,valkey,webhook}`).
- 회복성: primary pod force kill → STS 재생성 → operator 재
  promote → 데이터 보존 (canary `preserved`).
- scale up / down: 3 → 5 → 2, `master_link_status:up`, 데이터
  보존.
- 클러스터 모드: 3 shards × 2 instances, `cluster_state:ok`,
  `slots:16384/16384` OK.
- TLS + mTLS 클러스터 (cert-manager + selfsigned ClusterIssuer):
  Phase=Running, slots=16384/16384 OK, 데이터 plane SET / GET 성공
  (cluster mode `-c`, 다중 shard 분산).
- NetworkPolicy 리소스 정합성: deny-by-default + selfPeer ingress
  (6379) + ownerReferences (Standalone). cluster mode 시 16379
  추가. 강제 동작 검증은 Calico / Cilium CNI 필요 (kindnet 미지원).
- operator metrics endpoint (HTTPS:8443, ServiceAccount 토큰
  인증): `controller_runtime_*` 메트릭 정상 노출. 커스텀
  `valkey_cluster_*` 메트릭은 ValkeyCluster reconcile 시 emit.

### Added (iter 1-6 — 이전 사이클)
- ValkeyCluster Reconcile 14 단계 구현 (cluster mode CRD
  bootstrap → CLUSTER MEET / ADDSLOTS / REPLICATE → status
  polling). [iter 1]
- `internal/valkey/cluster.go`: `CreateCluster` 단계별 멱등 분리
  (`ensureMeet` / `ensureSlots` / `ensureReplicas`). partial-state
  회복 가능. [iter 2]
- `internal/valkey/nodes.go`: `CLUSTER NODES` 응답 파서
  (`NodeView`, `SlotRange`). [iter 2]
- 통합 테스트 (`//go:build integration`): 실 valkey:8 컨테이너
  6 노드 클러스터 — 4 시나리오 PASS. [iter 2-4]
- Finalizer graceful cleanup: `gracefulClusterTeardown`
  (best-effort `CLUSTER FORGET`, 30s timeout). [iter 2]
- Prometheus metrics: 7 시계열 (`state_ok`, `assigned_slots`,
  `shards`, `ready_replicas`, `reconcile_total`,
  `reconcile_errors_total`, `phase`). [iter 3]
- `ScalePolicy.Deliberate` 가드: 미동의 시 `Status.PendingScale`
  기록 + STS replicas 보존. [iter 3]
- ServiceMonitor (`monitoring.coreos.com/v1` unstructured) 자동
  생성 + metrics Service 분리. [iter 3]
- AutoFailover ConfigMap 디렉티브 통합
  (`cluster-replica-no-failover yes`). [iter 3]
- `make integration-test` Makefile target. [iter 2]
- `buildShardStatusFromNodes`: CLUSTER NODES 기반 ShardStatus
  (failover 정확 반영, ADR-0007). [iter 4]
- TLS RootCAs 로드: `Spec.TLS.CustomCert.SecretName.ca.crt` →
  x509 CertPool (ADR-0008). [iter 4]
- Validating + Mutating Webhook (양 CRD): 8 조합 검증 +
  immutable 가드 (Mode, Storage, TLS toggle). [iter 5]
- ShardStatus pod ordinal 매핑: `buildPodAddrMap` (K8s Pod list →
  "vk-N"). [iter 5]
- cert-manager Certificate 자동 생성:
  `Spec.TLS.CertManager.IssuerRef` 명시 시 Certificate CR 자동
  + secretName 자동 발견 (ADR-0010). [iter 6]
- Version upgrade 감지: `decidePhase` 가
  `Spec.Version != Status.Version` 감지 시 Phase=Upgrading.
  [iter 6]
- ADR 0001-0010 (10 건, 2 건 supersede): defaulting → webhook
  (0001 → 0009), ShardStatus spec → NODES (0004 → 0007), TLS
  단계적 통합 (0003 → 0008 → 0010). [iter 1-6]
- lefthook 설정 (`.lefthook.yml`). [iter 3]

### Changed
- `valkey/replication.go`: `SlaveOf` → `ReplicaOf` (Redis 5.0+
  deprecated API). 모던 Valkey `role:replica` 인식 추가. [iter 1]
- `valkeycluster_controller.go`: `pollClusterState` 가 모든 노드
  fallback (`queryAnyNode`) — pod-0 SPOF 제거. [iter 1]
- `dialPod` 가 `Spec.TLS.Enabled` 통과 (이전엔 무시됨). [iter 1]
- SetupWithManager: `Owns(PDB, NetworkPolicy)` 추가 — drift
  감지. [iter 1]

### Fixed
- 컴파일 오류: `&appsv1StatefulSet{}.s` →
  `(&appsv1StatefulSet{}).Inner()` (Go struct literal
  addressability). [iter 1]
- `ensureReplicas` 에 gossip 수렴 retry — `replicateWithRetry`
  (10 회 backoff, "Unknown node" 흡수). [iter 2]
- `parseReplicationInfo` 가 modern Valkey `role:replica` 인식 —
  이전엔 매 reconcile 마다 ReplicaOf 재호출 (멱등성 결함). [iter 1]

### Documentation
- ADR 인덱스 (`docs/kb/adr/INDEX.md`).
- 본 CHANGELOG. [iter 3, iter 6 갱신]

### Test Coverage Snapshot (iter 6 끝)
- `internal/controller`: 50.5%.
- `internal/resources`: 33%+.
- `internal/valkey`: 33.7%.
- **`internal/webhook/v1alpha1`: 80.7%** (신규 패키지).
- 단위 테스트: 60+ 건.
- 통합 테스트: 4 시나리오 (실 Valkey 6 노드).
