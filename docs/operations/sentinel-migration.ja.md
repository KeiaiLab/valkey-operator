# Sentinel → valkey-operator Replication mode 移行 runbook (日本語)

> English: [sentinel-migration.md](sentinel-migration.md) — canonical / 正本


> ADR-0017 (Replication Mode Failover) で Sentinel 採用を見送った決定に対する
> *外部ユーザー向け移行 path*。

## 背景

valkey-operator は ADR-0017 で Sentinel を意図的に見送り、同等の可用性を
*Replication mode + AutoFailover* (operator が管理する leader election +
STS rollout + `master_repl_offset` が最大の replica を選出) によって提供している。

本 runbook は *既存の Sentinel ベース Redis/Valkey 環境* から valkey-operator
へ移行する運用担当者向けの手順書である。

## 可用性の等価性

| 観点 | Sentinel HA (既存) | valkey-operator Replication + AutoFailover |
|---|---|---|
| failover の決定 | Sentinel quorum 投票 | operator leader election + ADR-0017 の largest `master_repl_offset` |
| データ無損失保証 | Sentinel `min-replicas-to-write` ガード | Replication mode で `additionalConfig` から同等設定が可能 |
| 復旧時間 | Sentinel tilt しきい値 (~5〜30 秒) | operator reconcile interval (~10〜30 秒、`RequeueAfter` `requeueSteady`) |
| split-brain 防止 | Sentinel quorum (≥ 3) | operator leader election (単一 leader、K8s `Lease`) |
| client ディスカバリ | Sentinel-aware client (Sentinel address pool) | Service ClusterIP / DNS (`<name>.<ns>.svc.cluster.local`) |

**最大の差分**: client ディスカバリ。Sentinel-aware client (jedis /
redisson / go-redis sentinel mode) は *Service-aware client* への移行が
必須となる。

## 4 ステップでの移行

### ステップ 1 — 既存 Sentinel インフラの棚卸し

```bash
# Sentinel インスタンスを特定
kubectl -n <ns> get pods -l app.kubernetes.io/component=sentinel
kubectl -n <ns> get svc <release>-sentinel

# 現状の master / replica マッピング
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel masters
kubectl -n <ns> exec -it <sentinel-pod> -- redis-cli -p 26379 sentinel slaves <master-name>

# 永続化設定の確認
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get save
kubectl -n <ns> exec -it <master-pod> -- redis-cli config get appendonly
```

### ステップ 2 — valkey-operator のインストールと Valkey CR 作成

```bash
# Helm でインストール
helm repo add keiailab https://keiailab.github.io/valkey-operator
helm install valkey-operator keiailab/valkey-operator -n valkey-operator-system --create-namespace

# あるいは manifest:
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
  replicas: 3                       # primary 1 + replica 2 (Sentinel quorum と等価)
  version: 9.0.4
  storage:
    size: 8Gi
    storageClassName: <fast-ssd>
  auth:
    enabled: true                   # ADR-0013 — v1alpha1 では強制、v1alpha2 では必須の toggle
  monitoring:
    enabled: true
    serviceMonitor:
      enabled: true
  scalePolicy:
    deliberate: false               # auto failover を有効化 (ADR-0006)
  additionalConfig: |
    # Sentinel の min-slaves-to-write と等価 — write には replica 1 つ以上の ack が必要
    min-replicas-to-write 1
    min-replicas-max-lag 10
```

### ステップ 3 — データ移行

#### 選択肢 A: RDB import (downtime 許容)

```bash
# 1. Sentinel 側の master から RDB を dump
kubectl -n <ns> exec -it <sentinel-master-pod> -- redis-cli BGSAVE
kubectl -n <ns> cp <sentinel-master-pod>:/data/dump.rdb /tmp/migration.rdb

# 2. ValkeyRestore で復元 (ADR-0015 init-container パターン)
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

> ⚠️ Redis 8.2.x → Valkey 9.0.4 の RDB 互換性は *非互換* と検証済み
> (RDB format version 12)。**選択肢 B を推奨する。**

#### 選択肢 B: オンライン key コピー (downtime 最小化)

```bash
# valkey-cli MIGRATE、もしくはユーザー側ツール
# (redis-shake / redis-port など) を利用する。
# 例: redis-shake (online sync, dual-write 対応)

# 1. valkey-shake config (source = 既存 Sentinel master, target = valkey-operator primary)
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

# 2. 実行 (long-running)
redis-shake -c shake.toml
```

### ステップ 4 — client 切り替えと Sentinel decommission

#### client 変更 (例: go-redis)

**Sentinel-aware (移行前)**:

```go
client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{"sentinel-0:26379", "sentinel-1:26379", "sentinel-2:26379"},
    Password:      "<old-password>",
})
```

**Service-aware (移行後)**:

```go
client := redis.NewClient(&redis.Options{
    Addr:     "my-cache.data.svc.cluster.local:6379",  // operator 管理の Service
    Password: "<new-password>",
})
```

failover 時、valkey-operator は Service endpoint を新しい primary に自動で
更新する (Service selector + readiness probe)。client 側は *エラー時に
reconnect する* だけでよい — ほとんどの client ライブラリはこれを透過的に
処理する。

#### Decommission

```bash
# 1. client トラフィックを valkey-operator へ 100% 切り替えたうえで検証
kubectl -n data port-forward svc/my-cache 6379:6379
redis-cli ping  # PONG

# 2. 既存の Sentinel master / replica / sentinel インスタンスを削除
helm uninstall <existing-release> -n <ns>

# 3. PVC cleanup (正常終了を確認後)
kubectl -n <ns> delete pvc -l app.kubernetes.io/instance=<existing-release>
```

## 運用検証チェックリスト

- [ ] Valkey CR が `status.phase=Running` かつ
      `status.readyReplicas == replicas`。
- [ ] `valkey-cli INFO replication` で `role=master`、
      `connected_slaves=N-1` を確認。
- [ ] failover ドリル: primary pod を delete → 30 秒以内に新 primary が
      選出され、Service endpoint が切り替わる。
- [ ] data integrity: 10K key のサンプルで移行前後の GET 結果が一致する。
- [ ] client reconnect: primary 変更後、client が 5 秒以内にエラーから復旧する。

## 参照

- ADR-0017 — Replication Mode Failover (`master_repl_offset` 最大の replica を
  採用)、Sentinel 不採用の根拠。
- ADR-0006 — `ScalePolicy.Deliberate=false` をデフォルト (auto failover ON)。
- ADR-0015 — `ValkeyRestore` の Init Container ベース RDB ロード。
