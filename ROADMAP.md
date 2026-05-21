<p align="center">
  <b>English</b> |
  <a href="ROADMAP.ko.md">한국어</a> |
  <a href="ROADMAP.ja.md">日本語</a> |
  <a href="ROADMAP.zh.md">中文</a>
</p>

# ROADMAP — valkey-operator

> 한국어 버전: [ROADMAP.ko.md](ROADMAP.ko.md)

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
  - [x] 4 SecurityContext helpers unified — `internal/resources/security.go`
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
  - [x] v1alpha1 → v1alpha2 conversion webhook —
    `api/v1alpha2/conversion.go`
  - [x] 5 CRDs (Valkey, ValkeyCluster, ValkeyBackup, ValkeyRestore,
    ValkeyBackupTarget)
  - Verify: `kubectl apply -f <v1alpha1.yaml>` and check it is stored
    as a v1alpha2 object

- [x] **Online PVC resize** —
  `internal/controller/pvc_resize.go`

- [x] **Webhook admission validation (5 CRDs)** —
  `internal/webhook/v1alpha2/`
  - [x] RBD storageClass baseline validation —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateStorageClassName` (DNS-1123 subdomain)
  - [x] Topology-spread consistency validation —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateTopologySpread` (MaxSkew / TopologyKey /
    WhenUnsatisfiable / duplicate key, #77)
  - [ ] Wire the replicaCount lower-bound check into the webhook
  - Verify: invalid specs rejected by the webhook

- [x] **Encryption audit (TLS / encryption surveillance)** —
  `internal/controller/encryption_audit.go`,
  `encryption_enforce_test.go`

### Operations and delivery

- [x] Helm chart published — `keiailab.github.io/valkey-operator`
- [x] 3-repo (mongodb / postgres / valkey) governance asset
  alignment (CODE_OF_CONDUCT / GOVERNANCE / MAINTAINERS / ROADMAP)
- [x] **GitHub Actions release pipeline restored** (ADR-0045) —
  scoped deviation from RFC-0002 for externally-facing OSS repos;
  see [ADR-0045](docs/kb/adr/0045-restore-github-actions-for-oss-ci.md)
- [x] **SLSA-3 provenance + cosign keyless signing** for the image,
  Helm chart, and SBOM (ADR-0046) — verification commands in
  [SECURITY.md](SECURITY.md). Active from v1.0.13.
- [ ] **argos cluster deploy**
  - [ ] CRD-install manifest
  - [ ] ArgoCD application registration
  - [ ] Migrate `argos-platform-data/valkey` from a plain
    StatefulSet to the operator
  - Verify: ArgoCD Synced/Healthy and
    `kubectl get valkey/valkeycluster -A`
- [x] **Migration runbook** — plain StatefulSet → ValkeyCluster CR (PR #136)
  - [x] Document the zero-downtime procedure — `docs/migration/zero-downtime.md` (PR #136)
  - [x] Secondary-promote-based cutover — `docs/migration/secondary-promote.md` (PR #136)
  - [x] Rollback procedure — `docs/migration/rollback.md` (PR #136)
  - Verify: staging dry-run with RTO / RPO measurements recorded
- [x] **release-smoke-test.sh** — port the mongodb-operator pattern (PR #136)
  - [x] Five stages: image / SBOM / trivy / chart index / smoke — `scripts/release-smoke-test.sh` (PR #136)
  - Verify: `bash hack/release-smoke-test.sh <tag>` 12/12 PASS

### Observability and security

- [x] **Prometheus ServiceMonitor automatic** —
  `internal/resources/servicemonitor.go`,
  `servicemonitor_test.go`, chart
  `metrics.serviceMonitor.enabled=true`
- [x] **OpenSSF Scorecard + dependency-review + CodeQL SAST + DCO
  workflows** — see `.github/workflows/`
- [x] Grafana dashboards (cluster shard distribution / replication (PR open)
  lag / memory pressure)
  - [x] 4 panels: cluster overview, replication, memory, latency — `charts/valkey-operator/dashboards/{cluster-overview,replication,memory,latency}.json` (PR open)
  - [x] Helm-chart ConfigMap integration — `charts/valkey-operator/templates/grafana-dashboards.yaml` (PR open)
- [ ] OpenTelemetry trace propagation
  - [ ] Instrument the controller reconcile span
  - [ ] Wire up the OTLP exporter
- [x] Image SBOM (SPDX) + trivy HIGH/CRITICAL fixed-only scan (PR open)
  - [x] Adopt the shared 3-repo script — `scripts/sbom-attach.sh` (PR open)
  - [x] Auto-attach at release time — `cosign attest` + `gh release upload` (PR open)

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

- [ ] **Controller v2**
  - [ ] workqueue rate-limiter tuning
  - [ ] reconcile fan-out optimization
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
| 2026-05-12 | English becomes canonical; Korean preserved as `ROADMAP.ko.md`; ADR-0045 (GH Actions restoration) + ADR-0046 (SLSA-3 + cosign) noted in Operations and Security sections | i18n initiative |
| 2026-05-11 | Added webhook `validateStorageClassName` — RBD storageClass DNS-1123 baseline validation `[x]` | ralph-loop iter#2 |
| 2026-05-11 | Full rewrite — factual corrections (ServiceMonitor etc.), finer sub-task granularity, new items exposed (VolumeSnapshot multipod, conversion webhook) | parallel-leaping-seal plan |
| 2026-05-07 | Document created — 3-repo governance asset alignment | INC-2026-05-07 |

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
