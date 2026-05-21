# 運用ドキュメント索引 (日本語)

> English: [README.md](README.md) — canonical / 正本

> 日本語ユーザー向け: すべてのドキュメントは `<name>.ko.md` の韓国語版を併設しています (本ファミリーの i18n 初期 wave は韓国語起点)。

このディレクトリには `valkey-operator` の運用手順を集約しています。
**全ドキュメントの正本は英語** で、それぞれに `<name>.ko.md` の韓国語の姉妹ファイルを置いています (例外: `artifacthub-trust.md` は当初から英語で書かれており韓国語版はありません)。

## 日常運用

| ドキュメント | 用途 |
|---|---|
| [runbook.md](runbook.md) | 障害対応と日常運用。すべての Prometheus alert の `runbook_url` annotation の SSOT。 |
| [troubleshooting.md](troubleshooting.md) | アラートが鳴らない (もしくはアラートが整備される前の) 事象向けの 症状 → 原因 → 診断 → 対処 のフローチャート。 |
| [metrics-glossary.md](metrics-glossary.md) | すべての `valkey_cluster_*` メトリクス — ラベル cardinality、発生箇所、どの alert が利用しているか。 |
| [capacity-planning.md](capacity-planning.md) | Sizing ガイド — トポロジー別のメモリ、レプリケーション係数、シャード数、p95/p99 レイテンシ目標。 |
| [webhook.md](webhook.md) | Admission webhook アーキテクチャ、cert-manager 証明書パス、webhook 拒否のデバッグ。 |
| [pdb-per-shard.md](pdb-per-shard.md) | シャード化された `ValkeyCluster` のシャード単位 `PodDisruptionBudget` 運用 — drain シナリオ、モード遷移、書き込み可用性の保証。 |

## バックアップ・リストア・リカバリ

| ドキュメント | 用途 |
|---|---|
| [pitr-guide.md](pitr-guide.md) | Point-in-time recovery: phase-1 API + webhook、phase-2 reconciler dispatch、手動回避、ロールバック。 |
| [chaos-testing.md](chaos-testing.md) | chaos-mesh の 4 シナリオ e2e 手順: pod-kill、network partition、IO ENOSPC、IO latency。 |

## リリース & サプライチェーン

| ドキュメント | 用途 |
|---|---|
| [release-checklist.md](release-checklist.md) | リリース前ゲート一覧: build、47 SSOT gate、サプライチェーン (SLSA + cosign)、docs、operations。 |
| [post-merge-cleanup.md](post-merge-cleanup.md) | Squash-merge 後のローカルブランチ整理。 |
| [artifacthub-trust.md](artifacthub-trust.md) | Artifact Hub の `Signed` / `Official` 信頼バッジ運用手順。 |

## マイグレーション

| ドキュメント | 用途 |
|---|---|
| [sentinel-migration.md](sentinel-migration.md) | Sentinel → valkey-operator Replication モード移行 runbook (ADR-0017 backstop)。 |

## 横断的な参照

- アーキテクチャ決定: [`docs/kb/adr/INDEX.md`](../kb/adr/INDEX.md)
- インシデント履歴: [`docs/kb/incident/`](../kb/incident/)
- リリース検証コマンド: [`SECURITY.md → "Verifying release artifacts"`](../../.github/SECURITY.md#verifying-release-artifacts-signed-releases--v1013)
- ロードマップ (プロジェクトの方向性): [`ROADMAP.md`](../ROADMAP.md)

## i18n ステータス

`docs/operations/` 配下のすべての運用ドキュメントは現在 **英語が正本** です。i18n 取り組みは PR #93 / #97 / #98 / #103 / #104 / #105 / #106 / #107 / #108 / #109 / #110 / #111 にまたがって着地しました。韓国語の原文は `<name>.ko.md` の姉妹ファイルとしてそのまま保持されます (`artifacthub-trust.md` のみ最初から英語で執筆されているため例外)。
