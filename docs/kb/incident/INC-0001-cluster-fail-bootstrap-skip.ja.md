# INC-0001: ValkeyCluster が cluster_state:fail のまま bootstrap が再実行されない (日本語)

> English: [INC-0001-cluster-fail-bootstrap-skip.md](INC-0001-cluster-fail-bootstrap-skip.md) — canonical / 正本

- Detected: 2026-05-09 14:27 (KST) — production cluster、運用クラスタ
- Resolved: 2026-05-10 09:18 (KST)
- Severity: SEV-2 (単一 cluster への影響、application traffic への影響なし — test data のみ)
- Owners: @eightynine01
- Tags: [valkey, cluster, reconcile, status, controller-runtime]

## Impact

- **ユーザ影響**: 0 (cluster の keys はすべて test data — `test_valkey_br_*`、`test_prod_*`、`test_failover_*`、unique キー 6 件 × 2 master+replica)。production application traffic は未接続の状態だった。
- **システム影響**: ValkeyCluster (運用インスタンス) (3 shards × 2 = 6 pods、16384 slots) — 約 19 時間にわたり `cluster_state: fail` のままスタック。
- **財務/法務影響**: なし。

## Timeline

- **2026-05-07 06:14**: cluster を初期デプロイ。Bootstrap が正常に完了。`clusterInitialized: true`。
- **2026-05-08 07:21**: ClusterReady=False / ClusterNotConverged condition が発現 (原因不明 — pods の変動または cluster bus partition の可能性)。
- **2026-05-09 14:27**: Pod が再起動 (9 時間前)。新しい IP が割り当てられる。`nodes.conf` の myself IP が *以前の IP* (例: 10.42.6.172) のまま更新されない。他ノードとの cluster gossip が失敗し、`cluster_state:fail` となる。controller は *clusterInitialized=true* を認識し、cluster bootstrap の再実行を *スキップ*。STS reconcile のみを試行し、STS conflict ("the object has been modified") を繰り返す。
- **2026-05-10 00:02**: ReconcileError condition が最後に transition。以降、controller queue は exponential backoff によって reconcile 頻度が低下。
- **2026-05-10 09:00 ~ 09:18**: デバッグ + 修復:
  1. controller pod restart (効果なし — clusterInitialized はそのまま)。
  2. 6 pods に対し CLUSTER RESET HARD を試行 — keys を保持している 3 master pods が拒否。
  3. Pod-1 の nodes.conf を削除 + restart — 部分的に復旧 (自 shard のみ OK)。
  4. 6 pods 全てで FLUSHALL + CLUSTER RESET HARD + AOF/nodes.conf 削除 + 同時 restart — fresh state に到達。
  5. controller が reconcile するも bootstrap せず — `clusterInitialized: true` が阻害要因。
  6. `kubectl patch --subresource=status` で `clusterInitialized: false` を強制 → controller が即座に bootstrap を再実行 → 16384 slots すべて OK。

## Root Cause

5 Whys:

1. **なぜ cluster_state:fail か?** Pod の nodes.conf に stale な myself IP が残り、cluster gossip が失敗した。
2. **なぜ nodes.conf が stale か?** Pod 再起動時、PVC 上の nodes.conf に *以前の IP* が保存されたままで、valkey の起動時にそれを読み込んだ。
3. **なぜ controller が復旧させなかったか?** controller が cluster bootstrap (CLUSTER MEET / ADDSLOTS / REPLICATE) ステップをスキップした。
4. **なぜ bootstrap ステップをスキップしたか?** `status.clusterInitialized: true` は *初期化完了* 時に set され、*cluster fail 状態であっても reset されない* — controller コードの *one-shot init* という前提があった。
5. **なぜ one-shot init を前提としたか?** 初期設計時に *cluster が一度 bootstrap されれば永久に healthy* という前提を置いていた — pod restart 後の IP 変更シナリオが未考慮。ADR-0017 (failover 保存) の決定時にも *cluster topology の自動回復* 領域は対象外だった。

寄与要因:

- `nodes.conf` は PVC に保存される (stateful) — 新しい IP を反映させるには valkey-cli による cluster reset か announce-ip の更新が必要。
- controller の ReconcileError condition は STS conflict のみを反映しており、*cluster 自体の fail* シグナルを発行していない (alert が発火せず、recovery も起動しない)。

## Resolution

手動収拾 (本 INCIDENT):

1. data 損失の評価: keys はすべて test data → 安全に wipe 可能と判断。
2. 6 pods の PVC データ (AOF + nodes.conf + dump.rdb) をすべて削除。
3. 6 pods を同時に restart (fresh state)。
4. `status.clusterInitialized: false` を強制 patch。
5. controller の spec mutation で reconcile を trigger。
6. controller が即座に cluster bootstrap を再実行 → 16384 slots OK。

恒久 fix (別 PR — 本 INC の後続):

- controller コードで *clusterInitialized* flag を評価する際、`cluster_state == "ok"` AND `assignedSlots == 16384` まで検証する。fail または partial assignment であれば *automatic re-bootstrap* を実行する。
- alert rule の追加: `cluster_state:fail` が 30s 以上継続したら PrometheusRule を発火させる。

## Prevention

短期 (本 incident 内):

- ✓ INCIDENT KB 起草 (本ドキュメント)。
- ⏳ Alert rule (`prometheus.io/scrape` annotation の metrics path — `cluster_status_ok` metric)。

中期 (別 PR):

- **PR-INC-0001-fix**: controller が `clusterInitialized` true であっても *再検証* するロジックを追加。`cluster_state != "ok"` || `assignedSlots != 16384` の時に bootstrap を再実行する。
- **PR-INC-0001-alert**: PrometheusRule (`groups[].rules` に `ValkeyClusterFail` alert を追加)。

長期 (RFC 後続):

- RFC-0005 (別 RFC): cluster topology 自己治癒方針。nodes.conf の myself IP の自動検証 + cluster announce-ip の動的更新。valkey 9.x の *cluster-announce-bus-port* + DNS-aware IP advertisement を活用する。

## Action Items

- [ ] AI-0001: PR-INC-0001-fix — controller の re-bootstrap ロジック (Owner: @eightynine01、Due: 2026-05-15)。
- [ ] AI-0002: PR-INC-0001-alert — PrometheusRule (`ValkeyClusterFail` warning + critical)。
- [ ] AI-0003: e2e regression test — pod IP 変更シナリオ + clusterInitialized=true の阻害要因を検証 (test/e2e/cluster_recovery_test.go)。
- [ ] AI-0004: docs/operations/runbook.md — cluster_state:fail の収拾手順を文書化。

## References

- ADR-0017 (failover 保存) — 本 INC は対象領域外のシナリオ (cluster topology 復旧)。
- HANDOFF.md の PR-A2.2.5 storageversion fix — *別 incident*、本 INC とは無関係。
- mongodb INC-0001 (別 repo) — patterns の cross-cut audit 候補。
