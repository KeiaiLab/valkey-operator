<p align="center">
  <b>English</b> |
  <a href="i18n/ko/ROADMAP.md">한국어</a> |
  <a href="i18n/ja/ROADMAP.md">日本語</a> |
  <a href="i18n/zh/ROADMAP.md">中文</a>
</p>

# ROADMAP — valkey-operator

> 한국어 버전: [ROADMAP.ko.md](i18n/ko/ROADMAP.md)

This roadmap tracks progress as a **verifiable feature checklist**,
not a calendar of deadlines. Time-based deadlines are intentionally
avoided per the project's
[`standards/workflow.md`](https://github.com/keiailab/valkey-operator/blob/main/docs/kb/adr/INDEX.md)
("no time-based roadmap" rule); progress is measured by feature
completion.

## Checkbox semantics

| Marker | Meaning |
|---|---|
| `[x]` | Code and tests both exist; regression covered by an e2e or unit test |
| `[~]` | Partial — fields are defined but a helper is not yet wired in, or a verification item is still open |
| `[ ]` | Not started (design or PoC) |

The *Verify* line on each sub-task cites the exact command or e2e
file used to confirm the checkbox.

## Current (1.x line — Active)

### Stability and maturity

- [x] **PodSecurity restricted compliance**
  - [x] Restricted SecurityContext helpers
    (`buildRestrictedContainerSecurityContext` etc.) applied across the
    resource builders — `internal/resources/statefulset.go`,
    `backup_job.go`, `download_job.go`, `upload_job.go`, `restore.go`
  - [x] Regression guard for restricted PSA in resource builders
  - [x] Controller and webhook podSpec conversion paths fully guarded
    — `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validatePodSecurityRestricted` (6 items —
    runAsNonRoot/runAsUser/privileged/allowPrivilegeEscalation, 9 unit
    tests, #78)
  - Verify: label the namespace
    `pod-security.kubernetes.io/enforce=restricted`, then pod Ready

- [x] **Cluster mode (5 shards × replica=2)**
  - [x] Ordinal-based restore Init Container —
    `internal/controller/valkeycluster_controller.go`
  - [x] Automatic 16384-slot distribution
  - [x] Automatic failover (chaos-tested) —
    `test/e2e/cluster_recovery_test.go`, `failover.go`
  - [x] Primary kill → master re-election —
    `test/e2e/failover_test.go`
  - Verify: `test/e2e/cluster_recovery_test.go` PASS, all 16384 slots
    intact, data preserved

- [x] **HPA / PDB / NetworkPolicy automation (opt-in)**
  - [x] HPA (ADR-0027, Replication mode) — chart
    `autoscaling.enabled`
  - [x] PDB default — `internal/controller/pdb_default.go`
  - [x] NetworkPolicy default-deny + explicit allow — chart
    `networkPolicy.enabled`
  - Verify: `pdb_default_test.go` PASS,
    `kubectl get pdb/networkpolicy`

- [x] **Backup / Restore — S3 + PVC ROX + VolumeSnapshot**
  - [x] S3 (minio-go) backup —
    `internal/controller/valkeybackup_controller.go`
  - [x] PVC ROX multi-mount restore —
    `internal/controller/valkeyrestore_controller.go`
  - [x] VolumeSnapshot lifecycle —
    `internal/controller/backup_volumesnapshot.go`
  - [x] Multipod snapshot replication restore —
    `multipod_volumesnapshot_replication_test.go`
  - [x] `ValkeyBackupTarget` CRD (external backup destination) —
    `api/v1alpha2/valkeybackuptarget_types.go`
  - Verify: `test/e2e/backup_restore_test.go` PASS

- [x] **chart RBAC conditional fix** (2026-05-07, commit `06237be`)
  - [x] Prevent informer startup failure when
    `features.{cluster,backup}.enabled=false`
  - Verify: chart install with
    `--set features.cluster.enabled=false` and the operator pod
    becomes Ready

- [x] **Version-upgrade reconcile fix**
  - [x] Fresh-scenario path correct (iteration 7 diagnosis)
  - [x] Restore → patch chain regression guard (iteration 18 V2) —
    `test/e2e/backup_restore_test.go` "Restored instance 8.1.6 → 9.0.4
    version patch chain (V2)"
  - [x] RDB v80 compatibility (`foo=bar1` retained)
  - Verify: the e2e above PASS = the two narrow blockers are
    permanently resolved

- [x] **Valkey 9.x support (default 9.0.4)**
  - [x] Chart `image.tag: 9.0.4` default —
    `charts/valkey-operator/values.yaml`
  - [x] RDB-format v80 compatibility verified
  - Verify: boot a new instance and run
    `valkey-cli INFO server | grep redis_version`

- [x] **API version evolution**
  - [x] v1alpha2 active — `api/v1alpha2/`
  - [~] v1alpha1 → v1alpha2 conversion webhook —
    `api/v1alpha2/conversion.go` (conversion funcs + Hub markers
    shipped; webhook serving path not yet wired — no
    `spec.conversion.strategy: Webhook` in config/crd, no conversion
    clientConfig in the chart webhook, not registered in cmd/main.go;
    see `api/v1alpha2/doc.go`)
  - [x] 5 CRDs (Valkey, ValkeyCluster, ValkeyBackup, ValkeyRestore,
    ValkeyBackupTarget)
  - Verify: `kubectl apply -f <v1alpha1.yaml>` and check it is stored
    as a v1alpha2 object

- [x] **Online PVC resize** — `commonspvc.ExpandDataPVCs`
  (operator-commons `pkg/pvc`) invoked from
  `internal/controller/valkey_controller.go` and
  `internal/controller/valkeycluster_controller.go` (ADR-0049)

- [x] **Webhook admission validation (4 validating webhooks +
  conversion webhook)** — `internal/webhook/v1alpha1/`
  (validating webhooks for Valkey, ValkeyCluster, ValkeyBackupTarget,
  ValkeyRestore; ValkeyBackup has no validating webhook — the 5th CRD
  is covered via the conversion path)
  - [x] RBD storageClass baseline validation —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateStorageClassName` (DNS-1123 subdomain)
  - [x] Topology-spread consistency validation —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateTopologySpread` (MaxSkew / TopologyKey /
    WhenUnsatisfiable / duplicate key, #77)
  - [x] replicaCount lower-bound check wired into the webhook —
    `internal/webhook/v1alpha1/valkey_webhook.go` (mode=Replication →
    replicas ≥ 2; mode=Standalone → replicas = 1; autoscaling.minReplicas
    ≥ 2) + `valkeycluster_webhook.go` (autoFailover → replicasPerShard ≥ 1)
  - Verify: `go test ./internal/webhook/v1alpha1/` PASS
    (`TestValkey_Validate_Replication_replicas_min_2`,
    `TestValkey_Autoscaling_min_below_2_rejected`,
    `TestValkeyClusterValidate_AutoFailover_requires_replicas`)

- [x] **Encryption audit (TLS / encryption surveillance)** —
  `internal/controller/encryption_audit.go`,
  `encryption_enforce_test.go`

- [~] **AutoUpdate — operator-managed 자동 버전 업데이트** — channel(patch/minor)
  제약 내 안전 버전을 maintenance window 시간대에 자동 적용. major 상승은 자동 금지.
  - [x] 순수 결정 로직 (semver 채널 비교 + 자정 넘김 window + 통합) —
    `internal/autoupdate/autoupdate.go`, `autoupdate_test.go`
  - [x] `AutoUpdateSpec` API (v1alpha1 reconcile hub + v1alpha2) + 헬퍼 —
    `api/v1alpha1/autoupdate.go`, `api/v1alpha2/autoupdate.go`
  - [x] reconcile wiring (Valkey) — effective version in-memory 주입 →
    STS 이미지 + Status.Version 자동 전파 —
    `internal/controller/autoupdate_integration.go`
  - [x] ValkeyCluster 통합 — `applyAutoUpdateCluster`,
    `internal/controller/valkeycluster_controller.go` (샤드 전체 동일 버전)
  - [x] 운영자 수동 major 변경 차단 webhook (Valkey + ValkeyCluster, v1.2.0) —
    `autoupdate.IsMajorUpgrade` → `validateValkeyImmutable`/`validateClusterImmutable`
  - [ ] 버전 카탈로그 레지스트리 폴링 (정적 catalog → ghcr 태그 폴링, #263)
  - Verify: `go test ./internal/autoupdate/ ./internal/webhook/v1alpha1/ -run 'AutoUpdate|Major'` PASS

- [~] **Valkey official module presets (Redis Stack equivalent)** —
  turnkey loading of the BSD-licensed `valkey-search` / `valkey-json` /
  `valkey-bloom` modules via `ValkeySpec.Modules`. External Redis Stack
  modules (RediSearch / RedisJSON, RSALv2 / SSPL) are a deliberate
  non-goal — license-incompatible with Valkey's BSD-3 (ADR-0032)
  - [x] `ModuleSpec` type + `ValkeySpec.Modules []ModuleSpec` field
    (PR-C6.1) — `api/v1alpha2/valkey_types.go`
  - [x] Controller wiring — init-container `.so` mount (emptyDir) +
    `--loadmodule` in the StatefulSet podSpec (PR-C6.2, live since 1.1.0) —
    `internal/resources/module_init.go` `BuildModuleInitContainers`,
    `internal/controller/valkey_controller.go` `Modules: v.Spec.Modules`
  - [x] Official-preset allow-list validation (외부 Redis Stack 거부, v1.2.0 unit test) —
    `internal/webhook/v1alpha1/valkey_webhook.go` `validateModules`.
    ⚠️ 클러스터 admission 실작동은 webhook 활성화 필요 (현재 `ENABLE_WEBHOOKS=false` —
    chart hook 순서 chicken-egg, 별도 이슈)
  - [x] Chart module-list exposure (v1.2.0) — `charts/valkey-operator/values.yaml`
    module preset 문서 + `config/samples/cache_v1alpha1_valkey.yaml` modules 예시
  - [ ] e2e — `valkey-search` `FT.SEARCH` round-trip — `test/e2e`
  - Verify: apply a Valkey CR with a `valkey-search` preset under
    `modules`, then `valkey-cli MODULE LIST` shows the module loaded

### Operations and delivery

- [x] Helm chart published — `keiailab.github.io/valkey-operator`
- [x] 3-repo (mongodb / postgres / valkey) governance asset
  alignment (CODE_OF_CONDUCT / GOVERNANCE / MAINTAINERS / ROADMAP)
- [x] **GitHub Actions release pipeline restored** (ADR-0045) —
  scoped deviation from RFC-0002 for externally-facing OSS repos;
  see [ADR-0045](kb/adr/0045-restore-github-actions-for-oss-ci.md)
- [x] **SLSA-3 provenance + cosign keyless signing** for the image,
  Helm chart, and SBOM (ADR-0046) — verification commands in
  [SECURITY.md](../.github/SECURITY.md). Active from v1.0.13.
- [x] **Production cluster adoption** <!-- live-verified: 2026-05-27 -->
  - [x] CRD-install manifest — shipped via the operator Helm chart
  - [x] ArgoCD application registration — operator + workload apps
    Synced/Healthy
  - [x] Production workloads migrated to operator-managed CRs —
    4 live instances spanning cluster (3-shard, 16384 slots ok) and
    replication modes, no longer plain StatefulSets
  - Verify: ArgoCD Synced/Healthy and
    `kubectl get valkey/valkeycluster -A`
- [x] **Migration runbook** — plain StatefulSet → ValkeyCluster CR (PR #136)
  - [x] Document the zero-downtime procedure — `docs/migration/zero-downtime.md` (PR #136)
  - [x] Secondary-promote-based cutover — `docs/migration/secondary-promote.md` (PR #136)
  - [x] Rollback procedure — `docs/migration/rollback.md` (PR #136)
  - Verify: staging dry-run with RTO / RPO measurements recorded
- [x] **release-smoke-test.sh** — follow established pattern (PR #136)
  - [x] Five stages: image / SBOM / trivy / chart index / smoke — `scripts/release-smoke-test.sh` (PR #136)
  - Verify: `bash scripts/release-smoke-test.sh <tag>` 12/12 PASS

### Observability and security

- [x] **Prometheus ServiceMonitor automatic** —
  `internal/resources/servicemonitor.go`,
  `servicemonitor_test.go`, chart
  `metrics.serviceMonitor.enabled=true`
- [x] **OpenSSF Scorecard + dependency-review + CodeQL SAST + DCO
  workflows** — see `.github/workflows/`
- [x] Grafana dashboards (cluster shard distribution / replication
  lag / memory pressure)
  - [x] 4 panels: cluster overview, replication, memory, latency — `charts/valkey-operator/dashboards/{cluster-overview,replication,memory,latency}.json`
  - [x] Helm-chart ConfigMap integration — `charts/valkey-operator/templates/grafana-dashboards.yaml`
- [x] OpenTelemetry trace propagation
  - [x] Instrument the controller reconcile span — all 5 controllers
    call `observability.StartReconcileSpan` + child `StartCallSpan`
    (`internal/controller/*_controller.go`)
  - [x] Wire up the OTLP exporter — `internal/observability/tracing.go`
    `SetupTracing` (OTLP gRPC, opt-in via `OTEL_EXPORTER_OTLP_ENDPOINT`,
    ADR-0025)
- [x] Image SBOM (SPDX) + trivy HIGH/CRITICAL fixed-only scan
  - [x] Adopt the shared 3-repo script — `scripts/sbom-attach.sh`
  - [x] Auto-attach at release time — `cosign attest` + `gh release upload`

## Next (2.x line — Planning)

### Features

- [ ] **Valkey 9.x feature follow-up** — flags / cluster-mode
  changes
- [ ] **Multi-cluster federation**
  - [ ] Separate ClusterRoles
  - [ ] Topology-aware routing
  - [ ] New CRD `ValkeyFederation`
- [ ] **Cross-region backup replication**
  - [ ] S3 SSE-KMS key management
  - [ ] Automatic lifecycle policies
- [ ] **Online schema-less migration**
  - [ ] RDB diff tool
  - [ ] LWW conflict resolution
- [ ] **Weighted read-replica routing** (latency-aware)

### Architecture

- [x] **Controller v2** (live since 1.1.0)
  - [x] workqueue rate-limiter tuning — typed rate-limiter, log `valkeycluster worker count:2`
  - [x] reconcile fan-out optimization — `MaxConcurrentReconciles` (valkey:3 / valkeycluster:2)
- [ ] **CRD v1 graduation**
  - [ ] Schema stabilization
  - [ ] v1alpha2 → v1 conversion webhook
  - Verify: six months with zero BREAKING CHANGEs and 3-repo
    compatibility

## Non-Goals (deliberate non-scope)

- ❌ **Multi-tenancy isolation** — namespace-level only. Stronger
  isolation belongs to a separate cluster.
- ❌ **In-house secret rotation logic** — delegated to ESO
  (External Secrets Operator) + OpenBao.
- ❌ **Sentinel mode** — Redis-Sentinel compatibility is not
  supported. Cluster mode is the path forward.
- ❌ **Calendar-based roadmap deadlines** — see
  `standards/workflow.md`.

## Change log

| Date | Change | Refs |
|---|---|---|
| 2026-06-04 | **v1.2.0** — module admission allow-list (외부 Redis Stack 거부 3중) + 수동 major-block webhook (Valkey+ValkeyCluster) shipped; Module controller wiring + Controller v2 마커 truth-up `[x]` (live since 1.1.0, reconcile path 실측); webhook admission 실작동은 `ENABLE_WEBHOOKS=false` (chart hook chicken-egg, #268) | PR #262, #263-268 |
| 2026-06-04 | Added **AutoUpdate — operator-managed 자동 버전 업데이트** as a `[~]` item — pure decision logic (`internal/autoupdate`), `AutoUpdateSpec` on v1alpha1(reconcile hub)+v1alpha2, and Valkey reconcile wiring (effective version → STS image + Status.Version) shipped; ValkeyCluster integration + major-block webhook remain. channel patch/minor, maintenance window, major auto-upgrade prohibited | PR #254 |
| 2026-06-03 | Added **Valkey official module presets (Redis Stack equivalent)** as a `[~]` item under "Stability and maturity" — the `ModuleSpec` / `ValkeySpec.Modules` API surface shipped (PR-C6.1); controller init-container wiring, webhook allow-list, chart values, and e2e remain (PR-C6.2). External Redis Stack modules stay out of scope (RSALv2 / SSPL ↔ BSD-3) | ADR-0032 |
| 2026-06-03 | Citation truth-up — fix phantom cited paths that the 2026-05-27 pass missed (features real, paths wrong): conversion webhook serving path not wired → `[~]` (`api/v1alpha2/doc.go`); PodSecurity helpers live in `statefulset.go` et al. (no `security.go`); webhook header `v1alpha2/`→`v1alpha1/` + "4 validating webhooks + conversion"; Online PVC resize → `commonspvc.ExpandDataPVCs` (ADR-0049, no `pvc_resize.go`); smoke-test Verify `hack/`→`scripts/`. Added `internal/observability/roadmap_citation_test.go` regression guard | docs/roadmap-citation-truthup |
| 2026-05-27 | Truth-up — flip stale `[ ]`→`[x]`: replicaCount lower-bound (already wired in webhooks), OpenTelemetry trace propagation (`SetupTracing` + 5-controller spans), Production cluster adoption (operator + 4 live CRs, ArgoCD Synced/Healthy); drop merged "(PR open)" tags (Grafana dashboards / SBOM) | lexical-puzzling-graham plan |
| 2026-05-12 | English becomes canonical; Korean preserved as `ROADMAP.ko.md`; ADR-0045 (GH Actions restoration) + ADR-0046 (SLSA-3 + cosign) noted in Operations and Security sections | i18n initiative |
| 2026-05-11 | Added webhook `validateStorageClassName` — RBD storageClass DNS-1123 baseline validation `[x]` | ralph-loop iter#2 |
| 2026-05-11 | Full rewrite — factual corrections (ServiceMonitor etc.), finer sub-task granularity, new items exposed (VolumeSnapshot multipod, conversion webhook) | parallel-leaping-seal plan |
| 2026-05-07 | Document created — 3-repo governance asset alignment | INC-2026-05-07 |
