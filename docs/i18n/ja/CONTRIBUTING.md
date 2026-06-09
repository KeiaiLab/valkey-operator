<p align="center">
  <a href="CONTRIBUTING.md">English</a> |
  <a href="CONTRIBUTING.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="CONTRIBUTING.zh.md">中文</a>
</p>

# コントリビュート

> 英語版: [CONTRIBUTING.md](../../../.github/CONTRIBUTING.md) — canonical / 正本

`valkey-operator` への関心をお寄せいただきありがとうございます。本ドキュメントは PR プロセス、テストの実行方法、Architecture Decision Record (ADR) が必要となる場面について説明します。

## はじめに

### 前提ツール

| ツール | 最低バージョン | 備考 |
|---|---|---|
| Go | 1.26 | `go.mod` と一致 |
| Docker | 24+ | buildx のデフォルトビルダー |
| kind | 0.27+ | ローカル end-to-end テスト用 |
| kubectl | 1.34+ | k3s / kind に対応 |
| cert-manager | 1.16+ | Webhook 用 serving cert |
| make | GNU make | すべての Makefile target を駆動 |

### 初回ビルド + テスト

```sh
git clone https://github.com/keiailab/valkey-operator.git
cd valkey-operator

# pre-commit hook をインストール (lefthook)。
brew install lefthook       # または `go install github.com/evilmartians/lefthook@latest`
lefthook install

# 単体テスト (envtest バイナリは自動取得)。
make test

# Integration test (実際の Valkey コンテナを起動するため Docker 必須)。
make integration-test

# End-to-end (kind cluster に operator をデプロイ)。
make test-e2e
```

## Pull Request ワークフロー

1. **まず issue を開く**: 軽微でない変更 (アーキテクチャ、API、
   セキュリティ) は短い alignment スレッドを残しておくと、後の
   書き直しを防げます。
2. **DCO sign-off は必須**: すべての commit には末尾に
   `Signed-off-by:` trailer が必要です (`git commit -s`)。commit-msg
   の lefthook hook がこれを強制し、未署名の PR はマージできません。
   [Developer Certificate of Origin](https://developercertificate.org/)
   を参照してください。
3. **Conventional Commits**: subject 行は
   `<type>(<scope>): <subject>` の形式 (例: `feat(backup): TTL auto-cleanup`)。
   本文は英語または韓国語のいずれでも構いません。
4. **テスト必須**: 振る舞いを変える変更には、それを動作させる単体テスト
   を少なくとも 1 件添付してください。`make test` が必ず通る必要があります。
5. **lefthook の通過が必須**: `gofmt`、`go vet`、`golangci-lint` が
   commit ごとに実行されます。hook が失敗すると commit がブロックされます。
6. **PR 本文に含めるべき内容**:
   - ユーザー視点のシナリオ (なぜこの変更が必要か)
   - 検証コマンドと整形した出力 (`make test`、
     `kubectl apply -f …` など)
   - 影響範囲 — 回帰を確認した領域
   - 関連する ADR や issue へのリンク
7. **レビュー SLA**: best-effort で 24 時間以内に初回レビュー。

## Architecture Decision Records (ADR)

以下のいずれかに該当する変更では、ADR を `docs/kb/adr/NNNN-<slug>.md`
として作成してください:

- 新規 CRD、または既存 CRD field の意味論的な変更
- 新規のサードパーティ依存関係 (ADR には `sonatype-guide` と
  `context7` の両方の評価を引用)
- セキュリティ、認証、データフローに関連する変更
- 同じ問題を異なる手段で解こうとする 3 回目以降の試み
  (収束 ADR)

Nygard の 5 セクションテンプレート (Context / Decision / Consequences
/ Alternatives Considered / Status) を用いてください。同じ commit で
`docs/kb/adr/INDEX.md` も必ず更新します。

## コードスタイル

- **Go**: `gofmt` と `golangci-lint` (lefthook 経由で実行)。`errcheck`
  は強制です。
- **コメント**: 英語と韓国語のいずれも歓迎します。*何を* ではなく
  *なぜ* を説明してください — *何を* するかはコード自体が物語ります。
- **テスト**: fake client を優先し、`envtest` は本物の controller
  統合パスに限定して用います。`WithStatusSubresource` を必ず使用し、
  spec と status を分離してください。

## 設計検討

大きな変更を行う前に:

1. `docs/plans/` 配下の既存の plan を確認します。
2. 6 つ以上の設計分岐を検討した場合は、事後ではなく事前に決定を
   ADR としてキャプチャしてください。
3. atomic commit を貫いてください — 論理的な 1 ステップ = 1 commit、
   それぞれが lefthook の 4 ステージすべてを通過すること。

## 品質システム (SSOT ゲート)

本リポジトリには 35+ の Single-Source-of-Truth 同期ゲートが搭載されています (release cycle 20–77 にかけて積み上げられたもの)。これにより「広告された表面 == 実際の挙動」がビルド不変条件として保証され、希望ではなくなります。

### ゲートの所在

- `internal/observability/*_test.go` — 33+ のすべての SSOT ゲートテスト
- インベントリ: [docs/operations/release-checklist.md §2](../../operations/release-checklist.md)

### ゲートがブロックする例 (マージ前)

- alert-rules + runbook の anchor が伴わない新規 metric
- `INDEX.md` 行、または必須の Nygard セクションを伴わない新規 ADR
- `config/rbac/role.yaml` の対応更新を伴わない新規 `kubebuilder:rbac`
  marker (`make manifests` を実行)
- どの template からも参照されない新規 Helm `values` キー
  (サイレントなタイポを検出)
- リリースチェックリスト §2 にまだ載っていない新規 SSOT ゲート
  (ゲートインベントリ自体がゲート)

### そもそも drift を防ぐ自動化

- `make manifests` が chart の CRD を自動同期 (cycle 38)
- `git push` で 6-hook の lefthook パイプラインが走り、full lint、
  gitleaks、`go mod tidy`、helm lint、helm template、unit test を実行
- pre-push の `go mod tidy` ステップが direct / indirect の drift を
  ブロック (cycle 47)

### ホットパスのベンチマーク

- `go test -bench=. ./internal/valkey/` — 5 種類のパーサーに対する
  ベースライン。baseline 比で 2x 低速化したら回帰のサインです。

### ゲート失敗が自己説明的であること

ほとんどのゲートは、失敗メッセージに正確な修正コマンドを出力します。例:

- `TestCRDBaseChartSync`: `cp config/crd/bases/X charts/.../crds/X && git commit`
- `TestRBACMarkerResourcesInRole`: `run make manifests`
- `TestReleaseChecklistGatesSync`: 新しいゲートを release-checklist §2 に追加

新たなコントリビューターが、隣接するどの表面を更新すべきか推測する
必要はありません — 失敗するテストがそれを教えてくれます。

## セキュリティ問題

脆弱性については公開 issue を開かないでください。非公開報告チャネル
(GitHub Security Advisory および PGP 署名付きメールアドレス) は
[SECURITY.md](../../../.github/SECURITY.md) を参照してください。

## ライセンス

本プロジェクトは MIT License です。コントリビュートすることで、
あなたのコントリビュートが同じライセンスの下で配布されることに
同意するものとします。
