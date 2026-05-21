# valkey-operator (日本語)

> [English](README.md) | [한국어](README.ko.md) | **日本語** (placeholder) | [中文](README.zh.md) (placeholder)
>
> English README (canonical): [README.md](README.md)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![Valkey](https://img.shields.io/badge/Valkey-8.0+-FF4438?logo=redis)](https://valkey.io/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-326CE5?logo=kubernetes)](https://kubernetes.io/)

[Valkey](https://valkey.io/) (Redis の BSD-3 fork) のための Kubebuilder ベースの
Kubernetes operator。単一の controller が統一された CRD 表面の背後で 3 つの
運用トポロジを管理します。

> **状態**: `[~]` 部分実装 (placeholder) — RFC-0025 §1.2 チェックボックスの意味。
> native reviewer による品質検証後、`[x]` 完了状態へ昇格 candidate。

## CRD 概要

| CRD | 目的 | トポロジ |
|---|---|---|
| `Valkey` | 単一インスタンス、または 1 primary + N replica | Standalone / Replication |
| `ValkeyCluster` | シャード型 Valkey Cluster (16384 slot) | 3+ shard × (1 primary + 0–5 replica) |
| `ValkeyBackup` | One-shot RDB または AOF バックアップ | PVC (`<backup>-backup`)、外部ストレージはオプション |
| `ValkeyBackupTarget` | S3 互換外部ストレージ抽象 | Backup と Restore で共有 (ADR-0016) |
| `ValkeyRestore` | RDB を Valkey または ValkeyCluster インスタンスへ復元 | Init Container パターン (ADR-0015) |

operator は `StatefulSet`、`ConfigMap`、`Secret`、`Service` (headless + ClusterIP)、
`PodDisruptionBudget`、`NetworkPolicy`、`cert-manager` `Certificate`、Prometheus
`ServiceMonitor` を spec-drift detection 付きで reconcile します。

## Quickstart / インストール / CRD 例 / 運用

詳細は [English README](README.md) を参照してください。Native reviewer による
翻訳完了後、本 placeholder は拡張されます。

- Quickstart (kind): [README.md → Quickstart](README.md#quickstart-kind)
- CRD examples: [README.md → CRD examples](README.md)
- 既知の制限: [README.md → Known operational issues](README.md)

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) を参照。外部からの貢献を歓迎します。
非自明な変更については、コードを書く前に API 表面の整合を取るため、
issue を開いてください。

## 脆弱性報告

public issue を **開かないでください**。[SECURITY.md](SECURITY.md) の
private channel を使用してください — GitHub Security Advisory または
`security@keiailab.com` (PGP key は `artifacthub-repo.yml`)。

## License

Copyright 2026 Keiailab.

Apache License, Version 2.0 のもとでライセンスされます
(<http://www.apache.org/licenses/LICENSE-2.0>)。`AS IS` ベースで、いかなる
種類の保証または条件もなく頒布されます。完全な本文は [LICENSE](LICENSE)
ファイルを参照してください。

---

<p align="center">
  <b>keiailab operator family</b><br/>
  <a href="https://github.com/keiailab/operator-commons">operator-commons</a> ·
  <a href="https://github.com/keiailab/postgres-operator">postgres-operator</a> ·
  <a href="https://github.com/keiailab/mongodb-operator">mongodb-operator</a> ·
  <a href="https://github.com/keiailab/valkey-operator">valkey-operator</a> ·
  <a href="https://github.com/keiailab/forgewise">forgewise</a>
</p>

<p align="center">© 2026 keiailab · Apache-2.0 · <a href="https://keiailab.com">keiailab.com</a></p>
