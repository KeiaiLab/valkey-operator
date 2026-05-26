<p align="center">
  <a href="../../BRANDING.md">English</a> |
  <a href="../ko/BRANDING.md">한국어</a> |
  <b>日本語</b> |
  <a href="../zh/BRANDING.md">中文</a>
</p>

# ブランディングガイド — `valkey-operator`

> keiailab operator ファミリーの視覚的アイデンティティ、ボイス、トーン。

本ドキュメントは `valkey-operator` のブランディング決定に関する canonical なリファレンスです。README、リリースノート、マーケティング資料、および本プロジェクトを代表するあらゆる第三者コミュニケーションに適用されます。

## 1. アイデンティティ

**Organization**: [keiailab](https://keiailab.com) — Kubernetes-native なデータプラットフォーム operator (Apache-2.0、license-clean、vanilla-upstream 互換)。

**Project**: `valkey-operator` — Kubernetes 向け Apache-2.0 Valkey Operator — Standalone + Cluster + Backup/Restore、BSD-3 license-clean。

## 2. ロゴおよびビジュアルアセット

| アセット | URL | 用途 |
|---|---|---|
| Primary logo (SVG) | `https://keiailab.com/assets/logo.svg` | README header、スライド |
| Mono mark | `https://keiailab.com/assets/mark.svg` | Favicon、ソーシャルカード |
| Wordmark | `https://keiailab.com/assets/wordmark.svg` | フッター、暗い背景 |

**Logo placement**: README の上部中央、幅 120px。常に https://keiailab.com へリンクします。

**Clear space**: ロゴ周囲の最小パディング = ロゴ幅の 25%。

**禁止事項**:
- ロゴの再着色
- ドロップシャドウやフィルターの追加
- コントラストが不十分な背景への配置
- keiailab ブランドの承認なしに他ロゴと組み合わせること

## 3. カラーパレット

| Role | Hex | Usage |
|---|---|---|
| Primary (keiailab teal) | `#0EA5A8` | ヘッダー、primary アクション、リンク |
| Secondary (deep navy) | `#0F172A` | 暗い背景、コードブロック |
| Accent (warm amber) | `#F59E0B` | ハイライト、badge アクセント |
| Neutral grey | `#64748B` | 明るい背景上の本文テキスト |
| Background light | `#F8FAFC` | ドキュメントページ背景 |
| Background dark | `#020617` | コードエディタテーマ、ダークモード |

GitHub README の shield.io badge には上記 hex の利用を推奨します。

## 4. タイポグラフィ

- **Headings**: System default (GitHub の default `-apple-system, BlinkMacSystemFont, Segoe UI, ...`)
- **Body**: 同上 (GitHub-native との整合)
- **Code**: `ui-monospace, SFMono-Regular, Consolas, ...` (GitHub の default monospace)

別途 webfont は使用しません (GitHub README rendering との整合のため)。

## 5. ボイスとトーン

**Audience**: Kubernetes プラットフォームエンジニア / DBA / SRE。

**ボイス原則**:
- **Direct (直接的)** — 可能な場合は段落より bullet-point を優先
- **Evidence-based (根拠ベース)** — 主張には benchmark / SLA / リンクを含める
- **Vendor-neutral (ベンダーニュートラル)** — upstream (PostgreSQL、MongoDB、Valkey) を参照するが、第三者 operator を embed / wrap しない
- **License-aware (ライセンス意識)** — Apache-2.0 + BSD/MIT/PG-license の依存関係のみ

**避けるべき表現**:
- マーケティング的な最上級表現 ("blazing fast"、"revolutionary"、"best-in-class")
- 曖昧な比較 ("X-class quality") — *具体的なメトリクスまたは benchmark で裏付ける*
- ロードマップでの時間ベースの締切 (`standards/roadmap.md §1.1` の feature チェックリストを利用)

## 6. README ヘッダー標準

すべての README の最初のブロックは以下の形式とします (Wave 3 標準):

```markdown
<p align="center">
  <img src="https://keiailab.com/assets/logo.svg" alt="keiailab" width="120"/>
</p>

# valkey-operator

> **Apache-2.0 Valkey Operator for Kubernetes — Standalone + Cluster + Backup/Restore, BSD-3 license-clean**

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"/></a>
  <!-- 기존 shield.io badges 유지 + 정합 -->
</p>

<p align="center">
  <b>English</b> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.zh.md">中文</a>
</p>
```

## 7. README フッター標準

すべての README + root-level .md ファイルの末尾に次の footer を付与します (Wave 3 標準):

```markdown```

## 8. Badges 標準順序

README の shield.io badge の順序 (左→右):

1. License (Apache-2.0)
2. Go Version (1.25+)
3. Database (e.g. PostgreSQL 18+ / MongoDB 7.0+ / Valkey 8.0+)
4. Kubernetes Version (1.26+)
5. Container Image (ghcr.io/keiailab)
6. Helm Chart (Chart.yaml version + Artifact Hub link)
7. OpenSSF Scorecard
8. GitHub Discussions

## 9. Discussions / Issues / PR テンプレート

- **Discussions**: `https://github.com/keiailab/valkey-operator/discussions` — 機能アイデア、Q&A
- **Issues**: バグレポート + ユースケースを伴う具体的な feature request
- **PR template**: `.github/PULL_REQUEST_TEMPLATE.md` 標準 (ユーザーシナリオ + 検証コマンドの引用が必須、`standards/checklist.md §3`)

## 10. ソーシャルおよび外部リンク

- **Website**: https://keiailab.com
- **GitHub Org**: https://github.com/keiailab
- **Artifact Hub** (Helm): https://artifacthub.io/packages/search?repo=keiailab-valkey-operator
- **GHCR** (Container): https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator

## 11. ライセンスおよび表記

- License: [Apache-2.0](../../../LICENSE)
- Copyright: © 2026 keiailab contributors
- Third-party attributions: [NOTICE](../../../NOTICE) を参照 (該当する場合)
