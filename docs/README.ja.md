# ドキュメント — valkey-operator

> English: [README.md](README.md) — canonical / 正本

> 正本となるドキュメントはすべて **英語版** である。韓国語の姉妹版が
> 存在する場合は `<name>.ko.md` として併置されており、日本語 (`.ja.md`) と
> 中国語 (`.zh.md`) も同様の命名規則に従う。本ページは `docs/` 配下の
> **単一の入口** である。

## 運用ガイド

| ドキュメント | 用途 |
|---|---|
| [運用 index](operations/README.md) | 日常 runbook + トラブルシューティング + metric + webhook + PDB + サイジング |
| [Runbook](operations/runbook.md) | 障害対応と日常運用 — 全 Prometheus alert の `runbook_url` の SSOT |
| [Troubleshooting](operations/troubleshooting.md) | 症状 → 原因 → 診断 → 対処 |
| [Metric 用語集](operations/metrics-glossary.md) | 全 `valkey_cluster_*` metric の cardinality と alert consumer |
| [キャパシティプランニング](operations/capacity-planning.md) | サイジング — メモリ、replication factor、shard 数、p95/p99 レイテンシ目標 |
| [Webhook](operations/webhook.md) | admission webhook のアーキテクチャ、cert-manager 経路、denial デバッグ |
| [Shard 単位 PDB](operations/pdb-per-shard.md) | shard 単位の `PodDisruptionBudget` 運用と drain シナリオ |
| [PITR ガイド](operations/pitr-guide.md) | Point-in-time recovery — API + webhook + manual workaround |
| [Chaos テスト](operations/chaos-testing.md) | chaos-mesh の 4 シナリオ e2e 手順 |

## リリースとサプライチェーン

| ドキュメント | 用途 |
|---|---|
| [Release checklist](operations/release-checklist.md) | release 前ゲート一覧: build、SSOT gate、サプライチェーン、ドキュメント |
| [Post-merge cleanup](operations/post-merge-cleanup.md) | squash-merge 後のローカル branch 整理 |
| [Artifact Hub trust](operations/artifacthub-trust.md) | Artifact Hub の `Signed` / `Official` バッジ運用手順 |
| [Upgrading](UPGRADING.md) | minor / major アップグレード時のマイグレーション、静的 manifest 利用者向け RBAC patch |

## マイグレーション

| ドキュメント | 用途 |
|---|---|
| [Migration index](migration/README.md) | StatefulSet → ValkeyCluster CR マイグレーション runbook 一覧 |
| [Sentinel マイグレーション](operations/sentinel-migration.md) | 既存 Sentinel HA → Replication-mode + AutoFailover (ADR-0017 backstop) |

## アーキテクチャと決定記録

| ドキュメント | 用途 |
|---|---|
| [ADR index](kb/adr/INDEX.md) | 52+ 件の Architecture Decision Record |
| [Observability — OpenTelemetry](observability/otel.md) | OTel trace 伝播、OTLP exporter、controller-reconcile span |
| [Valkey 9.x feature flag](version/9x-flags.md) | 新しい cluster-mode フラグと operator 側の統合経路 |

## ナレッジベース

| ドキュメント | 用途 |
|---|---|
| [Incident KB](kb/incident/INDEX.md) | postmortem-lite 記録 (blameless) |
| [依存変更ログ](kb/deps/2026-05.md) | go.mod / go.sum の diff 履歴 |

## コミュニティ衛生

GitHub が検出するコミュニティ衛生ファイルは ADR-0053 (root `.md` 方針 +
tool-dependency 例外) に従って `.github/` 配下に置かれる:

| ドキュメント | パス |
|---|---|
| Contributing guidelines | [.github/CONTRIBUTING.md](../.github/CONTRIBUTING.md) |
| Code of Conduct | [.github/CODE_OF_CONDUCT.md](../.github/CODE_OF_CONDUCT.md) |
| Security policy + artifact verification | [.github/SECURITY.md](../.github/SECURITY.md) |
| Support resources | [.github/SUPPORT.md](../.github/SUPPORT.md) |
| Project governance | [.github/GOVERNANCE.md](../.github/GOVERNANCE.md) |

## i18n の状況

韓国語の姉妹版は英語の canonical の隣に併置される (`<name>.ko.md`)。
日本語 (`.ja.md`) と中国語 (`.zh.md`) のカバレッジは **トップレベル
ファイル** (README、ROADMAP、CONTRIBUTING) を中心に整備されており、
運用 runbook は採用状況の需要に応じて段階的にローカライズしていく。
