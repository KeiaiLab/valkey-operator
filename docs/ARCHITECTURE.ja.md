# ARCHITECTURE — valkey-operator (日本語)

> English: [ARCHITECTURE.md](ARCHITECTURE.md) — canonical / 正本

> 1 ページ完結のアーキテクチャ記述。CRD 表面 / トポロジー / reconcile パターンが変更された際に併せて更新する。

## 概要

- **目的**: [Valkey](https://valkey.io) (Redis の BSD-3 fork) のための Kubebuilder ベースの K8s Operator。1 つのコントローラーが統一された CRD 表面の背後で 3 つのトポロジーを管理する。
- **スコープ**: Standalone / Replication / Cluster (16384-slot) トポロジー + バックアップ・リストア + S3 互換外部ストレージ。
- **安定度ティア**: v1.0.13 (standalone + replication + cluster は GA; federation は alpha)
- **最新リリース**: v1.0.13 (2026-05-13)
- **ライセンス**: Apache-2.0
- **モジュールパス**: `github.com/keiailab/valkey-operator`

## CRD 表面 (5 CRD)

| CRD | apiVersion | トポロジー | 説明 |
|---|---|---|---|
| `Valkey` | `valkey.keiailab.com/v1alpha2` | Standalone / Replication | 単一インスタンス または 1 primary + N replicas |
| `ValkeyCluster` | `valkey.keiailab.com/v1alpha2` | シャード化 Cluster (16384 slot) | 3+ シャード × (1 primary + 0–5 replicas) |
| `ValkeyBackup` | `valkey.keiailab.com/v1alpha2` | — | PVC + 外部ストレージへの単発 RDB または AOF バックアップ |
| `ValkeyBackupTarget` | `valkey.keiailab.com/v1alpha2` | — | S3 互換の外部ストレージ抽象化 (ADR-0016) |
| `ValkeyRestore` | `valkey.keiailab.com/v1alpha2` | — | Init Container 経由で RDB を Valkey または ValkeyCluster へリストア (ADR-0015) |

Conversion webhook が v1alpha1 ↔ v1alpha2 の変換をサポート。

## Reconcile フロー

```
Watch CRD events
      │
      ▼
Reconcile loop
      │
      ├── StatefulSet (per shard)
      ├── ConfigMap (valkey.conf)
      ├── Secret (auth + TLS keys)
      ├── Service (headless + ClusterIP)
      ├── PodDisruptionBudget
      ├── NetworkPolicy (deny-by-default)
      ├── cert-manager Certificate (webhook serving + TLS)
      └── Prometheus ServiceMonitor

すべてのリソースは spec-drift 検知付きで reconcile される。
Cluster トポロジー: シャードスケール時に slot リバランス + replica 再選出。
```

## RBAC スコープ

- ClusterRole: CRD watch + cert-manager Certificate + Prometheus ServiceMonitor
- Role (namespace 単位): StatefulSet / Service / Secret / ConfigMap / PVC / PDB / NetworkPolicy / Job
- ServiceAccount: `valkey-operator`
- Webhook: validation + conversion (cert-manager 経由の TLS)

## operator-commons import 表面

`operator-commons/ARCHITECTURE.md` のマトリクスに基づく採用率: **8/8 (100%)** — *カーボンコピーのリファレンス実装*。

| パッケージ | 状態 | 用途 |
|---|---|---|
| `pkg/security` | ✅ | restricted PSA SecurityContext (it8) |
| `pkg/version` | ✅ | Valkey バージョン allowlist (it8) |
| `pkg/labels` | ✅ | 推奨ラベル (it29) |
| `pkg/monitoring` | ✅ | ServiceMonitor reconciler (it23) |
| `pkg/networkpolicy` | ✅ | Deny-by-default + オプション (it25) |
| `pkg/webhook` | ✅ | Validation ヘルパ (it31) |
| `pkg/finalizer` | ✅ | `Add` / `Remove` / `Has` |
| `pkg/status` | ✅ | Condition reason |

valkey は *最初の 100% 採用者* — mongodb / postgres は自らの移行に際して本リポジトリをリファレンスとして利用している。

## テスト階層

| 階層 | 位置 | カバレッジ |
|---|---|---|
| Unit | `internal/**/_test.go`, `api/**/_test.go` | gocovmerge → cover-final.out |
| Integration (envtest) | `test/integration/` | reconcile + conversion + webhook |
| E2E (kind) | `test/e2e/`, `Makefile setup-test-e2e` | release クリティカルシナリオ |
| Scorecard | `bundle/tests/scorecard/` | OLM v1alpha3 6-test parity |

## ビルド / デプロイ

- コンテナイメージ: `ghcr.io/keiailab/valkey-operator:v1.0.13`
- Helm chart: `charts/valkey-operator/` (`keiailab.github.io/valkey-operator` で公開)
- OLM bundle: `bundle/`
- ArtifactHub: `keiailab-valkey-operator`
- Quickstart: kind クラスター + cert-manager 1.16+ (`make setup-test-e2e`)

## セキュリティサプライチェーン

- **SLSA-3 provenance** (ADR-0046)
- **cosign keyless 署名** (ADR-0046)
- **OpenSSF Scorecard** 有効化済み (README にバッジ)
- **CodeQL** + **dependency-review** + **DCO** ワークフロー
- **`.gitleaks.toml`** によるシークレットスキャン (カバレッジ 42/44)
- **go-licenses** による依存ライセンススキャン + allowlist

## ADR クロスリンク (45 ADR — 3 Operator 中で最多)

主要なもの:
- ADR-0015: Init Container パターンによる Restore
- ADR-0016: ValkeyBackupTarget — S3 抽象化
- ADR-0045: GitHub Actions release パイプラインの復旧
- ADR-0046: SLSA-3 + cosign keyless
- ADR-0047: community-operators 上流同期の自動化 (cycle 25)

全件: `docs/kb/adr/INDEX.md`。

## ロードマップ状況

- 完了: 31 項目 (Cluster モード + バックアップ・リストア + HPA/PDB/NP + バージョンアップグレード + Valkey 9.x + API 進化 + webhook admission + Helm + SLSA-3 + ServiceMonitor + OpenSSF)
- 残: 38 項目 (production cluster 採用 + 移行 runbook + smoke test + Grafana + OTel + SBOM + 9.x 機能フォロー + multi-cluster federation + cross-region レプリケーション + オンライン schema-less マイグレーション + 重み付き replica ルーティング + controller v2 + CRD v1 graduation)

## Non-goals

- ❌ Redis の組み込み (我々は Valkey を提供する — ライセンス互換の BSD-3 fork)
- ❌ third-party Valkey chart の組み込み (ネイティブに実装する)
- ❌ Redis Sentinel トポロジー (代わりに 3-shard cluster を使用)
- ❌ Valkey 8.0 未満

## 参考

- `README.md` / `README.ko.md`
- `ROADMAP.md`
- `CHANGELOG.md`
- `ADOPTERS.md` / `ADOPTERS.ko.md`
- `CONTRIBUTING.md` / `CONTRIBUTING.ko.md`
- `GOVERNANCE.md` / `GOVERNANCE.ko.md`
- `AGENTS.md`
- `docs/kb/adr/INDEX.md`

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
