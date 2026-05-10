# Capacity Planning — valkey-operator

ValkeyCluster / Valkey CR 을 *어떤 spec 으로 시작할 것인가* 의 산정 가이드.
production 운영 전 본 문서의 §3 워크로드 패턴 매트릭스로 1차 sizing → §4 감시
지표로 *주 단위 재산정*.

본 문서는 *완벽한 capacity model* 이 아닌 *실용적 시작점* 제공. 정확한 값은 운영
중 metric 으로 보정.

## 1. 산정 4 차원

모든 sizing 은 다음 4 차원의 곱:

```
필요 자원 = (워크로드 패턴) × (데이터 양) × (QPS) × (가용성 요구)
```

| 차원 | 측정 단위 | 영향 자원 |
|---|---|---|
| 워크로드 패턴 | cache / session / queue / pub-sub / leaderboard | memory model, persistence, eviction |
| 데이터 양 | 활성 keyspace MB | memory request, PVC size |
| QPS | read / write / batch 분리 | CPU request, replica 수 |
| 가용성 요구 | RPO / RTO 분 단위 | replicas, shards, backup 주기, target 수 |

## 2. 토폴로지 선택

```
                ┌─ 단일 인스턴스 + 영속 불필요 → kind: Valkey, mode: Standalone
                │
워크로드 ───────┼─ 1 primary + N read replica + RPO≈0 → kind: Valkey, mode: Replication
                │
                └─ 분산 (>50GB 또는 >100K QPS write) → kind: ValkeyCluster (sharded)
```

**전환 임계 (경험치)**:
- Standalone → Replication: 데이터 손실 1초도 불가 시점 (자동 failover 필요).
- Replication → Cluster: 단일 primary memory 가 *physical RAM × 0.5* 초과 또는
  단일 primary CPU 가 1 core 80%+ 지속.
- Cluster shard 추가: 단일 shard 의 95th percentile latency 가 SLO 초과.

## 3. 워크로드 패턴별 권장 시작점

### 3.1 Cache (read-heavy, eviction allowed)

| 항목 | 권장 |
|---|---|
| memory | 데이터셋 × 1.3 (fragmentation 30% 여유) |
| persistence | RDB (없어도 OK), AOF off |
| eviction | `allkeys-lru` 또는 `allkeys-lfu` |
| replicas | 2 (read scaling) |
| backup | 불필요 또는 1일 1회 |
| spec 예시 | replicas=2, requests.memory=4Gi, persistence.size=8Gi |

### 3.2 Session store (read+write balanced, no loss)

| 항목 | 권장 |
|---|---|
| memory | 동시 세션 수 × 평균 session 크기 × 1.5 |
| persistence | AOF `everysec` |
| eviction | `noeviction` (TTL 기반 자연 만료) |
| replicas | 2 (HA + read scaling) |
| backup | 6시간 1회 |
| spec 예시 | replicas=2, requests.memory=8Gi, persistence.size=20Gi |

### 3.3 Queue (write-heavy, FIFO)

| 항목 | 권장 |
|---|---|
| memory | 평균 queue depth × message 크기 × 1.5 |
| persistence | AOF `always` (loss 0 요구 시) 또는 `everysec` |
| eviction | `noeviction` (queue overflow = consumer 부재 의미) |
| replicas | 2 (failover) |
| backup | 1시간 1회 (point-in-time 부재 — interval 짧게) |
| spec 예시 | replicas=2, requests.cpu=1000m, requests.memory=4Gi |

### 3.4 Pub-Sub (transient, no persistence)

| 항목 | 권장 |
|---|---|
| memory | 256MB ~ 1GB (메시지 자체 미저장) |
| persistence | off |
| replicas | 0~1 (subscriber 가 reconnect 가능 시 단일 OK) |
| spec 예시 | replicas=1, requests.memory=512Mi, persistence.enabled=false |

### 3.5 Leaderboard / sorted set (heavy compute, large dataset)

