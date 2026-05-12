# Metrics Glossary — valkey-operator (한국어)

> English: [metrics-glossary.md](metrics-glossary.md) — canonical / 정본


Operator 가 노출하는 Prometheus metrics 의 *의미 / 타입 / 라벨 / 정상 범위 / 진단
용도* 를 한 문서에 정리. SSOT 코드: `internal/controller/metrics.go`. SSOT alert:
`config/prometheus/alert-rules.yaml`.

본 glossary 는 **운영자가 dashboard / alert 의 각 metric 을 *왜 보는가* 를 이해**
하기 위한 reference. 새 alert 추가 또는 dashboard 작성 시 우선 본 표를 확인.

## 1. 표기 규약

- **타입**: Gauge (snapshot) / Counter (monotonic, rate 사용) / Histogram (현재 미정의).
- **라벨**: cardinality 폭발 방지 위해 `namespace` + `name` 만 (cluster 단위). pod /
  shard 단위 라벨은 *의도적 제외* — 100 shard cluster 에서 timeseries 폭증.
- **정상 범위**: 운영 중 *정상 operating envelope*. 임계 초과 시 §5 alert 매핑 참조.

## 2. ValkeyCluster 상태 metrics (Gauge)

| Metric | 의미 | 정상 | 비정상 → alert |
|---|---|---|---|
| `valkey_cluster_state_ok{namespace,name}` | CLUSTER INFO `cluster_state` == ok 시 1. | `1` | `0` 5m → `ValkeyClusterStateNotOK` (critical) |
| `valkey_cluster_assigned_slots{namespace,name}` | 할당된 hash slot 수. 정상 = 16384 (Redis cluster spec). | `16384` | `< 16384` 5m → `ValkeyClusterSlotsMismatch` (critical) |
| `valkey_cluster_shards{namespace,name}` | primary 노드 수 (= cluster 의 shard 수). | spec.shards 와 일치 | mismatch = scale 진행 중 또는 reconcile 실패 |
| `valkey_cluster_ready_replicas{namespace,name}` | StatefulSet 의 readyReplicas. | `>= spec.replicas` | `0` → critical, `0 < x < 2` warning |
| `valkey_cluster_phase{namespace,name,phase}` | active phase 만 1, 나머지 0. phase ∈ {Pending, Initializing, Running, Resharding, Failed, Upgrading}. | `Running=1` | `Failed=1` 5m → `ValkeyClusterPhaseFailed` |

**진단 활용**:
- `valkey_cluster_state_ok == 0 unless valkey_cluster_phase{phase="Initializing"} == 1`
  → Initializing 중인 cluster 의 false positive 제외.
- `valkey_cluster_assigned_slots != 16384 and on(namespace,name) valkey_cluster_phase{phase="Resharding"} == 0`
  → Resharding 진행 중이 아닌데 slot 부족 = 진짜 장애.

## 3. Reconcile metrics (Counter / Histogram)

| Metric | 타입 | 의미 | 정상 (rate 5m) | 비정상 |
|---|---|---|---|---|
| `valkey_cluster_reconcile_total{namespace,name}` | Counter | Reconcile 호출 누적. | `0.01 ~ 1 /s` (RequeueAfter 30s+ 기반) | `> 5 /s` = thrashing 의심 |
| `valkey_cluster_reconcile_errors_total{namespace,name,component}` | Counter | reconcile 단계별 실패 카운터. component=`secret`/`sts`/`svc`/`tls`/`backup`/... | `0` | `rate > 0.1 /s` 5m → `ValkeyOperatorReconcileErrorsHigh` |
| `valkey_cluster_reconcile_duration_seconds{namespace,name,result}` | Histogram | reconcile wall-clock latency. result=`success`/`error`. Buckets: 5ms~30s. | p95 `< 1s` (steady), `< 5s` (init/scale) | p95 `> 5s` 지속 = SLO 위반 |

**Histogram 활용 PromQL** (SLO 추적):

```promql
# reconcile p95 (success only) — typical operation 의 SLO 측정
histogram_quantile(0.95,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket{result="success"}[5m])
  )
)

# reconcile p99 (모든 결과) — worst-case 측정
histogram_quantile(0.99,
  sum by (le, namespace, name) (
    rate(valkey_cluster_reconcile_duration_seconds_bucket[5m])
  )
)

# reconcile 평균 latency
rate(valkey_cluster_reconcile_duration_seconds_sum[5m])
  / rate(valkey_cluster_reconcile_duration_seconds_count[5m])
```

