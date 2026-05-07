# Changelog

본 프로젝트의 모든 주요 변경은 본 파일에 기록된다.
형식: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
버저닝: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

자동 생성: `git-cliff` (P1 §2.3 표준) — release tag 시점에 PR 자동 갱신.

## [Unreleased]

## [0.1.0-alpha.2] - 2026-05-07

ADR-0057 Phase A1 (argos 클러스터 사전 배포) 진행 중 발견된 chart RBAC 결함 fix.

### Fixed
- **chart RBAC P0 — `features.{cluster,backup}.enabled=false` 시 informer startup 실패** (`charts/valkey-operator/templates/clusterrole.yaml`):
  이전 chart 가 `features.cluster.enabled` / `features.backup.enabled` 조건부로 `valkeyclusters` / `valkeybackups` / `valkeybackuptargets` / `valkeyrestores` RBAC 부여 — 그러나 operator manager (`cmd/main.go`) 는 *항상* 모든 controller 등록 → flag=false 시 informer 가 `forbidden` 으로 startup 실패. RBAC 와 코드 mismatch 가 production-grade 차단 요인. RBAC 를 *항상 모든 CRD 권한 부여* 로 단순화, feature flag 는 controller 코드 측에서만 처리.

### Verified (argos 클러스터 Phase A1 + A2)
- valkey-operator pod 1/1 Running, Certificate/Issuer/ValidatingWebhookConfiguration Ready
- Valkey CR `valkey-test` (Standalone, valkey 8.1.6, 1Gi ceph-rbd) 1/1 Running
- SET/GET smoke: `SET phase-a2-smoke "OK-2026-05-07"` → `OK`, `GET` → 정상 round-trip
- `INFO server`: valkey_version=8.1.6, tcp_port=6379

### Refs
- ADR-0057 (argos-infra-bootstrap 43fd542): self-hosted valkey-operator 채택 로드맵
- 운영 사고 분석 + Phase A 진행: keiailab/mongodb-operator HANDOFF.md (2026-05-07)

### Added (GitOps deploy 정합)

- `deploy/overlays/prod/` GitOps 진입점 — config/{crd,rbac,manager} 를 prod ns 로
  정렬 + 자동 생성 Namespace 제거. ArgoCD 단방향 동기 전제.
- `deploy/valkey-cluster.yaml` — production ValkeyCluster sample (db ns,
  shards=3, replicasPerShard=1, ceph-block, auth.enabled=true).
- `deploy/README.md` — 운영 런북.
- ADR-0029 — GitOps deploy 오버레이 도입 (mongodb-operator / postgresql-operator 와 3-repo 정합).

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
