# ROADMAP — valkey-operator

본 ROADMAP 은 *현재* 와 *다음 6 개월* 의 우선순위를 명시합니다. 기간 기반 deadline 은 의도적으로 회피하며, *기능 단위* 로 진행을 추적합니다 (글로벌 §workflow.md "시간 기반 로드맵 금지").

## 현재 (1.x 라인 — Active)

### 안정성 / 성숙도
- [x] PodSecurity restricted compliance (4 곳 SecurityContext helper 통일, 회귀 가드)
- [x] Cluster mode (5 shard × replica=2) ordinal 기반 restore init container
- [x] Cluster mode 자동 failover 검증 (Phase A3 chaos 2026-05-07 — primary kill → 자동 master 재선출, 16384 slots OK 유지, 데이터 잔존)
- [x] HPA / PDB / NetworkPolicy 자동화 (opt-in)
- [x] Backup / Restore — S3 (minio-go) + PVC ROX 다중 마운트
- [x] **chart RBAC conditional 결함 fix** (2026-05-07 commit 06237be — `features.{cluster,backup}.enabled=false` 시 informer startup 실패) — production-grade 차단 요인 P0
- [ ] **Valkey 9.x 지원 격상 (1.x 라인 진입)** — ROADMAP 2.x 에서 1.x 로. **Phase B 마이그레이션 prerequisite** (bitnami/valkey 9.0.4 → 자체 operator 시 RDB format v80 호환 필수). 2026-05-07 Phase B PoC 시 차단 확인.
- [x] **version upgrade reconcile 결함** — Phase B PoC (2026-05-07) 가 `spec.version.version` patch 의 STS image 미반영을 의심. iteration 7 진단: **fresh 시나리오 정상**. iteration 18 (Phase 2 V2): **narrow scope (restore→patch chain) 회귀 가드 영구화** — `test/e2e/backup_restore_test.go` 의 "Restored 인스턴스의 8.1.6 → 9.0.4 version patch chain (V2)" Context 추가. 가설 A/B/C 3 영역 + RDB v80 호환성 (foo=bar1 보존) 모두 회귀 가드. 본 e2e PASS = 차단요인 2 narrow scope 까지 영구 해소.
- [ ] PodSecurity restricted *전수* 회귀 — controller / webhook 측 podSpec 변환 경로도 가드 추가
- [ ] webhook validation rule 통합 — RBD storageClass / topology spread / replicaCount lower bound

### 운영 / 배포
- [x] Helm chart `keiailab.github.io/valkey-operator` publish
- [x] 3-repo (mongodb / postgresql / valkey) governance 자산 정합 (CODE_OF_CONDUCT / GOVERNANCE / MAINTAINERS / ROADMAP)
- [ ] argos 클러스터 deploy — CRD 설치 + ArgoCD app 등록 (현재 `argos-platform-data/valkey` 는 plain StatefulSet, operator 미적용)
- [ ] Migration runbook — plain StatefulSet → ValkeyCluster CR (zero-downtime, secondary-promote 기반)
- [ ] release-smoke-test.sh — mongodb-operator 패턴 적용 (image / sbom / trivy / chart index / smoke)

### 관측 / 보안
- [ ] Prometheus ServiceMonitor 자동 — chart values 노출
- [ ] Grafana 대시보드 (cluster shard 분포 / replication lag / memory pressure)
- [ ] OpenTelemetry trace propagation — controller reconcile span
- [ ] Image SBOM (SPDX) + trivy HIGH/CRITICAL fixed-only 스캔 (3-repo 표준)

## 다음 (2.x 라인 — Planning)

### 기능
- [ ] Valkey 9.x 지원 — flag/cluster mode 변경분 follow-up
- [ ] Multi-cluster federation — ClusterRole 분리 + 토폴로지 인식 라우팅
- [ ] Cross-region backup replication — S3 SSE-KMS + lifecycle policy 자동
- [ ] Online schema-less migration — RDB diff + LWW conflict resolution
- [ ] Read replicas 가중치 라우팅 (latency-aware)

### 아키텍처
- [ ] Controller v2 — workqueue rate limiter 튜닝 + reconcile fan-out 최적화
- [ ] CRD v1 졸업 (현재 v1alpha1) — schema 안정화 + conversion webhook

## Non-Goals (의식적 비대상)

- **Multi-tenancy 격리** — namespace 단위 격리만 제공. 더 강한 격리는 별도 클러스터로 위임.
- **자체 시크릿 관리** — ESO (External Secrets Operator) + OpenBao 위임. operator 자체 시크릿 회전 로직은 *추가하지 않음*.
- **GitHub Actions** — RFC 0002 (글로벌) 영구 금지. 모든 게이트는 로컬 4 계층.

## 변경 이력

| Date | Change | Refs |
|---|---|---|
| 2026-05-07 | 본 문서 신설 — 3-repo governance 자산 정합 | INC-2026-05-07 |
