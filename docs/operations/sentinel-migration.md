# Sentinel → valkey-operator Replication-mode migration runbook

> 한국어 버전: [sentinel-migration.ko.md](sentinel-migration.ko.md)

> Companion to Plan §4 PR-C7 (rejected) — the **external-user
> migration path** for ADR-0017 (Replication Mode Failover), which
> declined to ship Sentinel mode.

## Background

The ArtifactHub comparison (Plan §1 Phase 1) showed that both the
Bitnami Redis chart (v25.5.2) and the CloudPirates Redis chart
(v0.27.6) explicitly support **Sentinel HA**. valkey-operator
deliberately declined Sentinel (ADR-0017) and provides equivalent
availability through **Replication mode + AutoFailover**
(operator-managed leader election + STS rollout + selection of the
replica with the largest `master_repl_offset`).

This runbook is the operator guide for migrating an **existing
Sentinel deployment** onto valkey-operator.

## Availability equivalence

| Dimension | Sentinel (Bitnami / CloudPirates) | valkey-operator Replication + AutoFailover |
|---|---|---|
| Failover decision | Sentinel quorum vote | Operator leader election + ADR-0017 largest `master_repl_offset` |
| No-data-loss guarantee | Sentinel `min-replicas-to-write` guard | Equivalent setting in Replication mode via `additionalConfig` |
| Recovery time | Sentinel tilt threshold (~5–30 s) | Operator reconcile interval (~10–30 s, `RequeueAfter` `requeueSteady`) |
| Split-brain prevention | Sentinel quorum (≥ 3) | Operator leader election (single leader, K8s `Lease`) |
| Client discovery | Sentinel-aware client (Sentinel address pool) | Service ClusterIP / DNS (`<name>.<ns>.svc.cluster.local`) |

**Key difference**: client discovery. Sentinel-aware clients
(jedis / redisson / go-redis sentinel mode) must move to a
**Service-aware** client.

## 4-step migration

### Step 1 — Inventory the existing Sentinel infrastructure

```bash
# Identify Sentinel instances
kubectl -n <ns> get pods -l app.kubernetes.io/component=sentinel
kubectl -n <ns> get svc <release>-sentinel

# Current master / replica mapping
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel masters
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel slaves <master-name>

# Persistence settings in use
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get save
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get appendonly
```

### Step 2 — Install valkey-operator and create the Valkey CR

```bash
# Helm install
helm repo add keiailab https://keiailab.github.io/valkey-operator
helm install valkey-operator keiailab/valkey-operator -n valkey-operator-system --create-namespace

# Or via manifest:
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
  replicas: 3                       # 1 primary + 2 replicas (Sentinel quorum equivalent)
  version: 9.0.4
  storage:
    size: 8Gi
    storageClassName: <fast-ssd>
  auth:
    enabled: true                   # ADR-0013 — forced in v1alpha1, *required toggle in v1alpha2
  monitoring:
    enabled: true
    serviceMonitor:
      enabled: true
  scalePolicy:
    deliberate: false               # auto failover ON (ADR-0006)
  additionalConfig: |
    # Sentinel min-slaves-to-write equivalent — write requires ≥ 1 replica ack
    min-replicas-to-write 1
    min-replicas-max-lag 10
```

### Step 3 — Data migration

#### Option A: RDB import (downtime acceptable)

```bash
# 1. Dump RDB on the Sentinel-side master
kubectl -n <ns> exec -it <bitnami-master> -- redis-cli BGSAVE
kubectl -n <ns> cp <bitnami-master>:/data/dump.rdb /tmp/migration.rdb

# 2. Restore via ValkeyRestore (ADR-0015 init-container pattern)
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

> ⚠️ Bitnami Redis 8.2.x → Valkey 9.0.4 RDB compatibility was
> verified incompatible on 2026-05-07 (RDB format version 12,
> see HANDOFF.md). **Option B is recommended.**

#### Option B: Online key copy (minimal downtime)

```bash
# Use valkey-cli MIGRATE, or a user-side tool such as
# redis-shake / redis-port.
# Example: redis-shake (online sync, dual-write capable)

# 1. valkey-shake config (source = Bitnami master, target = valkey-operator primary)
cat > shake.toml <<EOF
[source]
type = "standalone"
address = "<bitnami-master-svc>:6379"
password = "<old-password>"

[target]
type = "standalone"
address = "my-cache.data.svc.cluster.local:6379"
password = "<new-password>"

type = "sync"
EOF

# 2. Run (long-running)
redis-shake -c shake.toml
```

### Step 4 — Client cut-over + Sentinel decommission

#### Client change (example: go-redis)

**Sentinel-aware (before)**:

```go
client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{"sentinel-0:26379", "sentinel-1:26379", "sentinel-2:26379"},
    Password:      "<old-password>",
})
```

**Service-aware (after)**:

```go
client := redis.NewClient(&redis.Options{
    Addr:     "my-cache.data.svc.cluster.local:6379",  // operator-managed Service
    Password: "<new-password>",
})
```

On failover, valkey-operator updates the Service endpoint to the
new primary automatically (Service selector + readiness probe).
The client only needs to **reconnect on error** — most client
libraries do this transparently.

#### Decommission

```bash
# 1. Switch 100 % of client traffic to valkey-operator, then verify
kubectl -n data port-forward svc/my-cache 6379:6379
redis-cli ping  # PONG

# 2. Delete the Sentinel + Bitnami master/replica instances
helm uninstall <bitnami-release> -n <ns>

# 3. PVC cleanup (after a clean shutdown)
kubectl -n <ns> delete pvc -l app.kubernetes.io/instance=<bitnami-release>
```

## Operational verification checklist

- [ ] Valkey CR `status.phase=Running` and
      `status.readyReplicas == replicas`.
- [ ] `valkey-cli INFO replication` shows `role=master`,
      `connected_slaves=N-1`.
- [ ] Failover drill: delete the primary pod → a new primary is
      elected and the Service endpoint updates within 30 s.
- [ ] Data integrity: GET results match before and after on a
      10 K-key sample.
- [ ] Client reconnect: after a primary change, the client
      recovers from errors within 5 s.

## References

- ADR-0017 — Replication Mode Failover (replica with the largest
  `master_repl_offset`); Sentinel rejection rationale.
- ADR-0006 — `ScalePolicy.Deliberate=false` default (auto failover
  ON).
- ADR-0015 — `ValkeyRestore` Init Container-based RDB load.
- Plan §1 Phase 1 — Bitnami v25.5.2 / CloudPirates v0.27.6
  comparison; Sentinel availability equivalence.
- External tools:
  - redis-shake: <https://github.com/tair-opensource/RedisShake>
  - valkey-cli MIGRATE: <https://valkey.io/commands/migrate/>
