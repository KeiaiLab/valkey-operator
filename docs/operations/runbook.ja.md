# 運用 Runbook — valkey-operator (日本語)

> English: [runbook.md](runbook.md) — canonical / 正本

障害対応と日常運用の手順書。主要シナリオに絞って記載しており、詳細は各 ADR / Issue を参照のこと。

## 1. ヘルスチェック

```sh
# operator pod の状態
kubectl -n valkey-operator-system get pods -l control-plane=controller-manager

# 全 CR の phase
kubectl get vk,vc,vkb,vbt,vkr -A

# operator メトリクス (HTTPS:8443)
kubectl -n valkey-operator-system port-forward \
  svc/valkey-operator-controller-manager-metrics-service 8443:8443
curl -k https://localhost:8443/metrics | grep valkey_cluster_state_ok
```

## 2. 一般障害対応

### 2.1 CR の `Phase=Failed`

```sh
kubectl describe vk <name>     # Status.Conditions の Reason / Message
kubectl get events --field-selector involvedObject.name=<name>
```

手順:
1. `Reason` を分類する (TargetNotFound / AuthSecret / ConfigMap /
   TLS / …)。
2. 原因を解消したうえで CR を再作成するか、
   `kubectl annotate ... cache.keiailab.io/retry=true`
   で再 reconcile を発火する。

### 2.2 Pod の CrashLoopBackOff

```sh
kubectl logs <pod> -p          # 直前のコンテナログ
kubectl logs <pod> -c valkey
```

頻出する根本原因:

- TLS Secret がマウントされていない → ADR-0014 を参照
  (`/tls/tls.crt: No such file`)。
- Auth パスワード不一致 → Auth Secret を再生成する (CR を delete
  → recreate)。

### 2.3 `ValkeyCluster cluster_state=fail`

**自動修復 (ADR-0039, 2026-05-10)**: operator は
`ClusterInitialized=true` の状態で
`cluster_state != ok` (または `slots != 16384`) を検知すると、
自動的に `ensureClusterMeet` を再呼び出しする。本節は自動修復が
*失敗* した場合 (5 分以上スタックし、人手介入が必要なケース) を
扱う。

```sh
PASS=$(kubectl get secret <name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster info
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster nodes
```

#### 診断の順序

1. **Pod は Ready か**:
   `kubectl get pod -n <ns> -l app.kubernetes.io/instance=<name>`。
   いずれかの pod が `Running 2/2` でなければ、そちらを先に修復する
   (PVC pending、NetworkPolicy による拒否など)。
2. **operator ログ**:
   `kubectl logs -n <op-ns> deploy/<op-name>` を確認し、
   *INC-0001 self-heal* 試行のログ行を探す:
   ```
   ValkeyCluster post-init fail detected; attempting re-bootstrap (INC-0001 self-heal)
     state=fail slotsAssigned=0 slotsOK=0
   ```
   このメッセージが 30 分以上繰り返している場合、自動修復は失敗
   している → 手順 3 へ進む。
3. **`nodes.conf` の `myself` IP 確認**: 各 pod の
   `/data/nodes.conf` 内 `myself` 行の IP が、実際の pod IP
   (`kubectl get pod -o wide`) と一致するか確認する。不一致なら
   INC-0001 の再発 → 手順 4 へ。

#### 手動復旧 (INC-0001 パターン)

まずデータ損失の影響を評価する:

```sh
# pod 毎の key 数
for i in 0 1 2 3 4 5; do
  echo "pod-$i: $(kubectl exec <name>-$i -- valkey-cli -a "$PASS" dbsize | tail -1)"
done

# key のサンプル取得 (本番データかどうかを判別)
kubectl exec <name>-0 -- valkey-cli -a "$PASS" --scan | head -20
```

**本番データが存在する場合**: 復旧の **前に** 必ずバックアップを
取得する (`make backup` または `valkey-cli BGSAVE`)。`ValkeyBackup`
CR を使う方法が推奨される。