| 항목 | 권장 |
|---|---|
| memory | (active leaderboards × top-N × 평균 score+member 크기) × 2 |
| persistence | RDB 1시간 + AOF `everysec` |
| replicas | 3 (read replica 2 — ZRANGE 부하 분산) |
| topology | ValkeyCluster 권장 (leaderboard 분할 가능 시) |
| spec 예시 | shards=3, replicas=2, requests.memory=8Gi/shard |

## 4. 산정 검증 — 운영 첫 1주의 metric 체크

배포 후 7일간 다음을 매일 1회 확인 후 spec 보정:

| 지표 | PromQL | 임계 (보정 트리거) |
|---|---|---|
| memory 사용률 | (sidecar exporter) `redis_memory_used_bytes / redis_memory_max_bytes` | `> 0.7` 5일 연속 = memory 증액 |
| QPS | `rate(redis_commands_total[5m])` | `> capacity × 0.7` = CPU 증액 또는 shard 추가 |
| reconcile error rate | `rate(valkey_cluster_reconcile_errors_total[1h])` | `> 0` = 운영 안정성 검토 |
| failover 빈도 | `increase(valkey_cluster_failover_total[7d])` | `> 1` = 인프라 점검 (네트워크 / 노드) |
| backup 성공률 | `1 - rate(valkey_cluster_backup_total{phase="Failed"}[7d]) / rate(valkey_cluster_backup_total[7d])` | `< 0.99` = backup 인프라 점검 |

## 5. resource request / limit 시작값

```yaml
# Valkey CR (Replication mode 권장값)
spec:
  resources:
    requests:
      cpu: 500m       # 평균 5K QPS 기준
      memory: 2Gi     # 1.6GB working set + 400MB overhead
    limits:
      cpu: 2000m      # burst 4x
      memory: 2Gi     # request == limit (OOM 예방, swap 회피)
  storage:
    size: 10Gi        # data + AOF rewrite 임시 공간 = data × 2
    storageClass: gp3 # IOPS 보장 storage 권장
```

**memory request == limit 권장 이유**: K8s scheduler 가 노드의 *available
memory* 만 보고 schedule. limit 만 크면 노드 over-commit 위험. valkey 는 OOM
시 데이터 손실이 critical.

**CPU limit 의 burst 허용 이유**: AOF rewrite, RDB save, BGSAVE 같은 background
job 이 일시적으로 CPU 2~4x burst. limit 너무 낮으면 throttle → latency 폭증.

## 6. 가용성 요구별 replica / shard 매트릭스

| RPO / RTO | 권장 |
|---|---|
| RPO=0, RTO<10s | Replication 3 replica + AOF `always` + automatic failover |
| RPO<1s, RTO<30s | Replication 2 replica + AOF `everysec` (default) |
| RPO<1h, RTO<5min | Replication 1 replica + RDB hourly + backup S3 |
| RPO<1d, RTO<1h | Standalone + RDB daily + backup PVC |

**Cluster mode 의 가용성**: 각 shard 는 *replication mode 의 가용성과 동일* —
shard 별 replica 수 = `Spec.Shards[].Replicas`. shard 자체의 quorum 은 majority
of primaries (e.g. 3 shard cluster 는 2 shard 동작 시 partial cluster 운영 가능
— 단, 해당 slot 의 데이터만 영향).

## 7. 한계 / scope 외

본 가이드는 다음을 *가정 / 제외*:

- 노드 인프라 (CPU 종류, NIC bandwidth, disk IOPS) 가 *충분* 가정. EBS gp2 등
  IOPS 제한 storage 는 별도 산정.
- 멀티 region active-active 는 본 operator scope 밖 (별도 ADR / 도구 필요).
- Hot-key 시나리오 (단일 key QPS 가 cluster 총 QPS 의 50%+) 는 cluster 로 해소
  불가 — application 분할 또는 caching 계층 추가.

## 8. 참조

- ADR-0017: Replication failover 정책
- ADR-0027: HPA 미지원 사유 (수동 spec 변경 권장)
- runbook.md §4: scale up/down 절차
- metrics-glossary.md: 본 문서의 PromQL 사용 metric 의미
