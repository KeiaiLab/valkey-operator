<p align="center">
  <a href="ROADMAP.md">English</a> |
  <a href="ROADMAP.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="ROADMAP.zh.md">中文</a>
</p>

# ROADMAP — valkey-operator

> English (canonical / 正本): [ROADMAP.md](../../ROADMAP.md)

本 ROADMAP は *日付の約束ではなく*、検証可能な機能チェックリストとして進捗を追跡します。時間ベースの deadline はプロジェクトの
[`standards/workflow.md`](https://github.com/keiailab/valkey-operator/blob/main/docs/kb/adr/INDEX.md)
("時間ベースのロードマップ禁止" ルール) に従い意図的に排除しており、進捗は機能完成度で測定します。

## チェックボックスの意味

| マーカー | 意味 |
|---|---|
| `[x]` | コード + テストの両方が存在。e2e または unit test で回帰ガードを確保 |
| `[~]` | 部分実装 (フィールドのみ定義、helper 未統合、または検証項目が未完了) |
| `[ ]` | 未着手 (設計または PoC 段階) |

各 sub-task 右側の *Verify* は、チェックボックスを確認するための正確なコマンドまたは e2e ファイルを引用しています。

## 現在 (1.x ライン — Active)

### 安定性と成熟度

- [x] **PodSecurity restricted compliance**
  - [x] restricted SecurityContext helper (`buildRestrictedContainerSecurityContext` など) を
    resources ビルダー全体に適用 — `internal/resources/statefulset.go`、
    `backup_job.go`、`download_job.go`、`upload_job.go`、`restore.go`
  - [x] resources ビルダーにおける restricted PSA 回帰ガード
  - [x] controller / webhook 側 podSpec 変換経路の完全ガード
    — `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validatePodSecurityRestricted` (6 項目 —
    runAsNonRoot/runAsUser/privileged/allowPrivilegeEscalation、9 unit
    test、#78)
  - Verify: namespace に
    `pod-security.kubernetes.io/enforce=restricted` ラベルを付けた後 pod が Ready になること

- [x] **Cluster mode (5 shard × replica=2)**
  - [x] Ordinal ベースの restore Init Container —
    `internal/controller/valkeycluster_controller.go`
  - [x] 16384 slot の自動分配
  - [x] 自動 failover (chaos test 済み) —
    `test/e2e/cluster_recovery_test.go`, `failover.go`
  - [x] Primary kill → master 再選出 —
    `test/e2e/failover_test.go`
  - Verify: `test/e2e/cluster_recovery_test.go` PASS、16384 slot
    全保持、データ保全

- [x] **HPA / PDB / NetworkPolicy 自動化 (opt-in)**
  - [x] HPA (ADR-0027、Replication mode) — chart
    `autoscaling.enabled`
  - [x] PDB デフォルト — `internal/controller/pdb_default.go`
  - [x] NetworkPolicy default-deny + 明示的 allow — chart
    `networkPolicy.enabled`
  - Verify: `pdb_default_test.go` PASS、
    `kubectl get pdb/networkpolicy`

- [x] **Backup / Restore — S3 + PVC ROX + VolumeSnapshot**
  - [x] S3 (minio-go) backup —
    `internal/controller/valkeybackup_controller.go`
  - [x] PVC ROX マルチマウント restore —
    `internal/controller/valkeyrestore_controller.go`
  - [x] VolumeSnapshot lifecycle —
    `internal/controller/backup_volumesnapshot.go`
  - [x] Multipod snapshot replication restore —
    `multipod_volumesnapshot_replication_test.go`
  - [x] `ValkeyBackupTarget` CRD (外部 backup destination) —
    `api/v1alpha2/valkeybackuptarget_types.go`
  - Verify: `test/e2e/backup_restore_test.go` PASS

- [x] **chart RBAC conditional fix** (2026-05-07、commit `06237be`)
  - [x] `features.{cluster,backup}.enabled=false` 時の
    informer 起動失敗を防止
  - Verify:
    `--set features.cluster.enabled=false` で chart install 後、operator pod が Ready になること

- [x] **Version-upgrade reconcile fix**
  - [x] Fresh シナリオ経路は正常 (iteration 7 診断)
  - [x] Restore → patch chain 回帰ガード (iteration 18 V2) —
    `test/e2e/backup_restore_test.go` "Restored instance 8.1.6 → 9.0.4
    version patch chain (V2)"
  - [x] RDB v80 互換性 (`foo=bar1` 保持)
  - Verify: 上記 e2e PASS = 2 つの狭い blocker が恒久的に解消

- [x] **Valkey 9.x サポート (デフォルト 9.0.4)**
  - [x] Chart `image.tag: 9.0.4` デフォルト —
    `charts/valkey-operator/values.yaml`
  - [x] RDB フォーマット v80 互換性検証済み
  - Verify: 新規インスタンスを起動し
    `valkey-cli INFO server | grep redis_version`

- [x] **API バージョンの進化**
  - [x] v1alpha2 active — `api/v1alpha2/`
  - [~] v1alpha1 → v1alpha2 conversion webhook —
    `api/v1alpha2/conversion.go` (変換関数 + Hub マーカーは存在するが
    webhook サービング経路が未配線 — config/crd に
    `spec.conversion.strategy: Webhook` なし / chart webhook に conversion
    clientConfig なし / cmd/main.go 未登録、`api/v1alpha2/doc.go` 参照)
  - [x] 5 CRD (Valkey、ValkeyCluster、ValkeyBackup、ValkeyRestore、
    ValkeyBackupTarget)
  - Verify: `kubectl apply -f <v1alpha1.yaml>` を実行し、v1alpha2 オブジェクトとして保存されることを確認

- [x] **Online PVC resize** — `commonspvc.ExpandDataPVCs`
  (operator-commons `pkg/pvc`) を
  `internal/controller/valkey_controller.go` /
  `internal/controller/valkeycluster_controller.go` から呼び出し (ADR-0049)

- [x] **Webhook admission validation (4 validating webhook +
  conversion webhook)** — `internal/webhook/v1alpha1/`
  (Valkey / ValkeyCluster / ValkeyBackupTarget / ValkeyRestore の
  validating webhook; ValkeyBackup は validating webhook なし —
  5 番目の CRD は conversion 経路で処理)
  - [x] RBD storageClass 基本検証 —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateStorageClassName` (DNS-1123 subdomain)
  - [x] Topology-spread 一貫性検証 —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateTopologySpread` (MaxSkew / TopologyKey /
    WhenUnsatisfiable / duplicate key、#77)
  - [x] replicaCount lower-bound チェックを webhook に統合 — `valkey_webhook.go` (Replication → replicas ≥ 2 / Standalone → replicas = 1 / autoscaling.minReplicas ≥ 2) + `valkeycluster_webhook.go` (autoFailover → replicasPerShard ≥ 1)
  - Verify: `go test ./internal/webhook/v1alpha1/` PASS

- [x] **Encryption audit (TLS / 暗号化 surveillance)** —
  `internal/controller/encryption_audit.go`、
  `encryption_enforce_test.go`

- [~] **Valkey 公式 module preset (Redis Stack 相当)** — BSD ライセンスの module `valkey-search` / `valkey-json` / `valkey-bloom` を `ValkeySpec.Modules` で turnkey ロード。外部 Redis Stack module (RediSearch / RedisJSON、RSALv2 / SSPL) は *意図的な非対象* — Valkey BSD-3 とライセンス非互換 (ADR-0032)
  - [x] `ModuleSpec` type + `ValkeySpec.Modules []ModuleSpec` フィールド (PR-C6.1) — `api/v1alpha2/valkey_types.go`
  - [ ] Controller 配線 — init container `.so` mount (emptyDir) + StatefulSet podSpec の `--loadmodule` (PR-C6.2) — `internal/resources/statefulset.go`
  - [ ] 公式 preset allow-list 検証 + 公式 image 自動 resolve (admission webhook) — `internal/webhook/v1alpha1/valkey_webhook.go`
  - [ ] chart module リスト公開 — `charts/valkey-operator/values.yaml`
  - [ ] e2e — `valkey-search` `FT.SEARCH` 往復 — `test/e2e`
  - Verify: `modules` に `valkey-search` preset を指定した Valkey CR を適用後、`valkey-cli MODULE LIST` に module ロードを確認

### 運用とデリバリ

- [x] Helm chart 公開 — `keiailab.github.io/valkey-operator`
- [x] 3-repo (mongodb / postgres / valkey) ガバナンス資産
  整合 (CODE_OF_CONDUCT / GOVERNANCE / MAINTAINERS / ROADMAP)
- [x] **GitHub Actions release pipeline 復元** (ADR-0045) —
  外部公開 OSS リポジトリ向けの RFC-0002 からの scoped 逸脱;
  詳細は [ADR-0045](../../kb/adr/0045-restore-github-actions-for-oss-ci.md) を参照
- [x] **SLSA-3 provenance + cosign keyless signing** をイメージ、
  Helm chart、SBOM に対して適用 (ADR-0046) — 検証コマンドは
  [SECURITY.md](../../../.github/SECURITY.md) を参照。v1.0.13 から有効。
- [x] **本番クラスタへの導入** <!-- live-verified: 2026-05-27 -->
  - [x] CRD-install manifest — operator Helm chart で配備
  - [x] ArgoCD application 登録 — operator + workload app すべて Synced/Healthy
  - [x] 本番ワークロードを operator-managed CR に移行 — 4 つのライブ
    インスタンス (cluster 3-shard 16384 slot ok + replication)、plain StatefulSet ではない
  - Verify: ArgoCD Synced/Healthy +
    `kubectl get valkey/valkeycluster -A`
- [x] **Migration runbook** — plain StatefulSet → ValkeyCluster CR (PR #136)
  - [x] zero-downtime 手順のドキュメント化 — `docs/migration/zero-downtime.md` (PR #136)
  - [x] secondary-promote ベースの cutover — `docs/migration/secondary-promote.md` (PR #136)
  - [x] ロールバック手順 — `docs/migration/rollback.md` (PR #136)
  - Verify: staging dry-run + RTO / RPO 測定結果の記録
  - [x] 5 ステージ: image / SBOM / trivy / chart index / smoke — `scripts/release-smoke-test.sh` (PR #136)
  - Verify: `bash scripts/release-smoke-test.sh <tag>` で 12/12 PASS

### 可観測性とセキュリティ

- [x] **Prometheus ServiceMonitor 自動** —
  `internal/resources/servicemonitor.go`、
  `servicemonitor_test.go`、chart
  `metrics.serviceMonitor.enabled=true`
- [x] **OpenSSF Scorecard + dependency-review + CodeQL SAST + DCO
  workflows** — `.github/workflows/` を参照
- [x] Grafana ダッシュボード (cluster shard 分布 / replication
  lag / memory pressure)
  - [x] 4 パネル: cluster overview、replication、memory、latency — `charts/valkey-operator/dashboards/{cluster-overview,replication,memory,latency}.json`
  - [x] Helm chart ConfigMap 統合 — `charts/valkey-operator/templates/grafana-dashboards.yaml`
- [x] OpenTelemetry trace 伝播
  - [x] controller reconcile span の計装 — 5 controller が `observability.StartReconcileSpan` を呼び出し
  - [x] OTLP exporter の組み込み — `internal/observability/tracing.go` `SetupTracing` (opt-in、ADR-0025)
- [x] Image SBOM (SPDX) + trivy HIGH/CRITICAL fixed-only スキャン
  - [x] 3-repo 共有スクリプトの採用 — `scripts/sbom-attach.sh`
  - [x] release 時の自動添付 — `cosign attest` + `gh release upload`

## 次 (2.x ライン — Planning)

### 機能

- [ ] **Valkey 9.x 新機能フォローアップ** — flag / cluster-mode
  変更点の追従
- [ ] **Multi-cluster federation**
  - [ ] ClusterRole 分離
  - [ ] トポロジ認識ルーティング
  - [ ] 新規 CRD `ValkeyFederation`
- [ ] **Cross-region backup replication**
  - [ ] S3 SSE-KMS キー管理
  - [ ] 自動 lifecycle policy
- [ ] **Online schema-less migration**
  - [ ] RDB diff ツール
  - [ ] LWW conflict resolution
- [ ] **Read replica 加重ルーティング** (latency-aware)

### アーキテクチャ

- [ ] **Controller v2**
  - [ ] workqueue rate-limiter チューニング
  - [ ] reconcile fan-out 最適化
- [ ] **CRD v1 graduation**
  - [ ] スキーマ安定化
  - [ ] v1alpha2 → v1 conversion webhook
  - Verify: 6 ヶ月間 BREAKING CHANGE 0 件 + 3-repo
    互換性

## Non-Goals (意図的な範囲外)

- ❌ **マルチテナント分離** — namespace 単位のみ。より強い
  分離は別クラスタに委ねる。
- ❌ **自前のシークレットローテーション** — ESO
  (External Secrets Operator) + OpenBao に委譲。
- ❌ **Sentinel mode** — Redis-Sentinel 互換は
  サポート対象外。Cluster mode が推進路線。
- ❌ **カレンダーベースの ROADMAP deadline** —
  `standards/workflow.md` を参照。

## 変更履歴

| 日付 | 変更 | 参照 |
|---|---|---|
| 2026-06-03 | **Valkey 公式 module preset (Redis Stack 相当)** を「安定性と成熟度」に `[~]` 項目として追加 — `ModuleSpec` / `ValkeySpec.Modules` API 表面は出荷済み (PR-C6.1)、controller init container 配線 / webhook allow-list / chart values / e2e は PR-C6.2 残り。外部 Redis Stack module は非対象維持 (RSALv2 / SSPL ↔ BSD-3) | ADR-0032 |
| 2026-06-03 | 引用パス訂正 — 2026-05-27 の訂正が見落とした phantom 引用パス fix (機能は実在、パスのみ誤り): conversion webhook サービング未配線 → `[~]` (`api/v1alpha2/doc.go`); PodSecurity helper 実体は `statefulset.go` 等 (`security.go` 不在); webhook ヘッダ `v1alpha2/`→`v1alpha1/` + "4 validating webhook + conversion"; Online PVC resize → `commonspvc.ExpandDataPVCs` (ADR-0049, `pvc_resize.go` 不在); smoke-test Verify `hack/`→`scripts/`。`internal/observability/roadmap_citation_test.go` 回帰ガード追加 | docs/roadmap-citation-truthup |
| 2026-05-12 | English を正本に昇格、韓国語は `ROADMAP.ko.md` として保持、ADR-0045 (GH Actions 復元) + ADR-0046 (SLSA-3 + cosign) を Operations と Security セクションに反映 | i18n initiative |
| 2026-05-11 | webhook `validateStorageClassName` 追加 — RBD storageClass DNS-1123 基本検証 `[x]` | ralph-loop iter#2 |
| 2026-05-11 | 全面改稿 — 事実訂正 (ServiceMonitor 等)、sub-task の粒度を細分化、新規項目 (VolumeSnapshot multipod、conversion webhook) 露出 | parallel-leaping-seal plan |
| 2026-05-07 | 文書作成 — 3-repo ガバナンス資産整合 | INC-2026-05-07 |