**テストデータのみ (または損失許容) の場合**:

```sh
# 1. 6 つの pod 全てで PVC を初期化する (AOF + nodes.conf)
for i in 0 1 2 3 4 5; do
  kubectl exec <name>-$i -- sh -c 'rm -rf /data/appendonlydir /data/nodes.conf /data/dump.rdb'
done

# 2. 全 pod を同時に再起動する (controller が STS を再生成する)
kubectl delete pod <name>-0 <name>-1 <name>-2 <name>-3 <name>-4 <name>-5 --wait=false

# 3. ClusterInitialized=false を強制 patch (controller の bootstrap 経路に再進入)
kubectl patch valkeycluster <name> --type=json --subresource=status \
  -p='[{"op":"replace","path":"/status/clusterInitialized","value":false},
       {"op":"replace","path":"/status/shards","value":[]},
       {"op":"replace","path":"/status/clusterState","value":""}]'

# 4. spec を変更して reconcile イベントを発火させる
kubectl patch valkeycluster <name> --type=merge \
  -p '{"spec":{"nodeTimeoutMillis":15001}}'

# 5. 60 秒待機後に検証
kubectl exec <name>-0 -- valkey-cli -a "$PASS" cluster info
# 期待値: cluster_state:ok, cluster_slots_assigned:16384, cluster_slots_ok:16384
```

#### 参考資料

- INC-0001:
  `docs/kb/incident/INC-0001-cluster-fail-bootstrap-skip.md`
  (2026-05-09、19 時間障害)。
- ADR-0039: `docs/kb/adr/0039-cluster-self-heal-post-init.md`
  (恒久対処)。
- Alert: PrometheusRule `ValkeyClusterStateNotOK`
  (`for: 5m`、severity critical) — 本節の入口となるアラート。

## 3. バックアップ / リストア

### 3.1 日次バックアップ (PVC 保持)

```sh
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-$(date +%Y%m%d), namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  type: RDB
  retainPVC: true
  ttl: 168h        # 7 日
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeybackup/vkb-...
```

### 3.2 外部ストレージへのバックアップ (S3)

```sh
# 事前準備: ValkeyBackupTarget + 認証情報 Secret を作成
# (config/samples/ 参照)
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyBackup
metadata: { name: vkb-s3-$(date +%Y%m%d), namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  destination:
    type: TargetRef
    targetRef:
      name: s3-prod
      path: $(date +%Y/%m/%d)/dump.rdb
  ttl: 720h        # 30 日
EOF
```

### 3.3 リストア (災害復旧)

**注意**: `ValkeyRestore` は対象クラスタの既存データを **上書き**
する。事前に独立したバックアップを取得しておくこと。

```sh
# Standalone Valkey、PVC ソース
kubectl apply -f - <<EOF
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata: { name: vkr-recovery, namespace: default }
spec:
  clusterRef: { kind: Valkey, name: valkey-prod }
  source:
    pvc:
      name: vkb-20260506-backup
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Completed valkeyrestore/vkr-recovery --timeout=10m

# 検証
kubectl get vkr vkr-recovery -o jsonpath='{.status.restoredKeys}'
```

進行状況のモニタリングは
`kubectl get vkr vkr-recovery -o jsonpath='{.status.phase}'` で行う →
Pending → Mounting → Restoring → Verifying → Completed。

## 4. スケーリング

