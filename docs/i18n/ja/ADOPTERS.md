<p align="center">
  <a href="ADOPTERS.md">English</a> |
  <a href="ADOPTERS.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="ADOPTERS.zh.md">中文</a>
</p>

# valkey-operator の Adopters

> English version: [ADOPTERS.md](../../ADOPTERS.md)

本ドキュメントは `keiailab/valkey-operator` を稼働または評価している組織・プロジェクトの **public** リストです。自己登録を歓迎します — 行を追加する PR をオープンしてください。

## Production users

production グレードの SLA を伴って `valkey-operator` を本番稼働させている組織です。

| ユーザー | コンポーネント | 利用パターン | 初版 | 現バージョン | 掲載日 |
|---|---|---|---|---|---|
| **社内本番クラスタ** ([keiailab](https://github.com/keiailab)) | Valkey 9.0.4 (Standalone + sharded Cluster 3×1) | 社内本番ワークロードのキャッシュおよび pub/sub レイヤー。6-pod ValkeyCluster、`cluster_state=ok`、ServiceMonitor + alert-rules.yaml + PodSecurity restricted。 | v1.0.0 | v1.0.3 | 2026-05-07 |

## Evaluators

PoC、評価中、および Bitnami redis-cluster からの移行候補です。

| ユーザー | フェーズ | 備考 |
|---|---|---|
| _自己登録歓迎_ | — | 行を追加する PR をオープンしてください。ValkeyRestore ドキュメントに記載されている Redis 8.2 → Valkey 9.0 RDB 互換性制限に留意してください。 |

## 自分自身を追加する方法

上記のいずれかの表に行を append する PR をオープンしてください:

```markdown
| **<組織またはプロジェクト>** ([profile](<URL>)) | <コンポーネント + トポロジー> | <利用パターン> | <初版> | <現バージョン> | <YYYY-MM-DD> |
```

匿名で掲載を希望する場合は [SECURITY.md](../../../.github/SECURITY.md) のセキュリティ連絡先から連絡してください。メンテナーが代理で組織名を匿名化した行を登録します。

## CNCF Sandbox 参照

本リストは CNCF graduation 基準である「≥ 1 public adopter」の public 参照としても機能します。

## Bitnami redis-cluster からの移行

Bitnami `redis-cluster` (Redis 7.x / 8.x) を運用していて Valkey を評価中の場合、`ROADMAP.md` → **Phase B (RDB 互換性と代替移行パス)** を参照してください。一部の Redis 8.2.x RDB ファイルは Valkey 9.0.4 に直接リストアできません。その場合 `ValkeyRestore` は fail-fast で失敗するため、運用者が silent error を無期限に待つことはありません。

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
