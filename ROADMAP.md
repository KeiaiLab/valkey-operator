# ROADMAP — valkey-operator

본 ROADMAP 은 *날짜 약속이 아니라* 검증 가능한 기능 체크리스트로 진행을 추적한다. 시간 기반 deadline 은 의도적으로 회피하며 (글로벌 `standards/workflow.md` "시간 기반 로드맵 금지"), 기능 단위로 진행을 추적한다.

## 체크박스 의미

| 마커 | 의미 |
|---|---|
| `[x]` | 코드 + 테스트 양쪽 존재. e2e 또는 unit test 로 회귀 가드 확보 |
| `[~]` | 부분 구현 (필드만 존재, helper 미통합, 또는 잔여 검증 항목 있음) |
| `[ ]` | 미시작 (설계 또는 PoC 단계) |

각 sub-task 우측 *Verify* 는 검증 명령 또는 e2e 파일을 인용한다.

## 현재 (1.x 라인 — Active)

### 안정성 / 성숙도

- [x] **PodSecurity restricted compliance**
  - [x] 4 곳 SecurityContext helper 통일 — `internal/resources/security.go`
  - [x] restricted PSA 회귀 가드 (resources 빌더)
  - [ ] controller / webhook 측 podSpec 변환 경로 전수 가드 — `internal/webhook/v1alpha{1,2}/*.go`
  - Verify: `kubectl label ns <ns> pod-security.kubernetes.io/enforce=restricted` 후 pod ready

- [x] **Cluster mode (5 shard × replica=2)**
  - [x] Ordinal 기반 restore init container — `internal/controller/valkeycluster_controller.go`
  - [x] 16384 slots 자동 분배
  - [x] 자동 failover (chaos 검증) — `test/e2e/cluster_recovery_test.go`, `failover.go`
  - [x] Primary kill → master 재선출 — `test/e2e/failover_test.go`
  - Verify: `test/e2e/cluster_recovery_test.go` PASS, slot 16384 유지, 데이터 잔존

- [x] **HPA / PDB / NetworkPolicy 자동화 (opt-in)**
  - [x] HPA (ADR-0027, Replication mode) — chart `autoscaling.enabled`
  - [x] PDB 자동 — `internal/controller/pdb_default.go`
  - [x] NetworkPolicy default-deny + 명시 규칙 — chart `networkPolicy.enabled`
  - Verify: `pdb_default_test.go` PASS, `kubectl get pdb/networkpolicy` 출력 확인

- [x] **Backup / Restore — S3 + PVC ROX + VolumeSnapshot**
  - [x] S3 (minio-go) backup — `internal/controller/valkeybackup_controller.go`
  - [x] PVC ROX 다중 마운트 restore — `internal/controller/valkeyrestore_controller.go`
  - [x] VolumeSnapshot lifecycle — `internal/controller/backup_volumesnapshot.go`
  - [x] Multipod snapshot replication restore — `multipod_volumesnapshot_replication_test.go`
  - [x] `ValkeyBackupTarget` CRD (외부 backup destination) — `api/v1alpha2/valkeybackuptarget_types.go`
  - Verify: `test/e2e/backup_restore_test.go` PASS

- [x] **chart RBAC conditional 결함 fix** (2026-05-07 commit 06237be)
  - [x] `features.{cluster,backup}.enabled=false` 시 informer startup 실패 차단
  - Verify: chart `--set features.cluster.enabled=false` 설치 후 operator pod Ready

- [x] **Version upgrade reconcile 결함**
  - [x] Fresh scenario 정상 (iteration 7 진단)
  - [x] Restore → patch chain 회귀 가드 (iteration 18 V2) — `test/e2e/backup_restore_test.go` "Restored 인스턴스의 8.1.6 → 9.0.4 version patch chain (V2)"
  - [x] RDB v80 호환성 (foo=bar1 보존)
  - Verify: 본 e2e PASS = 차단요인 2 narrow scope 영구 해소

- [x] **Valkey 9.x 지원 (기본값 9.0.4)**
  - [x] Chart `image.tag: 9.0.4` 기본값 — `charts/valkey-operator/values.yaml`
  - [x] RDB format v80 호환 검증
  - Verify: 신규 인스턴스 기동 후 `valkey-cli INFO server | grep redis_version`

- [x] **API 버전 진화**
  - [x] v1alpha2 활성 — `api/v1alpha2/`
  - [x] v1alpha1 → v1alpha2 conversion webhook — `api/v1alpha2/conversion.go`
  - [x] 5 CRD (Valkey, ValkeyCluster, ValkeyBackup, ValkeyRestore, ValkeyBackupTarget)
  - Verify: `kubectl apply -f <v1alpha1.yaml>` 후 v1alpha2 객체로 변환 확인

- [x] **PVC online resize** — `internal/controller/pvc_resize.go`

