# Sentinel → valkey-operator Replication Mode 마이그레이션 runbook (한국어)

> English: [sentinel-migration.md](sentinel-migration.md) — canonical / 정본


> ADR-0017 (Replication Mode Failover) 의 Sentinel 거절 결정에 대한 *외부
> 사용자 마이그레이션 path*.

## 배경

valkey-operator 는 ADR-0017 거절 — 동등 가용성을 *Replication mode +
AutoFailover* (operator-managed leader-elect + STS rollout + largest
master_repl_offset 선출) 로 제공.

본 runbook 은 *기존 Sentinel 기반 Redis/Valkey 인프라* 에서 valkey-operator
로 이전하는 운영자 가이드.

## 가용성 동등성

| 측면 | Sentinel HA (기존) | valkey-operator Replication + AutoFailover |
|---|---|---|
| failover 결정 | sentinel quorum 투표 | operator leader-elect + ADR-0017 largest `master_repl_offset` |
| 데이터 무손실 보장 | sentinel `min-replicas-to-write` 가드 | replication mode 에서 `min-replicas-to-write` 동등 설정 가능 (`additionalConfig`) |
| 복구 시간 | sentinel-tilt threshold 기반 (~5-30s) | operator reconcile interval 기반 (~10-30s, RequeueAfter `requeueSteady`) |
| split-brain 방지 | sentinel quorum (>=3) | operator leader-elect (single leader, K8s Lease 기반) |
| client 디스커버리 | sentinel-aware client (sentinel address pool) | Service ClusterIP / DNS (`<name>.<ns>.svc.cluster.local`) |

**핵심 차이**: client 디스커버리. Sentinel-aware client (jedis / redisson /
go-redis sentinel mode) 를 *Service-aware client* 로 변경 의무.

## 마이그레이션 4 단계

### 단계 1 — 기존 Sentinel 인프라 평가

```bash
# Sentinel 인스턴스 식별
kubectl -n <ns> get pods -l app.kubernetes.io/component=sentinel
kubectl -n <ns> get svc <release>-sentinel

# 현재 master / replica 매핑
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel masters
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel slaves <master-name>

# RDB / AOF 사용 여부
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get save
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get appendonly
```

### 단계 2 — valkey-operator 설치 + Valkey CR 생성

```bash
# operator 설치 (Helm)
helm repo add keiailab https://keiailab.github.io/valkey-operator
helm install valkey-operator keiailab/valkey-operator -n valkey-operator-system --create-namespace

# 또는 manifest:
kubectl apply -f https://github.com/keiailab/valkey-operator/releases/latest/download/install.yaml
```

Valkey CR (Replication mode):
```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: my-cache
  namespace: data
spec:
  mode: Replication
  replicas: 3                        # primary 1 + replica 2 (sentinel quorum 동등)
  version: 9.0.4
  storage:
    size: 8Gi
    storageClassName: <fast-ssd>
  auth:
    enabled: true                    # ADR-0013 — 강제 (v1alpha1) 또는 v1alpha2 의 required *bool 토글
  monitoring:
    enabled: true
    serviceMonitor:
      enabled: true
  scalePolicy:
    deliberate: false                # auto failover 활성 (ADR-0006)
  additionalConfig: |
    # sentinel min-slaves-to-write 동등 — write 시 최소 1 replica ack
    min-replicas-to-write 1
    min-replicas-max-lag 10
```

### 단계 3 — 데이터 마이그레이션

#### 옵션 A: RDB import (downtime 허용)

```bash
# 1. Sentinel 측 master 에서 RDB dump
kubectl -n <ns> exec -it <sentinel-master-pod> -- redis-cli BGSAVE
kubectl -n <ns> cp <sentinel-master-pod>:/data/dump.rdb /tmp/migration.rdb

# 2. ValkeyRestore CR 로 복구 (ADR-0015 init-container 패턴)
kubectl -n data create configmap migration-rdb --from-file=dump.rdb=/tmp/migration.rdb
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: migrate-from-sentinel
  namespace: data
spec:
  sourceBackup: ...
  targetRef:
    name: my-cache
    kind: Valkey
EOF
```

> ⚠️ Redis 8.2.x → Valkey 9.0.4 RDB 호환성 검증 결과 *호환 불가*
> (RDB format version 12). 옵션 B 권장.

#### 옵션 B: 온라인 key copy (downtime 최소화)

```bash
# valkey-cli MIGRATE 또는 사용자 측 redis-shake / redis-port 도구
# 예: redis-shake (online sync, dual-write 가능)

# 1. valkey-shake config (source: 기존 Sentinel master, target: valkey-operator primary)
cat > shake.toml <<EOF
[source]
type = "standalone"
address = "<sentinel-master-svc>:6379"
password = "<old-password>"

[target]
type = "standalone"
address = "my-cache.data.svc.cluster.local:6379"
password = "<new-password>"

type = "sync"
EOF

# 2. shake 실행 (long-running)
redis-shake -c shake.toml
```

### 단계 4 — Client 변경 + Sentinel Decommission

#### Client 변경 (예: go-redis)

**Sentinel-aware (이전)**:
```go
client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{"sentinel-0:26379", "sentinel-1:26379", "sentinel-2:26379"},
    Password:      "<old-password>",
})
```

**Service-aware (이후)**:
```go
client := redis.NewClient(&redis.Options{
    Addr:     "my-cache.data.svc.cluster.local:6379",  // operator Service
    Password: "<new-password>",
})
```

failover 시 valkey-operator 가 Service endpoint 를 새 primary 로 자동
갱신 (Service selector + readiness probe). client 측 *재연결 retry* 만
필요 (대부분 client lib 가 자동 처리).

#### Decommission

```bash
# 1. client 트래픽 100% valkey-operator 로 전환 후 검증
kubectl -n data port-forward svc/my-cache 6379:6379
redis-cli ping  # PONG

# 2. 기존 Sentinel master/replica/sentinel 인스턴스 삭제
helm uninstall <existing-release> -n <ns>

# 3. PVC cleanup (정상 종료 확인 후)
kubectl -n <ns> delete pvc -l app.kubernetes.io/instance=<existing-release>
```

## 운영 검증 체크리스트

- [ ] Valkey CR `status.phase=Running` + `status.readyReplicas=replicas`
- [ ] `valkey-cli INFO replication` 에서 role=master / connected_slaves=N-1
- [ ] failover 시뮬레이션: primary pod delete → 30s 내 새 primary 선출 + Service endpoint 갱신
- [ ] data integrity: 이전 / 이후 GET 결과 일치 (sample 10K key)
- [ ] client 측 reconnect 동작: primary 변경 시 client error 5s 내 복구

## 참고

- ADR-0017: Replication Mode Failover — Replica with Largest `master_repl_offset` (Sentinel 거절 근거).
- ADR-0006: ScalePolicy.Deliberate=false 기본값 (auto failover 활성).
- ADR-0015: ValkeyRestore — Init Container 기반 RDB 로드.
