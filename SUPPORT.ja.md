<p align="center">
  <a href="SUPPORT.md">English</a> |
  <b>日本語</b> |
  <a href="SUPPORT.zh.md">中文</a>
</p>

# サポート

> 日本語ユーザー: 本ドキュメントのチャネルは英語・日本語のいずれも歓迎します。

`valkey-operator` をご利用いただきありがとうございます。本ページでは支援を得られる場所を案内します。

## 必要なものを判断する

| 状況 | 行き先 |
|---|---|
| **セキュリティ脆弱性を発見したと思われる場合。** | **public な issue を開かないでください。** [SECURITY.md](SECURITY.md) を参照 — GitHub Security Advisory または `security@keiailab.com` (PGP 署名) を利用してください。 |
| 「これは X のように動作するのが正しいのか?」「Y はどのように設定するか?」といった質問。 | [GitHub Discussions](https://github.com/keiailab/valkey-operator/discussions)。検索可能で、将来の運用者にもインデックスされます。 |
| バグを発見した — ドキュメントと異なる挙動。 | **Bug report** テンプレートで [issue を作成](https://github.com/keiailab/valkey-operator/issues/new/choose) してください。 |
| 機能追加や挙動の変更を希望する。 | **Feature request** テンプレートで [issue を作成](https://github.com/keiailab/valkey-operator/issues/new/choose) してください。すでに計画済みかどうか [ROADMAP.md](ROADMAP.md) を先に確認してください。 |
| 「これは FAQ に入れるべき」という質問。 | **Question** テンプレートで [issue を作成](https://github.com/keiailab/valkey-operator/issues/new/choose) してください。 |
| Prometheus アラートに遭遇し、MTTR 手順が必要な場合。 | [`docs/operations/runbook.md`](docs/operations/runbook.md) §9 (各アラートの `runbook_url` アノテーションがここを指します)。 |
| アラートは出ていないが挙動がおかしい場合。 | [`docs/operations/troubleshooting.md`](docs/operations/troubleshooting.md) — 症状 → 原因 → 診断 → 対処のフローチャート。 |
| コードまたはドキュメントにコントリビュートしたい。 | [CONTRIBUTING.md](CONTRIBUTING.md) を参照。 |

## issue を開く前にお願いしたいこと

1. [既存の issue](https://github.com/keiailab/valkey-operator/issues?q=is%3Aissue) と [Discussions](https://github.com/keiailab/valkey-operator/discussions) を検索してください — 同じ質問への回答が既に存在する可能性があります。
2. [トラブルシューティングフローチャート](docs/operations/troubleshooting.md) を試してください。
3. レポートに以下を準備してください:
   - `valkey-operator` のバージョン (`kubectl get deploy -n valkey-operator-system -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'`)
   - Kubernetes バージョン (`kubectl version`)
   - Helm chart バージョン (`helm list -A | grep valkey-operator`)
   - 再現可能な最小ケース
   - `kubectl describe <Valkey|ValkeyCluster> <name>` の出力

## レスポンスに関する期待値

本プロジェクトはベストエフォートでメンテナンスされる OSS です。意思決定とレビューのプロセスは [GOVERNANCE.md](GOVERNANCE.md) に記載されています。issue への返答は通常 2-3 営業日以内、セキュリティレポートは [SECURITY.md](SECURITY.md) の SLA に従って処理します (初回 ack は 72 時間以内、severity トリアージは 7 日以内)。

有償サポート契約または厳密な SLA が必要な場合は `security@keiailab.com` までご連絡いただければ、オプションについてご相談できます。

## 商用ベンダー

現時点で `valkey-operator` は有償サポートベンダーを公認していません。今後変更があれば、ベンダーの提供条件と該当する upstream 機能とともにここに記載されます。

## 行動規範 (Code of Conduct)

上記のすべてのチャネルは [Code of Conduct](CODE_OF_CONDUCT.md) によって管理されています。参加前に必ずお読みください。

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