**활용 패턴**:
- Reconcile error rate 가 한 component 에 집중 = 그 component 의 의존성 검토.
  - `secret` → AuthSecret 미존재 / RBAC 부족
  - `sts` → admission webhook reject / quota 부족
  - `tls` → cert-manager Certificate 미생성 또는 NotReady
- Total 이 폭증 + error 0 = phase 진동 (phase 변경마다 즉시 reconcile).

## 4. Lifecycle event metrics (Counter)

| Metric | 의미 | 정상 (rate 1h) | 비정상 |
|---|---|---|---|
| `valkey_cluster_backup_total{namespace,name,phase}` | ValkeyBackup 종료 카운터. phase=`Completed`/`Failed`. | `> 0` (Completed 만), `0` (Failed) | `rate(...phase="Failed"[1h]) > 0.0017` (1주 1회) → `ValkeyBackupFailureRateHigh` |
| `valkey_cluster_restore_total{namespace,name,phase}` | ValkeyRestore 종료 카운터. | `0` (재해 외 발생 없음) | `rate(...phase="Failed"[1h]) > 0.0017` → `ValkeyRestoreFailureRateHigh` |
| `valkey_cluster_failover_total{namespace,name}` | Replication mode 자동 failover 발생. ADR-0017. | `0` | `increase(...[1h]) >= 2` → `ValkeyFailoverHigh` (1시간 2회 이상 = 인프라 불안정) |

**SLO 산정 가이드**:
- backup 성공률 SLO: `1 - rate(backup_total{phase="Failed"}[7d]) / rate(backup_total[7d])`
  목표 ≥ 99%.
- failover MTTR: `histogram_quantile(0.95, ...)` — Histogram 미정의 상태이므로 추정치는 §6 참조.

## 5. Operator 자체 상태 metrics (Gauge / from kube-state)

| Metric | source | 정상 | 비정상 → alert |
|---|---|---|---|
| `up{job=~"valkey-operator.*"}` | Prometheus scraping | `1` | `0` 2m → `ValkeyOperatorDown` (critical) |
| `valkey_cluster_build_info{version,commit,date}` | `SetBuildInfo` 부팅 1회 | always `1` (현재 release tag 식별용) | 라벨 mismatch = ldflags 주입 실패 (cycles 53-56) |
| `valkey_cluster_capability_active{namespace,name,capability}` | reconcile 마다 갱신 (PR #64) | active=`1`, inactive=`0`. capability 토큰: TLS / TLS-AutoCA / Auth / Autoscaling / SlowLog / EncryptionAudit / EncryptionEnforce / NetworkPolicy / Monitoring | fleet-wide 채택 추적 — `sum by (capability) (valkey_cluster_capability_active)` |

## 6. 파생 / 권장 PromQL (운영 dashboard)

```promql
# cluster availability (SLI)
avg_over_time(valkey_cluster_state_ok[5m])

# reconcile latency proxy (Histogram 미정의 → rate 비율)
rate(valkey_cluster_reconcile_total[5m]) / scalar(count(up{job=~"valkey-operator.*"} == 1))

# backup 성공률 (SLO)
1 - (
  rate(valkey_cluster_backup_total{phase="Failed"}[7d])
  / rate(valkey_cluster_backup_total[7d])
)

# operator 운영 중 image 식별 (release sanity)
valkey_cluster_build_info * 0 + group_left(version, commit) (1)
```

## 7. cardinality 추정 (운영 capacity)

| 운영 규모 | timeseries 추정 |
|---|---|
| 10 cluster (소규모) | ~110 (= 11 metric × 10 cluster) |
| 100 cluster | ~1100 |
| 1000 cluster | ~11000 (Prometheus 단일 인스턴스 권장 한계 약 5-10M, 충분) |

**확장 시 주의**: `reconcile_errors_total` 의 `component` 차원이 N 종류 →
`N × cluster` 곱연산. component 종류 추가 시 본 표 갱신.

## 8. 미정의 (의도적 미수집)

- pod-level latency (개별 valkey instance 의 INFO `latest_fork_usec` 등) — operator
  scope 밖. 별도 `redis_exporter` sidecar 로 수집 권장.
- key count / memory usage — 동일 (sidecar exporter).
- network bytes — node_exporter / cAdvisor 표준.

operator 의 책임은 *Kubernetes 레벨 컨트롤러 상태* 만 노출. 데이터 plane metric
은 별도 layer.

## 9. 참조

- 코드: `internal/controller/metrics.go`
- alert: `config/prometheus/alert-rules.yaml`
- alert MTTR: `docs/operations/runbook.md` §9.x
- ADR: ADR-0017 (failover), ADR-0027 (HPA deferred), ADR-0030 (build_info ldflags)