- [x] **Webhook admission validation (5 CRD 대상)** — `internal/webhook/v1alpha2/`
  - [x] RBD storageClass 기본 검증 — `internal/webhook/v1alpha1/valkeycluster_webhook.go` `validateStorageClassName` (DNS-1123 subdomain)
  - [ ] topology spread 일관성 검증
  - [ ] replicaCount lower bound 검증 통합
  - Verify: invalid spec 적용 시 webhook reject

- [x] **Encryption audit (TLS/암호화 감시)** — `internal/controller/encryption_audit.go`, `encryption_enforce_test.go`

### 운영 / 배포

- [x] Helm chart publish — `keiailab.github.io/valkey-operator`
- [x] 3-repo (mongodb/postgres/valkey) governance 자산 정합 (CODE_OF_CONDUCT / GOVERNANCE / MAINTAINERS / ROADMAP)
- [ ] **argos 클러스터 deploy**
  - [ ] CRD 설치 manifest
  - [ ] ArgoCD application 등록
  - [ ] 현재 `argos-platform-data/valkey` 가 plain StatefulSet — operator 적용 마이그레이션
  - Verify: ArgoCD Synced/Healthy + `kubectl get valkey/valkeycluster -A`
- [ ] **Migration runbook** — plain StatefulSet → ValkeyCluster CR
  - [ ] Zero-downtime 절차 문서화
  - [ ] secondary-promote 기반 cutover
  - [ ] 롤백 절차
  - Verify: 스테이징 환경 dry-run + RTO/RPO 측정 기록
- [ ] **release-smoke-test.sh** — mongodb-operator 패턴 적용
  - [ ] image / sbom / trivy / chart index / smoke 5단계
  - Verify: `bash hack/release-smoke-test.sh <tag>` 12/12 PASS

### 관측 / 보안

- [x] **Prometheus ServiceMonitor 자동** — `internal/resources/servicemonitor.go`, `servicemonitor_test.go`, chart `metrics.serviceMonitor.enabled=true`
- [ ] Grafana 대시보드 (cluster shard 분포 / replication lag / memory pressure)
  - [ ] 4개 패널 (cluster overview / replication / memory / latency)
  - [ ] Helm chart ConfigMap 통합
- [ ] OpenTelemetry trace propagation
  - [ ] Controller reconcile span 계측
  - [ ] OTLP exporter 통합
- [ ] Image SBOM (SPDX) + trivy HIGH/CRITICAL fixed-only 스캔
  - [ ] 3-repo 표준 스크립트 도입
  - [ ] Release 시점 자동 첨부

## 다음 (2.x 라인 — Planning)

### 기능

- [ ] **Valkey 9.x 신규 기능 활용** — flag / cluster mode 변경분 follow-up
- [ ] **Multi-cluster federation**
  - [ ] ClusterRole 분리
  - [ ] 토폴로지 인식 라우팅
  - [ ] 신규 CRD `ValkeyFederation`
- [ ] **Cross-region backup replication**
  - [ ] S3 SSE-KMS 키 관리
  - [ ] Lifecycle policy 자동
- [ ] **Online schema-less migration**
  - [ ] RDB diff 도구
  - [ ] LWW conflict resolution
- [ ] **Read replicas 가중치 라우팅** (latency-aware)

### 아키텍처

- [ ] **Controller v2**
  - [ ] workqueue rate limiter 튜닝
  - [ ] reconcile fan-out 최적화
- [ ] **CRD v1 졸업**
  - [ ] schema 안정화
  - [ ] v1alpha2 → v1 conversion webhook
  - Verify: 6개월 BREAKING CHANGE 0건 + 3 repo 호환

## Non-Goals (의식적 비대상)

- ❌ **Multi-tenancy 격리** — namespace 단위만. 더 강한 격리는 별도 클러스터로 위임.
- ❌ **자체 시크릿 회전 로직** — ESO (External Secrets Operator) + OpenBao 위임.
- ❌ **Sentinel mode** — Redis Sentinel 호환 미지원. Cluster mode 우선.
- ❌ **GitHub Actions** — RFC 0002 글로벌 영구 금지. 모든 게이트는 로컬 4 계층.
- ❌ **시간 기반 로드맵 deadline** — 글로벌 §workflow.md.

## 변경 이력

| Date | Change | Refs |
|---|---|---|
| 2026-05-11 | webhook `validateStorageClassName` 추가 — RBD storageClass 기본 검증 (DNS-1123 subdomain) `[x]` | ralph-loop iter#2 |
| 2026-05-11 | 전면 재작성 — 사실 정정 (ServiceMonitor 등) + sub-task 체크리스트 입자도 + 신규 항목 (VolumeSnapshot multipod / conversion webhook) 노출 | parallel-leaping-seal plan |
| 2026-05-07 | 본 문서 신설 — 3-repo governance 자산 정합 | INC-2026-05-07 |
