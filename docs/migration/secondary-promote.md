# Secondary-Promote Cutover

> ValkeyCluster 의 replica 를 primary 로 promote 후 기존 primary 폐기.
> 데이터 정합성 우선 패턴 — RDB snapshot 보다 *현재 in-flight write* 보존.

## 흐름

```
초기:  primary(A)  replica(B,C)
1단계: primary(A)  replica(B,C)  + sync 강제
2단계: primary(A)  replica(B,C)  + write 차단 (read-only)
3단계: primary(A)  replica(B,C)  + B promote → primary
4단계: replica(A)  primary(B)  replica(C)  (A demote)
5단계: A teardown
```

## 실행

### 1. Replica 정합 강제

```bash
kubectl -n <ns> exec <cluster>-shard-0-0 -- valkey-cli WAIT 1 5000
# 5초 내 1 replica 가 ack — 미달 시 retry
```

### 2. Write 차단

```bash
kubectl -n <ns> patch valkeycluster <cluster> --type=merge \
    -p '{"spec":{"writePolicy":"ReadOnly"}}'
# operator 가 client config map 갱신 + Pod ENV 전파
```

### 3. Replica promote

```bash
kubectl -n <ns> exec <cluster>-shard-0-1 -- valkey-cli REPLICAOF NO ONE
kubectl -n <ns> annotate pod <cluster>-shard-0-1 \
    valkey.keiailab.com/primary-override=true
# operator reconcile → Service selector 갱신
```

### 4. Old primary demote

```bash
kubectl -n <ns> exec <cluster>-shard-0-0 -- \
    valkey-cli REPLICAOF <cluster>-shard-0-1.<cluster>-headless 6379
```

### 5. Write 복원

```bash
kubectl -n <ns> patch valkeycluster <cluster> --type=merge \
    -p '{"spec":{"writePolicy":"ReadWrite"}}'
```

## Verify

- Step 1: `WAIT` 출력 ≥1
- Step 3: 새 primary 의 `INFO replication` role:master
- Step 5: 연속 write 100건 PASS

## Refs

- ROADMAP.md (P-C.3.2)
- `zero-downtime.md` (parent 흐름)