### 4.1 レプリケーションのスケール (replicas N → M)

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"replicas":5}}'
# operator が新しい STS replicas を適用する。新規レプリカは
# master_link_status が up を報告するまで待機する。
```

### 4.2 ValkeyCluster のシャード拡張

未実装 — ROADMAP の Phase B (Track B) を参照。手動手順:

```sh
# 新規 shard pod を作成し、CLUSTER MEET と reshard を手動で実行する。
# 運用ガイドは別途管理。
```

## 5. アップグレード

### 5.1 Valkey バージョンアップグレード

```sh
kubectl patch vk valkey-prod --type=merge -p '{"spec":{"version":{"version":"8.1.7"}}}'
# operator が Phase=Upgrading に遷移し、STS の rolling restart を
# 実行する。Replication モードでは レプリカ → プライマリの順に
# 実行されるため、master_link_status を随時監視すること。
```

### 5.2 operator バージョンアップグレード

`make deploy IMG=...` または Helm chart (別コミットで管理)。

## 6. 緊急対応

### 6.1 operator manager の強制再起動

```sh
kubectl -n valkey-operator-system rollout restart deploy/valkey-operator-controller-manager
```

### 6.2 誤発火した `ValkeyRestore` の中断

```sh
# Restore は STS に init container を追加し、paused アノテーションを
# 設定する。operator が自己復旧できない場合の手動クリーンアップ:
kubectl delete vkr <name>                                # finalizer が STS を巻き戻し、paused を解除する
# finalizer 自身が固まっている場合 (稀):
kubectl patch vkr <name> -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl annotate vk <target> cache.keiailab.io/paused-     # paused アノテーションを手動削除
kubectl edit sts <target>                                  # init container "valkey-restore-init" を削除
```

### 6.3 データプレーンへの直接アクセス

```sh
PASS=$(kubectl get secret <cr-name>-auth -o jsonpath='{.data.password}' | base64 -d)
kubectl exec -it <cr-name>-0 -- valkey-cli -a "$PASS"
# TLS 有効時: valkey-cli --tls --cacert /tls/ca.crt --cert /tls/tls.crt --key /tls/tls.key -p 6380
```

## 7. 可観測性の規約

- **メトリクス** (subsystem `valkey_cluster_*`): `state_ok`,
  `assigned_slots`, `shards`, `ready_replicas`, `reconcile_total`,
  `reconcile_errors_total`, `phase`, `backup_total`,
  `restore_total`, `failover_total`, `build_info` (cycle 57)。
  `Spec.Monitoring.ServiceMonitor.Enabled` が有効なら
  ServiceMonitor が自動登録される。
- **イベント**: `kubectl get events --field-selector involvedObject.kind=Valkey`。
- **ログ**: 構造化ログ (`zap`)。
  `kubectl logs <operator-pod> -f --tail=100`。

### 7.0 Prometheus ServiceMonitor TLS — 本番強化 (cycle 100)

**既定値**: `config/prometheus/monitor.yaml` の ServiceMonitor は
Kubebuilder のデフォルトに従い `insecureSkipVerify: true` で
出荷される。**本番ではこれは MITM 攻撃の対象面となる。**
cert-manager 検証を有効化する手順:

```sh
# 1. 事前に cert-manager をクラスタ全体にインストールする。
# 2. config/prometheus/kustomization.yaml の patches ブロックを uncomment する。
sed -i '' 's|^#patches:|patches:|; s|^#  - path: monitor_tls_patch.yaml|  - path: monitor_tls_patch.yaml|; s|^#    target:|    target:|; s|^#      kind: ServiceMonitor|      kind: ServiceMonitor|' \
  config/prometheus/kustomization.yaml
