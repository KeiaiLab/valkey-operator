<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

<p align="center">
  <a href="family.md">English</a> |
  <a href="family.ko.md">한국어</a> |
  <b>日本語</b> |
  <a href="family.zh.md">中文</a>
</p>

# keiailab オペレーターファミリー

> 共通基盤 (`operator-commons` Go ライブラリ + Helm partial + Apache-2.0 スタック) 上に構築された 4 つの姉妹 Kubernetes Operator。

本ページは `valkey-operator` リポジトリで作成されており、ファミリー全体の canonical なクロスリンクページです。

## ファミリー概要

| プロジェクト | データベース | ステータス | リポジトリ |
|---|---|---|---|
| **`postgres-operator`** | PostgreSQL 18+ | active | https://github.com/keiailab/postgres-operator |
| **`mongodb-operator`** | MongoDB 7.0+ | active | https://github.com/keiailab/mongodb-operator |
| **`valkey-operator`** | Valkey 8.0+ (Redis fork, BSD-3) | active | https://github.com/keiailab/valkey-operator |
| **`operator-commons`** | 共通 Go ライブラリ | v0.7.0 | https://github.com/keiailab/operator-commons |

## 共有しているもの

4 つのプロジェクトすべてが同じ運用プリミティブに収束します:

- **Apache-2.0** 一貫 — SSPL なし、SaaS 面に copyleft なし
- **`operator-commons`** 共通 Go ライブラリ (v0.7.0+) — finalizer、label、status sugar、security context builder、NetworkPolicy / ServiceMonitor partial
- **Helm chart skeleton** — RFC-0027 `default` falsy-toggle 防止、RFC-0026 component-keyed values、cycle 26 hardening 6 marker (priorityClassName / lifecycle / SA / minReadySeconds / automount / revisionHistoryLimit)
- **OLM bundle parity** — scorecard v1alpha3 6-test matrix
- **i18n** — README + 11 件の canonical ドキュメントを英語 / 한국어 / 日本語 / 中文 で (cleanup supercycle 2026-05-21 の Wave 4)

## 行わないこと

- ❌ **third-party Operator の埋め込み・ラッピング** — license-clean、copyleft 義務なし
- ❌ **GitHub Actions を release gate として使用** — ローカル 4-layer hook システム (RFC-0002 参照)
- ❌ **時間ベースの roadmap deadline** — 機能チェックリスト + 完成度パーセンテージ
- ❌ **ベンダーロック型コンテナイメージ** — keiailab-published Apache-2.0 イメージのみ使用

## どこから始めるか

| タスク | 開始点 |
|---|---|
| Kubernetes 上に `valkey-operator` を展開 | [README.md](../README.md) Quickstart セクション |
| アーキテクチャを読む | [ARCHITECTURE.md](../ARCHITECTURE.md) |
| issue または機能リクエスト | https://github.com/keiailab/valkey-operator/issues |
| デザインまたはロードマップを議論 | https://github.com/keiailab/valkey-operator/discussions |
| コードをコントリビュート | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| セキュリティ問題を報告 | [SECURITY.md](../SECURITY.md) |
| ブランド・ボイスを学ぶ | [BRANDING.md](../BRANDING.md) |
| adopter / 利用者の追跡 | [ADOPTERS.md](../ADOPTERS.md) |
| メンテナーを探す | [MAINTAINERS.md](../MAINTAINERS.md) |
| ガバナンスモデルを確認 | [GOVERNANCE.md](../GOVERNANCE.md) |
| 今後の作業を確認 | [ROADMAP.md](../ROADMAP.md) |

## ファミリー間互換性 (operator-commons)

3 つのデータベース Operator はすべて一致するバージョン (現在 `v0.7.0+`) の `github.com/keiailab/operator-commons` を import します:

```go
import (
    "github.com/keiailab/operator-commons/pkg/version"
    "github.com/keiailab/operator-commons/pkg/security"
    "github.com/keiailab/operator-commons/pkg/labels"
    "github.com/keiailab/operator-commons/pkg/monitoring"
    "github.com/keiailab/operator-commons/pkg/finalizer"
    "github.com/keiailab/operator-commons/pkg/status"
)
```

`operator-commons` の breaking change は 3 つのデータベース Operator すべてに同期した bump が必要 — supercycle Wave 5 の `make cross-validation` target で検証。

## i18n

本ページ (および全 canonical プロジェクトドキュメント) は 4 言語で提供されます:

- [English](family.md) (canonical、正本)
- [한국어](family.ko.md)
- **日本語** (本ファイル)
- [中文](family.zh.md)

技術内容については英語版が正本であり、現地化版は同じ意思決定を native な表現で反映します。

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a>
</p>

<p align="center">
  © 2026 keiailab · <a href="../LICENSE">Apache-2.0</a> · <a href="https://keiailab.com">keiailab.com</a>
</p>
