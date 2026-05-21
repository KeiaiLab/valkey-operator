# Troubleshooting — valkey-operator (日本語)

> English: [troubleshooting.md](troubleshooting.md) — canonical / 正本


運用中の *症状 → 想定原因 → 診断コマンド → 復旧* の flow chart 形式。アラート別 MTTR
は `runbook.md §9` を SSOT とする — 本書は **アラートが発火しない (もしくは発火する
前段階の) 症状** を扱う。

問題が発生したら *症状の分類 → 本書の §X.Y → 診断 → 復旧* の順に活用する。

## 1. CR が作成されない、もしくは phase に進まない

### 1.1 `kubectl apply` 直後に CR 自体が拒否される

**症状**: `apply` コマンドが admission webhook エラーで reject される。

```sh
kubectl apply -f my-valkey.yaml
# Error: admission webhook "vvalkey-v1alpha1.kb.io" denied the request: ...
```

**想定原因と診断**:

| 原因 | 検証コマンド |
|---|---|
| webhook configuration 未設置 / cert 未準備 | `kubectl get validatingwebhookconfiguration -l app.kubernetes.io/name=valkey-operator` + `kubectl describe ...` |
| `Spec.TLS.CertManager` と `CustomCert` を同時に指定 | `kubectl explain valkey.spec.tls` (mutually exclusive — ADR-0010 webhook validation) |
| Valkey version が allowlist 外 | `internal/version/matrix.go::SupportedValkeyVersions` を確認 (現状 `8.0.9` / `8.1.6` / `8.1.7` / `9.0.4`) |
| ResourceQuota 違反 | `kubectl describe resourcequota -n <ns>` |

**復旧**: 上表の該当原因を解消したうえで再実行する。webhook 自体が不調なら
`runbook.md §6.1` の manager 強制再起動手順に従う。

### 1.2 CR は生成されたが phase が `Pending` から進まない

**症状**: `kubectl get vk` の `PHASE: Pending` が 5 分以上継続する。

**診断**:

```sh
kubectl describe vk <name>      # Events + Conditions
kubectl logs -n valkey-operator-system -l control-plane=controller-manager \
  --since=10m | grep "<ns>/<name>"
```

**頻出する根本原因**:

- AuthSecret が別 namespace の `Secret` を参照している → `Reason: SecretNotFound`。
  operator は namespace スコープ動作で、cross-namespace の Secret 参照は未サポート。
- StorageClass 未対応、もしくは `volumeBindingMode: WaitForFirstConsumer` で
  割り当て可能なノードがない → PVC が Pending → STS Pod が Pending →
  reconcile が進まない。
- `ENABLE_CLUSTER_RECONCILER=false` の状態で `kind: ValkeyCluster` を作成して
  しまった (chart の `features.cluster.enabled=false` 環境)。Helm install 時には
  ガードされるが、手書き CR の場合は黙ったまま放置される。

## 2. cluster_state=ok にもかかわらずデータプレーンが無応答

### 2.1 client 接続 timeout

**症状**: cluster は healthy なのに、application から connection refused / timeout
が返る。

**診断の順序**:

1. **service endpoints の確認**:
   ```sh
   kubectl get svc,endpointslices -l cache.keiailab.io/cluster=<name>
   ```
   `Endpoints: 0` は label selector の不一致 (operator が STS template を更新した
   際に patched-only label が漏れた状態)。reconcile を強制的に発火させる:
   `kubectl annotate vk <name> cache.keiailab.io/retry=true --overwrite`。

2. **NetworkPolicy による拒否確認** (ADR-0035 AutoCreate 有効時):
   ```sh
   kubectl get networkpolicy -l cache.keiailab.io/cluster=<name>
   kubectl describe networkpolicy <name>-allow
   ```
   ingress の `podSelector` が client pod のラベルと一致しなければ、トラフィックは
   既定で拒否される。

3. **TLS handshake 失敗** (TLS enabled cluster):
   ```sh
   kubectl exec -it <client-pod> -- valkey-cli -h <svc> -p 6379 --tls --insecure ping
   ```
   失敗時は `runbook.md §2.2` の Pod CrashLoopBackOff / TLS Secret 未マウント
   経路に従う。

### 2.2 一部の key だけ timeout する (Cluster mode)

**症状**: 同一 cluster で key A は成功するが、key B が MOVED もしくは timeout になる。

**原因**: いずれかの shard のプライマリが到達不能で、レプリカの自動昇格がまだ
完了していない。

```sh
# 1. 対象 key が属する slot
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER KEYSLOT <key>

# 2. その slot を保持する shard
kubectl exec -it <pod> -- valkey-cli -c -h <svc> CLUSTER SLOTS | grep <slot>

# 3. 当該 shard pod のログを確認
kubectl logs <shard-N-0>
```

