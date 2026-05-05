# Changelog

본 프로젝트의 모든 주요 변경은 본 파일에 기록된다.
형식: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
버저닝: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

자동 생성: `git-cliff` (P1 §2.3 표준) — release tag 시점에 PR 자동 갱신.

## [Unreleased]

### Added
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
