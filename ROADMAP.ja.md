<p align="center">
  <a href="ROADMAP.md">English</a> |
  <a href="ROADMAP.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="ROADMAP.zh.md">中文</a>
</p>

# ROADMAP — valkey-operator

> English (canonical / 正本): [ROADMAP.md](docs/ROADMAP.md)

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
  - [x] 4 ヶ所の SecurityContext helper 統一 — `internal/resources/security.go`
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
  - [x] v1alpha1 → v1alpha2 conversion webhook —
    `api/v1alpha2/conversion.go`
  - [x] 5 CRD (Valkey、ValkeyCluster、ValkeyBackup、ValkeyRestore、
    ValkeyBackupTarget)
  - Verify: `kubectl apply -f <v1alpha1.yaml>` を実行し、v1alpha2 オブジェクトとして保存されることを確認

- [x] **Online PVC resize** —
  `internal/controller/pvc_resize.go`

- [x] **Webhook admission validation (5 CRD)** —
  `internal/webhook/v1alpha2/`
  - [x] RBD storageClass 基本検証 —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateStorageClassName` (DNS-1123 subdomain)
  - [x] Topology-spread 一貫性検証 —
    `internal/webhook/v1alpha1/valkeycluster_webhook.go`
    `validateTopologySpread` (MaxSkew / TopologyKey /
    WhenUnsatisfiable / duplicate key、#77)
  - [ ] replicaCount lower-bound チェックを webhook に統合
  - Verify: invalid spec が webhook によって reject されること

- [x] **Encryption audit (TLS / 暗号化 surveillance)** —
  `internal/controller/encryption_audit.go`、
  `encryption_enforce_test.go`

### 運用とデリバリ

- [x] Helm chart 公開 — `keiailab.github.io/valkey-operator`
- [x] 3-repo (mongodb / postgres / valkey) ガバナンス資産
  整合 (CODE_OF_CONDUCT / GOVERNANCE / MAINTAINERS / ROADMAP)
- [x] **GitHub Actions release pipeline 復元** (ADR-0045) —
  外部公開 OSS リポジトリ向けの RFC-0002 からの scoped 逸脱;
  詳細は [ADR-0045](docs/kb/adr/0045-restore-github-actions-for-oss-ci.md) を参照
- [x] **SLSA-3 provenance + cosign keyless signing** をイメージ、
  Helm chart、SBOM に対して適用 (ADR-0046) — 検証コマンドは
  [SECURITY.md](.github/SECURITY.md) を参照。v1.0.13 から有効。
- [ ] **本番クラスタへの導入**
  - [ ] CRD-install manifest
  - [ ] ArgoCD application 登録
  - [ ] 本番 Valkey ワークロードを plain StatefulSet から operator に
    マイグレーション
  - Verify: ArgoCD Synced/Healthy +
    `kubectl get valkey/valkeycluster -A`
- [x] **Migration runbook** — plain StatefulSet → ValkeyCluster CR (PR #136)
  - [x] zero-downtime 手順のドキュメント化 — `docs/migration/zero-downtime.md` (PR #136)
  - [x] secondary-promote ベースの cutover — `docs/migration/secondary-promote.md` (PR #136)
  - [x] ロールバック手順 — `docs/migration/rollback.md` (PR #136)
  - Verify: staging dry-run + RTO / RPO 測定結果の記録
- [x] **release-smoke-test.sh** — mongodb-operator パターンを移植 (PR #136)
  - [x] 5 ステージ: image / SBOM / trivy / chart index / smoke — `scripts/release-smoke-test.sh` (PR #136)
  - Verify: `bash hack/release-smoke-test.sh <tag>` で 12/12 PASS

### 可観測性とセキュリティ

- [x] **Prometheus ServiceMonitor 自動** —
  `internal/resources/servicemonitor.go`、
  `servicemonitor_test.go`、chart
  `metrics.serviceMonitor.enabled=true`
- [x] **OpenSSF Scorecard + dependency-review + CodeQL SAST + DCO
  workflows** — `.github/workflows/` を参照
- [x] Grafana ダッシュボード (cluster shard 分布 / replication (PR open)
  lag / memory pressure)
  - [x] 4 パネル: cluster overview、replication、memory、latency — `charts/valkey-operator/dashboards/{cluster-overview,replication,memory,latency}.json` (PR open)
  - [x] Helm chart ConfigMap 統合 — `charts/valkey-operator/templates/grafana-dashboards.yaml` (PR open)
- [ ] OpenTelemetry trace 伝播
  - [ ] controller reconcile span の計装
  - [ ] OTLP exporter の組み込み
- [x] Image SBOM (SPDX) + trivy HIGH/CRITICAL fixed-only スキャン (PR open)
  - [x] 3-repo 共有スクリプトの採用 — `scripts/sbom-attach.sh` (PR open)
  - [x] release 時の自動添付 — `cosign attest` + `gh release upload` (PR open)

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
| 2026-05-12 | English を正本に昇格、韓国語は `ROADMAP.ko.md` として保持、ADR-0045 (GH Actions 復元) + ADR-0046 (SLSA-3 + cosign) を Operations と Security セクションに反映 | i18n initiative |
| 2026-05-11 | webhook `validateStorageClassName` 追加 — RBD storageClass DNS-1123 基本検証 `[x]` | ralph-loop iter#2 |
| 2026-05-11 | 全面改稿 — 事実訂正 (ServiceMonitor 等)、sub-task の粒度を細分化、新規項目 (VolumeSnapshot multipod、conversion webhook) 露出 | parallel-leaping-seal plan |
| 2026-05-07 | 文書作成 — 3-repo ガバナンス資産整合 | INC-2026-05-07 |

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