**復旧**: 該当キーを保持するプライマリが到達不能な場合、ADR-0017 によって最大
オフセットを持つレプリカが自動的に昇格する。5 分経過しても回復しない場合は
`runbook.md §2.3` の `cluster_state=fail` 回復手順に従う。

## 3. 性能劣化 (latency 上昇)

### 3.1 reconcile thrashing

**症状**: operator pod が CPU 100% に張り付き、`valkey_cluster_reconcile_total` の
レートが急上昇する。

**診断**:

```sh
# Prometheus 上での rate
rate(valkey_cluster_reconcile_total[1m])
# > 5/s は thrashing

# エラーが集中している component の特定
sum by (component) (rate(valkey_cluster_reconcile_errors_total[5m]))
```

**頻出する根本原因**:

- Status 更新の無限ループ: operator が spec drift を検知 → patch → reconcile →
  再び同じ drift を検知。`metadata.generation` が変わらないのに reconcile が
  再突入している場合はこれを疑う。
- 外部依存の flapping: cert-manager の `Certificate` が `Ready` ↔ `NotReady`
  を行き来し、毎サイクル operator が再実行される。

**復旧**: `kubectl logs ... --follow` で reconcile 理由をトレースし、drift の
根本原因を解消する — 多くは admission webhook の mutating defaulter と
reconciler の desired state の不一致。

### 3.2 データプレーン応答 latency の上昇 (slow log)

**症状**: client p95 latency が平常時の ~1ms から ~100ms へ上昇する。

**想定原因** (operator のスコープ外だが診断可能):

| 原因 | 診断 |
|---|---|
| AOF rewrite 進行中 | `valkey-cli INFO persistence \| grep aof_rewrite_in_progress` |
| RDB snapshot 進行中 | `valkey-cli INFO persistence \| grep rdb_bgsave_in_progress` |
| 巨大キー (BIGKEY) の存在 | `valkey-cli --bigkeys` — 単一キーが 100MB+ だと slow O(N) コマンドが顕在化する |
| メモリ swap | `kubectl top pod <pod>` および node 上の `vmstat 1` |

ここでの tuning は **データプレーン** の判断であり、operator は直接介入しない。
`runbook.md §6.3` から `valkey-cli` に直接接続して調査する。

## 4. backup / restore の失敗

### 4.1 `ValkeyBackup phase=Failed`

```sh
kubectl describe vkb <name>     # Conditions の Reason
kubectl logs job/<backup-job-name>
```

**頻出する根本原因**:

- `TargetRef` の Secret 認証情報が無効 → S3 client が `403 SignatureDoesNotMatch`
  を返す。`kubectl describe vbt <target>` の `status.reachable` を確認する。
- bucket が存在しない → `404 NoSuchBucket`。`ValkeyBackupTarget` の
  `spec.s3.bucket` を検証する。
- PVC 容量不足 → `no space left on device`。RDB サイズの目安は `INFO memory` の
  `used_memory_rss`。

### 4.2 `ValkeyRestore` がいつまでも終わらない

ADR-B02 により RDB フォーマット不一致 (例: Redis 8.2.1 → Valkey 9.0.4) は
*fail-fast* で `Status.Phase=Failed` になる。それ以外で止まる典型パターン:

- Pod CrashLoopBackOff が 5 回以上 = restore コンテナが繰り返し失敗している。
  ログ中に `Can't handle RDB format version` 等の明示的な理由がないか確認する。
- `TargetRef` の RDB ダウンロードが timeout (大容量 + 低速回線) — Job の
  timeout spec を引き上げる。

## 5. 一般診断 cheat-sheet

```sh
# 全 CR の phase を一覧
kubectl get vk,vc,vkb,vbt,vkr -A \
  -o custom-columns=NS:.metadata.namespace,KIND:.kind,NAME:.metadata.name,PHASE:.status.phase

# 直近 5 分の reconcile 理由 (operator log)
kubectl -n valkey-operator-system logs -l control-plane=controller-manager --since=5m \
  | jq -R 'fromjson? | select(.msg)' | head -50

# Conditions の差分 (前回 vs 現在 status)
kubectl get vk <name> -o jsonpath='{.status.conditions}' | jq

# 最近の events
kubectl get events --field-selector involvedObject.name=<name> --sort-by='.lastTimestamp'
```

## 6. 最後の手段

- `runbook.md §6` の緊急手順 (manager の強制再起動、restore の強制中断、
  データプレーンへの直接介入)。
- `INC-0001` のような cluster-fail シナリオ → ADR-0039 self-heal が *大半* を
  自動回復する。自動回復しない場合は `runbook.md §2.3` の手動回復手順を実行する。

## 7. 問題が収束しないとき

同一の問題が 3 回以上再発する場合は、グローバル `incident-kb.md §2 trigger` に
従って `docs/kb/incident/INC-NNNN-*.md` を起こす。システム変更が必要なら
RFC → ADR → コード変更の正規ルートを踏む。
