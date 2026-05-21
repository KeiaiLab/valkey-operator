<p align="center">
  <a href="MAINTAINERS.md">English</a> |
  <a href="MAINTAINERS.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="MAINTAINERS.zh.md">中文</a>
</p>

# メンテナー

> English version: [MAINTAINERS.md](MAINTAINERS.md)

本ドキュメントは `keiailab/valkey-operator` に対する意思決定権限を持つメンテナーを追跡します。

## 現在のメンテナー

| 名前 / チーム | GitHub | 役割 | 領域 |
|---|---|---|---|
| keiailab maintainers | [@keiailab/maintainers](https://github.com/orgs/keiailab/teams/maintainers) | Lead | 全領域 |

GitHub チーム `@keiailab/maintainers` がプロジェクトのすべての領域について merge および承認権限を保持します。個別メンテナーの追加は下記の手順に従います。

## 適格性

コントリビューターは、以下を少なくとも 6 ヶ月にわたって継続した後にメンテナーとして指名される可能性があります:

- 意味のあるコードまたはドキュメントの merged PR が 20 件以上
- 建設的フィードバックを伴う PR レビューが 30 件以上
- [GOVERNANCE.md](.github/GOVERNANCE.md) および [CODE_OF_CONDUCT.md](.github/CODE_OF_CONDUCT.md) への遵守実績
- 少なくとも 1 つのコア領域 — controller、resource builder、restore / backup、cluster sharding、observability 等 — に対する深い理解

## メンテナーの追加

1. 既存メンテナー (または候補者本人) が、追加を提案する issue または ADR をオープンします。
2. `@keiailab/maintainers` による 7 日間のコメントウィンドウでの lazy consensus。
3. 反対がなければ、コントリビューターは GitHub チームに追加され、`MAINTAINERS.md` は follow-up PR で更新されます。

## 非アクティブなメンテナー

6 ヶ月連続してアクティビティのないメンテナーは emeritus ステータスに移行されます (権限は剥奪、名誉的な掲載は維持)。復帰経路は新規メンテナー手順と同一です。

## 領域カバレッジ (CODEOWNERS と同期)

ディレクトリ別のレビューアルーティングについては `.github/CODEOWNERS` を参照してください。

## Emeritus

_(まだいません)_

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