# 3. config/default/kustomization.yaml の [METRICS WITH CERTMANAGER] パッチも uncomment する。
# 4. make build-installer または make deploy で再デプロイする。
```

検証完了後、`monitor_tls_patch.yaml` は `insecureSkipVerify: false`
へ切り替わり、cert-manager が発行した `metrics-server-cert` Secret
を参照することで **検証可能な相互 TLS** を実現する。これは
ADR-0003 が定める「TLS InsecureSkipVerify 暫定対応」を本番品質に
昇格させた状態である。

### 7.1 operator 環境変数 (cycle 80)

該当クラスタで *どの reconciler が実際に動いているか* を診断する
ために用いる:

| 環境変数 | 既定値 | 効果 |
|---|---|---|
| `ENABLE_CLUSTER_RECONCILER` | `true` | `false` で ValkeyClusterReconciler を無効化 — chart の `features.cluster.enabled=false` 時に自動注入される。 |
| `ENABLE_BACKUP_RECONCILER` | `true` | `false` で ValkeyBackup / BackupTarget / Restore reconciler を無効化 — chart の `features.backup.enabled=false` 時に自動注入される。 |
| `ENABLE_WEBHOOKS` | `true` | `false` で ValkeyWebhook + ValkeyClusterWebhook の登録を無効化する。**envtest 専用** であり本番では絶対に設定しないこと。 |
| `WATCH_NAMESPACES` | 未設定 (cluster-wide) | カンマ区切りリスト (`ns1,ns2`)。`cache.DefaultNamespaces` に渡される — chart の `watch.namespaces` から自動注入される。 |
| `OPERATOR_IMAGE` | `controller:latest` | Upload/Download Job が利用するイメージ — chart の `valkey-operator.image` ヘルパーから自動注入される (cycle 64)。未設定だと ImagePullBackOff のリスクがある。 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 未設定 (no-op) | OTLP gRPC エンドポイント — chart の `tracing.endpoint` から自動注入される (cycle 65)。未設定時は 22 個の span がゼロコストになる。 |
| `OTEL_SERVICE_NAME` | `valkey-operator` | OTEL サービス識別子 — chart の `tracing.serviceName` から自動注入される。Jaeger / Tempo UI の service id として利用される。 |

**注意**: `ENABLE_*_RECONCILER` と `ENABLE_WEBHOOKS` は
**大文字小文字を区別する**。無効化する場合は小文字の `"false"` のみ
が有効である。`"FALSE"` や `"False"` などの別表記、あるいは
`"0"` や `"no"` といった他の「偽値」は Kubebuilder の慣例に従い
**有効** と解釈される。

診断用コマンド:

```sh
# 現在の環境変数
kubectl exec -n valkey-operator-system <operator-pod> -- env | \
  grep -E "ENABLE_|WATCH_NAMESPACES|OPERATOR_IMAGE|OTEL_"

