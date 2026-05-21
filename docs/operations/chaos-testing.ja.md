# Chaos testing — valkey-operator (日本語)

> English: [chaos-testing.md](chaos-testing.md) — canonical / 正本

ADR-0041 の chaos-mesh ベース 4 シナリオの chaos engineering e2e スイート実行手順。

## 事前準備

1. **Kind cluster** (あるいは任意の Kubernetes) を起動する:
   ```sh
   make setup-test-e2e   # または: kind create cluster --name valkey-e2e
   ```

2. **valkey-operator を deploy する**:
   ```sh
   make docker-build IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   make deploy IMG=ghcr.io/keiailab/valkey-operator:e2e-dev
   ```

3. **chaos-mesh を install する**:
   ```sh
   make chaos-mesh-install
   # 手動: kubectl apply -f https://mirrors.chaos-mesh.org/v2.7.2/chaos-mesh.yaml
   ```

4. **対象の ValkeyCluster** (namespace `valkey-chaos-e2e`):
   ```yaml
   apiVersion: cache.keiailab.io/v1alpha1
   kind: ValkeyCluster
   metadata: { name: vc-chaos, namespace: valkey-chaos-e2e }
   spec:
     shards: 3
     replicasPerShard: 1
     autoFailover: true
     version: { version: "9.0.4" }
   ```

## 実行

```sh
make chaos-e2e
# 対象 namespace を上書きする場合:
CHAOS_TEST_NAMESPACE=my-ns make chaos-e2e
```

## シナリオ (4 種)

| ID | Chaos 種別 | 動作 | 回復検証 |
|---|---|---|---|
| 1 | PodChaos (pod-kill) | 5 分間、1 分ごとに random pod kill | `cluster_state=ok` が 5 分以内に回復 |
| 2 | NetworkChaos (partition) | master ↔ replica の 30 秒分断 | 3 分以内に failover または回復 |
| 3 | IOChaos (`ENOSPC` fault) | disk 80% 充填を 60 秒間シミュレート | 3 分以内に cluster degraded but healthy |
| 4 | IOChaos (latency) | replica の I/O に 100 ms 遅延を 60 秒間付与 | master は影響を受けない (failover が発生しない) ことを 3 分以内に確認 |

各シナリオは chaos CR の適用 → 時間経過 → 自動 cleanup → cluster の healthy 回復を
検証する。`BeforeSuite` で作成された `vc-chaos` CR は全シナリオを通じて保持される。

## 運用への組み込み

- **開発者ローカル**: reconciler の変更後に実行を推奨する
  (full e2e + chaos ≈ 30 分)。
- **CI nightly**: ADR-0041 AI-005 (別 follow-up) — CI インフラ作業の着地後に自動化する。
- **production debug**: chaos-mesh を production で直接実行することは **絶対に行わない**。
  staging / pre-prod 環境専用とする。

## クリーンアップ

```sh
make chaos-mesh-uninstall
kubectl delete namespace valkey-chaos-e2e
```

## シナリオの追加

- 新規 chaos CRD: `chaos-mesh.org/v1alpha1` の他の kind
  (`TimeChaos`, `DNSChaos`, `KernelChaos` など) を採用できる。
- パターン: `test/chaos/scenarios_test.go` に新しい `var _ = Describe(...)`
  ブロックを追加し、`makeChaos(kind, name, ns, spec)` helper を使う。
- chaos-mesh CRD spec のリファレンス: <https://chaos-mesh.org/docs/>

## トラブルシューティング

| 症状 | 原因 / 対処 |
|---|---|
| `chaos-mesh.org/v1alpha1: NoMatchError` | chaos-mesh CRD 未導入 — `make chaos-mesh-install` |
| `kubectl apply` で permission denied | chaos-mesh controller の **namespace 権限** が不足している。`--local kind` インストールオプションを確認する。 |
| シナリオが timeout する | cluster サイズ / image pull が遅い — `--timeout=30m` 以上を指定して再実行する。 |
| Pod が `Terminating` のまま停止する | finalizer の解除が必要 — `kubectl patch pod ... --type=merge -p '{"metadata":{"finalizers":[]}}'` |

## 参照

- ADR-0041 — chaos-mesh 採用根拠と候補比較。
- ADR-0040 §gap #4 — chaos engineering e2e。
- chaos-mesh: <https://chaos-mesh.org/>
- Makefile targets: `chaos-mesh-install`, `chaos-mesh-uninstall`,
  `chaos-e2e`。
