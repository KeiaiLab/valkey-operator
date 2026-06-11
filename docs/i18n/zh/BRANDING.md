<p align="center">
  <a href="../../BRANDING.md">English</a> |
  <a href="../ko/BRANDING.md">한국어</a> |
  <a href="../ja/BRANDING.md">日本語</a> |
  <b>中文</b>
</p>

# 品牌指南 (Branding Guide) — `valkey-operator`

> keiailab operator 系列的视觉识别 (Visual Identity)、声音 (Voice) 与语调 (Tone)。

本文档是 `valkey-operator` 品牌决策的 canonical 参考。适用于 README、release notes、市场材料以及代表本项目的所有第三方沟通。

## 1. 身份标识 (Identity)

**Organization**: [keiailab](https://keiailab.com) — Kubernetes-native 数据平台 operator (MIT 许可、license-clean、vanilla-upstream 兼容)。

**Project**: `valkey-operator` — Kubernetes 的 MIT-licensed Valkey Operator — Standalone + Cluster + Backup/Restore,BSD-3 license-clean。

## 2. Logo 与视觉资源 (Visual Assets)

| 资源 | URL | 用途 |
|---|---|---|
| Current primary logo | `docs/branding/symbol.png` | README header, slides |
| Current favicon | `https://keiailab.com/favicon.ico` | Favicon, social cards |
| Planned SVG kit | `https://keiailab.com/assets/{logo,mark,wordmark}.svg` | Future replacement after URLs return 200 |

**Logo placement**: README 顶部居中,宽度 96px。始终链接到 https://keiailab.com。

**Clear space**: Logo 周围最小留白 = logo 宽度的 25%。

**禁止事项**:
- 修改 logo 颜色
- 添加阴影或滤镜
- 放置于对比度不足的背景上
- 未经 keiailab 品牌批准与其他 logo 组合

## 3. 调色板 (Color Palette)

| Role | Hex | Usage |
|---|---|---|
| Primary (keiailab teal) | `#0EA5A8` | 标题、primary 操作、链接 |
| Secondary (deep navy) | `#0F172A` | 深色背景、代码块 |
| Accent (warm amber) | `#F59E0B` | 强调、badge 点缀 |
| Neutral grey | `#64748B` | 浅色背景下的正文文字 |
| Background light | `#F8FAFC` | 文档页面背景 |
| Background dark | `#020617` | 代码编辑器主题、暗色模式 |

GitHub README 的 shield.io badge 建议使用上述 hex。

## 4. 排版 (Typography)

- **Headings**: System default (GitHub 默认的 `-apple-system, BlinkMacSystemFont, Segoe UI, ...`)
- **Body**: 同上 (与 GitHub-native 一致)
- **Code**: `ui-monospace, SFMono-Regular, Consolas, ...` (GitHub 默认 monospace)

不使用额外的 webfont (与 GitHub README rendering 保持一致)。

## 5. 声音与调性 (Voice & Tone)

**Audience**: Kubernetes 平台工程师 / DBA / SRE。

**声音原则 (Voice principles)**:
- **Direct (直接)** — 尽可能使用 bullet-point 代替段落
- **Evidence-based (基于证据)** — 论断需附 benchmark / SLA / 链接
- **Vendor-neutral (厂商中立)** — 引用 upstream (PostgreSQL、MongoDB、Valkey),但不 embed / wrap 第三方 operator
- **License-aware (许可证意识)** — 仅使用 MIT/BSD/Apache-2.0/PG-license 依赖

**应避免的表达**:
- 市场化的最高级表述 ("blazing fast"、"revolutionary"、"best-in-class")
- 模糊的比较 ("X-class quality") — *请使用具体指标或 benchmark 加以限定*
- Roadmap 中基于时间的截止期 (改用 `standards/roadmap.md §1.1` 的 feature 清单)

## 6. README Header 标准

所有 README 的首段须采用以下格式 (Wave 3 标准):

```markdown
<p align="center">
  <img src="docs/branding/symbol.png" alt="keiailab" width="96"/>
</p>

# valkey-operator

> **MIT-licensed Valkey Operator for Kubernetes — Standalone + Cluster + Backup/Restore, BSD-3 license-clean**

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-0EA5A8.svg" alt="License"/></a>
  <!-- 기존 shield.io badges 유지 + 정합 -->
</p>

<p align="center">
  <b>English</b> |
  <a href="README.ko.md">한국어</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.zh.md">中文</a>
</p>
```

## 7. README Footer 标准

所有 README 与根级 .md 文件的末尾须附以下 footer (Wave 3 标准):

```markdown```

## 8. Badges 标准顺序

README 中 shield.io badge 的顺序 (左→右):

1. License (MIT)
2. Go Version (1.25+)
3. Database (e.g. PostgreSQL 18+ / MongoDB 7.0+ / Valkey 8.0+)
4. Kubernetes Version (1.26+)
5. Container Image (ghcr.io/keiailab)
6. Helm Chart (Chart.yaml version + Artifact Hub link)
7. OpenSSF Scorecard
8. GitHub Discussions

## 9. Discussions / Issues / PR 模板

- **Discussions**: `https://github.com/keiailab/valkey-operator/discussions` — 功能想法、Q&A
- **Issues**: bug 报告 + 带具体用例的 feature request
- **PR template**: `.github/PULL_REQUEST_TEMPLATE.md` 标准 (强制引用用户场景 + 验证命令,`standards/checklist.md §3`)

## 10. 社交与外部链接 (Social & External)

- **Website**: https://keiailab.com
- **GitHub Org**: https://github.com/keiailab
- **Artifact Hub** (Helm): https://artifacthub.io/packages/search?repo=keiailab-valkey-operator
- **GHCR** (Container): https://github.com/keiailab/valkey-operator/pkgs/container/valkey-operator

## 11. 许可证与归属 (License & Attribution)

- License: [MIT](../../../LICENSE)
- Copyright: © 2026 keiailab contributors
- Third-party attributions: 见 [NOTICE](../../../NOTICE) (如适用)