# 起動ログ (スキップされた reconciler、watch スコープ、バージョン)
kubectl logs -n valkey-operator-system <operator-pod> | head -20
```

## 8. ADR / RFC 参照

- ADR-0010 cert-manager 自動検出 / ADR-0013 Auth 常時有効化 /
  ADR-0014 TLS ボリュームマウント
- ADR-0015 Restore init-container パターン / ADR-0016
  `ValkeyBackupTarget`
- ADR-0022 minio-go / ADR-0023 サブコマンドパターン
- ADR-0045 GH Actions 復旧 / ADR-0046 SLSA-3 + cosign

完全な INDEX: `docs/kb/adr/INDEX.md`。

## 9. アラート別対応 (Prometheus アラート → MTTR)

各アラートの `runbook_url` アノテーションは本節を指す。オンコール
担当は **Trigger → Diagnosis → Mitigation → Escalation** の流れで
対応する。

### 9.1 ValkeyClusterStateNotOK
- **Trigger**: `valkey_cluster_state_ok == 0` が 5 分継続。`CLUSTER INFO` の `cluster_state` ≠ ok。
- **自動修復 (ADR-0039)**: operator は `ClusterInitialized=true`
  の状態でも `ensureClusterMeet` を再呼び出しする。operator ログ
  の「INC-0001 self-heal」行を確認すること。5 分の評価窓内に
  復旧しなければ自動修復も失敗している状態 → 手動対応に切り替える。
- **Diagnosis**: §2.3 (「ValkeyCluster cluster_state=fail」)
  の手順 (診断順 + 手動復旧) をそのまま実施する。
- **Mitigation**:
  1. 欠落している slot を特定し `CLUSTER ADDSLOTS` を実行する
     (5 分以内の復旧を期待)。
  2. `nodes.conf` が陳腐化していれば §2.3 の「手動復旧」手順
     (PVC wipe + `clusterInitialized` リセット) を実施する。
     データを必ず先にバックアップすること。
- **References**: INC-0001、ADR-0039。

### 9.2 ValkeyClusterSlotsMismatch
- **Trigger**: `valkey_cluster_assigned_slots != 16384` が 5 分継続。
- **Diagnosis**: `valkey-cli cluster nodes` で slot の分布を確認する。
  resharding の途中である可能性 (一時的事象)。
- **Mitigation**: 5 分以上継続する場合は手動で `CLUSTER ADDSLOTS`
  を実行するか operator を再起動する。

### 9.3 ValkeyClusterNoReadyReplicas
- **Trigger**: `valkey_cluster_ready_replicas == 0` が 5 分継続。全 pod が NotReady。
- **Diagnosis**: §2.2 (CrashLoopBackOff) に加え、ノードレベルの
  シグナル (ディスク逼迫など) を確認する。
  `kubectl get pods -l app.kubernetes.io/name=valkey` + describe。
- **Mitigation**: PVC の再 bind、image pull の問題、OOMKilled など、
  根本原因に応じて §2 の手順を適用する。
- **Escalation**: クラスタノード自体がダウンしている場合は、
  ノードを増設するか別ノードへ再スケジュールする。

### 9.4 ValkeyClusterDegraded
- **Trigger**: `0 < ready_replicas < 2` が 5 分継続。一部の pod が NotReady。
- **Diagnosis**: NotReady な各 pod のログとイベントを確認する。
- **Mitigation**: 通常は §2.2 のパターン。

### 9.5 ValkeyClusterPhaseFailed
- **Trigger**: `valkey_cluster_phase{phase="Failed"} == 1` が 1 分継続。
- **Diagnosis**: §2.1 (「Phase=Failed CR」)。Conditions の
  `LastError` を確認する。
- **Mitigation**: エラー内容に応じて対処する (典型的には admission、
  RBAC、StorageClass の問題)。

### 9.6 ValkeyOperatorReconcileErrorsHigh
- **Trigger**: `rate(valkey_cluster_reconcile_errors_total[5m]) > 0.1` が 5 分継続。
- **Diagnosis**: operator ログを `level=error` で grep し、
  kubectl events も合わせて確認する。RBAC、API サーバ負荷、CR
  バリデーション拒否などが典型。
- **Mitigation**: 一時的なものは自己回復する。継続する場合は §6.1
  (operator 再起動) を実施する。

### 9.7 ValkeyOperatorDown
- **Trigger**: `up{job=~"valkey-operator.*"} == 0` が 2 分継続。
- **Diagnosis**: §6.1 (「operator manager 強制再起動」)。
  Deployment Available の状態、Pod 状態、ノード状態を確認する。
- **Mitigation**: §6.1 に従い rollout restart する。
  `ImagePullBackOff` の場合は image を確認する。
- **Escalation**: 全 reconcile が停止する — 新規 CR も Phase 遷移も
  発生しない。SEV-1 扱い。

### 9.8 ValkeyBackupFailureRateHigh
- **Trigger**: `rate(valkey_cluster_backup_total{phase="Failed"}[1h]) > 0.0017` (時間あたり約 6 件) が 10 分継続。
- **Diagnosis**: §3 (「バックアップ / リストア」)。Failed 状態の
  `ValkeyBackup` の `LastError` と、Job / Upload Pod のログを確認
  する。認証情報、S3 バケットの権限、ディスク容量が代表的な
  原因。
- **Mitigation**: 認証情報をローテーションするか、`BackupTarget`
  のエンドポイントを変更する。保持ポリシー (TTL) への影響を
  評価したうえで再実行する。

### 9.9 ValkeyRestoreFailureRateHigh
- **Trigger**: `rate(valkey_cluster_restore_total{phase="Failed"}[1h]) > 0.0017` が 10 分継続。
- **Diagnosis**: §3.3 (「リストア」)。ソース RDB の完全性、init
  container のログ、PVC の ROX マウントを確認する。
- **Mitigation**: §6.2 で誤発火した Restore を中断したうえで再実行
  する。Failed 状態の Restore CR は finalizer のクリーンアップ後に
  削除する。

### 9.10 ValkeyFailoverHigh
- **Trigger**: `rate(valkey_cluster_failover_total[1h]) > 0.005` (時間あたり約 18 件) が 10 分継続。
- **Diagnosis**: failover の頻発はプライマリ不安定のサイン。
  プライマリの OOMKilled、ネットワーク分断、レプリケーション遅延
  を確認する。`valkey-cli info replication`。
- **Mitigation**: リソース上限の調整、ネットワークポリシーの監査、
  プライマリの負荷分散 (read replica の活用)。ディスク I/O の
  ボトルネックも確認する。
- **Escalation**: split-brain が疑われる場合は §2.3 の手順に加え、
  ADR-0017 も精査する。

### 9.11 ValkeyOperatorReconcileLatencyP95High
- **Trigger**: reconcile 成功時の p95 > 1 秒が 10 分継続。
- **Diagnosis**: クラスタ API サーバ負荷、operator pod の CPU
  スロットリング、reconciler 内の外部呼び出しタイムアウトを疑う。
  CPU は `kubectl top pod -n valkey-operator-system` で確認。
- **Mitigation**: operator pod の `resources.requests.cpu` を
  引き上げるか、他コントローラの API バーストの影響から隔離する。

### 9.12 ValkeyOperatorReconcileLatencyP99Critical
- **Trigger**: reconcile (success + error) の p99 > 5 秒が 10 分継続。
  controller-runtime の既定 context タイムアウト (30 秒) に
  危険なほど近い水準。
- **Diagnosis**: 9.11 と同様だが **深刻な飽和** 状態である。
  operator pod の状態と、`reconcile_errors_total` の
  `component` ラベル分布を確認する。
- **Mitigation**: operator pod を即時再起動する
  (`kubectl rollout restart deploy/valkey-operator-controller-manager -n valkey-operator-system`)。

### 9.13 ValkeyOperatorReconcileErrorRateHigh
- **Trigger**: reconcile のエラー率が 5 % 超で 10 分継続。
- **Diagnosis**: `valkey_cluster_reconcile_errors_total` を
  `component` ラベル (secret / sts / svc / tls / backup) ごとに
  分解し、どの段階で失敗しているかを特定する。
- **Mitigation**: §2.1 の「Phase=Failed CR」手順を適用する
  (Conditions の Reason で分類)。

## 10. Replication モードの自動 Failover

ADR-0017 (replication failover — 最大オフセットを持つレプリカへ
昇格)。Failover の判断は operator が一元的に保持し、Sentinel
サイドカーは存在しない。

### 10.1 起動条件

- `Spec.Mode == Replication` かつ `Spec.Replicas > 1`
- `Spec.AutoFailover == true` (既定値 — `false` で無効化)

### 10.2 アルゴリズム (7 ステップ)

1. 現在のプライマリ (`Status.CurrentPrimary` または `<name>-0`) を
   読み取り、pod の Ready 状態を確認する。
2. `NotReady` が 30 秒未満であれば一時的な揺らぎとみなして無視する。
3. プライマリを除く全レプリカに対して `INFO replication` を発行し、
   `master_repl_offset` を取得する。
4. `selectFailoverCandidate` が最大オフセットを持つレプリカを
   選出する (同値の場合は ordinal の小さい方を優先)。
5. 候補に対して `PromoteToPrimary` が `REPLICAOF NO ONE` を発行し、
   新プライマリとする。
6. 他の `Ready` レプリカ全てに対し `EnsureReplicaOf <new-primary>`
   を発行する。
7. `Status.CurrentPrimary` をメモリ上で更新する。次回 reconcile
   で CR に永続化される。

各フェーズの OTel span: `Failover/INFO_replication`、
`Failover/PromoteToPrimary`、`Failover/EnsureReplicaOf_all`
(詳細は [`observability/otel.md`](../observability/otel.md))。

### 10.3 無効化

```yaml
spec:
  autoFailover: false   # operator は failover 経路をスキップする (手動復旧のみ)
