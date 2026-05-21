<p align="center">
  <a href="GOVERNANCE.md">English</a> |
  <a href="GOVERNANCE.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="GOVERNANCE.zh.md">中文</a>
</p>

# ガバナンス

> English version: [GOVERNANCE.md](GOVERNANCE.md)

本ドキュメントは `keiailab/valkey-operator` における意思決定方法を定義します。

## 原則

1. **オープンネス。** すべての意思決定は public なチャネル — GitHub issue、pull request、RFC — で行われます。
2. **Lazy consensus (遅延合意)。** 日常的な変更は、誰も反対しなければ ship されます。
3. **明示的合意。** アーキテクチャ変更、CRD 変更、セキュリティモデル変更、ライセンス変更には、RFC を経た上で **メンテナーの 2/3 supermajority** が必要です。通常の RFC (単一コンポーネント、ツール採用、ポリシー強化) には **simple majority** (>50%) が必要です。本 `GOVERNANCE.md` への変更は常に 2/3 supermajority が必要です。
4. **共有責任。** メンテナーはコード品質、ユーザー安全性、コミュニティの健全性に対して共同で責任を負います。

## 意思決定の分類

### Routine (lazy consensus)

- バグ修正、ドキュメント改善、新規テスト、minor/patch の依存関係 bump、public API 変更のないリファクタリング
- プロセス: PR → 少なくとも 1 名のメンテナーから LGTM → マージ
- コメントウィンドウ: なし。ローカルゲートが pass すれば PR はただちにマージ可能 (RFC-0002 に従い GitHub Actions にはゲートを依存しません。pre-commit / pre-push hook と Makefile が enforcement ポイントです)。

### Medium (明示的合意)

- 新規 CRD フィールド、新規 reconciler、メジャー依存関係アップグレード、public API への変更
- プロセス: 変更を提案する issue を開く → 7 日間のコメントウィンドウ → メンテナー過半数の LGTM → マージ
- 反対 1 件で議論のためのメンテナーミーティングがトリガーされます。

### Architectural (RFC 必須)

- 新規コンポーネントの導入、セキュリティモデルの変更、ライセンス変更、後方互換性の破壊
- プロセス:
  1. `docs/kb/adr/NNNN-title.md` に ADR または RFC を提出
  2. 14 日間のコメントウィンドウ
  3. メンテナーの 2/3 承認
  4. ADR/RFC の `Status` を `Draft` から `Accepted` に移行、その後実装 PR をオープン

## セキュリティ判断

CVE レポートおよび secrets / auth モデルへの変更は、まず [SECURITY.md](SECURITY.md) のプライベートチャネルで処理します。public な合意は patch リリースが ship された後に行われます。

## リリース判断

単一メンテナーは lazy consensus のもとでリリースブランチを切り、バージョンを bump できます。新規 LTS ラインの作成または既存ラインの End-of-Life 宣言は、常に明示的合意が必要です。

## 変更履歴

| 日付 | 変更 | Refs |
|---|---|---|
| 2026-05-07 | ドキュメント作成 — 3 リポジトリ (mongodb / postgresql / valkey) のガバナンスアセット整合 | INC-2026-05-07 |
| 2026-05-12 | 英語版を canonical に変更、韓国語版は `GOVERNANCE.ko.md` として保持 | i18n PR-K |

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