```

### 10.4 既知の制約

- **ネットワーク分断時の split-brain を厳密には防げない** — 2 つの
  プライマリが選出されうる。緩和策: 30 秒の `NotReady` 閾値と
  operator の leader election (シングルレプリカ運用)。
- **ValkeyCluster モードでは適用外** — Valkey ネイティブの
  クラスタモードは `cluster_replica_validity_factor` を使う。
  `ValkeyClusterReconciler` は意図的に failover に関与しない。
- **e2e 自動化は未整備** — replication-failover の e2e は別 cycle
  扱い。

## 11. 実機検証済みの運用シナリオ

リリース検証作業中に確認した動作の記録。各行はライブ環境の
operator に対して実行したシナリオであり、「動作」列に観測結果を
記載している。

| シナリオ | 動作 | データ |
|---|---|---|
| プライマリ pod の強制 kill | STS が再生成され、operator が pod-0 を再昇格 | PVC 保持 |
| レプリカのスケールアップ (3 → 5) | 新レプリカが自動参加し `master link up` | — |
| レプリカのスケールダウン (5 → 2) | 余剰 pod がクリーンアップされる | 既存データ保持 |
| ValkeyCluster shard pod の kill | `cluster_state=ok` を維持 (レプリカが引き継ぐ) | 全 slot 保持 |
| TLS + mTLS ValkeyCluster (cert-manager) | `Phase=Running`、`slots=16384`、データプレーン SET/GET ✓ | — |
| TLS + mTLS Valkey Standalone (cert-manager) | `Phase=Running`、ポート 6380 で ping/set/get ✓ | — |
| TLS + mTLS Valkey Replication (3 replicas) | `master_link_status:up`、レプリカ間で書き込み伝播 ✓ | — |
| ValkeyBackup (RDB) | Pending → InProgress → Completed、`/data/dump.rdb` 89 B 生成 | — |
| ValkeyBackup M3.5 (Job ベース PVC) | Copying → Completed、`<name>-backup` PVC に `dump.rdb` を保持 | TLS が自動伝播 |
| ValkeyRestore (Standalone PVC) | Mounting → Restoring → Verifying → Completed、init container が `/data/dump.rdb` を cp | `Status.RestoredKeys` を更新 |
| ValkeyRestore (`Source.TargetRef` S3) | 一時 PVC + Download Job → 通常の init-container 経路 | クラスタ間リストア |
| ValkeyRestore (ValkeyCluster、ROX) | shard ordinal → `SHARD_IDX` のシェルマッピング → pod ごとの cp | ROX ソース PVC が必須 |
| Replication 自動 failover (ADR-0017) | プライマリの `NotReady` 30 秒以上 → 最大オフセットのレプリカへ `REPLICAOF NO ONE` → `Status.CurrentPrimary` 更新 | e2e: `test/e2e/failover_test.go` |
| NetworkPolicy リソース生成 | selfPeer + 6379 / 16379 ingress + `ownerReferences` | (CNI 依存) |
| operator メトリクスエンドポイント (HTTPS:8443) | `controller_runtime_*` + `valkey_cluster_*` を公開 | — |
| Prometheus アラートルール | 13 アラート (state / slots / replicas / phase / errors / latency / operator down) | `config/prometheus/alert-rules.yaml` |
